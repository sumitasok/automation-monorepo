# Phase 1 Data Model: Expense Classification Rules Engine

## Rule

One entry in `data/config/expense-rules.yaml`'s ordered `rules:` list.

| Field | Type | Required | Notes |
|---|---|---|---|
| `name` | string | yes | Unique, stable identifier (kebab-case, mirrors event-registry ids in ADR 0011). Written verbatim into the `Source` column/field as `rule:<name>` (Decision 5). |
| `description` | string | no | Free-text "why" — the human rationale (e.g. "HungerBox is the office cafeteria vendor"). Not evaluated; documentation only, same role as `categorizer_rules.yaml`'s per-rule comments but structured so it survives programmatically (e.g. a future "explain this rule" report). |
| `enabled` | bool | no (default `true`) | FR-013. A disabled rule is parsed but skipped during evaluation — never deleted by tooling. |
| `applies_to` | list of `categorize` \| `event` | yes, at least one value | FR-012. Which decision(s) this rule may produce an outcome for. A rule scoped to `[event]` only is never consulted by `gmail-categorize`, and vice versa. |
| `match` | MatchCondition | yes | See below. All stated conditions AND together (FR-002). |
| `outcome` | Outcome | yes | See below. |

Rules are evaluated **in file order**; the first `enabled` rule (scoped to the
current decision type) whose `match` conditions all hold wins (FR-006,
research Decision 2). Order in the file *is* precedence — there is no
separate priority number.

## MatchCondition

All fields optional; an absent field is not evaluated (vacuously true) — a
rule with only `merchant_contains` set matches on that alone. At least one
condition field must be present (an empty `match: {}` is a config error,
rejected at load time, since it would unconditionally match everything).

| Field | Type | Semantics |
|---|---|---|
| `merchant_contains` | list of string | Case-insensitive substring match against the transaction's merchant/counterparty field. Matches if **any** listed string is a substring (OR within the field; the field itself ANDs with sibling fields). |
| `keyword_contains` | list of string | Case-insensitive substring match against the transaction's free-text description/subject/info fields (whichever the pack's `Item` exposes). Same any-of-list semantics as `merchant_contains`. |
| `day_of_week` | list of `mon`\|`tue`\|`wed`\|`thu`\|`fri`\|`sat`\|`sun` | Matches if the transaction's date falls on one of the listed days. Requires only the date portion of the transaction's timestamp. |
| `time_between` | `["HH:MM", "HH:MM"]` (start, end) | Matches if the transaction's time-of-day falls within `[start, end]` inclusive, in the timestamp's local wall-clock value (no timezone conversion — the CSV already stores wall-clock bank-alert time). Requires the transaction's timestamp to include a time component (research Decision 7); if it doesn't, this condition fails closed (does not match). |
| `amount_between` | `[min, max]` (numbers) | Matches if the transaction's amount falls within `[min, max]` inclusive. |

## Outcome

| Field | Type | Required | Notes |
|---|---|---|---|
| `category` | string | only if `applies_to` includes `categorize` and this rule sets a category | Must resolve against `config/taxonomy.yaml` via the existing `Taxonomy.Resolve` — an unresolvable value is rejected at evaluation time (FR-008), logged, and treated as if the rule hadn't matched (falls through to AI) for that row, matching the Edge Cases section's requirement to reject rather than silently apply an invalid outcome. |
| `subcategory` | string | paired with `category` | Same validation. |
| `labels` | list of string | no | Filtered/capped through `Taxonomy.ResolveLabels`, same as an AI-produced assignment. |
| `event_relevance` | `routine` (only defined value today) | no | FR-003. When set to `routine` and `applies_to` includes `event`, `expenses-update-event` marks the transaction as intentionally event-less (`state.Mark(id, "", 1.0)`) without calling the AI matcher for that row. |

A rule scoped to `applies_to: [categorize, event]` may set both the
category/subcategory/labels fields and `event_relevance` in one outcome block
— both consumers read only the fields relevant to them.

## ClassificationDecision (cross-cutting, not a new file — an additive field on existing records)

Represents how one transaction ended up with its current outcome. Not a new
entity/table; realized as:

- **gmail side**: a new `Source` column on each `transactions.csv` row,
  alongside the existing `Category`/`SubCategory`/`Labels` columns. Value is
  `rule:<rule-name>` or `ai:<provider-name>`; empty for rows written before
  this feature shipped.
- **expenses side**: a new `Source` field on each `AssignmentEntry` in
  `state.json`, alongside the existing `EventID`/`Confidence`/`AssignedAt`.
  Same two value shapes; empty for legacy entries.

| Field (conceptual) | Type | Notes |
|---|---|---|
| `source_kind` | `rule` \| `ai` | Which mechanism produced the outcome. |
| `source_name` | string | The rule's `name`, or the AI provider's name (e.g. `deepseek`) — whatever `Assigner.Name()`/`Matcher.Name()` already returns today. |

## Existing entities referenced (unchanged by this feature)

- **Transaction** (`packs/gmail/store` row / `packs/expenses/internal/csvtxn`
  row): merchant, info/description, subject, amount, `TxnDate`, `MessageID`.
  Read-only input to rule matching; no new fields needed on the read side.
- **Taxonomy** (`packs/gmail/categorize/taxonomy.go`): unchanged; reused
  as-is to validate rule outcomes exactly as it validates AI outcomes.
- **Event registry / assignment ledger** (`packs/expenses/internal/event`):
  unchanged in shape aside from the additive `Source` field above; a rule's
  `event_relevance: routine` outcome uses the registry's existing "no event"
  representation (empty `EventID`) rather than introducing a new state.

## Validation rules summary

1. `data/config/expense-rules.yaml` missing entirely → empty rule set, no
   error (SC-005).
2. A rule with no condition fields set → rejected at load time (config
   error), since it would match every transaction unconditionally.
3. A rule's `category`/`subcategory`/`labels` that don't resolve against the
   current taxonomy → that rule is treated as a non-match for the affected
   row at evaluation time; logged as a warning (mirrors how an unresolvable
   AI assignment is already skipped and logged in `categorize.go`).
4. A `time_between` condition against a transaction whose timestamp has no
   time component → condition fails closed; rule does not match.
5. Two+ enabled, in-scope rules could match the same transaction → only the
   first (in file order) is evaluated/applied; later rules are never
   consulted for that transaction (short-circuit, not "collect all
   matches then pick one").
