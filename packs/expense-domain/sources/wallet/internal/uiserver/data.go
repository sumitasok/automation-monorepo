// Package uiserver reads the wallet pack's outputs from the external config
// data directory (~/automation-monorepo-config/data/expense-domain/wallet by
// convention) and serves them to a static browser UI as JSON.
//
// Nothing here writes: the directory is treated as read-only, per the
// "packs/ read-only, external config is canonical" split in CLAUDE.md. Records
// are decoded into the Wallet SDK's own Record type (sdk/go/wallet) so the UI
// and the API client agree on field names and shapes.
package uiserver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sdk "github.com/sumitasok/sa.automation.wallet/sdk/go/wallet"
)

// LabelView is a label flattened for the UI.
type LabelView struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Archived bool   `json:"archived"`
}

// RecordView is one wallet record flattened into the shape the browser wants:
// scalars instead of nested optionals, dates pre-split into a local day key,
// and the untouched source object alongside for the detail view.
type RecordView struct {
	ID            string      `json:"id"`
	AccountID     string      `json:"accountId"`
	AccountName   string      `json:"accountName"`
	CounterParty  string      `json:"counterParty"`
	Note          string      `json:"note"`
	Amount        float64     `json:"amount"`
	Currency      string      `json:"currency"`
	RecordDate    string      `json:"recordDate"`
	Day           string      `json:"day"`
	CategoryID    string      `json:"categoryId"`
	CategoryName  string      `json:"categoryName"`
	CategoryColor string      `json:"categoryColor"`
	CategoryGroup string      `json:"categoryGroup"`
	Labels        []LabelView `json:"labels"`
	RecordState   string      `json:"recordState"`
	RecordType    string      `json:"recordType"`
	Source        string      `json:"source"`
	IsTransfer    bool        `json:"isTransfer"`
	TransferID    string      `json:"transferId"`
	CreatedAt     string      `json:"createdAt"`
	UpdatedAt     string      `json:"updatedAt"`
	SourceFile    string      `json:"sourceFile"`

	// Raw is the object exactly as it sits on disk, for the detail drawer.
	Raw json.RawMessage `json:"raw,omitempty"`
}

// Issue is a non-fatal problem hit while reading the data directory. The UI
// shows these rather than silently presenting a short dataset as complete.
type Issue struct {
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
}

// FileInfo describes one file the loader consumed.
type FileInfo struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
	Records  int    `json:"records,omitempty"`
}

// RecordSet is everything the records endpoint returns.
type RecordSet struct {
	Records []RecordView `json:"records"`
	Files   []FileInfo   `json:"files"`
	Issues  []Issue      `json:"issues"`
}

// recordFileGlobs are the places wallet-sync and `wallet fetch` drop records,
// relative to the data directory. Whatever exists is read; nothing is required.
var recordFileGlobs = []string{
	"records*.jsonl",
	"records*.json",
	"records/*.jsonl",
	"records/*.json",
}

// notRecords are files whose names match the record globs but hold something
// else entirely (dedup output written beside the records it scanned).
var notRecords = []string{"-dedup-results.json", "-dedup-report.json"}

// LoadRecords reads every record file under dataDir, decodes it with the SDK's
// Record type and returns one entry per distinct record id — the newest
// updatedAt wins when the same record appears in more than one file.
func LoadRecords(dataDir string) (RecordSet, error) {
	out := RecordSet{Records: []RecordView{}, Files: []FileInfo{}, Issues: []Issue{}}

	paths, err := collectRecordFiles(dataDir)
	if err != nil {
		return out, err
	}

	byID := map[string]RecordView{}
	var ordered []string // ids in first-seen order, so id-less records survive

	for _, path := range paths {
		rel := relTo(dataDir, path)
		info := FileInfo{Path: path, Name: rel, Kind: "records"}
		if st, err := os.Stat(path); err == nil {
			info.Size = st.Size()
			info.Modified = st.ModTime().UTC().Format(time.RFC3339)
		}

		raws, issues := readObjects(path)
		for _, iss := range issues {
			iss.File = rel
			out.Issues = append(out.Issues, iss)
		}

		for i, raw := range raws {
			var rec sdk.Record
			if err := json.Unmarshal(raw, &rec); err != nil {
				out.Issues = append(out.Issues, Issue{File: rel, Line: i + 1, Message: err.Error()})
				continue
			}
			if rec.ID == "" && rec.CounterParty == "" && rec.Amount.Value == 0 {
				continue // header line or unrelated object
			}
			view := toView(rec, raw, rel)
			info.Records++

			key := view.ID
			if key == "" {
				key = fmt.Sprintf("%s#%d", rel, i)
			}
			prev, seen := byID[key]
			if !seen {
				ordered = append(ordered, key)
				byID[key] = view
				continue
			}
			if view.UpdatedAt >= prev.UpdatedAt {
				byID[key] = view
			}
		}
		out.Files = append(out.Files, info)
	}

	for _, id := range ordered {
		out.Records = append(out.Records, byID[id])
	}
	// Newest transaction first; ties broken by creation time so a re-synced
	// duplicate sits directly under the record it duplicates.
	sort.SliceStable(out.Records, func(i, j int) bool {
		a, b := out.Records[i], out.Records[j]
		if a.RecordDate != b.RecordDate {
			return a.RecordDate > b.RecordDate
		}
		return a.CreatedAt > b.CreatedAt
	})
	return out, nil
}

func collectRecordFiles(dataDir string) ([]string, error) {
	seen := map[string]bool{}
	var paths []string
	for _, g := range recordFileGlobs {
		matches, err := filepath.Glob(filepath.Join(dataDir, g))
		if err != nil {
			return nil, fmt.Errorf("glob %s: %w", g, err)
		}
		for _, m := range matches {
			if seen[m] || isExcluded(m) {
				continue
			}
			if st, err := os.Stat(m); err != nil || st.IsDir() {
				continue
			}
			seen[m] = true
			paths = append(paths, m)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func isExcluded(path string) bool {
	base := filepath.Base(path)
	for _, suffix := range notRecords {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

// readObjects handles both shapes the pack writes: a JSON array of records, or
// JSONL with one object per line (optionally preceded by a header line).
func readObjects(path string) ([]json.RawMessage, []Issue) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []Issue{{Message: err.Error()}}
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var arr []json.RawMessage
		if err := json.Unmarshal([]byte(trimmed), &arr); err != nil {
			return nil, []Issue{{Message: "parse JSON array: " + err.Error()}}
		}
		return arr, nil
	}
	// Otherwise JSONL. A line may itself be an object wrapping a records array
	// (what `wallet fetch` writes when it saves a whole page), which is unwrapped
	// below alongside the plain one-record-per-line case.
	var objs []json.RawMessage
	var issues []Issue
	for i, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !json.Valid([]byte(line)) {
			issues = append(issues, Issue{Line: i + 1, Message: "line is not valid JSON"})
			continue
		}
		var probe struct {
			Records []json.RawMessage `json:"records"`
			Type    string            `json:"_type"`
		}
		if err := json.Unmarshal([]byte(line), &probe); err == nil {
			if probe.Type != "" {
				continue // metadata header line
			}
			if len(probe.Records) > 0 {
				objs = append(objs, probe.Records...)
				continue
			}
		}
		objs = append(objs, json.RawMessage(line))
	}
	return objs, issues
}

func toView(r sdk.Record, raw json.RawMessage, sourceFile string) RecordView {
	v := RecordView{
		ID:           r.ID,
		AccountID:    r.AccountID,
		AccountName:  r.AccountName,
		CounterParty: r.CounterParty,
		Note:         r.Note,
		Amount:       r.Amount.Value,
		Currency:     r.Amount.CurrencyCode,
		RecordState:  string(r.RecordState),
		RecordType:   string(r.RecordType),
		Source:       string(r.Source),
		IsTransfer:   r.IsTransfer(),
		SourceFile:   sourceFile,
		Labels:       []LabelView{},
		Raw:          raw,
	}
	if r.RecordDate.Valid() {
		v.RecordDate = r.RecordDate.UTC().Format(time.RFC3339)
		v.Day = r.RecordDate.UTC().Format("2006-01-02")
	}
	if r.CreatedAt.Valid() {
		v.CreatedAt = r.CreatedAt.UTC().Format(time.RFC3339)
	}
	if r.UpdatedAt.Valid() {
		v.UpdatedAt = r.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if r.Category != nil {
		v.CategoryID = r.Category.ID
		v.CategoryName = r.Category.Name
		v.CategoryColor = r.Category.Color
		if r.Category.Group != nil {
			v.CategoryGroup = r.Category.Group.Name
		}
	}
	if r.Transfer != nil {
		v.TransferID = r.Transfer.TransferID
	}
	for _, l := range r.Labels {
		v.Labels = append(v.Labels, LabelView{ID: l.ID, Name: l.Name, Color: l.Color, Archived: l.Archived})
	}
	return v
}

// ---------------------------------------------------------------------------
// Duplicates
// ---------------------------------------------------------------------------

// DupRecord is one member of a duplicate group as the dedup scan reports it.
type DupRecord struct {
	ID           string  `json:"id"`
	CreatedAt    string  `json:"createdAt"`
	IsOriginal   bool    `json:"isOriginal"`
	CounterParty string  `json:"counterParty"`
	Amount       float64 `json:"amount"`
	Category     string  `json:"category"`
}

// Decision is the keep/delete call recorded for a duplicate group.
type Decision struct {
	DuplicateKey    string   `json:"duplicateKey"`
	Action          string   `json:"action"`
	KeepRecordIDs   []string `json:"keepRecordIds"`
	DeleteRecordIDs []string `json:"deleteRecordIds"`
	Reason          string   `json:"reason"`
	SourceFile      string   `json:"sourceFile,omitempty"`
}

// DupGroup is a scanned duplicate group with its decision attached, if one was
// ever recorded for it.
type DupGroup struct {
	DuplicateKey string      `json:"duplicateKey"`
	MatchType    string      `json:"matchType"`
	Confidence   float64     `json:"confidence"`
	Records      []DupRecord `json:"records"`
	Decision     *Decision   `json:"decision,omitempty"`
	SourceFile   string      `json:"sourceFile"`
	ScannedAt    string      `json:"scannedAt,omitempty"`
}

// DupSet is everything the duplicates endpoint returns.
type DupSet struct {
	Groups []DupGroup `json:"groups"`
	Files  []FileInfo `json:"files"`
	Issues []Issue    `json:"issues"`
}

// LoadDuplicates reads the newest dedup scan plus every decision file, and
// joins them on duplicateKey so the UI can show what was found next to what was
// decided about it.
func LoadDuplicates(dataDir string) (DupSet, error) {
	out := DupSet{Groups: []DupGroup{}, Files: []FileInfo{}, Issues: []Issue{}}

	decisions, decFiles, decIssues := loadDecisions(dataDir)
	out.Files = append(out.Files, decFiles...)
	out.Issues = append(out.Issues, decIssues...)

	scans, err := filepath.Glob(filepath.Join(dataDir, "dedup-scan-*.json"))
	if err != nil {
		return out, err
	}
	sort.Strings(scans)

	for _, path := range scans {
		rel := relTo(dataDir, path)
		info := FileInfo{Path: path, Name: rel, Kind: "dedup-scan"}
		st, err := os.Stat(path)
		if err == nil {
			info.Size = st.Size()
			info.Modified = st.ModTime().UTC().Format(time.RFC3339)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			out.Issues = append(out.Issues, Issue{File: rel, Message: err.Error()})
			continue
		}
		var scan struct {
			DuplicateGroupsFound int        `json:"duplicateGroupsFound"`
			ScannedAt            string     `json:"scannedAt"`
			Groups               []DupGroup `json:"groups"`
		}
		if err := json.Unmarshal(data, &scan); err != nil {
			out.Issues = append(out.Issues, Issue{File: rel, Message: "parse dedup scan: " + err.Error()})
			out.Files = append(out.Files, info)
			continue
		}
		for _, g := range scan.Groups {
			g.SourceFile = rel
			g.ScannedAt = scan.ScannedAt
			if g.ScannedAt == "" {
				g.ScannedAt = info.Modified
			}
			if d, ok := decisions[g.DuplicateKey]; ok {
				dd := d
				g.Decision = &dd
			}
			out.Groups = append(out.Groups, g)
		}
		info.Records = len(scan.Groups)
		out.Files = append(out.Files, info)
	}
	return out, nil
}

// loadDecisions reads both shapes the dedup review step writes: JSONL decision
// files (a header line then one decision per line) and a JSON object with a
// "decisions" array. Later files win on a repeated duplicateKey.
func loadDecisions(dataDir string) (map[string]Decision, []FileInfo, []Issue) {
	decisions := map[string]Decision{}
	var files []FileInfo
	var issues []Issue

	var paths []string
	for _, g := range []string{"dedup-decisions-*.json", "*-dedup-results.json"} {
		m, err := filepath.Glob(filepath.Join(dataDir, g))
		if err != nil {
			continue
		}
		paths = append(paths, m...)
	}
	sort.Strings(paths)

	for _, path := range paths {
		rel := relTo(dataDir, path)
		info := FileInfo{Path: path, Name: rel, Kind: "dedup-decisions"}
		if st, err := os.Stat(path); err == nil {
			info.Size = st.Size()
			info.Modified = st.ModTime().UTC().Format(time.RFC3339)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			issues = append(issues, Issue{File: rel, Message: err.Error()})
			continue
		}
		trimmed := strings.TrimSpace(string(data))
		if trimmed == "" {
			continue
		}

		var found []Decision
		if strings.HasPrefix(trimmed, "{") && strings.Contains(trimmed, "\"decisions\"") {
			var wrapper struct {
				Decisions []Decision `json:"decisions"`
			}
			if err := json.Unmarshal([]byte(trimmed), &wrapper); err != nil {
				issues = append(issues, Issue{File: rel, Message: "parse decisions: " + err.Error()})
				continue
			}
			found = wrapper.Decisions
		} else {
			for i, line := range strings.Split(trimmed, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.Contains(line, "\"_type\"") {
					continue
				}
				var d Decision
				if err := json.Unmarshal([]byte(line), &d); err != nil {
					issues = append(issues, Issue{File: rel, Line: i + 1, Message: err.Error()})
					continue
				}
				found = append(found, d)
			}
		}

		for _, d := range found {
			if d.DuplicateKey == "" {
				continue
			}
			d.SourceFile = rel
			decisions[d.DuplicateKey] = d
		}
		info.Records = len(found)
		files = append(files, info)
	}
	return decisions, files, issues
}

// ---------------------------------------------------------------------------
// Sync state and logs
// ---------------------------------------------------------------------------

// SyncState is last-sync.json passed through untouched, plus where it came from.
type SyncState struct {
	File     string          `json:"file"`
	Modified string          `json:"modified"`
	State    json.RawMessage `json:"state"`
	Missing  bool            `json:"missing"`
}

// LoadSyncState reads last-sync.json. A missing file is reported, not an error:
// a pack that has never run has nothing to show but is not broken.
func LoadSyncState(dataDir string) (SyncState, error) {
	path := filepath.Join(dataDir, "last-sync.json")
	out := SyncState{File: relTo(dataDir, path), State: json.RawMessage("null")}

	st, err := os.Stat(path)
	if os.IsNotExist(err) {
		out.Missing = true
		return out, nil
	} else if err != nil {
		return out, err
	}
	out.Modified = st.ModTime().UTC().Format(time.RFC3339)

	data, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	if !json.Valid(data) {
		return out, fmt.Errorf("last-sync.json is not valid JSON")
	}
	out.State = json.RawMessage(data)
	return out, nil
}

// LogEntry is one run log in the logs directory.
type LogEntry struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
	Status   string `json:"status"` // ok | error | empty
	Headline string `json:"headline"`
}

// errorMarkers are the strings that mean a run did not finish cleanly. The
// launchd wrapper does not set a status anywhere, so the log text is the signal.
var errorMarkers = []string{"Traceback (most recent call last)", "CalledProcessError", "ERROR", "❌", "FAILED", "Error:"}

// LoadLogs lists the run logs, newest first, classifying each by whether its
// text contains an error marker.
func LoadLogs(dataDir string, limit int) ([]LogEntry, error) {
	dir := filepath.Join(dataDir, "logs")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []LogEntry{}, nil
	} else if err != nil {
		return nil, err
	}

	logs := []LogEntry{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		st, err := e.Info()
		if err != nil {
			continue
		}
		entry := LogEntry{
			Name:     e.Name(),
			Size:     st.Size(),
			Modified: st.ModTime().UTC().Format(time.RFC3339),
			Status:   "ok",
		}
		if st.Size() == 0 {
			entry.Status = "empty"
		} else if body, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
			text := string(body)
			for _, marker := range errorMarkers {
				if strings.Contains(text, marker) {
					entry.Status = "error"
					break
				}
			}
			entry.Headline = headline(text, entry.Status == "error")
		}
		logs = append(logs, entry)
	}

	sort.Slice(logs, func(i, j int) bool { return logs[i].Modified > logs[j].Modified })
	if limit > 0 && len(logs) > limit {
		logs = logs[:limit]
	}
	return logs, nil
}

// headline picks the line worth showing in the list: the last error line for a
// failed run, otherwise the last non-empty line.
func headline(text string, failed bool) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if failed {
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}
			for _, marker := range errorMarkers {
				if strings.Contains(line, marker) {
					return truncate(line, 200)
				}
			}
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return truncate(line, 200)
		}
	}
	return ""
}

// ReadLog returns the tail of one log file. name is validated against the logs
// directory listing by the caller's path check, so no traversal is possible.
func ReadLog(dataDir, name string, tailLines int) (string, error) {
	if name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("invalid log name %q", name)
	}
	if !strings.HasSuffix(name, ".log") {
		return "", fmt.Errorf("not a log file: %q", name)
	}
	data, err := os.ReadFile(filepath.Join(dataDir, "logs", name))
	if err != nil {
		return "", err
	}
	if tailLines <= 0 {
		return string(data), nil
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
	}
	return strings.Join(lines, "\n"), nil
}

// ---------------------------------------------------------------------------

func relTo(base, path string) string {
	if rel, err := filepath.Rel(base, path); err == nil {
		return rel
	}
	return filepath.Base(path)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
