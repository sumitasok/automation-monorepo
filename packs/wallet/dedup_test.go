package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// TestDuplicateGroupMarshaling verifies DuplicateGroup can be marshaled to JSON.
func TestDuplicateGroupMarshaling(t *testing.T) {
	group := DuplicateGroup{
		DuplicateKey: "2026-08-25 | -1000 | Uber",
		MatchType:    "exact",
		Confidence:   1.0,
		Records: []RecordSummary{
			{
				ID:           "record-1",
				CreatedAt:    "2026-08-25T08:00:00Z",
				IsOriginal:   true,
				CounterParty: "Uber",
				Amount:       -1000,
				Category:     "Transport",
			},
		},
	}

	data, err := json.Marshal(group)
	if err != nil {
		t.Fatalf("Failed to marshal DuplicateGroup: %v", err)
	}

	// Verify it can be unmarshaled back
	var unmarshaled DuplicateGroup
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal DuplicateGroup: %v", err)
	}

	if unmarshaled.DuplicateKey != group.DuplicateKey {
		t.Errorf("Expected key %q, got %q", group.DuplicateKey, unmarshaled.DuplicateKey)
	}
}

// TestDedupDecisionMarshaling verifies DedupDecision can be marshaled to JSON.
func TestDedupDecisionMarshaling(t *testing.T) {
	decision := DedupDecision{
		DuplicateKey:    "2026-08-25 | -1000 | Uber",
		Action:          "keep_first_delete_rest",
		KeepRecordIDs:   []string{"record-1"},
		DeleteRecordIDs: []string{"record-2"},
		Reason:          "User selected keep-first",
	}

	data, err := json.Marshal(decision)
	if err != nil {
		t.Fatalf("Failed to marshal DedupDecision: %v", err)
	}

	var unmarshaled DedupDecision
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal DedupDecision: %v", err)
	}

	if unmarshaled.Action != decision.Action {
		t.Errorf("Expected action %q, got %q", decision.Action, unmarshaled.Action)
	}
}

// TestEmptyDatasetDetection verifies that an empty records.json returns no duplicates.
func TestEmptyDatasetDetection(t *testing.T) {
	// TODO: Implement test
	// This will be populated in Phase 2/3 once detection logic is implemented
}

// TestExactDuplicateDetection verifies that records with identical amount/date/counterparty are detected.
func TestExactDuplicateDetection(t *testing.T) {
	// TODO: Implement test
	// This will be populated in Phase 3 once detection logic is implemented
}

// TestNoDuplicatesWithDifferentCounterparty verifies no false positives.
func TestNoDuplicatesWithDifferentCounterparty(t *testing.T) {
	// TODO: Implement test
	// This will be populated in Phase 3 once detection logic is implemented
}

// Helper function to create a temporary test records.json file
func createTestRecordsFile(t *testing.T, records interface{}) string {
	t.Helper()

	data, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("Failed to marshal test records: %v", err)
	}

	f, err := os.CreateTemp("", "test-records-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	return f.Name()
}

// Helper function to clean up test files
func cleanupTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Logf("Warning: failed to clean up test file %q: %v", path, err)
	}
}

// BenchmarkDetectDuplicates benchmarks the duplicate detection on large datasets.
func BenchmarkDetectDuplicates(b *testing.B) {
	// TODO: Implement benchmark
	// This will be populated in Phase 6 once detection logic is implemented
}
