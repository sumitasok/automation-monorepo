package wallet

import (
	"net/http"
	"sort"
	"strings"
	"testing"
)

func TestDeleteHappyPath(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 200, Body: `{
		"results":[{"inputIndex":0,"id":"r1","success":true},{"inputIndex":1,"id":"r2","success":true}],
		"summary":{"total":2,"succeeded":2,"clientErrors":0,"serverErrors":0,"documentsWritten":2}}`})
	c := newTestClient(t, m)
	resp, err := c.Delete(testCtx(t), EntityRecords, []string{"r1", "r2"})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !resp.Summary.AllSucceeded() {
		t.Errorf("summary = %+v", resp.Summary)
	}
	req := m.Last(t)
	if req.Method != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", req.Method)
	}
	if got, want := req.Path, "/v1/api/records"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	if !strings.Contains(string(req.Body), `"ids":["r1","r2"]`) {
		t.Errorf("body = %s", req.Body)
	}
}

func TestDeleteIsIdempotentPerID(t *testing.T) {
	// A repeated delete of an already-gone document succeeds while writing
	// nothing, which is why documentsWritten can trail succeeded.
	m := newMockServer(t, mockResponse{Status: 200, Body: `{
		"results":[{"inputIndex":0,"id":"r1","success":true}],
		"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":0}}`})
	c := newTestClient(t, m)
	resp, err := c.Delete(testCtx(t), EntityRecords, []string{"r1"})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if resp.Summary.Succeeded != 1 || resp.Summary.DocumentsWritten != 0 {
		t.Errorf("summary = %+v, want succeeded 1 with 0 documents written", resp.Summary)
	}
}

func TestDeletePairedTransferRejectionIsPerItem(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 400, Body: `{
		"results":[{"inputIndex":0,"id":"r1","success":false,
		            "error":"paired transfer: the other leg r2 must be deleted in the same request",
		            "errorType":"client_error","fields":["ids"]}],
		"summary":{"total":1,"succeeded":0,"clientErrors":1,"serverErrors":0,"documentsWritten":0}}`})
	c := newTestClient(t, m)
	resp, err := c.Delete(testCtx(t), EntityRecords, []string{"r1"})
	if err != nil {
		t.Fatalf("a batch 400 must decode: %v", err)
	}
	if resp.Results[0].Success || !strings.Contains(resp.Results[0].Error, "other leg r2") {
		t.Errorf("result = %+v, want the missing-leg error", resp.Results[0])
	}
}

func TestDeleteValidation(t *testing.T) {
	tests := []struct {
		name       string
		entityType EntityType
		ids        []string
		field      string
	}{
		{"empty ids", EntityRecords, nil, "ids"},
		{"too many ids", EntityRecords, makeIDs(11), "ids"},
		{"blank id", EntityRecords, []string{"r1", ""}, "ids[1]"},
		{"duplicate id", EntityRecords, []string{"r1", "r1"}, "ids[1]"},
		{"empty type", "", []string{"r1"}, "type"},
		{"unknown type", "widgets", []string{"r1"}, "type"},
		{"board-only orders", EntityOrders, []string{"o1"}, "type"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := deleteArgs{entityType: tc.entityType, body: &DeleteRequest{IDs: tc.ids}}
			errs := mustInvalidRequest(t, args.Validate())
			wantField(t, errs, tc.field)
		})
	}
}

func TestDeleteAcceptsEveryWalletEntityType(t *testing.T) {
	for _, et := range DeletableEntityTypes {
		if et == EntityOrders {
			continue // Board-only, tested separately
		}
		et := et
		t.Run(string(et), func(t *testing.T) {
			m := newMockServer(t, mockResponse{Status: 200, Body: `{
				"results":[{"inputIndex":0,"id":"x","success":true}],
				"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":1}}`})
			c := newTestClient(t, m)
			if _, err := c.Delete(testCtx(t), et, []string{"x"}); err != nil {
				t.Fatalf("Delete(%s): %v", et, err)
			}
			if got, want := m.Last(t).Path, "/v1/api/"+string(et); got != want {
				t.Errorf("path = %q, want %q", got, want)
			}
		})
	}
}

func TestDeleteOrdersIsRejectedBeforeSending(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 404, Body: `{"error":"not found"}`})
	c := newTestClient(t, m)
	_, err := c.Delete(testCtx(t), EntityOrders, []string{"o1"})
	errs := mustInvalidRequest(t, err)
	wantField(t, errs, "type")
	if m.Count() != 0 {
		t.Errorf("the Board-only collection should be caught locally, but %d requests arrived", m.Count())
	}
	// Disabling validation lets the caller reach the 404 if they insist.
	c2 := newTestClient(t, m, func(cfg *Config) { cfg.SkipValidation = true })
	_, err = c2.Delete(testCtx(t), EntityOrders, []string{"o1"})
	mustAPIError(t, err, KindNotFound)
}

func TestReferencesHappyPath(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{
		"550e8400-e29b-41d4-a716-446655440000": {
			"records":{"field":"categoryId","ids":["a1b2c3d4"],"total":3,"limit":50,"hasMore":false},
			"budgets":{"field":"categoryIds","ids":["b2c3d4e5"],"total":1,"limit":50,"hasMore":false},
			"recordRules":{"field":"categoryId","ids":[],"total":0,"limit":50,"hasMore":false},
			"standingOrders":{"field":"categoryId","ids":[],"total":0,"limit":50,"hasMore":false}
		},
		"missing-id": {"error":"not found"},
		"wrong-type": {"error":"type_mismatch","actualType":"account",
		               "message":"this id exists as an account, not a category"}
	}`})
	c := newTestClient(t, m)
	res, err := c.References(testCtx(t), &ReferencesParams{
		Type: EntityCategories,
		IDs:  []string{"550e8400-e29b-41d4-a716-446655440000", "missing-id", "wrong-type"},
	})
	if err != nil {
		t.Fatalf("References: %v", err)
	}
	if got, want := m.Last(t).Path, "/v1/api/categories/references"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	if got := m.Last(t).Query.Get("id"); !strings.Contains(got, ",") {
		t.Errorf("id = %q, want a comma-separated list", got)
	}

	referenced := res.Entries["550e8400-e29b-41d4-a716-446655440000"]
	if !referenced.Resolved() || referenced.TotalReferences() != 4 {
		t.Errorf("entry = %+v, want 4 references (3 records + 1 budget)", referenced)
	}
	if referenced.Records == nil || referenced.Records.Field != "categoryId" {
		t.Errorf("records = %+v", referenced.Records)
	}

	missing := res.Entries["missing-id"]
	if missing.Resolved() || missing.Error != "not found" {
		t.Errorf("missing entry = %+v", missing)
	}
	mismatch := res.Entries["wrong-type"]
	if mismatch.ActualType != "account" || mismatch.Message == "" {
		t.Errorf("mismatch entry = %+v", mismatch)
	}

	unresolved := res.Unresolved()
	if len(unresolved) != 2 {
		t.Errorf("Unresolved() = %v, want the two bad ids", unresolved)
	}
	if got := res.SafeToDelete(); len(got) != 0 {
		t.Errorf("SafeToDelete() = %v, want none — everything is referenced or unresolved", got)
	}
}

func TestReferencesSafeToDelete(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{
		"free-1": {"records":{"total":0,"ids":[],"limit":50},"budgets":{"total":0,"ids":[],"limit":50}},
		"free-2": {"records":{"total":0,"ids":[],"limit":50}},
		"busy":   {"records":{"total":7,"ids":["r1"],"limit":50,"hasMore":false}}
	}`})
	c := newTestClient(t, m)
	res, err := c.References(testCtx(t), &ReferencesParams{Type: EntityLabels, IDs: []string{"free-1", "free-2", "busy"}})
	if err != nil {
		t.Fatalf("References: %v", err)
	}
	got := res.SafeToDelete()
	sort.Strings(got)
	if len(got) != 2 || got[0] != "free-1" || got[1] != "free-2" {
		t.Errorf("SafeToDelete() = %v, want the two unreferenced ids", got)
	}
}

func TestReferencesNormalisesSingularTypes(t *testing.T) {
	for singular, plural := range map[EntityType]EntityType{
		"category": EntityCategories,
		"record":   EntityRecords,
		"account":  EntityAccounts,
		"label":    EntityLabels,
	} {
		singular, plural := singular, plural
		t.Run(string(singular), func(t *testing.T) {
			m := newMockServer(t, mockResponse{Body: `{}`})
			c := newTestClient(t, m)
			if _, err := c.References(testCtx(t), &ReferencesParams{Type: singular, IDs: []string{"x"}}); err != nil {
				t.Fatalf("References: %v", err)
			}
			if got, want := m.Last(t).Path, "/v1/api/"+string(plural)+"/references"; got != want {
				t.Errorf("path = %q, want the canonical plural %q", got, want)
			}
		})
	}
}

func TestReferencesValidation(t *testing.T) {
	tests := []struct {
		name   string
		params *ReferencesParams
		field  string
	}{
		{"nil", nil, ""},
		{"no ids", &ReferencesParams{Type: EntityRecords}, "id"},
		{"too many ids", &ReferencesParams{Type: EntityRecords, IDs: makeIDs(21)}, "id"},
		{"missing type", &ReferencesParams{IDs: []string{"r1"}}, "type"},
		{"unreferenceable budgets", &ReferencesParams{Type: EntityBudgets, IDs: []string{"b1"}}, "type"},
		{"unreferenceable standing orders", &ReferencesParams{Type: EntityStandingOrders, IDs: []string{"s1"}}, "type"},
		{"unreferenceable record rules", &ReferencesParams{Type: EntityRecordRules, IDs: []string{"r1"}}, "type"},
		{"board-only orders", &ReferencesParams{Type: EntityOrders, IDs: []string{"o1"}}, "type"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := mustInvalidRequest(t, tc.params.Validate())
			if tc.field != "" {
				wantField(t, errs, tc.field)
			}
		})
	}
}

func TestReferencesAcceptsExactlyTwentyIDs(t *testing.T) {
	p := &ReferencesParams{Type: EntityRecords, IDs: makeIDs(20)}
	if err := p.Validate(); err != nil {
		t.Errorf("20 ids is the documented ceiling and must be accepted: %v", err)
	}
}

func TestNeedsReferenceCheck(t *testing.T) {
	tests := map[EntityType]bool{
		EntityCategories:     true,
		EntityRecords:        true,
		EntityAccounts:       true,
		EntityLabels:         true,
		EntityBudgets:        false,
		EntityStandingOrders: false,
		EntityRecordRules:    false,
	}
	for et, want := range tests {
		if got := et.NeedsReferenceCheck(); got != want {
			t.Errorf("%s.NeedsReferenceCheck() = %v, want %v", et, got, want)
		}
	}
}

func TestDeleteUnreferencedSkipsReferencedIDs(t *testing.T) {
	m := newMockHandler(t, func(w http.ResponseWriter, r *http.Request, n int) {
		if strings.HasSuffix(r.URL.Path, "/references") {
			writeMock(w, mockResponse{Body: `{
				"free":   {"records":{"total":0,"ids":[],"limit":50}},
				"busy":   {"records":{"total":2,"ids":["r1","r2"],"limit":50},"budgets":{"total":1,"ids":["b1"],"limit":50}},
				"absent": {"error":"not found"}
			}`})
			return
		}
		writeMock(w, mockResponse{Status: 200, Body: `{
			"results":[{"inputIndex":0,"id":"free","success":true}],
			"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":1}}`})
	})
	c := newTestClient(t, m)
	resp, refs, skipped, err := c.DeleteUnreferenced(testCtx(t), EntityLabels, []string{"free", "busy", "absent"})
	if err != nil {
		t.Fatalf("DeleteUnreferenced: %v", err)
	}
	if refs == nil {
		t.Fatal("the references consulted should be returned")
	}
	if resp == nil || resp.Summary.Succeeded != 1 {
		t.Fatalf("delete response = %+v", resp)
	}
	if len(skipped) != 2 {
		t.Fatalf("skipped = %v, want two entries", skipped)
	}
	if !strings.Contains(skipped["busy"], "records") || !strings.Contains(skipped["busy"], "budgets") {
		t.Errorf("skipped[busy] = %q, want it to name both referencing kinds", skipped["busy"])
	}
	if skipped["absent"] != "not found" {
		t.Errorf("skipped[absent] = %q, want the server's reason", skipped["absent"])
	}
	// Only the free ID should have reached the delete.
	deleteReq := m.Requests()[1]
	if !strings.Contains(string(deleteReq.Body), `"ids":["free"]`) {
		t.Errorf("delete body = %s, want only the unreferenced id", deleteReq.Body)
	}
}

func TestDeleteUnreferencedSendsNoDeleteWhenNothingIsFree(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{"busy":{"records":{"total":1,"ids":["r1"],"limit":50}}}`})
	c := newTestClient(t, m)
	resp, refs, skipped, err := c.DeleteUnreferenced(testCtx(t), EntityLabels, []string{"busy"})
	if err != nil {
		t.Fatalf("DeleteUnreferenced: %v", err)
	}
	if resp != nil {
		t.Errorf("no delete should have been issued, got %+v", resp)
	}
	if refs == nil || len(skipped) != 1 {
		t.Errorf("refs/skipped = %v / %v", refs, skipped)
	}
	if m.Count() != 1 {
		t.Errorf("requests = %d, want just the references check", m.Count())
	}
}

func TestDeleteUnreferencedSkipsTheCheckForUnreferenceableTypes(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 200, Body: `{
		"results":[{"inputIndex":0,"id":"b1","success":true}],
		"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":1}}`})
	c := newTestClient(t, m)
	resp, refs, skipped, err := c.DeleteUnreferenced(testCtx(t), EntityBudgets, []string{"b1"})
	if err != nil {
		t.Fatalf("DeleteUnreferenced: %v", err)
	}
	if refs != nil || skipped != nil {
		t.Errorf("nothing references a budget, so no check should have run: %v / %v", refs, skipped)
	}
	if resp == nil || m.Count() != 1 {
		t.Errorf("expected exactly one request, the delete; got %d", m.Count())
	}
	if got, want := m.Last(t).Path, "/v1/api/budgets"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

func TestDeleteUnreferencedPropagatesTheReferencesError(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 500, Body: `{"error":"boom"}`})
	c := newTestClient(t, m, func(cfg *Config) { cfg.Retry = RetryConfig{MaxRetries: -1} })
	resp, refs, skipped, err := c.DeleteUnreferenced(testCtx(t), EntityRecords, []string{"r1"})
	mustAPIError(t, err, KindServer)
	if resp != nil || refs != nil || skipped != nil {
		t.Errorf("a failed check must return nothing usable: %v / %v / %v", resp, refs, skipped)
	}
}

func TestReferencesResultMarshalsBackToABareMap(t *testing.T) {
	res := ReferencesResult{Entries: map[string]ReferencesResponse{
		"a": {Records: &ReferenceResult{Total: 1, IDs: []string{"r1"}, Limit: 50, Field: "categoryId"}},
	}}
	got, err := res.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if !strings.HasPrefix(string(got), `{"a":`) {
		t.Errorf("marshalled = %s, want a bare map keyed by id", got)
	}
	empty := ReferencesResult{}
	got, err = empty.MarshalJSON()
	if err != nil || string(got) != "{}" {
		t.Errorf("empty marshalled = %s (%v), want {}", got, err)
	}
}
