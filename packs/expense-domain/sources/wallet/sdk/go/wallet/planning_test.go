package wallet

import (
	"net/http"
	"testing"
	"time"
)

func TestListGoals(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{"goals":[{
		"id":"g1","name":"New laptop","note":"for work","color":"#673ab7",
		"state":"active","stateUpdatedAt":"2025-06-15T10:30:00.000Z",
		"desiredDate":"2025-12-31T00:00:00.000Z",
		"initialAmount":{"currencyCode":"USD","value":500},
		"targetAmount":{"currencyCode":"USD","value":10000},
		"createdAt":"2024-01-01T00:00:00Z"}]}`})
	c := newTestClient(t, m)
	page, err := c.ListGoals(testCtx(t), &ListGoalsParams{
		Note: ptrFilter(ContainsI("work")), Name: ptrFilter(Contains("laptop")), Limit: Int(10),
	})
	if err != nil {
		t.Fatalf("ListGoals: %v", err)
	}
	g := page.Goals[0]
	if g.State != GoalActive {
		t.Errorf("state = %q, want active", g.State)
	}
	if g.TargetAmount == nil || g.TargetAmount.Value != 10000 {
		t.Errorf("targetAmount = %+v", g.TargetAmount)
	}
	if g.InitialAmount == nil || g.InitialAmount.CurrencyCode != "USD" {
		t.Errorf("initialAmount = %+v", g.InitialAmount)
	}
	if !g.DesiredDate.Valid() {
		t.Errorf("desiredDate = %v", g.DesiredDate)
	}
	q := m.Last(t).Query
	if q.Get("note") != "contains-i.work" || q.Get("name") != "contains.laptop" {
		t.Errorf("query = %v", q)
	}
}

func TestListGoalsValidation(t *testing.T) {
	errs := mustInvalidRequest(t, (&ListGoalsParams{Note: ptrFilter(Gte("x"))}).Validate())
	wantField(t, errs, "note")
	errs = mustInvalidRequest(t, (&ListGoalsParams{Limit: Int(500)}).Validate())
	wantField(t, errs, "limit")
}

func TestAllGoalStatesDecode(t *testing.T) {
	for _, state := range AllGoalStates {
		state := state
		t.Run(string(state), func(t *testing.T) {
			m := newMockServer(t, mockResponse{Body: `{"goals":[{"id":"g1","state":"` + string(state) + `"}]}`})
			c := newTestClient(t, m)
			page, err := c.ListGoals(testCtx(t), nil)
			if err != nil {
				t.Fatalf("ListGoals: %v", err)
			}
			if page.Goals[0].State != state {
				t.Errorf("state = %q, want %q", page.Goals[0].State, state)
			}
		})
	}
}

func TestListStandingOrders(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{"standingOrders":[{
		"id":"so1","name":"Rent","accountId":"acc1","amount":1234.56,"currencyCode":"USD",
		"categoryId":"c1","counterParty":"Landlord","note":"monthly",
		"dueDate":"2025-12-01T00:00:00Z","dueDateNotificationEnabled":true,
		"threeDaysBeforeNotificationEnabled":true,"reminder":"3_days",
		"generateFromDate":"2025-01-01","manualPayment":false,
		"paymentCount":12,"recurrenceRule":"FREQ=MONTHLY;BYMONTHDAY=1",
		"type":"expense","labels":[{"id":"l1","name":"housing"}],
		"createdAt":"2024-01-01T00:00:00Z"}]}`})
	c := newTestClient(t, m)
	page, err := c.ListStandingOrders(testCtx(t), &ListStandingOrdersParams{
		CurrencyCode: String("usd"), LabelID: String("l1"),
	})
	if err != nil {
		t.Fatalf("ListStandingOrders: %v", err)
	}
	so := page.StandingOrders[0]
	if so.Reminder != Reminder3Days || so.Type != RecordTypeExpense {
		t.Errorf("standing order enums = %q/%q", so.Reminder, so.Type)
	}
	if so.Amount != 1234.56 || so.PaymentCount != 12 {
		t.Errorf("standing order = %+v", so)
	}
	if so.RecurrenceRule == "" || len(so.Labels) != 1 {
		t.Errorf("standing order rule/labels = %q/%v", so.RecurrenceRule, so.Labels)
	}
	if !so.DueDateNotificationEnabled || !so.ThreeDaysBeforeNotificationEnabled {
		t.Error("both notification flags should decode as true")
	}
}

func TestStandingOrderCurrencyValidation(t *testing.T) {
	errs := mustInvalidRequest(t, (&ListStandingOrdersParams{CurrencyCode: String("DOLLARS")}).Validate())
	wantField(t, errs, "currencyCode")
}

func TestListStandingOrderItems(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{"standingOrderItems":[
		{"id":"i1","standingOrderId":"so1","originalDate":"2025-01-01T00:00:00Z",
		 "alignedDate":"2025-01-03T00:00:00Z","paidDate":"2025-01-03T00:00:00Z",
		 "paidFromAppDate":"2025-01-03T00:00:00Z","dismissed":false,"recordIds":["r1","r2"]},
		{"id":"i2","standingOrderId":"so1","originalDate":"2025-02-01T00:00:00Z","dismissed":true}
	]}`})
	c := newTestClient(t, m)
	page, err := c.ListStandingOrderItems(testCtx(t), &ListStandingOrderItemsParams{
		StandingOrderID: String("so1"),
		OriginalDate:    []Filter{Gte("2025-01-01T00:00:00Z"), Lt("2025-03-01T00:00:00Z")},
		PaidDate:        []Filter{Gte("2025-01-01T00:00:00Z")},
		Dismissed:       Bool(false),
		RecordID:        String("r1"),
	})
	if err != nil {
		t.Fatalf("ListStandingOrderItems: %v", err)
	}
	paid, unpaid := page.StandingOrderItems[0], page.StandingOrderItems[1]
	if !paid.Paid() || len(paid.RecordIDs) != 2 {
		t.Errorf("item 0 = %+v, want paid with two records", paid)
	}
	if unpaid.Paid() || !unpaid.Dismissed {
		t.Errorf("item 1 = %+v, want dismissed and unpaid", unpaid)
	}
	q := m.Last(t).Query
	if got := q["originalDate"]; len(got) != 2 {
		t.Errorf("originalDate = %v, want two bounds", got)
	}
	if q.Get("standingOrderId") != "so1" || q.Get("dismissed") != "false" || q.Get("recordId") != "r1" {
		t.Errorf("query = %v", q)
	}
	if got, want := m.Last(t).Path, "/v1/api/standing-orders/items"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

func TestStandingOrderItemsRangeValidation(t *testing.T) {
	errs := mustInvalidRequest(t, (&ListStandingOrderItemsParams{
		PaidDate: []Filter{Gte("a"), Lt("b"), Eq("c")},
	}).Validate())
	wantField(t, errs, "paidDate")
}

// ---------------------------------------------------------------------------
// API usage
// ---------------------------------------------------------------------------

func TestAPIUsageHappyPath(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{
		"period":"7days","granularity":"daily",
		"from":"2026-08-30T00:00:00Z","to":"2026-09-06T00:00:00Z","total":123,
		"usage":[{"from":"2026-08-30T00:00:00Z","to":"2026-08-31T00:00:00Z","total":50},
		         {"from":"2026-09-01T00:00:00Z","to":"2026-09-02T00:00:00Z","total":73}]
	}`})
	c := newTestClient(t, m)
	stats, err := c.APIUsage(testCtx(t), &APIUsageParams{Period: "7days"})
	if err != nil {
		t.Fatalf("APIUsage: %v", err)
	}
	if stats.Granularity != GranularityDaily || stats.Total != 123 {
		t.Errorf("stats = %+v", stats)
	}
	if len(stats.Usage) != 2 {
		t.Fatalf("usage buckets = %d, want 2 (zero-usage periods are omitted)", len(stats.Usage))
	}
	// The range is half-open, so the second bucket starts after a gap the
	// server left out.
	wantFrom := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if !stats.Usage[1].From.Time.Equal(wantFrom) {
		t.Errorf("usage[1].From = %v, want %v", stats.Usage[1].From, wantFrom)
	}
	if got := m.Last(t).Query.Get("period"); got != "7days" {
		t.Errorf("period = %q", got)
	}
	if got, want := m.Last(t).Path, "/v1/api/api-usage/stats"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

func TestAPIUsagePeriodValidation(t *testing.T) {
	valid := []string{"1days", "366days", "1weeks", "53weeks", "1months", "13months"}
	for _, p := range valid {
		if err := (&APIUsageParams{Period: p}).Validate(); err != nil {
			t.Errorf("period %q should be accepted: %v", p, err)
		}
	}
	invalid := []string{"", "367days", "54weeks", "14months", "0days", "-5days", "30", "thirtydays", "30day", "1years"}
	for _, p := range invalid {
		if err := (&APIUsageParams{Period: p}).Validate(); err == nil {
			t.Errorf("period %q should be rejected", p)
		}
	}
	errs := mustInvalidRequest(t, (*APIUsageParams)(nil).Validate())
	wantField(t, errs, "period")
}

func TestAPIUsageRejectsBadPeriodBeforeSending(t *testing.T) {
	m := newMockServer(t, mockResponse{JSONBody: map[string]any{}})
	c := newTestClient(t, m)
	_, err := c.APIUsage(testCtx(t), &APIUsageParams{Period: "400days"})
	errs := mustInvalidRequest(t, err)
	wantField(t, errs, "period")
	if m.Count() != 0 {
		t.Errorf("nothing should have been sent, but %d requests arrived", m.Count())
	}
}

func TestEveryReadEndpointSurfacesA401(t *testing.T) {
	// One sweep proving the error path is wired identically everywhere.
	calls := map[string]func(*Client) error{
		"accounts": func(c *Client) error { _, err := c.ListAccounts(testCtx(t), nil); return err },
		"budgets":  func(c *Client) error { _, err := c.ListBudgets(testCtx(t), nil); return err },
		"categories": func(c *Client) error {
			_, err := c.ListCategories(testCtx(t), nil)
			return err
		},
		"goals":   func(c *Client) error { _, err := c.ListGoals(testCtx(t), nil); return err },
		"labels":  func(c *Client) error { _, err := c.ListLabels(testCtx(t), nil); return err },
		"records": func(c *Client) error { _, err := c.ListRecords(testCtx(t), nil); return err },
		"recordRules": func(c *Client) error {
			_, err := c.ListRecordRules(testCtx(t), nil)
			return err
		},
		"standingOrders": func(c *Client) error {
			_, err := c.ListStandingOrders(testCtx(t), nil)
			return err
		},
		"standingOrderItems": func(c *Client) error {
			_, err := c.ListStandingOrderItems(testCtx(t), nil)
			return err
		},
		"apiUsage": func(c *Client) error {
			_, err := c.APIUsage(testCtx(t), &APIUsageParams{Period: "7days"})
			return err
		},
		"references": func(c *Client) error {
			_, err := c.References(testCtx(t), &ReferencesParams{Type: EntityRecords, IDs: []string{"r1"}})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			m := newMockServer(t, mockResponse{Status: http.StatusUnauthorized, Body: `{"error":"Invalid or expired token"}`})
			c := newTestClient(t, m)
			mustAPIError(t, call(c), KindUnauthorized)
		})
	}
}

func TestEveryWriteEndpointSurfacesA403(t *testing.T) {
	forbidden := "Forbidden — the token belongs to the other app."
	calls := map[string]func(*Client) error{
		"createAccount": func(c *Client) error {
			_, err := c.CreateAccount(testCtx(t), &CreateAccountRequest{Name: "n", AccountType: AccountTypeCash, CurrencyCode: "USD"})
			return err
		},
		"patchAccounts": func(c *Client) error {
			_, err := c.PatchAccounts(testCtx(t), []PatchAccountItem{{ID: "a1", Archived: Bool(true)}}, nil)
			return err
		},
		"createBudget": func(c *Client) error {
			_, err := c.CreateBudget(testCtx(t), &CreateBudgetRequest{Name: "n", CurrencyCode: "USD", Type: BudgetIntervalMonth, Limit: 1})
			return err
		},
		"patchBudgets": func(c *Client) error {
			_, err := c.PatchBudgets(testCtx(t), []PatchBudgetRequest{{ID: "b1", Name: String("x")}}, nil)
			return err
		},
		"createCategory": func(c *Client) error {
			_, err := c.CreateSubcategory(testCtx(t), &CreateSubcategoryRequest{Name: "n", ParentID: "p"})
			return err
		},
		"patchCategories": func(c *Client) error {
			_, err := c.PatchCategories(testCtx(t), []PatchCategoryRequest{{ID: "c1", Name: String("x")}}, nil)
			return err
		},
		"createLabel": func(c *Client) error {
			_, err := c.CreateLabel(testCtx(t), &CreateLabelRequest{Name: "n"})
			return err
		},
		"patchLabels": func(c *Client) error {
			_, err := c.PatchLabels(testCtx(t), []PatchLabelItem{{ID: "l1", Name: String("x")}}, nil)
			return err
		},
		"createRecords": func(c *Client) error {
			_, err := c.CreateRecords(testCtx(t), []CreateRecordRequest{validRecord()}, nil)
			return err
		},
		"patchRecords": func(c *Client) error {
			_, err := c.PatchRecords(testCtx(t), []PatchRecordItem{{ID: "r1", Note: String("x")}}, nil)
			return err
		},
		"delete": func(c *Client) error {
			_, err := c.Delete(testCtx(t), EntityRecords, []string{"r1"})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			m := newMockServer(t, mockResponse{Status: http.StatusForbidden, Body: forbidden, ContentType: "text/plain"})
			c := newTestClient(t, m)
			apiErr := mustAPIError(t, call(c), KindForbidden)
			if apiErr.Message != forbidden {
				t.Errorf("Message = %q, want the text/plain body verbatim", apiErr.Message)
			}
		})
	}
}
