package wallet

import (
	"context"
	"net/http"
	"net/url"
)

// DeleteRequest is the body of DELETE /v1/api/{type}.
type DeleteRequest struct {
	// IDs are the entities to delete, 1 to 10 of them.
	IDs []string `json:"ids"`
}

// Validate checks the batch bounds and the IDs themselves.
func (r *DeleteRequest) Validate() error {
	if r == nil {
		return errorf("", "request body is required")
	}
	var p problems
	p.addErr(validateBatchSize("ids", len(r.IDs), MaxDeleteIDs))
	p.addErr(validateIDList("ids", r.IDs, MaxDeleteIDs))
	return p.err()
}

// deleteArgs bundles the path type with the body so both validate together.
type deleteArgs struct {
	entityType EntityType
	body       *DeleteRequest
}

func (a deleteArgs) Validate() error {
	var p problems
	p.addErr(validateEntityType("type", a.entityType, DeletableEntityTypes))
	p.addErr(a.body.Validate())
	return p.err()
}

// Delete removes up to 10 entities of one type.
//
// Operation: deleteByType. DELETE /v1/api/{type}. The response envelope is
// returned for 200, 207, 400 and 500 alike; inspect Summary and Results.
//
// Two rules deserve attention. For accounts, categories, labels and records,
// call References first — deleting something another entity points at is
// rejected. And a paired transfer deletes as a pair: both record IDs must be
// in the same call, or the delete is refused and the error names the missing
// leg.
//
// Delete is idempotent per ID, so a repeated ID succeeds while contributing
// nothing to Summary.DocumentsWritten.
func (c *Client) Delete(ctx context.Context, entityType EntityType, ids []string) (*DeleteResponse, error) {
	body := &DeleteRequest{IDs: ids}
	if err := c.validate(deleteArgs{entityType: entityType, body: body}); err != nil {
		return nil, err
	}
	out := &DeleteResponse{}
	err := c.do(ctx, request{
		op:     "deleteByType",
		method: http.MethodDelete,
		path:   "/" + url.PathEscape(string(entityType)),
		body:   body,
		batch:  true,
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ReferencesParams are the parameters of GET /v1/api/{type}/references.
type ReferencesParams struct {
	// Type is the collection to check. Plural forms are canonical; the
	// singular forms category, record, account and label are normalised.
	// Budgets, standing orders and record rules are unsupported because
	// nothing references them.
	Type EntityType
	// IDs are the entities to check, 1 to 20 of them. Required.
	IDs []string
}

// Validate checks the type and the ID list.
func (p *ReferencesParams) Validate() error {
	if p == nil {
		return errorf("", "parameters are required")
	}
	var probs problems
	probs.addErr(validateEntityType("type", NormalizeEntityType(p.Type), ReferenceableEntityTypes))
	if len(p.IDs) == 0 {
		probs.add("id", "at least one ID is required")
	}
	probs.addErr(validateIDList("id", p.IDs, MaxReferenceIDs))
	return probs.err()
}

func (p *ReferencesParams) query() url.Values {
	q := newQuery()
	if p == nil {
		return q.values
	}
	q.setCSV("id", p.IDs)
	return q.values
}

// References reports which entities point at the given ones, so a caller can
// tell whether a delete would be rejected.
//
// Operation: getReferences. GET /v1/api/{type}/references. The response is a
// map keyed by entity ID; an ID that does not resolve gets an entry carrying
// Error ("not found" or "type_mismatch") instead of reference counts, which
// is why an unknown ID does not fail the whole call.
func (c *Client) References(ctx context.Context, params *ReferencesParams) (*ReferencesResult, error) {
	if err := c.validate(params); err != nil {
		return nil, err
	}
	entityType := NormalizeEntityType(params.Type)
	out := &ReferencesResult{}
	err := c.do(ctx, request{
		op:     "getReferences",
		method: http.MethodGet,
		path:   "/" + url.PathEscape(string(entityType)) + "/references",
		query:  params.query(),
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteUnreferenced is the safe delete the API documentation asks for: it
// checks references first and deletes only the IDs nothing points at.
//
// It returns the delete response, the references it consulted, and the IDs it
// skipped with the reason each was skipped. When every ID is skipped no
// request is sent and the delete response is nil, so check it before use.
//
// Types nothing can reference — budgets, standing orders and record rules —
// skip the check and go straight to the delete.
func (c *Client) DeleteUnreferenced(ctx context.Context, entityType EntityType, ids []string) (*DeleteResponse, *ReferencesResult, map[string]string, error) {
	normalized := NormalizeEntityType(entityType)
	if !normalized.NeedsReferenceCheck() {
		resp, err := c.Delete(ctx, normalized, ids)
		return resp, nil, nil, err
	}

	refs, err := c.References(ctx, &ReferencesParams{Type: normalized, IDs: ids})
	if err != nil {
		return nil, nil, nil, err
	}

	skipped := map[string]string{}
	var deletable []string
	for _, id := range ids {
		entry, ok := refs.Entries[id]
		switch {
		case !ok:
			skipped[id] = "no reference entry returned for this id"
		case entry.Error != "":
			skipped[id] = entry.Error
		case entry.TotalReferences() > 0:
			skipped[id] = referencedReason(entry)
		default:
			deletable = append(deletable, id)
		}
	}
	if len(deletable) == 0 {
		return nil, refs, skipped, nil
	}
	resp, err := c.Delete(ctx, normalized, deletable)
	return resp, refs, skipped, err
}

// referencedReason renders which kinds of entity block a delete.
func referencedReason(entry ReferencesResponse) string {
	parts := make([]string, 0, 5)
	for _, kind := range []struct {
		name string
		res  *ReferenceResult
	}{
		{"records", entry.Records},
		{"standingOrders", entry.StandingOrders},
		{"budgets", entry.Budgets},
		{"recordRules", entry.RecordRules},
		{"other", entry.Other},
	} {
		if kind.res != nil && kind.res.Total > 0 {
			parts = append(parts, kind.name)
		}
	}
	if len(parts) == 0 {
		return "referenced by other entities"
	}
	return "referenced by " + joinComma(parts)
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
