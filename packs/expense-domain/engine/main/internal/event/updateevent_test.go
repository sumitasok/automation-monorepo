package event

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

var txnHeader = []string{
	"MessageID", "TxnDate", "Type", "Amount", "Merchant", "Info", "Subject",
	"BankFrom", "Category", "SubCategory", "UserComment",
}

func txnRow(id, merchant, category, comment string) []string {
	return []string{id, "2026-06-26", "Debit", "500.00", merchant, "info", "subj", "bank", category, "", comment}
}

func writeTxnCSV(t *testing.T, path string, rows [][]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := csv.NewWriter(f)
	w.Write(txnHeader)
	for _, r := range rows {
		w.Write(r)
	}
	w.Flush()
	f.Close()
}

// recordingMatcher is a stub Matcher that returns canned results and records
// the Item each id was sent with, so tests can assert on Item.Comment.
type recordingMatcher struct {
	calls     int
	out       []MatchResult
	itemsByID map[string]Item
}

func (m *recordingMatcher) Name() string { return "stub" }
func (m *recordingMatcher) Match(_ context.Context, _ []EventRef, batch []Item) ([]MatchResult, error) {
	m.calls++
	if m.itemsByID == nil {
		m.itemsByID = make(map[string]Item)
	}
	for _, it := range batch {
		m.itemsByID[it.ID] = it
	}
	return m.out, nil
}

func newScratchPaths(t *testing.T) (csvPath, regPath, statePath string) {
	dir := t.TempDir()
	return filepath.Join(dir, "transactions.csv"), filepath.Join(dir, "events.json"), filepath.Join(dir, "state.json")
}

func loadStateFile(t *testing.T, path string) map[string]AssignmentEntry {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Assigned map[string]AssignmentEntry `json:"assigned"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return s.Assigned
}

// TestUpdateEventCommentAttachedToUnassignedRow (spec 003, US2): a comment on
// an unassigned row reaches the matcher as Item.Comment, and the resulting
// Source is suffixed "+comment"; a comment-free row keeps the plain
// "ai:<provider>" Source and an empty Item.Comment.
func TestUpdateEventCommentAttachedToUnassignedRow(t *testing.T) {
	csvPath, regPath, statePath := newScratchPaths(t)
	writeTxnCSV(t, csvPath, [][]string{
		txnRow("m1", "Goa Resort", "Travel", "Goa trip - day 2 dinner"),
		txnRow("m2", "Local Cafe", "Food & Drinks", ""),
	})

	stub := &recordingMatcher{out: []MatchResult{
		{ID: "m1", EventID: "", Confidence: 0, NewEventName: "Goa Trip", NewEventDescription: "A trip to Goa", NewEventKeywords: []string{"goa"}},
		{ID: "m2", EventID: "", Confidence: 0},
	}}

	_, err := Run(context.Background(), Config{
		CSVPath: csvPath, RegistryPath: regPath, StatePath: statePath, Matcher: stub,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := stub.itemsByID["m1"].Comment; got != "Goa trip - day 2 dinner" {
		t.Errorf("m1 Item.Comment = %q, want the comment text", got)
	}
	if got := stub.itemsByID["m2"].Comment; got != "" {
		t.Errorf("m2 Item.Comment = %q, want empty (comment-free row)", got)
	}

	assigned := loadStateFile(t, statePath)
	if got := assigned["m1"].Source; got != "ai:stub+comment" {
		t.Errorf("m1 Source = %q, want ai:stub+comment", got)
	}
	if got := assigned["m1"].Comment; got != "Goa trip - day 2 dinner" {
		t.Errorf("m1 Comment = %q, want the comment text", got)
	}
	if got := assigned["m2"].Source; got != "ai:stub" {
		t.Errorf("m2 Source = %q, want plain ai:stub (no +comment)", got)
	}
}

// TestUpdateEventAlreadyAssignedUntouchedWithoutCommentChange (spec 003, US4,
// FR-011): an already-assigned row with no comment, or an unchanged comment,
// is never re-sent to the matcher.
func TestUpdateEventAlreadyAssignedUntouchedWithoutCommentChange(t *testing.T) {
	csvPath, regPath, statePath := newScratchPaths(t)
	writeTxnCSV(t, csvPath, [][]string{
		txnRow("m1", "Local Cafe", "Food & Drinks", ""),
		txnRow("m2", "Goa Resort", "Travel", "same note"),
	})
	writeState(t, statePath, map[string]AssignmentEntry{
		"m1": {EventID: "", Confidence: 1.0, Source: "ai:stub"},
		"m2": {EventID: "trip-a", Confidence: 1.0, Source: "ai:stub+comment", Comment: "same note"},
	})

	stub := &recordingMatcher{}
	res, err := Run(context.Background(), Config{
		CSVPath: csvPath, RegistryPath: regPath, StatePath: statePath, Matcher: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 0 || stub.calls != 0 {
		t.Errorf("total=%d calls=%d, want 0/0 — neither row should be reprocessed", res.Total, stub.calls)
	}
}

// TestUpdateEventCommentAddedReclassifies (spec 003, US4, scenario 1): adding
// a comment to an already-assigned row re-opens it.
func TestUpdateEventCommentAddedReclassifies(t *testing.T) {
	csvPath, regPath, statePath := newScratchPaths(t)
	writeTxnCSV(t, csvPath, [][]string{
		txnRow("m1", "Goa Resort", "Travel", "actually this was a work offsite"),
	})
	writeState(t, statePath, map[string]AssignmentEntry{
		"m1": {EventID: "goa-trip", Confidence: 0.9, Source: "ai:stub"},
	})

	stub := &recordingMatcher{out: []MatchResult{
		{ID: "m1", EventID: "", Confidence: 0, NewEventName: "Work Offsite", NewEventDescription: "Company offsite"},
	}}
	res, err := Run(context.Background(), Config{
		CSVPath: csvPath, RegistryPath: regPath, StatePath: statePath, Matcher: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || stub.calls != 1 {
		t.Fatalf("total=%d calls=%d, want 1/1", res.Total, stub.calls)
	}
	assigned := loadStateFile(t, statePath)
	if got := assigned["m1"].Source; got != "ai:stub+comment" {
		t.Errorf("m1 Source = %q, want ai:stub+comment", got)
	}
	if got := assigned["m1"].EventID; got == "goa-trip" {
		t.Errorf("m1 EventID unchanged (%q) — expected reclassification to a new event", got)
	}
}

// TestUpdateEventCommentBypassesRoutineRule (spec 003, US4, FR-012, event
// side): a row with a non-empty comment is routed to the AI matcher even
// though a "routine" rule would otherwise have marked it not event-worthy.
func TestUpdateEventCommentBypassesRoutineRule(t *testing.T) {
	csvPath, regPath, statePath := newScratchPaths(t)
	writeTxnCSV(t, csvPath, [][]string{
		txnRow("m1", "HungerBox", "Food & Drinks", "this one was actually part of the Goa trip"),
		txnRow("m2", "HungerBox", "Food & Drinks", ""), // comment-free — rule should still apply
	})

	dir := filepath.Dir(csvPath)
	rulesPath := filepath.Join(dir, "expense-rules.yaml")
	const rulesYAML = `rules:
  - name: hungerbox-routine
    applies_to: [event]
    match:
      merchant_contains: ["hungerbox"]
    outcome:
      event_relevance: routine
`
	if err := os.WriteFile(rulesPath, []byte(rulesYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	stub := &recordingMatcher{out: []MatchResult{
		{ID: "m1", EventID: "", Confidence: 0, NewEventName: "Goa Trip", NewEventDescription: "A trip to Goa"},
	}}
	res, err := Run(context.Background(), Config{
		CSVPath: csvPath, RegistryPath: regPath, StatePath: statePath, RulesFile: rulesPath, Matcher: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.RuleDecided != 1 {
		t.Errorf("RuleDecided = %d, want 1 (only the comment-free row)", res.RuleDecided)
	}
	if stub.calls != 1 {
		t.Errorf("matcher calls=%d, want 1 (only the commented row)", stub.calls)
	}
	assigned := loadStateFile(t, statePath)
	if got := assigned["m1"].Source; got != "ai:stub+comment" {
		t.Errorf("m1 (commented) Source = %q, want ai:stub+comment — rule must not have decided it", got)
	}
	if got := assigned["m2"].Source; got != "rule:hungerbox-routine" {
		t.Errorf("m2 (comment-free) Source = %q, want the rule to have decided it", got)
	}
}

func writeState(t *testing.T, path string, assigned map[string]AssignmentEntry) {
	t.Helper()
	raw, err := json.Marshal(struct {
		Assigned map[string]AssignmentEntry `json:"assigned"`
	}{Assigned: assigned})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
