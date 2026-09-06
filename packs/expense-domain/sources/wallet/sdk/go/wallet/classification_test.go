package wallet

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Categories
// ---------------------------------------------------------------------------

func TestListCategoriesSerialisesEveryParameter(t *testing.T) {
	m := newMockServer(t, mockResponse{JSONBody: map[string]any{"categories": []any{}}})
	c := newTestClient(t, m)
	card := CardinalityMust
	if _, err := c.ListCategories(testCtx(t), &ListCategoriesParams{
		Limit:          Int(100),
		Offset:         Int(0),
		AgentHints:     Bool(false),
		WithTotal:      Bool(true),
		IDs:            []string{"c1"},
		Name:           ptrFilter(Eq("Groceries")),
		CreatedAt:      []Filter{Gte("2024-01-01T00:00:00Z")},
		UpdatedAt:      []Filter{Lt("2025-01-01T00:00:00Z")},
		CustomCategory: Bool(true),
		Cardinality:    &card,
		Archived:       Bool(false),
		BudgetID:       String("b1"),
		SortBy:         ptrSort(Asc("updatedAt")),
	}); err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	q := m.Last(t).Query
	checks := map[string]string{
		"limit": "100", "offset": "0", "agentHints": "false", "withTotal": "true",
		"id": "c1", "name": "eq.Groceries", "customCategory": "true",
		"cardinality": "must", "archived": "false", "budgetId": "b1", "sortBy": "+updatedAt",
	}
	for key, want := range checks {
		if got := q.Get(key); got != want {
			t.Errorf("query %q = %q, want %q", key, got, want)
		}
	}
}

func TestListCategoriesDecodesSystemAndCustomShapes(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{"categories":[
		{"id":"5c5c07d0-0014-8000-8000-000000000000","name":"Groceries","systemId":"food_and_drinks__groceries",
		 "customCategory":false,"customName":false,"enabled":true,"color":"#26c6da","cardinality":"need",
		 "group":{"id":"food_and_drinks","name":"Food & Drinks"},
		 "createdAt":null,"updatedAt":null},
		{"id":"custom-1","name":"Weekend Dining","customCategory":true,"customName":true,
		 "parentId":"5c5c07d0-0014-8000-8000-000000000000","parentName":"Groceries",
		 "cardinality":"want","archived":true,
		 "createdAt":"2024-05-01T10:00:00Z","updatedAt":"2024-06-01T10:00:00Z"}
	]}`})
	c := newTestClient(t, m)
	page, err := c.ListCategories(testCtx(t), nil)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	system, custom := page.Categories[0], page.Categories[1]
	if system.SystemID != "food_and_drinks__groceries" || system.CustomCategory {
		t.Errorf("system category = %+v", system)
	}
	if system.Group == nil || system.Group.Name != "Food & Drinks" {
		t.Errorf("group = %+v", system.Group)
	}
	// A system category's timestamps are nullable and must decode to zero.
	if system.CreatedAt.Valid() || system.UpdatedAt.Valid() {
		t.Errorf("null timestamps should decode to zero, got %v/%v", system.CreatedAt, system.UpdatedAt)
	}
	if custom.ParentID == "" || custom.ParentName != "Groceries" || !custom.Archived {
		t.Errorf("custom category = %+v", custom)
	}
	if custom.Cardinality != CardinalityWant {
		t.Errorf("cardinality = %q, want want", custom.Cardinality)
	}
}

func TestFixedCategoryIDsMatchTheRegistry(t *testing.T) {
	fixed := map[string]string{
		"Unknown income":  CategoryUnknownIncome,
		"Unknown expense": CategoryUnknownExpense,
		"Uncategorized":   CategoryUncategorized,
		"Transfer":        CategoryTransfer,
	}
	want := map[string]string{
		"Unknown income":  "5c5c32c8-0082-8000-8000-000000000000",
		"Unknown expense": "5c5c32c9-0082-8000-8000-000000000000",
		"Uncategorized":   "5c5c4e23-00c8-8000-8000-000000000000",
		"Transfer":        "5c5c4e21-00c8-8000-8000-000000000000",
	}
	for name, got := range fixed {
		if got != want[name] {
			t.Errorf("%s = %q, want %q", name, got, want[name])
		}
	}
}

func TestCreateSubcategoryHappyPath(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 201, Body: `{
		"category":{"id":"c9","name":"Weekend Dining","customCategory":true,"parentId":"p1"},
		"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":1}
	}`})
	c := newTestClient(t, m)
	card := CardinalityWant
	colour := ColorRed
	resp, err := c.CreateSubcategory(testCtx(t), &CreateSubcategoryRequest{
		Name: "Weekend Dining", ParentID: "p1", Cardinality: &card, Color: &colour,
	})
	if err != nil {
		t.Fatalf("CreateSubcategory: %v", err)
	}
	if resp.Category == nil || !resp.Category.CustomCategory {
		t.Fatalf("category = %+v", resp.Category)
	}
	if got, want := m.Last(t).Path, "/v1/api/categories/custom"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	body := string(m.Last(t).Body)
	if !strings.Contains(body, `"cardinality":"want"`) || !strings.Contains(body, `"color":"Red"`) {
		t.Errorf("body = %s", body)
	}
}

func TestCreateSubcategoryValidation(t *testing.T) {
	tests := []struct {
		name  string
		body  *CreateSubcategoryRequest
		field string
	}{
		{"nil", nil, ""},
		{"missing name", &CreateSubcategoryRequest{ParentID: "p1"}, "name"},
		{"name too long", &CreateSubcategoryRequest{Name: makeString(81), ParentID: "p1"}, "name"},
		{"missing parent", &CreateSubcategoryRequest{Name: "n"}, "parentId"},
		{"bad cardinality", &CreateSubcategoryRequest{Name: "n", ParentID: "p", Cardinality: ptrCardinality("maybe")}, "cardinality"},
		{"bad colour", &CreateSubcategoryRequest{Name: "n", ParentID: "p", Color: ptrColor("Beige")}, "color"},
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

func TestPatchCategoryResetAndSetConflict(t *testing.T) {
	tests := []struct {
		name string
		item PatchCategoryRequest
	}{
		{"name", PatchCategoryRequest{ID: "c1", Name: String("x"), Reset: []ResettableCategoryField{ResetName}}},
		{"cardinality", PatchCategoryRequest{ID: "c1", Cardinality: ptrCardinality(CardinalityMust), Reset: []ResettableCategoryField{ResetCardinality}}},
		{"colour", PatchCategoryRequest{ID: "c1", Color: ptrColor(ColorRed), Reset: []ResettableCategoryField{ResetColor}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := mustInvalidRequest(t, tc.item.Validate())
			wantField(t, errs, "reset[0]")
		})
	}
}

func TestPatchCategoryValidation(t *testing.T) {
	tests := []struct {
		name  string
		items []PatchCategoryRequest
		field string
	}{
		{"empty batch", nil, "items"},
		{"too many items", makePatchCategories(11), "items"},
		{"missing id", []PatchCategoryRequest{{Name: String("x")}}, "items[0].id"},
		{"nothing to do", []PatchCategoryRequest{{ID: "c1"}}, "items[0]"},
		{"unknown reset field", []PatchCategoryRequest{{ID: "c1", Reset: []ResettableCategoryField{"parentId"}}}, "items[0].reset[0]"},
		{"duplicate reset field", []PatchCategoryRequest{{ID: "c1", Reset: []ResettableCategoryField{ResetName, ResetName}}}, "items[0].reset[1]"},
		{"bad cardinality", []PatchCategoryRequest{{ID: "c1", Cardinality: ptrCardinality("maybe")}}, "items[0].cardinality"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := mustInvalidRequest(t, patchCategoriesBody(tc.items).Validate())
			wantField(t, errs, tc.field)
		})
	}
}

func TestPatchCategoriesSendsResetArray(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 200, Body: `{"results":[{"inputIndex":0,"id":"c1","success":true}],
		"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":1}}`})
	c := newTestClient(t, m)
	if _, err := c.PatchCategories(testCtx(t),
		[]PatchCategoryRequest{{ID: "c1", Reset: []ResettableCategoryField{ResetName, ResetColor}}},
		&PatchCategoriesParams{ReturnData: Bool(true)}); err != nil {
		t.Fatalf("PatchCategories: %v", err)
	}
	if !strings.Contains(string(m.Last(t).Body), `"reset":["name","color"]`) {
		t.Errorf("body = %s", m.Last(t).Body)
	}
}

func TestCardinalityNoneClearsRatherThanRejects(t *testing.T) {
	item := PatchCategoryRequest{ID: "c1", Cardinality: ptrCardinality(CardinalityNone)}
	if err := item.Validate(); err != nil {
		t.Errorf(`cardinality "none" clears the value and must be accepted: %v`, err)
	}
}

// ---------------------------------------------------------------------------
// Labels
// ---------------------------------------------------------------------------

func TestListLabelsSerialisesEveryParameter(t *testing.T) {
	m := newMockServer(t, mockResponse{JSONBody: map[string]any{"labels": []any{}}})
	c := newTestClient(t, m)
	if _, err := c.ListLabels(testCtx(t), &ListLabelsParams{
		Limit:           Int(20),
		IDs:             []string{"l1"},
		Name:            ptrFilter(ContainsI("vac")),
		CreatedAt:       []Filter{Gte("2024-01-01T00:00:00Z")},
		UpdatedAt:       []Filter{Lte("2025-01-01T00:00:00Z")},
		Archived:        Bool(true),
		RecordID:        String("r1"),
		BudgetID:        String("b1"),
		StandingOrderID: String("so1"),
		RecordRuleID:    String("rr1"),
		SortBy:          ptrSort(Desc("updatedAt")),
	}); err != nil {
		t.Fatalf("ListLabels: %v", err)
	}
	q := m.Last(t).Query
	checks := map[string]string{
		"limit": "20", "id": "l1", "name": "contains-i.vac", "archived": "true",
		"recordId": "r1", "budgetId": "b1", "standingOrderId": "so1",
		"recordRuleId": "rr1", "sortBy": "-updatedAt",
	}
	for key, want := range checks {
		if got := q.Get(key); got != want {
			t.Errorf("query %q = %q, want %q", key, got, want)
		}
	}
}

func TestCreateLabelHappyPath(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 201, Body: `{
		"label":{"id":"l1","name":"vacation","color":"#d32f2f","archived":false},
		"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":1}
	}`})
	c := newTestClient(t, m)
	colour := ColorBlue
	resp, err := c.CreateLabel(testCtx(t), &CreateLabelRequest{Name: "vacation", Color: &colour})
	if err != nil {
		t.Fatalf("CreateLabel: %v", err)
	}
	if resp.Label == nil || resp.Label.ID != "l1" {
		t.Fatalf("label = %+v", resp.Label)
	}
	// The API returns hex colours even though the write side takes names.
	if resp.Label.Color != "#d32f2f" {
		t.Errorf("colour = %q, want the hex form the read side uses", resp.Label.Color)
	}
}

func TestCreateLabelValidation(t *testing.T) {
	tests := []struct {
		name  string
		body  *CreateLabelRequest
		field string
	}{
		{"nil", nil, ""},
		{"missing name", &CreateLabelRequest{}, "name"},
		{"blank name", &CreateLabelRequest{Name: "   "}, "name"},
		{"name too long", &CreateLabelRequest{Name: makeString(81)}, "name"},
		{"bad colour", &CreateLabelRequest{Name: "n", Color: ptrColor("Beige")}, "color"},
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

func TestPatchLabelsValidation(t *testing.T) {
	tests := []struct {
		name  string
		items []PatchLabelItem
		field string
	}{
		{"empty batch", nil, "items"},
		{"missing id", []PatchLabelItem{{Name: String("x")}}, "items[0].id"},
		{"nothing to do", []PatchLabelItem{{ID: "l1"}}, "items[0]"},
		{"name too long", []PatchLabelItem{{ID: "l1", Name: String(makeString(81))}}, "items[0].name"},
		{"bad colour", []PatchLabelItem{{ID: "l1", Color: ptrColor("Beige")}}, "items[0].color"},
		{"duplicate id", []PatchLabelItem{{ID: "l1", Archived: Bool(true)}, {ID: "l1", Archived: Bool(false)}}, "items[1].id"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := mustInvalidRequest(t, patchLabelsBody(tc.items).Validate())
			wantField(t, errs, tc.field)
		})
	}
}

func TestPatchLabelsArchive(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 200, Body: `{
		"results":[{"inputIndex":0,"id":"l1","success":true,"label":{"id":"l1","archived":true}}],
		"summary":{"total":1,"succeeded":1,"clientErrors":0,"serverErrors":0,"documentsWritten":1}}`})
	c := newTestClient(t, m)
	resp, err := c.PatchLabels(testCtx(t), []PatchLabelItem{{ID: "l1", Archived: Bool(true)}}, nil)
	if err != nil {
		t.Fatalf("PatchLabels: %v", err)
	}
	if resp.Results[0].Label == nil || !resp.Results[0].Label.Archived {
		t.Errorf("label = %+v, want archived", resp.Results[0].Label)
	}
}

// ---------------------------------------------------------------------------
// Record rules
// ---------------------------------------------------------------------------

func TestListRecordRules(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{"recordRules":[{
		"id":"rr1","name":"Coffee","keywords":["starbucks","costa"],
		"category":{"id":"c1","name":"Coffee"},
		"labels":[{"id":"l1","name":"treat"}],
		"fromAccountId":"acc1","toAccountId":"acc2",
		"createdAt":"2024-01-01T00:00:00Z"}]}`})
	c := newTestClient(t, m)
	page, err := c.ListRecordRules(testCtx(t), &ListRecordRulesParams{
		LabelID: String("l1"), Name: ptrFilter(Contains("Cof")), Limit: Int(5),
	})
	if err != nil {
		t.Fatalf("ListRecordRules: %v", err)
	}
	rule := page.RecordRules[0]
	if len(rule.Keywords) != 2 || rule.Category == nil || len(rule.Labels) != 1 {
		t.Errorf("rule = %+v", rule)
	}
	if rule.FromAccountID != "acc1" || rule.ToAccountID != "acc2" {
		t.Errorf("rule accounts = %q/%q", rule.FromAccountID, rule.ToAccountID)
	}
	q := m.Last(t).Query
	if q.Get("labelId") != "l1" || q.Get("name") != "contains.Cof" || q.Get("limit") != "5" {
		t.Errorf("query = %v", q)
	}
	if got, want := m.Last(t).Path, "/v1/api/record-rules"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

func ptrCardinality(c Cardinality) *Cardinality { return &c }

func makePatchCategories(n int) []PatchCategoryRequest {
	out := make([]PatchCategoryRequest, n)
	ids := makeIDs(n)
	for i := range out {
		out[i] = PatchCategoryRequest{ID: ids[i], Name: String("x")}
	}
	return out
}
