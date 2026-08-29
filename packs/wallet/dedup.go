// Package main implements wallet record deduplication.
//
// The dedup subcommand identifies, reviews, and removes duplicate transaction records
// from records.json. Three operations are supported:
//   - scan: Identify duplicates without modifications
//   - review: Collect user decisions on which records to keep/delete
//   - execute: Apply decisions atomically with backups and audit trail
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

func detectRecordDuplicates(recordsFile, configPath string, minConfidence float64) ([]DuplicateGroup, error) {
	// TODO: Implement duplicate detection logic
	return nil, nil
}

func reviewDuplicates(groups []DuplicateGroup, interactive bool) ([]DedupDecision, error) {
	// TODO: Implement review logic
	return nil, nil
}

func executeDuplicates(recordsFile string, decisions []DedupDecision, dryRun bool) error {
	// TODO: Implement execution logic
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
