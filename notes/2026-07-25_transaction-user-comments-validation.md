# User comments inform transaction classification — implementation & validation

## Objective

Implement spec `003-transaction-user-comments` (`/speckit-plan #003` →
`/speckit-tasks #003` → `/speckit-implement #003`) end to end: plan, tasks,
and working code across `packs/gmail` and `packs/expenses` for all six user
stories (comment-driven categorisation, comment-driven event matching,
already-decided-row reopening with rule precedence, decision-source
visibility, opt-in retroactive suggestions, and git-committed rule capture).

## Approach

Read the existing spec 002 (expense-rules-engine) implementation in both
packs as the established precedent for shared-file, dual-pack features
(rules loading, `Source` tracking, duplicated-not-shared Go code across the
two independently-versioned repos), then extended that exact shape rather
than inventing a new one. Full design captured in
`specs/003-transaction-user-comments/{plan,research,data-model}.md` and
`contracts/`.

## Inputs

- `specs/003-transaction-user-comments/spec.md` (pre-existing, from an
  earlier `/speckit-specify` run).
- `specs/002-expense-rules-engine/*` and the two packs' existing
  `rules.go`/`state.go`/`categorize.go`/`updateevent.go` as precedent.
- `docs/adr/0010`, `0011`, `0013`, `0016` for the constraints this feature
  had to respect (taxonomy/registry validation, single-writer-per-file,
  additive-schema-only growth).

## Steps & Findings

1. **Plan phase**: wrote `plan.md`, `research.md` (12 numbered decisions),
   `data-model.md`, `contracts/cli.md`, `contracts/rule-capture.md`,
   `quickstart.md`. No `.specify/extensions.yml` exists in this repo, so
   pre/post-plan/tasks/implement hooks were skipped silently throughout, per
   each command's own instructions.
2. **Tasks phase**: generated `tasks.md`, 57 tasks across 9 phases
   (Setup, Foundational, US1–US6, Polish), organized by user story per the
   spec's own priorities (US1/US2/US4 = P1, US3/US5 = P2, US6 = P3).
3. **Implementation** (all tasks T001–T050 completed; see `tasks.md` for the
   per-task checklist):
   - **Foundational**: `UserComment`/`CommentConsidered` columns
     (`packs/gmail/store/csv.go`); `Comment` field on
     `AssignmentEntry`/`csvtxn.Txn` (`packs/expenses`); `isInteractive()`
     TTY-check helper duplicated in both packs.
   - **US1/US2**: `Comment` field on both packs' AI `Item` types;
     descriptive-context-only system-prompt sentence; `Source` gains a
     `+comment` suffix.
   - **US4**: row-selection predicate upgraded (`NeedsReclassification()` /
     `needsReprocessing()`); a present comment always bypasses
     `expense-rules.yaml` matching for that row.
   - **US3**: covered by US1/US2/US4's `Source` work; locked down with
     explicit unit-test assertions on the exact `Source` value vocabulary.
   - **US5**: `suggest.go` in both packs — candidate selection (same
     merchant and/or same prior rule, excluding self and already-matching
     outcomes, oldest-first) plus a `bufio.Scanner`-based approve/skip loop;
     `--suggest-similar` flag wired into both `main.go`s.
   - **US6**: `rulecapture.go` in both packs — git-clean precondition
     (`git status --porcelain`), hand-appended YAML entry (not a full
     remarshal, to preserve the file's header comments byte-for-byte), `git
     add`/`git commit`, all scoped to the workspace root derived from the
     already-resolved `--rules-file` path.
4. **Testing**: table-driven Go unit tests added alongside every change
   (comment dirty-tracking, rule-bypass precedence, candidate selection,
   git-backed rule capture against real temp git repos). No AI provider or
   real terminal was available in this environment, so:
   - AI-calling paths (US1/US2/US4) are exercised via stub
     `Assigner`/`Matcher` implementations that record what `Item` they were
     sent — verifying the comment reaches the payload and the resulting
     `Source`, without a live DeepSeek call.
   - The interactive approve/skip and rule-capture *prompts* themselves
     (`isInteractive()` gating) are not exercised end-to-end here — this
     environment has no TTY on stdin — but the pure candidate-selection
     logic they call into (`suggestCandidates`) and the git-backed
     write/commit sequence (`captureRule`) are both directly unit-tested.

## Results

```
packs/gmail:     go build ./... && go vet ./... && go test ./...  →  67 passed, 8 packages
packs/expenses:  go build ./... && go vet ./... && go test ./...  →  26 passed, 3 packages
```

New/changed files:
- `packs/gmail`: `store/csv.go`, `store/csv_test.go` (new),
  `categorize/{categorize,deepseek,interactive(new),suggest(new),
  rulecapture(new),categorize_test,suggest_test(new),rulecapture_test(new)}.go`,
  `main.go`, `jobs/gmail-categorize/manifest.yaml`, `RUNBOOK.md`.
- `packs/expenses`: `internal/event/{state,matcher,updateevent,
  interactive(new),suggest(new),rulecapture(new),state_test(new),
  updateevent_test(new),suggest_test(new),rulecapture_test(new)}.go`,
  `internal/csvtxn/csvtxn.go`, `internal/event/{fillsimilar,bulkassign}.go`
  (Mark() signature update only), `main.go`,
  `jobs/expenses-update-event/manifest.yaml`, `RUNBOOK.md`.
- Root repo: `specs/003-transaction-user-comments/{plan,research,
  data-model,tasks,quickstart}.md` + `contracts/`,
  `docs/adr/0017-user-comment-driven-classification.md`, this file.

## Interpretation

All three P1 stories (US1, US2, US4) and both P2/P3 stories (US5, US6) are
implemented and unit-tested; US3 (decision-source visibility) was
substantially delivered as a byproduct of US1/US2/US4 and is now
regression-guarded by explicit `Source`-vocabulary assertions. The feature
is additive throughout — every new code path is gated on a non-empty,
trimmed comment (or, for US5/US6, on `isInteractive()`), so a workspace with
zero comments ever written behaves identically to before this feature
(SC-003), matching spec 002's own zero-regression bar.

## Caveats

- `quickstart.md`'s scenarios that require a live DeepSeek call or a real
  interactive terminal (Scenarios 1, 2, 8, 9 specifically) were **not**
  executed end-to-end against live data in this session — only their
  underlying logic, via unit tests with stubs/fakes. Running them for real
  (with `DEEPSEEK_API_KEY` set, from an actual terminal, against a scratch
  copy of `transactions.csv`) is a reasonable next step before relying on
  `--suggest-similar`/rule capture against real financial data.
- Rule capture's git commits are scoped to whatever git repository contains
  `data/config/expense-rules.yaml` (this monorepo) — confirmed by the
  `workspaceRoot()` unit test and the temp-repo `captureRule` tests, but not
  exercised against this actual repo's real history.
