package event

import (
	"encoding/json"
	"os"
	"time"
)

// AssignmentEntry records that a transaction (by MessageID) was assigned to
// an event, with the confidence the matcher reported and when.
type AssignmentEntry struct {
	EventID    string  `json:"eventId"`
	Confidence float64 `json:"confidence"`
	AssignedAt string  `json:"assignedAt"`
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
func (s *State) Mark(messageID, eventID string, confidence float64) {
	s.Assigned[messageID] = AssignmentEntry{
		EventID:    eventID,
		Confidence: confidence,
		AssignedAt: time.Now().UTC().Format(time.RFC3339),
	}
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
