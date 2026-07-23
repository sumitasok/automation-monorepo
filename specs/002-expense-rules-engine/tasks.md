---

description: "Task list for the expense classification rules engine feature"
---

# Tasks: Expense Classification Rules Engine

**Input**: Design documents from `/specs/002-expense-rules-engine/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md (all present)

**Tests**: Not explicitly requested in the spec beyond the manual `quickstart.md` scenarios; this feature follows the existing repo convention (`categorize_test.go` already exists) of adding table-driven Go `*_test.go` files alongside new logic rather than a separate TDD-first test phase. Test tasks are included per user story.

**Organization**: Tasks are grouped by user story (from spec.md: US1–US4) to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1–US4)
- Exact file paths are included in every task

## Path Conventions

Two existing Go packs, extended in place, sharing one new data file — see `plan.md`'s Project Structure:

- `data/config/expense-rules.yaml` — the shared rules file (workspace root)
- `packs/gmail/...` — gmail-categorize side (separate git submodule, module `sa.automation.gmail`)
- `packs/expenses/...` — expenses-update-event side (separate Go module, in-repo)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the shared rules file and wire both jobs' manifests to declare it as a read dependency, so later phases have a real file to load against.

- [X] T001 Create `data/config/expense-rules.yaml` with the header comment block and field contract summary from `contracts/expense-rules.schema.md`, starting with an empty `rules: []` (no rules committed yet — later phases add real ones)
- [X] T002 [P] Update `packs/gmail/jobs/gmail-categorize/manifest.yaml`'s `data.reads` to add the shared rules file path (per `contracts/cli.md`'s Manifests section)
- [X] T003 [P] Update `packs/expenses/jobs/expenses-update-event/manifest.yaml`'s `data.reads` to add the shared rules file path (per `contracts/cli.md`'s Manifests section)

**Checkpoint**: The shared rules file exists (empty) and both manifests declare it; running either job today still behaves identically (nothing loads it yet).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before US3 (expenses-side rules) can be implemented — `packs/expenses` has no YAML dependency today.

**⚠️ CRITICAL**: US3 cannot begin until this phase is complete. US1, US2, and US4's gmail-side work do not depend on this phase (gmail already has `gopkg.in/yaml.v3`).

- [X] T004 Add `gopkg.in/yaml.v3` to `packs/expenses/go.mod` (and run `go mod tidy` to populate `go.sum`) — the one new dependency justified in `plan.md`'s Complexity Tracking, needed before any expenses-side rules code can parse `data/config/expense-rules.yaml`

**Checkpoint**: `packs/expenses` can now import `gopkg.in/yaml.v3`; US3 implementation can begin.

---

## Phase 3: User Story 1 - Author a merchant-based classification rule (Priority: P1) 🎯 MVP

**Goal**: A user-authored merchant rule (e.g. HungerBox → workplace food) is evaluated by `gmail-categorize` before the AI provider; a confirmed match sets the outcome directly, deterministically, with no AI call for that row.

**Independent Test**: Add one merchant-name rule to `data/config/expense-rules.yaml`, run `go run . categorize --dry-run` in `packs/gmail` against a CSV containing an uncategorised matching transaction, and confirm the row is classified per the rule, repeatably across runs (`quickstart.md` Scenario 1).

### Tests for User Story 1

- [X] T005 [P] [US1] Table-driven tests in `packs/gmail/categorize/rules_test.go` for: loading a well-formed rules file, graceful degrade on a missing file (empty rule set, no error), load-time rejection of a rule with an empty `match` object, `merchant_contains` and `keyword_contains` matching (case-insensitive substring, any-of-list), `enabled: false` rules being skipped, and `applies_to` scoping excluding a rule from the wrong decision type

### Implementation for User Story 1

- [X] T006 [P] [US1] Create `packs/gmail/categorize/rules.go`: `ExpenseRule`, `ExpenseRules`, `MatchCondition`, `Outcome` types (per `data-model.md`) and `LoadExpenseRules(path string) (ExpenseRules, error)` mirroring `discover/rules.go`'s graceful-missing-file convention; implement matching for `merchant_contains` and `keyword_contains` only in this task (day/time/amount conditions arrive in US2) — *implementation note: the full condition evaluator (including day/time/amount) was written in one pass alongside this task since it's the same function/file; T010/T011 in Phase 4 cover the corresponding test/validation additions rather than re-touching this code separately*
- [X] T007 [US1] Add `RulesFile string` to `categorize.Config` (`packs/gmail/categorize/categorize.go`) and, in `Run()`, load the rules file once, then for each uncategorised item evaluate `applies_to: categorize` rules in file order (first enabled match wins); on match, resolve `category`/`subcategory` via the existing `Taxonomy.Resolve` and `labels` via `Taxonomy.ResolveLabels` exactly as an AI assignment already is, apply the outcome, and remove that item from the batch sent to the AI assigner; on an unresolvable rule outcome, log a warning and let the item fall through to the AI batch instead (depends on T006)
- [X] T008 [US1] Add `--rules-file` flag to the `categorize` subcommand in `packs/gmail/main.go`, defaulting to `$AUTO_DATA_DIR/config/expense-rules.yaml` when `AUTO_DATA_DIR` is set, else `../../data/config/expense-rules.yaml` (per `contracts/cli.md`), wired to `categorize.Config.RulesFile` (depends on T007)
- [X] T009 [US1] Add the `hungerbox-workplace-food` rule from `contracts/expense-rules.schema.md` to `data/config/expense-rules.yaml`, and manually run `quickstart.md` Scenario 0 (no-rules regression) and Scenario 1 (merchant rule, including the repeat-run determinism check) against a scratch CSV — validated: rule-matched rows decided with no AI call, non-matching rows fell through to the AI path identically to pre-feature behavior, re-run produced byte-identical output

**Checkpoint**: User Story 1 is fully functional and independently testable — a merchant rule deterministically decides matching rows in `gmail-categorize` without an AI call.

---

## Phase 4: User Story 2 - Author a time-and-pattern based travel rule (Priority: P2)

**Goal**: A rule combining a merchant condition with time-of-day/day-of-week conditions (e.g. weekday-afternoon Uber → work travel) is evaluated the same way, proving the engine handles more than flat merchant lookups.

**Independent Test**: Add the Uber weekday-afternoon rule, run `categorize` against a mix of matching and non-matching Uber transactions, and confirm only matching ones get the rule's outcome while others fall through to AI (`quickstart.md` Scenario 2).

### Tests for User Story 2

- [X] T010 [P] [US2] Extend `packs/gmail/categorize/rules_test.go` with cases for `day_of_week`, `time_between`, and `amount_between` matching, including a transaction whose `TxnDate` has no time component (must fail closed on a `time_between` condition per `research.md` Decision 7) and a malformed/unparseable `TxnDate`

### Implementation for User Story 2

- [X] T011 [US2] Extend `MatchCondition` evaluation in `packs/gmail/categorize/rules.go` to support `day_of_week`, `time_between`, and `amount_between`, parsing `TxnDate` defensively (date-only vs. `"2006-01-02 15:04:05"`) and failing the specific condition closed — not erroring the whole rule/run — whenever required time data is absent or unparseable (depends on T006/T007)
- [X] T012 [US2] Add the `uber-weekday-afternoon-commute` rule from `contracts/expense-rules.schema.md` to `data/config/expense-rules.yaml`
- [X] T013 [US2] Manually run `quickstart.md` Scenario 2 against a scratch CSV with one weekday-afternoon Uber row and one weekend/off-hours Uber row, confirming only the former is rule-classified — validated: Monday 14:30 Uber matched the commute rule, Saturday 14:30 and Monday 09:00 Uber rows correctly fell through to the AI path

**Checkpoint**: User Stories 1 and 2 both work independently — the gmail-side rules engine now supports the full condition vocabulary from `data-model.md`.

---

## Phase 5: User Story 3 - Rules inform the expense-event matcher, not just categorisation (Priority: P2)

**Goal**: The same rules mark routine transactions (e.g. daily commute, workplace lunch) as not event-worthy, so `expenses-update-event` stops proposing spurious new events for them.

**Independent Test**: Run `expenses-update-event` over a batch containing a rule-matched transaction and confirm it's excluded from new-event proposals, while unmatched transactions still go through the existing AI event-matching path (`quickstart.md` Scenario 3).

### Tests for User Story 3

- [X] T014 [P] [US3] Table-driven tests in `packs/expenses/internal/event/rules_test.go` mirroring T005/T010's coverage (load/degrade/validate, all five condition types, `enabled`/`applies_to` scoping) for the expenses-side rule engine

### Implementation for User Story 3

- [X] T015 [P] [US3] Create `packs/expenses/internal/event/rules.go`: the same `ExpenseRule`/`ExpenseRules`/`MatchCondition`/`Outcome` shapes and `LoadExpenseRules` as gmail's `rules.go` (full condition set from the start, since the shared schema is already fully defined by Phase 4), evaluating only the `event_relevance` outcome field on this side (depends on Foundational T004)
- [X] T016 [US3] Add `RulesFile string` to `event.Config` (`packs/expenses/internal/event/updateevent.go`) and, in `Run()`, load the rules file once, then for each not-yet-assigned item evaluate `applies_to: event` rules in file order; on a match with `event_relevance: routine`, call `st.Mark(item.ID, "", 1.0, source)` directly and remove that item from the batch sent to the `Matcher` (depends on T015) — *note: `Mark` gained a `source` parameter here rather than in Phase 6, since it's the same call site (T021/T022 folded in)*
- [X] T017 [US3] Add `--rules-file` flag to the `update-event` subcommand in `packs/expenses/main.go`, same default-computation logic as T008, wired to `event.Config.RulesFile` (depends on T016)
- [X] T018 [US3] Manually run `quickstart.md` Scenario 3 against a scratch `transactions.csv`/`state.json`, confirming the HungerBox row is excluded from new-event proposals and never sent to the AI matcher — validated: rule-decided rows (HungerBox, Uber commute) marked routine with zero AI calls; unrelated row correctly fell through to the AI matcher; `state.json`'s new `source` field confirmed on write

**Checkpoint**: All three of US1/US2/US3 work independently — rules now drive both categorisation and event-relevance decisions.

---

## Phase 6: User Story 4 - Review why a transaction was classified the way it was (Priority: P3)

**Goal**: Every classified transaction records whether a rule (and which one) or the AI produced its outcome, so decisions stay auditable without reading code or logs.

**Independent Test**: Classify a batch with a mix of rule-matched and AI-matched transactions, then confirm each row's decision source is visible in `transactions.csv` / `state.json` and in `--dry-run` output (`quickstart.md` Scenario 4).

### Implementation for User Story 4

- [X] T019 [P] [US4] Add a `Source` column to `packs/gmail/store/csv.go`'s `csvHeader` (and a `colSource` constant), extending the write path so every enriched row records `rule:<rule-name>` or `ai:<provider-name>`; leave the column empty on legacy rows already enriched before this feature (additive schema change, mirrors ADR 0010's original column addition) — *done in Phase 3 alongside T007, same edited function*
- [X] T020 [US4] Wire source-tagging into `categorize.Run()` (`packs/gmail/categorize/categorize.go`): pass `"rule:<name>"` for rule-decided rows and `"ai:<assigner.Name()>"` for AI-decided rows into the CSV write call (depends on T007/T011 and T019) — *done in Phase 3*
- [X] T021 [P] [US4] Add a `Source string` field to `AssignmentEntry` in `packs/expenses/internal/event/state.go`, and a way to set it alongside `EventID`/`Confidence` (extend `Mark` or add `MarkWithSource`); leave empty on legacy entries (no migration needed — `encoding/json` zero-values missing fields) — *done in Phase 5 alongside T016*
- [X] T022 [US4] Wire source-tagging into `update-event`'s `Run()` (`packs/expenses/internal/event/updateevent.go`): record `"rule:<name>"` for rule-decided rows and `"ai:<matcher.Name()>"` for AI-decided rows (depends on T016 and T021) — *done in Phase 5, including the pre-existing bulk-assign ("manual") and fill-similar call sites*
- [X] T023 [P] [US4] Add `[rule:<name>]` / `[ai:<provider>]` tags to both jobs' existing `--dry-run` print statements (`categorize.go`'s per-row `fmt.Printf`, `updateevent.go`'s per-row dry-run prints), per `contracts/cli.md`'s Output contract — *done in Phases 3 and 5*
- [X] T024 [US4] Manually run `quickstart.md` Scenario 4 (Source column/field present and correct in both files) and Scenario 5 (a rule with an invalid taxonomy outcome is rejected, logged, and falls through to AI rather than being silently written) — validated: gmail's transactions.csv Source column and expenses' state.json source field both confirmed on real (non-dry-run) writes; Scenario 5 validated in Phase 3

**Checkpoint**: All four user stories are independently functional; every classified transaction's decision source is auditable end-to-end.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and final regression confirmation across both packs.

- [X] T025 [P] Write `docs/adr/0016-expense-rules-engine.md` documenting the decision (shared `data/config/expense-rules.yaml` via `AUTO_DATA_DIR`, duplicated per-pack Go loader/matcher rather than a cross-repo import, additive `Source` auditability), mirroring the style of ADR 0010/0011
- [X] T026 [P] Update `packs/gmail/RUNBOOK.md` with an entry for the `categorize` changes (new `rules.go`, `--rules-file` flag, `Source` column)
- [X] T027 [P] Update `packs/expenses/RUNBOOK.md` with an entry for the `update-event` changes (new `rules.go`, `--rules-file` flag, `Source` field)
- [X] T028 Re-run `quickstart.md` Scenario 0 end-to-end against both packs with an empty `data/config/expense-rules.yaml` to confirm zero regression (SC-005) after all phases are complete — validated: both `gmail categorize` and `expenses update-event`, given an empty rules file, sent every row to the AI and failed at the identical pre-feature point (`DEEPSEEK_API_KEY not set`), confirming byte-identical behavior with zero rules defined

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: No dependency on Setup's file contents, only needs `packs/expenses/go.mod` to exist (it does) — BLOCKS US3 only, not US1/US2/US4's gmail-side work
- **User Story 1 (Phase 3)**: Depends on Setup (T001, needs the file path to exist) — no dependency on Foundational
- **User Story 2 (Phase 4)**: Depends on User Story 1's `rules.go`/`Config` existing (same files, extended in place) — not independent of US1's *code*, though it is independently testable/valuable once US1 ships
- **User Story 3 (Phase 5)**: Depends on Setup and Foundational (T004) — independent of US1/US2's gmail-side code (separate pack, separate files)
- **User Story 4 (Phase 6)**: Depends on US1 (T007) and US3 (T016) both existing, since it instruments both jobs' decision paths
- **Polish (Phase 7)**: Depends on all four user stories being complete

### Parallel Opportunities

- T002 and T003 (different manifest files) run in parallel
- T005 and T006 (test file vs. implementation file) can be drafted in parallel, though T005's tests should fail against a stub until T006 lands
- T014 and T015 similarly
- T019 and T021 (gmail CSV schema vs. expenses state schema — different packs, different files) run in parallel
- T025, T026, T027 (independent documentation files) run in parallel

---

## Parallel Example: User Story 1

```bash
# Draft tests and implementation together (same story, different files):
Task: "Table-driven tests in packs/gmail/categorize/rules_test.go for load/degrade/validate/merchant/keyword matching"
Task: "Create packs/gmail/categorize/rules.go with ExpenseRule/ExpenseRules/MatchCondition/Outcome types and LoadExpenseRules"
```

## Parallel Example: User Story 4

```bash
# gmail-side and expenses-side auditability are fully independent files:
Task: "Add Source column to packs/gmail/store/csv.go's csvHeader"
Task: "Add Source field to AssignmentEntry in packs/expenses/internal/event/state.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 3: User Story 1 (Foundational/Phase 2 is not required for US1 — it only gates US3)
3. **STOP and VALIDATE**: Run `quickstart.md` Scenarios 0 and 1 against a scratch CSV
4. At this point, `gmail-categorize` alone already delivers the feature's core value (deterministic, cost-free classification of known merchants) with zero risk to `expenses-update-event`, which is untouched

### Incremental Delivery

1. Setup → Phase 3 (US1) → validate → this alone is shippable
2. Add Phase 4 (US2) → validate → richer gmail-side rules
3. Add Phase 2 (Foundational) + Phase 5 (US3) → validate → rules now also inform event-matching
4. Add Phase 6 (US4) → validate → full auditability across both jobs
5. Phase 7 (Polish) → ADRs, runbooks, final regression check

### Parallel Team Strategy

With two developers: one can own Phases 1, 3, 4, and the gmail-half of Phase 6 (T019, T020, T023's gmail line, T026); the other can own Phase 2 and Phase 5 and the expenses-half of Phase 6 (T021, T022, T023's expenses line, T027) — the two packs share no code, only the read-only rules file, so this split has no merge conflicts.

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- No repo-wide automated test runner exists in this workspace (same finding as `specs/001-job-orchestrator`) — `go test ./...` within each pack is the verification unit, plus `quickstart.md`'s manual scenarios
- Commit after each task or logical group, per the repo's existing convention (see root `RUNBOOK.md` entries for prior features)
- Verify new tests fail (or are meaningfully absent) before the corresponding implementation task lands
- Avoid: introducing a shared Go import between `packs/gmail` and `packs/expenses` — duplication across the two `rules.go` files is intentional (see `research.md` Decision 3), not an oversight to "fix" during implementation
