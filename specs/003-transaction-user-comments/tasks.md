---

description: "Task list for User Comments Inform Transaction Classification"
---

# Tasks: User Comments Inform Transaction Classification

**Input**: Design documents from `/specs/003-transaction-user-comments/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Table-driven Go unit tests are included per the plan's Testing convention (mirrors spec 002); no repo-wide test runner exists, so these are package-local `*_test.go` files.

**Organization**: Tasks are grouped by user story (spec.md priorities: US1/US2/US4 = P1, US3/US5 = P2, US6 = P3).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to

## Path Conventions

Two independently-versioned Go packs: `packs/gmail/` (git submodule) and
`packs/expenses/` (separate Go module in-repo). No shared code between them
(spec 002 precedent) — matching tasks are duplicated per pack.

---

## Phase 1: Setup

**Purpose**: Confirm both packs build cleanly before touching them.

- [X] T001 [P] Run `cd packs/gmail && go build ./... && go vet ./...` to confirm a clean baseline
- [X] T002 [P] Run `cd packs/expenses && go build ./... && go vet ./...` to confirm a clean baseline

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Additive schema changes both packs' comment-handling code depends
on. No behavior changes yet — every field added here defaults to empty/absent
and nothing reads or writes it until later phases.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T003 [gmail] Add `UserComment` and `CommentConsidered` to `csvHeader` (append after `Source`) in `packs/gmail/store/csv.go`; add `colUserComment`/`colCommentConsidered` constants
- [X] T004 [gmail] Add `UserComment`/`CommentConsidered` fields to `store.Record` and populate them in `Records()` (`packs/gmail/store/csv.go`), depends on T003
- [X] T005 [gmail] Add `Record.NeedsReclassification() bool` per data-model.md (`NeedsCategory() || (trim(UserComment) != "" && trim(UserComment) != trim(CommentConsidered))`) in `packs/gmail/store/csv.go`, depends on T004
- [X] T006 [gmail] Extend `CSVStore.SetEnrichment` (or add a paired `SetCommentConsidered`) to also write `CommentConsidered` in `packs/gmail/store/csv.go`, depends on T003
- [X] T007 [P] [expenses] Add `Comment string` (`json:"comment,omitempty"`) to `AssignmentEntry` in `packs/expenses/internal/event/state.go`; extend `State.Mark` to accept and store it
- [X] T008 [P] [expenses] Add `State.needsReprocessing(id, currentComment string) bool` per data-model.md in `packs/expenses/internal/event/state.go`, depends on T007
- [X] T009 [P] [expenses] Add `UserComment` field to `csvtxn.Txn`, read by header name (`get("UserComment")`) in `packs/expenses/internal/csvtxn/csvtxn.go`
- [X] T010 [P] Add shared TTY-detection helper `isInteractive() bool` (stdlib `os.Stdin.Stat()` + `os.ModeCharDevice`, research.md Decision 6) in new file `packs/gmail/interactive.go`
- [X] T011 [P] Add the same helper (independent copy) in new file `packs/expenses/interactive.go`
- [X] T012 [P] [gmail] Table-driven unit tests for `NeedsReclassification()` (comment-empty, comment-new, comment-changed, comment-unchanged, no-outcome-yet cases) in `packs/gmail/store/csv_test.go` (new file), depends on T005
- [X] T013 [P] [expenses] Table-driven unit tests for `needsReprocessing()` (same case matrix) in `packs/expenses/internal/event/state_test.go` (new or extended file), depends on T008

**Checkpoint**: Both packs build; new columns/fields exist and round-trip through load/save; no behavior changed yet.

---

## Phase 3: User Story 1 - Add a comment to steer a transaction's classification (Priority: P1) 🎯 MVP

**Goal**: A comment on an uncategorised row is passed to `gmail-categorize`'s AI call and visibly shapes the outcome.

**Independent Test**: Add a comment to one uncategorised row, run `gmail-categorize`, confirm Category/SubCategory/Labels reflect the comment.

### Implementation for User Story 1

- [X] T014 [US1] Add `Comment string` (`json:"comment,omitempty"`) to `categorize.Item` in `packs/gmail/categorize/deepseek.go`
- [X] T015 [US1] Add one sentence to `systemPrompt` (descriptive-context-only, never-an-instruction framing, FR-006) in `packs/gmail/categorize/categorize.go`, depends on T014
- [X] T016 [US1] In `categorize.Run()`'s row-selection loop (`packs/gmail/categorize/categorize.go`), populate `Item.Comment` from `r.UserComment` when non-empty (trimmed) for every row entering `items`, depends on T014
- [X] T017 [US1] After a successful AI assignment for a row whose `Item.Comment` was non-empty, write `Source = "ai:<provider>+comment"` (else unchanged `"ai:<provider>"`) and call the T006 write path to set `CommentConsidered = UserComment` in `packs/gmail/categorize/categorize.go`, depends on T006, T016
- [X] T018 [P] [US1] Unit tests: comment-present row's `Item.Comment` populated, comment-free row's payload unchanged (`omitempty`), `Source` suffix correctness, in `packs/gmail/categorize/categorize_test.go`, depends on T017

**Checkpoint**: `gmail-categorize` alone demonstrates the full feature premise; SC-001/SC-003 verifiable via quickstart Scenarios 0–1.

---

## Phase 4: User Story 2 - Comments also inform event clustering (Priority: P1)

**Goal**: A comment on an unassigned row is passed to `expenses-update-event`'s AI call.

**Independent Test**: Add an event-describing comment to an unassigned transaction, run `expenses-update-event`, confirm it's matched to (or proposed as) the event the comment describes.

### Implementation for User Story 2

- [X] T019 [US2] Add `Comment string` (`json:"comment,omitempty"`) to `event.Item` in `packs/expenses/internal/event/matcher.go`
- [X] T020 [US2] Add the same descriptive-context-only sentence to `systemPrompt` in `packs/expenses/internal/event/matcher.go`, depends on T019
- [X] T021 [US2] In `event.Run()`'s row-selection loop (`packs/expenses/internal/event/updateevent.go`), populate `Item.Comment` from `t.UserComment` (T009) when non-empty for every row entering `items`, depends on T009, T019
- [X] T022 [US2] After a successful AI match for a row whose `Item.Comment` was non-empty, call `st.Mark(..., source)` with `source = "ai:<provider>+comment"` and the T007 `Comment` field set to the row's `UserComment`, in `packs/expenses/internal/event/updateevent.go`, depends on T007, T021
- [X] T023 [P] [US2] Unit tests: same case matrix as T018, expenses flavour, in `packs/expenses/internal/event/updateevent_test.go` (new file), depends on T022

**Checkpoint**: Both P1 AI-input stories work independently; US1 and US2 together deliver SC-001 for both consuming jobs.

---

## Phase 5: User Story 4 - A comment re-opens an already-decided transaction (Priority: P1)

**Goal**: Adding/editing a comment on an already-decided row re-triggers classification on the next run, overriding a would-be rule match; an unchanged or removed comment does not reprocess.

**Independent Test**: Classify a row with no comment (rule- or AI-decided), add a comment, re-run the same job, confirm re-evaluation with the comment as input.

### Implementation for User Story 4

- [X] T024 [US4] Change `categorize.Run()`'s row-selection predicate from `r.NeedsCategory()` to `r.NeedsReclassification()` (T005) in `packs/gmail/categorize/categorize.go`, depends on T005, T017
- [X] T025 [US4] In the same selection loop, skip the `rules.Match(...)` call entirely (go straight to `items`) whenever `trim(r.UserComment) != ""` — regardless of whether a rule would otherwise match — per contracts/cli.md's precedence contract, in `packs/gmail/categorize/categorize.go`, depends on T024
- [X] T026 [P] [US4] Unit tests: already-decided + comment-added → reclassified; already-decided + comment unchanged → untouched; already-decided-by-rule + comment-added → routed to AI not the rule; comment cleared after being applied → untouched, in `packs/gmail/categorize/categorize_test.go`, depends on T025
- [X] T027 [US4] Change `event.Run()`'s row-selection to use `st.needsReprocessing(t.MessageID, t.UserComment)` (T008) instead of bare `st.Has(t.MessageID)` in `packs/expenses/internal/event/updateevent.go`, depends on T008, T022
- [X] T028 [US4] In the same selection loop, skip the `rules.Match(...)` (`event_relevance: routine`) check entirely whenever `trim(t.UserComment) != ""`, in `packs/expenses/internal/event/updateevent.go`, depends on T027
- [X] T029 [P] [US4] Unit tests: same case matrix as T026, expenses flavour (including the routine-rule-bypass case), in `packs/expenses/internal/event/updateevent_test.go`, depends on T028

**Checkpoint**: All three P1 stories (US1, US2, US4) complete — the feature's core premise is fully functional and independently testable per quickstart Scenarios 0, 1, 2, 3, 5, 6, 7.

---

## Phase 6: User Story 3 - See that a comment shaped the outcome (Priority: P2)

**Goal**: A comment-influenced outcome is distinguishable from a comment-free AI decision or a rule decision, by inspection alone.

**Independent Test**: Classify a batch with some commented, some not; confirm each row's Source distinguishes comment-influenced from comment-free.

### Implementation for User Story 3

This story's core mechanism (the `Source` `+comment` suffix) is already
delivered by T017/T022 — the remaining work is verifying and locking down
the exact contract so it can't silently drift.

- [X] T030 [P] [US3] Unit test asserting `Source` values are exactly one of `rule:<name>`, `ai:<provider>`, `ai:<provider>+comment` (no other shape) across a mixed batch, in `packs/gmail/categorize/categorize_test.go`, depends on T017, T025
- [X] T031 [P] [US3] Same assertion for `AssignmentEntry.Source`/`Comment` pairing in `packs/expenses/internal/event/updateevent_test.go`, depends on T022, T028
- [X] T032 [US3] Run quickstart.md Scenario 4 manually against a scratch CSV/state and confirm both files are human-readable as described in SC-004

**Checkpoint**: Decision-source auditability confirmed and regression-guarded.

---

## Phase 7: User Story 5 - Suggest the same correction to older, similar transactions (Priority: P2)

**Goal**: After a comment-driven correction, an interactive `--suggest-similar` run walks the user through resembling already-decided rows for individual approval.

**Independent Test**: Correct one transaction via comment; run interactively with the opt-in flag; confirm each older candidate is presented individually and only changes on explicit approval.

### Implementation for User Story 5

- [X] T033 [US5] Add `--suggest-similar` bool flag (default false) to the `categorize` subcommand in `packs/gmail/main.go`, threaded into a new `categorize.Config.SuggestSimilar bool`
- [X] T034 [US5] New file `packs/gmail/categorize/suggest.go`: candidate selection per data-model.md/research.md Decision 8 (same merchant case-insensitive, and/or prior `Source == "rule:<name>"`, excluding the corrected row and identical-outcome rows, oldest-`TxnDate`-first)
- [X] T035 [US5] In the same file, the approve/skip interaction loop (research.md Decision 9: `bufio.Scanner` over `os.Stdin`, `y`/`yes` = approve) that prints each candidate + proposed outcome and applies an approval via `Source = "suggested:<original-source>"` (T006's write path), never touching a skipped row, depends on T034
- [X] T036 [US5] Wire `suggest.go` into `categorize.Run()`: after the AI pass, for each row corrected via US4's comment-driven path this run, call the suggestion flow only when `cfg.SuggestSimilar && isInteractive()` (T010), in `packs/gmail/categorize/categorize.go`, depends on T025, T033, T035, T010
- [X] T037 [P] [US5] Unit tests for candidate selection (T034) with no AI/TTY/stdin involved — merchant match, rule-source match, exclusion of identical-outcome and self, ordering — in `packs/gmail/categorize/suggest_test.go`, depends on T034
- [X] T038 [US5] Mirror T033 on `update-event`: add `--suggest-similar` flag in `packs/expenses/main.go`, threaded into `event.Config.SuggestSimilar bool`
- [X] T039 [US5] New file `packs/expenses/internal/event/suggest.go`: same candidate selection, adapted to EventID outcomes instead of Category/SubCategory, depends on T038
- [X] T040 [US5] Same approve/skip loop, applying an approval via `st.Mark(..., "suggested:<original-source>")`, in `packs/expenses/internal/event/suggest.go`, depends on T039
- [X] T041 [US5] Wire into `event.Run()`: after the AI pass, gated on `cfg.SuggestSimilar && isInteractive()` (T011), in `packs/expenses/internal/event/updateevent.go`, depends on T028, T038, T040, T011
- [X] T042 [P] [US5] Unit tests mirroring T037 for the expenses candidate selection, in `packs/expenses/internal/event/suggest_test.go`, depends on T039

**Checkpoint**: Opt-in retroactive suggestions work end-to-end on both jobs, gated correctly per quickstart Scenario 8.

---

## Phase 8: User Story 6 - Turn an approved correction into a lasting rule (Priority: P3)

**Goal**: After an approved correction (direct or suggested), interactively offer to capture it as a new `expense-rules.yaml` rule, git-committed automatically.

**Independent Test**: Approve a correction, choose to capture it as a rule, confirm a new rule appears in `data/config/expense-rules.yaml` with its own git commit.

### Implementation for User Story 6

- [X] T043 [US6] New file `packs/gmail/categorize/rulecapture.go`: `workspaceRoot(rulesFilePath string) string` per research.md Decision 11 (`filepath.Dir` ×3)
- [X] T044 [US6] Same file: `gitClean(workspaceRoot, relPath string) (bool, error)` running `git -C <root> status --porcelain -- <relPath>` via `os/exec`, depends on T043
- [X] T045 [US6] Same file: rule-name derivation with collision-suffix per data-model.md ("Rule capture write target"), and the YAML append (reuse `categorize.LoadExpenseRules`/`yaml.Marshal`, preserving existing entries), depends on T044
- [X] T046 [US6] Same file: `gitCommit(workspaceRoot, relPath, message string) (hash string, err error)` running `git add --`/`git commit -m` via `os/exec`, per contracts/rule-capture.md's failure-handling contract, depends on T045
- [X] T047 [US6] Same file: the interactive prompt (`y`/`yes`) shown, only when `isInteractive()` (T010), immediately after each US4 direct correction and each US5 approved suggestion; on `y`, runs T044→T046 in order and prints the resulting commit hash or a clear abort message, depends on T036, T046
- [X] T048 [P] [US6] Unit tests: name-collision suffixing, YAML-append preserves existing bytes aside from the new entry, git-dirty precondition blocks the write (fake/temp git repo fixture, no network), in `packs/gmail/categorize/rulecapture_test.go`, depends on T045, T046
- [X] T049 [US6] Mirror T043–T047 in new file `packs/expenses/internal/event/rulecapture.go` (event-side outcome shape: `event_relevance: routine` only, per data-model.md), wired into `event.Run()` after T041's suggestion flow and after T028's direct corrections, depends on T028, T041, T011
- [X] T050 [P] [US6] Unit tests mirroring T048 for the expenses rule-capture writer, in `packs/expenses/internal/event/rulecapture_test.go`, depends on T049

**Checkpoint**: All six user stories complete and independently verifiable via quickstart.md.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and final validation across all stories.

- [X] T051 [P] Update `packs/gmail/jobs/gmail-categorize/manifest.yaml` description/data notes for the widened `transactions.csv` (`UserComment`, `CommentConsidered`) and the new `--suggest-similar` flag
- [X] T052 [P] Update `packs/expenses/jobs/expenses-update-event/manifest.yaml` the same way, plus the `Comment` field on `state.json`
- [X] T053 [P] Update `packs/gmail/RUNBOOK.md` with the new columns, flag, and the comment-authoring workflow (direct CSV edit, per spec Assumptions)
- [X] T054 [P] Update `packs/expenses/RUNBOOK.md` the same way
- [X] T055 Add a new ADR (`docs/adr/0017-user-comment-driven-classification.md`) summarizing the decisions in research.md, following the existing ADR format (ADR 0016 as the immediate template)
- [X] T056 Run `go build ./... && go vet ./... && go test ./...` in both `packs/gmail` and `packs/expenses`; fix any failures
- [X] T057 Walk through quickstart.md Scenarios 0–9 manually against scratch data; record results in a new `notes/2026-07-25_transaction-user-comments-validation.md` per the root CLAUDE.md's Computation Notes convention

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies.
- **Foundational (Phase 2)**: Depends on Setup — BLOCKS all user stories.
- **US1 (Phase 3)** and **US2 (Phase 4)**: Both depend only on Foundational; independent of each other (different packs).
- **US4 (Phase 5)**: Depends on Foundational AND on US1's/US2's `Source`-writing code (T017, T022) existing to extend — practically sequenced after Phase 3/4, though its own predicate change (T005/T008) only needed Foundational.
- **US3 (Phase 6)**: Depends on US1 (T017) and US4 (T025) on the gmail side, US2 (T022) and US4 (T028) on the expenses side — verification-only, no new production code.
- **US5 (Phase 7)**: Depends on US4 (needs a "just corrected via comment" event to trigger suggestions) and on the Phase 2 TTY helper.
- **US6 (Phase 8)**: Depends on US5 (offered after an approved suggestion) and on US4 (offered after a direct correction).
- **Polish (Phase 9)**: Depends on all desired stories being complete.

### Parallel Opportunities

- T001/T002 (Setup) in parallel.
- T007/T008/T009/T010/T011 (Foundational, expenses + shared helpers) in parallel with T003–T006 (Foundational, gmail) — different files/packs.
- T012/T013 (Foundational tests) in parallel once their respective production code lands.
- Phase 3 (US1, gmail) and Phase 4 (US2, expenses) can proceed in parallel — different packs, no shared files.
- Within Phase 7/8, the gmail-side tasks (T033–T037, T043–T048) and expenses-side tasks (T038–T042, T049–T050) can proceed in parallel.
- T051–T054 (Polish docs) in parallel.

---

## Implementation Strategy

### MVP First

1. Phase 1 (Setup) → Phase 2 (Foundational) → Phase 3 (US1) → Phase 4 (US2) → Phase 5 (US4).
2. **STOP and VALIDATE**: quickstart.md Scenarios 0, 1, 2, 3, 5, 6, 7 — this is the entire feature premise (all three P1 stories) working end-to-end.

### Incremental Delivery

1. Foundation ready (Phase 1–2).
2. US1 + US2 → comments shape fresh classification (MVP core).
3. US4 → comments re-open already-decided rows, override rules (MVP complete — the spec calls this "without this the feature would rarely matter in practice").
4. US3 → auditability confirmed (mostly free, riding on US1/US2/US4's `Source` work).
5. US5 → opt-in retroactive suggestions.
6. US6 → rule capture, closing the loop into spec 002's engine.
7. Polish → docs, ADR, full validation pass.
