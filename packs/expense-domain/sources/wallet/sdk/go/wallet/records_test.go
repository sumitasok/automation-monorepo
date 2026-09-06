package wallet

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestListRecordsSerialisesEveryParameter(t *testing.T) {
	m := newMockServer(t, mockResponse{JSONBody: map[string]any{"records": []any{}}})
	c := newTestClient(t, m)

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	recordType := RecordTypeExpense
	if _, err := c.ListRecords(testCtx(t), &ListRecordsParams{
		IDs:          []string{"r1", "r2"},
		AccountIDs:   []string{"acc1", "acc2"},
		RecordDate:   Between(from, to),
		Limit:        Int(200),
		Offset:       Int(10),
		AgentHints:   Bool(true),
		WithTotal:    Bool(true),
		CategoryIDs:  []string{CategoryUnknown},
		LabelID:      String("lab1"),
		Note:         ptrFilter(Contains("lunch")),
		CounterParty: ptrFilter(Eq("Downtown Bistro")),
		Amount:       []Filter{AmountFilter(OpLte, -100.5)},
		CreatedAt:    []Filter{Gte("2024-01-01T00:00:00Z")},
		UpdatedAt:    []Filter{Lt("2024-05-01T00:00:00Z")},
		RecordType:   &recordType,
		IsTransfer:   Bool(false),
		TransferIDs:  []string{"t1"},
		RecordStates: []RecordState{RecordStateReconciled, RecordStateUncleared},
		Sources:      []RecordSource{SourceMCP, SourceREST},
		SortBy:       ptrSort(Asc("recordDate")),
		ConvertTo:    String("CZK"),
	}); err != nil {
		t.Fatalf("ListRecords: %v", err)
	}

	q := m.Last(t).Query
	checks := map[string]string{
		"id":           "r1,r2",
		"accountId":    "acc1,acc2",
		"limit":        "200",
		"offset":       "10",
		"agentHints":   "true",
		"withTotal":    "true",
		"categoryId":   "unknown",
		"labelId":      "lab1",
		"note":         "contains.lunch",
		"counterParty": "eq.Downtown Bistro",
		"amount":       "lte.-100.5",
		"recordType":   "expense",
		"isTransfer":   "false",
		"transferId":   "t1",
		"recordState":  "reconciled,uncleared",
		"source":       "mcp,rest",
		"sortBy":       "+recordDate",
		"convertTo":    "CZK",
	}
	for key, want := range checks {
		if got := q.Get(key); got != want {
			t.Errorf("query %q = %q, want %q", key, got, want)
		}
	}
	if got := q["recordDate"]; len(got) != 2 {
		t.Errorf("recordDate = %v, want two bounds", got)
	}
}

func TestAmountFilterTrimsTrailingZeroes(t *testing.T) {
	tests := map[float64]string{
		100.50:  "100.5",
		100.00:  "100",
		-42.5:   "-42.5",
		0.01:    "0.01",
		1234.56: "1234.56",
	}
	for in, want := range tests {
		if got := FormatAmount(in); got != want {
			t.Errorf("FormatAmount(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestListRecordsReportsTheAppliedDefaultWindow(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{
		"records": [],
		"appliedRecordDateFilters": ["gte.2026-06-06T00:00:00Z","lt.2026-09-06T00:00:00Z"]
	}`})
	c := newTestClient(t, m)
	page, err := c.ListRecords(testCtx(t), nil)
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if len(page.AppliedRecordDateFilters) != 2 {
		t.Fatalf("appliedRecordDateFilters = %v, want the implicit 3-month window", page.AppliedRecordDateFilters)
	}
	if !page.DefaultWindowApplied(false) {
		t.Error("DefaultWindowApplied should be true when no bound was requested")
	}
	if page.DefaultWindowApplied(true) {
		t.Error("DefaultWindowApplied should be false when a bound was requested")
	}
}

func TestListRecordsNullAppliedFiltersOnIDLookup(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{"records":[{"id":"r1"}],"appliedRecordDateFilters":null}`})
	c := newTestClient(t, m)
	page, err := c.ListRecords(testCtx(t), &ListRecordsParams{IDs: []string{"r1"}})
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if page.AppliedRecordDateFilters != nil {
		t.Errorf("an ID lookup bypasses the window, so the field should be nil, got %v", page.AppliedRecordDateFilters)
	}
}

func TestListRecordsDecodesTheFullRecordShape(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{
		"limit":30,"offset":0,
		"records":[{
			"id":"550e8400-e29b-41d4-a716-446655440000",
			"accountId":"-Account_abc123",
			"accountName":"Checking Account",
			"accountIsBankSync":false,
			"amount":{"currencyCode":"USD","value":-42.5},
			"convertedAmount":{"currencyCode":"CZK","value":-1080.35,"ratio":25.42,"conversionPair":"USD:CZK"},
			"category":{"id":"cat1","name":"Health & beauty","color":"#26c6da",
			            "group":{"id":"food_and_drinks","name":"Food & Drinks"}},
			"counterParty":"Downtown Bistro",
			"labels":[{"id":"l1","name":"Important","color":"#d32f2f","archived":false}],
			"note":"Lunch at restaurant",
			"photos":[{"createdAt":"2021-03-24T09:28:41.200Z","temporaryUrl":"https://example.test/p.jpg"}],
			"place":{"address":"1 Main St","latitude":50.08,"longitude":14.42,"name":"Bistro"},
			"recordDate":"2025-03-15T14:30:00Z",
			"recordState":"cleared",
			"recordType":"expense",
			"source":"rest",
			"transfer":{"transferId":"tr1","type":"paired",
			            "mirrorRecord":{"id":"r2","accountId":"acc2","counterParty":"Self",
			                            "note":"mirror","amount":{"currencyCode":"USD","value":42.5}}},
			"createdAt":"2021-03-24T09:28:41.200Z",
			"updatedAt":"2021-03-25T09:28:41.200Z"
		}]
	}`})
	c := newTestClient(t, m)
	page, err := c.ListRecords(testCtx(t), nil)
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	r := page.Records[0]
	if r.Amount.Value != -42.5 || r.Amount.CurrencyCode != "USD" {
		t.Errorf("amount = %+v, want -42.5 USD", r.Amount)
	}
	if r.ConvertedAmount == nil || !r.ConvertedAmount.OK() || r.ConvertedAmount.Ratio != 25.42 {
		t.Errorf("convertedAmount = %+v, want a successful conversion", r.ConvertedAmount)
	}
	if r.Category == nil || r.Category.Group == nil || r.Category.Group.ID != "food_and_drinks" {
		t.Errorf("category = %+v, want an embedded group", r.Category)
	}
	if len(r.Labels) != 1 || r.Labels[0].Name != "Important" {
		t.Errorf("labels = %+v", r.Labels)
	}
	if len(r.Photos) != 1 || r.Photos[0].TemporaryURL == "" {
		t.Errorf("photos = %+v", r.Photos)
	}
	if r.Place == nil || r.Place.Latitude != 50.08 {
		t.Errorf("place = %+v", r.Place)
	}
	if !r.IsTransfer() || r.Transfer == nil || !r.Transfer.Paired() {
		t.Errorf("transfer = %+v, want a paired transfer", r.Transfer)
	}
	if r.Transfer.MirrorRecord.Amount == nil || r.Transfer.MirrorRecord.Amount.Value != 42.5 {
		t.Errorf("mirror amount = %+v, want the opposite leg", r.Transfer.MirrorRecord.Amount)
	}
	if r.Source != SourceREST || r.RecordState != RecordStateCleared || r.RecordType != RecordTypeExpense {
		t.Errorf("record enums = %s/%s/%s", r.Source, r.RecordState, r.RecordType)
	}
}

func TestRecordConvertedAmountErrorIsSurfaced(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{"records":[{"id":"r1","amount":{"value":1},
		"convertedAmount":{"currencyCode":"XYZ","error":"no rate available"}}]}`})
	c := newTestClient(t, m)
	page, err := c.ListRecords(testCtx(t), &ListRecordsParams{ConvertTo: String("XYZ")})
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	ca := page.Records[0].ConvertedAmount
	if ca == nil || ca.OK() {
		t.Fatalf("convertedAmount = %+v, want an error", ca)
	}
	if ca.Error != "no rate available" {
		t.Errorf("error = %q", ca.Error)
	}
}

func TestRecordOutputOnlyStatesDecode(t *testing.T) {
	for _, state := range []RecordState{RecordStateVoid, RecordStateWaitForAssign} {
		state := state
		t.Run(string(state), func(t *testing.T) {
			m := newMockServer(t, mockResponse{Body: `{"records":[{"id":"r1","recordState":"` + string(state) + `"}]}`})
			c := newTestClient(t, m)
			page, err := c.ListRecords(testCtx(t), nil)
			if err != nil {
				t.Fatalf("ListRecords: %v", err)
			}
			if got := page.Records[0].RecordState; got != state {
				t.Errorf("recordState = %q, want %q", got, state)
			}
			if !state.Valid() || state.Writable() {
				t.Errorf("%s should be readable but not writable", state)
			}
		})
	}
}

func TestUnknownEnumValueStillDecodes(t *testing.T) {
	// A new server-side enum member must not break an old client.
	m := newMockServer(t, mockResponse{Body: `{"records":[{"id":"r1","source":"smartwatch","recordState":"quantum"}]}`})
	c := newTestClient(t, m)
	page, err := c.ListRecords(testCtx(t), nil)
	if err != nil {
		t.Fatalf("an unknown enum value must not fail decoding: %v", err)
	}
	r := page.Records[0]
	if r.Source != "smartwatch" || r.RecordState != "quantum" {
		t.Errorf("unknown values should round-trip verbatim, got %q / %q", r.Source, r.RecordState)
	}
	if r.Source.Valid() {
		t.Error("Valid() should report an unknown source as unknown")
	}
}

func TestListRecordsValidation(t *testing.T) {
	tests := []struct {
		name   string
		params *ListRecordsParams
		field  string
	}{
		{"too many accounts", &ListRecordsParams{AccountIDs: makeIDs(11)}, "accountId"},
		{"too many categories", &ListRecordsParams{CategoryIDs: makeIDs(11)}, "categoryId"},
		{"too many ids", &ListRecordsParams{IDs: makeIDs(31)}, "id"},
		{"three recordDate conditions", &ListRecordsParams{RecordDate: []Filter{Gte("a"), Lt("b"), Eq("c")}}, "recordDate"},
		{"unprefixed note", &ListRecordsParams{Note: ptrFilter(Filter{Value: "x"})}, "note"},
		{"unprefixed counterParty", &ListRecordsParams{CounterParty: ptrFilter(Filter{Value: "x"})}, "counterParty"},
		{"bad record type", &ListRecordsParams{RecordType: ptrRecordType("refund")}, "recordType"},
		{"unfilterable record state", &ListRecordsParams{RecordStates: []RecordState{RecordStateVoid}}, "recordState[0]"},
		{"bad source", &ListRecordsParams{Sources: []RecordSource{"smartwatch"}}, "source[0]"},
		{"unsortable field", &ListRecordsParams{SortBy: ptrSort(Asc("note"))}, "sortBy"},
		{"bad convertTo", &ListRecordsParams{ConvertTo: String("EURO")}, "convertTo"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := mustInvalidRequest(t, tc.params.Validate())
			wantField(t, errs, tc.field)
		})
	}
}

func TestListRecordsAcceptsMagicConvertToValues(t *testing.T) {
	for _, v := range []string{ConvertToBase, ConvertToAccount, "EUR", "czk"} {
		p := &ListRecordsParams{ConvertTo: String(v)}
		if err := p.Validate(); err != nil {
			t.Errorf("convertTo=%q should be accepted: %v", v, err)
		}
	}
}

func TestListRecordsAcceptsEveryRecordSortField(t *testing.T) {
	for _, field := range RecordSortFields {
		for _, s := range []Sort{Asc(field), Desc(field)} {
			s := s
			p := &ListRecordsParams{SortBy: &s}
			if err := p.Validate(); err != nil {
				t.Errorf("sortBy=%s should be accepted: %v", s, err)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func validRecord() CreateRecordRequest {
	return CreateRecordRequest{
		AccountID:  "acc1",
		Amount:     AmountInput{Value: -42.5},
		RecordDate: NewTime(fixedTime.Add(-time.Hour)),
	}
}

func TestCreateRecordsHappyPath(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 200, Body: `{
		"results":[{"inputIndex":0,"id":"r1","success":true,
		            "record":{"id":"r1","amount":{"value":-42.5,"currencyCode":"USD"},"recordType":"expense"}}],
		"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":1}
	}`})
	c := newTestClient(t, m)
	rec := validRecord()
	rec.CategoryID = String(CategoryUnknownExpense)
	rec.CounterParty = String("Downtown Bistro")
	rec.Note = String("Lunch")
	rec.LabelIDs = []string{"l1"}
	rec.RecordState = ptrRecordState(RecordStateUncleared)
	rec.Place = &PlaceInput{Name: String("Bistro"), Latitude: Float(50.08), Longitude: Float(14.42)}

	resp, err := c.CreateRecords(testCtx(t), []CreateRecordRequest{rec}, nil)
	if err != nil {
		t.Fatalf("CreateRecords: %v", err)
	}
	if !resp.Summary.AllSucceeded() || resp.Results[0].Record == nil {
		t.Fatalf("response = %+v", resp)
	}

	// The body must be a bare array, as the endpoint expects.
	var sent []map[string]any
	m.Last(t).JSON(t, &sent)
	if len(sent) != 1 {
		t.Fatalf("body = %v, want a one-element array", sent)
	}
	if sent[0]["categoryId"] != CategoryUnknownExpense {
		t.Errorf("categoryId = %v", sent[0]["categoryId"])
	}
	if sent[0]["recordDate"] != "2026-09-06T11:00:00.000Z" {
		t.Errorf("recordDate = %v, want the canonical millisecond form", sent[0]["recordDate"])
	}
	amount, _ := sent[0]["amount"].(map[string]any)
	if amount["value"] != -42.5 {
		t.Errorf("amount = %v", amount)
	}
	if _, ok := amount["currencyCode"]; ok {
		t.Errorf("an unset currencyCode should be omitted so the account's applies: %v", amount)
	}
}

func TestCreateRecordSingleConvenience(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 200, Body: `{
		"results":[{"inputIndex":0,"id":"r1","success":true}],
		"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":1}
	}`})
	c := newTestClient(t, m)
	result, resp, err := c.CreateRecord(testCtx(t), validRecord(), nil)
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if result.ID != "r1" || !result.Success {
		t.Errorf("result = %+v", result)
	}
	if resp.Summary.Total != 1 {
		t.Errorf("summary = %+v", resp.Summary)
	}
}

func TestCreateRecordSingleReportsAFailedItemWithoutAnError(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 400, Body: `{
		"results":[{"inputIndex":0,"success":false,"error":"account not found","errorType":"client_error","fields":["accountId"]}],
		"summary":{"total":1,"succeeded":0,"clientErrors":1,"serverErrors":0,"documentsWritten":0}
	}`})
	c := newTestClient(t, m)
	result, _, err := c.CreateRecord(testCtx(t), validRecord(), nil)
	if err != nil {
		t.Fatalf("a per-item failure is data, not an error: %v", err)
	}
	if result.Success || !result.IsClientError() {
		t.Errorf("result = %+v, want a client error", result)
	}
}

func TestCreateRecordSingleErrorsWhenServerReturnsNoResult(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 200, Body: `{"results":[],"summary":{"total":1,"succeeded":0,"clientErrors":0,"serverErrors":0,"documentsWritten":0}}`})
	c := newTestClient(t, m)
	_, _, err := c.CreateRecord(testCtx(t), validRecord(), nil)
	mustAPIError(t, err, KindDecode)
}

func TestCreateRecordsTransferMirrorID(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 200, Body: `{
		"results":[{"inputIndex":0,"id":"r1","success":true,"createdMirrorRecordId":"r2"}],
		"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":2}
	}`})
	c := newTestClient(t, m)
	rec := validRecord()
	rec.Transfer = &TransferInput{PairingMode: PairingNew, AccountID: String("acc2")}
	resp, err := c.CreateRecords(testCtx(t), []CreateRecordRequest{rec}, &CreateRecordsParams{ReturnData: Bool(false)})
	if err != nil {
		t.Fatalf("CreateRecords: %v", err)
	}
	if got := resp.Results[0].CreatedMirrorRecordID; got != "r2" {
		t.Errorf("createdMirrorRecordId = %q, want r2 — it survives returnData=false", got)
	}
	// documentsWritten exceeding total is expected: the mirror is a document
	// without being an item.
	if resp.Summary.DocumentsWritten <= resp.Summary.Total {
		t.Errorf("documentsWritten = %d, expected it to exceed total (%d) for a paired transfer",
			resp.Summary.DocumentsWritten, resp.Summary.Total)
	}
	if got := m.Last(t).Query.Get("returnData"); got != "false" {
		t.Errorf("returnData = %q, want false", got)
	}
}

func TestCreateRecordsValidation(t *testing.T) {
	future := validRecord()
	future.RecordDate = NewTime(fixedTime.Add(25 * time.Hour))

	zeroAmount := validRecord()
	zeroAmount.Amount.Value = 0

	longNote := validRecord()
	longNote.Note = String(makeString(256))

	longParty := validRecord()
	longParty.CounterParty = String(makeString(256))

	badState := validRecord()
	badState.RecordState = ptrRecordState(RecordStateVoid)

	badPlace := validRecord()
	badPlace.Place = &PlaceInput{Latitude: Float(91), Longitude: Float(-181), Address: String(makeString(256))}

	badCurrency := validRecord()
	badCurrency.Amount.CurrencyCode = String("US")

	noAccount := validRecord()
	noAccount.AccountID = ""

	noDate := validRecord()
	noDate.RecordDate = Time{}

	tests := []struct {
		name    string
		records []CreateRecordRequest
		field   string
	}{
		{"empty batch", nil, "records"},
		{"too many records", make([]CreateRecordRequest, 51), "records"},
		{"missing account", []CreateRecordRequest{noAccount}, "records[0].accountId"},
		{"missing date", []CreateRecordRequest{noDate}, "records[0].recordDate"},
		{"date too far ahead", []CreateRecordRequest{future}, "records[0].recordDate"},
		{"zero amount", []CreateRecordRequest{zeroAmount}, "records[0].amount.value"},
		{"bad amount currency", []CreateRecordRequest{badCurrency}, "records[0].amount.currencyCode"},
		{"note too long", []CreateRecordRequest{longNote}, "records[0].note"},
		{"counterParty too long", []CreateRecordRequest{longParty}, "records[0].counterParty"},
		{"unwritable state", []CreateRecordRequest{badState}, "records[0].recordState"},
		{"latitude out of range", []CreateRecordRequest{badPlace}, "records[0].place.latitude"},
		{"longitude out of range", []CreateRecordRequest{badPlace}, "records[0].place.longitude"},
		{"address too long", []CreateRecordRequest{badPlace}, "records[0].place.address"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := createRecordsBody{records: tc.records, now: fixedTime}
			errs := mustInvalidRequest(t, body.Validate())
			wantField(t, errs, tc.field)
		})
	}
}

func TestCreateRecordAcceptsDateJustInsideTheFutureWindow(t *testing.T) {
	rec := validRecord()
	rec.RecordDate = NewTime(fixedTime.Add(23 * time.Hour))
	if err := rec.validateAt(fixedTime); err != nil {
		t.Errorf("23 hours ahead is inside the 24-hour window: %v", err)
	}
	rec.RecordDate = NewTime(fixedTime.Add(MaxRecordDateFuture))
	if err := rec.validateAt(fixedTime); err != nil {
		t.Errorf("exactly 24 hours ahead is the boundary and must be accepted: %v", err)
	}
	rec.RecordDate = NewTime(fixedTime.Add(MaxRecordDateFuture + time.Second))
	if err := rec.validateAt(fixedTime); err == nil {
		t.Error("one second past the window must be rejected")
	}
}

func TestTransferInputValidation(t *testing.T) {
	tests := []struct {
		name     string
		transfer *TransferInput
		amount   float64
		field    string
	}{
		{"missing mode", &TransferInput{}, -10, "pairingMode"},
		{"unknown mode", &TransferInput{PairingMode: "clone"}, -10, "pairingMode"},
		{"new without account", &TransferInput{PairingMode: PairingNew}, -10, "accountId"},
		{"new with recordId", &TransferInput{PairingMode: PairingNew, AccountID: String("a"), RecordID: String("r")}, -10, "recordId"},
		{"existing without record", &TransferInput{PairingMode: PairingExisting}, -10, "recordId"},
		{"existing with account", &TransferInput{PairingMode: PairingExisting, RecordID: String("r"), AccountID: String("a")}, -10, "accountId"},
		{"existing with counterAmount", &TransferInput{PairingMode: PairingExisting, RecordID: String("r"),
			CounterAmount: &CounterAmountInput{Value: 10, CurrencyCode: "EUR"}}, -10, "counterAmount"},
		{"unpaired with account", &TransferInput{PairingMode: PairingUnpaired, AccountID: String("a")}, -10, "accountId"},
		{"unpaired with record", &TransferInput{PairingMode: PairingUnpaired, RecordID: String("r")}, -10, "recordId"},
		{"counterAmount same sign", &TransferInput{PairingMode: PairingNew, AccountID: String("a"),
			CounterAmount: &CounterAmountInput{Value: -10, CurrencyCode: "EUR"}}, -10, "counterAmount.value"},
		{"counterAmount zero", &TransferInput{PairingMode: PairingNew, AccountID: String("a"),
			CounterAmount: &CounterAmountInput{Value: 0, CurrencyCode: "EUR"}}, -10, "counterAmount.value"},
		{"counterAmount bad currency", &TransferInput{PairingMode: PairingNew, AccountID: String("a"),
			CounterAmount: &CounterAmountInput{Value: 10, CurrencyCode: ""}}, -10, "counterAmount.currencyCode"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := mustInvalidRequest(t, tc.transfer.Validate(tc.amount))
			wantField(t, errs, tc.field)
		})
	}
}

func TestTransferInputValidCombinations(t *testing.T) {
	valid := []*TransferInput{
		{PairingMode: PairingNew, AccountID: String("acc2")},
		{PairingMode: PairingNew, AccountID: String("acc2"),
			CounterAmount: &CounterAmountInput{Value: 40, CurrencyCode: "EUR"}},
		{PairingMode: PairingExisting, RecordID: String("r9")},
		{PairingMode: PairingUnpaired},
		nil,
	}
	for i, tr := range valid {
		if err := tr.Validate(-42.5); err != nil {
			t.Errorf("valid transfer %d rejected: %v", i, err)
		}
	}
}

func TestCreateRecordSendsTransferBlockVerbatim(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 200, Body: `{"results":[{"inputIndex":0,"id":"r1","success":true}],
		"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":2}}`})
	c := newTestClient(t, m)
	rec := validRecord()
	rec.Transfer = &TransferInput{
		PairingMode:   PairingNew,
		AccountID:     String("acc2"),
		CounterAmount: &CounterAmountInput{Value: 40, CurrencyCode: "EUR"},
	}
	if _, err := c.CreateRecords(testCtx(t), []CreateRecordRequest{rec}, nil); err != nil {
		t.Fatalf("CreateRecords: %v", err)
	}
	var sent []map[string]any
	m.Last(t).JSON(t, &sent)
	tr, _ := sent[0]["transfer"].(map[string]any)
	if tr["pairingMode"] != "new" || tr["accountId"] != "acc2" {
		t.Errorf("transfer = %v", tr)
	}
	ca, _ := tr["counterAmount"].(map[string]any)
	if ca["value"] != float64(40) || ca["currencyCode"] != "EUR" {
		t.Errorf("counterAmount = %v", ca)
	}
}

// ---------------------------------------------------------------------------
// Patch
// ---------------------------------------------------------------------------

func TestPatchRecordsSerialisesParams(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 200, Body: `{"results":[{"inputIndex":0,"id":"r1","success":true}],
		"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":1}}`})
	c := newTestClient(t, m)
	mode := ValidationStrict
	if _, err := c.PatchRecords(testCtx(t), []PatchRecordItem{{ID: "r1", Note: String("updated")}},
		&PatchRecordsParams{Validation: &mode, ReturnData: Bool(false)}); err != nil {
		t.Fatalf("PatchRecords: %v", err)
	}
	q := m.Last(t).Query
	if q.Get("validation") != "strict" || q.Get("returnData") != "false" {
		t.Errorf("query = %v", q)
	}
}

func TestPatchRecordsRejectsUnknownValidationMode(t *testing.T) {
	mode := ValidationMode("paranoid")
	errs := mustInvalidRequest(t, (&PatchRecordsParams{Validation: &mode}).Validate())
	wantField(t, errs, "validation")
}

func TestPatchRecordClearAndSetConflict(t *testing.T) {
	tests := []struct {
		name  string
		item  PatchRecordItem
		field string
	}{
		{"note", PatchRecordItem{ID: "r1", Note: String("x"), Clear: []ClearableRecordField{ClearNote}}, "$clear[0]"},
		{"categoryId", PatchRecordItem{ID: "r1", CategoryID: String("c"), Clear: []ClearableRecordField{ClearCategoryID}}, "$clear[0]"},
		{"labelIds", PatchRecordItem{ID: "r1", LabelIDs: ReplaceIDs([]string{"l"}), Clear: []ClearableRecordField{ClearLabelIDs}}, "$clear[0]"},
		{"counterParty", PatchRecordItem{ID: "r1", CounterParty: String("x"), Clear: []ClearableRecordField{ClearCounterParty}}, "$clear[0]"},
		{"place", PatchRecordItem{ID: "r1", Place: &PlaceInput{}, Clear: []ClearableRecordField{ClearPlace}}, "$clear[0]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := mustInvalidRequest(t, tc.item.Validate())
			wantField(t, errs, tc.field)
		})
	}
}

func TestPatchRecordClearTransferHasNoSetterToConflictWith(t *testing.T) {
	item := PatchRecordItem{ID: "r1", Clear: []ClearableRecordField{ClearTransfer}}
	if err := item.Validate(); err != nil {
		t.Errorf("clearing transfer alone is legal: %v", err)
	}
}

func TestPatchRecordValidation(t *testing.T) {
	tests := []struct {
		name  string
		item  PatchRecordItem
		field string
	}{
		{"missing id", PatchRecordItem{Note: String("x")}, "id"},
		{"nothing to do", PatchRecordItem{ID: "r1"}, ""},
		{"zero amount", PatchRecordItem{ID: "r1", Amount: &AmountInput{Value: 0}}, "amount.value"},
		{"note too long", PatchRecordItem{ID: "r1", Note: String(makeString(256))}, "note"},
		{"counterParty too long", PatchRecordItem{ID: "r1", CounterParty: String(makeString(256))}, "counterParty"},
		{"unwritable state", PatchRecordItem{ID: "r1", RecordState: ptrRecordState(RecordStateWaitForAssign)}, "recordState"},
		{"unknown clear field", PatchRecordItem{ID: "r1", Clear: []ClearableRecordField{"amount"}}, "$clear[0]"},
		{"duplicate clear field", PatchRecordItem{ID: "r1", Clear: []ClearableRecordField{ClearNote, ClearNote}}, "$clear[1]"},
		{"empty label delta", PatchRecordItem{ID: "r1", LabelIDs: AddRemoveIDs(nil, nil)}, "labelIds"},
		{"zero recordDate", PatchRecordItem{ID: "r1", RecordDate: &Time{}}, "recordDate"},
		{"bad place", PatchRecordItem{ID: "r1", Place: &PlaceInput{Latitude: Float(200)}}, "place.latitude"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := mustInvalidRequest(t, tc.item.Validate())
			if tc.field != "" {
				wantField(t, errs, tc.field)
			}
		})
	}
}

func TestStringSetEncodesBothDocumentedShapes(t *testing.T) {
	tests := []struct {
		name string
		set  *StringSet
		want string
	}{
		{"replace", ReplaceIDs([]string{"a", "b"}), `["a","b"]`},
		{"clear", ClearIDs(), `[]`},
		{"replace nil is a clear", ReplaceIDs(nil), `[]`},
		{"add only", AddRemoveIDs([]string{"a"}, nil), `{"add":["a"]}`},
		{"remove only", AddRemoveIDs(nil, []string{"b"}), `{"remove":["b"]}`},
		{"both", AddRemoveIDs([]string{"a"}, []string{"b"}), `{"add":["a"],"remove":["b"]}`},
		{"unset", nil, `null`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.set)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("marshalled = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestStringSetRoundTrips(t *testing.T) {
	for _, raw := range []string{`["a","b"]`, `[]`, `{"add":["a"],"remove":["b"]}`, `{"add":["a"]}`, `null`} {
		var set StringSet
		if err := json.Unmarshal([]byte(raw), &set); err != nil {
			t.Fatalf("Unmarshal(%s): %v", raw, err)
		}
		got, err := json.Marshal(&set)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if string(got) != raw {
			t.Errorf("round trip of %s produced %s", raw, got)
		}
	}
}

func TestStringSetAccessors(t *testing.T) {
	replace := ReplaceIDs([]string{"a"})
	if replace.IsDelta() || len(replace.Values()) != 1 {
		t.Errorf("replace set = %+v", replace)
	}
	if add, remove := replace.Delta(); add != nil || remove != nil {
		t.Errorf("Delta on a replace set should be empty, got %v/%v", add, remove)
	}
	delta := AddRemoveIDs([]string{"a"}, []string{"b"})
	if !delta.IsDelta() || delta.Values() != nil {
		t.Errorf("delta set = %+v", delta)
	}
	add, remove := delta.Delta()
	if len(add) != 1 || len(remove) != 1 {
		t.Errorf("Delta = %v/%v", add, remove)
	}
	var nilSet *StringSet
	if nilSet.IsDelta() || nilSet.Values() != nil {
		t.Error("a nil set must be inert")
	}
	if err := nilSet.Validate("labelIds"); err != nil {
		t.Errorf("a nil set validates as unset: %v", err)
	}
}

func TestPatchRecordBodySendsTheClearKey(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 200, Body: `{"results":[{"inputIndex":0,"id":"r1","success":true}],
		"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":1}}`})
	c := newTestClient(t, m)
	if _, err := c.PatchRecords(testCtx(t), []PatchRecordItem{{
		ID:       "r1",
		Clear:    []ClearableRecordField{ClearNote, ClearPlace},
		LabelIDs: AddRemoveIDs([]string{"l1"}, []string{"l2"}),
	}}, nil); err != nil {
		t.Fatalf("PatchRecords: %v", err)
	}
	body := string(m.Last(t).Body)
	if !strings.Contains(body, `"$clear":["note","place"]`) {
		t.Errorf("body should carry the $clear array verbatim: %s", body)
	}
	if !strings.Contains(body, `"labelIds":{"add":["l1"],"remove":["l2"]}`) {
		t.Errorf("body should carry the label delta: %s", body)
	}
}

func TestPatchRecordsStrictModeReturns400WithFullResults(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 400, Body: `{
		"results":[
			{"inputIndex":0,"id":"r1","success":false,"error":"validated but not written","errorType":"client_error"},
			{"inputIndex":1,"id":"r2","success":false,"error":"category not found","errorType":"client_error","fields":["categoryId"]}],
		"summary":{"total":2,"succeeded":0,"clientErrors":2,"serverErrors":0,"documentsWritten":0}
	}`})
	c := newTestClient(t, m)
	mode := ValidationStrict
	resp, err := c.PatchRecords(testCtx(t), []PatchRecordItem{
		{ID: "r1", Note: String("a")},
		{ID: "r2", CategoryID: String("missing")},
	}, &PatchRecordsParams{Validation: &mode})
	if err != nil {
		t.Fatalf("strict-mode 400 must decode to the envelope: %v", err)
	}
	if resp.Summary.DocumentsWritten != 0 {
		t.Errorf("strict mode writes nothing, got documentsWritten=%d", resp.Summary.DocumentsWritten)
	}
	if len(resp.Results) != 2 {
		t.Errorf("results = %d, want 2", len(resp.Results))
	}
}

func ptrRecordType(r RecordType) *RecordType    { return &r }
func ptrRecordState(r RecordState) *RecordState { return &r }
