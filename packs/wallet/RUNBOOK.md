# Wallet pack — RUNBOOK

Sync the transactions extracted by the **gmail** pack
(`data/gmail/transactions.csv`) into the **BudgetBakers Wallet** app via the
Wallet REST API. One record per transaction, processed day by day, deduped by
`MessageID+Amount` composite key (preventing duplicates even if emails repeat),
preserving actual purchase timestamps from bank alerts, and each tagged with the
label `source:automation-monorepo`.

See design & rationale in `docs/adr/0009-wallet-pack.md`.

---

## TL;DR

### Sync Wallet Records

```bash
# Via auto CLI (recommended):
./auto run wallet-sync

# Or directly:
cd packs/wallet && go run . sync
```

### Dedup Wallet Records (4-Phase Workflow)

```bash
cd packs/wallet

# Phase 1: Scan for duplicates (read-only)
go run . dedup scan
# Output: DUPLICATE: X groups | Y records to delete | Z to keep

# Phase 2: Review and collect decisions (interactive)
go run . dedup review --decisions-file decisions.jsonl

# Phase 3: Execute - saves deletion plan (read-only on records)
go run . dedup execute --decisions-file decisions.jsonl
# Output: dedup-results.jsonl with deletion IDs

# Phase 4: Delete from Wallet API, then finalize
# (manually delete from Wallet API using IDs in dedup-results.jsonl)
go run . dedup finalize --dedup-results records-dedup-results.jsonl
# Prompts: Confirm Wallet API deletions? (yes/no)
# Updates records.jsonl only after confirmation
```

---

## One-time setup

### 1. Get a Wallet REST API token

Requires a **Premium** Wallet plan.

1. Open the Wallet web app → **Settings → REST API**
   (`https://web.budgetbakers.com/settings/rest-api`).
2. Create a token and copy it.
3. Ensure the token has the **`records.create`** (write) scope — read-only tokens
   can list data but cannot create records. (The MCP connector is read-only by
   default; the REST token is separate.)

### 2. Map your CSV accounts to Wallet accounts

The CSV `Account` column is a bank last-4 / identifier (e.g. `3690`, `3176`,
`1008`, `0878`, `XXX383`, `XX6102`). These don't match Wallet account names, so
the mapping is explicit.

1. List your Wallet account UUIDs:

   ```bash
   curl -s -H "Authorization: Bearer $WALLET_API_TOKEN" \
     "https://rest.budgetbakers.com/wallet/accounts?limit=200" | \
     python3 -c 'import sys,json; [print(a["id"], a["currencyCode"], a["name"]) for a in json.load(sys.stdin)["accounts"]]'
   ```

2. Copy the starter map and fill in the UUIDs:

   ```bash
   cp packs/wallet/accounts.sample.json config/wallet/accounts.json
   $EDITOR config/wallet/accounts.json
   ```

   Each entry maps a CSV code to a Wallet account (and optional payment type):

   ```json
   {
     "3690":  { "accountId": "…uuid…", "paymentType": "debit_card" },
     "0878":  { "accountId": "…uuid…", "paymentType": "credit_card" },
     "_default": { "accountId": "", "paymentType": "debit_card" }
   }
   ```

   - `_default` catches empty/unmapped codes. **Leave its `accountId` empty to
     SKIP** those rows (recommended) rather than misfiling them.
   - Map only **manual** accounts — Wallet rejects records created on bank-synced
     accounts.

### 3. Provide the token (config injection, ADR 0007)

```bash
./auto config init wallet          # scaffold config/wallet/config.yaml
$EDITOR config/wallet/config.yaml  # set WALLET_API_TOKEN
./auto config wallet               # verify what's set vs missing
```

`config.sample.yaml` documents every knob. Real values live in `config/wallet/`
(git-ignored) and are injected into the job at runtime — nothing secret is
committed.

---

## Running

### Dry run (always start here)

Parses the CSV, applies the account map, and prints what it would create — **no
token required, no API calls** (but `accounts.json` is still required so rows map
onto the correct Wallet accounts):

```bash
make dry-run
# or scope it:
go run . sync --dry-run --since 2026-07-01
```

Each line shows `DRY <day> <signed amount> <paymentType> <merchant>`. The final
summary line reports counts: created / already-synced / unmapped / out-of-range /
failed / malformed.

### Real sync

```bash
make sync
# useful bounds:
go run . sync --since 2026-07-01          # only recent rows
go run . sync --limit 20                  # push at most 20 (safe first push)
go run . sync --until 2026-06-30          # only up to a date
```

Records are pushed in batches of ≤20 per day. Successes are written to
`state.json` immediately after each batch, so an interrupted run resumes cleanly.

### Scheduler path

`auto run` injects env + `accounts.json` and applies the manifest's timeout:

```bash
./auto run wallet-sync              # real sync
./auto run wallet-sync -- --dry-run # dry-run via auto (accounts.json still required)
```

Enable the daily schedule (07:30 IST, after gmail-extract) by setting
`schedule.enabled: true` in `jobs/wallet-sync/manifest.yaml`, then:

```bash
./auto schedule sync
```

### Detect duplicates

Check for potential duplicate records (same MessageID or MessageID+Amount appearing
multiple times):

```bash
# Direct command:
go run . detect-duplicates

# Via auto:
./auto run wallet-detect-duplicates

# Export as JSON for processing:
go run . detect-duplicates --format json
```

The tool reports:
- **CSV Duplicates**: same MessageID appearing multiple times in `transactions.csv`
- **State Duplicates**: evidence of same transaction synced multiple times
- **Cross-Check Issues**: MessageID with different amounts (data anomaly)

Example output:
```
=== Duplicate Detection Report ===
Total Duplicate Records: 0
Recommendations:
✓ No duplicates detected. Deduplication is working correctly.
```

---

## Flags (`go run . sync --help`)

| Flag | Default | Meaning |
|------|---------|---------|
| `--csv PATH` | `../gmail/transactions.csv` | source CSV |
| `--state PATH` | `state.json` | dedupe ledger |
| `--accounts PATH` | `$AUTO_PACK_CONFIG_DIR/accounts.json`, then `./accounts.json` | account map |
| `--dry-run` | off | report only; no token, no API calls |
| `--since YYYY-MM-DD` | — | only records on/after this date |
| `--until YYYY-MM-DD` | — | only records on/before this date |
| `--limit N` | `0` (no cap) | cap records pushed |

## Environment (from `config/wallet/config.yaml`)

| Var | Default | Notes |
|-----|---------|-------|
| `WALLET_API_TOKEN` | — | **required** for real runs |
| `WALLET_LABEL` | `source:automation-monorepo` | display name only, used in logs |
| `WALLET_LABEL_ID` | — | the label's Wallet UUID; **required** to actually attach a label — see [Labels](#labels-no-rest-endpoint) below |
| `WALLET_DEFAULT_PAYMENT_TYPE` | `debit_card` | used when an account rule has none |
| `WALLET_TIMEZONE` | `Asia/Kolkata` | interprets date-only `TxnDate` values |
| `WALLET_BASE_URL` | `https://rest.budgetbakers.com/wallet` | rarely changed |

---

## How records are built

- **amount** — thousands separators stripped; `Debit` → negative (expense),
  `Credit` → positive (income).
- **recordDate** — Prefers `EmailDate` (full timestamp with time) over `TxnDate`
  (often date-only). This preserves the actual time of purchase from bank alerts
  (e.g. 14:27:56) instead of using the sync schedule time. Date-only values are
  placed at local midnight in `WALLET_TIMEZONE`.
- **counterParty** — the CSV `Merchant`.
- **note** — the CSV `Info`/`Subject` plus `[gmail-csv <shortId>]` for tracing
  back to the source email.
- **labels** — `[WALLET_LABEL_ID]` if set, otherwise no label at all (see below).
- **category** — left unset (Wallet auto-assigns Unknown income/expense);
  recategorise in the app.

## Account matching: exact, then last-4/last-3-digit fuzzy match

Bank alert emails mask the same account inconsistently — real examples seen
in `transactions.csv`: `0878`, `X0878`, and `XXXXX860878` all refer to the
same card. `ResolveAccount` tries an exact key match in `accounts.json`
first; if that misses, it compares trailing digits (last 4, then last 3)
against every mapped code and uses the one it finds — **only when exactly
one** mapped code shares that suffix. A suffix shared by two or more mapped
codes, or matching none, falls through to `_default` (skip) unchanged — it
never guesses when ambiguous. Nothing to configure; this applies automatically
on top of your existing `accounts.json`.

## Labels: no REST endpoint

The Wallet REST API does **not** expose `/labels` (confirmed 2026-07-16: `GET
/labels` returns 404; the official quick-reference at
`https://rest.budgetbakers.com/wallet/reference` lists only
records/accounts/budgets/categories as API-managed, and its "Deleting
Entities" table explicitly marks labels "Not supported via API"). This
pack originally assumed labels could be looked up/created through the same
API (ADR 0009 decision 4) — that assumption was wrong.

To attach a label to synced records:

1. Create the label once, manually, in the Wallet app (any name, e.g.
   `source:automation-monorepo`).
2. Find its UUID. There's no public way to list labels via the REST API —
   easiest is asking an assistant with the Wallet MCP connector enabled to
   run `get_labels`, or inspecting the Wallet web app's network requests.
3. Set `WALLET_LABEL_ID` in `config/wallet/config.yaml` to that UUID.

If `WALLET_LABEL_ID` is left empty, `sync` still runs — records are just
created without a label — rather than failing the whole run (the old
behavior called the nonexistent endpoint and aborted on its 404).

## Idempotency & re-runs

`state.json` records every pushed transaction using a composite key: `MessageID + Amount`.
This ensures idempotency: even if the same email appears multiple times in the CSV
(or if you have duplicate emails in Gmail), each unique transaction is only synced
once.

Re-running skips anything already synced, so the daily job is safe to run repeatedly.
To force a full re-sync, delete `state.json` (this will create duplicates unless you
also remove the previously-created records in Wallet — filter them by the
`source:automation-monorepo` label).

## Filtering / undoing what this pack wrote

Everything is tagged. In the Wallet app, filter records by the
`source:automation-monorepo` label to see (or bulk-delete) only machine-imported
records.

---

## Deduplication Workflow

Identify and remove duplicate wallet records using a 4-phase workflow that's safe and reversible.

### Overview

```
Phase 1: Scan      → Detect duplicates (records.jsonl, read-only)
Phase 2: Review    → Collect user decisions (decisions.jsonl, interactive)
Phase 3: Execute   → Save deletion plan (dedup-results.jsonl, read-only)
Phase 4: Finalize  → Delete from Wallet API → update records.jsonl
```

### File Formats (JSONL - one JSON object per line)

- `records.jsonl` — wallet transaction records (streaming-friendly)
- `decisions.jsonl` — dedup review decisions with metadata
- `dedup-results.jsonl` — deletion plan from execute phase

### Complete Example

```bash
cd packs/wallet

# Step 1: Scan for duplicates (shows summary)
go run . dedup scan
# Output: DUPLICATE: 1 groups | 6328 records to delete | 1 to keep

# Step 2: Review and collect decisions (interactive)
go run . dedup review --decisions-file decisions.jsonl
# For each group: choose keep-first, custom, or skip

# Step 3: Execute - generates deletion plan (read-only)
go run . dedup execute --decisions-file decisions.jsonl
# Output: DEDUPLICATED: 6328 to delete | 1 to keep
#         Results saved: records-dedup-results.jsonl

# Step 3b: Delete from Wallet API
# (Manual step: delete these 6328 records from Wallet using the IDs in dedup-results.jsonl)
# You can use: wallet sync --dedup-results records-dedup-results.jsonl

# Step 4: Finalize (only after Wallet API confirms deletions)
go run . dedup finalize --dedup-results records-dedup-results.jsonl
# Prompt: Confirm: Have you deleted these records from Wallet API? (yes/no)
# Updates records.jsonl ONLY AFTER confirmation
# Creates backup: records.jsonl.backup.{timestamp}
```

### Flags

#### Scan Phase
- `--format json|text` — output format (default: text, summary only)
- `--min-confidence 0-1` — only show matches above threshold (default: 0.5)

#### Review Phase
- `--decisions-file PATH` — where to save decisions (default: .dedup-decisions-{timestamp}.jsonl)
- `--dry-run` — show decisions without saving

#### Execute Phase
- `--decisions-file PATH` — file from review phase
- `--dry-run` — preview deletions without generating results

#### Finalize Phase
- `--dedup-results PATH` — file from execute phase
- `--state-file PATH` — audit trail location

### Safety Features

✅ **Phase 3 never touches records.jsonl** — only saves dedup-results.jsonl  
✅ **Phase 4 confirms Wallet API deletions** — waits for user confirmation  
✅ **Automatic backup** — records.jsonl.backup.{timestamp} created before write  
✅ **Audit trail** — all operations logged to state.json  
✅ **Atomic writes** — temp file + rename prevents corruption  

### If Something Goes Wrong

- **After Phase 3, before Phase 4**: Restart from Phase 1 (decisions.jsonl is scratch)
- **After Phase 4 fails**: Both records.jsonl and records.jsonl.backup.{timestamp} exist
  - Manually restore: `cp records.jsonl.backup.{timestamp} records.jsonl`
  - Re-run from Phase 1 with fresh decisions

---

## Troubleshooting

| Symptom | Cause / fix |
|---------|-------------|
| `WALLET_API_TOKEN is not set` | set it in `config/wallet/config.yaml` (or export it). Not needed for `--dry-run`. |
| `unauthorized (401)` | bad/expired token, or missing `records.create` scope. |
| `wallet sync in progress (409)` | initial data sync running; retry in a few minutes. |
| `rate limited (429)` | 300 req/hour cap; wait and re-run (state means no duplicates). |
| `skip (unmapped account "X")` | add `X` to `accounts.json` (exact or fuzzy last-4/last-3 digit match — see above), or rely on `_default`. |
| `POST /records: HTTP 404: <body>` | as of this fix, errors now include the raw response body — read it, it names the actual reason (bad `accountId`, validation error, etc.). Common causes: a mapped `accountId` in `accounts.json` no longer exists or was mistyped; double-check it against `GET /accounts`. If the body itself says "not found" for the *route* rather than a field, the Wallet API's path structure may have changed since ADR 0009 was written — check `https://rest.budgetbakers.com/wallet/openapi/ui` for the current spec. |
| Old rows show `failed` | Wallet rejects `recordDate` >10 years in the past; bound with `--since`. |
| `ensure label ...: GET /labels: HTTP 404` (old versions: fatal) | expected — the API has no `/labels` endpoint. Now non-fatal: sync continues without a label. Set `WALLET_LABEL_ID` (see [Labels](#labels-no-rest-endpoint) above) to attach one. |
| Records rejected on an account | that account is bank-synced; map only manual accounts. |

## Files

| Path | Committed? | What |
|------|-----------|------|
| `main.go`, `internal/**` | yes | the app (pure stdlib Go) |
| `config.sample.yaml` | yes | config contract (declarations only) |
| `accounts.sample.json` | yes | starter account map (no real UUIDs) |
| `jobs/wallet-sync/manifest.yaml` | yes | job definition |
| `config/wallet/config.yaml` | **no** (workspace) | real token + env |
| `config/wallet/accounts.json` | **no** (workspace) | real account map |
| `state.json` | **no** (git-ignored) | dedupe ledger (produced data, ADR 0005) |
