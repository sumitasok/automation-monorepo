package event

import (
	"testing"

	"github.com/sumitasok/sa.automation.expenses/internal/csvtxn"
)

func txn(id, txnDate, merchant string) csvtxn.Txn {
	return csvtxn.Txn{MessageID: id, TxnDate: txnDate, Merchant: merchant}
}

func TestSuggestCandidatesSameMerchantDifferentEvent(t *testing.T) {
	corr := correction{ID: "new1", Merchant: "HungerBox", PriorSource: "rule:hungerbox-routine", EventID: "goa-trip"}
	txns := []csvtxn.Txn{
		txn("old1", "2026-05-01", "HungerBox"), // candidate: same merchant + same rule, different outcome
		txn("old2", "2026-04-01", "HungerBox"), // already assigned to goa-trip — excluded
		txn("new1", "2026-06-01", "HungerBox"), // the corrected transaction itself — excluded
		txn("other", "2026-03-01", "Uber"),     // different merchant, not assigned — excluded
	}
	assigned := map[string]AssignmentEntry{
		"old1": {EventID: "", Source: "rule:hungerbox-routine"},
		"old2": {EventID: "goa-trip", Source: "ai:stub"},
	}

	got := suggestCandidates(txns, assigned, corr)
	if len(got) != 1 || got[0].Txn.MessageID != "old1" {
		t.Fatalf("suggestCandidates = %v, want exactly [old1]", candidateIDs(got))
	}
}

func TestSuggestCandidatesExcludesUnassignedTxns(t *testing.T) {
	corr := correction{ID: "new1", Merchant: "HungerBox", EventID: "goa-trip"}
	txns := []csvtxn.Txn{txn("old1", "2026-05-01", "HungerBox")}
	got := suggestCandidates(txns, map[string]AssignmentEntry{}, corr)
	if len(got) != 0 {
		t.Fatalf("suggestCandidates = %v, want none (transaction has no existing assignment)", candidateIDs(got))
	}
}

func TestSuggestCandidatesOrderedOldestFirst(t *testing.T) {
	corr := correction{ID: "new1", Merchant: "HungerBox", EventID: "goa-trip"}
	txns := []csvtxn.Txn{
		txn("mid", "2026-05-15", "HungerBox"),
		txn("newest", "2026-06-01", "HungerBox"),
		txn("oldest", "2026-01-01", "HungerBox"),
	}
	assigned := map[string]AssignmentEntry{
		"mid":    {EventID: "other-trip"},
		"newest": {EventID: "other-trip"},
		"oldest": {EventID: "other-trip"},
	}
	got := suggestCandidates(txns, assigned, corr)
	want := []string{"oldest", "mid", "newest"}
	if len(got) != len(want) {
		t.Fatalf("suggestCandidates = %v, want %v", candidateIDs(got), want)
	}
	for i, id := range want {
		if got[i].Txn.MessageID != id {
			t.Errorf("position %d = %s, want %s", i, got[i].Txn.MessageID, id)
		}
	}
}

func candidateIDs(cands []candidate) []string {
	out := make([]string, len(cands))
	for i, c := range cands {
		out[i] = c.Txn.MessageID
	}
	return out
}
