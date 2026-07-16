package event

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Item is one transaction handed to the Matcher. ID is the CSV MessageID.
type Item struct {
	ID       string `json:"id"`
	Date     string `json:"date"`
	Type     string `json:"type"`
	Amount   string `json:"amount"`
	Merchant string `json:"merchant"`
	Info     string `json:"info"`
	Subject  string `json:"subject"`
	Category string `json:"category"`
}

// EventRef is the compact view of a known event sent to the model — enough
// context to recognise a repeat, without re-sending every past transaction.
type EventRef struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
}

// MatchResult is the raw (unvalidated) response for one transaction id.
// EventID is set (and NewEventName empty) when the model matched a known
// event; otherwise EventID is empty and NewEvent* describe a proposed event.
type MatchResult struct {
	ID                  string   `json:"id"`
	EventID             string   `json:"event_id"`
	Confidence          float64  `json:"confidence"`
	NewEventName        string   `json:"new_event_name"`
	NewEventDescription string   `json:"new_event_description"`
	NewEventKeywords    []string `json:"new_event_keywords"`
}

// Matcher is the Strategy interface for AI-backed event matching (mirrors the
// gmail pack's categorize.Assigner). It takes the current registry plus a
// batch of transactions and returns one MatchResult per input id.
type Matcher interface {
	Name() string
	Match(ctx context.Context, events []EventRef, batch []Item) ([]MatchResult, error)
}

// systemPrompt steers the model toward strict, schema-constrained JSON.
const systemPrompt = `You are an expense-event clustering assistant for personal bank transactions.
An "event" is an ad-hoc real-world occasion that groups several transactions together in time and story (e.g. a trip, a festival, a house move) — NOT a spending category.
For each transaction, either match it to one of the existing events (by id) with a confidence score, or propose a new event.
If several transactions in this batch belong to the same new event, you MUST reuse the exact same new_event_name for all of them, so they can be grouped into a single event.
Respond ONLY with valid JSON — no prose, no markdown fences.`

// buildPrompt renders the known events plus the batch of transactions and the
// required response shape. eventsJSON/itemsJSON are the marshalled slices.
func buildPrompt(eventsJSON, itemsJSON string) string {
	return fmt.Sprintf(`=== Known events (id, name, description, keywords) ===
%s

=== Transactions to assign (unassigned so far) ===
%s

=== Instructions ===
For every transaction object above, decide ONE of:
  a) It clearly belongs to one of the known events above — set "event_id" to that
     event's exact id and "confidence" to your certainty (0.0-1.0).
  b) It does not clearly match any known event — leave "event_id" as "" and instead
     propose a new event via "new_event_name" (short, human, title case),
     "new_event_description" (one sentence), and "new_event_keywords" (0-5 short
     strings). Reuse the SAME new_event_name across transactions in this batch that
     belong to the same new event.
  c) It doesn't belong to any meaningful event at all (e.g. an ordinary routine
     purchase) — leave "event_id" as "" and also leave new_event_name as "" so no
     event is created for it.
Match on merchant, info, subject, amount, category and date proximity.
Every input id must appear exactly once in the output.

Respond ONLY with a JSON object of this shape — no prose, no markdown fences:
{
  "assignments": [
    {"id": "<transaction id>", "event_id": "<existing event id or \"\">", "confidence": 0.0,
     "new_event_name": "<or \"\">", "new_event_description": "<or \"\">", "new_event_keywords": []}
  ]
}`, eventsJSON, itemsJSON)
}

// parseMatchResults extracts the assignments array from a model response,
// tolerating markdown fences and either a bare array or the documented
// {"assignments":[...]} object.
func parseMatchResults(text string) ([]MatchResult, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var wrapper struct {
		Assignments []MatchResult `json:"assignments"`
	}
	if err := json.Unmarshal([]byte(text), &wrapper); err == nil && wrapper.Assignments != nil {
		return wrapper.Assignments, nil
	}

	var arr []MatchResult
	if err := json.Unmarshal([]byte(text), &arr); err == nil {
		return arr, nil
	}

	return nil, fmt.Errorf("parsing update-event JSON: unrecognised shape\nraw: %s", text)
}
