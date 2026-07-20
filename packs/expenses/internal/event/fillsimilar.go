package event

import (
	"context"
	"fmt"
	"log"

	"github.com/sumitasok/sa.automation.expenses/internal/csvtxn"
)

// FillSimilarConfig parameterises a fill-similar run.
type FillSimilarConfig struct {
	CSVPath      string  // path to transactions.csv (read-only)
	RegistryPath string  // path to the event registry
	StatePath    string  // path to the assignment ledger
	Provider     string  // "" or "deepseek"
	Model        string  // override model (else env/default)
	Threshold    float64 // confidence cutoff for accepting a match
	BatchSize    int     // transactions per API call (<=0 = one single call)
	DryRun       bool    // print assignments without writing state

	// Matcher optionally injects the classifier (used in tests).
	Matcher Matcher
}

// FillSimilarResult summarises a fill-similar run.
type FillSimilarResult struct {
	Total       int // unassigned rows considered
	Assigned    int // rows matched to an event via similarity
	NoMatch     int // rows the model left without a match
	Malformed   int // rows that failed CSV normalisation
	EventsToFill int // number of events with manual assignments
}

// FillSimilar uses AI matching to find unassigned transactions similar to
// transactions manually assigned to known events. It sends each event's
// assigned transactions as context to the model and asks it to find other
// similar transactions in the unassigned batch.
func FillSimilar(ctx context.Context, cfg FillSimilarConfig) (FillSimilarResult, error) {
	var res FillSimilarResult

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
		log.Printf("[WARN] fill-similar: skip line %d: %s", s.Line, s.Reason)
	}

	// Build a map of transactions by MessageID for quick lookup
	txnByID := make(map[string]csvtxn.Txn)
	for _, t := range txns {
		txnByID[t.MessageID] = t
	}

	// Find unassigned transactions
	var unassigned []Item
	for _, t := range txns {
		if !st.Has(t.MessageID) {
			unassigned = append(unassigned, Item{
				ID:       t.MessageID,
				Date:     t.TxnDate,
				Type:     t.Type,
				Amount:   t.Amount,
				Merchant: t.Merchant,
				Info:     t.Info,
				Subject:  t.Subject,
				Category: t.Category,
			})
		}
	}
	res.Total = len(unassigned)

	if len(unassigned) == 0 {
		log.Printf("[INFO] fill-similar: no unassigned rows in %s — nothing to do", cfg.CSVPath)
		return res, nil
	}

	// Group assigned transactions by event to use as examples
	type eventContext struct {
		Event        Event
		Examples     []Item // up to 3 examples of transactions in this event
	}
	eventContexts := make(map[string]eventContext)

	for msgID, assignment := range st.Assigned {
		if eventID := assignment.EventID; eventID != "" {
			if event, found := reg.Find(eventID); found {
				ctx := eventContexts[eventID]
				ctx.Event = event
				if len(ctx.Examples) < 3 {
					if t, ok := txnByID[msgID]; ok {
						ctx.Examples = append(ctx.Examples, Item{
							ID:       t.MessageID,
							Date:     t.TxnDate,
							Type:     t.Type,
							Amount:   t.Amount,
							Merchant: t.Merchant,
							Info:     t.Info,
							Subject:  t.Subject,
							Category: t.Category,
						})
					}
				}
				eventContexts[eventID] = ctx
			}
		}
	}

	// Filter to only events with manual assignments (confidence == 1.0)
	var eventsToFill []eventContext
	for eventID, ctx := range eventContexts {
		// Check if this event has any manually-assigned transactions
		hasManual := false
		for _, assignment := range st.Assigned {
			if assignment.EventID == eventID && assignment.Confidence == 1.0 {
				hasManual = true
				break
			}
		}
		if hasManual && len(ctx.Examples) > 0 {
			eventsToFill = append(eventsToFill, ctx)
		}
	}
	res.EventsToFill = len(eventsToFill)

	if len(eventsToFill) == 0 {
		log.Printf("[INFO] fill-similar: no events with manual assignments — nothing to do")
		return res, nil
	}

	log.Printf("[INFO] fill-similar: %d unassigned row(s) via %s, %d event(s) to fill",
		len(unassigned), matcher.Name(), len(eventsToFill))

	// For each event with manual assignments, send its examples to the model
	// with the unassigned batch and ask which unassigned rows belong to it.
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = len(unassigned)
	}

	for _, eventCtx := range eventsToFill {
		log.Printf("  processing event %q (%s) with %d example(s)",
			eventCtx.Event.ID, eventCtx.Event.Name, len(eventCtx.Examples))

		// Build a synthetic event ref for the AI to use for matching
		eventRefForContext := []EventRef{
			{
				ID:          eventCtx.Event.ID,
				Name:        eventCtx.Event.Name,
				Description: eventCtx.Event.Description,
				Keywords:    eventCtx.Event.Keywords,
			},
		}

		// Process unassigned rows in batches
		for start := 0; start < len(unassigned); start += batchSize {
			end := start + batchSize
			if end > len(unassigned) {
				end = len(unassigned)
			}
			batch := unassigned[start:end]

			// Ask the model to match this batch against the event,
			// given the examples we've seen
			results, err := matcher.Match(ctx, eventRefForContext, append(eventCtx.Examples, batch...))
			if err != nil {
				log.Printf("    [WARN] batch %d-%d: %v", start, end, err)
				continue
			}

			// Filter results to only those from the unassigned batch (skip example ids)
			exampleIDs := make(map[string]bool)
			for _, ex := range eventCtx.Examples {
				exampleIDs[ex.ID] = true
			}

			for _, result := range results {
				if exampleIDs[result.ID] || result.ID == "" {
					continue // skip examples and empty ids
				}
				if result.EventID != eventCtx.Event.ID || result.Confidence < threshold {
					continue // doesn't match this event or below threshold
				}

				if cfg.DryRun {
					fmt.Printf("      %s -> %q (similar, confidence %.2f)\n", result.ID, eventCtx.Event.ID, result.Confidence)
				} else {
					st.Mark(result.ID, eventCtx.Event.ID, result.Confidence)
					reg.Touch(eventCtx.Event.ID, 1)
				}
				res.Assigned++
			}
		}
	}

	if cfg.DryRun {
		log.Printf("[INFO] fill-similar: dry-run — %d matched via similarity — nothing written",
			res.Assigned)
		return res, nil
	}

	if err := reg.Save(); err != nil {
		return res, fmt.Errorf("saving registry: %w", err)
	}
	if err := st.Save(); err != nil {
		return res, fmt.Errorf("saving state: %w", err)
	}

	log.Printf("[INFO] fill-similar: assigned %d row(s) via similarity matching", res.Assigned)
	return res, nil
}
