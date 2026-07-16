# Wallet pack — RUNBOOK

Sync the transactions extracted by the **gmail** pack
(`data/gmail/transactions.csv`) into the **BudgetBakers Wallet** app via the
Wallet REST API. One record per transaction, processed day by day, deduped by
`MessageID` (so re-runs never duplicate), each tagged with the label
`source:automation-monorepo`.

See design & rationale in `docs/adr/0009-wallet-pack.md`.

---

## TL;DR

```bash
cd packs/wallet

# 1. See exactly what WOULD be synced — no token, no API calls (but accounts.json is still required):
make dry-run                       # or: go run . sync --dry-run

# 2. Once configured (token + accounts.json), push for real:
make sync                          # or: go run . sync

# Scheduler path (env-injected, per ADR 0007):
../../auto run wallet-sync         # or: make auto-sync
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
../../auto run wallet-sync
```

Enable the daily schedule (07:30 IST, after gmail-extract) by setting
`schedule.enabled: true` in `jobs/wallet-sync/manifest.yaml`, then:

```bash
../../auto schedule sync
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
| `WALLET_LABEL` | `source:automation-monorepo` | tag on every record; created if missing |
| `WALLET_DEFAULT_PAYMENT_TYPE` | `debit_card` | used when an account rule has none |
| `WALLET_TIMEZONE` | `Asia/Kolkata` | interprets date-only `TxnDate` values |
| `WALLET_BASE_URL` | `https://rest.budgetbakers.com/wallet` | rarely changed |

---

## How records are built

- **amount** — thousands separators stripped; `Debit` → negative (expense),
  `Credit` → positive (income).
- **recordDate** — `TxnDate` if parseable, else `EmailDate`; date-only values are
  placed at local midnight in `WALLET_TIMEZONE`.
- **counterParty** — the CSV `Merchant`.
- **note** — the CSV `Info`/`Subject` plus `[gmail-csv <shortId>]` for tracing
  back to the source email.
- **labels** — `[WALLET_LABEL]`.
- **category** — left unset (Wallet auto-assigns Unknown income/expense);
  recategorise in the app.

## Idempotency & re-runs

`state.json` records every pushed `MessageID` (and the returned Wallet record ID).
Re-running skips anything already there, so the daily job is safe to run
repeatedly. To force a full re-sync, delete `state.json` (this will create
duplicates unless you also remove the previously-created records in Wallet —
filter them by the `source:automation-monorepo` label).

## Filtering / undoing what this pack wrote

Everything is tagged. In the Wallet app, filter records by the
`source:automation-monorepo` label to see (or bulk-delete) only machine-imported
records.

---

## Troubleshooting

| Symptom | Cause / fix |
|---------|-------------|
| `WALLET_API_TOKEN is not set` | set it in `config/wallet/config.yaml` (or export it). Not needed for `--dry-run`. |
| `unauthorized (401)` | bad/expired token, or missing `records.create` scope. |
| `wallet sync in progress (409)` | initial data sync running; retry in a few minutes. |
| `rate limited (429)` | 300 req/hour cap; wait and re-run (state means no duplicates). |
| `skip (unmapped account "X")` | add `X` to `accounts.json`, or rely on `_default`. |
| Old rows show `failed` | Wallet rejects `recordDate` >10 years in the past; bound with `--since`. |
| `POST /labels: HTTP 4xx` | create the label named `source:automation-monorepo` in the Wallet app, then re-run. |
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
