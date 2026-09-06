package wallet

import (
	"context"
	"net/http"
	"net/url"
)

// ---------------------------------------------------------------------------
// Goals
// ---------------------------------------------------------------------------

// ListGoalsParams are the query parameters of GET /v1/api/goals.
type ListGoalsParams struct {
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
	// Note filters by note and requires a text prefix.
	Note *Filter
	// CreatedAt filters by creation timestamp; up to two range conditions.
	CreatedAt []Filter
	// UpdatedAt filters by last sync timestamp; up to two range conditions.
	UpdatedAt []Filter
}

// Validate checks every documented constraint on the parameters.
func (p *ListGoalsParams) Validate() error {
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
	if p.Note != nil {
		probs.addErr(validateTextFilter("note", *p.Note))
	}
	probs.addErr(validateRangeFilters("createdAt", p.CreatedAt))
	probs.addErr(validateRangeFilters("updatedAt", p.UpdatedAt))
	return probs.err()
}

func (p *ListGoalsParams) query() url.Values {
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
	q.setFilter("note", p.Note)
	q.setFilters("createdAt", p.CreatedAt)
	q.setFilters("updatedAt", p.UpdatedAt)
	return q.values
}

// ListGoals returns one page of financial goals, newest first by default.
//
// Operation: getGoals. GET /v1/api/goals.
func (c *Client) ListGoals(ctx context.Context, params *ListGoalsParams) (*GoalsPage, error) {
	if err := c.validate(params); err != nil {
		return nil, err
	}
	out := &GoalsPage{}
	err := c.do(ctx, request{
		op:     "getGoals",
		method: http.MethodGet,
		path:   "/goals",
		query:  params.query(),
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Record rules
// ---------------------------------------------------------------------------

// ListRecordRulesParams are the query parameters of GET /v1/api/record-rules.
type ListRecordRulesParams struct {
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
	// LabelID returns rules that apply this label.
	LabelID *string
	// CreatedAt filters by creation timestamp; up to two range conditions.
	CreatedAt []Filter
	// UpdatedAt filters by last sync timestamp; up to two range conditions.
	UpdatedAt []Filter
}

// Validate checks every documented constraint on the parameters.
func (p *ListRecordRulesParams) Validate() error {
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
	return probs.err()
}

func (p *ListRecordRulesParams) query() url.Values {
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
	q.setString("labelId", p.LabelID)
	q.setFilters("createdAt", p.CreatedAt)
	q.setFilters("updatedAt", p.UpdatedAt)
	return q.values
}

// ListRecordRules returns one page of automatic categorisation rules, newest
// first by default.
//
// Operation: getRecordRules. GET /v1/api/record-rules.
func (c *Client) ListRecordRules(ctx context.Context, params *ListRecordRulesParams) (*RecordRulesPage, error) {
	if err := c.validate(params); err != nil {
		return nil, err
	}
	out := &RecordRulesPage{}
	err := c.do(ctx, request{
		op:     "getRecordRules",
		method: http.MethodGet,
		path:   "/record-rules",
		query:  params.query(),
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Standing orders
// ---------------------------------------------------------------------------

// ListStandingOrdersParams are the query parameters of
// GET /v1/api/standing-orders.
type ListStandingOrdersParams struct {
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
	// LabelID returns standing orders carrying this label.
	LabelID *string
	// CreatedAt filters by creation timestamp; up to two range conditions.
	CreatedAt []Filter
	// UpdatedAt filters by last sync timestamp; up to two range conditions.
	UpdatedAt []Filter
}

// Validate checks every documented constraint on the parameters.
func (p *ListStandingOrdersParams) Validate() error {
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
	if p.CurrencyCode != nil {
		probs.addErr(validateCurrencyCode("currencyCode", *p.CurrencyCode))
	}
	probs.addErr(validateRangeFilters("createdAt", p.CreatedAt))
	probs.addErr(validateRangeFilters("updatedAt", p.UpdatedAt))
	return probs.err()
}

func (p *ListStandingOrdersParams) query() url.Values {
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
	q.setString("labelId", p.LabelID)
	q.setFilters("createdAt", p.CreatedAt)
	q.setFilters("updatedAt", p.UpdatedAt)
	return q.values
}

// ListStandingOrders returns one page of recurring payments, newest first by
// default.
//
// Operation: getStandingOrders. GET /v1/api/standing-orders.
func (c *Client) ListStandingOrders(ctx context.Context, params *ListStandingOrdersParams) (*StandingOrdersPage, error) {
	if err := c.validate(params); err != nil {
		return nil, err
	}
	out := &StandingOrdersPage{}
	err := c.do(ctx, request{
		op:     "getStandingOrders",
		method: http.MethodGet,
		path:   "/standing-orders",
		query:  params.query(),
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListStandingOrderItemsParams are the query parameters of
// GET /v1/api/standing-orders/items.
type ListStandingOrderItemsParams struct {
	// Limit is the page size, 1-200. The server defaults to 30.
	Limit *int
	// Offset skips items for pagination.
	Offset *int
	// AgentHints asks for structured hints alongside the data.
	AgentHints *bool
	// WithTotal asks for a total count, at the cost of an extra query.
	WithTotal *bool
	// StandingOrderID scopes the query to one standing order. Omit it for a
	// cross-order query.
	StandingOrderID *string
	// OriginalDate filters by the scheduled date; up to two range conditions.
	OriginalDate []Filter
	// Dismissed selects dismissed (true) or live (false) occurrences.
	Dismissed *bool
	// RecordID returns the occurrence a given record settled.
	RecordID *string
	// PaidDate filters by payment date; up to two range conditions.
	PaidDate []Filter
}

// Validate checks every documented constraint on the parameters.
func (p *ListStandingOrderItemsParams) Validate() error {
	if p == nil {
		return nil
	}
	var probs problems
	probs.addErr(validateLimit(p.Limit))
	probs.addErr(validateOffset(p.Offset))
	probs.addErr(validateRangeFilters("originalDate", p.OriginalDate))
	probs.addErr(validateRangeFilters("paidDate", p.PaidDate))
	return probs.err()
}

func (p *ListStandingOrderItemsParams) query() url.Values {
	q := newQuery()
	if p == nil {
		return q.values
	}
	q.setInt("limit", p.Limit)
	q.setInt("offset", p.Offset)
	q.setBool("agentHints", p.AgentHints)
	q.setBool("withTotal", p.WithTotal)
	q.setString("standingOrderId", p.StandingOrderID)
	q.setFilters("originalDate", p.OriginalDate)
	q.setBool("dismissed", p.Dismissed)
	q.setString("recordId", p.RecordID)
	q.setFilters("paidDate", p.PaidDate)
	return q.values
}

// ListStandingOrderItems returns one page of standing order occurrences,
// newest first by default. Each item is one period of a standing order and
// tracks its paid, unpaid or dismissed status.
//
// Operation: getStandingOrderItems. GET /v1/api/standing-orders/items.
func (c *Client) ListStandingOrderItems(ctx context.Context, params *ListStandingOrderItemsParams) (*StandingOrderItemsPage, error) {
	if err := c.validate(params); err != nil {
		return nil, err
	}
	out := &StandingOrderItemsPage{}
	err := c.do(ctx, request{
		op:     "getStandingOrderItems",
		method: http.MethodGet,
		path:   "/standing-orders/items",
		query:  params.query(),
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Monitoring
// ---------------------------------------------------------------------------

// APIUsageParams are the query parameters of GET /v1/api/api-usage/stats.
type APIUsageParams struct {
	// Period is required: Xdays (max 366, daily buckets), Xweeks (max 53, ISO
	// weeks starting Monday) or Xmonths (max 13).
	Period string
}

// Validate checks the period format and its per-unit ceiling.
func (p *APIUsageParams) Validate() error {
	if p == nil {
		return errorf("period", "is required")
	}
	return validateUsagePeriod("period", p.Period)
}

func (p *APIUsageParams) query() url.Values {
	q := newQuery()
	if p == nil {
		return q.values
	}
	q.setRaw("period", p.Period)
	return q.values
}

// APIUsage returns REST API usage statistics for the authenticated client.
// Every successful request is counted except calls to this endpoint itself.
// Bucket ranges are half-open: From is inclusive, To exclusive.
//
// Operation: getApiUsageStats. GET /v1/api/api-usage/stats.
func (c *Client) APIUsage(ctx context.Context, params *APIUsageParams) (*APIUsageStats, error) {
	if err := c.validate(params); err != nil {
		return nil, err
	}
	out := &APIUsageStats{}
	err := c.do(ctx, request{
		op:     "getApiUsageStats",
		method: http.MethodGet,
		path:   "/api-usage/stats",
		query:  params.query(),
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
