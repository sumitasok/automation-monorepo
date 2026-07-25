# ADR 0017 — User comments inform transaction classification (cross-pack)

**Status:** accepted — 2026-07-25

## Context

`gmail-categorize` (ADR 0010) and `expenses-update-event` (ADR 0011) both
classify a transaction from its bare bank-alert fields (merchant, amount,
info, subject) plus, since ADR 0016, any deterministic rule match. Neither
has any way to take Sumit's own knowledge about a *specific* transaction into
account — "this was Chinju's birthday dinner, not routine groceries," "this
is a reimbursement, not a real spend," "Goa trip - day 2 dinner." That
context only exists in Sumit's head unless he writes it down somewhere the
pipeline reads. See `specs/003-transaction-user-comments/spec.md` for the
full feature spec.

## Decision

1. **A new, hand-edited `UserComment` column on `transactions.csv`,**
   distinct from the existing `Note` column (ADR 0013, populated only by the
   forwarded-email mechanism). The file itself is the interface — no new CLI
   command, no new UI. Both `Note` and `UserComment` may legitimately hold
   different content on the same row.

2. **A comment is AI input, never an instruction.** Both packs' `Item` types
   gain an `omitempty` `Comment` field; both system prompts gain one sentence
   framing it as descriptive context only — the taxonomy (gmail) / event
   registry (expenses) remain the sole vocabulary the model may choose from,
   unchanged by what a comment says. A comment-free row's AI payload is
   byte-identical to before this feature.

3. **A comment re-opens an already-decided row, and outranks a matching
   rule, for as long as it's present.** A new system-written snapshot column
   (`CommentConsidered` on gmail's `transactions.csv`; `Comment` on
   expenses' `state.json` `AssignmentEntry`) records what comment (if any)
   was last used to produce the row's current outcome. A row re-enters
   classification only when its live comment differs from that snapshot —
   added, edited, or (transiently) present after being empty. While a
   non-empty comment is in that eligible state, the row skips
   `expense-rules.yaml` matching entirely and goes straight to the AI; once
   classified, the snapshot catches up and the row drops out of
   consideration again, exactly like every other idempotent selection in
   this codebase (`NeedsCategory()` / `state.Has()`, both extended rather
   than replaced).

4. **`Source` gains a `+comment` suffix.** `"ai:<provider>+comment"` marks a
   comment-influenced outcome, keeping ADR 0016's existing `rule:<name>` /
   `ai:<provider>` vocabulary intact and still prefix-matchable as `"ai:"`.

5. **Two further, interactive-only capabilities, both requiring a real TTY
   on stdin (not just a config flag) so neither can ever fire on a
   scheduled/cron run:**
   - **Retroactive suggestions** (`--suggest-similar`, opt-in): after a
     comment-driven correction to an already-decided row, other rows sharing
     its merchant and/or its prior rule are presented one at a time for
     explicit approve/skip — never applied without that approval. Approved
     rows get `Source = "suggested:<original-source>"`.
   - **Rule capture**: right after an approved correction, the user is
     offered to turn it into a new `expense-rules.yaml` entry, appended
     (never rewriting existing content) and committed to git automatically —
     refusing to proceed if the file already has uncommitted changes.

6. **TTY detection is stdlib-only** (`os.Stdin.Stat()`'s `os.ModeCharDevice`
   bit), duplicated per pack rather than adding a dependency — `auto run`
   (interactive) and a cron-installed `auto run <job-id>` invoke the same
   code path with no other distinguishing signal (`framework/tools/auto` was
   read to confirm this), so this is the correct, dependency-free way to
   draw that line.

## Consequences

- Zero comments ever written reproduces today's (post-ADR-0016) behavior
  exactly — every new code path is gated on a non-empty, trimmed comment.
- Comments are never lost: they live in a column/field neither job ever
  overwrites, following the same additive-schema discipline as every prior
  `transactions.csv`/`state.json` growth (ADR 0010, ADR 0013, ADR 0016).
- Rule capture is the first place either pack shells out to `git` — scoped
  explicitly to the workspace root (derived from the already-resolved
  `--rules-file` path), never either pack's own independently-versioned
  repository.
- Complements ADR 0016 (the rules file and `Source` vocabulary this feature
  extends rather than replaces) and ADR 0011 decision 2/decision 4's
  duplication-over-shared-module precedent, applied again here to
  `suggest.go`/`rulecapture.go`/`interactive.go` in both packs.
