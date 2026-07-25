# Quickstart: User Comments Inform Transaction Classification

Validation scenarios for this feature. Each maps to an acceptance scenario in
`spec.md`. Run from the repository root unless noted. Use scratch/synthetic
CSV and state files — no real financial data needed.

## Prerequisites

- `packs/gmail` and `packs/expenses` build (`cd packs/gmail && go build ./...`,
  `cd packs/expenses && go build ./...`).
- A scratch `transactions.csv` with the current header (including `Source`
  from spec 002) plus a few rows, some categorised, some not.
- `DEEPSEEK_API_KEY` set for scenarios that reach the AI (comment-influenced
  classification). Rule-only and dirty-tracking scenarios don't need it.

## Scenario 0 — Zero comments ever written: no regression (SC-003)

1. Run `categorize --dry-run` and `update-event --dry-run` against a CSV/state
   with no `UserComment` values anywhere.
2. **Expected**: identical output to before this feature — no `+comment`
   suffixes, no `CommentConsidered`/`Comment` writes, same rows selected as
   spec 002 alone would select.

## Scenario 1 — A comment steers classification (User Story 1)

1. Add an uncategorised row with an ambiguous merchant; set its `UserComment`
   to context that should change the outcome (e.g. "birthday dinner for
   Chinju, not routine groceries").
2. Run `categorize` (not dry-run, with a provider configured).
3. **Expected**: the row's Category/SubCategory reflect the comment's
   context; `Source` reads `ai:<provider>+comment`; `CommentConsidered` now
   equals the comment; re-running `categorize` again makes no further change
   to this row (it's no longer eligible).

## Scenario 2 — A comment steers event matching (User Story 2)

1. Add an unassigned row with `UserComment` naming a known event
   (`config/events.json` already has a matching entry).
2. Run `update-event` (not dry-run).
3. **Expected**: the row is matched to the named event; `state.json`'s entry
   for it has `Source: "ai:<provider>+comment"` and `Comment` equal to the
   row's `UserComment`.

## Scenario 3 — Comment-free rows behave exactly as before (User Story 1/2, FR-005)

1. Run `categorize` and `update-event` against rows with no `UserComment`.
2. **Expected**: byte-identical outcome to spec 002 alone — `Source` has no
   `+comment` suffix anywhere these ran.

## Scenario 4 — Decision-source visibility (User Story 3)

1. After Scenarios 1–3, open `transactions.csv` and `state.json`.
2. **Expected**: rule-decided rows still read `rule:<name>`;
   comment-influenced rows read `ai:<provider>+comment`; comment-free
   AI-decided rows read `ai:<provider>` — all three are distinguishable at a
   glance (SC-004).

## Scenario 5 — Adding a comment re-opens an AI-decided row (User Story 4, scenario 1)

1. Classify a row with no comment (gets `ai:<provider>`).
2. Add a `UserComment` to that same row.
3. Re-run `categorize`.
4. **Expected**: the row is re-classified with the comment as AI input; the
   outcome may change; `Source` becomes `ai:<provider>+comment`.

## Scenario 6 — A comment overrides a rule (User Story 4, scenario 2, FR-012)

1. With a rule present that would deterministically decide a row (e.g.
   `hungerbox-workplace-food` from spec 002's contract), add that row
   uncategorised with `UserComment` set (e.g. "this one was a personal
   dinner, not office food").
2. Run `categorize`.
3. **Expected**: the rule is **not** applied — the row is routed to the AI
   with the comment attached instead; `Source` is `ai:<provider>+comment`,
   never `rule:hungerbox-workplace-food`.

## Scenario 7 — Editing a comment reclassifies again; removing one does not reprocess (User Story 4, scenarios 3–4)

1. Take a row already classified with a comment (Scenario 1's result).
2. Edit `UserComment` to different text; re-run `categorize`.
   **Expected**: re-classified again, `CommentConsidered` updated to the new
   text.
3. Now clear `UserComment` back to empty; re-run `categorize`.
   **Expected**: no change — the row is treated as any other already-decided
   row (not reprocessed), since it's not missing an outcome and its (empty)
   comment doesn't trigger the comment-changed branch.

## Scenario 8 — Retroactive suggestions, interactive + opt-in only (User Story 5)

1. Prepare several older rows from the same merchant, already decided with an
   outcome that a fresh comment-driven correction will disagree with (e.g.
   several old HungerBox rows categorised by a rule that's about to be
   overridden by a comment on one new HungerBox row).
2. Run interactively (real terminal): `categorize --suggest-similar`.
3. **Expected**: after the comment-driven row is corrected, each older
   HungerBox candidate is presented one at a time with its proposed new
   outcome; approving one updates it (`Source: suggested:<original-source>`)
   and moves to the next; declining leaves it untouched; a row is never
   changed without that specific approval (SC-006).
4. Re-run the same command **without** `--suggest-similar`.
   **Expected**: identical to Stories 1/2/4 alone — no suggestion prompts at
   all (FR-018).
5. Simulate a non-interactive run (e.g. pipe `/dev/null` into stdin, or run
   under something that redirects stdin) with `--suggest-similar` set.
   **Expected**: no suggestion flow triggers regardless of the flag (FR-017).

## Scenario 9 — Capture a correction as a rule (User Story 6)

1. Interactively repeat Scenario 1 or approve a Scenario 8 candidate.
2. When prompted "capture this as a rule?", answer `y`.
3. **Expected**: `data/config/expense-rules.yaml` gains one new rule entry
   matching the merchant and outcome just approved; `git log -1 --
   data/config/expense-rules.yaml` shows a new, descriptive commit; existing
   rules in the file are byte-for-byte unchanged aside from the append.
4. Repeat, but first leave an unrelated uncommitted edit in
   `data/config/expense-rules.yaml`.
   **Expected**: the capture aborts with a clear message; no edit is made;
   the pre-existing uncommitted change is untouched (FR-020).
5. Repeat Scenario 9 step 1–2 but answer anything other than `y`.
   **Expected**: `data/config/expense-rules.yaml` is completely unchanged
   (FR-022).
