---

description: "Task list for feature implementation"
---

# Tasks: Gmail Transactions Editor UI

**Input**: Design documents from `/specs/004-transaction-editor-ui/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/transactions-api.md, quickstart.md (all present)

**Tests**: Not explicitly requested in the spec as strict TDD, but this repo's established convention (see `store/csv_test.go`, `categorize/*_test.go`) is table-driven Go tests written alongside every change — followed here as part of each relevant task rather than as separate contract-test-first tasks.

**Organization**: Tasks are grouped by user story (spec.md priorities: US1/US2 = P1, US3 = P2) to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no ordering dependency)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- All paths are relative to the repository root; the feature lives entirely inside the existing `packs/gmail` Go module (see plan.md's Structure Decision).

---

## Phase 1: Setup

**Purpose**: Scaffold the new package so later tasks have somewhere to write code.

- [X] T001 Create `packs/gmail/webui/` package with empty placeholder files: `server.go` (package `webui` declaration only), `templates/index.html`, `static/app.js`, `static/app.css`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Store-layer change and server scaffolding that every user story depends on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T002 Add `SetAnnotation(idx int, category, subCategory string, labels []string, note, userComment string) error` to `packs/gmail/store/csv.go`, per data-model.md: always writes Category/SubCategory/Labels/Note/UserComment; sets `Source = "user"` only when the incoming category/subCategory/labels differ from the row's current values; never touches `CommentConsidered`.
- [X] T003 [P] Add table-driven tests for `SetAnnotation` in `packs/gmail/store/csv_test.go`: (a) editing only Note/UserComment leaves `Source`/`CommentConsidered` untouched, (b) editing Category/SubCategory/Labels sets `Source = "user"`, (c) a `Save()`-then-reread round trip through a real temp file (`t.TempDir()`, matching this file's existing test style) produces no row corruption.
- [X] T004 In `packs/gmail/webui/server.go`, implement a `Server` type (`store *store.CSVStore`, `mu sync.Mutex`, `csvPath string`, `loadedAt time.Time`) with `New(csvPath string) (*Server, error)` (opens the `CSVStore`, records the file's mtime as `loadedAt`) and `Reload() error` (re-opens and refreshes `loadedAt`).
- [X] T005 In `packs/gmail/webui/server.go`, implement `Run(csvPath, addr string) error`: builds a `*Server` via `New`, registers an `http.ServeMux` with placeholder (not-yet-implemented) routes for `GET /`, `GET /api/transactions`, `PATCH /api/transactions/{messageId}`, and calls `http.ListenAndServe(addr, mux)`, logging the serving URL.
- [X] T006 Add a `serve` case to the subcommand dispatch in `packs/gmail/main.go` (alongside `discover`/`categorize`), parsing `--port` (default `8090`) and `--csv` (default existing `csvFile` constant) flags and calling `webui.Run`.
- [X] T007 In `packs/gmail/webui/server.go`, implement a pure mapping function from `store.Record` to the JSON API resource shape in data-model.md (including the `readOnly` field list).
- [X] T008 In `packs/gmail/webui/server.go`, implement a pure sort function: transactions ordered by `TxnDate` descending (lexicographic string sort — see research.md §6), rows with an empty/unparseable `TxnDate` sorted after every row that has one, stable among equal/empty dates.
- [X] T009 [P] Add unit tests for the T007 mapping function and T008 sort function (including a same-date-stability case and a missing-`TxnDate` case) in `packs/gmail/webui/server_test.go`.

**Checkpoint**: `go build ./...` succeeds in `packs/gmail`; the server starts and serves empty/placeholder responses. User story implementation can now begin.

---

## Phase 3: User Story 1 - Review transactions in chronological order (Priority: P1) 🎯 MVP

**Goal**: Opening the Transactions view shows every transaction, most recent `TxnDate` first.

**Independent Test**: Start the server against the existing `data/gmail/transactions.csv`; confirm `GET /api/transactions`'s first element (and the rendered page's first row) has the most recent `TxnDate`, descending from there.

- [X] T010 [US1] Implement the real `GET /api/transactions` handler in `packs/gmail/webui/server.go`: load records from the `Server`'s `CSVStore`, map via T007, sort via T008, return `{"loadedAt": ..., "transactions": [...]}` (contracts/transactions-api.md).
- [X] T011 [US1] Implement the real `GET /` handler in `packs/gmail/webui/server.go`: server-render `templates/index.html` via `html/template`, passing the initial sorted transaction list.
- [X] T012 [P] [US1] Write `packs/gmail/webui/templates/index.html`: a tab shell (single "Transactions" tab active, markup structured so a second tab can be added later per spec Assumptions) containing a table skeleton, and a `<script src="/static/app.js">` include.
- [X] T013 [P] [US1] Write `packs/gmail/webui/static/app.css`: minimal styling for the tab shell and transactions table.
- [X] T014 [US1] Write `packs/gmail/webui/static/app.js`: on load, `fetch('/api/transactions')` and render rows into the table, newest-first (mirrors server ordering defensively).
- [X] T015 [P] [US1] Add `httptest`-based coverage in `packs/gmail/webui/server_test.go` for `GET /api/transactions` (ordering, same-date stability, missing-`TxnDate` placement) and `GET /` (200, contains the tab shell markup).

**Checkpoint**: User Story 1 is fully functional and independently testable (`quickstart.md` scenario 1).

---

## Phase 4: User Story 2 - Edit a transaction's details (Priority: P1)

**Goal**: A user can edit a transaction's annotation fields (Category, SubCategory, Labels, Note, UserComment) in the UI and have the change persist to `data/gmail/transactions.csv`.

**Independent Test**: Edit one row's Category via the UI (or a raw `PATCH`), confirm the new value survives a page reload and appears in the CSV file; attempt an invalid edit and confirm it's rejected with the stored value unchanged; cancel an edit and confirm nothing is written.

- [X] T016 [US2] In `packs/gmail/webui/server.go`, implement request parsing and validation for a `PATCH` body per data-model.md's validation rules (Category/SubCategory non-blank-when-provided; Labels split on the existing `labelsSep`, each trimmed label non-empty; Note/UserComment unconstrained free text).
- [X] T017 [US2] Implement the real `PATCH /api/transactions/{messageId}` handler in `packs/gmail/webui/server.go`: lock the `Server`'s mutex; if the request's `loadedAt` doesn't match the in-memory value, reload from disk and return `409` (contracts/transactions-api.md) with no write; validate the body (T016), returning `422` with no write on failure; look up the row by `MessageID`, returning `404` if absent; call `store.SetAnnotation` (T002) then `Save()`; update `loadedAt`; return the updated resource with the new `loadedAt`.
- [X] T018 [P] [US2] Add `server_test.go` coverage: happy-path edit returns `200` and persists (reread the temp CSV file to confirm); validation failure returns `422` with the file unchanged; unknown `messageId` returns `404`; a stale `loadedAt` returns `409` with the file unchanged.
- [X] T019 [US2] Add inline editing to `packs/gmail/webui/static/app.js`: click-to-edit controls on the Category/SubCategory/Labels/Note/UserComment cells, with Save (sends `PATCH` with the row's last-seen `loadedAt`) and Cancel (reverts the DOM, sends no request) actions per row.
- [X] T020 [US2] In `packs/gmail/webui/static/app.js`, handle non-`200` `PATCH` responses: a `409` prompts the user to refresh the list; a `422` shows the field-specific message inline next to the offending control, per contracts/transactions-api.md.

**Checkpoint**: User Stories 1 and 2 both work independently (`quickstart.md` scenarios 2–4).

---

## Phase 5: User Story 3 - Find a specific transaction (Priority: P2)

**Goal**: A user can filter the transaction list by merchant, category, or date range without losing the newest-first ordering, and sees a clear empty state when nothing matches.

**Independent Test**: Apply a merchant filter that matches a known subset of rows — only those rows appear, still newest-first; apply a filter that matches nothing — an empty-state message appears, not an error.

- [X] T021 [US3] Extend the `GET /api/transactions` handler (T010) in `packs/gmail/webui/server.go` to read optional `merchant`, `category`, `from`, `to` query parameters and filter the record set (case-insensitive substring for merchant/category, inclusive `TxnDate` bounds for from/to) before sorting and returning.
- [X] T022 [P] [US3] Add `server_test.go` coverage for each filter parameter individually, combined filters, and a no-match case (expect a `200` with an empty `transactions` array, not an error).
- [X] T023 [US3] Add filter controls (merchant/category text inputs, from/to date inputs) to `packs/gmail/webui/templates/index.html` and wire them in `packs/gmail/webui/static/app.js` to re-query `/api/transactions` with the corresponding query parameters on change.
- [X] T024 [P] [US3] In `packs/gmail/webui/static/app.js`, render a clear empty-state message when `transactions` is empty (whether from no data or a filter with no matches), per FR-009.

**Checkpoint**: All three user stories are independently functional (`quickstart.md` scenario 5).

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T025 [P] Add a usage entry for `serve` to `packs/gmail/RUNBOOK.md` and its command reference in `packs/gmail/README.md` (mirrors the existing `discover`/`categorize` documentation).
- [X] T026 Run `go build ./... && go vet ./... && go test ./...` in `packs/gmail` and fix any failures.
- [X] T027 Execute all six `specs/004-transaction-editor-ui/quickstart.md` validation scenarios manually against `go run . serve`, including scenario 6 (staleness — trigger it by touching `transactions.csv`'s mtime or running `categorize` concurrently) and record the outcome.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies.
- **Foundational (Phase 2)**: Depends on Setup — BLOCKS all user stories (store method + server scaffolding + mapping/sort helpers that every story's handlers use).
- **User Story 1 (Phase 3)**: Depends on Foundational only. Delivers the MVP.
- **User Story 2 (Phase 4)**: Depends on Foundational; reuses US1's `templates/index.html`/`static/app.js` scaffolding but adds its own handler and UI logic — independently testable via `PATCH` even before US1's rendering exists.
- **User Story 3 (Phase 5)**: Depends on Foundational and extends US1's `GET /api/transactions` handler (T010) — implement after US1.
- **Polish (Phase 6)**: Depends on all three user stories being complete.

### Parallel Opportunities

- T003 (store tests) can run in parallel with T004–T009 (webui package) — different package entirely.
- Within Phase 2: T007/T008/T009 touch `webui/server.go`/`server_test.go` sequentially (same files), but T003 is parallel to all of them.
- Within Phase 3: T012 (`index.html`) and T013 (`app.css`) can run in parallel with each other and with T010/T011 (`server.go`) — different files, no compile dependency between markup/CSS and the Go handlers.
- Within Phase 4: T018 (tests) can be written in parallel with T019/T020 (JS) once T017 (handler) lands.
- Within Phase 5: T022 (tests) can run in parallel with T023/T024 (JS/HTML).

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 (Setup) and Phase 2 (Foundational).
2. Complete Phase 3 (User Story 1) — a read-only, newest-first Transactions view.
3. **STOP and VALIDATE**: run `quickstart.md` scenario 1 against real data.

### Incremental Delivery

1. Setup + Foundational → foundation ready.
2. User Story 1 → validate → this is already useful on its own (a correctly-ordered view, even before editing exists).
3. User Story 2 → validate → the core requested capability (editing) is now live.
4. User Story 3 → validate → search/filter layered on top.
5. Polish → docs, full test/build pass, full quickstart run.

## Notes

- No test-first/TDD gate was requested in the spec; tests are written alongside each task per this repo's established convention, not as a separate red-green-refactor phase.
- Commit after each phase (Setup, Foundational, each User Story, Polish), matching this repo's `RUNBOOK.md`-per-run convention.
- Avoid: editing `packs/gmail/store/csv.go`'s existing methods (`SetEnrichment`, `SetNote`, `Save`, etc.) — this feature only adds `SetAnnotation` (T002), consistent with every prior change to this file being additive.
