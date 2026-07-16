package event

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/sumitasok/sa.automation.expenses/internal/csvtxn"
)

// defaultThreshold is the confidence at/above which a model-proposed match to
// an existing event is accepted; below it, the transaction is treated as
// unmatched (candidate for a new event, or no event at all).
const defaultThreshold = 0.6

// Config parameterises an update-event run.
type Config struct {
	CSVPath      string  // path to transactions.csv (read-only)
	RegistryPath string  // path to the event registry (config/events.json)
	StatePath    string  // path to the assignment ledger (state.json)
	Provider     string  // "" or "deepseek"
	Model        string  // override model (else env/default)
	Threshold    float64 // confidence cutoff for accepting a match; <=0 uses defaultThreshold
	BatchSize    int     // transactions per API call (<=0 = one single call for all rows)
	Limit        int     // stop after N unassigned rows (0 = all)
	DryRun       bool    // print assignments/new events without writing registry or state

	// Matcher optionally injects the classifier (used in tests). When nil, Run
	// builds one from Provider/Model via NewMatcher.
	Matcher Matcher
}

// Result summarises a run.
type Result struct {
	Total     int // unassigned rows considered
	Assigned  int // rows matched to an event (existing or newly created)
	NewEvents int // brand-new registry entries created
	NoEvent   int // rows the model deliberately left without an event
	Malformed int // rows that failed CSV normalisation
}

// pending pairs a transaction with the raw match result awaiting grouping
// into a (possibly shared) new event.
type pending struct {
	item Item
	res  MatchResult
}

// Run enriches every not-yet-assigned transaction: it loads the registry and
// ledger, finds rows missing an assignment, asks the AI provider to match them
// against known events (or propose new ones) in batches, and persists the
// result. It returns a summary Result.
func Run(ctx context.Context, cfg Config) (Result, error) {
	var res Result

	threshold := cfg.Threshold
	if threshold <= 0 {
		threshold = defaultThreshold
	}

	reg, err := LoadRegistry(cfg.RegistryPath)
	if err != nil {
		return res, fmt.Errorf("load registry: %w", err)
	}
	st, err := LoadState(cfg.StatePath)
	if err != nil {
		return res, fmt.Errorf("load state: %w", err)
	}

	matcher := cfg.Matcher
	if matcher == nil {
		matcher, err = NewMatcher(cfg.Provider, cfg.Model)
		if err != nil {
			return res, err
		}
	}

	txns, malformed, err := csvtxn.Read(cfg.CSVPath)
	if err != nil {
		return res, fmt.Errorf("csv: %w", err)
	}
	res.Malformed = len(malformed)
	for _, s := range malformed {
		log.Printf("[WARN] update-event: skip line %d: %s", s.Line, s.Reason)
	}

	var items []Item
	for _, t := range txns {
		if st.Has(t.MessageID) {
			continue
		}
		items = append(items, Item{
			ID:       t.MessageID,
			Date:     t.TxnDate,
			Type:     t.Type,
			Amount:   t.Amount,
			Merchant: t.Merchant,
			Info:     t.Info,
			Subject:  t.Subject,
			Category: t.Category,
		})
		if cfg.Limit > 0 && len(items) >= cfg.Limit {
			break
		}
	}
	res.Total = len(items)

	if len(items) == 0 {
		log.Printf("[INFO] update-event: no unassigned rows in %s — nothing to do", cfg.CSVPath)
		return res, nil
	}
	log.Printf("[INFO] update-event: %d unassigned row(s) via %s (%d known event(s))",
		len(items), matcher.Name(), len(reg.Events))

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = len(items)
	}

	for start := 0; start < len(items); start += batchSize {
		end := start + batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := items[start:end]

		results, err := matcher.Match(ctx, eventRefs(reg), batch)
		if err != nil {
			return res, fmt.Errorf("update-event batch %d-%d: %w", start, end, err)
		}
		byID := make(map[string]MatchResult, len(results))
		for _, r := range results {
			byID[r.ID] = r
		}

		var unmatched []pending
		for _, item := range batch {
			r, ok := byID[item.ID]
			if !ok {
				log.Printf("  [WARN] update-event: no assignment returned for %s — will retry next run", item.ID)
				continue
			}

			if r.EventID != "" {
				if _, known := reg.Find(r.EventID); known && r.Confidence >= threshold {
					if cfg.DryRun {
						fmt.Printf("  %s -> event %q (existing, confidence %.2f)\n", item.ID, r.EventID, r.Confidence)
					} else {
						st.Mark(item.ID, r.EventID, r.Confidence)
						reg.Touch(r.EventID, 1)
					}
					res.Assigned++
					continue
				}
				// Unknown id or below threshold: fall through, treated the same
				// as an explicit non-match (ADR 0011 decision 7).
			}

			if strings.TrimSpace(r.NewEventName) != "" {
				unmatched = append(unmatched, pending{item: item, res: r})
				continue
			}

			// Model deliberately found no event for this transaction.
			if cfg.DryRun {
				fmt.Printf("  %s -> no event\n", item.ID)
			} else {
				st.Mark(item.ID, "", r.Confidence)
			}
			res.NoEvent++
		}

		// Group proposed-new-event rows by normalised name so several
		// transactions proposing the "same" new event become ONE registry
		// entry, not one per row (ADR 0011 decision 6).
		groups := make(map[string][]pending)
		var order []string
		for _, p := range unmatched {
			key := strings.ToLower(strings.TrimSpace(p.res.NewEventName))
			if _, seen := groups[key]; !seen {
				order = append(order, key)
			}
			groups[key] = append(groups[key], p)
		}
		for _, key := range order {
			group := groups[key]
			first := group[0].res
			if cfg.DryRun {
				fmt.Printf("  NEW EVENT %q (%d txn): %v\n", first.NewEventName, len(group), idsOf(group))
				res.NewEvents++
				res.Assigned += len(group)
				continue
			}
			eventID := reg.CreateEvent(first.NewEventName, first.NewEventDescription, mergeKeywords(group), len(group))
			for _, p := range group {
				st.Mark(p.item.ID, eventID, 1.0)
			}
			res.NewEvents++
			res.Assigned += len(group)
		}

		log.Printf("  [INFO] update-event: processed batch %d-%d of %d", start, end, len(items))
	}

	if cfg.DryRun {
		log.Printf("[INFO] update-event: dry-run — %d assigned (%d new event(s)), %d no-event — nothing written",
			res.Assigned, res.NewEvents, res.NoEvent)
		return res, nil
	}

	if err := reg.Save(); err != nil {
		return res, fmt.Errorf("saving registry: %w", err)
	}
	if err := st.Save(); err != nil {
		return res, fmt.Errorf("saving state: %w", err)
	}
	log.Printf("[INFO] update-event: %d assigned (%d new event(s) created), %d left without an event",
		res.Assigned, res.NewEvents, res.NoEvent)
	return res, nil
}

func eventRefs(reg *Registry) []EventRef {
	refs := make([]EventRef, len(reg.Events))
	for i, e := range reg.Events {
		refs[i] = EventRef{ID: e.ID, Name: e.Name, Description: e.Description, Keywords: e.Keywords}
	}
	return refs
}

func idsOf(group []pending) []string {
	ids := make([]string, len(group))
	for i, p := range group {
		ids[i] = p.item.ID
	}
	return ids
}

// mergeKeywords de-duplicates keywords proposed across every row in a group.
func mergeKeywords(group []pending) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range group {
		for _, k := range p.res.NewEventKeywords {
			k = strings.TrimSpace(k)
			if k == "" || seen[strings.ToLower(k)] {
				continue
			}
			seen[strings.ToLower(k)] = true
			out = append(out, k)
		}
	}
	return out
}
