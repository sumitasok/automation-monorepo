// Package main implements wallet record deduplication.
//
// The dedup subcommand identifies, reviews, and removes duplicate transaction records.
// IMPORTANT: All operations happen on a working copy in memory. records.json is never
// modified until the final atomic write after user confirmation and verification.
//
// Workflow:
//   1. scan: Load records.json → create working copy → detect duplicates → report findings
//   2. review: Load working copy state → collect user decisions → save to decisions.json
//   3. execute: Load working copy → apply decisions → backup records.json → atomic write
//
// Three operations are supported:
//   - scan: Identify duplicates without touching records.json
//   - review: Collect user decisions on which records to keep/delete
//   - execute: Apply decisions atomically (backup before write, verify after)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sumitasok/sa.automation.wallet/internal/wallet"
)

// DuplicateGroup represents a set of duplicate records matching on amount+date+counterparty.
type DuplicateGroup struct {
	DuplicateKey string              `json:"duplicateKey"`
	MatchType    string              `json:"matchType"` // "exact" or "uncertain"
	Confidence   float64             `json:"confidence"`
	Records      []RecordSummary     `json:"records"`
}

// RecordSummary is a minimal record representation for display and decisions.
type RecordSummary struct {
	ID         string                 `json:"id"`
	CreatedAt  string                 `json:"createdAt"`
	IsOriginal bool                   `json:"isOriginal"`
	CounterParty string                `json:"counterParty"`
	Amount     float64                `json:"amount"`
	Category   string                 `json:"category"`
}

// DedupDecision represents a user's choice for a duplicate group.
type DedupDecision struct {
	DuplicateKey  string   `json:"duplicateKey"`
	Action        string   `json:"action"` // "keep_first_delete_rest", "custom", "skip"
	KeepRecordIDs []string `json:"keepRecordIds"`
	DeleteRecordIDs []string `json:"deleteRecordIds"`
	Reason        string   `json:"reason"`
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
	data, err := os.ReadFile(recordsFile)
	if err != nil {
		return nil, fmt.Errorf("load records: %w", err)
	}

	var snap RecordsSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parse records.json: %w", err)
	}

	return &snap, nil
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
		Timestamp       string   `json:"timestamp"`
		Operation       string   `json:"operation"`
		DeletedRecordIDs []string `json:"deletedRecordIds"`
		TotalRecordsBefore int    `json:"totalRecordsBefore"`
		TotalRecordsAfter  int    `json:"totalRecordsAfter"`
		BackupFile      string   `json:"backupFile"`
	}

	entry := auditEntry{
		Timestamp:         time.Now().UTC().Format(time.RFC3339),
		Operation:         operation,
		DeletedRecordIDs:  deletedIDs,
		TotalRecordsBefore: countBefore,
		TotalRecordsAfter:  countAfter,
		BackupFile:        backupFile,
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

	for _, part := range parts {
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
			val, _ := getFieldValue(rec, keyField)
			keyParts = append(keyParts, fmt.Sprintf("%v", val))
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

// formatGroupsText formats duplicate groups as human-readable text output.
func formatGroupsText(groups []DuplicateGroup) string {
	if len(groups) == 0 {
		return "No duplicate records found.\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== Dedup Scan Results ===\nDuplicate Groups Found: %d\n\n", len(groups)))

	for i, group := range groups {
		sb.WriteString(fmt.Sprintf("Group %d: %s\n", i+1, group.DuplicateKey))
		sb.WriteString(fmt.Sprintf("  Match Type: %s (confidence: %.0f%%)\n", group.MatchType, group.Confidence*100))
		for j, rec := range group.Records {
			marker := "duplicate"
			if rec.IsOriginal {
				marker = "original"
			}
			sb.WriteString(fmt.Sprintf("  Record %d (%s) [%s]\n", j+1, marker, rec.CreatedAt))
			sb.WriteString(fmt.Sprintf("    ID: %s\n", rec.ID))
			sb.WriteString(fmt.Sprintf("    Amount: %.2f %s\n", rec.Amount, ""))
			sb.WriteString(fmt.Sprintf("    Counterparty: %s\n", rec.CounterParty))
			if rec.Category != "" {
				sb.WriteString(fmt.Sprintf("    Category: %s\n", rec.Category))
			}
		}
		sb.WriteString("\n")
	}

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
	recordsFile := fs.String("records-file", "", "path to records.json (default: $AUTO_DATA_DIR/wallet/records.json or ./records.json)")
	dedupConfig := fs.String("dedup-config", "", "path to dedup config (default: config.yaml)")
	format := fs.String("format", "text", "output format: text or json")
	minConfidence := fs.Float64("min-confidence", 0.5, "minimum confidence threshold (0.0-1.0)")
	fs.Parse(args)

	// Resolve paths
	if *recordsFile == "" {
		*recordsFile = resolveDataPath("wallet/records.json", "records.json")
	}

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

// reviewDuplicates collects user decisions on duplicate groups (from working copy).
// No modifications to records.json. Decisions are saved separately.
// NOTE: Implemented in Phase 4 (review operation)
func reviewDuplicates(groups []DuplicateGroup, interactive bool) ([]DedupDecision, error) {
	// 1. Present duplicate groups from working copy to user
	// 2. Collect which records to keep/delete
	// 3. Save decisions to decisions.json
	// 4. Original records.json untouched
	return nil, nil
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

func runDedup(args []string) error {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, `wallet dedup — identify and remove duplicate transaction records

Usage:
  wallet dedup scan [flags]      identify duplicates (read-only)
  wallet dedup review [flags]    collect user decisions on which to delete
  wallet dedup execute [flags]   apply decisions atomically with backup

Run 'wallet dedup scan --help' for scan flags.
`)
		os.Exit(2)
	}

	switch args[0] {
	case "scan":
		return runDedupScan(args[1:])
	case "review":
		return runDedupReview(args[1:])
	case "execute":
		return runDedupExecute(args[1:])
	case "-h", "--help", "help":
		fmt.Fprintf(os.Stderr, `wallet dedup — identify and remove duplicate transaction records

Usage:
  wallet dedup scan [flags]      identify duplicates (read-only)
  wallet dedup review [flags]    collect user decisions on which to delete
  wallet dedup execute [flags]   apply decisions atomically with backup
`)
	default:
		fmt.Fprintf(os.Stderr, "unknown dedup command %q\n", args[0])
		os.Exit(2)
	}
	return nil
}

func runDedupReview(args []string) error {
	// TODO: Implement review subcommand (Phase 4)
	return fmt.Errorf("review command not yet implemented")
}

func runDedupExecute(args []string) error {
	// TODO: Implement execute subcommand (Phase 5)
	return fmt.Errorf("execute command not yet implemented")
}
