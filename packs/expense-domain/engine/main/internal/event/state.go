package event

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

// AssignmentEntry records that a transaction (by MessageID) was assigned to
// an event, with the confidence the matcher reported and when.
type AssignmentEntry struct {
	EventID    string  `json:"eventId"`
	Confidence float64 `json:"confidence"`
	AssignedAt string  `json:"assignedAt"`
	// Source records which mechanism produced this assignment: "rule:<name>",
	// "ai:<provider>", "ai:<provider>+comment" or "suggested:<original-source>"
	// (ADR 0016; spec 003). Empty on entries written before this feature
	// shipped — encoding/json zero-values the missing field on load, so old
	// state.json files remain readable with no migration step.
	Source string `json:"source,omitempty"`
	// Comment (spec 003) is the exact UserComment value considered the last
	// time this transaction was matched — the dirty-tracking snapshot that
	// lets a newly-added or edited comment re-open an already-assigned
	// transaction (FR-010/FR-011) without reprocessing an unchanged one.
	// Empty/absent on entries written before this feature, and on entries
	// whose match never considered a comment.
	Comment string `json:"comment,omitempty"`
}

// State is the on-disk assignment ledger — local, produced data (ADR 0005),
// analogous to the wallet pack's dedupe state.json. It makes `update-event`
// idempotent: a MessageID already present here is skipped on re-run.
type State struct {
	path     string
	Assigned map[string]AssignmentEntry `json:"assigned"`
}

// LoadState reads state from path, returning an empty state if absent.
func LoadState(path string) (*State, error) {
	s := &State{path: path, Assigned: map[string]AssignmentEntry{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(raw, s); err != nil {
		return nil, err
	}
	if s.Assigned == nil {
		s.Assigned = map[string]AssignmentEntry{}
	}
	s.path = path
	return s, nil
}

// Has reports whether a MessageID has already been assigned to an event.
func (s *State) Has(messageID string) bool {
	_, ok := s.Assigned[messageID]
	return ok
}

// Mark records a MessageID as assigned to eventID with the given confidence.
// source is "rule:<name>", "ai:<provider>", "ai:<provider>+comment" or
// "suggested:<original-source>" (ADR 0016; spec 003); "" is accepted for
// callers that don't track it. comment is the UserComment value considered
// for this assignment, "" when none was.
func (s *State) Mark(messageID, eventID string, confidence float64, source, comment string) {
	s.Assigned[messageID] = AssignmentEntry{
		EventID:    eventID,
		Confidence: confidence,
		AssignedAt: time.Now().UTC().Format(time.RFC3339),
		Source:     source,
		Comment:    comment,
	}
}

// needsReprocessing reports whether the transaction identified by messageID
// should be (re)sent through the matching flow (spec 003, FR-010/FR-011):
// either it has no assignment yet, or its currentComment has been added or
// edited since the last assignment that considered it. A comment that is
// unchanged, or was cleared back to empty after already being considered,
// does not trigger reprocessing.
func (s *State) needsReprocessing(messageID, currentComment string) bool {
	entry, has := s.Assigned[messageID]
	if !has {
		return true
	}
	comment := strings.TrimSpace(currentComment)
	return comment != "" && comment != strings.TrimSpace(entry.Comment)
}

// Save writes state atomically.
func (s *State) Save() error {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
