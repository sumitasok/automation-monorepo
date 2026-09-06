package wallet

import (
	"context"
	"net/http"
	"net/url"
)

// ListLabelsParams are the query parameters of GET /v1/api/labels.
type ListLabelsParams struct {
	// Limit is the page size, 1-200. The server defaults to 30.
	Limit *int
	// Offset skips items for pagination.
	Offset *int
	// AgentHints asks for structured hints alongside the data.
	AgentHints *bool
	// WithTotal asks for a total count, at the cost of an extra query.
	WithTotal *bool
	// IDs filters by identifier, up to 30 of them.
	IDs []string
	// Name filters by name and requires a text prefix.
	Name *Filter
	// CreatedAt filters by creation timestamp; up to two range conditions.
	CreatedAt []Filter
	// UpdatedAt filters by last sync timestamp; up to two range conditions.
	UpdatedAt []Filter
	// Archived selects archived (true) or active (false) labels.
	Archived *bool
	// RecordID returns only the labels a given record uses.
	RecordID *string
	// BudgetID returns only the labels a given budget uses.
	BudgetID *string
	// StandingOrderID returns only the labels a given standing order uses.
	StandingOrderID *string
	// RecordRuleID returns only the labels a given record rule uses.
	RecordRuleID *string
	// SortBy orders the page. Only createdAt and updatedAt are sortable; the
	// server otherwise defaults to name ascending.
	SortBy *Sort
}

// Validate checks every documented constraint on the parameters.
func (p *ListLabelsParams) Validate() error {
	if p == nil {
		return nil
	}
	var probs problems
	probs.addErr(validateLimit(p.Limit))
	probs.addErr(validateOffset(p.Offset))
	probs.addErr(validateIDList("id", p.IDs, MaxIDFilterValues))
	if p.Name != nil {
		probs.addErr(validateTextFilter("name", *p.Name))
	}
	probs.addErr(validateRangeFilters("createdAt", p.CreatedAt))
	probs.addErr(validateRangeFilters("updatedAt", p.UpdatedAt))
	probs.addErr(validateSort("sortBy", p.SortBy, CommonSortFields))
	return probs.err()
}

func (p *ListLabelsParams) query() url.Values {
	q := newQuery()
	if p == nil {
		return q.values
	}
	q.setInt("limit", p.Limit)
	q.setInt("offset", p.Offset)
	q.setBool("agentHints", p.AgentHints)
	q.setBool("withTotal", p.WithTotal)
	q.setCSV("id", p.IDs)
	q.setFilter("name", p.Name)
	q.setFilters("createdAt", p.CreatedAt)
	q.setFilters("updatedAt", p.UpdatedAt)
	q.setBool("archived", p.Archived)
	q.setString("recordId", p.RecordID)
	q.setString("budgetId", p.BudgetID)
	q.setString("standingOrderId", p.StandingOrderID)
	q.setString("recordRuleId", p.RecordRuleID)
	q.setSort("sortBy", p.SortBy)
	return q.values
}

// ListLabels returns one page of labels, name-ascending by default.
//
// Operation: getLabels. GET /v1/api/labels.
func (c *Client) ListLabels(ctx context.Context, params *ListLabelsParams) (*LabelsPage, error) {
	if err := c.validate(params); err != nil {
		return nil, err
	}
	out := &LabelsPage{}
	err := c.do(ctx, request{
		op:     "getLabels",
		method: http.MethodGet,
		path:   "/labels",
		query:  params.query(),
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CreateLabelRequest is the body of POST /v1/api/labels.
type CreateLabelRequest struct {
	// Name is the label name. Required, at most 80 characters.
	Name string `json:"name"`
	// Color is a universal colour name. A random one is assigned if omitted.
	Color *UniversalColor `json:"color,omitempty"`
}

// Validate checks every documented constraint on the body.
func (r *CreateLabelRequest) Validate() error {
	if r == nil {
		return errorf("", "request body is required")
	}
	var p problems
	p.addErr(validateRequiredText("name", r.Name, MaxNameLength))
	p.addErr(validateColor("color", r.Color))
	return p.err()
}

// CreateLabel creates a label.
//
// Operation: createLabel. POST /v1/api/labels. Answers 201 on success.
func (c *Client) CreateLabel(ctx context.Context, body *CreateLabelRequest) (*CreateLabelResponse, error) {
	if err := c.validate(body); err != nil {
		return nil, err
	}
	out := &CreateLabelResponse{}
	err := c.do(ctx, request{
		op:     "createLabel",
		method: http.MethodPost,
		path:   "/labels",
		body:   body,
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PatchLabelItem is one item of PATCH /v1/api/labels.
type PatchLabelItem struct {
	// ID identifies the label to update. Required.
	ID string `json:"id"`
	// Name is the new label name, at most 80 characters.
	Name *string `json:"name,omitempty"`
	// Archived sets or clears the archived flag.
	Archived *bool `json:"archived,omitempty"`
	// Color is the new universal colour name.
	Color *UniversalColor `json:"color,omitempty"`
}

// Validate checks every documented constraint on one patch item.
func (i *PatchLabelItem) Validate() error {
	if i == nil {
		return errorf("", "patch item is required")
	}
	var p problems
	p.addErr(validateRequiredID("id", i.ID))
	if i.Name != nil {
		p.addErr(validateRequiredText("name", *i.Name, MaxNameLength))
	}
	p.addErr(validateColor("color", i.Color))
	if i.Name == nil && i.Archived == nil && i.Color == nil {
		p.add("", "no fields to update; set at least one field besides id")
	}
	return p.err()
}

// PatchLabelsParams are the query parameters of PATCH /v1/api/labels.
type PatchLabelsParams struct {
	// ReturnData controls whether each result carries the updated label. The
	// server defaults to true.
	ReturnData *bool
}

func (p *PatchLabelsParams) query() url.Values {
	q := newQuery()
	if p == nil {
		return q.values
	}
	q.setBool("returnData", p.ReturnData)
	return q.values
}

type patchLabelsBody []PatchLabelItem

func (b patchLabelsBody) Validate() error {
	var p problems
	p.addErr(validateBatchSize("items", len(b), MaxPatchItems))
	seen := map[string]int{}
	for i := range b {
		p.prefixed(itemPath(i), b[i].Validate())
		if id := b[i].ID; id != "" {
			if first, dup := seen[id]; dup {
				p.add(itemPath(i)+".id", "duplicates items[%d] (%q); the API applies both in order", first, id)
			} else {
				seen[id] = i
			}
		}
	}
	return p.err()
}

// PatchLabels updates up to 10 labels in one call.
//
// Operation: patchLabels. PATCH /v1/api/labels. The response envelope is
// returned for 200, 207, 400 and 500 alike; inspect Summary and Results.
func (c *Client) PatchLabels(ctx context.Context, items []PatchLabelItem, params *PatchLabelsParams) (*PatchLabelsResponse, error) {
	if err := c.validate(patchLabelsBody(items)); err != nil {
		return nil, err
	}
	out := &PatchLabelsResponse{}
	err := c.do(ctx, request{
		op:     "patchLabels",
		method: http.MethodPatch,
		path:   "/labels",
		query:  params.query(),
		body:   items,
		batch:  true,
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
