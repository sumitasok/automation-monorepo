# Implementation Plan: User Comments Inform Transaction Classification

**Branch**: `003-transaction-user-comments` | **Date**: 2026-07-25 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/003-transaction-user-comments/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

Add a new, additive `UserComment` column to `transactions.csv` that Sumit edits by
hand. Both `gmail-categorize` and `expenses-update-event` are extended to (a) pass
a row's comment to the AI as descriptive context whenever the row is
classified/matched, (b) treat a newly-added or edited comment as grounds to
re-open an already-decided row — overriding a would-be `expense-rules.yaml` rule
match for that row — while leaving an unchanged comment alone, and (c) record on
the existing `Source` field whether a comment shaped the outcome. Two further,
lower-priority capabilities layer on top, gated to interactive terminal runs
only: an opt-in retroactive-suggestion flow that walks the user through other
already-decided rows resembling a just-corrected one (Story 5), and a
same-flow prompt to capture an approved correction as a new `expense-rules.yaml`
rule, git-committed automatically (Story 6). This is a direct, in-place
extension of the same two packs (and the same shared rules file) that
`specs/002-expense-rules-engine` already established — no new pack, no new
storage location, no shared Go module between the two independently-versioned
repos.

## Technical Context

**Language/Version**: Go 1.22 (both `packs/gmail` and `packs/expenses` are existing Go modules on this version already; no version change).

**Primary Dependencies**: No new external dependencies in either pack. `packs/gmail` already depends on `gopkg.in/yaml.v3` (rules/taxonomy) and the Gmail API client; `packs/expenses` already depends on `gopkg.in/yaml.v3` (added by spec 002, for the shared rules file) plus stdlib. The one new capability this feature needs beyond existing imports — TTY detection to distinguish an interactive run from a scheduled/cron one (FR-013) — is done with stdlib only (`os.Stdin.Stat()`'s `os.ModeCharDevice` bit), so `packs/expenses` stays at exactly one external dependency (unchanged from spec 002). Rule-capture git commits (Story 6) shell out to the `git` binary via `os/exec`, the same way the rest of this workspace already assumes `git` is on `PATH` (per the root `CLAUDE.md`/`RUNBOOK.md` auto-commit convention) — no Go git library is introduced.

**Storage**: `data/gmail/transactions.csv` (gmail pack, sole writer) gains two additive columns: `UserComment` (user-authored, hand-edited) and `CommentConsidered` (system-written snapshot of the comment value last used to produce the row's current outcome — the dirty-tracking field that lets FR-010/FR-011 tell "new/changed comment" apart from "same comment, already reflected"). `packs/expenses/state.json`'s `AssignmentEntry` gains one additive field, `Comment string`, playing the same dirty-tracking role for event assignments (expenses never writes to `transactions.csv`, ADR 0011 decision 2, so it cannot reuse gmail's `CommentConsidered` column — it needs its own copy in the file it already owns). `data/config/expense-rules.yaml` (the shared, git-committed rules file from spec 002) is also a write target, but only for the opt-in, user-approved Story 6 flow — read by both packs already; writes are conditional on the file's git-clean-first check (FR-020).

**Testing**: Same convention both packs already follow (spec 002): table-driven Go unit tests colocated with the new/changed code (`categorize/*_test.go`, `internal/event/*_test.go`) for the pure logic (comment-changed detection, rule-bypass-on-comment, similarity candidate selection, TTY/interactive gating), plus `quickstart.md` manual end-to-end scenarios for the AI-calling and git-committing paths that aren't practical to unit test against a live provider/live git history. No repo-wide test runner exists in this workspace (see `specs/001-job-orchestrator/plan.md`'s Testing note, reused by spec 002).

**Target Platform**: macOS/Linux — unchanged; the same two CLI binaries.

**Project Type**: In-place extension of two existing CLI/app-backed packs (`packs/gmail`, `packs/expenses`) plus additive fields on two already-existing files (`transactions.csv`, `state.json`) and conditional writes to a third, already-existing shared file (`data/config/expense-rules.yaml`). No new service, frontend, or top-level project.

**Constraints**: Zero comments ever written must reproduce today's behavior exactly (SC-003) — every new code path in both `Run()` functions is gated on `UserComment`/`Comment` being non-empty after trimming. A row already fully decided and whose comment is unchanged must never be touched (FR-011) — this is the same idempotent-selection discipline `NeedsCategory()`/`state.Has()` already enforce, extended rather than replaced. A comment must never be able to change the AI's output contract or the taxonomy/registry it's constrained to (FR-006) — it is appended to the prompt as clearly-delimited, labelled context, never merged into the instruction text itself. The retroactive-suggestion flow (Story 5) must never run unattended under any circumstance (FR-017) — gated on both an explicit opt-in flag AND a real TTY on stdin, not either alone, so a misconfigured cron entry still can't trigger it. Rule capture (Story 6) must never mix its own commit with pre-existing uncommitted changes to the rules file (FR-020) — checked via `git status --porcelain` scoped to that one file before any write.

**Scale/Scope**: Same personal-workspace scale as specs 001/002 — a `transactions.csv` on the order of hundreds of rows, comments added to a small, human-paced subset of them per session. No performance design needed; every new scan is linear over the already-loaded row/ledger set, matching the existing `O(rules × rows)` rule-matching and `O(rows)` CSV/state scans.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is still the unfilled template — no ratified project principles exist to gate against (same finding as specs/001 and specs/002). This plan instead holds itself to the constraints the spec, the existing ADRs, and specs/002's own precedent already establish: reuse the existing per-pack AI-provider/taxonomy/registry validation paths rather than inventing new ones (ADR 0010, ADR 0011); keep the comment strictly as descriptive AI input, never as an instruction that can widen the taxonomy/registry vocabulary (FR-006, mirrors ADR 0010/0011's "validate before write" posture); respect single-writer-per-file discipline (ADR 0005) — gmail alone writes `transactions.csv`, expenses alone writes `state.json`/`config/events.json`, and the one shared write target introduced by Story 6 (`data/config/expense-rules.yaml`) is guarded by the same git-clean-first check FR-020 requires precisely because two packs *and* a human hand-editor all touch that one file; add no new external dependency to `packs/expenses` (Technical Context above — the TTY check is stdlib-only).

**Post-Design Re-check** (after Phase 1 research/data-model/contracts/quickstart): No ratified constitution to re-gate against. The Phase 1 design adds three additive fields across two already-existing, already-committed-pattern files (`UserComment`/`CommentConsidered` columns on `transactions.csv`, `Comment` field on `state.json`'s `AssignmentEntry`) and one new conditional writer to an existing shared file (`expense-rules.yaml`, opt-in and git-guarded). No new dependency, no new service, no new cross-repo Go import. Gate still passes.

## Project Structure

### Documentation (this feature)

```text
specs/003-transaction-user-comments/
├── plan.md               # This file (/speckit-plan command output)
├── research.md           # Phase 0 output (/speckit-plan command)
├── data-model.md         # Phase 1 output (/speckit-plan command)
├── quickstart.md         # Phase 1 output (/speckit-plan command)
├── contracts/            # Phase 1 output (/speckit-plan command)
└── tasks.md              # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
# In-place extension of two existing packs, mirroring specs/002's own
# "extend what exists" structure — no new project, no shared Go module.

packs/gmail/                     # git submodule (sa.automation.gmail) — independently versioned
├── store/csv.go                 # MODIFIED — csvHeader gains "UserComment", "CommentConsidered";
│                                 #   new colUserComment/colCommentConsidered constants; Record
│                                 #   gains UserComment/CommentConsidered; new
│                                 #   NeedsReclassification() alongside existing NeedsCategory();
│                                 #   SetEnrichment gains a commentConsidered param (or a paired
│                                 #   SetCommentConsidered) so a classification pass always
│                                 #   snapshots what comment (if any) it used
├── categorize/
│   ├── categorize.go            # MODIFIED — Run()'s row-selection loop uses
│   │                             #   NeedsReclassification() instead of NeedsCategory(); a row
│   │                             #   with a non-empty current UserComment skips the rules.Match
│   │                             #   call entirely (FR-012) and always reaches the AI with its
│   │                             #   comment attached; Source gains a "+comment" suffix when a
│   │                             #   comment was considered (Story 3); Story 5/6 entry points
│   │                             #   wired in after the AI pass
│   ├── deepseek.go               # MODIFIED — Item gains Comment string `json:"comment,omitempty"`;
│   │                             #   systemPrompt/buildPrompt gain the descriptive-context-only
│   │                             #   framing and render the comment as a clearly labelled,
│   │                             #   non-instructional field (FR-006)
│   ├── suggest.go                # NEW — Story 5: interactive-only retroactive similarity
│   │                             #   suggestion loop (candidate selection + approve/skip prompt)
│   ├── rulecapture.go            # NEW — Story 6: interactive-only "capture as rule" prompt,
│   │                             #   git-clean check, YAML append, git commit
│   ├── categorize_test.go        # MODIFIED — comment-aware selection/precedence cases
│   ├── suggest_test.go           # NEW — candidate-selection unit tests (no AI, no git, no TTY)
│   └── rulecapture_test.go       # NEW — YAML-append + git-clean-check unit tests (fake git)
├── interactive.go                # NEW — shared os.Stdin TTY check (small, pure stdlib);
│                                 #   used by both suggest.go and rulecapture.go
└── main.go                       # MODIFIED — categorize subcommand gains --suggest-similar
                                   #   (Story 5 opt-in, FR-014)

packs/expenses/                  # in-repo, separate Go module — independently versioned
├── internal/event/
│   ├── state.go                  # MODIFIED — AssignmentEntry gains Comment string
│   │                             #   `json:"comment,omitempty"`; State gains a helper to decide
│   │                             #   "needs reprocessing" (Has() alone is no longer sufficient)
│   ├── matcher.go                 # MODIFIED — Item gains Comment string `json:"comment,omitempty"`;
│   │                             #   systemPrompt/buildPrompt gain the same descriptive-context
│   │                             #   framing as the gmail side
│   ├── updateevent.go            # MODIFIED — Run()'s selection loop uses the new "needs
│   │                             #   reprocessing" helper instead of bare `st.Has()`; a row with
│   │                             #   a non-empty current comment skips the routine-rule check
│   │                             #   (FR-012, event side) and always reaches the AI matcher;
│   │                             #   Source gains "+comment" suffix; Story 5/6 entry points wired
│   │                             #   in after the AI pass
│   ├── suggest.go                # NEW — Story 5, event-assignment flavour (same shape as
│   │                             #   gmail's, independent copy per spec 002's Decision 3
│   │                             #   precedent)
│   ├── rulecapture.go            # NEW — Story 6, event-assignment flavour (routine-only
│   │                             #   outcome capture)
│   ├── state_test.go             # MODIFIED (or NEW if absent) — comment dirty-tracking cases
│   ├── suggest_test.go           # NEW
│   └── rulecapture_test.go       # NEW
├── interactive.go                # NEW — same shared TTY check, independent copy
└── main.go                       # MODIFIED — update-event subcommand gains --suggest-similar

packs/gmail/csvtxn is N/A (gmail owns store/csv.go directly); packs/expenses/internal/csvtxn/csvtxn.go
├── csvtxn.go                     # MODIFIED — Txn gains UserComment (read-only mirror of gmail's
│                                 #   column, looked up by header name per the file's existing
│                                 #   "tolerates gmail adding columns" contract)

data/config/expense-rules.yaml   # UNCHANGED schema (Story 6 only ever appends well-formed
                                  # entries in the existing shape from specs/002); written to,
                                  # conditionally, by both packs' new rulecapture.go

packs/gmail/jobs/gmail-categorize/manifest.yaml       # MODIFIED — data.reads/writes note
                                                        #   already-covered by transactions.csv
                                                        #   width growth; no new data path
packs/expenses/jobs/expenses-update-event/manifest.yaml  # MODIFIED — same

# No tests/ tree, no repo-wide test runner — package-local *_test.go + quickstart.md,
# same convention as specs/001 and specs/002.
```

**Structure Decision**: Two in-place pack extensions, no new project, no shared
Go module across the `packs/gmail`/`packs/expenses` boundary — the same
structure specs/001 and specs/002 already established. Story 5 and Story 6 each
get one new, small file per pack (`suggest.go`, `rulecapture.go`) rather than
being folded into the already-large `categorize.go`/`updateevent.go`, since
both are interactive-only, optional flows with their own distinct
responsibilities (candidate walking; git-backed rule authoring) that the core
`Run()` functions should stay decoupled from — `Run()` only needs to call each
after its own pass completes, when the caller is interactive.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations — no new dependency, no new project, no new cross-repo import.
Table intentionally left empty.
