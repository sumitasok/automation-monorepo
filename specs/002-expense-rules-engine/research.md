# Phase 0 Research: Expense Classification Rules Engine

No `[NEEDS CLARIFICATION]` markers remained in `spec.md` (the two open design
questions — rule-vs-AI precedence and conflict resolution — were resolved
there as documented Assumptions). This research phase instead investigates
*how* to realize those assumptions against the actual, current state of the
two consumer packs, since the spec deliberately left implementation choices
open.

## Decision 1: Where the shared rules file lives

**Decision**: `data/config/expense-rules.yaml`, at the workspace root, read by
both packs via the `$AUTO_DATA_DIR` environment variable.

**Rationale**: Three candidate locations were inspected:

- Root `config/<pack>/` (e.g. `config/gmail/`, `config/expenses/`) — per
  `config/README.md` and `.gitignore` (`config/*` ignored except `README.md`
  and `config/ai/*.example.yaml`/`README.md`), this entire tree is **git-ignored
  secrets/machine-local overrides**. A rules file here would silently fail to
  sync across machines and would misuse a location documented as "nothing in
  here except this README is versioned." Rejected.
- A pack-local file (e.g. `packs/gmail/config/expense-rules.yaml`, next to the
  existing `taxonomy.yaml`) — works for gmail alone, but `expenses-update-event`
  also needs to read the identical file, and `packs/expenses` cannot reach
  into a sibling pack's config directory without a fragile, undocumented
  relative path. Rejected as not actually shared.
- `data/config/expense-rules.yaml` — this location is already **documented and
  committed** for exactly this purpose. `data/README.md` states: "`config/` —
  VERSIONED YAML. Settings, lookup tables, small structured data you hand-edit.
  Input to jobs. Committed, syncs on `git pull`." The existing
  `data/config/global.yaml` (already tracked in git) is annotated "Cross-job
  configuration... Jobs read this via `AUTO_DATA_DIR/config/global.yaml`."
  `AUTO_DATA_DIR` is already injected into **every** job's process
  unconditionally by `framework/tools/auto`'s `execute_job()` (`"AUTO_DATA_DIR":
  str(DATA)`), regardless of the job's manifest. No pack consumes this today —
  this feature is the first real consumer — but it is the mechanism the
  workspace already built and documented for cross-job/cross-pack shared
  config, and needs no manifest or `auto` changes to use. **Accepted.**

**Alternative considered and rejected**: The `--ai-profiles-dir` flag in
`packs/gmail/main.go` defaults to a hardcoded relative path
(`../../config/ai`) rather than using `AUTO_DATA_DIR`. This is real, working
precedent for pack → workspace-root file access, but it predates (or simply
never adopted) the `AUTO_DATA_DIR`/`data/config/` convention, and a relative
`../../` path breaks if a job's `workdir` ever changes (whereas the env var
does not). Rather than propagate the older pattern, this feature adopts the
newer, already-documented one — and each pack's flag still accepts an
explicit relative-path override for local `go run` outside `auto` (see
Decision 4), so nothing is lost for local development.

## Decision 2: Rule schema and Go representation

**Decision**: Model the new rule file directly on the shape of the existing
`packs/gmail/discover/rules.go` (`CategorizerRule`/`CategorizerRules`,
consumed from `config/categorizer_rules.yaml`) — an ordered YAML list,
first-match-wins, graceful degrade to an empty rule set when the file is
absent — extended with the richer condition/outcome vocabulary this feature
needs.

**Rationale**: `discover/rules.go` is proof this exact shape (ordered rules,
domain/keyword conditions, `LoadX` returning an empty result rather than an
error on a missing file) is already idiomatic in this codebase, for a
different classification problem (email → catalog category, not transaction →
expense category). Reusing the shape — not the code, since the condition and
outcome fields differ — keeps the new rules file recognizable to the same
person (Sumit) who already authors `categorizer_rules.yaml` by hand. See
`data-model.md` and `contracts/expense-rules.schema.md` for the concrete
field-by-field schema.

**Alternatives considered**: A rule-priority integer field (rather than pure
declaration order) was considered and rejected — `categorizer_rules.yaml`
already establishes "rules are evaluated in order; the first matching rule
wins" with a comment convention ("put more specific rules before broad
rules"), and the spec's Assumption already commits to that same convention.
Adding a redundant priority number would invite the two mechanisms to
disagree.

## Decision 3: Duplicated Go code, shared data — not a shared Go module

**Decision**: Write the rule-loading/matching logic once per pack
(`packs/gmail/categorize/rules.go`, `packs/expenses/internal/event/rules.go`),
not as a shared importable Go package.

**Rationale**: `packs/gmail` is a separate git submodule/repository
(`sa.automation.gmail`, per `.gitmodules`) with its own `go.mod`
(`github.com/sumitasok/sa.automation.gmail`); `packs/expenses` is a distinct
Go module (`github.com/sumitasok/sa.automation.expenses`) living directly in
the monorepo. ADR 0011 decision 2 already confronted this exact boundary and
chose duplication over cross-repo sharing ("A second pack writing columns
into a file it doesn't own would duplicate schema knowledge across two
independently-versioned repos") — and decision 4 already duplicates the
DeepSeek-provider Strategy interface (`Assigner` in gmail, `Matcher` in
expenses) between the two packs rather than extracting a shared library. This
feature's rule engine is small enough (a struct, a loader, an evaluator) that
duplicating it follows the grain of the codebase rather than introducing the
first-ever cross-repo Go dependency between the two packs.

## Decision 4: CLI flag wiring

**Decision**: Both `categorize` (gmail) and `update-event` (expenses) gain a
`--rules-file` flag. Its default is computed at startup: if `AUTO_DATA_DIR` is
set (i.e. the job is running under `auto run`/`auto orchestrate`), the
default is `$AUTO_DATA_DIR/config/expense-rules.yaml`; otherwise it falls back
to a relative path from the pack's own root
(`../../data/config/expense-rules.yaml`), matching how every other flag in
both `main.go` files already provides a workable relative default for local
`go run .` invocations outside `auto` (e.g. `--csv
../gmail/transactions.csv`, `--events config/events.json`).

**Rationale**: Keeps the feature usable identically whether invoked via `auto`
(the normal path, both jobs' manifests already declare `runtime.env` entries
consumed this way) or directly via `go run .` during development, without
requiring a manifest change to inject a new env var — `AUTO_DATA_DIR` is
already unconditional.

## Decision 5: Decision-source auditability storage

**Decision**:
- `packs/gmail`: add one new `Source` column to `transactions.csv`
  (`store/csv.go`'s `csvHeader`), holding `"rule:<rule-name>"` or
  `"ai:<provider>"`. Empty for legacy rows written before this feature ships —
  an additive, backward-compatible schema change, exactly like ADR 0010's
  original three-column addition ("The first `categorize` run migrates legacy
  13-column rows to 16 columns") and ADR 0013's later `Note` column.
- `packs/expenses`: add one new `Source string` field to `AssignmentEntry`
  (`internal/event/state.go`), same two values. Empty for entries written
  before this feature ships (`encoding/json` leaves missing fields as the zero
  value on load, so old `state.json` files remain readable with no migration
  step).

**Rationale**: Both packs already have exactly one enrichment/ledger file they
alone own (single-writer-per-file, ADR 0005) with established precedent for
additive schema growth. Piggybacking on the existing files (rather than a new
side-table) keeps FR-010 ("record... whether the classification came from a
rule... or the AI") queryable with the tools Sumit already uses to inspect
these files (opening the CSV, reading `state.json`).

## Decision 6: What "retroactive re-application" actually means here (correcting a spec assumption)

**Finding**: `spec.md`'s Assumptions section states retroactive re-application
would reuse "the existing recategorisation/bulk-assign mechanisms already
present in the codebase (`gmail-recategorize`, `expenses-bulk-assign`)." On
inspection, `gmail-recategorize` (`go run . recategorize`) operates on a
**different file and domain entirely** — `email_catalog.csv`'s finance/job/
social/etc. sender-domain classification (`packs/gmail/discover/recategorize.go`
+ `categorizer_rules.yaml`), not `transactions.csv`'s expense
Category/SubCategory/Labels. There is no existing job that forces
re-classification of an already-categorized `transactions.csv` row.

**Resolution**: This feature does not add such a job — FR-011's requirement
("MUST NOT alter transactions already classified... unless the user
explicitly triggers a re-categorisation") is satisfied by today's existing,
sufficient capability: `categorize.Run()` already selects rows via
`r.NeedsCategory()`, so an already-categorized row is re-evaluated (by rule or
AI) only if its Category/SubCategory/Labels cells are first cleared by hand —
which is "explicit" by construction (a deliberate manual edit) and requires no
new code. `expenses-bulk-assign` genuinely is the existing mechanism on the
expenses side (it already imports manual `MessageID,EventID` overrides into
`state.json`, described as overriding "any existing auto-matched
assignments") and needs no change for this feature. `spec.md`'s Assumptions
line is superseded by this finding; no spec edit is required since FR-011 as
worded is satisfied either way, but future readers of this plan should treat
this research note, not the original assumption text, as authoritative on the
gmail side.

## Decision 7: Time-of-day / day-of-week condition feasibility

**Finding**: `packs/gmail/parser/parser.go`'s `NormaliseDate`/`ParseWithLayout`
already normalize transaction timestamps to `"2006-01-02 15:04:05"` (date +
time) whenever the source bank alert includes a time component, and to a
date-only string otherwise — so `TxnDate` values are a heterogeneous mix
depending on the originating bank filter. A rule's `time_between`/`day_of_week`
condition therefore does not always have data to evaluate against.

**Decision**: Parse `TxnDate` defensively; if it doesn't include a time
component (or fails to parse at all), any condition requiring time-of-day
fails closed (does not match) and the transaction falls through exactly as
the spec's Edge Cases section already requires ("the rule must not match...
and the transaction must fall through to the existing AI classification").
Day-of-week only needs the date portion, so it degrades more gracefully (it
still works even for date-only `TxnDate` values); only `time_between`
specifically requires the time component.
