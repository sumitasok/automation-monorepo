// Package main implements wallet record deduplication.
//
// The dedup subcommand identifies, reviews, and removes duplicate transaction records.
// IMPORTANT: All operations happen on a working copy in memory. records.json is never
// modified until the final atomic write after user confirmation and verification.
//
// Workflow:
//  1. scan: Load records.json → create working copy → detect duplicates → report findings
//  2. review: Load working copy state → collect user decisions → save to decisions.json
//  3. execute: Load working copy → apply decisions → backup records.json → atomic write
//
// Three operations are supported:
//   - scan: Identify duplicates without touching records.json
//   - review: Collect user decisions on which records to keep/delete
//   - execute: Apply decisions atomically (backup before write, verify after)
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sumitasok/sa.automation.wallet/internal/wallet"
)

// DuplicateGroup represents a set of duplicate records matching on amount+date+counterparty.
type DuplicateGroup struct {
	DuplicateKey string          `json:"duplicateKey"`
	MatchType    string          `json:"matchType"` // "exact" or "uncertain"
	Confidence   float64         `json:"confidence"`
	Records      []RecordSummary `json:"records"`
}

// RecordSummary is a minimal record representation for display and decisions.
type RecordSummary struct {
	ID           string  `json:"id"`
	CreatedAt    string  `json:"createdAt"`
	IsOriginal   bool    `json:"isOriginal"`
	CounterParty string  `json:"counterParty"`
	Amount       float64 `json:"amount"`
	Category     string  `json:"category"`
}

// DedupDecision represents a user's choice for a duplicate group.
type DedupDecision struct {
	DuplicateKey    string   `json:"duplicateKey"`
	Action          string   `json:"action"` // "keep_first_delete_rest", "custom", "skip"
	KeepRecordIDs   []string `json:"keepRecordIds"`
	DeleteRecordIDs []string `json:"deleteRecordIds"`
	Reason          string   `json:"reason"`
}

// DedupConfig holds configuration for dedup operations.
type DedupConfig struct {
	PrimaryKeys   []string `json:"primaryKeys"`
	OptionalKeys  []string `json:"optionalKeys"`
	MinConfidence float64  `json:"minConfidence"`
}

// RecordsSnapshot is the structure of records.json (from wallet-fetch).
type RecordsSnapshot struct {
	FetchedAt    string          `json:"fetchedAt"`
	Mode         string          `json:"mode"`
	Since        string          `json:"since,omitempty"`
	UpdatedSince string          `json:"updatedSince,omitempty"`
	Count        int             `json:"count"`
	APITotal     int             `json:"apiTotal,omitempty"`
	DeltaFetched int             `json:"deltaFetched,omitempty"`
	Records      []wallet.Record `json:"records"`
}

// loadRecords reads records.json into a working copy in memory.
// The original file is never modified. Returns the snapshot with working copy of records.
func loadRecords(recordsFile string) (*RecordsSnapshot, error) {
	file, err := os.Open(recordsFile)
	if err != nil {
		return nil, fmt.Errorf("load records: %w", err)
	}
	defer file.Close()

	snap := &RecordsSnapshot{
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
		Records:   []wallet.Record{},
	}

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Skip metadata header line (contains fetchedAt, recordCount, apiTotal)
		if lineNum == 1 {
			var meta map[string]interface{}
			if err := json.Unmarshal(line, &meta); err == nil {
				if _, ok := meta["fetchedAt"]; ok {
					continue // Skip metadata line
				}
			}
		}

		var rec wallet.Record
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil, fmt.Errorf("parse record line %d: %w", lineNum, err)
		}

		snap.Records = append(snap.Records, rec)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read records.jsonl: %w", err)
	}

	snap.Count = len(snap.Records)
	return snap, nil
}

// loadDedupConfig loads dedup configuration from config path or uses defaults.
func loadDedupConfig(configPath string, minConfidence float64) (*DedupConfig, error) {
	config := &DedupConfig{
		PrimaryKeys:   []string{"recordDate", "amount.value", "counterParty"},
		OptionalKeys:  []string{},
		MinConfidence: minConfidence,
	}

	if configPath == "" {
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		// If config doesn't exist, use defaults
		return config, nil
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if dedupConfig, ok := raw["dedup"].(map[string]interface{}); ok {
		if keys, ok := dedupConfig["primaryKeys"].([]interface{}); ok {
			config.PrimaryKeys = make([]string, len(keys))
			for i, k := range keys {
				config.PrimaryKeys[i] = fmt.Sprintf("%v", k)
			}
		}
		if keys, ok := dedupConfig["optionalKeys"].([]interface{}); ok {
			config.OptionalKeys = make([]string, len(keys))
			for i, k := range keys {
				config.OptionalKeys[i] = fmt.Sprintf("%v", k)
			}
		}
		if min, ok := dedupConfig["minConfidence"].(float64); ok {
			config.MinConfidence = min
		}
	}

	return config, nil
}

// createBackup creates a timestamped backup of the records file before modification.
// Returns the backup file path.
func createBackup(recordsFile string) (string, error) {
	data, err := os.ReadFile(recordsFile)
	if err != nil {
		return "", fmt.Errorf("read original: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	backupPath := recordsFile + ".backup." + timestamp

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", fmt.Errorf("create backup: %w", err)
	}

	log.Printf("backup created: %s", backupPath)
	return backupPath, nil
}

// appendAuditTrail appends a dedup operation entry to state.json.
func appendAuditTrail(statePath, operation string, deletedIDs []string, countBefore, countAfter int, backupFile string) error {
	type auditEntry struct {
		Timestamp          string   `json:"timestamp"`
		Operation          string   `json:"operation"`
		DeletedRecordIDs   []string `json:"deletedRecordIds"`
		TotalRecordsBefore int      `json:"totalRecordsBefore"`
		TotalRecordsAfter  int      `json:"totalRecordsAfter"`
		BackupFile         string   `json:"backupFile"`
	}

	entry := auditEntry{
		Timestamp:          time.Now().UTC().Format(time.RFC3339),
		Operation:          operation,
		DeletedRecordIDs:   deletedIDs,
		TotalRecordsBefore: countBefore,
		TotalRecordsAfter:  countAfter,
		BackupFile:         backupFile,
	}

	// Try to read existing state.json to preserve other fields
	var stateData map[string]interface{}
	if data, err := os.ReadFile(statePath); err == nil {
		json.Unmarshal(data, &stateData)
	}
	if stateData == nil {
		stateData = make(map[string]interface{})
	}

	// Add entry to audit trail (as array)
	var audit []auditEntry
	if existing, ok := stateData["dedupAuditTrail"].([]interface{}); ok {
		for _, e := range existing {
			if data, err := json.Marshal(e); err == nil {
				var ae auditEntry
				if err := json.Unmarshal(data, &ae); err == nil {
					audit = append(audit, ae)
				}
			}
		}
	}
	audit = append(audit, entry)
	stateData["dedupAuditTrail"] = audit

	// Write state.json
	data, err := json.MarshalIndent(stateData, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	return os.WriteFile(statePath, data, 0644)
}

// getFieldValue extracts a field value from a record using dot notation (e.g., "amount.value").
// Works with nested maps created from JSON unmarshaling.
func getFieldValue(record wallet.Record, fieldPath string) (interface{}, bool) {
	parts := strings.Split(fieldPath, ".")
	var current interface{} = record

	for i, part := range parts {
		if current == nil {
			return nil, false
		}

		// Try to access as a map (any is alias for interface{})
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}

		next, ok := m[part]
		if !ok {
			return nil, false
		}
		current = next
	}

	return current, true
}

// matchKey checks if two records match on the primary dedup keys.
func matchKey(rec1, rec2 wallet.Record, config *DedupConfig) bool {
	for _, key := range config.PrimaryKeys {
		val1, ok1 := getFieldValue(rec1, key)
		val2, ok2 := getFieldValue(rec2, key)

		if ok1 != ok2 {
			return false
		}
		if !ok1 {
			continue // both missing
		}

		// Normalize for comparison
		s1 := fmt.Sprintf("%v", val1)
		s2 := fmt.Sprintf("%v", val2)
		if s1 != s2 {
			return false
		}
	}

	return true
}

// calculateConfidence calculates match confidence (1.0 = exact, <1.0 = uncertain).
func calculateConfidence(rec1, rec2 wallet.Record, config *DedupConfig) float64 {
	// If they match on primary keys, check optional keys
	if len(config.OptionalKeys) == 0 {
		return 1.0 // Exact match on primary keys, no optional fields
	}

	matchedOptional := 0
	for _, key := range config.OptionalKeys {
		val1, ok1 := getFieldValue(rec1, key)
		val2, ok2 := getFieldValue(rec2, key)

		if ok1 && ok2 {
			s1 := fmt.Sprintf("%v", val1)
			s2 := fmt.Sprintf("%v", val2)
			if s1 == s2 {
				matchedOptional++
			}
		}
	}

	if matchedOptional == len(config.OptionalKeys) {
		return 1.0 // Exact match on all fields
	}

	// Partial match - return confidence based on optional matches
	if len(config.OptionalKeys) > 0 {
		return float64(matchedOptional) / float64(len(config.OptionalKeys))
	}

	return 1.0
}

// detectRecordDuplicates loads records.json into a working copy and identifies duplicates.
// The original records.json is NEVER modified. All operations happen on the working copy.
// Returns the list of duplicate groups found in the working copy.
func detectRecordDuplicates(recordsFile, configPath string, minConfidence float64) ([]DuplicateGroup, error) {
	// 1. Load records.json into memory (working copy)
	snap, err := loadRecords(recordsFile)
	if err != nil {
		return nil, err
	}

	// 2. Load config
	config, err := loadDedupConfig(configPath, minConfidence)
	if err != nil {
		return nil, err
	}

	// 3. Find duplicates on working copy (original records untouched)
	groups := findDuplicateGroups(snap.Records, config)

	return groups, nil
}

// findDuplicateGroups identifies all duplicate groups in records using the config.
func findDuplicateGroups(records []wallet.Record, config *DedupConfig) []DuplicateGroup {
	type groupKey string
	groups := make(map[groupKey][]wallet.Record)
	seenKeys := make(map[groupKey]bool)

	// Group records by dedup key
	for i, rec := range records {
		// Build a key string from primary key values
		var keyParts []string
		for _, keyField := range config.PrimaryKeys {
			val, ok := getFieldValue(rec, keyField)
			if !ok || val == nil {
				keyParts = append(keyParts, "")
			} else {
				keyParts = append(keyParts, fmt.Sprintf("%v", val))
			}
		}
		key := groupKey(strings.Join(keyParts, " | "))
		groups[key] = append(groups[key], records[i])
		seenKeys[key] = true
	}

	// Build DuplicateGroup results (only groups with 2+ records)
	var results []DuplicateGroup
	for key, recs := range groups {
		if len(recs) < 2 {
			continue
		}

		// Sort by createdAt to identify original
		for i := 0; i < len(recs); i++ {
			for j := i + 1; j < len(recs); j++ {
				created1, _ := getFieldValue(recs[i], "createdAt")
				created2, _ := getFieldValue(recs[j], "createdAt")
				if fmt.Sprintf("%v", created2) < fmt.Sprintf("%v", created1) {
					recs[i], recs[j] = recs[j], recs[i]
				}
			}
		}

		// Build group
		group := DuplicateGroup{
			DuplicateKey: string(key),
			MatchType:    "exact",
			Confidence:   1.0,
			Records:      make([]RecordSummary, len(recs)),
		}

		for i, rec := range recs {
			amount, _ := getFieldValue(rec, "amount.value")
			amountF, _ := strconv.ParseFloat(fmt.Sprintf("%v", amount), 64)
			counterparty, _ := getFieldValue(rec, "counterParty")
			category, _ := getFieldValue(rec, "category")
			categoryStr := ""
			if catMap, ok := category.(map[string]interface{}); ok {
				if catName, ok := catMap["name"]; ok {
					categoryStr = fmt.Sprintf("%v", catName)
				}
			}

			group.Records[i] = RecordSummary{
				ID:           fmt.Sprintf("%v", rec["id"]),
				CreatedAt:    fmt.Sprintf("%v", rec["createdAt"]),
				IsOriginal:   i == 0,
				CounterParty: fmt.Sprintf("%v", counterparty),
				Amount:       amountF,
				Category:     categoryStr,
			}
		}

		results = append(results, group)
	}

	return results
}

// formatGroupsText formats duplicate groups as minimal summary output.
func formatGroupsText(groups []DuplicateGroup) string {
	if len(groups) == 0 {
		return ""
	}

	// countDuplicateRecords counts only the duplicate (non-original) records
	toDelete := countDuplicateRecords(groups)
	toKeep := len(groups) // one original per group

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("DUPLICATE: %d groups | %d records to delete | %d to keep\n", len(groups), toDelete, toKeep))

	return sb.String()
}

// formatGroupsJSON formats duplicate groups as JSON array.
func formatGroupsJSON(groups []DuplicateGroup) ([]byte, error) {
	output := map[string]interface{}{
		"timestamp":             time.Now().UTC().Format(time.RFC3339),
		"duplicateGroupsFound":  len(groups),
		"groups":                groups,
		"totalDuplicateRecords": countDuplicateRecords(groups),
	}

	return json.MarshalIndent(output, "", "  ")
}

// countDuplicateRecords returns the total number of duplicate records (excluding originals).
func countDuplicateRecords(groups []DuplicateGroup) int {
	count := 0
	for _, group := range groups {
		for _, rec := range group.Records {
			if !rec.IsOriginal {
				count++
			}
		}
	}
	return count
}

// runDedupScan implements the `wallet dedup scan` subcommand.
func runDedupScan(args []string) error {
	fs := flag.NewFlagSet("dedup scan", flag.ExitOnError)
	recordsFile := fs.String("records-file", "", "path to records.json (default: $AUTO_DATA_DIR/wallet/records.jsonl or ./records.json)")
	dedupConfig := fs.String("dedup-config", "", "path to dedup config (default: config.yaml)")
	format := fs.String("format", "text", "output format: text or json")
	minConfidence := fs.Float64("min-confidence", 0.5, "minimum confidence threshold (0.0-1.0)")
	fs.Parse(args)

	// Resolve paths
	if *recordsFile == "" {
		*recordsFile = resolveDataPath("wallet/records.jsonl", "records.jsonl")
	}

	fmt.Printf("Working directory: %s\n", filepath.Dir(*recordsFile))
	fmt.Printf("Reading records: %s\n\n", *recordsFile)

	// Detect duplicates
	groups, err := detectRecordDuplicates(*recordsFile, *dedupConfig, *minConfidence)
	if err != nil {
		return err
	}

	// Output results
	switch *format {
	case "json":
		data, err := formatGroupsJSON(groups)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	case "text":
		fmt.Print(formatGroupsText(groups))
	default:
		return fmt.Errorf("unknown format: %s (text or json)", *format)
	}

	return nil
}

// printDuplicateGroup displays a duplicate group to the user for review.
func printDuplicateGroup(index int, group DuplicateGroup) {
	fmt.Printf("\nGroup %d of duplicate records: %s\n", index, group.DuplicateKey)
	fmt.Printf("Match Type: %s (confidence: %.0f%%)\n\n", group.MatchType, group.Confidence*100)

	for i, rec := range group.Records {
		marker := "DUPLICATE"
		if rec.IsOriginal {
			marker = "ORIGINAL"
		}
		fmt.Printf("  Record %d [%s] %s\n", i+1, marker, rec.CreatedAt)
		fmt.Printf("    ID: %s\n", rec.ID)
		fmt.Printf("    Amount: %.2f\n", rec.Amount)
		fmt.Printf("    Counterparty: %s\n", rec.CounterParty)
		if rec.Category != "" {
			fmt.Printf("    Category: %s\n", rec.Category)
		}
		fmt.Println()
	}
}

// readUserDecision prompts user for action on a duplicate group.
func readUserDecision(groupIndex int, group DuplicateGroup) (string, error) {
	for {
		fmt.Printf("Action for Group %d? (keep-first/custom/skip) [keep-first]: ", groupIndex)
		var input string
		fmt.Scanln(&input)
		if input == "" {
			input = "keep-first"
		}

		switch input {
		case "keep-first", "custom", "skip":
			return input, nil
		default:
			fmt.Printf("Invalid action: %q. Use keep-first, custom, or skip.\n", input)
		}
	}
}

// parseCustomDecision parses user's custom keep/delete selection.
func parseCustomDecision(group DuplicateGroup) ([]string, []string, error) {
	fmt.Printf("Enter record numbers to KEEP (comma-separated, e.g., 1,3): ")
	var input string
	fmt.Scanln(&input)

	if input == "" {
		return nil, nil, fmt.Errorf("must keep at least one record")
	}

	parts := strings.Split(input, ",")
	var keepIndices []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		idx, err := strconv.Atoi(p)
		if err != nil || idx < 1 || idx > len(group.Records) {
			return nil, nil, fmt.Errorf("invalid record number: %q", p)
		}
		keepIndices = append(keepIndices, idx-1)
	}

	var keepIDs, deleteIDs []string
	for i, rec := range group.Records {
		found := false
		for _, keepIdx := range keepIndices {
			if i == keepIdx {
				found = true
				break
			}
		}
		if found {
			keepIDs = append(keepIDs, rec.ID)
		} else {
			deleteIDs = append(deleteIDs, rec.ID)
		}
	}

	if len(keepIDs) == 0 {
		return nil, nil, fmt.Errorf("must keep at least one record")
	}

	return keepIDs, deleteIDs, nil
}

// collectDecisions walks through duplicate groups and collects user decisions.
func collectDecisions(groups []DuplicateGroup) ([]DedupDecision, error) {
	var decisions []DedupDecision

	for i, group := range groups {
		printDuplicateGroup(i+1, group)

		action, err := readUserDecision(i+1, group)
		if err != nil {
			return nil, err
		}

		if action == "skip" {
			fmt.Printf("Skipping Group %d\n", i+1)
			continue
		}

		var decision DedupDecision
		decision.DuplicateKey = group.DuplicateKey

		if action == "keep-first" {
			decision.Action = "keep_first_delete_rest"
			decision.KeepRecordIDs = []string{group.Records[0].ID}
			for _, rec := range group.Records[1:] {
				decision.DeleteRecordIDs = append(decision.DeleteRecordIDs, rec.ID)
			}
			decision.Reason = "User selected keep-first (keep oldest)"
		} else if action == "custom" {
			decision.Action = "custom"
			keepIDs, deleteIDs, err := parseCustomDecision(group)
			if err != nil {
				fmt.Printf("Error: %v. Skipping group.\n", err)
				continue
			}
			decision.KeepRecordIDs = keepIDs
			decision.DeleteRecordIDs = deleteIDs
			decision.Reason = "User selected custom keep/delete"
		}

		decisions = append(decisions, decision)
		fmt.Printf("✓ Group %d: keep %d, delete %d\n\n", i+1, len(decision.KeepRecordIDs), len(decision.DeleteRecordIDs))
	}

	return decisions, nil
}

// saveDedupDecisions writes decisions to a JSON file.
func saveDedupDecisions(decisions []DedupDecision, outputPath string) error {
	// Write JSONL format: one decision per line
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create decisions file: %w", err)
	}
	defer file.Close()

	// Write header with summary
	header := map[string]interface{}{
		"_type":               "dedup-decisions-header",
		"timestamp":           time.Now().UTC().Format(time.RFC3339),
		"totalGroups":         len(decisions),
		"totalRecordsToDelete": countDeleteRecords(decisions),
	}
	headerLine, _ := json.Marshal(header)
	file.Write(append(headerLine, '\n'))

	// Write each decision
	for _, decision := range decisions {
		line, err := json.Marshal(decision)
		if err != nil {
			return fmt.Errorf("marshal decision: %w", err)
		}
		if _, err := file.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("write decision: %w", err)
		}
	}

	return nil
}

// countDeleteRecords returns total records marked for deletion.
func countDeleteRecords(decisions []DedupDecision) int {
	count := 0
	for _, d := range decisions {
		count += len(d.DeleteRecordIDs)
	}
	return count
}

// printDecisionSummary shows a summary of all decisions before execution.
func printDecisionSummary(decisions []DedupDecision) {
	totalDelete := countDeleteRecords(decisions)
	fmt.Printf("\n=== Dedup Decision Summary ===\n")
	fmt.Printf("Groups to process: %d\n", len(decisions))
	fmt.Printf("Total records to delete: %d\n", totalDelete)
	fmt.Printf("\nReview groups:\n")
	for i, d := range decisions {
		fmt.Printf("  %d. %s: keep %d, delete %d\n", i+1, d.DuplicateKey, len(d.KeepRecordIDs), len(d.DeleteRecordIDs))
	}
}

// readFinalConfirmation prompts user for final confirmation before executing dedup.
func readFinalConfirmation() (bool, error) {
	fmt.Printf("\n⚠️  WARNING: This will DELETE %d records from records.json.\n", 0) // count passed separately
	fmt.Printf("A backup will be created before any modification.\n")
	fmt.Printf("\nProceed with dedup? (yes/no) [no]: ")
	var input string
	fmt.Scanln(&input)

	if input == "yes" {
		return true, nil
	}
	return false, nil
}

// runDedupReview implements the `wallet dedup review` subcommand.
func runDedupReview(args []string) error {
	fs := flag.NewFlagSet("dedup review", flag.ExitOnError)
	recordsFile := fs.String("records-file", "", "path to records.json (default: $AUTO_DATA_DIR/wallet/records.jsonl)")
	dedupConfig := fs.String("dedup-config", "", "path to dedup config")
	decisionsFile := fs.String("decisions-file", "", "path to save decisions (default: .dedup-decisions-{timestamp}.json)")
	dryRun := fs.Bool("dry-run", false, "show decisions without saving")
	fs.Parse(args)

	// Resolve paths
	if *recordsFile == "" {
		*recordsFile = resolveDataPath("wallet/records.jsonl", "records.jsonl")
	}

	fmt.Printf("Working directory: %s\n", filepath.Dir(*recordsFile))
	fmt.Printf("Reading records: %s\n\n", *recordsFile)

	// Detect duplicates first
	groups, err := detectRecordDuplicates(*recordsFile, *dedupConfig, 0.5)
	if err != nil {
		return err
	}

	if len(groups) == 0 {
		fmt.Println("No duplicates found.")
		return nil
	}

	fmt.Printf("Found %d duplicate groups. Review each group to decide which records to keep.\n\n", len(groups))

	// Collect user decisions
	decisions, err := collectDecisions(groups)
	if err != nil {
		return err
	}

	if len(decisions) == 0 {
		fmt.Println("No decisions made (all groups skipped).")
		return nil
	}

	// Show summary
	printDecisionSummary(decisions)

	if *dryRun {
		fmt.Println("\n(DRY RUN - no decisions saved)")
		return nil
	}

	// Save decisions to file
	if *decisionsFile == "" {
		*decisionsFile = ".dedup-decisions-" + time.Now().Format("20060102-150405") + ".json"
	}

	if err := saveDedupDecisions(decisions, *decisionsFile); err != nil {
		return err
	}

	fmt.Printf("\n✓ Decisions saved to: %s\n", *decisionsFile)
	fmt.Printf("Next: run 'wallet dedup execute --decisions-file %s' to apply changes\n", *decisionsFile)

	return nil
}

// reviewDuplicates collects user decisions on duplicate groups (from working copy).
// No modifications to records.json. Decisions are saved separately.
func reviewDuplicates(groups []DuplicateGroup, interactive bool) ([]DedupDecision, error) {
	// 1. Present duplicate groups from working copy to user
	// 2. Collect which records to keep/delete
	// 3. Save decisions to decisions.json
	// 4. Original records.json untouched
	return collectDecisions(groups)
}

// executeDuplicates applies user decisions to records.json atomically.
// SAFETY: Backup created BEFORE any modification. records.json only updated after
// verification that new file is valid. Original file + backup both exist on failure.
// NOTE: Implemented in Phase 5 (execute operation)
func executeDuplicates(recordsFile string, decisions []DedupDecision, dryRun bool) error {
	// 1. Load records.json into working copy
	// 2. Apply decisions (filter out deleted records)
	// 3. Create backup: records.json.backup.{timestamp}
	// 4. Validate new records are valid JSON
	// 5. Atomically write to records.json (write to temp, then rename)
	// 6. Append audit trail to state.json
	// 7. On failure: both records.json and backup exist for recovery
	return nil
}

// loadDedupDecisions reads decisions from JSONL file (one decision per line).
func loadDedupDecisions(decisionFile string) ([]DedupDecision, error) {
	file, err := os.Open(decisionFile)
	if err != nil {
		return nil, fmt.Errorf("read decisions file: %w", err)
	}
	defer file.Close()

	var decisions []DedupDecision
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Skip header line
		var headerCheck map[string]interface{}
		if err := json.Unmarshal(line, &headerCheck); err != nil {
			continue
		}
		if isHeader, ok := headerCheck["_type"].(string); ok && isHeader == "dedup-decisions-header" {
			continue
		}

		// Parse decision
		var decision DedupDecision
		if err := json.Unmarshal(line, &decision); err != nil {
			continue
		}
		decisions = append(decisions, decision)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read decisions.jsonl: %w", err)
	}

	if len(decisions) == 0 {
		return nil, fmt.Errorf("no decisions found in %s", decisionFile)
	}

	return decisions, nil
}

// applyDecisions removes records marked for deletion, returning the filtered set.
func applyDecisions(records []wallet.Record, decisions []DedupDecision) ([]wallet.Record, error) {
	// Build set of record IDs to delete
	toDelete := make(map[string]bool)
	for _, decision := range decisions {
		for _, id := range decision.DeleteRecordIDs {
			toDelete[id] = true
		}
	}

	// Filter records: keep those not in delete set
	var filtered []wallet.Record
	for _, rec := range records {
		id, ok := rec["id"].(string)
		if !ok {
			return nil, fmt.Errorf("record missing or invalid id field")
		}
		if !toDelete[id] {
			filtered = append(filtered, rec)
		}
	}

	return filtered, nil
}

// validateUpdateJSON ensures the filtered records form valid JSON.
func validateUpdateJSON(originalRecords, updatedRecords []wallet.Record) error {
	if len(updatedRecords) >= len(originalRecords) {
		return fmt.Errorf("no records deleted: expected fewer records after dedup")
	}

	if len(updatedRecords) == 0 {
		return fmt.Errorf("dedup would delete all records; aborting")
	}

	// Test marshal/unmarshal
	data, err := json.Marshal(updatedRecords)
	if err != nil {
		return fmt.Errorf("marshal updated records: %w", err)
	}

	var test []wallet.Record
	if err := json.Unmarshal(data, &test); err != nil {
		return fmt.Errorf("unmarshal updated records: %w", err)
	}

	return nil
}

// writeRecordsJSON atomically writes records to file using temp file + rename.
func writeRecordsJSON(recordsFile string, records []wallet.Record) error {
	// Write to temp file first (JSONL format: one record per line)
	tmpFile := recordsFile + ".tmp"
	tmpWriter, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	for _, rec := range records {
		line, err := json.Marshal(rec)
		if err != nil {
			tmpWriter.Close()
			os.Remove(tmpFile)
			return fmt.Errorf("marshal record: %w", err)
		}
		if _, err := tmpWriter.Write(append(line, '\n')); err != nil {
			tmpWriter.Close()
			os.Remove(tmpFile)
			return fmt.Errorf("write record: %w", err)
		}
	}
	tmpWriter.Close()

	// Atomic rename
	if err := os.Rename(tmpFile, recordsFile); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("rename temp to records.jsonl: %w", err)
	}

	return nil
}

// rollbackOnFailure restores records from backup if something went wrong.
func rollbackOnFailure(recordsFile, backupPath string) error {
	if backupPath == "" {
		return fmt.Errorf("no backup available for rollback")
	}

	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("read backup file: %w", err)
	}

	if err := os.WriteFile(recordsFile, data, 0644); err != nil {
		return fmt.Errorf("restore from backup: %w", err)
	}

	return nil
}

// readExecutionConfirmation prompts user to confirm deletion before proceeding.
func readExecutionConfirmation(countToDelete int) (bool, error) {
	fmt.Printf("\nConfirm delete %d records? (yes/no) [no]: ", countToDelete)

	var input string
	fmt.Scanln(&input)

	if input == "yes" {
		return true, nil
	}
	return false, nil
}

// executeDedup orchestrates the full dedup execution workflow.
func executeDedup(recordsFile, decisionFile, stateFile string, dryRun bool) error {
	// Load original records
	snap, err := loadRecords(recordsFile)
	if err != nil {
		log.Printf("[ERROR] load records: %v", err)
		return fmt.Errorf("load records: %w", err)
	}
	originalRecords := snap.Records
	log.Printf("[INFO] Loaded %d records from %s", len(originalRecords), recordsFile)

	// Load decisions
	decisions, err := loadDedupDecisions(decisionFile)
	if err != nil {
		log.Printf("[ERROR] load decisions: %v", err)
		return fmt.Errorf("load decisions: %w", err)
	}
	log.Printf("[INFO] Loaded %d decisions from %s", len(decisions), decisionFile)

	// Count records to delete
	countToDelete := countDeleteRecords(decisions)
	fmt.Printf("DEDUPLICATING: %d records to delete | %d to keep\n", countToDelete, len(originalRecords)-countToDelete)
	log.Printf("[DEDUP] %d records to delete from %d total", countToDelete, len(originalRecords))

	// Confirm before proceeding
	if !dryRun {
		ok, err := readExecutionConfirmation(countToDelete)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("Execution cancelled.")
			return nil
		}
	}

	if dryRun {
		fmt.Printf("DRY RUN: would delete %d | keep %d\n", countToDelete, len(originalRecords)-countToDelete)
		return nil
	}

	// Build list of deleted record IDs from decisions
	var deletedIDs []string
	for _, decision := range decisions {
		deletedIDs = append(deletedIDs, decision.DeleteRecordIDs...)
	}

	// Save dedup-results.json (for Wallet API sync phase, NOT for direct records.json update)
	resultsFile := strings.TrimSuffix(recordsFile, ".json") + "-dedup-results.json"
	results := map[string]interface{}{
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
		"recordsToDelete":  deletedIDs,
		"totalToDelete":    countToDelete,
		"totalToKeep":      len(originalRecords) - countToDelete,
		"decisions":        decisions,
		"instructions":     "1) Review: cat " + resultsFile + "\n2) Delete from Wallet API: wallet sync --dedup-results\n3) After Wallet confirms, run: dedup finalize --dedup-results",
	}

	resultsData, _ := json.MarshalIndent(results, "", "  ")
	if err := os.WriteFile(resultsFile, resultsData, 0644); err != nil {
		return fmt.Errorf("save dedup results: %w", err)
	}

	fmt.Printf("DEDUPLICATED: %d to delete | %d to keep\n", countToDelete, len(originalRecords)-countToDelete)
	fmt.Printf("\nResults saved: %s\n", resultsFile)
	fmt.Printf("\nNext: Delete from Wallet API, then run 'dedup finalize'\n")
	log.Printf("[SUCCESS] Dedup results saved: delete=%d, keep=%d, file=%s", countToDelete, len(originalRecords)-countToDelete, resultsFile)
	return nil
}

// runDedupExecute implements the `wallet dedup execute` subcommand.
// validateRecordsFile checks if records.json is accessible and valid.
func validateRecordsFile(recordsFile string) error {
	if recordsFile == "" {
		return fmt.Errorf("records file path is empty")
	}

	// Check file exists
	stat, err := os.Stat(recordsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("records file not found: %s", recordsFile)
		}
		return fmt.Errorf("cannot access records file: %w", err)
	}

	// Check it's readable
	if !stat.Mode().IsRegular() {
		return fmt.Errorf("records file is not a regular file: %s", recordsFile)
	}

	// Check file size (warn if very large)
	if stat.Size() > 100*1024*1024 { // 100MB
		fmt.Printf("warning: records file is large (%d MB), dedup may be slow\n", stat.Size()/1024/1024)
	}

	// Validate JSON structure
	_, err = loadRecords(recordsFile)
	if err != nil {
		return fmt.Errorf("invalid records.json: %w", err)
	}

	return nil
}

// validateDedupConfig checks if dedup config is valid.
func validateDedupConfig(cfg *DedupConfig) error {
	if len(cfg.PrimaryKeys) == 0 {
		return fmt.Errorf("dedup config: no primaryKeys defined")
	}
	if cfg.MinConfidence < 0 || cfg.MinConfidence > 1 {
		return fmt.Errorf("dedup config: minConfidence must be 0-1, got %.2f", cfg.MinConfidence)
	}
	return nil
}

// validateDiskSpace checks if enough space exists for backup.
func validateDiskSpace(recordsFile string, requiredBytes int64) error {
	// Get directory for disk space check
	dir := filepath.Dir(recordsFile)
	if dir == "" {
		dir = "."
	}

	// Try to write test file to verify disk is writable
	testFile := recordsFile + ".disk-test"
	if err := os.WriteFile(testFile, make([]byte, 1024), 0644); err != nil {
		return fmt.Errorf("disk space: cannot write to %s (may be full or permission denied)", dir)
	}
	os.Remove(testFile) // cleanup test file

	return nil
}

// runDedupFinalize updates records.json after Wallet API confirms deletions.
func runDedupFinalize(args []string) error {
	fs := flag.NewFlagSet("dedup finalize", flag.ExitOnError)
	recordsFile := fs.String("records-file", "", "path to records.json")
	resultsFile := fs.String("dedup-results", "", "path to dedup-results.json (from execute command)")
	stateFile := fs.String("state-file", "", "path to state.json for audit trail (optional)")
	fs.Parse(args)

	// Resolve paths
	if *recordsFile == "" {
		*recordsFile = resolveDataPath("wallet/records.jsonl", "records.jsonl")
	}
	if *resultsFile == "" {
		*resultsFile = strings.TrimSuffix(*recordsFile, ".json") + "-dedup-results.json"
	}
	if *stateFile == "" {
		*stateFile = resolveDataPath("wallet/state.json", "state.json")
	}

	fmt.Printf("Working directory: %s\n", filepath.Dir(*recordsFile))
	fmt.Printf("Records file: %s\n", *recordsFile)
	fmt.Printf("Results file: %s\n\n", *resultsFile)

	// Load original records
	snap, err := loadRecords(*recordsFile)
	if err != nil {
		return fmt.Errorf("load records: %w", err)
	}
	originalRecords := snap.Records
	log.Printf("[INFO] Loaded %d records from %s", len(originalRecords), *recordsFile)

	// Load dedup results
	resultsData, err := os.ReadFile(*resultsFile)
	if err != nil {
		return fmt.Errorf("load dedup results: %w", err)
	}
	var results map[string]interface{}
	if err := json.Unmarshal(resultsData, &results); err != nil {
		return fmt.Errorf("parse dedup results: %w", err)
	}
	log.Printf("[INFO] Loaded dedup results from %s", *resultsFile)

	// Confirm Wallet API deletions are complete
	fmt.Printf("Confirm: Have you deleted these records from Wallet API? (yes/no) [no]: ")
	var input string
	fmt.Scanln(&input)
	if input != "yes" {
		fmt.Println("Finalization cancelled. Records.json not updated.")
		return nil
	}

	// Extract deletion data
	deleteIface := results["recordsToDelete"]
	var recordsToDelete []string
	if delList, ok := deleteIface.([]interface{}); ok {
		for _, id := range delList {
			if idStr, ok := id.(string); ok {
				recordsToDelete = append(recordsToDelete, idStr)
			}
		}
	}

	// Apply decisions to create filtered set
	decisions := []DedupDecision{}
	if decisionsIface, ok := results["decisions"]; ok {
		decData, _ := json.Marshal(decisionsIface)
		json.Unmarshal(decData, &decisions)
	}

	filteredRecords, err := applyDecisions(originalRecords, decisions)
	if err != nil {
		return fmt.Errorf("apply decisions: %w", err)
	}

	// Validate new records
	if err := validateUpdateJSON(originalRecords, filteredRecords); err != nil {
		return fmt.Errorf("validate filtered records: %w", err)
	}

	// Create backup BEFORE writing
	backupPath, err := createBackup(*recordsFile)
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}
	log.Printf("[INFO] Backup created: %s", backupPath)

	// Write records atomically
	if err := writeRecordsJSON(*recordsFile, filteredRecords); err != nil {
		log.Printf("[ERROR] write records: %v", err)
		fmt.Printf("ERROR: Write failed. Attempting rollback...\n")
		if rbErr := rollbackOnFailure(*recordsFile, backupPath); rbErr != nil {
			return fmt.Errorf("write failed and rollback failed: %w", err)
		}
		return fmt.Errorf("write failed (rolled back): %w", err)
	}

	// Append audit trail
	if *stateFile != "" {
		if err := appendAuditTrail(*stateFile, "dedup_finalized", recordsToDelete, len(originalRecords), len(filteredRecords), backupPath); err != nil {
			log.Printf("[WARN] audit trail: %v", err)
		}
	}

	// Clean up dedup results file
	os.Remove(*resultsFile)

	fmt.Printf("FINALIZED: %d deleted | %d remaining\n", len(recordsToDelete), len(filteredRecords))
	fmt.Printf("Records.json updated. Backup: %s\n", backupPath)
	log.Printf("[SUCCESS] Finalization complete: deleted=%d, remaining=%d", len(recordsToDelete), len(filteredRecords))

	return nil
}

func runDedupExecute(args []string) error {
	fs := flag.NewFlagSet("dedup execute", flag.ExitOnError)
	recordsFile := fs.String("records-file", "", "path to records.json")
	decisionFile := fs.String("decisions-file", "", "path to decisions.json (from review command)")
	stateFile := fs.String("state-file", "", "path to state.json for audit trail (optional)")
	dryRun := fs.Bool("dry-run", false, "show what would be deleted without making changes")
	fs.Parse(args)

	// Resolve paths
	if *recordsFile == "" {
		*recordsFile = resolveDataPath("wallet/records.jsonl", "records.jsonl")
	}
	if *decisionFile == "" {
		return fmt.Errorf("--decisions-file required (from dedup review command)")
	}
	if *stateFile == "" {
		*stateFile = resolveDataPath("wallet/state.json", "state.json")
	}

	resultsFile := strings.TrimSuffix(*recordsFile, ".json") + "-dedup-results.json"
	fmt.Printf("Working directory: %s\n", filepath.Dir(*recordsFile))
	fmt.Printf("Records: %s\n", *recordsFile)
	fmt.Printf("Decisions: %s\n", *decisionFile)
	fmt.Printf("Results will be saved to: %s\n\n", resultsFile)

	// Validate inputs before proceeding
	if err := validateRecordsFile(*recordsFile); err != nil {
		return fmt.Errorf("invalid records: %w", err)
	}

	if err := validateDiskSpace(*recordsFile, 10*1024*1024); err != nil {
		return fmt.Errorf("disk space check: %w", err)
	}

	// Execute dedup
	if err := executeDedup(*recordsFile, *decisionFile, *stateFile, *dryRun); err != nil {
		return err
	}

	return nil
}
