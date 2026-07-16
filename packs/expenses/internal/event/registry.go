// Package event implements the `update-event` subcommand: it reads
// transactions.csv (via csvtxn), asks an AI provider to match each
// not-yet-assigned transaction against a known event or propose a new one,
// and persists the result. See docs/adr/0011-expenses-pack.md.
//
// Two files, split by provenance (ADR 0005):
//   - Registry (config/events.json): the event catalog itself — versioned,
//     since matching needs it to be identical across machines.
//   - Ledger (state.json, see state.go): which MessageID was assigned to
//     which event — local, produced data, git-ignored.
package event

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// Event is one entry in the registry: an ad-hoc cluster of transactions that
// belong together in time/story (e.g. "Goa Trip") but not in category.
type Event struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
	TxnCount    int      `json:"txnCount"`
}

// Registry is the on-disk event catalog.
type Registry struct {
	path   string
	Events []Event `json:"events"`
	byID   map[string]int
}

// LoadRegistry reads the registry from path, returning an empty registry if
// the file is absent (first run).
func LoadRegistry(path string) (*Registry, error) {
	reg := &Registry{path: path}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			reg.reindex()
			return reg, nil
		}
		return nil, fmt.Errorf("reading registry %s: %w", path, err)
	}
	if len(raw) == 0 {
		reg.reindex()
		return reg, nil
	}
	if err := json.Unmarshal(raw, reg); err != nil {
		return nil, fmt.Errorf("parsing registry %s: %w", path, err)
	}
	reg.path = path
	reg.reindex()
	return reg, nil
}

func (r *Registry) reindex() {
	r.byID = make(map[string]int, len(r.Events))
	for i, e := range r.Events {
		r.byID[e.ID] = i
	}
}

// Find returns the event with the given id, if known.
func (r *Registry) Find(id string) (Event, bool) {
	if id == "" {
		return Event{}, false
	}
	i, ok := r.byID[id]
	if !ok {
		return Event{}, false
	}
	return r.Events[i], true
}

// Touch increments txnCount and bumps updatedAt for an existing event —
// called when a transaction is matched to it.
func (r *Registry) Touch(id string, n int) {
	i, ok := r.byID[id]
	if !ok {
		return
	}
	r.Events[i].TxnCount += n
	r.Events[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

// CreateEvent appends a brand-new event, deriving a stable id from name that
// does not collide with any existing id. Returns the assigned id.
func (r *Registry) CreateEvent(name, description string, keywords []string, txnCount int) string {
	id := uniqueSlug(r.byID, name)
	now := time.Now().UTC().Format(time.RFC3339)
	e := Event{
		ID:          id,
		Name:        name,
		Description: description,
		Keywords:    keywords,
		CreatedAt:   now,
		UpdatedAt:   now,
		TxnCount:    txnCount,
	}
	r.byID[id] = len(r.Events)
	r.Events = append(r.Events, e)
	return id
}

// Save writes the registry back to disk, pretty-printed so it stays diffable
// in git (ADR 0005 — a versioned, must-sync definitions file).
func (r *Registry) Save() error {
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugNonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "event"
	}
	return s
}

// uniqueSlug slugifies name and appends -2, -3, ... if the slug already
// exists in the registry (e.g. two differently-worded events that happen to
// slugify the same, such as "Goa Trip" and "Goa trip 2027").
func uniqueSlug(existing map[string]int, name string) string {
	base := slugify(name)
	id := base
	for n := 2; ; n++ {
		if _, taken := existing[id]; !taken {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, n)
	}
}
