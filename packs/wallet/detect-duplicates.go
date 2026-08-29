// Command detect-duplicates finds potential duplicate wallet records by checking
// if the same Gmail MessageID or MessageID+Amount combination appears multiple times
// in the transactions.csv or if they've been synced multiple times to the wallet app.
//
// Usage:
//   go run . detect-duplicates
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sumitasok/sa.automation.wallet/internal/csvtxn"
	"github.com/sumitasok/sa.automation.wallet/internal/state"
)

type Duplicate struct {
	MessageID  string
	Amount     float64
	Count      int
	Dates      []string
	RecordIDs  []string
	Note       string
}

type DuplicateReport struct {
	Timestamp           string
	CSVDuplicates       []Duplicate
	StateDuplicates     []Duplicate
	CrossCheckIssues    []string
	TotalDuplicateCount int
	Recommendations     []string
}

func detectDuplicates(csvPath, statePath string) (DuplicateReport, error) {
	report := DuplicateReport{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Check CSV for duplicate MessageIDs
	csvDups := checkCSVDuplicates(csvPath)
	report.CSVDuplicates = csvDups

	// Check state.json for duplicate MessageIDs
	stateDups := checkStateDuplicates(statePath)
	report.StateDuplicates = stateDups

	// Cross-check: same MessageID synced multiple times with different amounts
	st, err := state.Load(statePath)
	if err == nil {
		txns, _, _ := csvtxn.Read(csvPath, time.UTC)
		report.CrossCheckIssues = checkCrossIssues(txns, st)
	}

	// Count total duplicates
	for _, d := range csvDups {
		report.TotalDuplicateCount += d.Count - 1
	}
	for _, d := range stateDups {
		report.TotalDuplicateCount += d.Count - 1
	}

	// Generate recommendations
	if len(csvDups) > 0 {
		report.Recommendations = append(report.Recommendations,
			"CSV contains duplicate MessageIDs — check if gmail-extract ran multiple times or if there are duplicate emails in your mailbox")
	}
	if len(stateDups) > 0 {
		report.Recommendations = append(report.Recommendations,
			"Multiple records synced for same MessageID — wallet may contain duplicates. Review manually in wallet app.")
	}
	if len(report.CrossCheckIssues) > 0 {
		report.Recommendations = append(report.Recommendations,
			"Data inconsistencies detected — review state.json against transactions.csv. May need manual cleanup in wallet app.")
	}
	if report.TotalDuplicateCount == 0 {
		report.Recommendations = append(report.Recommendations,
			"✓ No duplicates detected. Deduplication is working correctly.")
	}

	return report, nil
}

func checkCSVDuplicates(csvPath string) []Duplicate {
	txns, _, err := csvtxn.Read(csvPath, time.UTC)
	if err != nil {
		return []Duplicate{{Note: fmt.Sprintf("Error reading CSV: %v", err)}}
	}

	byID := make(map[string][]string)
	for _, t := range txns {
		date := t.Date.Format("2006-01-02")
		byID[t.MessageID] = append(byID[t.MessageID], date)
	}

	var dups []Duplicate
	for msgID, dates := range byID {
		if len(dates) > 1 {
			dups = append(dups, Duplicate{
				MessageID: msgID,
				Count:     len(dates),
				Dates:     dates,
				Note:      "Duplicate in CSV",
			})
		}
	}

	sort.Slice(dups, func(i, j int) bool {
		return dups[i].Count > dups[j].Count
	})
	return dups
}

func checkStateDuplicates(statePath string) []Duplicate {
	raw, err := os.ReadFile(statePath)
	if err != nil {
		return []Duplicate{{Note: fmt.Sprintf("Error reading state: %v", err)}}
	}

	var st struct {
		Pushed map[string]struct {
			RecordID string  `json:"recordId"`
			Date     string  `json:"date"`
			Amount   float64 `json:"amount"`
		} `json:"pushed"`
	}

	if err := json.Unmarshal(raw, &st); err != nil {
		return []Duplicate{{Note: fmt.Sprintf("Error parsing state: %v", err)}}
	}

	byRecordID := make(map[string][]string)
	for msgID, entry := range st.Pushed {
		if entry.RecordID != "" {
			byRecordID[entry.RecordID] = append(byRecordID[entry.RecordID], msgID)
		}
	}

	var dups []Duplicate
	for recordID, msgIDs := range byRecordID {
		if len(msgIDs) > 1 {
			dups = append(dups, Duplicate{
				MessageID: strings.Join(msgIDs, ","),
				RecordIDs: []string{recordID},
				Count:     len(msgIDs),
				Note:      "Multiple MessageIDs for one RecordID",
			})
		}
	}

	sort.Slice(dups, func(i, j int) bool {
		return dups[i].Count > dups[j].Count
	})
	return dups
}

func checkCrossIssues(txns []csvtxn.Txn, st *state.State) []string {
	var issues []string
	seen := make(map[string]float64)

	for _, t := range txns {
		amount := t.SignedAmount()
		key := t.MessageID
		if prevAmount, exists := seen[key]; exists {
			if prevAmount != amount {
				issues = append(issues, fmt.Sprintf(
					"MessageID %s appears with different amounts: %.2f vs %.2f",
					key, prevAmount, amount))
			}
		}
		seen[key] = amount
	}

	return issues
}

func runDetectDuplicates(args []string) error {
	fs := flag.NewFlagSet("detect-duplicates", flag.ExitOnError)
	csvPath := fs.String("csv", "", "path to transactions.csv (default: $AUTO_DATA_DIR/gmail/transactions.csv)")
	statePath := fs.String("state", "", "path to wallet state.json (default: $AUTO_DATA_DIR/wallet/state.json)")
	reportFormat := fs.String("format", "text", "output format: text or json")
	fs.Parse(args)

	// Resolve data paths
	resolvedCSVPath := *csvPath
	if resolvedCSVPath == "" {
		if dataDir := os.Getenv("AUTO_DATA_DIR"); dataDir != "" {
			resolvedCSVPath = filepath.Join(dataDir, "gmail/transactions.csv")
		} else {
			resolvedCSVPath = "../gmail/transactions.csv"
		}
	}
	resolvedStatePath := *statePath
	if resolvedStatePath == "" {
		if dataDir := os.Getenv("AUTO_DATA_DIR"); dataDir != "" {
			resolvedStatePath = filepath.Join(dataDir, "wallet/state.json")
		} else {
			resolvedStatePath = "state.json"
		}
	}

	report, err := detectDuplicates(resolvedCSVPath, resolvedStatePath)
	if err != nil {
		return err
	}

	if *reportFormat == "json" {
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	fmt.Printf("=== Duplicate Detection Report ===\n")
	fmt.Printf("Generated: %s\n\n", report.Timestamp)

	if len(report.CSVDuplicates) > 0 {
		fmt.Printf("CSV Duplicates Found (%d sets):\n", len(report.CSVDuplicates))
		for _, dup := range report.CSVDuplicates {
			fmt.Printf("  MessageID: %s\n    Count: %d\n    Note: %s\n", dup.MessageID, dup.Count, dup.Note)
		}
		fmt.Println()
	}

	if len(report.StateDuplicates) > 0 {
		fmt.Printf("State Duplicates Found (%d sets):\n", len(report.StateDuplicates))
		for _, dup := range report.StateDuplicates {
			fmt.Printf("  MessageID: %s\n    Count: %d\n    Note: %s\n", dup.MessageID, dup.Count, dup.Note)
		}
		fmt.Println()
	}

	if len(report.CrossCheckIssues) > 0 {
		fmt.Printf("Cross-Check Issues Found (%d):\n", len(report.CrossCheckIssues))
		for _, issue := range report.CrossCheckIssues {
			fmt.Printf("  - %s\n", issue)
		}
		fmt.Println()
	}

	fmt.Printf("Total Duplicate Records: %d\n", report.TotalDuplicateCount)
	fmt.Println()

	fmt.Println("Recommendations:")
	for i, rec := range report.Recommendations {
		fmt.Printf("%d. %s\n", i+1, rec)
	}

	return nil
}
