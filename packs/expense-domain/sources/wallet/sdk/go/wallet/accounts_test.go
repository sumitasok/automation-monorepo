package wallet

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestListAccountsSerialisesEveryParameter(t *testing.T) {
	m := newMockServer(t, mockResponse{JSONBody: map[string]any{"accounts": []any{}}})
	c := newTestClient(t, m)

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	accountType := AccountTypeCurrentAccount
	if _, err := c.ListAccounts(testCtx(t), &ListAccountsParams{
		Limit:        Int(50),
		Offset:       Int(30),
		AgentHints:   Bool(true),
		WithTotal:    Bool(true),
		IDs:          []string{"a1", "a2"},
		Name:         ptrFilter(ContainsI("check")),
		AccountType:  &accountType,
		CurrencyCode: String("EUR"),
		CreatedAt:    Between(from, to),
		UpdatedAt:    []Filter{Gte("2024-02-01T00:00:00Z")},
		Archived:     Bool(false),
		BudgetID:     String("b1"),
		SortBy:       ptrSort(Desc("createdAt")),
	}); err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}

	q := m.Last(t).Query
	checks := map[string]string{
		"limit":        "50",
		"offset":       "30",
		"agentHints":   "true",
		"withTotal":    "true",
		"id":           "a1,a2",
		"name":         "contains-i.check",
		"accountType":  "CurrentAccount",
		"currencyCode": "EUR",
		"archived":     "false",
		"budgetId":     "b1",
		"sortBy":       "-createdAt",
	}
	for key, want := range checks {
		if got := q.Get(key); got != want {
			t.Errorf("query %q = %q, want %q", key, got, want)
		}
	}
	// A range parameter sends one entry per condition, not a joined string.
	if got := q["createdAt"]; len(got) != 2 || got[0] != "gte.2024-01-01T00:00:00Z" || got[1] != "lt.2024-04-01T00:00:00Z" {
		t.Errorf("createdAt = %v, want the two half-open bounds", got)
	}
	if got := q["updatedAt"]; len(got) != 1 || got[0] != "gte.2024-02-01T00:00:00Z" {
		t.Errorf("updatedAt = %v, want one bound", got)
	}
	if got, want := m.Last(t).Path, "/v1/api/accounts"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

func TestListAccountsOmitsUnsetParameters(t *testing.T) {
	m := newMockServer(t, mockResponse{JSONBody: map[string]any{"accounts": []any{}}})
	c := newTestClient(t, m)
	if _, err := c.ListAccounts(testCtx(t), nil); err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if got := m.Last(t).Query; len(got) != 0 {
		t.Errorf("query = %v, want empty when no params are given", got)
	}
}

func TestListAccountsDecodesTheFullAccountShape(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{
		"limit": 30, "offset": 0, "nextOffset": 30, "total": 42,
		"accounts": [{
			"id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
			"name": "My Checking Account",
			"accountType": "CurrentAccount",
			"archived": false,
			"bankAccountNumber": "CZ65 0800 0000 1920 0014 5399",
			"color": "#d32f2f",
			"currencyCode": "USD",
			"excludeFromStats": false,
			"isBankSync": true,
			"isInvestmentAccount": false,
			"createdAt": "2021-03-24T09:28:41.200Z",
			"updatedAt": "2021-03-25T09:28:41.200Z",
			"balance": {
				"balanceMode": "creditCardBank",
				"balanceModeFormula": "creditBalance = -(initial + totalIncomes - totalExpenses)",
				"currencyCode": "USD",
				"currentBalance": 3343.75,
				"rawCurrentBalance": 3300.00,
				"creditLimit": 5000,
				"creditBalance": -1656.25,
				"availableCredit": 3343.75,
				"formula": "initial + totalIncomes - totalExpenses",
				"initial": 1000
			},
			"recordStats": {
				"recordCount": 42,
				"totalExpenses": 12890.75,
				"totalIncomes": 15234.5,
				"lastUpdatedAt": "2025-03-20T14:22:31.456Z",
				"recordDate": {"min": "2024-01-15T00:00:00.000Z", "max": "2025-03-20T00:00:00.000Z"},
				"createdAt": {"min": "2024-01-15T10:30:00.000Z", "max": "2025-03-20T14:22:00.000Z"}
			}
		}],
		"agentHints": [{
			"type": "pagination.has_more", "severity": "instruction",
			"text": "More results available",
			"action": {"url": "https://example.test/next"},
			"data": {"remaining": 12}
		}]
	}`})
	c := newTestClient(t, m)
	page, err := c.ListAccounts(testCtx(t), nil)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}

	if !page.HasMore() || *page.NextOffset != 30 {
		t.Errorf("HasMore/NextOffset = %v/%v, want true/30", page.HasMore(), page.NextOffset)
	}
	if page.Total == nil || *page.Total != 42 {
		t.Errorf("Total = %v, want 42", page.Total)
	}

	a := page.Accounts[0]
	if a.AccountType != AccountTypeCurrentAccount || !a.IsBankSync {
		t.Errorf("account = %+v, want a bank-synced current account", a)
	}
	if a.Balance == nil || a.Balance.BalanceMode != BalanceCreditCardBank {
		t.Fatalf("balance = %+v, want creditCardBank mode", a.Balance)
	}
	if a.Balance.CurrentBalance == nil || *a.Balance.CurrentBalance != 3343.75 {
		t.Errorf("currentBalance = %v, want 3343.75", a.Balance.CurrentBalance)
	}
	if a.Balance.Initial == nil || *a.Balance.Initial != 1000 {
		t.Errorf("initial = %v, want 1000 — the write-only initialBalance reads back here", a.Balance.Initial)
	}
	if a.RecordStats == nil || a.RecordStats.RecordCount != 42 {
		t.Fatalf("recordStats = %+v, want 42 records", a.RecordStats)
	}
	if a.RecordStats.RecordDate == nil || a.RecordStats.RecordDate.Min != "2024-01-15T00:00:00.000Z" {
		t.Errorf("recordDate range = %+v", a.RecordStats.RecordDate)
	}
	wantCreated := time.Date(2021, 3, 24, 9, 28, 41, 200_000_000, time.UTC)
	if !a.CreatedAt.Time.Equal(wantCreated) {
		t.Errorf("createdAt = %v, want %v", a.CreatedAt, wantCreated)
	}

	hint, ok := page.AgentHints.First(HintPaginationHasMore)
	if !ok {
		t.Fatal("expected a pagination.has_more hint")
	}
	if hint.Severity != HintInstruction || hint.Action == nil || hint.Action.URL == "" {
		t.Errorf("hint = %+v, want an instruction with an action URL", hint)
	}
	if got := page.AgentHints.ByType(HintResultEmpty); len(got) != 0 {
		t.Errorf("ByType(result.empty) = %v, want none", got)
	}
}

func TestAccountNullBalanceAndStatsDecodeToNil(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{"accounts":[{"id":"a1","balance":null,"recordStats":null,"currencyCode":""}]}`})
	c := newTestClient(t, m)
	page, err := c.ListAccounts(testCtx(t), nil)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	a := page.Accounts[0]
	if a.Balance != nil || a.RecordStats != nil {
		t.Errorf("null balance/stats should decode to nil, got %+v / %+v", a.Balance, a.RecordStats)
	}
	if a.CurrencyCode != "" {
		t.Errorf("currencyCode = %q, want the documented empty string when unresolvable", a.CurrencyCode)
	}
}

func TestListAccountsValidation(t *testing.T) {
	tests := []struct {
		name   string
		params *ListAccountsParams
		field  string
	}{
		{"limit too low", &ListAccountsParams{Limit: Int(0)}, "limit"},
		{"limit too high", &ListAccountsParams{Limit: Int(201)}, "limit"},
		{"negative offset", &ListAccountsParams{Offset: Int(-1)}, "offset"},
		{"too many ids", &ListAccountsParams{IDs: makeIDs(31)}, "id"},
		{"empty id", &ListAccountsParams{IDs: []string{"a", "  "}}, "id[1]"},
		{"duplicate id", &ListAccountsParams{IDs: []string{"a", "a"}}, "id[1]"},
		{"comma inside id", &ListAccountsParams{IDs: []string{"a,b"}}, "id[0]"},
		{"unprefixed name", &ListAccountsParams{Name: ptrFilter(Filter{Value: "bare"})}, "name"},
		{"range op on text filter", &ListAccountsParams{Name: ptrFilter(Gte("x"))}, "name"},
		{"empty text filter value", &ListAccountsParams{Name: ptrFilter(Contains(""))}, "name"},
		{"three range conditions", &ListAccountsParams{CreatedAt: []Filter{Gte("a"), Lt("b"), Eq("c")}}, "createdAt"},
		{"text op on range filter", &ListAccountsParams{CreatedAt: []Filter{Contains("x")}}, "createdAt[0]"},
		{"unknown account type", &ListAccountsParams{AccountType: ptrAccountType("Crypto")}, "accountType"},
		{"bad currency", &ListAccountsParams{CurrencyCode: String("EURO")}, "currencyCode"},
		{"unsortable field", &ListAccountsParams{SortBy: ptrSort(Desc("name"))}, "sortBy"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := mustInvalidRequest(t, tc.params.Validate())
			wantField(t, errs, tc.field)
		})
	}
}

func TestListAccountsAcceptsEveryValidAccountTypeFilter(t *testing.T) {
	for _, at := range AllAccountTypes {
		at := at
		t.Run(string(at), func(t *testing.T) {
			p := &ListAccountsParams{AccountType: &at}
			if err := p.Validate(); err != nil {
				t.Errorf("%s should be filterable: %v", at, err)
			}
		})
	}
}

func TestCreateAccountHappyPath(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 201, Body: `{
		"account": {"id":"a1","name":"My new account","accountType":"General","currencyCode":"EUR",
		            "balance":{"initial":0,"currencyCode":"EUR"}},
		"summary": {"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":1}
	}`})
	c := newTestClient(t, m)
	color := ColorTurquoise
	resp, err := c.CreateAccount(testCtx(t), &CreateAccountRequest{
		Name:             "My new account",
		AccountType:      AccountTypeGeneral,
		CurrencyCode:     "EUR",
		InitialBalance:   0,
		Color:            &color,
		ExcludeFromStats: Bool(true),
	})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if resp.Meta.StatusCode != 201 {
		t.Errorf("StatusCode = %d, want 201", resp.Meta.StatusCode)
	}
	if resp.Account == nil || resp.Account.ID != "a1" {
		t.Fatalf("account = %+v, want a1", resp.Account)
	}
	if resp.Summary == nil || !resp.Summary.AllSucceeded() {
		t.Errorf("summary = %+v, want an all-succeeded single write", resp.Summary)
	}

	var sent map[string]any
	m.Last(t).JSON(t, &sent)
	// initialBalance must be present even at zero, since the API requires it.
	if _, ok := sent["initialBalance"]; !ok {
		t.Errorf("body omitted the required initialBalance: %v", sent)
	}
	if sent["color"] != "Turquoise" {
		t.Errorf("color = %v, want Turquoise", sent["color"])
	}
	if sent["excludeFromStats"] != true {
		t.Errorf("excludeFromStats = %v, want true", sent["excludeFromStats"])
	}
	// Optional fields left unset must not appear at all.
	if _, ok := sent["bankAccountNumber"]; ok {
		t.Errorf("unset bankAccountNumber should be omitted: %v", sent)
	}
}

func TestCreateAccountValidation(t *testing.T) {
	long := makeString(81)
	tests := []struct {
		name  string
		body  *CreateAccountRequest
		field string
	}{
		{"nil body", nil, ""},
		{"missing name", &CreateAccountRequest{AccountType: AccountTypeCash, CurrencyCode: "USD"}, "name"},
		{"name too long", &CreateAccountRequest{Name: long, AccountType: AccountTypeCash, CurrencyCode: "USD"}, "name"},
		{"missing type", &CreateAccountRequest{Name: "n", CurrencyCode: "USD"}, "accountType"},
		{"read-only type", &CreateAccountRequest{Name: "n", AccountType: AccountTypeCreditCard, CurrencyCode: "USD"}, "accountType"},
		{"unknown type", &CreateAccountRequest{Name: "n", AccountType: "Crypto", CurrencyCode: "USD"}, "accountType"},
		{"missing currency", &CreateAccountRequest{Name: "n", AccountType: AccountTypeCash}, "currencyCode"},
		{"bad currency", &CreateAccountRequest{Name: "n", AccountType: AccountTypeCash, CurrencyCode: "US"}, "currencyCode"},
		{"digits in currency", &CreateAccountRequest{Name: "n", AccountType: AccountTypeCash, CurrencyCode: "U5D"}, "currencyCode"},
		{"iban too long", &CreateAccountRequest{Name: "n", AccountType: AccountTypeCash, CurrencyCode: "USD", BankAccountNumber: String(long)}, "bankAccountNumber"},
		{"bad colour", &CreateAccountRequest{Name: "n", AccountType: AccountTypeCash, CurrencyCode: "USD", Color: ptrColor("Beige")}, "color"},
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

func TestCreateAccountAcceptsEveryCreatableType(t *testing.T) {
	for _, at := range CreatableAccountTypes {
		at := at
		t.Run(string(at), func(t *testing.T) {
			body := &CreateAccountRequest{Name: "n", AccountType: at, CurrencyCode: "USD"}
			if err := body.Validate(); err != nil {
				t.Errorf("%s should be creatable: %v", at, err)
			}
		})
	}
	for _, at := range []AccountType{AccountTypeCreditCard, AccountTypeBonus, AccountTypeInvestment, AccountTypeLoan, AccountTypeMortgage, AccountTypeOverdraft} {
		at := at
		t.Run("rejects "+string(at), func(t *testing.T) {
			body := &CreateAccountRequest{Name: "n", AccountType: at, CurrencyCode: "USD"}
			if err := body.Validate(); err == nil {
				t.Errorf("%s must not be creatable over REST", at)
			}
		})
	}
}

func TestCreateAccountAcceptsEveryUniversalColour(t *testing.T) {
	for _, colour := range AllUniversalColors {
		colour := colour
		body := &CreateAccountRequest{Name: "n", AccountType: AccountTypeCash, CurrencyCode: "USD", Color: &colour}
		if err := body.Validate(); err != nil {
			t.Errorf("%s should be accepted: %v", colour, err)
		}
	}
}

func TestPatchAccountsBatchOutcomes(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		wantPartial bool
		wantAllOK   bool
	}{
		{"all succeeded", 200, false, true},
		{"mixed", 207, true, false},
		{"all client errors", 400, false, false},
		{"all server errors", 500, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := batchBody(tc.status)
			m := newMockServer(t, mockResponse{Status: tc.status, Body: body})
			c := newTestClient(t, m)
			resp, err := c.PatchAccounts(testCtx(t), []PatchAccountItem{
				{ID: "a1", Name: String("one")},
				{ID: "a2", Name: String("two")},
			}, nil)
			if err != nil {
				t.Fatalf("a %d batch response must decode, not error: %v", tc.status, err)
			}
			if resp.Meta.StatusCode != tc.status {
				t.Errorf("StatusCode = %d, want %d", resp.Meta.StatusCode, tc.status)
			}
			if got := resp.Summary.Partial(); got != tc.wantPartial {
				t.Errorf("Partial() = %v, want %v", got, tc.wantPartial)
			}
			if got := resp.Summary.AllSucceeded(); got != tc.wantAllOK {
				t.Errorf("AllSucceeded() = %v, want %v", got, tc.wantAllOK)
			}
			if len(resp.Results) != 2 {
				t.Fatalf("results = %d, want 2", len(resp.Results))
			}
			// InputIndex correlates results back to inputs.
			for i, r := range resp.Results {
				if r.InputIndex != i {
					t.Errorf("results[%d].InputIndex = %d, want %d", i, r.InputIndex, i)
				}
			}
		})
	}
}

func TestPatchAccountsMixedResultCarriesPerItemDetail(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 207, Body: `{
		"results": [
			{"inputIndex":0,"id":"a1","success":true,"account":{"id":"a1","name":"Renamed"}},
			{"inputIndex":1,"id":"a2","success":false,"error":"account not found",
			 "errorType":"client_error","fields":["id"]}
		],
		"summary": {"total":2,"succeeded":1,"clientErrors":1,"serverErrors":0,"documentsWritten":1}
	}`})
	c := newTestClient(t, m)
	resp, err := c.PatchAccounts(testCtx(t), []PatchAccountItem{
		{ID: "a1", Name: String("Renamed")},
		{ID: "a2", Name: String("Nope")},
	}, &PatchAccountsParams{ReturnData: Bool(true)})
	if err != nil {
		t.Fatalf("PatchAccounts: %v", err)
	}
	ok, bad := resp.Results[0], resp.Results[1]
	if !ok.Success || ok.Account == nil || ok.Account.Name != "Renamed" {
		t.Errorf("results[0] = %+v, want a successful rename", ok)
	}
	if bad.Success || !bad.IsClientError() || bad.Error != "account not found" {
		t.Errorf("results[1] = %+v, want a client error", bad)
	}
	if len(bad.Fields) != 1 || bad.Fields[0] != "id" {
		t.Errorf("results[1].Fields = %v, want [id]", bad.Fields)
	}
	if resp.Summary.Failed() != 1 || !resp.Summary.Partial() {
		t.Errorf("summary = %+v, want one failure and a partial batch", resp.Summary)
	}
	if got := m.Last(t).Query.Get("returnData"); got != "true" {
		t.Errorf("returnData = %q, want true", got)
	}
	if got := m.Last(t).Method; got != http.MethodPatch {
		t.Errorf("method = %s, want PATCH", got)
	}
}

func TestPatchAccountsReturnDataFalseOmitsBodies(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 200, Body: `{
		"results":[{"inputIndex":0,"id":"a1","success":true}],
		"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":1}
	}`})
	c := newTestClient(t, m)
	resp, err := c.PatchAccounts(testCtx(t), []PatchAccountItem{{ID: "a1", Archived: Bool(true)}},
		&PatchAccountsParams{ReturnData: Bool(false)})
	if err != nil {
		t.Fatalf("PatchAccounts: %v", err)
	}
	if got := m.Last(t).Query.Get("returnData"); got != "false" {
		t.Errorf("returnData = %q, want false", got)
	}
	if resp.Results[0].Account != nil {
		t.Errorf("account should be absent with returnData=false, got %+v", resp.Results[0].Account)
	}
	if !resp.Results[0].Success || resp.Results[0].ID != "a1" {
		t.Errorf("the id and success flag must survive returnData=false: %+v", resp.Results[0])
	}
}

func TestPatchAccountsPlain400IsAValidationError(t *testing.T) {
	// A 400 whose body is a bare ValidationError, not a batch envelope, must
	// surface as an error rather than an empty envelope.
	m := newMockServer(t, mockResponse{Status: 400, Body: `{"error":"invalid parameter"}`})
	c := newTestClient(t, m)
	_, err := c.PatchAccounts(testCtx(t), []PatchAccountItem{{ID: "a1", Name: String("x")}}, nil)
	apiErr := mustAPIError(t, err, KindValidation)
	if apiErr.Message != "invalid parameter" {
		t.Errorf("Message = %q, want the ValidationError text", apiErr.Message)
	}
}

func TestPatchAccountsBatch500IsNotRetried(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 500, Body: batchBody(500)})
	c := newTestClient(t, m, func(cfg *Config) {
		cfg.Retry = RetryConfig{MaxRetries: 3, RetryWrites: true, NoJitter: true, BaseDelay: time.Millisecond}
	})
	resp, err := c.PatchAccounts(testCtx(t), []PatchAccountItem{{ID: "a1", Name: String("x")}, {ID: "a2", Name: String("y")}}, nil)
	if err != nil {
		t.Fatalf("a batch 500 must decode, not error: %v", err)
	}
	if m.Count() != 1 {
		t.Errorf("requests = %d, want 1 — a batch envelope carries per-item detail and must not be retried away", m.Count())
	}
	if resp.Summary.ServerErrors != 2 {
		t.Errorf("serverErrors = %d, want 2", resp.Summary.ServerErrors)
	}
}

func TestPatchAccountsValidation(t *testing.T) {
	long := makeString(81)
	tests := []struct {
		name  string
		items []PatchAccountItem
		field string
	}{
		{"empty batch", nil, "items"},
		{"too many items", makePatchAccounts(11), "items"},
		{"missing id", []PatchAccountItem{{Name: String("x")}}, "items[0].id"},
		{"no fields", []PatchAccountItem{{ID: "a1"}}, "items[0]"},
		{"name too long", []PatchAccountItem{{ID: "a1", Name: String(long)}}, "items[0].name"},
		{"blank name", []PatchAccountItem{{ID: "a1", Name: String("  ")}}, "items[0].name"},
		{"bad colour", []PatchAccountItem{{ID: "a1", Color: ptrColor("Beige")}}, "items[0].color"},
		{"iban too long", []PatchAccountItem{{ID: "a1", BankAccountNumber: String(long)}}, "items[0].bankAccountNumber"},
		{"duplicate id", []PatchAccountItem{{ID: "a1", Archived: Bool(true)}, {ID: "a1", Archived: Bool(false)}}, "items[1].id"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := mustInvalidRequest(t, patchAccountsBody(tc.items).Validate())
			wantField(t, errs, tc.field)
		})
	}
}

func TestPatchAccountEmptyBankAccountNumberClearsTheValue(t *testing.T) {
	// An empty string is meaningful here, so validation must let it through.
	item := &PatchAccountItem{ID: "a1", BankAccountNumber: String("")}
	if err := item.Validate(); err != nil {
		t.Errorf("an empty bankAccountNumber clears the value and must be accepted: %v", err)
	}
	body, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got, want := string(body), `{"id":"a1","bankAccountNumber":""}`; got != want {
		t.Errorf("marshalled = %s, want %s", got, want)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func ptrFilter(f Filter) *Filter { return &f }
func ptrSort(s Sort) *Sort       { return &s }

func ptrColor(c UniversalColor) *UniversalColor { return &c }
func ptrAccountType(a AccountType) *AccountType { return &a }

func makeIDs(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = string(rune('a'+i%26)) + string(rune('0'+i/26))
	}
	return out
}

func makeString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}

func makePatchAccounts(n int) []PatchAccountItem {
	out := make([]PatchAccountItem, n)
	for i := range out {
		out[i] = PatchAccountItem{ID: makeIDs(n)[i], Archived: Bool(true)}
	}
	return out
}

// batchBody builds a two-item batch envelope matching the given status.
func batchBody(status int) string {
	switch status {
	case 200:
		return `{"results":[{"inputIndex":0,"id":"a1","success":true},{"inputIndex":1,"id":"a2","success":true}],
		         "summary":{"total":2,"succeeded":2,"clientErrors":0,"serverErrors":0,"documentsWritten":2}}`
	case 207:
		return `{"results":[{"inputIndex":0,"id":"a1","success":true},
		                    {"inputIndex":1,"id":"a2","success":false,"error":"nope","errorType":"client_error"}],
		         "summary":{"total":2,"succeeded":1,"clientErrors":1,"serverErrors":0,"documentsWritten":1}}`
	case 400:
		return `{"results":[{"inputIndex":0,"id":"a1","success":false,"error":"bad","errorType":"client_error"},
		                    {"inputIndex":1,"id":"a2","success":false,"error":"bad","errorType":"client_error"}],
		         "summary":{"total":2,"succeeded":0,"clientErrors":2,"serverErrors":0,"documentsWritten":0}}`
	default:
		return `{"results":[{"inputIndex":0,"id":"a1","success":false,"error":"boom","errorType":"server_error"},
		                    {"inputIndex":1,"id":"a2","success":false,"error":"boom","errorType":"server_error"}],
		         "summary":{"total":2,"succeeded":0,"clientErrors":0,"serverErrors":2,"documentsWritten":0}}`
	}
}
