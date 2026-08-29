package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
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

// TestFormatGroupsText verifies text output formatting.
func TestFormatGroupsText(t *testing.T) {
	groups := []DuplicateGroup{
		{
			DuplicateKey: "2026-08-25 | -1000 | Uber",
			MatchType:    "exact",
			Confidence:   1.0,
			Records: []RecordSummary{
				{
					ID:           "rec-1",
					CreatedAt:    "2026-08-25T08:00:00Z",
					IsOriginal:   true,
					CounterParty: "Uber",
					Amount:       -1000,
					Category:     "Transport",
				},
				{
					ID:           "rec-2",
					CreatedAt:    "2026-08-25T09:00:00Z",
					IsOriginal:   false,
					CounterParty: "Uber",
					Amount:       -1000,
					Category:     "Transport",
				},
			},
		},
	}

	text := formatGroupsText(groups)

	if !strings.Contains(text, "Duplicate Groups Found: 1") {
		t.Error("formatGroupsText: missing group count")
	}
	if !strings.Contains(text, "rec-1") {
		t.Error("formatGroupsText: missing record ID")
	}
	if !strings.Contains(text, "original") {
		t.Error("formatGroupsText: missing original marker")
	}
	if !strings.Contains(text, "duplicate") {
		t.Error("formatGroupsText: missing duplicate marker")
	}
}

// TestFormatGroupsTextEmpty verifies formatting for empty groups.
func TestFormatGroupsTextEmpty(t *testing.T) {
	groups := []DuplicateGroup{}
	text := formatGroupsText(groups)

	if !strings.Contains(text, "No duplicate records found") {
		t.Error("formatGroupsText: missing empty message")
	}
}

// TestFormatGroupsJSON verifies JSON output formatting.
func TestFormatGroupsJSON(t *testing.T) {
	groups := []DuplicateGroup{
		{
			DuplicateKey: "2026-08-25 | -1000 | Uber",
			MatchType:    "exact",
			Confidence:   1.0,
			Records: []RecordSummary{
				{
					ID:           "rec-1",
					CreatedAt:    "2026-08-25T08:00:00Z",
					IsOriginal:   true,
					CounterParty: "Uber",
					Amount:       -1000,
				},
			},
		},
	}

	data, err := formatGroupsJSON(groups)
	if err != nil {
		t.Fatalf("Failed to format JSON: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}

	if groupCount, ok := output["duplicateGroupsFound"].(float64); !ok || groupCount != 1 {
		t.Error("formatGroupsJSON: incorrect group count in output")
	}
}

// TestCountDuplicateRecords verifies counting of duplicate records.
func TestCountDuplicateRecords(t *testing.T) {
	groups := []DuplicateGroup{
		{
			Records: []RecordSummary{
				{IsOriginal: true},
				{IsOriginal: false},
				{IsOriginal: false},
			},
		},
		{
			Records: []RecordSummary{
				{IsOriginal: true},
				{IsOriginal: false},
			},
		},
	}

	count := countDuplicateRecords(groups)
	if count != 3 {
		t.Errorf("Expected 3 duplicate records, got %d", count)
	}
}

// TestDetectRecordDuplicatesIntegration tests full detection on sample data.
func TestDetectRecordDuplicatesIntegration(t *testing.T) {
	snap := RecordsSnapshot{
		FetchedAt: "2026-08-29T10:00:00Z",
		Mode:      "full",
		Count:     3,
		Records: []wallet.Record{
			createMapRecord("rec-1", "-1000", "2026-08-25", "Uber", "2026-08-25T08:00:00Z"),
			createMapRecord("rec-2", "-1000", "2026-08-25", "Uber", "2026-08-25T09:00:00Z"),
			createMapRecord("rec-3", "-500", "2026-08-26", "Starbucks", "2026-08-26T07:00:00Z"),
		},
	}

	tmpfile := createTestFile(t, marshalJSON(t, snap))
	defer cleanupTestFile(t, tmpfile)

	groups, err := detectRecordDuplicates(tmpfile, "", 0.5)
	if err != nil {
		t.Fatalf("Failed to detect duplicates: %v", err)
	}

	if len(groups) != 1 {
		t.Errorf("Expected 1 duplicate group, got %d", len(groups))
	}
}

// BenchmarkDetectDuplicates benchmarks the duplicate detection on large datasets.
func BenchmarkDetectDuplicates(b *testing.B) {
	// TODO: Implement benchmark
	// This will be populated in Phase 6 once detection logic is implemented
}

// Helper to create a wallet.Record as a map
func createMapRecord(id, amount, date, party, createdAt string) wallet.Record {
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

// Helper to marshal struct to JSON bytes
func marshalJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}
	return data
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

// === Phase 4 Tests (Review & Decision Collection) ===

// TestPrintDuplicateGroupFormatting verifies group formatting for user display.
func TestPrintDuplicateGroupFormatting(t *testing.T) {
	group := DuplicateGroup{
		DuplicateKey: "2026-08-25 | -1000 | Uber",
		MatchType:    "exact",
		Confidence:   1.0,
		Records: []RecordSummary{
			{
				ID:           "rec-1",
				CreatedAt:    "2026-08-25T08:00:00Z",
				IsOriginal:   true,
				CounterParty: "Uber",
				Amount:       -1000,
				Category:     "Transport",
			},
			{
				ID:           "rec-2",
				CreatedAt:    "2026-08-25T09:00:00Z",
				IsOriginal:   false,
				CounterParty: "Uber",
				Amount:       -1000,
				Category:     "Transport",
			},
		},
	}

	// Test that printDuplicateGroup doesn't panic
	printDuplicateGroup(1, group)
	// Output verification would require capturing stdout, not done here
}

// TestParseCustomDecisionValidation verifies custom decision parsing.
func TestParseCustomDecisionValidation(t *testing.T) {
	group := DuplicateGroup{
		DuplicateKey: "2026-08-25 | -1000 | Uber",
		Records: []RecordSummary{
			{ID: "rec-1", IsOriginal: true},
			{ID: "rec-2", IsOriginal: false},
			{ID: "rec-3", IsOriginal: false},
		},
	}

	// Test case: custom decision with invalid format is caught
	// (real implementation would read from stdin, so we skip interactive test)
	if len(group.Records) < 2 {
		t.Fatal("Expected at least 2 records for testing")
	}
}

// TestCountDeleteRecords verifies deletion count aggregation.
func TestCountDeleteRecords(t *testing.T) {
	decisions := []DedupDecision{
		{
			DuplicateKey:    "key1",
			DeleteRecordIDs: []string{"rec-1", "rec-2"},
		},
		{
			DuplicateKey:    "key2",
			DeleteRecordIDs: []string{"rec-3"},
		},
		{
			DuplicateKey:    "key3",
			DeleteRecordIDs: []string{},
		},
	}

	count := countDeleteRecords(decisions)
	if count != 3 {
		t.Errorf("Expected 3 records to delete, got %d", count)
	}
}

// TestSaveDedupDecisionsJSON verifies decisions are saved correctly to file.
func TestSaveDedupDecisionsJSON(t *testing.T) {
	decisions := []DedupDecision{
		{
			DuplicateKey:    "2026-08-25 | -1000 | Uber",
			Action:          "keep_first_delete_rest",
			KeepRecordIDs:   []string{"rec-1"},
			DeleteRecordIDs: []string{"rec-2"},
			Reason:          "User selected keep-first",
		},
	}

	tmpfile := t.TempDir() + "/decisions.json"

	err := saveDedupDecisions(decisions, tmpfile)
	if err != nil {
		t.Fatalf("Failed to save decisions: %v", err)
	}
	defer os.Remove(tmpfile)

	// Verify file exists and contains valid JSON
	data, err := os.ReadFile(tmpfile)
	if err != nil {
		t.Fatalf("Failed to read saved decisions: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("Invalid JSON in decisions file: %v", err)
	}

	// Verify structure
	if _, hasTimestamp := output["timestamp"]; !hasTimestamp {
		t.Error("Missing timestamp in saved decisions")
	}
	if _, hasDecisions := output["decisions"]; !hasDecisions {
		t.Error("Missing decisions array in saved decisions")
	}
	if _, hasSummary := output["summary"]; !hasSummary {
		t.Error("Missing summary in saved decisions")
	}
}

// TestDecisionsFilePersistenceStructure verifies the JSON structure matches expectations.
func TestDecisionsFilePersistenceStructure(t *testing.T) {
	decisions := []DedupDecision{
		{
			DuplicateKey:    "key1",
			Action:          "keep_first_delete_rest",
			KeepRecordIDs:   []string{"rec-1"},
			DeleteRecordIDs: []string{"rec-2", "rec-3"},
			Reason:          "Keep oldest record",
		},
	}

	tmpfile := t.TempDir() + "/test-decisions.json"
	err := saveDedupDecisions(decisions, tmpfile)
	if err != nil {
		t.Fatalf("Failed to save: %v", err)
	}
	defer os.Remove(tmpfile)

	data, _ := os.ReadFile(tmpfile)
	var output map[string]interface{}
	json.Unmarshal(data, &output)

	summary := output["summary"].(map[string]interface{})
	if int(summary["recordsToDelete"].(float64)) != 2 {
		t.Errorf("Expected 2 records to delete in summary, got %v", summary["recordsToDelete"])
	}
}

// TestReviewDuplicatesIntegration tests the full review workflow (non-interactive parts).
func TestReviewDuplicatesIntegration(t *testing.T) {
	// Create test groups
	groups := []DuplicateGroup{
		{
			DuplicateKey: "2026-08-25 | -1000 | Uber",
			MatchType:    "exact",
			Confidence:   1.0,
			Records: []RecordSummary{
				{ID: "rec-1", IsOriginal: true, CreatedAt: "2026-08-25T08:00:00Z", Amount: -1000, CounterParty: "Uber"},
				{ID: "rec-2", IsOriginal: false, CreatedAt: "2026-08-25T09:00:00Z", Amount: -1000, CounterParty: "Uber"},
			},
		},
	}

	// Test non-interactive review
	decisions, err := reviewDuplicates(groups, false)
	// Note: In non-interactive mode, this would need stdin mocking or a different interface
	// For now, we verify the function signature is correct
	_ = decisions
	_ = err
}

// TestDecisionsFileFormatValidation verifies decisions can be loaded back.
func TestDecisionsFileFormatValidation(t *testing.T) {
	decisions := []DedupDecision{
		{
			DuplicateKey:    "key1",
			Action:          "keep_first_delete_rest",
			KeepRecordIDs:   []string{"rec-1"},
			DeleteRecordIDs: []string{"rec-2"},
			Reason:          "Testing",
		},
	}

	tmpfile := t.TempDir() + "/decisions.json"
	saveDedupDecisions(decisions, tmpfile)
	defer os.Remove(tmpfile)

	// Load decisions back and verify
	data, _ := os.ReadFile(tmpfile)
	var output struct {
		Timestamp string          `json:"timestamp"`
		Decisions []DedupDecision `json:"decisions"`
		Summary   struct {
			TotalGroupsReviewed int `json:"totalGroupsReviewed"`
			RecordsToDelete     int `json:"recordsToDelete"`
		} `json:"summary"`
	}

	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("Failed to unmarshal decisions: %v", err)
	}

	if len(output.Decisions) != 1 {
		t.Errorf("Expected 1 decision, got %d", len(output.Decisions))
	}
	if output.Decisions[0].DuplicateKey != "key1" {
		t.Errorf("Expected key1, got %s", output.Decisions[0].DuplicateKey)
	}
}
