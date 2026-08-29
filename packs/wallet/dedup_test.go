package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/sumitasok/sa.automation.wallet/internal/wallet"
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

// TestLoadRecords verifies records.json loading into memory (working copy).
func TestLoadRecords(t *testing.T) {
	// Create test snapshot file
	snap := RecordsSnapshot{
		FetchedAt: "2026-08-29T10:00:00Z",
		Mode:      "full",
		Count:     2,
		Records: []wallet.Record{
			{
				"id":           "rec-1",
				"amount.value": -1000.0,
				"counterParty": "Uber",
				"recordDate":   "2026-08-25",
				"createdAt":    "2026-08-25T08:00:00Z",
			},
			{
				"id":           "rec-2",
				"amount.value": -500.0,
				"counterParty": "Starbucks",
				"recordDate":   "2026-08-26",
				"createdAt":    "2026-08-26T07:00:00Z",
			},
		},
	}

	tmpfile := createTestRecordsFile(t, snap)
	defer cleanupTestFile(t, tmpfile)

	// Test load
	loaded, err := loadRecords(tmpfile)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}

	if loaded.Count != 2 {
		t.Errorf("Expected 2 records, got %d", loaded.Count)
	}
	if len(loaded.Records) != 2 {
		t.Errorf("Expected 2 records in slice, got %d", len(loaded.Records))
	}
}

// TestLoadRecordsInvalidJSON verifies error handling for corrupt JSON.
func TestLoadRecordsInvalidJSON(t *testing.T) {
	tmpfile := createTestFile(t, []byte("{ invalid json"))
	defer cleanupTestFile(t, tmpfile)

	_, err := loadRecords(tmpfile)
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
}

// TestLoadDedupConfig verifies config loading with defaults.
func TestLoadDedupConfig(t *testing.T) {
	config, err := loadDedupConfig("", 0.5)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(config.PrimaryKeys) == 0 {
		t.Errorf("Expected primary keys, got empty")
	}
	if config.MinConfidence != 0.5 {
		t.Errorf("Expected minConfidence 0.5, got %f", config.MinConfidence)
	}
}

// TestCreateBackup verifies timestamped backup creation.
func TestCreateBackup(t *testing.T) {
	origContent := []byte("original content")
	tmpfile := createTestFile(t, origContent)
	defer cleanupTestFile(t, tmpfile)

	backupPath, err := createBackup(tmpfile)
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}
	defer cleanupTestFile(t, backupPath)

	// Verify backup exists and contains same content
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup: %v", err)
	}
	if string(backupContent) != string(origContent) {
		t.Errorf("Backup content doesn't match original")
	}
}

// TestGetFieldValue verifies dot-notation field extraction.
func TestGetFieldValue(t *testing.T) {
	// Create test data - using any type for map keys to match JSON unmarshal
	rec := wallet.Record{}
	rec["id"] = "test-1"
	rec["counterParty"] = "Uber"

	amountMap := make(map[string]interface{})
	amountMap["value"] = -1000.0
	amountMap["currencyCode"] = "INR"
	rec["amount"] = amountMap

	tests := []struct {
		field   string
		want    string
		wantOk  bool
	}{
		{"id", "test-1", true},
		{"counterParty", "Uber", true},
		{"amount.value", "-1000", true},
		{"amount.currencyCode", "INR", true},
		{"nonexistent", "", false},
		{"amount.nonexistent", "", false},
	}

	for _, tt := range tests {
		val, ok := getFieldValue(rec, tt.field)
		if ok != tt.wantOk {
			t.Errorf("getFieldValue(%q): got ok=%v, want %v", tt.field, ok, tt.wantOk)
		}
		if ok && fmt.Sprintf("%v", val) != tt.want {
			t.Errorf("getFieldValue(%q): got %v, want %v", tt.field, val, tt.want)
		}
	}
}

// TestMatchKey verifies duplicate key matching.
func TestMatchKey(t *testing.T) {
	config := &DedupConfig{
		PrimaryKeys: []string{"amount.value", "recordDate", "counterParty"},
	}

	createRec := func(amount float64, date, party string) wallet.Record {
		rec := wallet.Record{}
		amountMap := make(map[string]interface{})
		amountMap["value"] = amount
		rec["amount"] = amountMap
		rec["recordDate"] = date
		rec["counterParty"] = party
		return rec
	}

	rec1 := createRec(-1000.0, "2026-08-25", "Uber")
	rec2 := createRec(-1000.0, "2026-08-25", "Uber")
	rec3 := createRec(-1000.0, "2026-08-25", "Lyft")

	if !matchKey(rec1, rec2, config) {
		t.Error("matchKey: expected true for identical records")
	}
	if matchKey(rec1, rec3, config) {
		t.Error("matchKey: expected false for different counterparty")
	}
}

// TestCalculateConfidence verifies confidence scoring.
func TestCalculateConfidence(t *testing.T) {
	config := &DedupConfig{
		PrimaryKeys:  []string{"amount.value", "recordDate", "counterParty"},
		OptionalKeys: []string{"category.id"},
	}

	createRec := func(amount float64, date, party, catID string) wallet.Record {
		rec := wallet.Record{}
		amountMap := make(map[string]interface{})
		amountMap["value"] = amount
		rec["amount"] = amountMap
		rec["recordDate"] = date
		rec["counterParty"] = party

		catMap := make(map[string]interface{})
		catMap["id"] = catID
		rec["category"] = catMap
		return rec
	}

	rec1 := createRec(-1000.0, "2026-08-25", "Uber", "cat-1")
	rec2 := createRec(-1000.0, "2026-08-25", "Uber", "cat-1")
	rec3 := createRec(-1000.0, "2026-08-25", "Uber", "cat-2")

	conf12 := calculateConfidence(rec1, rec2, config)
	if conf12 != 1.0 {
		t.Errorf("Expected confidence 1.0 for exact match, got %f", conf12)
	}

	conf13 := calculateConfidence(rec1, rec3, config)
	if conf13 >= 1.0 {
		t.Errorf("Expected confidence <1.0 for category mismatch, got %f", conf13)
	}
}

// TestFindDuplicateGroups verifies duplicate grouping logic.
func TestFindDuplicateGroups(t *testing.T) {
	config := &DedupConfig{
		PrimaryKeys: []string{"amount.value", "recordDate", "counterParty"},
	}

	createRec := func(id, amount string, date, party, createdAt string) wallet.Record {
		rec := wallet.Record{}
		rec["id"] = id
		amountMap := make(map[string]interface{})
		amountVal, _ := strconv.ParseFloat(amount, 64)
		amountMap["value"] = amountVal
		rec["amount"] = amountMap
		rec["recordDate"] = date
		rec["counterParty"] = party
		rec["createdAt"] = createdAt
		return rec
	}

	records := []wallet.Record{
		createRec("rec-1", "-1000", "2026-08-25", "Uber", "2026-08-25T08:00:00Z"),
		createRec("rec-2", "-1000", "2026-08-25", "Uber", "2026-08-25T09:00:00Z"),
		createRec("rec-3", "-500", "2026-08-26", "Starbucks", "2026-08-26T07:00:00Z"),
	}

	groups := findDuplicateGroups(records, config)

	if len(groups) != 1 {
		t.Errorf("Expected 1 duplicate group, got %d", len(groups))
	}
	if len(groups[0].Records) != 2 {
		t.Errorf("Expected 2 records in group, got %d", len(groups[0].Records))
	}
	if !groups[0].Records[0].IsOriginal {
		t.Error("Expected first record to be marked as original")
	}
}

// BenchmarkDetectDuplicates benchmarks the duplicate detection on large datasets.
func BenchmarkDetectDuplicates(b *testing.B) {
	// TODO: Implement benchmark
	// This will be populated in Phase 6 once detection logic is implemented
}

// Helper to create a test file with arbitrary content
func createTestFile(t *testing.T, content []byte) string {
	t.Helper()
	f, err := os.CreateTemp("", "test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer f.Close()

	if _, err := f.Write(content); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	return f.Name()
}
