# Expenses pack — RUNBOOK

Cluster the transactions extracted by the **gmail** pack
(`data/gmail/transactions.csv`) into ad-hoc **events** — a trip, a festival, a
house move — using DeepSeek. Unlike gmail's `categorize` (a fixed taxonomy,
ADR 0010), the set of events isn't known in advance: it's discovered from the
transactions and grows over time, kept in a versioned registry
(`config/events.json`) so a transaction seen next month is recognised as
belonging to an event created by an earlier run instead of spawning a
duplicate.

See design & rationale in `docs/adr/0011-expenses-pack.md`.

---

## TL;DR

```bash
cd packs/expenses

# 1. See exactly what WOULD be assigned/created — no writes:
make expenses-update-event-dry     # or: go run . update-event --dry-run

# 2. Once configured (DEEPSEEK_API_KEY), run for real and enrich CSV:
make expense-eventify              # or: go run . update-event --write-csv

# 3. Preview enrichment without writing:
make expense-eventify-dry          # or: go run . update-event --write-csv --dry-run

# Scheduler path (env-injected, per ADR 0007):
../../auto run expenses-update-event   # or add --write-csv to the command
```

Run `make help` for every target (build, test, config, events, …).

---

## One-time setup

```bash
./auto config init expenses          # scaffold config/expenses/config.yaml
$EDITOR config/expenses/config.yaml  # set DEEPSEEK_API_KEY
./auto config expenses               # verify what's set vs missing
```

`config.sample.yaml` documents every knob. Real values live in
`config/expenses/` (git-ignored) and are injected into the job at runtime.

---

## Running

### Dry run (always start here)

```bash
make dry-run
# or scope it:
go run . update-event --dry-run --limit 20
```

Prints, per transaction: a match to an existing event with its confidence, a
proposed new event (grouped across the batch), or "no event" — with nothing
written to `config/events.json` or `state.json`.

### Real run

```bash
make update-event
# useful bounds:
go run . update-event --limit 20            # process at most 20 unassigned rows
go run . update-event --threshold 0.7        # require higher confidence before reusing an event
go run . update-event --batch-size 50        # split a large backlog into several API calls
```

### Scheduler path

```bash
../../auto run expenses-update-event    # or: make auto-update-event
```

No schedule is enabled by default (`jobs/expenses-update-event/manifest.yaml`
has `schedule.enabled: false`) — run manually after `gmail-extract`, or flip it
on once you're happy with the assignments it produces.

---

## Flags (`go run . update-event --help`)

| Flag | Default | Meaning |
|------|---------|---------|
| `--csv PATH` | `../gmail/transactions.csv` | source CSV (read for matching; enriched if `--write-csv`) |
| `--events PATH` | `config/events.json` | event registry, versioned |
| `--state PATH` | `state.json` | assignment ledger, local |
| `--ai-provider NAME` | `deepseek` | AI provider |
| `--ai-model NAME` | — | override model (else `DEEPSEEK_MODEL` or built-in default) |
| `--threshold N` | `0.6` | confidence cutoff to accept a match to an existing event |
| `--batch-size N` | `0` (one call for all) | transactions per API call |
| `--limit N` | `0` (no cap) | stop after N unassigned rows |
| `--dry-run` | off | report only; nothing written |
| `--write-csv` | off | enrich transactions.csv with `EventID` and `EventDescription` columns |
| `--rules-file PATH` | `$AUTO_DATA_DIR/config/expense-rules.yaml` (or `../../data/config/expense-rules.yaml` outside `auto`) | shared expense classification rules, evaluated before the AI matcher (spec 002, ADR 0016) |

## Environment (from `config/expenses/config.yaml`)

| Var | Default | Notes |
|-----|---------|-------|
| `DEEPSEEK_API_KEY` | — | **required** (including for `--dry-run`) |
| `DEEPSEEK_MODEL` | `deepseek-v4-flash` | override |
| `AI_PROVIDER` | `deepseek` | only DeepSeek is implemented today |

---

## Expense classification rules (spec 002, ADR 0016)

Before any transaction reaches the AI matcher, `update-event` checks it
against the shared, human-authored rules file at `--rules-file`
(`data/config/expense-rules.yaml` at the workspace root — shared with the
gmail pack's `categorize` command, see `docs/adr/0016-expense-rules-engine.md`).
A rule scoped to `event` whose outcome is `event_relevance: routine` marks
the transaction as intentionally not event-worthy (the same "no event"
representation the AI path already uses — an empty `EventID` in
`state.json`) **without calling the AI matcher for that row at all**. Rows
with no matching rule go through AI matching exactly as before.

This is how routine, recurring transactions (a daily workplace lunch, a
regular commute) stop being repeatedly risked as spurious new-event
proposals: once a rule says "this is routine," it's routine on every future
run too, deterministically, with no AI cost. An empty or absent rules file
is a no-op — every row goes to the AI exactly as it did before this feature.

Every assignment (rule- or AI-decided) now also gets a `Source` field in
`state.json` (`rule:<name>` or `ai:<provider>`, `"manual"` for bulk-assign),
so any transaction's event assignment is auditable after the fact. The
`done:` summary line reports a `rule-decided` count alongside the existing
counters.

See `internal/event/rules.go` for the rule schema
(`ExpenseRule`/`MatchCondition`/`Outcome`) and
`specs/002-expense-rules-engine/` in the parent workspace repo for the full
spec/plan/contracts.

## How matching works

- Every run sends the **full** current event registry plus every
  not-yet-assigned transaction to DeepSeek in one call (`--batch-size` to
  split very large backlogs).
- For each transaction the model either matches an existing event id with a
  confidence score, or proposes a new event (name, description, keywords).
- A match is accepted only if the id is a **known** event and confidence is at
  or above `--threshold` (default `0.6`); otherwise the transaction is treated
  as unmatched.
- Unmatched rows that propose a new event are **grouped by proposed name
  within the batch** before a registry entry is created — several
  transactions in the same run that plausibly belong to the same real-world
  event become ONE event, not one each.
- Rows the model deliberately leaves without an event (an ordinary routine
  purchase, not part of any event) are still marked in the ledger so they
  aren't re-sent to the model on every future run.

## CSV enrichment (--write-csv)

By default, `state.json` keeps the assignment ledger separate from
`transactions.csv`. Use `--write-csv` to enrich the CSV with two new columns:
- `EventID` — the matched/created event's id
- `EventDescription` — the event's description text (from `config/events.json`)

This is useful for:
- downstream analysis or export (spreadsheet tools, dashboards)
- seeing event context inline with transaction data
- single-file workflows that don't split CSV and state ledger

Example:
```bash
make expense-eventify                 # match events AND write CSV columns
make expense-eventify-dry             # preview what would be written
go run . update-event --write-csv --limit 50   # enrich first 50 unassigned rows only
```

The CSV is only written after successful matching (registry and state both
saved). Combined with `--dry-run`, `--write-csv` previews the enrichment without
touching the file.

## Manual event workflow (bulk-assign + fill-similar)

Use this when you want to manually specify that certain transactions belong to
an event, and let AI find other similar transactions automatically:

### 1. Create assignment CSV

Create a CSV file with two columns: `MessageID` and `EventID`. MessageID must
exist in transactions.csv, and EventID must already exist in config/events.json.

Example (`my-assignments.csv`):
```
MessageID,EventID
msg-2024-05-15-001,goa-trip-2024
msg-2024-05-16-002,goa-trip-2024
msg-2024-06-02-001,diwali-shopping
```

### 2. Import the assignments

```bash
make expense-bulk-assign ASSIGNMENTS=my-assignments.csv    # real run
make expense-bulk-assign-dry ASSIGNMENTS=my-assignments.csv # preview
```

Manual assignments override any existing auto-matches; confidence is set to 1.0
(max). The command validates every MessageID and EventID before writing.

### 3. Find similar transactions

Once manual assignments are imported, ask AI to find unassigned transactions
that belong to the same events:

```bash
make expense-fill-similar        # real run: find and assign similar txns
make expense-fill-similar-dry    # preview matches without writing
```

The model sees 1–3 example transactions from each manually-assigned event and
searches the unassigned batch for matches. Matches above `--threshold`
(default 0.6) are assigned with their confidence score. Existing auto-matches
are NOT overwritten (fill-similar only assigns unassigned rows).

### Full workflow

```bash
# 1. Preview the auto-matching on fresh data
make expenses-update-event-dry --limit 50

# 2. Create manual_fixes.csv with known event assignments
# (see example above)

# 3. Import and fill manually
make expense-bulk-assign-dry ASSIGNMENTS=manual_fixes.csv
make expense-bulk-assign ASSIGNMENTS=manual_fixes.csv

# 4. Have AI find similar ones automatically
make expense-fill-similar-dry
make expense-fill-similar

# 5. Optionally enrich the CSV with event info
make expense-eventify
```

## Idempotency & re-runs

`state.json` records every assigned `MessageID` (event id + confidence),
including rows explicitly left without an event. Re-running only considers
rows not yet in the ledger, so it's safe to run repeatedly and incrementally
as new transactions arrive. To force a full re-pass, delete `state.json` —
this can re-propose new events for transactions already clustered elsewhere,
so prefer scoping with `--limit`/a filtered CSV instead where possible.

## Files

| Path | Committed? | What |
|------|-----------|------|
| `main.go`, `internal/**` | yes | the app (pure stdlib Go) |
| `config.sample.yaml` | yes | config contract (declarations only) |
| `config/events.json` | yes | event registry — versioned so matching stays consistent across machines |
| `jobs/expenses-update-event/manifest.yaml` | yes | job definition |
| `config/expenses/config.yaml` | **no** (workspace) | real `DEEPSEEK_API_KEY` etc. |
| `state.json` | **no** (git-ignored) | assignment ledger (produced data, ADR 0005) |

## Troubleshooting

| Symptom | Cause / fix |
|---------|-------------|
| `DEEPSEEK_API_KEY not set` | set it in `config/expenses/config.yaml` (needed even for `--dry-run`). |
| Same real-world event keeps getting a "-2", "-3" suffix id | the model proposed a slightly different `new_event_name` than before, so it slugified differently — improve the event's `keywords`/`description` in `config/events.json` by hand, or lower `--threshold` slightly. |
| A transaction never gets an event | the model returned neither a matching id nor a new-event proposal for it (deliberately no event) — it's marked in `state.json` with an empty event id and won't be retried. |
| `no assignment returned for <id>` | the model's response omitted that id — it is **not** marked, so the next run retries it. |
