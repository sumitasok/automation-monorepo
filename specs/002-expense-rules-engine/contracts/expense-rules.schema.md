# Contract: `data/config/expense-rules.yaml`

This is the shared definitions file both `gmail-categorize` and
`expenses-update-event` load. Both packs must parse it identically — this
document is the contract they're each independently implemented against (see
`plan.md` Decision 3: duplicated Go code, one shared file).

## Shape

```yaml
rules:
  - name: hungerbox-workplace-food
    description: >
      HungerBox is the office cafeteria vendor — every charge from them is
      workplace food, not a personal dining decision.
    enabled: true
    applies_to: [categorize, event]
    match:
      merchant_contains: ["hungerbox"]
    outcome:
      category: "Food & Drinks"
      subcategory: "Groceries"
      labels: ["Work"]
      event_relevance: routine

  - name: uber-weekday-afternoon-commute
    description: >
      Uber taken on a weekday afternoon/evening is the office-to-home
      commute (no GPS data available — this is a time-of-day heuristic,
      see spec.md Assumptions).
    enabled: true
    applies_to: [categorize, event]
    match:
      merchant_contains: ["uber"]
      day_of_week: [mon, tue, wed, thu, fri]
      time_between: ["13:00", "20:00"]
    outcome:
      category: "Transportation"
      subcategory: "Business trips"
      labels: ["Work"]
      event_relevance: routine
```

## Field contract

- `rules` (required, list): evaluated top-to-bottom. Order is precedence —
  see `data-model.md`.
- Per rule:
  - `name` (required, string, unique across the file): identifies the rule in
    `Source` columns/fields as `rule:<name>`.
  - `description` (optional, string): not parsed for meaning; documentation
    only.
  - `enabled` (optional, bool, default `true`).
  - `applies_to` (required, list, one or both of `categorize`/`event`).
  - `match` (required, object, at least one field set):
    - `merchant_contains` (list of string)
    - `keyword_contains` (list of string)
    - `day_of_week` (list of `mon`/`tue`/`wed`/`thu`/`fri`/`sat`/`sun`)
    - `time_between` (2-element list of `"HH:MM"` strings, `[start, end]`)
    - `amount_between` (2-element list of numbers, `[min, max]`)
  - `outcome` (required, object, at least one field set):
    - `category` (string, requires `subcategory`)
    - `subcategory` (string, requires `category`)
    - `labels` (list of string)
    - `event_relevance` (string, only defined value: `routine`)

## Loader behavior contract (both packs implement this identically)

1. File does not exist → return an empty rule set, no error (graceful
   degrade — matches `discover.LoadCategorizerRules`'s existing convention).
2. File exists but fails to parse (invalid YAML) → return an error; the job
   fails loudly rather than silently ignoring malformed rules.
3. A rule with an empty `match` object → load-time error (see
   `data-model.md` validation rule 2).
4. A rule's `outcome.category`/`outcome.subcategory` that don't resolve
   against the pack's loaded taxonomy is **not** a load-time error (the
   taxonomy is loaded separately, and `expenses` has no taxonomy at all) — it
   is caught at evaluation time per-row, logged, and treated as a non-match
   for that row (research.md Decision 5 / data-model.md validation rule 3).

## Evaluation contract (per transaction, per consuming job)

```
for each rule in rules (in file order):
    if not rule.enabled: continue
    if job's decision type not in rule.applies_to: continue
    if not all(condition holds for condition in rule.match): continue
    # first match wins — stop here
    outcome := rule.outcome relevant to this job
    if outcome invalid (fails taxonomy validation, gmail side only):
        log warning; continue to next rule as if this one hadn't matched
    apply outcome; record Source = "rule:<rule.name>"; do not call the AI for this row
    break
else:
    # no rule matched (or all matches were invalid outcomes)
    send row to the AI provider exactly as today; record Source = "ai:<provider>"
```
