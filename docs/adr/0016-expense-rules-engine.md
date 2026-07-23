# ADR 0016 — Expense classification rules engine (shared, cross-pack)

**Status:** accepted — 2026-07-23

## Context

`gmail-categorize` (ADR 0010) and `expenses-update-event` (ADR 0011) both send
every unresolved transaction to an AI provider on every run — the former to
assign a Category/SubCategory/Labels, the latter to match against (or
propose) an "event." Some of that judgement isn't actually a judgement call:
Sumit already knows, as a fixed fact, that "HungerBox" is his office
cafeteria vendor, and that an Uber taken on a weekday afternoon is his
office-to-home commute, not a novel occasion. Re-sending these known,
recurring patterns to the AI on every run costs money and occasionally
produces inconsistent results for something that should be deterministic.
See `specs/002-expense-rules-engine/spec.md` for the full feature spec.

## Decision

1. **A single, shared, versioned rules file: `data/config/expense-rules.yaml`.**
   Read by both packs via the `$AUTO_DATA_DIR` environment variable —
   already injected into every job's process unconditionally by
   `framework/tools/auto`'s `execute_job()`, and already documented for
   exactly this purpose (`data/README.md`: "Cross-job configuration... Jobs
   read this via `AUTO_DATA_DIR/config/global.yaml`"), just not previously
   consumed by any pack. Root `config/<pack>/` was rejected — that whole
   tree is git-ignored secrets (`config/README.md`), not shared, versioned
   definitions.

2. **Ordered rules, first-match-wins, evaluated before the AI call.** Each
   rule states match conditions (`merchant_contains`, `keyword_contains`,
   `day_of_week`, `time_between`, `amount_between` — all AND together) and an
   outcome (`category`/`subcategory`/`labels` and/or `event_relevance:
   routine`). This mirrors the gmail pack's own pre-existing
   `discover/rules.go` + `config/categorizer_rules.yaml` convention (a
   different classification problem — email→catalog category, not
   transaction→expense category — but the same ordered, first-match-wins,
   graceful-missing-file shape), which Sumit already authors by hand.

3. **A confirmed rule match decides the outcome directly — no AI call for
   that row.** This is the crux of the feature: a rule isn't advisory
   context fed alongside the taxonomy for the AI to weigh, it's a
   deterministic override that skips the AI entirely for that transaction.
   Rejected: treating rules as extra prompt context (still calls the AI
   every time, doesn't save cost, doesn't guarantee determinism).

4. **Duplicated Go code per pack, not a shared Go module.** `packs/gmail` is
   an independently-versioned git submodule (`sa.automation.gmail`);
   `packs/expenses` is a separate Go module living in-repo. ADR 0011 decision
   2 already chose duplication over cross-repo sharing for this exact
   boundary ("duplicate schema knowledge across two independently-versioned
   repos"), and decision 4 already duplicates the DeepSeek-provider Strategy
   interface (`Assigner` in gmail, `Matcher` in expenses) rather than
   extracting a shared library. `packs/gmail/categorize/rules.go` and
   `packs/expenses/internal/event/rules.go` are independent implementations
   of the identical file contract, following that same grain.

5. **`packs/expenses` gains its first external dependency:
   `gopkg.in/yaml.v3`.** This breaks ADR 0011 decision 1's "pure-stdlib Go,
   no external dependencies" — a deliberate, minimal exception. The rules
   file needs to be one human-editable format both packs parse identically;
   YAML is already the format `packs/gmail` uses for its own
   `categorizer_rules.yaml`/`taxonomy.yaml`, specifically because rule
   authors need to leave inline rationale as comments (something JSON can't
   do, and something this rules file already uses extensively — see its
   `description` fields and header comment block).

6. **Rule outcomes validate against the existing taxonomy — no new
   vocabulary.** A rule's `category`/`subcategory` must resolve via the same
   `Taxonomy.Resolve`/`ResolveLabels` an AI assignment already goes through
   (ADR 0010 decision 5); an unresolvable outcome is rejected at evaluation
   time (logged, falls through to AI) rather than silently written. Rules
   introduce no parallel classification system.

7. **Decision-source auditability via additive schema fields.** gmail's
   `transactions.csv` gains a `Source` column (`rule:<name>` /
   `ai:<provider>`); expenses' `state.json` `AssignmentEntry` gains a
   `Source` field. Both are additive — legacy rows/entries read back empty,
   no migration step, following the exact precedent of ADR 0010's original
   Category/SubCategory/Labels column addition and ADR 0013's later `Note`
   column.

## Consequences

- Adopting the feature with an empty (or absent) `expense-rules.yaml` is a
  no-op: both jobs behave exactly as they did before this ADR.
- A rule's effect is forward-only — adding, editing, or disabling a rule
  never retroactively changes already-classified transactions; only rows
  still missing an outcome are evaluated on the next run (same idempotent
  selection both jobs already used before this feature).
- The rules file itself is the only artifact truly shared between the two
  packs; no Go code crosses the pack boundary, consistent with ADR 0011's
  existing read-only cross-pack file convention (`expenses` already reads
  gmail's `transactions.csv` the same way).
- Complements ADR 0005 (definitions vs. produced data — the rules file is a
  definition, `data/config/`'s existing home for exactly that) and ADR 0010
  and 0011 (taxonomy/registry validation, provider-Strategy duplication).
  This feature is the first real consumer of the `AUTO_DATA_DIR` env var /
  `data/config/` cross-job-configuration convention `framework/tools/auto`
  and `data/README.md` already established but that no pack had used until
  now.
