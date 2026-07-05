// Package config — state.go persists per-chat checkpoints so each run only
// summarizes messages newer than the last successful run. Same intent as the
// gmail pack's per-filter sidecar state, but Telegram has many dynamic chats,
// so all checkpoints live in one JSON map file (state.json) instead of a
// sidecar per chat.
package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// StateFile is the default checkpoint file (git-ignored, local per ADR 0005).
const StateFile = "state.json"

// State maps a chat ID to the last-seen (highest) message ID summarized for
// that chat on the previous run.
type State struct {
	// LastMessageID[chatID] = highest Telegram message ID already summarized.
	LastMessageID map[int64]int `json:"last_message_id"`
	// UpdatedAt records when the file was last written (human reference only).
	UpdatedAt time.Time `json:"updated_at"`
}

// LoadState reads path. A missing file returns an empty (first-run) state.
// A corrupt file is logged and treated as empty rather than aborting the run.
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &State{LastMessageID: map[int64]int{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading state file %q: %w", path, err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		log.Printf("[WARN] corrupt state file %q: %v — treating as first run", path, err)
		return &State{LastMessageID: map[int64]int{}}, nil
	}
	if s.LastMessageID == nil {
		s.LastMessageID = map[int64]int{}
	}
	return &s, nil
}

// Checkpoint returns the last-seen message ID for a chat (0 if never seen).
func (s *State) Checkpoint(chatID int64) int {
	if s.LastMessageID == nil {
		return 0
	}
	return s.LastMessageID[chatID]
}

// Advance records msgID as the new checkpoint for chatID if it is higher than
// the current one. Lower/equal IDs are ignored so state only moves forward.
func (s *State) Advance(chatID int64, msgID int) {
	if s.LastMessageID == nil {
		s.LastMessageID = map[int64]int{}
	}
	if msgID > s.LastMessageID[chatID] {
		s.LastMessageID[chatID] = msgID
	}
}

// Save atomically writes the state to path (write-temp-then-rename).
func (s *State) Save(path string) error {
	s.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating state dir: %w", err)
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing state temp file: %w", err)
	}
	return os.Rename(tmp, path)
}
