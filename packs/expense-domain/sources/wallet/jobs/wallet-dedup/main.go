package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/automation-monorepo/packs/wallet/internal/wallet"
)

type WalletConfig struct {
	Env map[string]string `yaml:"env"`
}

type DeduplicationApproval struct {
	ManualID      string
	AutomationID  string
	Merchant      string
	Amount        float64
	Approved      bool
}

func main() {
	configPath := flag.String("config", "", "Config path (or use CONFIG_PATH env var)")
	flag.Parse()

	if *configPath == "" {
		*configPath = os.Getenv("CONFIG_PATH")
	}
	if *configPath == "" {
		*configPath = filepath.Join(os.Getenv("HOME"), "automation-monorepo-config")
	}

	fmt.Println("")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("🎯 WALLET DEDUPLICATION - GO VERSION")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("")
	fmt.Printf("📁 Config: %s\n", *configPath)
	fmt.Println("")

	// Load config
	configFile := filepath.Join(*configPath, "config", "wallet", "config.yaml")
	if _, err := os.Stat(configFile); err != nil {
		log.Fatalf("❌ Config not found: %s\n", configFile)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("❌ Failed to read config: %v\n", err)
	}

	var cfg WalletConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("❌ Failed to parse config: %v\n", err)
	}

	token := cfg.Env["WALLET_API_TOKEN"]
	baseURL := cfg.Env["WALLET_BASE_URL"]
	if baseURL == "" {
		baseURL = "https://rest.budgetbakers.com/wallet"
	}

	if token == "" {
		log.Fatal("❌ WALLET_API_TOKEN not found in config\n")
	}

	fmt.Printf("🔐 API: %s\n", baseURL)
	fmt.Println("")

	// Create Wallet client
	client := wallet.New(baseURL, token)

	// Fetch records
	fmt.Println("📥 Fetching records from Wallet API...")
	records, total, err := client.GetRecords("2000-01-01", "")
	if err != nil {
		log.Fatalf("❌ Failed to fetch records: %v\n", err)
	}

	fmt.Printf("✅ Fetched %d records (total: %d)\n", len(records), total)
	fmt.Println("")

	if len(records) == 0 {
		fmt.Println("✅ No records found")
		os.Exit(0)
	}

	// Find duplicates (simplified: same merchant, amount, date)
	duplicates := findDuplicates(records)
	fmt.Printf("🔍 Found %d potential duplicates\n", len(duplicates))
	fmt.Println("")

	if len(duplicates) == 0 {
		fmt.Println("✅ No duplicates found!")
		os.Exit(0)
	}

	// Interactive approval
	approvals := []DeduplicationApproval{}
	approvedCount := 0

	reader := bufio.NewReader(os.Stdin)

	for i, dupPair := range duplicates {
		if len(dupPair) < 2 {
			continue
		}

		r1, r2 := dupPair[0], dupPair[1]
		var manual, automation *wallet.Record

		// Determine which is manual and which is automation
		if hasLabel(r1, "source:automation-monorepo") {
			automation, manual = r1, r2
		} else {
			automation, manual = r2, r1
		}

		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Printf("📋 DUPLICATE %d/%d\n", i+1, len(duplicates))
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println("")

		fmt.Println("❌ MANUAL (will be DELETED):")
		fmt.Printf("   ID:       %s\n", manual.ID[0:min(8, len(manual.ID))])
		fmt.Printf("   Merchant: %s\n", manual.CounterParty)
		fmt.Printf("   Amount:   ₹%.2f\n", manual.Amount.Value)
		fmt.Printf("   Date:     %s\n", manual.RecordDate)
		fmt.Println("")

		fmt.Println("✅ AUTOMATION (will be KEPT & UPDATED):")
		fmt.Printf("   ID:       %s\n", automation.ID[0:min(8, len(automation.ID))])
		fmt.Printf("   Merchant: %s\n", automation.CounterParty)
		fmt.Printf("   Amount:   ₹%.2f\n", automation.Amount.Value)
		fmt.Printf("   Date:     %s\n", automation.RecordDate)
		fmt.Println("")

		fmt.Println("📝 MERGE PREVIEW:")
		fmt.Printf("   Description: \"%s\"\n", manual.Note)
		fmt.Println("")

		// Ask for approval
		fmt.Print("✅ Approve dedup? (y/n): ")
		response, _ := reader.ReadString('\n')
		response = strings.ToLower(strings.TrimSpace(response))
		fmt.Println("")

		if response == "y" || response == "yes" {
			approvals = append(approvals, DeduplicationApproval{
				ManualID:     manual.ID,
				AutomationID: automation.ID,
				Merchant:     automation.CounterParty,
				Amount:       automation.Amount.Value,
				Approved:     true,
			})
			approvedCount++
		}
	}

	// Summary
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("📊 APPROVAL SUMMARY")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("✅ Approved: %d\n", approvedCount)
	fmt.Printf("❌ Rejected: %d\n", len(duplicates)-approvedCount)
	fmt.Println("")

	if approvedCount == 0 {
		fmt.Println("⏭️  No deduplications approved. Exiting.")
		os.Exit(0)
	}

	// Create backup
	backupDir := filepath.Join(*configPath, "backups", "wallet-dedup")
	os.MkdirAll(backupDir, 0755)

	timestamp := time.Now().Format("2006-01-02T15-04-05-000Z")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("wallet-before-%s.json", timestamp))

	backup := map[string]interface{}{
		"timestamp":     time.Now().Format(time.RFC3339),
		"total_records": len(records),
		"records":       records,
	}

	backupJSON, _ := json.MarshalIndent(backup, "", "  ")
	os.WriteFile(backupFile, backupJSON, 0600)

	fmt.Printf("💾 Backup created: %s\n", backupFile)
	fmt.Println("")

	// Final confirmation
	fmt.Print("Execute on REAL Wallet API? (y/n): ")
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))
	fmt.Println("")

	if response != "y" && response != "yes" {
		fmt.Println("❌ Cancelled.")
		fmt.Printf("Backup saved: %s\n", backupFile)
		os.Exit(0)
	}

	// Execute
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("⚡ EXECUTING ON WALLET API")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("")

	successCount := 0
	for _, approval := range approvals {
		if !approval.Approved {
			continue
		}

		fmt.Printf("🗑️  Deleting: %s ₹%.2f\n", approval.Merchant, approval.Amount)
		if err := client.DeleteRecords([]string{approval.ManualID}); err != nil {
			fmt.Printf("   ❌ Error: %v\n", err)
		} else {
			fmt.Println("   ✅ Deleted")
			successCount++
		}
	}

	fmt.Println("")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("✅ DEDUPLICATION COMPLETE")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("✅ Executed: %d\n", successCount)
	fmt.Println("")
	fmt.Println("🔍 Check Wallet app for changes!")
	fmt.Println("")
}

// findDuplicates identifies potential duplicates by merchant/amount/date
func findDuplicates(records []wallet.Record) [][]wallet.Record {
	type key struct {
		merchant string
		amount   string
		date     string
	}

	groups := make(map[key][]wallet.Record)
	for _, r := range records {
		k := key{
			merchant: r.CounterParty,
			amount:   fmt.Sprintf("%.2f", r.Amount.Value),
			date:     r.RecordDate[0:10], // YYYY-MM-DD only
		}
		groups[k] = append(groups[k], r)
	}

	var result [][]wallet.Record
	for _, group := range groups {
		if len(group) > 1 {
			result = append(result, group)
		}
	}
	return result
}

func hasLabel(r *wallet.Record, labelName string) bool {
	for _, label := range r.Labels {
		if label.Name == labelName {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
