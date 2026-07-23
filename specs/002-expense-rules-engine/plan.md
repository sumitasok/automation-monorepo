# Implementation Plan: Expense Classification Rules Engine

**Branch**: `002-expense-rules-engine` | **Date**: 2026-07-23 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/002-expense-rules-engine/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

Add a single, shared, human-editable rules file (`data/config/expense-rules.yaml`) that both `gmail-categorize` (Go, `packs/gmail`) and `expenses-update-event` (Go, `packs/expenses`) load and evaluate for every uncategorised/unassigned transaction *before* calling their respective AI provider. Each rule states match conditions (merchant substring, keyword, time-of-day window, day-of-week) and an outcome (category/subcategory/labels and/or an "event-relevance: routine" signal). A confirmed rule match sets the outcome directly — no AI call for that row — and is recorded with its source (`rule:<name>` vs `ai:<provider>`) so decisions stay auditable. Rows with no rule match go through today's AI path unchanged. Because `packs/gmail` is an independently-versioned git submodule (its own repo, `sa.automation.gmail`) and `packs/expenses` is a separate Go module in-repo, the rule-loading/matching code is a small, duplicated Go file per pack (mirroring the DeepSeek-provider-Strategy duplication ADR 0011 already accepted) that both read the one shared definitions file.

## Technical Context

**Language/Version**: Go 1.22 (both `packs/gmail` and `packs/expenses` are existing Go modules on this version already; no version change)

**Primary Dependencies**: `gopkg.in/yaml.v3` (already a `packs/gmail` dependency, used for `config/taxonomy.yaml` and `discover/rules.go`'s `categorizer_rules.yaml`). `packs/expenses` is intentionally pure-stdlib (ADR 0011 decision 1, "no external dependencies, builds offline") and has no YAML library today — this feature is committed as YAML (for human-editability parity with the gmail-side rules file and the taxonomy/categorizer-rules precedent), so `packs/expenses` gains its first external dependency, `gopkg.in/yaml.v3`, solely to parse the shared rules file. This is a deliberate, minimal exception to ADR 0011 decision 1, called out in Complexity Tracking below.

**Storage**: `data/config/expense-rules.yaml` — a new file in the already-documented, already-committed `data/config/` cross-job-configuration location (`data/README.md`: "VERSIONED YAML... Input to jobs. Committed, syncs on `git pull`"; precedented by the existing, committed `data/config/global.yaml`). No database involved. Each pack additionally gains one additive field for decision-source auditability: a new `Source` column in `transactions.csv` (gmail) and a new `Source` field in `AssignmentEntry` (`packs/expenses/internal/event/state.go`'s `state.json`).

**Testing**: Both packs already ship Go unit tests for their AI-assignment logic (`categorize/categorize_test.go`; no equivalent test file yet under `internal/event`, but the package is structured for injectable fakes — `Config.Assigner` / `Config.Matcher`). This feature follows the same convention: table-driven Go unit tests for the new rule-matching logic (condition evaluation, first-match-wins, taxonomy/vocabulary validation) in both packs, plus `quickstart.md` manual end-to-end scenarios (no repo-wide test runner exists here — see `specs/001-job-orchestrator/plan.md`'s Testing note for the same established convention).

**Target Platform**: macOS/Linux — unchanged; rules evaluate inside the same two CLI binaries that already run there.

**Project Type**: In-place extension of two existing CLI/app-backed packs (`packs/gmail`, `packs/expenses`) plus one new shared, versioned data file. No new service, frontend, or top-level project.

**Constraints**: Must not change today's behavior when zero rules are defined or `expense-rules.yaml` is absent (spec SC-005) — the loader degrades gracefully to an empty rule set, exactly like `discover.LoadCategorizerRules` already does for a missing `categorizer_rules.yaml`. Must not introduce a shared Go module/import between `packs/gmail` and `packs/expenses` (they are independently-versioned repos per ADR 0011 decision 2) — logic is duplicated, data is shared. Rule outcomes must validate against the existing `config/taxonomy.yaml` (gmail side) using the existing `Taxonomy.Resolve` / `ResolveLabels`, never a new parallel vocabulary. Must not retroactively rewrite already-classified rows (spec FR-011) — evaluation only ever touches rows currently missing an outcome, exactly like both jobs' existing idempotent selection (`NeedsCategory()` / `!st.Has(messageID)`).

**Scale/Scope**: Same personal-workspace scale as the rest of this repo — a rules file with on the order of tens of rules, evaluated against a backlog of at most a few hundred uncategorised transactions per run. No performance design needed beyond linear scan (matches `discover.CategorizerRules.Categorize`'s existing O(rules × rows) approach).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is still the unfilled template — no ratified principles exist for this project, so there are no constitutional gates to evaluate against (same finding as `specs/001-job-orchestrator/plan.md`). This plan instead holds itself to the constraints the spec and existing ADRs already establish: reuse the existing per-pack AI-provider/taxonomy/registry validation paths rather than inventing new ones (ADR 0010, ADR 0011), keep `packs/expenses` a single external dependency and no more (ADR 0011 decision 1, addressed explicitly in Complexity Tracking), and respect the single-writer-per-file discipline (ADR 0005: `packs/gmail` alone writes `transactions.csv`; `packs/expenses` alone writes `state.json` and `config/events.json`).

**Post-Design Re-check** (after Phase 1 research/data-model/contracts/quickstart): No ratified constitution to re-gate against. The Phase 1 design added exactly one new dependency (`gopkg.in/yaml.v3` in `packs/expenses`, justified below), one new shared data file (additive, git-committed, no schema change to any existing file's *meaning* — only additive columns/fields), and no new service or cross-repo Go import. Gate still passes.

## Project Structure

### Documentation (this feature)

```text
specs/002-expense-rules-engine/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md         # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
# In-place extension of two existing packs, plus one new shared data file.
# No new top-level app/service. Mirrors specs/001-job-orchestrator's approach
# of extending existing tools rather than creating a new project.

data/
└── config/
    └── expense-rules.yaml      # NEW — the shared rules file (this feature's
                                  # single source of truth). Committed, versioned
                                  # (data/README.md's existing "data/config/" contract).
                                  # Read by both packs below via $AUTO_DATA_DIR,
                                  # already injected into every job's env by
                                  # framework/tools/auto's execute_job().

packs/gmail/                     # git submodule (sa.automation.gmail) — independently versioned
├── categorize/
│   ├── rules.go                # NEW — ExpenseRule/ExpenseRules: load + evaluate,
│   │                            #   mirrors discover/rules.go's shape (ordered,
│   │                            #   first-match-wins, graceful missing-file degrade)
│   ├── rules_test.go           # NEW — table-driven condition/precedence tests
│   └── categorize.go           # MODIFIED — Run() evaluates rules per item before
│                                #   batching remaining items to the AI assigner;
│                                #   writes the new Source column either way
├── store/csv.go                # MODIFIED — csvHeader gains "Source"; colSource;
│                                #   SetEnrichment (or a new SetSource) writes it
└── main.go                     # MODIFIED — categorize subcommand gains --rules-file

packs/expenses/                 # in-repo, separate Go module — independently versioned
├── internal/event/
│   ├── rules.go                 # NEW — same shape as gmail's rules.go, but its
│   │                            #   outcome is "routine, not event-worthy" (no
│   │                            #   category/subcategory concept on this side)
│   ├── rules_test.go            # NEW
│   ├── state.go                 # MODIFIED — AssignmentEntry gains Source string
│   └── updateevent.go           # MODIFIED — Run() evaluates rules per item before
│                                 #   batching remaining items to the Matcher
├── go.mod                       # MODIFIED — adds gopkg.in/yaml.v3 (see Complexity Tracking)
└── main.go                      # MODIFIED — update-event subcommand gains --rules-file

packs/gmail/jobs/gmail-categorize/manifest.yaml       # MODIFIED — data.reads gains
                                                        #   the shared rules file
packs/expenses/jobs/expenses-update-event/manifest.yaml  # MODIFIED — same

# No tests/ tree and no repo-wide test runner exists in this workspace today;
# this feature follows the established convention (package-local *_test.go +
# quickstart.md manual scenarios), same as specs/001-job-orchestrator.
```

**Structure Decision**: Two in-place pack extensions sharing one new data file — no
new project, no shared Go module across the pack boundary. This mirrors
`specs/001-job-orchestrator`'s "extend what exists" approach, and specifically
mirrors ADR 0011's own precedent of duplicating the AI-provider Strategy shape
between `packs/gmail` and `packs/expenses` rather than fighting their
independently-versioned-repo boundary with a shared library. The one shared
artifact is *data* (`data/config/expense-rules.yaml`), not code — consistent
with how `packs/expenses` already cross-reads `packs/gmail`'s `transactions.csv`
read-only (ADR 0011 decision 2) without the two packs sharing Go code.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| `packs/expenses` gains an external dependency (`gopkg.in/yaml.v3`), breaking ADR 0011 decision 1's "pure-stdlib Go, no external dependencies" | The shared rules file must be one human-editable format both packs parse identically; YAML is already the format `packs/gmail` uses for its own `categorizer_rules.yaml`/`taxonomy.yaml`, and the feature spec (FR-009) requires a "human-readable, version-controlled format" the user edits directly — re-deriving that ergonomics with hand-rolled `encoding/json` parsing (`packs/expenses`'s current stdlib-only approach) would mean either a second, JSON-flavored rules file (defeating "one shared definitions file") or a hand-written YAML-subset parser (needless, fragile complexity) | Committing the rules file as JSON instead (keeping `packs/expenses` stdlib-only) was considered and rejected: JSON has no comments, and every existing precedent for this exact kind of human-authored, ordered, commented rule list in this repo (`categorizer_rules.yaml`, `taxonomy.yaml`) is YAML specifically because rule authors need to leave inline rationale (see `categorizer_rules.yaml`'s extensive per-rule provenance comments) — a needed capability for a personal finance rules file that will accumulate exactly that kind of "why" annotation over time |

## Extension Hooks

No `.specify/extensions.yml` exists in this repository — before/after-plan hooks skipped silently, as they were for `/speckit-specify` and for `specs/001-job-orchestrator`'s planning phase.
