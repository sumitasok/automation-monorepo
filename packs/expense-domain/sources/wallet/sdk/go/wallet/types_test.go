package wallet

import (
	"encoding/json"
	"testing"
	"time"
	"unicode/utf8"
)

func TestTimeAcceptsEveryFormatTheAPIEmits(t *testing.T) {
	tests := []struct {
		raw  string
		want time.Time
	}{
		{`"2021-03-24T09:28:41.200Z"`, time.Date(2021, 3, 24, 9, 28, 41, 200_000_000, time.UTC)},
		{`"2021-03-24T09:28:41Z"`, time.Date(2021, 3, 24, 9, 28, 41, 0, time.UTC)},
		{`"2021-03-24T09:28:41.123456789Z"`, time.Date(2021, 3, 24, 9, 28, 41, 123456789, time.UTC)},
		{`"2021-03-24T09:28:41+02:00"`, time.Date(2021, 3, 24, 9, 28, 41, 0, time.FixedZone("", 7200))},
		{`"2021-03-24T09:28:41"`, time.Date(2021, 3, 24, 9, 28, 41, 0, time.UTC)},
		{`"2021-03-24"`, time.Date(2021, 3, 24, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			var got Time
			if err := json.Unmarshal([]byte(tc.raw), &got); err != nil {
				t.Fatalf("Unmarshal(%s): %v", tc.raw, err)
			}
			if !got.Time.Equal(tc.want) {
				t.Errorf("= %v, want %v", got.Time, tc.want)
			}
			if !got.Valid() {
				t.Error("Valid() = false for a present timestamp")
			}
		})
	}
}

func TestTimeNullAndEmptyDecodeToZero(t *testing.T) {
	for _, raw := range []string{`null`, `""`, `"   "`} {
		var got Time
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			t.Fatalf("Unmarshal(%s): %v", raw, err)
		}
		if got.Valid() {
			t.Errorf("%s should decode to a zero Time, got %v", raw, got.Time)
		}
		if got.String() != "" {
			t.Errorf("String() = %q, want empty for a zero Time", got.String())
		}
	}
}

func TestTimeRejectsUnparseableValues(t *testing.T) {
	for _, raw := range []string{`"yesterday"`, `"24/03/2021"`, `12345`, `{"t":1}`} {
		var got Time
		if err := json.Unmarshal([]byte(raw), &got); err == nil {
			t.Errorf("Unmarshal(%s) should fail, got %v", raw, got.Time)
		}
	}
}

func TestTimeMarshalsCanonically(t *testing.T) {
	tm := NewTime(time.Date(2021, 3, 24, 9, 28, 41, 200_000_000, time.FixedZone("", 7200)))
	got, err := json.Marshal(tm)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Always UTC, always millisecond precision.
	if string(got) != `"2021-03-24T07:28:41.200Z"` {
		t.Errorf("marshalled = %s, want the UTC millisecond form", got)
	}
	zero, err := json.Marshal(Time{})
	if err != nil {
		t.Fatalf("Marshal(zero): %v", err)
	}
	if string(zero) != "null" {
		t.Errorf("a zero Time should marshal to null, got %s", zero)
	}
}

func TestTimeRoundTrips(t *testing.T) {
	original := NewTime(time.Date(2024, 6, 15, 12, 30, 45, 500_000_000, time.UTC))
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Time
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !decoded.Time.Equal(original.Time) {
		t.Errorf("round trip changed %v into %v", original.Time, decoded.Time)
	}
}

func TestDateKeepsOnlyTheCalendarDay(t *testing.T) {
	tests := map[string]string{
		`"2025-06-15"`:               "2025-06-15",
		`"2025-06-15T23:45:00Z"`:     "2025-06-15",
		`"2025-06-15T00:00:00.000Z"`: "2025-06-15",
	}
	for raw, want := range tests {
		var got Date
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			t.Fatalf("Unmarshal(%s): %v", raw, err)
		}
		if got.String() != want {
			t.Errorf("Unmarshal(%s) = %q, want %q", raw, got.String(), want)
		}
	}
}

func TestDateNullAndMarshalling(t *testing.T) {
	var got Date
	if err := json.Unmarshal([]byte(`null`), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Valid() || got.String() != "" {
		t.Errorf("null should decode to a zero Date, got %v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil || string(encoded) != "null" {
		t.Errorf("zero Date marshalled to %s (%v), want null", encoded, err)
	}
	d := NewDate(2026, time.January, 1)
	encoded, err = json.Marshal(d)
	if err != nil || string(encoded) != `"2026-01-01"` {
		t.Errorf("Date marshalled to %s (%v), want \"2026-01-01\"", encoded, err)
	}
}

func TestDateRejectsUnparseableValues(t *testing.T) {
	var got Date
	if err := json.Unmarshal([]byte(`"the fifteenth"`), &got); err == nil {
		t.Error("an unparseable date should fail")
	}
}

// ---------------------------------------------------------------------------
// Filters
// ---------------------------------------------------------------------------

func TestFilterWireForms(t *testing.T) {
	tests := map[string]Filter{
		"eq.x":         Eq("x"),
		"gt.x":         Gt("x"),
		"gte.x":        Gte("x"),
		"lt.x":         Lt("x"),
		"lte.x":        Lte("x"),
		"contains.x":   Contains("x"),
		"contains-i.x": ContainsI("x"),
	}
	for want, f := range tests {
		if got := f.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}

func TestFilterOperatorClassification(t *testing.T) {
	tests := []struct {
		op            FilterOp
		isRange, text bool
	}{
		{OpEq, true, true},
		{OpGt, true, false},
		{OpGte, true, false},
		{OpLt, true, false},
		{OpLte, true, false},
		{OpContains, false, true},
		{OpContainsI, false, true},
		{"nope", false, false},
	}
	for _, tc := range tests {
		if got := tc.op.IsRange(); got != tc.isRange {
			t.Errorf("%s.IsRange() = %v, want %v", tc.op, got, tc.isRange)
		}
		if got := tc.op.IsText(); got != tc.text {
			t.Errorf("%s.IsText() = %v, want %v", tc.op, got, tc.text)
		}
	}
}

func TestParseFilterRoundTrips(t *testing.T) {
	for _, raw := range []string{"eq.x", "gt.1", "gte.2024-01-01", "lt.2024-02-01", "lte.9",
		"contains.hello world", "contains-i.HELLO"} {
		f, err := ParseFilter(raw)
		if err != nil {
			t.Fatalf("ParseFilter(%q): %v", raw, err)
		}
		if f.String() != raw {
			t.Errorf("round trip of %q produced %q", raw, f.String())
		}
	}
}

func TestParseFilterPrefersTheLongestPrefix(t *testing.T) {
	// "contains-i." starts with neither "contains." nor "eq.", but a naive
	// split on the first dot would mangle a value that contains one.
	f, err := ParseFilter("contains-i.a.b.c")
	if err != nil {
		t.Fatalf("ParseFilter: %v", err)
	}
	if f.Op != OpContainsI || f.Value != "a.b.c" {
		t.Errorf("parsed = %+v, want contains-i with value a.b.c", f)
	}
	f, err = ParseFilter("gte.2024-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("ParseFilter: %v", err)
	}
	if f.Op != OpGte || f.Value != "2024-01-01T00:00:00Z" {
		t.Errorf("parsed = %+v", f)
	}
}

func TestParseFilterRejectsAMissingPrefix(t *testing.T) {
	for _, raw := range []string{"bare", "unknown.x", "", "2024-01-01"} {
		if _, err := ParseFilter(raw); err == nil {
			t.Errorf("ParseFilter(%q) should fail", raw)
		}
	}
}

func TestMustParseFilterPanicsOnABadPrefix(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustParseFilter should panic on a missing prefix")
		}
	}()
	MustParseFilter("bare")
}

func TestBetweenBuildsAHalfOpenRange(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	got := Between(from, to)
	if len(got) != 2 || got[0].String() != "gte.2024-01-01T00:00:00Z" || got[1].String() != "lt.2024-04-01T00:00:00Z" {
		t.Errorf("Between = %v, want gte/lt", got)
	}
	gotDates := BetweenDates(from, to)
	if gotDates[0].String() != "gte.2024-01-01" || gotDates[1].String() != "lt.2024-04-01" {
		t.Errorf("BetweenDates = %v", gotDates)
	}
	// OnDate uses eq., which the API expands to the whole UTC day.
	onDate := OnDate(from)
	if len(onDate) != 1 || onDate[0].String() != "eq.2024-01-01" {
		t.Errorf("OnDate = %v, want a single eq. spanning the day", onDate)
	}
}

func TestSortWireForms(t *testing.T) {
	if got := Asc("recordDate").String(); got != "+recordDate" {
		t.Errorf("Asc = %q", got)
	}
	if got := Desc("amount").String(); got != "-amount" {
		t.Errorf("Desc = %q", got)
	}
}

func TestValidateSortRejectsAnEmptyField(t *testing.T) {
	s := Sort{}
	if err := validateSort("sortBy", &s, CommonSortFields); err == nil {
		t.Error("an empty sort field should be rejected")
	}
	if err := validateSort("sortBy", nil, CommonSortFields); err != nil {
		t.Errorf("a nil sort is simply unset: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

func TestEnumValidity(t *testing.T) {
	for _, v := range AllAccountTypes {
		if !v.Valid() {
			t.Errorf("AccountType %q should be valid", v)
		}
	}
	for _, v := range AllUniversalColors {
		if !v.Valid() {
			t.Errorf("UniversalColor %q should be valid", v)
		}
	}
	for _, v := range AllCardinalities {
		if !v.Valid() {
			t.Errorf("Cardinality %q should be valid", v)
		}
	}
	for _, v := range AllBudgetTypes {
		if !v.Valid() {
			t.Errorf("BudgetType %q should be valid", v)
		}
	}
	for _, v := range AllRecordStates {
		if !v.Valid() {
			t.Errorf("RecordState %q should be valid", v)
		}
	}
	for _, v := range AllRecordSources {
		if !v.Valid() {
			t.Errorf("RecordSource %q should be valid", v)
		}
	}
	for _, v := range AllPairingModes {
		if !v.Valid() {
			t.Errorf("PairingMode %q should be valid", v)
		}
	}
	for _, v := range AllReminders {
		if !v.Valid() {
			t.Errorf("Reminder %q should be valid", v)
		}
	}
	for _, v := range AllGoalStates {
		if !v.Valid() {
			t.Errorf("GoalState %q should be valid", v)
		}
	}
	for _, v := range AllValidationModes {
		if !v.Valid() {
			t.Errorf("ValidationMode %q should be valid", v)
		}
	}
	for _, v := range AllClearableRecordFields {
		if !v.Valid() {
			t.Errorf("ClearableRecordField %q should be valid", v)
		}
	}
	for _, v := range AllResettableCategoryFields {
		if !v.Valid() {
			t.Errorf("ResettableCategoryField %q should be valid", v)
		}
	}
}

func TestEnumInvalidity(t *testing.T) {
	if AccountType("Crypto").Valid() {
		t.Error("an unknown account type should be invalid")
	}
	if UniversalColor("Beige").Valid() {
		t.Error("an unknown colour should be invalid")
	}
	if Cardinality("maybe").Valid() {
		t.Error("an unknown cardinality should be invalid")
	}
	if RecordSource("smartwatch").Valid() {
		t.Error("an unknown source should be invalid")
	}
	if ClearableRecordField("amount").Valid() {
		t.Error("amount is not clearable")
	}
	if ResettableCategoryField("parentId").Valid() {
		t.Error("parentId is not resettable")
	}
}

func TestEntityTypeNormalisation(t *testing.T) {
	tests := map[EntityType]EntityType{
		"category": EntityCategories,
		"CATEGORY": EntityCategories,
		"record":   EntityRecords,
		"account":  EntityAccounts,
		"label":    EntityLabels,
		"order":    EntityOrders,
		"records":  EntityRecords,
		"budgets":  EntityBudgets,
		"nonsense": "nonsense",
	}
	for in, want := range tests {
		if got := NormalizeEntityType(in); got != want {
			t.Errorf("NormalizeEntityType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEntityTypeDeletable(t *testing.T) {
	for _, et := range DeletableEntityTypes {
		if !et.Deletable() {
			t.Errorf("%s should be deletable", et)
		}
	}
	if EntityType("widgets").Deletable() {
		t.Error("an unknown collection should not be deletable")
	}
}

func TestRecordStateWritability(t *testing.T) {
	for _, s := range WritableRecordStates {
		if !s.Writable() {
			t.Errorf("%s should be writable", s)
		}
	}
	for _, s := range []RecordState{RecordStateVoid, RecordStateWaitForAssign} {
		if s.Writable() {
			t.Errorf("%s is output-only and must not be writable", s)
		}
		if !s.Valid() {
			t.Errorf("%s should still be a valid value to read", s)
		}
	}
}

// ---------------------------------------------------------------------------
// Summaries and helpers
// ---------------------------------------------------------------------------

func TestBatchSummaryHelpers(t *testing.T) {
	tests := []struct {
		name                      string
		s                         BatchSummary
		failed                    int
		allOK, allFailed, partial bool
	}{
		{"all ok", BatchSummary{Total: 3, Succeeded: 3}, 0, true, false, false},
		{"all client errors", BatchSummary{Total: 3, ClientErrors: 3}, 3, false, true, false},
		{"all server errors", BatchSummary{Total: 2, ServerErrors: 2}, 2, false, true, false},
		{"mixed", BatchSummary{Total: 3, Succeeded: 1, ClientErrors: 1, ServerErrors: 1}, 2, false, false, true},
		{"empty", BatchSummary{}, 0, false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.Failed(); got != tc.failed {
				t.Errorf("Failed() = %d, want %d", got, tc.failed)
			}
			if got := tc.s.AllSucceeded(); got != tc.allOK {
				t.Errorf("AllSucceeded() = %v, want %v", got, tc.allOK)
			}
			if got := tc.s.AllFailed(); got != tc.allFailed {
				t.Errorf("AllFailed() = %v, want %v", got, tc.allFailed)
			}
			if got := tc.s.Partial(); got != tc.partial {
				t.Errorf("Partial() = %v, want %v", got, tc.partial)
			}
		})
	}
}

func TestItemErrorClassification(t *testing.T) {
	client := ItemError{ErrorType: ClientError}
	server := ItemError{ErrorType: ServerError}
	if !client.IsClientError() || client.IsServerError() {
		t.Error("client error misclassified")
	}
	if !server.IsServerError() || server.IsClientError() {
		t.Error("server error misclassified")
	}
	none := ItemError{}
	if none.IsClientError() || none.IsServerError() {
		t.Error("a successful item has no classification")
	}
}

func TestPaginationHasMore(t *testing.T) {
	next := 30
	if !(Pagination{NextOffset: &next}).HasMore() {
		t.Error("a nextOffset means more pages")
	}
	if (Pagination{}).HasMore() {
		t.Error("no nextOffset means the last page")
	}
}

func TestPointerHelpers(t *testing.T) {
	if *Int(5) != 5 || *Bool(true) != true || *Float(1.5) != 1.5 || *String("x") != "x" {
		t.Error("pointer helpers do not round-trip their values")
	}
}

func TestTransferOutputPaired(t *testing.T) {
	paired := TransferOutput{TransferID: "t1", MirrorRecord: &MirrorRecordEmbed{ID: "r2"}}
	unpaired := TransferOutput{TransferID: "t1"}
	severed := TransferOutput{TransferID: "t1", MirrorRecord: &MirrorRecordEmbed{}}
	if !paired.Paired() {
		t.Error("a transfer with a mirror is paired")
	}
	if unpaired.Paired() || severed.Paired() {
		t.Error("a transfer with no usable mirror is unpaired")
	}
}

func TestReferencesResponseTotals(t *testing.T) {
	entry := ReferencesResponse{
		Records:        &ReferenceResult{Total: 3},
		Budgets:        &ReferenceResult{Total: 1},
		StandingOrders: &ReferenceResult{Total: 0},
		Other:          &ReferenceResult{Total: 2},
	}
	if got := entry.TotalReferences(); got != 6 {
		t.Errorf("TotalReferences() = %d, want 6", got)
	}
	if !entry.Resolved() {
		t.Error("an entry with no error is resolved")
	}
	bad := ReferencesResponse{Error: "not found"}
	if bad.Resolved() || bad.TotalReferences() != 0 {
		t.Errorf("an unresolved entry = %+v", bad)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short = %q", got)
	}
	if got := truncate("hello world", 5); got != "hello…" {
		t.Errorf("truncate long = %q", got)
	}
}

func TestTruncateDoesNotSplitARune(t *testing.T) {
	// A byte-wise cut through "€" would yield invalid UTF-8 in an error message.
	s := "aa€bb"
	got := truncate(s, 3)
	if !utf8.ValidString(got) {
		t.Errorf("truncate produced invalid UTF-8: %q", got)
	}
	if got != "aa…" {
		t.Errorf("truncate = %q, want %q", got, "aa…")
	}
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("a short string should pass through, got %q", got)
	}
}
