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
make dry-run                       # or: go run . update-event --dry-run

# 2. Once configured (DEEPSEEK_API_KEY), run for real:
make update-event                  # or: go run . update-event

# Scheduler path (env-injected, per ADR 0007):
../../auto run expenses-update-event   # or: make auto-update-event
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
| `--csv PATH` | `../gmail/transactions.csv` | source CSV, read-only |
| `--events PATH` | `config/events.json` | event registry, versioned |
| `--state PATH` | `state.json` | assignment ledger, local |
| `--ai-provider NAME` | `deepseek` | AI provider |
| `--ai-model NAME` | — | override model (else `DEEPSEEK_MODEL` or built-in default) |
| `--threshold N` | `0.6` | confidence cutoff to accept a match to an existing event |
| `--batch-size N` | `0` (one call for all) | transactions per API call |
| `--limit N` | `0` (no cap) | stop after N unassigned rows |
| `--dry-run` | off | report only; nothing written |

## Environment (from `config/expenses/config.yaml`)

| Var | Default | Notes |
|-----|---------|-------|
| `DEEPSEEK_API_KEY` | — | **required** (including for `--dry-run`) |
| `DEEPSEEK_MODEL` | `deepseek-v4-flash` | override |
| `AI_PROVIDER` | `deepseek` | only DeepSeek is implemented today |

---

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
