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
//
// Run `expenses update-event --help` for flags. See RUNBOOK.md for setup and
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
	csvPath := fs.String("csv", "../gmail/transactions.csv", "path to transactions.csv (read-only)")
	registryPath := fs.String("events", "config/events.json", "path to the event registry")
	statePath := fs.String("state", "state.json", "path to the assignment ledger")
	provider := fs.String("ai-provider", envOr("AI_PROVIDER", ""), "AI provider (default: deepseek)")
	model := fs.String("ai-model", "", "override model (else DEEPSEEK_MODEL or built-in default)")
	threshold := fs.Float64("threshold", 0.6, "confidence cutoff (0-1) to accept a match to an existing event")
	batchSize := fs.Int("batch-size", 0, "transactions per API call (0 = one single call for all unassigned rows)")
	limit := fs.Int("limit", 0, "stop after N unassigned rows (0 = all)")
	dryRun := fs.Bool("dry-run", false, "print assignments/new events without writing the registry or state")
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
	}

	res, err := event.Run(context.Background(), cfg)
	if err != nil {
		return err
	}
	log.Printf("done: %d unassigned | %d assigned | %d new event(s) | %d no-event | %d malformed",
		res.Total, res.Assigned, res.NewEvents, res.NoEvent, res.Malformed)
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
  expenses update-event [flags]

Flags (update-event):
  --csv PATH         transactions.csv to read, read-only (default ../gmail/transactions.csv)
  --events PATH      event registry, versioned (default config/events.json)
  --state PATH       assignment ledger, local (default state.json)
  --ai-provider NAME  "" or "deepseek" (default deepseek)
  --ai-model NAME     override model (else DEEPSEEK_MODEL or built-in default)
  --threshold N       confidence cutoff to accept a match to an existing event (default 0.6)
  --batch-size N      transactions per API call (0 = one call for all unassigned rows)
  --limit N           stop after N unassigned rows (0 = all)
  --dry-run           report only; nothing written

Setup and design: see RUNBOOK.md and docs/adr/0011-expenses-pack.md
`)
}
