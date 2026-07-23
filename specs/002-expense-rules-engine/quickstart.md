# Quickstart: Expense Classification Rules Engine

Validation scenarios for this feature. Each maps to an acceptance scenario in
`spec.md`. Run from the repository root unless noted.

## Prerequisites

- `packs/gmail` and `packs/expenses` build (`cd packs/gmail && go build ./...`,
  `cd packs/expenses && go build ./...`).
- `data/gmail/transactions.csv` exists with at least a few rows missing
  Category/SubCategory/Labels (or use a scratch copy — do not need real
  financial data for these scenarios; synthetic rows are fine).
- No `DEEPSEEK_API_KEY` is required for the rule-matched scenarios below,
  since a confirmed rule match never calls the AI provider.

## Scenario 0 — No rules defined: zero regression (SC-005)

1. Ensure `data/config/expense-rules.yaml` does not exist (or is empty).
2. Run `go run . categorize --dry-run` in `packs/gmail` against a CSV with
   uncategorised rows.
3. **Expected**: identical output to before this feature existed — every row
   goes to the AI provider, no `[rule:...]` tags appear anywhere.

## Scenario 1 — Merchant rule (User Story 1)

1. Create `data/config/expense-rules.yaml` with the `hungerbox-workplace-food`
   rule from `contracts/expense-rules.schema.md`.
2. Add a synthetic uncategorised row to a scratch `transactions.csv` with
   `Merchant` containing "HungerBox".
3. Run `go run . categorize --rules-file ../../data/config/expense-rules.yaml --dry-run`.
4. **Expected**: the HungerBox row is printed with the rule's exact
   category/subcategory/labels and a `[rule:hungerbox-workplace-food]` tag —
   no AI call made for that row (confirm via `--ai-provider` left unset /
   no `DEEPSEEK_API_KEY` and the run still succeeding for that row).
5. Re-run the same command again. **Expected**: identical output byte-for-byte
   (determinism, SC-002).

## Scenario 2 — Time+pattern rule (User Story 2)

1. Add the `uber-weekday-afternoon-commute` rule.
2. Add two synthetic Uber rows: one `TxnDate` on a weekday at 14:00, one on a
   Saturday (or at 09:00 on a weekday).
3. Run `categorize --dry-run` again.
4. **Expected**: only the weekday-afternoon row gets the rule's work-travel
   outcome; the other row falls through and (if an AI provider/key is
   configured) is sent to the AI, or is left unassigned in a dry-run without
   a key configured — either way, **not** matched by the rule.

## Scenario 3 — Rules inform event-matching (User Story 3)

1. With the same `expense-rules.yaml`, add an uncategorised HungerBox row to
   `transactions.csv` and run `gmail-categorize` first (so it has a Category)
   or run `update-event` directly — event-relevance rules don't require prior
   categorization.
2. In `packs/expenses`, run
   `go run . update-event --rules-file ../../data/config/expense-rules.yaml --dry-run`.
3. **Expected**: the HungerBox row prints as `no event` (or is skipped from
   any new-event proposal) tagged `[rule:hungerbox-workplace-food]` — it is
   never included in a batch sent to the AI matcher.

## Scenario 4 — Decision-source auditability (User Story 4)

1. Run both jobs for real (not `--dry-run`) once with the rules file present
   and at least one row with no matching rule (so it goes to the AI, assuming
   a provider/key is configured — otherwise skip the AI-sourced half of this
   scenario and only confirm the rule-sourced half).
2. Open `data/gmail/transactions.csv` and confirm the new `Source` column
   reads `rule:<name>` for rule-matched rows.
3. Open `packs/expenses/state.json` and confirm the new `Source` field on the
   relevant `AssignmentEntry` reads `rule:<name>` (or `ai:<provider>` for
   AI-decided rows).

## Scenario 5 — Invalid rule outcome is rejected, not silently applied

1. Add a rule whose `outcome.category`/`outcome.subcategory` do not exist in
   `packs/gmail/config/taxonomy.yaml` (a typo).
2. Run `categorize --dry-run` against a row that matches the rule's `match`
   conditions.
3. **Expected**: a logged warning naming the rule and the unresolvable
   category/subcategory; the row is **not** written with the invalid value —
   it falls through to the AI path (or is left unassigned if no AI is
   configured), exactly as an unresolvable AI assignment already behaves
   today.
