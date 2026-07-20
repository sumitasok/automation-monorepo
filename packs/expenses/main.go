// Command expenses clusters gmail-extracted transactions into ad-hoc
// "events" (e.g. a trip, a festival) using an AI provider, keeping a
// versioned registry of known events so later transactions are recognised as
// belonging to an event created by an earlier run instead of spawning
// duplicates.
//
// Subcommands:
//
//	update-event   read transactions.csv (read-only), match each not-yet-
//	               assigned row against the known event registry or propose a
//	               new event, and persist the assignment.
//	bulk-assign    read a CSV with MessageID,EventID columns and import those
//	               manual assignments into the state ledger. Manual assignments
//	               override existing auto-matches.
//	fill-similar   for events with manual assignments, use AI to find similar
//	               unassigned transactions and assign them too.
//
// Run `expenses <cmd> --help` for flags. See RUNBOOK.md for setup and
// docs/adr/0011-expenses-pack.md for design rationale.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/sumitasok/sa.automation.expenses/internal/event"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "update-event":
		if err := runUpdateEvent(os.Args[2:]); err != nil {
			log.Fatalf("error: %v", err)
		}
	case "bulk-assign":
		if err := runBulkAssign(os.Args[2:]); err != nil {
			log.Fatalf("error: %v", err)
		}
	case "fill-similar":
		if err := runFillSimilar(os.Args[2:]); err != nil {
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

func runUpdateEvent(args []string) error {
	fs := flag.NewFlagSet("update-event", flag.ExitOnError)
	csvPath := fs.String("csv", "../gmail/transactions.csv", "path to transactions.csv")
	registryPath := fs.String("events", "config/events.json", "path to the event registry")
	statePath := fs.String("state", "state.json", "path to the assignment ledger")
	provider := fs.String("ai-provider", envOr("AI_PROVIDER", ""), "AI provider (default: deepseek)")
	model := fs.String("ai-model", "", "override model (else DEEPSEEK_MODEL or built-in default)")
	threshold := fs.Float64("threshold", 0.6, "confidence cutoff (0-1) to accept a match to an existing event")
	batchSize := fs.Int("batch-size", 0, "transactions per API call (0 = one single call for all unassigned rows)")
	limit := fs.Int("limit", 0, "stop after N unassigned rows (0 = all)")
	dryRun := fs.Bool("dry-run", false, "print assignments/new events without writing the registry or state")
	writeCsv := fs.Bool("write-csv", false, "enrich transactions.csv with EventID and EventDescription columns after processing")
	fs.Parse(args)

	cfg := event.Config{
		CSVPath:      *csvPath,
		RegistryPath: *registryPath,
		StatePath:    *statePath,
		Provider:     *provider,
		Model:        *model,
		Threshold:    *threshold,
		BatchSize:    *batchSize,
		Limit:        *limit,
		DryRun:       *dryRun,
		WriteCsv:     *writeCsv,
	}

	res, err := event.Run(context.Background(), cfg)
	if err != nil {
		return err
	}
	log.Printf("done: %d unassigned | %d assigned | %d new event(s) | %d no-event | %d malformed",
		res.Total, res.Assigned, res.NewEvents, res.NoEvent, res.Malformed)
	return nil
}

func runFillSimilar(args []string) error {
	fs := flag.NewFlagSet("fill-similar", flag.ExitOnError)
	csvPath := fs.String("csv", "../gmail/transactions.csv", "path to transactions.csv (read-only)")
	registryPath := fs.String("events", "config/events.json", "path to the event registry")
	statePath := fs.String("state", "state.json", "path to the assignment ledger")
	provider := fs.String("ai-provider", envOr("AI_PROVIDER", ""), "AI provider (default: deepseek)")
	model := fs.String("ai-model", "", "override model (else DEEPSEEK_MODEL or built-in default)")
	threshold := fs.Float64("threshold", 0.6, "confidence cutoff (0-1) to accept a match")
	batchSize := fs.Int("batch-size", 0, "transactions per API call (0 = one single call for all unassigned rows)")
	dryRun := fs.Bool("dry-run", false, "print assignments without writing the state")
	fs.Parse(args)

	cfg := event.FillSimilarConfig{
		CSVPath:      *csvPath,
		RegistryPath: *registryPath,
		StatePath:    *statePath,
		Provider:     *provider,
		Model:        *model,
		Threshold:    *threshold,
		BatchSize:    *batchSize,
		DryRun:       *dryRun,
	}

	res, err := event.FillSimilar(context.Background(), cfg)
	if err != nil {
		return err
	}
	log.Printf("done: %d total unassigned | %d assigned via similarity | %d no-match | %d events processed",
		res.Total, res.Assigned, res.NoMatch, res.EventsToFill)
	return nil
}

func runBulkAssign(args []string) error {
	fs := flag.NewFlagSet("bulk-assign", flag.ExitOnError)
	assignmentPath := fs.String("assignments", "", "path to CSV with MessageID,EventID columns (required)")
	txnPath := fs.String("transactions", "../gmail/transactions.csv", "path to transactions.csv (to validate MessageIDs)")
	registryPath := fs.String("events", "config/events.json", "path to the event registry (to validate EventIDs)")
	statePath := fs.String("state", "state.json", "path to the assignment ledger (to merge assignments)")
	dryRun := fs.Bool("dry-run", false, "print assignments without writing state")
	fs.Parse(args)

	if *assignmentPath == "" {
		fmt.Fprintf(os.Stderr, "error: --assignments flag is required\n")
		fs.Usage()
		os.Exit(2)
	}

	cfg := event.BulkAssignConfig{
		AssignmentCSVPath: *assignmentPath,
		TransactionCSVPath: *txnPath,
		RegistryPath:      *registryPath,
		StatePath:         *statePath,
		DryRun:            *dryRun,
	}

	res, err := event.BulkAssign(cfg)
	if err != nil {
		return err
	}
	log.Printf("done: %d valid | %d invalid-event | %d invalid-txn | %d duplicates | %d overwrote",
		res.Valid, res.InvalidEvent, res.InvalidTxn, res.Duplicates, res.Overwritten)
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func usage() {
	fmt.Fprint(os.Stderr, `expenses — cluster gmail-extracted transactions into AI-recognised events

Usage:
  expenses update-event [flags]    AI-powered event matching for unassigned transactions
  expenses bulk-assign [flags]     import manual event assignments from CSV
  expenses fill-similar [flags]    find unassigned transactions similar to manually-assigned ones
  expenses --help                  show this help

Flags (update-event):
  --csv PATH         transactions.csv to read and optionally enrich (default ../gmail/transactions.csv)
  --events PATH      event registry, versioned (default config/events.json)
  --state PATH       assignment ledger, local (default state.json)
  --ai-provider NAME  "" or "deepseek" (default deepseek)
  --ai-model NAME     override model (else DEEPSEEK_MODEL or built-in default)
  --threshold N       confidence cutoff to accept a match to an existing event (default 0.6)
  --batch-size N      transactions per API call (0 = one call for all unassigned rows)
  --limit N           stop after N unassigned rows (0 = all)
  --dry-run           report only; nothing written
  --write-csv         enrich transactions.csv with EventID and EventDescription columns

Flags (bulk-assign):
  --assignments PATH  CSV file with MessageID,EventID columns (required)
  --transactions PATH path to transactions.csv (default ../gmail/transactions.csv)
  --events PATH       event registry, versioned (default config/events.json)
  --state PATH        assignment ledger, local (default state.json)
  --dry-run           report only; nothing written

Flags (fill-similar):
  --csv PATH         transactions.csv to read (default ../gmail/transactions.csv)
  --events PATH      event registry, versioned (default config/events.json)
  --state PATH       assignment ledger, local (default state.json)
  --ai-provider NAME  "" or "deepseek" (default deepseek)
  --ai-model NAME     override model (else DEEPSEEK_MODEL or built-in default)
  --threshold N       confidence cutoff to accept a match (default 0.6)
  --batch-size N      transactions per API call (0 = one call for all unassigned rows)
  --dry-run           report only; nothing written

Setup and design: see RUNBOOK.md and docs/adr/0011-expenses-pack.md
`)
}
