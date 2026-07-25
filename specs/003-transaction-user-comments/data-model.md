# Phase 1 Data Model: User Comments Inform Transaction Classification

## Transaction (existing, `data/gmail/transactions.csv`) — additive columns

| Column | Type | Written by | Notes |
|---|---|---|---|
| `UserComment` | string | user (direct CSV edit) | FR-001/FR-002. Free text, optional, hand-edited — never written by any job. Preserved verbatim across every fetch/categorize/event run (FR-002); `store.CSVStore`'s existing "preserve columns beyond gmail's own known schema" / in-place row-update path already guarantees this once the column exists, the same way `Note`/`Category` survive a fetch re-run today. |
| `CommentConsidered` | string | `gmail-categorize` only | System-written snapshot of `UserComment` at the moment it was last used to produce this row's current outcome. Empty until the row's first comment-influenced (or otherwise re-evaluated) classification. Drives the dirty-check in Decision 2 (research.md): `trim(UserComment) != trim(CommentConsidered)` means "reconsider this row." |

Both columns are appended after the current last column (`Source`), following
the same append-only convention `Category`/`SubCategory`/`Labels` (ADR 0010),
`Note` (ADR 0013) and `Source` (ADR 0016) already established. Legacy rows
missing these columns are padded to full width on load, exactly as `store.go`
already does for every prior additive column.

### `Record.NeedsReclassification()` (new, alongside existing `NeedsCategory()`)

```
NeedsReclassification() bool {
    return NeedsCategory() ||
        (trim(UserComment) != "" && trim(UserComment) != trim(CommentConsidered))
}
```

`NeedsCategory()` itself is unchanged — it still means exactly "no outcome
yet." `NeedsReclassification()` is the new row-selection predicate
`categorize.Run()` uses in place of `NeedsCategory()`.

## AssignmentEntry (existing, `packs/expenses/state.json`) — additive field

| Field | Type | Written by | Notes |
|---|---|---|---|
| `Comment` | string (`json:"comment,omitempty"`) | `expenses-update-event` only | Same role as `CommentConsidered` above, but on the expenses side, since `expenses` never writes `transactions.csv` (ADR 0011 decision 2) and must track its own "comment last considered" snapshot in the one file it owns. Empty/absent on entries written before this feature (zero-value on JSON decode, no migration needed — same precedent as the existing `Source` field's rollout). |

### `State` selection helper (new, alongside existing `Has()`)

`Has(messageID)` is unchanged — still means "an assignment entry exists."
`updateevent.Run()`'s selection loop changes from `if st.Has(id) { continue }`
to:

```
needsReprocessing(id, currentComment string) bool {
    entry, has := s.Assigned[id]
    if !has {
        return true // never assigned — existing behaviour
    }
    c := trim(currentComment)
    return c != "" && c != trim(entry.Comment)
}
```

A row is skipped (left alone) when it's already assigned AND either has no
comment or its comment matches what was last considered — otherwise it's
included in this run's batch.

## Comment as AI input (existing `Item` types, both packs) — additive field

| Type | New field | Notes |
|---|---|---|
| `categorize.Item` (gmail) | `Comment string` `json:"comment,omitempty"` | Populated only when the row's trimmed `UserComment` is non-empty. Omitted key entirely on a comment-free row (`omitempty`), so the AI request payload for comment-free rows is byte-identical to pre-feature output (SC-003). |
| `event.Item` (expenses) | `Comment string` `json:"comment,omitempty"` | Same shape, populated from `csvtxn.Txn.UserComment`. |

## Txn (existing, `packs/expenses/internal/csvtxn`) — additive field

| Field | Type | Notes |
|---|---|---|
| `UserComment` | string | Read-only mirror of gmail's `UserComment` column, looked up by header name (the file's existing "tolerates gmail adding columns" contract — no positional assumption). |

## Source / decision-source vocabulary (existing field, both sides) — extended values

Building on spec 002's `"rule:<name>"` / `"ai:<provider>"`:

| Value shape | Meaning | Produced when |
|---|---|---|
| `rule:<name>` | unchanged from spec 002 | A row with **no** current comment matched a rule (Decision 3, research.md — a present comment always bypasses rule matching, so this value never co-occurs with a non-empty `UserComment`/`Comment` on the row that produced it). |
| `ai:<provider>` | unchanged from spec 002 | AI-decided, comment-free. |
| `ai:<provider>+comment` | **new** (Story 3) | AI-decided, and the row's comment was included in that call (`CommentConsidered`/`entry.Comment` non-empty after the call). |
| `suggested:<original-source>` | **new** (Story 5) | The outcome was copied from an approved retroactive-suggestion candidate (Decision 10, research.md), not produced by a fresh AI call or rule match for this row. `<original-source>` is the *correcting* row's own Source value (e.g. `suggested:ai:deepseek+comment`), preserving traceability to how the correction that seeded the suggestion was itself produced. |

## Suggested Correction (new, ephemeral — Story 5)

Not persisted anywhere; exists only in memory for the duration of one
interactive `--suggest-similar` session.

| Field | Type | Notes |
|---|---|---|
| `CandidateID` | string | The already-decided row/entry's MessageID. |
| `CurrentOutcome` | struct (Category/SubCategory/Labels, or EventID) | What the candidate currently holds. |
| `ProposedOutcome` | struct (same shape) | Copied from the just-corrected row's new outcome. |
| `Reason` | string | Why this candidate was surfaced — `"same merchant"`, `"same rule: <name>"`, or both, for display only (not persisted). |

Approving a candidate turns it into a real, persisted outcome update (Source
`suggested:<...>`, Decision 10); skipping discards it — nothing is written
for a skipped candidate (FR-016, SC-006).

## Rule capture write target (existing file, `data/config/expense-rules.yaml`) — no schema change

Story 6 writes a new `ExpenseRule` entry (spec 002's existing shape — `name`,
`description`, `enabled`, `applies_to`, `match`, `outcome`) built from the
approved correction:

- `name`: derived, kebab-case, from the merchant + a short outcome hint (e.g.
  `hungerbox-birthday-not-groceries`), guaranteed unique against existing
  rule names (a numeric suffix is appended on collision).
- `description`: auto-generated one-liner naming the originating comment/
  correction, e.g. `"Captured from a comment-driven correction on <date>."`
- `applies_to`: `[categorize]` when captured from `gmail-categorize`,
  `[event]` when captured from `expenses-update-event` (never both — a
  capture only ever comes from one side's correction).
- `match.merchant_contains`: `[<merchant>]`, the same merchant the
  correction/candidate shared (Decision 8, research.md).
- `outcome`: the corrected Category/SubCategory/Labels (gmail side) or
  `event_relevance` is **not** settable this way — Story 6's spec text and
  acceptance scenario are framed around category corrections; an
  event-side capture instead writes `event_relevance: routine` only when the
  correction being captured was itself "no event" (there is no "capture as a
  specific recurring event" concept — the rules engine has no per-event
  outcome field, only `routine`/not-routine).

No existing rule is ever edited or reordered by this feature — captures are
always appended (new entries) at the end of the `rules:` list, preserving
existing precedence (spec 002's data-model.md: "order in the file is
precedence").

## Validation rules summary (this feature, in addition to spec 002's)

1. `UserComment`/`CommentConsidered` missing from a legacy CSV → treated as
   `""`, same padding behaviour every prior additive column already has.
2. `AssignmentEntry.Comment` missing/absent on a legacy `state.json` entry →
   zero-value `""` on JSON decode, no migration step (same as `Source`
   before it).
3. A comment that is empty after trimming whitespace is treated as no
   comment everywhere — never sent to the AI, never triggers reclassification,
   never bypasses a rule (spec Edge Cases section, FR-005).
4. Before any Story 6 write to `data/config/expense-rules.yaml`, `git status
   --porcelain -- <path>` (run with `Dir` set to the resolved workspace root,
   research.md Decision 11) must return empty output; a non-empty result
   aborts the capture with a message to the user and makes no edit (FR-020).
5. A captured rule name colliding with an existing rule's `name` gets a
   numeric suffix (`-2`, `-3`, ...) rather than overwriting — captures never
   edit an existing rule.
