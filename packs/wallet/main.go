// Command wallet syncs transactions extracted by the gmail pack into the
// BudgetBakers Wallet app via the Wallet REST API.
//
// Subcommands:
//
//	sync                 read transactions.csv and create one Wallet record per transaction,
//	                     processed day by day, deduped by MessageID+Amount, tagged with a label.
//	detect-duplicates    find potential duplicate wallet records.
//	dedup                identify, review, and remove duplicate transaction records from records.json.
//	                     Sub-operations: scan, review, execute.
//
// Run `wallet sync --help` for flags. See RUNBOOK.md for setup.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/sumitasok/sa.automation.wallet/internal/config"
	"github.com/sumitasok/sa.automation.wallet/internal/sync"
	"github.com/sumitasok/sa.automation.wallet/internal/wallet"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "sync":
		if err := runSync(os.Args[2:]); err != nil {
			log.Fatalf("error: %v", err)
		}
	case "detect-duplicates":
		if err := runDetectDuplicates(os.Args[2:]); err != nil {
			log.Fatalf("error: %v", err)
		}
	case "dedup":
		if err := runDedup(os.Args[2:]); err != nil {
			log.Fatalf("error: %v", err)
		}
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func runSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	csvPath := fs.String("csv", "", "path to transactions.csv (default: $AUTO_DATA_DIR/gmail/transactions.csv)")
	statePath := fs.String("state", "", "path to dedupe state file (default: $AUTO_DATA_DIR/wallet/state.json)")
	accountsPath := fs.String("accounts", "", "path to accounts.json map (default: $AUTO_PACK_CONFIG_DIR/accounts.json)")
	dryRun := fs.Bool("dry-run", false, "parse, map and report — do not call the API or require a token")
	since := fs.String("since", "", "only sync records on/after this date (YYYY-MM-DD)")
	until := fs.String("until", "", "only sync records on/before this date (YYYY-MM-DD)")
	limit := fs.Int("limit", 0, "cap the number of records pushed (0 = no cap)")
	fs.Parse(args)

	loc, err := time.LoadLocation(envOr("WALLET_TIMEZONE", "Asia/Kolkata"))
	if err != nil {
		return fmt.Errorf("load timezone: %w", err)
	}

	// Resolve data paths: use auto's data dir if env is set, else fall back to relative paths
	resolvedCSVPath := *csvPath
	if resolvedCSVPath == "" {
		resolvedCSVPath = resolveDataPath("gmail/transactions.csv", "../gmail/transactions.csv")
	}
	resolvedStatePath := *statePath
	if resolvedStatePath == "" {
		resolvedStatePath = resolveDataPath("wallet/state.json", "state.json")
	}

	cfg, err := config.Load(*accountsPath, !*dryRun)
	if err != nil {
		return err
	}

	opts := sync.Options{
		CSVPath:   resolvedCSVPath,
		StatePath: resolvedStatePath,
		DryRun:    *dryRun,
		Limit:     *limit,
	}
	if opts.Since, err = parseDay(*since, loc); err != nil {
		return fmt.Errorf("--since: %w", err)
	}
	if opts.Until, err = parseDayEnd(*until, loc); err != nil {
		return fmt.Errorf("--until: %w", err)
	}

	runner := &sync.Runner{
		Cfg: cfg,
		Loc: loc,
		Out: log.Printf,
	}
	if !*dryRun {
		runner.Client = wallet.New(cfg.BaseURL, cfg.APIToken)
	}

	res, err := runner.Run(opts)
	if err != nil {
		return err
	}
	log.Printf("done: %d in csv | created %d | already-synced %d | unmapped %d | out-of-range %d | failed %d | malformed %d",
		res.Total, res.Created, res.Skipped, res.Unmapped, res.OutOfRange, res.Failed, res.Malformed)
	return nil
}

func parseDay(s string, loc *time.Location) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.ParseInLocation("2006-01-02", s, loc)
}

func parseDayEnd(s string, loc *time.Location) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, loc)
	if err != nil {
		return time.Time{}, err
	}
	return t.Add(24*time.Hour - time.Second), nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// resolveDataPath returns the path to a data file, using AUTO_DATA_DIR if set
// (workspace mode via ./auto), otherwise the relative fallback (dev mode).
func resolveDataPath(autoPath, fallback string) string {
	if dataDir := os.Getenv("AUTO_DATA_DIR"); dataDir != "" {
		return filepath.Join(dataDir, autoPath)
	}
	return fallback
}

func runDedup(args []string) error {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "dedup requires a subcommand: scan, review, or execute\n")
		os.Exit(2)
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "scan":
		return runDedupScan(subargs)
	case "review":
		return runDedupReview(subargs)
	case "execute":
		return runDedupExecute(subargs)
	default:
		return fmt.Errorf("unknown dedup subcommand: %s (use scan, review, or execute)", subcommand)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `wallet — sync gmail-extracted transactions into the Wallet app

Usage:
  wallet sync [flags]
  wallet dedup [scan|review|execute] [flags]

Flags (sync):
  --csv PATH        transactions.csv to read (default ../gmail/transactions.csv)
  --state PATH      dedupe state file (default state.json)
  --accounts PATH   accounts.json map (default $AUTO_PACK_CONFIG_DIR/accounts.json)
  --dry-run         report only; no API calls, no token required
  --since YYYY-MM-DD  only records on/after this date
  --until YYYY-MM-DD  only records on/before this date
  --limit N         cap records pushed (0 = no cap)

Flags (dedup scan):
  --records-file    path to records.json (default $AUTO_DATA_DIR/wallet/records.json)
  --dedup-config    path to dedup config in pack.yaml
  --format          text or json (default text)
  --min-confidence  minimum confidence score 0-1 (default 0.5)

Flags (dedup review):
  --records-file    path to records.json
  --dedup-config    path to dedup config
  --decisions-file  path to save decisions (default .dedup-decisions-{timestamp}.json)
  --dry-run         show decisions without saving

Setup and scheduling: see RUNBOOK.md
`)
}
