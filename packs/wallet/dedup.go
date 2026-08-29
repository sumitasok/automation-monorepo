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
	"fmt"
	"os"
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
	PrimaryKeys  []string `json:"primaryKeys"`
	OptionalKeys []string `json:"optionalKeys"`
	MinConfidence float64  `json:"minConfidence"`
}

// detectRecordDuplicates loads records.json into a working copy and identifies duplicates.
// The original records.json is NEVER modified. All operations happen on the working copy.
// Returns the list of duplicate groups found in the working copy.
func detectRecordDuplicates(recordsFile, configPath string, minConfidence float64) ([]DuplicateGroup, error) {
	// 1. Load records.json into memory (working copy)
	// 2. Run dedup detection on the working copy
	// 3. Return duplicate groups (original records.json untouched)
	return nil, nil
}

// reviewDuplicates collects user decisions on duplicate groups (from working copy).
// No modifications to records.json. Decisions are saved separately.
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
		usage()
		os.Exit(2)
	}

	switch args[0] {
	case "scan":
		return runDedupScan(args[1:])
	case "review":
		return runDedupReview(args[1:])
	case "execute":
		return runDedupExecute(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown dedup command %q\n", args[0])
		usage()
		os.Exit(2)
	}
	return nil
}

func runDedupScan(args []string) error {
	// TODO: Implement scan subcommand
	return fmt.Errorf("scan command not yet implemented")
}

func runDedupReview(args []string) error {
	// TODO: Implement review subcommand
	return fmt.Errorf("review command not yet implemented")
}

func runDedupExecute(args []string) error {
	// TODO: Implement execute subcommand
	return fmt.Errorf("execute command not yet implemented")
}
