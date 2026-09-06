package wallet

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ListBudgetsParams are the query parameters of GET /v1/api/budgets.
type ListBudgetsParams struct {
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
	// CurrencyCode filters by ISO 4217 code, case-insensitively.
	CurrencyCode *string
	// Closed selects closed (true) or open (false) budgets. Nil returns both.
	Closed *bool
	// Type filters by budget interval. BUDGET_ALL is read-only legacy but can
	// still be filtered on.
	Type *BudgetType
	// LabelID returns budgets carrying this label.
	LabelID *string
	// AccountID returns budgets tracking this account.
	AccountID *string
	// CategoryIDs returns budgets tracking these categories, up to 10. The
	// magic value CategoryUnknown expands to the three unknown categories.
	CategoryIDs []string
	// StartDate filters by budget start; up to two range conditions.
	StartDate []Filter
	// EndDate filters by budget end; up to two range conditions.
	EndDate []Filter
	// CreatedAt filters by creation timestamp; up to two range conditions.
	CreatedAt []Filter
	// UpdatedAt filters by last sync timestamp; up to two range conditions.
	UpdatedAt []Filter
	// Spending controls spending depth: "none" (the REST default), "current",
	// or "current+N" for N past periods.
	Spending *string
}

// Validate checks every documented constraint on the parameters.
func (p *ListBudgetsParams) Validate() error {
	if p == nil {
		return nil
	}
	var probs problems
	probs.addErr(validateLimit(p.Limit))
	probs.addErr(validateOffset(p.Offset))
	probs.addErr(validateIDList("id", p.IDs, MaxIDFilterValues))
	probs.addErr(validateIDList("categoryId", p.CategoryIDs, MaxCategoryIDFilterValues))
	if p.Name != nil {
		probs.addErr(validateTextFilter("name", *p.Name))
	}
	if p.CurrencyCode != nil {
		probs.addErr(validateCurrencyCode("currencyCode", *p.CurrencyCode))
	}
	if p.Type != nil && !p.Type.Valid() {
		probs.add("type", "%q is not a budget type; allowed: %s", *p.Type, enumNames(AllBudgetTypes))
	}
	probs.addErr(validateRangeFilters("startDate", p.StartDate))
	probs.addErr(validateRangeFilters("endDate", p.EndDate))
	probs.addErr(validateRangeFilters("createdAt", p.CreatedAt))
	probs.addErr(validateRangeFilters("updatedAt", p.UpdatedAt))
	if p.Spending != nil {
		probs.addErr(validateSpendingDepth("spending", *p.Spending))
	}
	return probs.err()
}

func (p *ListBudgetsParams) query() url.Values {
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
	q.setString("currencyCode", p.CurrencyCode)
	q.setBool("closed", p.Closed)
	setEnum(q, "type", p.Type)
	q.setString("labelId", p.LabelID)
	q.setString("accountId", p.AccountID)
	q.setCSV("categoryId", p.CategoryIDs)
	q.setFilters("startDate", p.StartDate)
	q.setFilters("endDate", p.EndDate)
	q.setFilters("createdAt", p.CreatedAt)
	q.setFilters("updatedAt", p.UpdatedAt)
	q.setString("spending", p.Spending)
	return q.values
}

// ListBudgets returns one page of budgets, newest first by default.
//
// Operation: getBudgets. GET /v1/api/budgets.
func (c *Client) ListBudgets(ctx context.Context, params *ListBudgetsParams) (*BudgetsPage, error) {
	if err := c.validate(params); err != nil {
		return nil, err
	}
	out := &BudgetsPage{}
	err := c.do(ctx, request{
		op:     "getBudgets",
		method: http.MethodGet,
		path:   "/budgets",
		query:  params.query(),
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CreateBudgetRequest is the body of POST /v1/api/budgets.
type CreateBudgetRequest struct {
	// Name is the display name. Required.
	Name string `json:"name"`
	// CurrencyCode is the 3-letter currency. Required.
	CurrencyCode string `json:"currencyCode"`
	// Type is the budget interval. Required and immutable after creation.
	// BUDGET_CUSTOM additionally requires StartDate and EndDate.
	Type BudgetType `json:"type"`
	// Limit is the spending limit. Required, and must be greater than zero.
	Limit float64 `json:"limit"`
	// AccountIDs are the accounts to track.
	AccountIDs []string `json:"accountIds,omitempty"`
	// CategoryIDs are the categories to track.
	CategoryIDs []string `json:"categoryIds,omitempty"`
	// LabelIDs are the labels to filter transactions by.
	LabelIDs []string `json:"labelIds,omitempty"`
	// StartDate is the first day. Required for BUDGET_CUSTOM.
	StartDate *Date `json:"startDate,omitempty"`
	// EndDate is the last day. Required for BUDGET_CUSTOM.
	EndDate *Date `json:"endDate,omitempty"`
}

// Validate checks every documented constraint on the body.
func (r *CreateBudgetRequest) Validate() error {
	if r == nil {
		return errorf("", "request body is required")
	}
	var p problems
	p.addErr(validateRequiredText("name", r.Name, MaxNameLength))
	p.addErr(validateCurrencyCode("currencyCode", r.CurrencyCode))
	switch {
	case r.Type == "":
		p.add("type", "is required")
	case !r.Type.Creatable():
		p.add("type", "%q cannot be created; allowed: %s", r.Type, enumNames(CreatableBudgetTypes))
	}
	if r.Limit <= 0 {
		p.add("limit", "must be greater than 0, got %v", r.Limit)
	}
	if r.Type == BudgetCustom {
		if r.StartDate == nil || !r.StartDate.Valid() {
			p.add("startDate", "is required for %s", BudgetCustom)
		}
		if r.EndDate == nil || !r.EndDate.Valid() {
			p.add("endDate", "is required for %s", BudgetCustom)
		}
	}
	if r.StartDate != nil && r.EndDate != nil && r.StartDate.Valid() && r.EndDate.Valid() &&
		r.EndDate.Time.Before(r.StartDate.Time) {
		p.add("endDate", "must not be before startDate (%s < %s)", r.EndDate, r.StartDate)
	}
	p.addErr(validateIDList("accountIds", r.AccountIDs, MaxIDFilterValues))
	p.addErr(validateIDList("categoryIds", r.CategoryIDs, MaxIDFilterValues))
	p.addErr(validateIDList("labelIds", r.LabelIDs, MaxIDFilterValues))
	return p.err()
}

// CreateBudget creates a budget.
//
// Operation: createBudget. POST /v1/api/budgets. Answers 201 on success.
func (c *Client) CreateBudget(ctx context.Context, body *CreateBudgetRequest) (*CreateBudgetResponse, error) {
	if err := c.validate(body); err != nil {
		return nil, err
	}
	out := &CreateBudgetResponse{}
	err := c.do(ctx, request{
		op:     "createBudget",
		method: http.MethodPost,
		path:   "/budgets",
		body:   body,
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// LimitOverrideInput is one entry of a budget's limitOverrides array.
//
// Overrides are EFFECTIVE-FROM: an entry sets the limit for its period and
// every period after it, until a later entry takes over. PeriodCount bounds
// it to N periods counting its own.
type LimitOverrideInput struct {
	// Period is the starting period: YYYY-MM, YYYY-Www or YYYY. Required, and
	// must not be earlier than the current period.
	Period string `json:"period"`
	// Limit is the new spending limit, greater than zero. It is mutually
	// exclusive with SetBaseline.
	Limit *float64 `json:"limit,omitempty"`
	// PeriodCount is how many periods the override lasts, counting its own:
	// 1 means this period only. It requires Limit and is not allowed with
	// SetBaseline. Omit for an open-ended override.
	PeriodCount *int `json:"periodCount,omitempty"`
	// SetBaseline writes an entry holding the budget's current limit, ending
	// any earlier override from this period on. It is mutually exclusive with
	// Limit and with PeriodCount.
	SetBaseline *bool `json:"setBaseline,omitempty"`
}

// Validate checks the mutual exclusions the API documents.
func (o *LimitOverrideInput) Validate() error {
	if o == nil {
		return errorf("", "limit override is required")
	}
	var p problems
	p.addErr(validatePeriodFormat("period", o.Period))
	baseline := o.SetBaseline != nil && *o.SetBaseline
	switch {
	case o.Limit != nil && baseline:
		p.add("limit", "is mutually exclusive with setBaseline")
	case o.Limit == nil && !baseline:
		p.add("limit", "one of limit or setBaseline is required")
	}
	if o.Limit != nil && *o.Limit <= 0 {
		p.add("limit", "must be greater than 0, got %v", *o.Limit)
	}
	if o.PeriodCount != nil {
		if baseline {
			p.add("periodCount", "is not allowed with setBaseline")
		}
		if o.Limit == nil {
			p.add("periodCount", "requires limit")
		}
		if *o.PeriodCount < MinPeriodCount || *o.PeriodCount > MaxPeriodCount {
			p.add("periodCount", "must be between %d and %d, got %d", MinPeriodCount, MaxPeriodCount, *o.PeriodCount)
		}
	}
	return p.err()
}

// PatchBudgetRequest is one item of PATCH /v1/api/budgets.
type PatchBudgetRequest struct {
	// ID identifies the budget to update. Required.
	ID string `json:"id"`
	// Name is the new display name, at most 80 characters.
	Name *string `json:"name,omitempty"`
	// AccountIDs replaces or deltas the tracked accounts.
	AccountIDs *StringSet `json:"accountIds,omitempty"`
	// CategoryIDs replaces or deltas the tracked categories.
	CategoryIDs *StringSet `json:"categoryIds,omitempty"`
	// LabelIDs replaces or deltas the label filter.
	LabelIDs *StringSet `json:"labelIds,omitempty"`
	// Closed closes (true, setting closedDate to today UTC) or reopens
	// (false, clearing closedDate) the budget.
	Closed *bool `json:"closed,omitempty"`
	// StartDate is the new first day. Only patchable on BUDGET_CUSTOM.
	StartDate *Date `json:"startDate,omitempty"`
	// EndDate is the new last day. Only patchable on BUDGET_CUSTOM.
	EndDate *Date `json:"endDate,omitempty"`
	// LimitOverrides replaces ALL current and future overrides — one not
	// re-sent is deleted, not merged. Past overrides are preserved. It is not
	// allowed on BUDGET_CUSTOM. At most 50 entries.
	LimitOverrides []LimitOverrideInput `json:"limitOverrides,omitempty"`
	// ResetLimit clears every override, past and future, and sets a uniform
	// baseline limit. It works on all budget types and, combined with
	// LimitOverrides, is applied first.
	ResetLimit *float64 `json:"resetLimit,omitempty"`
}

// Validate checks every documented constraint on one patch item.
//
// It cannot check the two constraints that depend on server state — that
// StartDate, EndDate and LimitOverrides are only legal for the right budget
// type, and that Period is not in the past — because the budget's type and
// the server's notion of "now" are not available here. The API enforces both.
func (r *PatchBudgetRequest) Validate() error {
	if r == nil {
		return errorf("", "patch item is required")
	}
	var p problems
	p.addErr(validateRequiredID("id", r.ID))
	if r.Name != nil {
		p.addErr(validateRequiredText("name", *r.Name, MaxNameLength))
	}
	p.addErr(r.AccountIDs.Validate("accountIds"))
	p.addErr(r.CategoryIDs.Validate("categoryIds"))
	p.addErr(r.LabelIDs.Validate("labelIds"))
	if len(r.LimitOverrides) > MaxLimitOverrides {
		p.add("limitOverrides", "at most %d entries allowed, got %d", MaxLimitOverrides, len(r.LimitOverrides))
	}
	seenPeriod := map[string]int{}
	for i := range r.LimitOverrides {
		at := fmt.Sprintf("limitOverrides[%d]", i)
		p.prefixed(at, r.LimitOverrides[i].Validate())
		if period := r.LimitOverrides[i].Period; period != "" {
			if first, dup := seenPeriod[period]; dup {
				p.add(at+".period", "duplicates limitOverrides[%d] (%q)", first, period)
			} else {
				seenPeriod[period] = i
			}
		}
	}
	if r.ResetLimit != nil && *r.ResetLimit <= 0 {
		p.add("resetLimit", "must be greater than 0, got %v", *r.ResetLimit)
	}
	if r.StartDate != nil && r.EndDate != nil && r.StartDate.Valid() && r.EndDate.Valid() &&
		r.EndDate.Time.Before(r.StartDate.Time) {
		p.add("endDate", "must not be before startDate (%s < %s)", r.EndDate, r.StartDate)
	}
	if r.Name == nil && r.AccountIDs == nil && r.CategoryIDs == nil && r.LabelIDs == nil &&
		r.Closed == nil && r.StartDate == nil && r.EndDate == nil &&
		r.LimitOverrides == nil && r.ResetLimit == nil {
		p.add("", "no fields to update; set at least one field besides id")
	}
	return p.err()
}

// PatchBudgetsParams are the query parameters of PATCH /v1/api/budgets.
type PatchBudgetsParams struct {
	// ReturnData controls whether each result carries the updated budget. The
	// server defaults to true.
	ReturnData *bool
}

func (p *PatchBudgetsParams) query() url.Values {
	q := newQuery()
	if p == nil {
		return q.values
	}
	q.setBool("returnData", p.ReturnData)
	return q.values
}

type patchBudgetsBody []PatchBudgetRequest

func (b patchBudgetsBody) Validate() error {
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

// PatchBudgets updates up to 10 budgets in one call.
//
// Operation: patchBudgets. PATCH /v1/api/budgets. The response envelope is
// returned for 200, 207, 400 and 500 alike; inspect Summary and Results.
func (c *Client) PatchBudgets(ctx context.Context, items []PatchBudgetRequest, params *PatchBudgetsParams) (*PatchBudgetsResponse, error) {
	if err := c.validate(patchBudgetsBody(items)); err != nil {
		return nil, err
	}
	out := &PatchBudgetsResponse{}
	err := c.do(ctx, request{
		op:     "patchBudgets",
		method: http.MethodPatch,
		path:   "/budgets",
		query:  params.query(),
		body:   items,
		batch:  true,
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// compile-time assurance that the patch body marshals as a bare array.
var _ json.Marshaler = (*StringSet)(nil)
