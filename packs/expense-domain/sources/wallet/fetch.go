package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sumitasok/sa.automation.wallet/internal/config"
	"github.com/sumitasok/sa.automation.wallet/internal/wallet"
)

func runFetch(args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ExitOnError)
	outputFile := fs.String("output", "", "path to save records (default: $AUTO_DATA_DIR/wallet/records.jsonl)")
	since := fs.String("since", "", "only fetch records updated after this date (YYYY-MM-DD)")
	fs.Parse(args)

	// Resolve output path
	if *outputFile == "" {
		*outputFile = resolveDataPath("wallet/records.jsonl", "records.jsonl")
	}

	fmt.Printf("Working directory: %s\n", filepath.Dir(*outputFile))
	fmt.Printf("Records will be saved to: %s\n\n", *outputFile)

	// Load config
	cfg, err := config.Load("", true)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Fetch records from API
	client := wallet.New(cfg.BaseURL, cfg.APIToken)
	fmt.Println("Fetching records from Wallet API...")

	records, total, err := client.GetRecords("", *since)
	if err != nil {
		return fmt.Errorf("fetch records: %w", err)
	}

	// Save to JSONL
	if err := os.MkdirAll(filepath.Dir(*outputFile), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	file, err := os.Create(*outputFile)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	// Write metadata header as first line
	metadata := map[string]interface{}{
		"fetchedAt":   time.Now().UTC().Format(time.RFC3339),
		"recordCount": len(records),
		"apiTotal":    total,
	}
	metaData, _ := json.Marshal(metadata)
	writer.WriteString(string(metaData) + "\n")

	// Write records (one per line)
	for i, rec := range records {
		data, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshal record %d: %w", i, err)
		}
		writer.WriteString(string(data) + "\n")
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush file: %w", err)
	}

	fmt.Printf("FETCHED: %d records from Wallet API\n", len(records))
	fmt.Printf("Saved to: %s\n", *outputFile)
	if total > len(records) {
		fmt.Printf("Note: API reports %d total records (incremental fetch shows %d)\n", total, len(records))
	}

	return nil
}
