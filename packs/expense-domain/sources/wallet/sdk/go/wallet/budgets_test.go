package wallet

import (
	"strings"
	"testing"
	"time"
)

func TestListBudgetsSerialisesEveryParameter(t *testing.T) {
	m := newMockServer(t, mockResponse{JSONBody: map[string]any{"budgets": []any{}}})
	c := newTestClient(t, m)
	budgetType := BudgetIntervalMonth
	if _, err := c.ListBudgets(testCtx(t), &ListBudgetsParams{
		Limit:        Int(10),
		Offset:       Int(5),
		AgentHints:   Bool(true),
		WithTotal:    Bool(false),
		IDs:          []string{"b1"},
		Name:         ptrFilter(Contains("Food")),
		CurrencyCode: String("USD"),
		Closed:       Bool(false),
		Type:         &budgetType,
		LabelID:      String("l1"),
		AccountID:    String("acc1"),
		CategoryIDs:  []string{"c1", "c2"},
		StartDate:    []Filter{Gte("2024-01-01")},
		EndDate:      []Filter{Lt("2024-12-31")},
		CreatedAt:    []Filter{Gte("2024-01-01T00:00:00Z")},
		UpdatedAt:    []Filter{Lte("2024-06-01T00:00:00Z")},
		Spending:     String("current+3"),
	}); err != nil {
		t.Fatalf("ListBudgets: %v", err)
	}
	q := m.Last(t).Query
	checks := map[string]string{
		"limit": "10", "offset": "5", "agentHints": "true", "withTotal": "false",
		"id": "b1", "name": "contains.Food", "currencyCode": "USD", "closed": "false",
		"type": "BUDGET_INTERVAL_MONTH", "labelId": "l1", "accountId": "acc1",
		"categoryId": "c1,c2", "startDate": "gte.2024-01-01", "endDate": "lt.2024-12-31",
		"spending": "current+3",
	}
	for key, want := range checks {
		if got := q.Get(key); got != want {
			t.Errorf("query %q = %q, want %q", key, got, want)
		}
	}
}

func TestListBudgetsDecodesSpendingBlock(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{
		"budgets":[{
			"id":"b1","name":"Monthly Food","type":"BUDGET_INTERVAL_MONTH","currencyCode":"USD",
			"limit":300,"closed":false,
			"startDate":"2026-01-01","endDate":"2027-01-01",
			"accountIds":["acc1"],"categoryIds":["c1"],"labelIds":[],
			"limitOverrides":[{"limit":400,"period":"2026-03","periodStart":"2026-03-01","periodCount":2,
			                   "createdAt":"2026-02-01T00:00:00Z"}],
			"pastLimitOverrides":[],
			"spending":{
				"computedAt":"2026-09-06T00:00:00Z",
				"current":{"period":"2026-09","effectiveLimit":300,"spent":120.5,"remaining":179.5,
				           "progress":0.4017,"overspent":null,"incomplete":true,"recordCount":8,
				           "periodStart":"2026-09-01","periodEnd":"2026-09-30",
				           "totalExpenses":120.5,"totalIncomes":0,
				           "convertedCurrencies":["EUR"],
				           "excluded":{"total":3,"transfers":1,"debts":0,"incomeCategories":1,
				                       "unknownCategories":1,"totalAmountSum":-45.2}},
				"past":[{"period":"2026-08","effectiveLimit":300,"spent":310,"overspent":10,"remaining":null}]
			}
		}]
	}`})
	c := newTestClient(t, m)
	page, err := c.ListBudgets(testCtx(t), nil)
	if err != nil {
		t.Fatalf("ListBudgets: %v", err)
	}
	b := page.Budgets[0]
	if b.Limit == nil || *b.Limit != 300 {
		t.Errorf("limit = %v, want 300", b.Limit)
	}
	if b.Closed == nil || *b.Closed {
		t.Errorf("closed = %v, want false", b.Closed)
	}
	if got := b.StartDate.String(); got != "2026-01-01" {
		t.Errorf("startDate = %q, want 2026-01-01", got)
	}
	if len(b.LimitOverrides) != 1 || b.LimitOverrides[0].PeriodCount != 2 {
		t.Errorf("limitOverrides = %+v", b.LimitOverrides)
	}
	if b.Spending == nil || b.Spending.Current == nil {
		t.Fatalf("spending = %+v", b.Spending)
	}
	cur := b.Spending.Current
	if cur.Spent == nil || *cur.Spent != 120.5 {
		t.Errorf("spent = %v, want 120.5", cur.Spent)
	}
	if cur.Overspent != nil {
		t.Errorf("overspent = %v, want nil for a period under budget", cur.Overspent)
	}
	if !cur.Incomplete {
		t.Error("the current period should be marked incomplete")
	}
	if cur.Excluded == nil || cur.Excluded.Total != 3 || cur.Excluded.TotalAmountSum == nil {
		t.Errorf("excluded = %+v", cur.Excluded)
	}
	if len(b.Spending.Past) != 1 || b.Spending.Past[0].Remaining != nil {
		t.Errorf("past = %+v, want one overspent period with a null remaining", b.Spending.Past)
	}
}

func TestBudgetClosedDateOnlyPresentWhenClosed(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{"budgets":[
		{"id":"b1","closed":true,"closedDate":"2025-06-15"},
		{"id":"b2","closed":false}
	]}`})
	c := newTestClient(t, m)
	page, err := c.ListBudgets(testCtx(t), nil)
	if err != nil {
		t.Fatalf("ListBudgets: %v", err)
	}
	if got := page.Budgets[0].ClosedDate.String(); got != "2025-06-15" {
		t.Errorf("closedDate = %q, want 2025-06-15", got)
	}
	if page.Budgets[1].ClosedDate.Valid() {
		t.Errorf("an open budget should have no closedDate, got %v", page.Budgets[1].ClosedDate)
	}
}

func TestBudgetAllIsReadableButNotCreatable(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{"budgets":[{"id":"b1","type":"BUDGET_ALL"}]}`})
	c := newTestClient(t, m)
	page, err := c.ListBudgets(testCtx(t), nil)
	if err != nil {
		t.Fatalf("ListBudgets: %v", err)
	}
	if page.Budgets[0].Type != BudgetAll {
		t.Errorf("type = %q, want BUDGET_ALL", page.Budgets[0].Type)
	}
	if !BudgetAll.Valid() || BudgetAll.Creatable() {
		t.Error("BUDGET_ALL should be valid to read but not creatable")
	}
	body := &CreateBudgetRequest{Name: "n", CurrencyCode: "USD", Type: BudgetAll, Limit: 1}
	errs := mustInvalidRequest(t, body.Validate())
	wantField(t, errs, "type")
}

func TestListBudgetsValidation(t *testing.T) {
	tests := []struct {
		name   string
		params *ListBudgetsParams
		field  string
	}{
		{"bad spending", &ListBudgetsParams{Spending: String("current+0")}, "spending"},
		{"bad spending word", &ListBudgetsParams{Spending: String("all")}, "spending"},
		{"bad type", &ListBudgetsParams{Type: ptrBudgetType("BUDGET_DAILY")}, "type"},
		{"too many categories", &ListBudgetsParams{CategoryIDs: makeIDs(11)}, "categoryId"},
		{"three startDate bounds", &ListBudgetsParams{StartDate: []Filter{Gte("a"), Lt("b"), Eq("c")}}, "startDate"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := mustInvalidRequest(t, tc.params.Validate())
			wantField(t, errs, tc.field)
		})
	}
}

func TestListBudgetsAcceptsEverySpendingDepth(t *testing.T) {
	for _, v := range []string{"none", "current", "current+1", "current+12", ""} {
		p := &ListBudgetsParams{Spending: String(v)}
		if err := p.Validate(); err != nil {
			t.Errorf("spending=%q should be accepted: %v", v, err)
		}
	}
}

func TestCreateBudgetHappyPath(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 201, Body: `{
		"budget":{"id":"b1","name":"Monthly Food","type":"BUDGET_INTERVAL_MONTH","limit":300},
		"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":1}
	}`})
	c := newTestClient(t, m)
	resp, err := c.CreateBudget(testCtx(t), &CreateBudgetRequest{
		Name:         "Monthly Food",
		CurrencyCode: "USD",
		Type:         BudgetIntervalMonth,
		Limit:        300,
		CategoryIDs:  []string{"550e8400-e29b-41d4-a716-446655440000"},
	})
	if err != nil {
		t.Fatalf("CreateBudget: %v", err)
	}
	if resp.Budget == nil || resp.Budget.ID != "b1" {
		t.Fatalf("budget = %+v", resp.Budget)
	}
	if !strings.Contains(string(m.Last(t).Body), `"BUDGET_INTERVAL_MONTH"`) {
		t.Errorf("body = %s", m.Last(t).Body)
	}
}

func TestCreateCustomBudgetRequiresBothDates(t *testing.T) {
	start := NewDate(2026, time.January, 1)
	end := NewDate(2026, time.June, 30)

	missingBoth := &CreateBudgetRequest{Name: "n", CurrencyCode: "USD", Type: BudgetCustom, Limit: 100}
	errs := mustInvalidRequest(t, missingBoth.Validate())
	wantField(t, errs, "startDate")
	wantField(t, errs, "endDate")

	missingEnd := &CreateBudgetRequest{Name: "n", CurrencyCode: "USD", Type: BudgetCustom, Limit: 100, StartDate: &start}
	errs = mustInvalidRequest(t, missingEnd.Validate())
	wantField(t, errs, "endDate")

	complete := &CreateBudgetRequest{Name: "n", CurrencyCode: "USD", Type: BudgetCustom, Limit: 100,
		StartDate: &start, EndDate: &end}
	if err := complete.Validate(); err != nil {
		t.Errorf("a complete custom budget should validate: %v", err)
	}

	reversed := &CreateBudgetRequest{Name: "n", CurrencyCode: "USD", Type: BudgetCustom, Limit: 100,
		StartDate: &end, EndDate: &start}
	errs = mustInvalidRequest(t, reversed.Validate())
	wantField(t, errs, "endDate")
}

func TestCreateBudgetValidation(t *testing.T) {
	tests := []struct {
		name  string
		body  *CreateBudgetRequest
		field string
	}{
		{"nil", nil, ""},
		{"missing name", &CreateBudgetRequest{CurrencyCode: "USD", Type: BudgetIntervalWeek, Limit: 1}, "name"},
		{"missing currency", &CreateBudgetRequest{Name: "n", Type: BudgetIntervalWeek, Limit: 1}, "currencyCode"},
		{"missing type", &CreateBudgetRequest{Name: "n", CurrencyCode: "USD", Limit: 1}, "type"},
		{"zero limit", &CreateBudgetRequest{Name: "n", CurrencyCode: "USD", Type: BudgetIntervalWeek, Limit: 0}, "limit"},
		{"negative limit", &CreateBudgetRequest{Name: "n", CurrencyCode: "USD", Type: BudgetIntervalWeek, Limit: -5}, "limit"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := mustInvalidRequest(t, tc.body.Validate())
			if tc.field != "" {
				wantField(t, errs, tc.field)
			}
		})
	}
}

func TestLimitOverrideValidation(t *testing.T) {
	tests := []struct {
		name  string
		over  *LimitOverrideInput
		field string
	}{
		{"missing period", &LimitOverrideInput{Limit: Float(100)}, "period"},
		{"bad period", &LimitOverrideInput{Period: "2026/03", Limit: Float(100)}, "period"},
		{"bad week", &LimitOverrideInput{Period: "2026-W99", Limit: Float(100)}, "period"},
		{"neither limit nor baseline", &LimitOverrideInput{Period: "2026-03"}, "limit"},
		{"both limit and baseline", &LimitOverrideInput{Period: "2026-03", Limit: Float(100), SetBaseline: Bool(true)}, "limit"},
		{"zero limit", &LimitOverrideInput{Period: "2026-03", Limit: Float(0)}, "limit"},
		{"periodCount with baseline", &LimitOverrideInput{Period: "2026-03", SetBaseline: Bool(true), PeriodCount: Int(2)}, "periodCount"},
		{"periodCount without limit", &LimitOverrideInput{Period: "2026-03", SetBaseline: Bool(true), PeriodCount: Int(2)}, "periodCount"},
		{"periodCount too low", &LimitOverrideInput{Period: "2026-03", Limit: Float(1), PeriodCount: Int(0)}, "periodCount"},
		{"periodCount too high", &LimitOverrideInput{Period: "2026-03", Limit: Float(1), PeriodCount: Int(121)}, "periodCount"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := mustInvalidRequest(t, tc.over.Validate())
			wantField(t, errs, tc.field)
		})
	}
}

func TestLimitOverrideAcceptsEveryPeriodShape(t *testing.T) {
	for _, period := range []string{"2026", "2026-03", "2026-W01", "2026-W53"} {
		over := &LimitOverrideInput{Period: period, Limit: Float(100)}
		if err := over.Validate(); err != nil {
			t.Errorf("period %q should be accepted: %v", period, err)
		}
	}
	for _, period := range []string{"26-03", "2026-13", "2026-W0", "2026-W54", "202603", "2026-Www"} {
		over := &LimitOverrideInput{Period: period, Limit: Float(100)}
		if err := over.Validate(); err == nil {
			t.Errorf("period %q should be rejected", period)
		}
	}
}

func TestPatchBudgetsValidation(t *testing.T) {
	tooMany := make([]LimitOverrideInput, 51)
	for i := range tooMany {
		tooMany[i] = LimitOverrideInput{Period: "2026-03", Limit: Float(1)}
	}
	tests := []struct {
		name  string
		items []PatchBudgetRequest
		field string
	}{
		{"empty batch", nil, "items"},
		{"missing id", []PatchBudgetRequest{{Name: String("x")}}, "items[0].id"},
		{"nothing to do", []PatchBudgetRequest{{ID: "b1"}}, "items[0]"},
		{"too many overrides", []PatchBudgetRequest{{ID: "b1", LimitOverrides: tooMany}}, "items[0].limitOverrides"},
		{"zero resetLimit", []PatchBudgetRequest{{ID: "b1", ResetLimit: Float(0)}}, "items[0].resetLimit"},
		{"empty category delta", []PatchBudgetRequest{{ID: "b1", CategoryIDs: AddRemoveIDs(nil, nil)}}, "items[0].categoryIds"},
		{"bad override", []PatchBudgetRequest{{ID: "b1", LimitOverrides: []LimitOverrideInput{{Period: "bad", Limit: Float(1)}}}},
			"items[0].limitOverrides[0].period"},
		{"duplicate override period", []PatchBudgetRequest{{ID: "b1", LimitOverrides: []LimitOverrideInput{
			{Period: "2026-03", Limit: Float(1)}, {Period: "2026-03", Limit: Float(2)}}}},
			"items[0].limitOverrides[1].period"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := mustInvalidRequest(t, patchBudgetsBody(tc.items).Validate())
			wantField(t, errs, tc.field)
		})
	}
}

func TestPatchBudgetSendsBothSetShapes(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 200, Body: `{"results":[{"inputIndex":0,"id":"b1","success":true}],
		"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":1}}`})
	c := newTestClient(t, m)
	if _, err := c.PatchBudgets(testCtx(t), []PatchBudgetRequest{{
		ID:          "b1",
		CategoryIDs: ReplaceIDs([]string{"c1", "c2"}),
		AccountIDs:  AddRemoveIDs([]string{"acc1"}, []string{"acc2"}),
		LabelIDs:    ClearIDs(),
		Closed:      Bool(true),
		ResetLimit:  Float(500),
		LimitOverrides: []LimitOverrideInput{
			{Period: "2026-10", Limit: Float(700), PeriodCount: Int(2)},
			{Period: "2026-12", SetBaseline: Bool(true)},
		},
	}}, nil); err != nil {
		t.Fatalf("PatchBudgets: %v", err)
	}
	body := string(m.Last(t).Body)
	for _, want := range []string{
		`"categoryIds":["c1","c2"]`,
		`"accountIds":{"add":["acc1"],"remove":["acc2"]}`,
		`"labelIds":[]`,
		`"closed":true`,
		`"resetLimit":500`,
		`"period":"2026-10"`,
		`"periodCount":2`,
		`"setBaseline":true`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %s\ngot: %s", want, body)
		}
	}
}

func ptrBudgetType(b BudgetType) *BudgetType { return &b }
