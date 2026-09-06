package uiserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// write drops a file into dir, creating parents, and fails the test on error.
func write(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

const recZomato = `{"id":"r1","accountId":"a1","accountName":"Canara CC","note":"Zomato | gm:1","counterParty":"Zomato","amount":{"value":-868.76,"currencyCode":"INR"},"recordDate":"2026-09-05T12:00:00.000Z","category":{"id":"c1","name":"Food & Drinks","group":{"id":"food_and_drinks","name":"Food & Drinks"},"color":"#ff3d00"},"recordState":"cleared","recordType":"expense","labels":[{"id":"l1","name":"Dinner","color":"#ff6f00"}],"createdAt":"2026-09-05T07:39:56.622Z","updatedAt":"2026-09-05T10:35:09.974Z","source":"rest"}`

const recSalary = `{"id":"r2","accountId":"a1","accountName":"Canara CC","counterParty":"Employer","amount":{"value":50000,"currencyCode":"INR"},"recordDate":"2026-09-01T12:00:00.000Z","recordState":"cleared","recordType":"income","createdAt":"2026-09-01T09:00:00.000Z","updatedAt":"2026-09-01T09:00:00.000Z","source":"rest"}`

func TestLoadRecordsFlattensJSONL(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "records-today.jsonl", recZomato+"\n"+recSalary+"\n")

	set, err := LoadRecords(dir)
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
	if len(set.Records) != 2 {
		t.Fatalf("got %d records, want 2", len(set.Records))
	}
	if len(set.Issues) != 0 {
		t.Fatalf("unexpected issues: %+v", set.Issues)
	}

	// Newest record date first.
	if set.Records[0].ID != "r1" {
		t.Errorf("first record is %q, want r1 (newer recordDate sorts first)", set.Records[0].ID)
	}

	r := set.Records[0]
	if r.Amount != -868.76 || r.Currency != "INR" {
		t.Errorf("amount = %v %s, want -868.76 INR", r.Amount, r.Currency)
	}
	if r.Day != "2026-09-05" {
		t.Errorf("day = %q, want 2026-09-05", r.Day)
	}
	if r.CategoryName != "Food & Drinks" || r.CategoryGroup != "Food & Drinks" || r.CategoryColor != "#ff3d00" {
		t.Errorf("category flattened wrong: %+v", r)
	}
	if len(r.Labels) != 1 || r.Labels[0].Name != "Dinner" {
		t.Errorf("labels = %+v, want one Dinner label", r.Labels)
	}
	if r.SourceFile != "records-today.jsonl" {
		t.Errorf("sourceFile = %q, want records-today.jsonl", r.SourceFile)
	}
	if len(r.Raw) == 0 {
		t.Error("raw payload not retained for the detail view")
	}
	if set.Files[0].Records != 2 {
		t.Errorf("file record count = %d, want 2", set.Files[0].Records)
	}
}

func TestLoadRecordsReadsJSONArrayAndNestedDir(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "records/records-2026-09.json", "[\n"+recZomato+",\n"+recSalary+"\n]\n")

	set, err := LoadRecords(dir)
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
	if len(set.Records) != 2 {
		t.Fatalf("got %d records, want 2", len(set.Records))
	}
	if got := set.Records[0].SourceFile; got != filepath.Join("records", "records-2026-09.json") {
		t.Errorf("sourceFile = %q, want the nested path", got)
	}
}

func TestLoadRecordsPrefersNewestUpdatedAtForSameID(t *testing.T) {
	dir := t.TempDir()
	stale := `{"id":"r1","counterParty":"Old name","amount":{"value":-10,"currencyCode":"INR"},"recordDate":"2026-09-05T12:00:00.000Z","updatedAt":"2026-09-05T08:00:00.000Z"}`
	fresh := `{"id":"r1","counterParty":"New name","amount":{"value":-10,"currencyCode":"INR"},"recordDate":"2026-09-05T12:00:00.000Z","updatedAt":"2026-09-06T08:00:00.000Z"}`
	// "records-a" sorts before "records-b", so the fresher copy arrives second.
	write(t, dir, "records-a.jsonl", stale+"\n")
	write(t, dir, "records-b.jsonl", fresh+"\n")

	set, err := LoadRecords(dir)
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
	if len(set.Records) != 1 {
		t.Fatalf("got %d records, want 1 after de-duplication by id", len(set.Records))
	}
	if set.Records[0].CounterParty != "New name" {
		t.Errorf("kept %q, want the copy with the newer updatedAt", set.Records[0].CounterParty)
	}
}

func TestLoadRecordsSkipsDedupOutputAndReportsBadLines(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "records-today.jsonl", recZomato+"\nnot json at all\n")
	// Matches records*.json but is dedup output, not records.
	write(t, dir, "records-today.jsonl-dedup-results.json", `{"decisions":[]}`)

	set, err := LoadRecords(dir)
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
	if len(set.Records) != 1 {
		t.Fatalf("got %d records, want 1", len(set.Records))
	}
	if len(set.Files) != 1 {
		t.Fatalf("read %d files, want only the records file: %+v", len(set.Files), set.Files)
	}
	if len(set.Issues) != 1 || set.Issues[0].Line != 2 {
		t.Fatalf("issues = %+v, want one issue on line 2", set.Issues)
	}
}

func TestLoadRecordsOnEmptyDirectory(t *testing.T) {
	set, err := LoadRecords(t.TempDir())
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
	if len(set.Records) != 0 || len(set.Files) != 0 {
		t.Errorf("want an empty set, got %+v", set)
	}
	// The JSON contract is "empty list", never null, so the UI can iterate it.
	body, _ := json.Marshal(set)
	if string(body) == "" || !json.Valid(body) {
		t.Fatalf("set does not marshal: %s", body)
	}
	var decoded struct {
		Records []RecordView `json:"records"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil || decoded.Records == nil {
		t.Errorf("records must marshal as [] not null: %s", body)
	}
}

func TestLoadDuplicatesJoinsDecisionsOntoGroups(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "dedup-scan-2026-09-05.json", `{
      "duplicateGroupsFound": 2,
      "groups": [
        {"duplicateKey":"k1","matchType":"exact","confidence":1,
         "records":[{"id":"r1","isOriginal":true,"counterParty":"Zomato","amount":-868.76,"category":"Food & Drinks"},
                    {"id":"r2","counterParty":"Zomato","amount":-868.76,"category":"Food & Drinks"}]},
        {"duplicateKey":"k2","matchType":"fuzzy","confidence":0.8,
         "records":[{"id":"r3","isOriginal":true,"counterParty":"Blinkit","amount":-607}]}
      ]}`)
	write(t, dir, "dedup-decisions-2026-09-05.json",
		`{"_type":"dedup-decisions-header","totalGroups":1}`+"\n"+
			`{"duplicateKey":"k1","action":"keep_first_delete_rest","keepRecordIds":["r1"],"deleteRecordIds":["r2"],"reason":"keep oldest"}`+"\n")

	set, err := LoadDuplicates(dir)
	if err != nil {
		t.Fatalf("LoadDuplicates: %v", err)
	}
	if len(set.Groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(set.Groups))
	}
	if set.Groups[0].Decision == nil {
		t.Fatal("group k1 should carry its decision")
	}
	d := set.Groups[0].Decision
	if d.Action != "keep_first_delete_rest" || len(d.DeleteRecordIDs) != 1 || d.DeleteRecordIDs[0] != "r2" {
		t.Errorf("decision joined wrong: %+v", d)
	}
	if d.SourceFile != "dedup-decisions-2026-09-05.json" {
		t.Errorf("decision sourceFile = %q", d.SourceFile)
	}
	if set.Groups[1].Decision != nil {
		t.Error("group k2 has no decision and must stay undecided")
	}
	if set.Groups[0].ScannedAt == "" {
		t.Error("scannedAt should fall back to the file's mtime")
	}
}

func TestLoadDuplicatesReadsResultsWrapperShape(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "dedup-scan-x.json", `{"groups":[{"duplicateKey":"k1","records":[{"id":"r1"}]}]}`)
	write(t, dir, "records.jsonl-dedup-results.json",
		`{"decisions":[{"duplicateKey":"k1","action":"keep_first_delete_rest","keepRecordIds":["r1"],"deleteRecordIds":[]}]}`)

	set, err := LoadDuplicates(dir)
	if err != nil {
		t.Fatalf("LoadDuplicates: %v", err)
	}
	if set.Groups[0].Decision == nil {
		t.Fatal("the {\"decisions\":[...]} shape must be read too")
	}
}

func TestLoadSyncStateMissingFileIsNotAnError(t *testing.T) {
	set, err := LoadSyncState(t.TempDir())
	if err != nil {
		t.Fatalf("LoadSyncState: %v", err)
	}
	if !set.Missing {
		t.Error("want Missing=true for a pack that has never run")
	}
}

func TestLoadSyncStatePassesStateThrough(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "last-sync.json", `{"last_run_status":"success","last_run_summary":{"created":3}}`)

	set, err := LoadSyncState(dir)
	if err != nil {
		t.Fatalf("LoadSyncState: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(set.State, &got); err != nil {
		t.Fatalf("state is not valid JSON: %v", err)
	}
	if got["last_run_status"] != "success" {
		t.Errorf("state = %+v", got)
	}
	if set.Modified == "" {
		t.Error("want the file's mtime")
	}
}

func TestLoadLogsClassifiesFailures(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "logs/sync-1.log", "start\nall good\ndone\n")
	write(t, dir, "logs/sync-2.log", "start\nTraceback (most recent call last):\n  boom\nsubprocess.CalledProcessError: exit 1\n")
	write(t, dir, "logs/notes.txt", "ignored")

	logs, err := LoadLogs(dir, 0)
	if err != nil {
		t.Fatalf("LoadLogs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("got %d logs, want 2 (.txt must be ignored)", len(logs))
	}

	byName := map[string]LogEntry{}
	for _, l := range logs {
		byName[l.Name] = l
	}
	if byName["sync-1.log"].Status != "ok" {
		t.Errorf("sync-1 status = %q, want ok", byName["sync-1.log"].Status)
	}
	if byName["sync-2.log"].Status != "error" {
		t.Errorf("sync-2 status = %q, want error", byName["sync-2.log"].Status)
	}
	if byName["sync-1.log"].Headline != "done" {
		t.Errorf("ok headline = %q, want the last line", byName["sync-1.log"].Headline)
	}
	if byName["sync-2.log"].Headline == "" {
		t.Error("a failed run should surface its error line")
	}
}

func TestLoadLogsMissingDirIsEmpty(t *testing.T) {
	logs, err := LoadLogs(t.TempDir(), 0)
	if err != nil {
		t.Fatalf("LoadLogs: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("want no logs, got %d", len(logs))
	}
}

func TestReadLogTailsAndRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "logs/sync.log", "one\ntwo\nthree\nfour\n")
	write(t, dir, "secret.json", `{"token":"nope"}`)

	body, err := ReadLog(dir, "sync.log", 2)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if body != "three\nfour" {
		t.Errorf("tail = %q, want the last two lines", body)
	}

	for _, bad := range []string{"../secret.json", "../../etc/passwd", "sub/dir.log", "secret.json", ".hidden.log"} {
		if _, err := ReadLog(dir, bad, 10); err == nil {
			t.Errorf("ReadLog(%q) succeeded; it must be rejected", bad)
		}
	}
}

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

func testServer(t *testing.T, dir string) http.Handler {
	t.Helper()
	srv, err := New(Options{
		DataDir: dir,
		Static:  fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<h1>ui</h1>")}},
		Addr:    "127.0.0.1:0",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv.Handler()
}

func TestNewRejectsMissingDataDir(t *testing.T) {
	_, err := New(Options{DataDir: filepath.Join(t.TempDir(), "nope"), Static: fstest.MapFS{}})
	if err == nil {
		t.Fatal("want an error for a data directory that does not exist")
	}
}

func TestServerServesRecordsAndStatic(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "records-today.jsonl", recZomato+"\n")
	h := testServer(t, dir)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/records", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store — the body is personal finance data", got)
	}
	var set RecordSet
	if err := json.Unmarshal(rec.Body.Bytes(), &set); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(set.Records) != 1 || set.Records[0].CounterParty != "Zomato" {
		t.Errorf("records = %+v", set.Records)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "<h1>ui</h1>" {
		t.Errorf("static index: status %d body %q", rec.Code, rec.Body.String())
	}
}

func TestServerIsReadOnly(t *testing.T) {
	h := testServer(t, t.TempDir())
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/api/records", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /api/records = %d, want 405", method, rec.Code)
		}
	}
}

func TestServerLogTailRejectsEscape(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "last-sync.json", `{"secret":true}`)
	h := testServer(t, dir)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/logs/..%2Flast-sync.json", nil))
	if rec.Code == http.StatusOK {
		t.Errorf("traversal returned 200 with body %s", rec.Body.String())
	}
}
