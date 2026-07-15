// Package state persists which transactions have already been pushed to Wallet,
// keyed by the CSV MessageID. This makes `sync` idempotent: re-running never
// creates duplicate records. The state file is local, git-ignored produced data
// (ADR 0005), analogous to the gmail/telegram packs' state.
package state

import (
	"encoding/json"
	"os"
	"time"
)

// Entry records one successfully-pushed transaction.
type Entry struct {
	RecordID string `json:"recordId,omitempty"`
	Date     string `json:"date"`
	PushedAt string `json:"pushedAt"`
}

// State is the on-disk dedupe ledger.
type State struct {
	path   string
	Pushed map[string]Entry `json:"pushed"`
}

// Load reads state from path, returning an empty state if the file is absent.
func Load(path string) (*State, error) {
	s := &State{path: path, Pushed: map[string]Entry{}}
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
	if s.Pushed == nil {
		s.Pushed = map[string]Entry{}
	}
	s.path = path
	return s, nil
}

// Has reports whether a MessageID has already been pushed.
func (s *State) Has(messageID string) bool {
	_, ok := s.Pushed[messageID]
	return ok
}

// Mark records a MessageID as pushed.
func (s *State) Mark(messageID, recordID, date string) {
	s.Pushed[messageID] = Entry{
		RecordID: recordID,
		Date:     date,
		PushedAt: time.Now().UTC().Format(time.RFC3339),
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
