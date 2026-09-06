package wallet

import "context"

// maxAutoPages bounds an auto-pagination walk, so a server that keeps
// answering with a nextOffset cannot spin the caller forever.
const maxAutoPages = 10_000

// walk drives an offset-paginated endpoint until the pages run out, calling
// fetch for each page and yield for each item. Both fetch and yield may stop
// the walk: fetch by returning an error, yield by returning false.
//
// It advances strictly, so a server that repeats or rewinds nextOffset ends
// the walk rather than looping.
func walk[T any](
	ctx context.Context,
	startOffset int,
	fetch func(ctx context.Context, offset int) (Pagination, []T, error),
	yield func(T) bool,
) error {
	offset := startOffset
	for page := 0; page < maxAutoPages; page++ {
		if err := ctx.Err(); err != nil {
			return &Error{Kind: KindTransport, Message: "pagination cancelled", Err: err}
		}
		meta, items, err := fetch(ctx, offset)
		if err != nil {
			return err
		}
		for _, item := range items {
			if !yield(item) {
				return nil
			}
		}
		if meta.NextOffset == nil {
			return nil
		}
		next := *meta.NextOffset
		// A next offset that does not advance would loop forever. An empty
		// page with a next offset would too. Treat both as the end.
		if next <= offset || len(items) == 0 {
			return nil
		}
		offset = next
	}
	return nil
}

// copyIntPtr returns a fresh pointer, so mutating a caller's params struct
// during a walk cannot corrupt the walk's own offset.
func copyIntPtr(v int) *int { return &v }

// AllAccounts walks every page of ListAccounts, calling yield for each
// account. Returning false from yield stops the walk. The Offset field of
// params seeds the first page; every other field is reused unchanged.
func (c *Client) AllAccounts(ctx context.Context, params *ListAccountsParams, yield func(Account) bool) error {
	local := ListAccountsParams{}
	if params != nil {
		local = *params
	}
	start := 0
	if local.Offset != nil {
		start = *local.Offset
	}
	return walk(ctx, start, func(ctx context.Context, offset int) (Pagination, []Account, error) {
		local.Offset = copyIntPtr(offset)
		page, err := c.ListAccounts(ctx, &local)
		if err != nil {
			return Pagination{}, nil, err
		}
		return page.Pagination, page.Accounts, nil
	}, yield)
}

// AllRecords walks every page of ListRecords, calling yield for each record.
//
// Remember the implicit window: without a RecordDate bound the server scopes
// the walk to the last three months. Pass one bound to cover more.
func (c *Client) AllRecords(ctx context.Context, params *ListRecordsParams, yield func(Record) bool) error {
	local := ListRecordsParams{}
	if params != nil {
		local = *params
	}
	start := 0
	if local.Offset != nil {
		start = *local.Offset
	}
	return walk(ctx, start, func(ctx context.Context, offset int) (Pagination, []Record, error) {
		local.Offset = copyIntPtr(offset)
		page, err := c.ListRecords(ctx, &local)
		if err != nil {
			return Pagination{}, nil, err
		}
		return page.Pagination, page.Records, nil
	}, yield)
}

// AllBudgets walks every page of ListBudgets.
func (c *Client) AllBudgets(ctx context.Context, params *ListBudgetsParams, yield func(Budget) bool) error {
	local := ListBudgetsParams{}
	if params != nil {
		local = *params
	}
	start := 0
	if local.Offset != nil {
		start = *local.Offset
	}
	return walk(ctx, start, func(ctx context.Context, offset int) (Pagination, []Budget, error) {
		local.Offset = copyIntPtr(offset)
		page, err := c.ListBudgets(ctx, &local)
		if err != nil {
			return Pagination{}, nil, err
		}
		return page.Pagination, page.Budgets, nil
	}, yield)
}

// AllCategories walks every page of ListCategories.
func (c *Client) AllCategories(ctx context.Context, params *ListCategoriesParams, yield func(Category) bool) error {
	local := ListCategoriesParams{}
	if params != nil {
		local = *params
	}
	start := 0
	if local.Offset != nil {
		start = *local.Offset
	}
	return walk(ctx, start, func(ctx context.Context, offset int) (Pagination, []Category, error) {
		local.Offset = copyIntPtr(offset)
		page, err := c.ListCategories(ctx, &local)
		if err != nil {
			return Pagination{}, nil, err
		}
		return page.Pagination, page.Categories, nil
	}, yield)
}

// AllLabels walks every page of ListLabels.
func (c *Client) AllLabels(ctx context.Context, params *ListLabelsParams, yield func(Label) bool) error {
	local := ListLabelsParams{}
	if params != nil {
		local = *params
	}
	start := 0
	if local.Offset != nil {
		start = *local.Offset
	}
	return walk(ctx, start, func(ctx context.Context, offset int) (Pagination, []Label, error) {
		local.Offset = copyIntPtr(offset)
		page, err := c.ListLabels(ctx, &local)
		if err != nil {
			return Pagination{}, nil, err
		}
		return page.Pagination, page.Labels, nil
	}, yield)
}

// AllGoals walks every page of ListGoals.
func (c *Client) AllGoals(ctx context.Context, params *ListGoalsParams, yield func(Goal) bool) error {
	local := ListGoalsParams{}
	if params != nil {
		local = *params
	}
	start := 0
	if local.Offset != nil {
		start = *local.Offset
	}
	return walk(ctx, start, func(ctx context.Context, offset int) (Pagination, []Goal, error) {
		local.Offset = copyIntPtr(offset)
		page, err := c.ListGoals(ctx, &local)
		if err != nil {
			return Pagination{}, nil, err
		}
		return page.Pagination, page.Goals, nil
	}, yield)
}

// AllRecordRules walks every page of ListRecordRules.
func (c *Client) AllRecordRules(ctx context.Context, params *ListRecordRulesParams, yield func(RecordRule) bool) error {
	local := ListRecordRulesParams{}
	if params != nil {
		local = *params
	}
	start := 0
	if local.Offset != nil {
		start = *local.Offset
	}
	return walk(ctx, start, func(ctx context.Context, offset int) (Pagination, []RecordRule, error) {
		local.Offset = copyIntPtr(offset)
		page, err := c.ListRecordRules(ctx, &local)
		if err != nil {
			return Pagination{}, nil, err
		}
		return page.Pagination, page.RecordRules, nil
	}, yield)
}

// AllStandingOrders walks every page of ListStandingOrders.
func (c *Client) AllStandingOrders(ctx context.Context, params *ListStandingOrdersParams, yield func(StandingOrder) bool) error {
	local := ListStandingOrdersParams{}
	if params != nil {
		local = *params
	}
	start := 0
	if local.Offset != nil {
		start = *local.Offset
	}
	return walk(ctx, start, func(ctx context.Context, offset int) (Pagination, []StandingOrder, error) {
		local.Offset = copyIntPtr(offset)
		page, err := c.ListStandingOrders(ctx, &local)
		if err != nil {
			return Pagination{}, nil, err
		}
		return page.Pagination, page.StandingOrders, nil
	}, yield)
}

// AllStandingOrderItems walks every page of ListStandingOrderItems.
func (c *Client) AllStandingOrderItems(ctx context.Context, params *ListStandingOrderItemsParams, yield func(StandingOrderItem) bool) error {
	local := ListStandingOrderItemsParams{}
	if params != nil {
		local = *params
	}
	start := 0
	if local.Offset != nil {
		start = *local.Offset
	}
	return walk(ctx, start, func(ctx context.Context, offset int) (Pagination, []StandingOrderItem, error) {
		local.Offset = copyIntPtr(offset)
		page, err := c.ListStandingOrderItems(ctx, &local)
		if err != nil {
			return Pagination{}, nil, err
		}
		return page.Pagination, page.StandingOrderItems, nil
	}, yield)
}

// CollectRecords walks ListRecords and returns every record, stopping at
// limit when limit is positive. It is the eager form of AllRecords, for the
// common case where the result set is known to be small.
func (c *Client) CollectRecords(ctx context.Context, params *ListRecordsParams, limit int) ([]Record, error) {
	var out []Record
	err := c.AllRecords(ctx, params, func(r Record) bool {
		out = append(out, r)
		return limit <= 0 || len(out) < limit
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CollectAccounts walks ListAccounts and returns every account, stopping at
// limit when limit is positive.
func (c *Client) CollectAccounts(ctx context.Context, params *ListAccountsParams, limit int) ([]Account, error) {
	var out []Account
	err := c.AllAccounts(ctx, params, func(a Account) bool {
		out = append(out, a)
		return limit <= 0 || len(out) < limit
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
