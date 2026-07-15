// Command wallet syncs transactions extracted by the gmail pack into the
// BudgetBakers Wallet app via the Wallet REST API.
//
// Subcommands:
//
//	sync   read transactions.csv and create one Wallet record per transaction,
//	       processed day by day, deduped by MessageID, tagged with a label.
//
// Run `wallet sync --help` for flags. See RUNBOOK.md for setup.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
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
	csvPath := fs.String("csv", "../gmail/transactions.csv", "path to transactions.csv")
	statePath := fs.String("state", "state.json", "path to dedupe state file")
	accountsPath := fs.String("accounts", "", "path to accounts.json map (default: $AUTO_PACK_CONFIG_DIR/accounts.json, then ./accounts.json)")
	dryRun := fs.Bool("dry-run", false, "parse, map and report — do not call the API or require a token")
	since := fs.String("since", "", "only sync records on/after this date (YYYY-MM-DD)")
	until := fs.String("until", "", "only sync records on/before this date (YYYY-MM-DD)")
	limit := fs.Int("limit", 0, "cap the number of records pushed (0 = no cap)")
	fs.Parse(args)

	loc, err := time.LoadLocation(envOr("WALLET_TIMEZONE", "Asia/Kolkata"))
	if err != nil {
		return fmt.Errorf("load timezone: %w", err)
	}

	cfg, err := config.Load(*accountsPath, !*dryRun)
	if err != nil {
		return err
	}

	opts := sync.Options{
		CSVPath:   *csvPath,
		StatePath: *statePath,
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

func usage() {
	fmt.Fprint(os.Stderr, `wallet — sync gmail-extracted transactions into the Wallet app

Usage:
  wallet sync [flags]

Flags (sync):
  --csv PATH        transactions.csv to read (default ../gmail/transactions.csv)
  --state PATH      dedupe state file (default state.json)
  --accounts PATH   accounts.json map (default $AUTO_PACK_CONFIG_DIR/accounts.json)
  --dry-run         report only; no API calls, no token required
  --since YYYY-MM-DD  only records on/after this date
  --until YYYY-MM-DD  only records on/before this date
  --limit N         cap records pushed (0 = no cap)

Setup and scheduling: see RUNBOOK.md
`)
}
