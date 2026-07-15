# ADR 0009 — Wallet pack: push gmail-extracted transactions into BudgetBakers Wallet

**Status:** accepted — 2026-07-15

## Context

The gmail pack (ADR 0006) extracts bank/UPI transaction alerts from email into
`data/gmail/transactions.csv` (412 rows at time of writing). That file is a
read-only byproduct — nothing consumes it. Sumit wants those transactions to land
in his finance app, **BudgetBakers Wallet**, so they show up alongside manually
entered records, grouped by day, and clearly marked as machine-imported so they
can be filtered (or bulk-removed) later.

Wallet exposes a first-party **REST API** (`https://rest.budgetbakers.com/wallet`,
Premium plan) with a bearer token minted in the web app
(`/settings/rest-api`). `POST /records` creates records (batch ≤ 20), with
`amount` sign deciding expense vs income, and supports `labelIds`, `note`, and
`counterParty`. This is the same access shape as gmail (first-party API, revocable
token) — unlike telegram's account-level credential (ADR 0008) — so no new
credential-handling pattern is introduced; it reuses ADR 0007 config injection.

Per the ADR-driven model (every decision documented), the new pack and the
choices its sync makes get an ADR before it is used.

## Decision

1. **A new `wallet` app-backed pack (ADR 0006), pure-stdlib Go.** Consistent with
   gmail/telegram (single-language Go). Deliberately **no external dependencies**
   — only `net/http`, `encoding/json`, `encoding/csv`, `time` — so it builds
   offline with no `go.sum` (unlike telegram's `gotd/td`). One job, `wallet-sync`
   (`go run . sync`).

2. **One Wallet record per CSV transaction, not a daily aggregate.** Preserves
   merchant, amount, and payment type. "By day" is the *processing* unit: rows are
   grouped by calendar day, sorted, and pushed day-by-day in ≤20 batches so a
   partial failure is contained to a day and reported. Rejected alternative: one
   summed record per day — compact but destroys merchant-level detail the gmail
   extract worked to capture.

3. **Idempotent via a local dedupe ledger (`state.json`), keyed by MessageID.**
   The gmail `MessageID` is a stable unique key. Already-pushed IDs are skipped,
   so re-runs (and the daily schedule) never create duplicates. `state.json` is
   produced data — git-ignored and local (ADR 0005), same shape as gmail's
   `discover.state` and telegram's checkpoints. The ledger is the source of truth
   for "already synced"; it also stores the returned Wallet record ID.

4. **Every record carries one label, `source:automation-monorepo`** (name
   configurable via `WALLET_LABEL`). The label is resolved once per run and
   created via the API if missing. This is the filter/rollback handle Sumit asked
   for — Wallet labels are a real filter primitive (filter by label in-app), which
   a free-text note is not. The MessageID is *also* written into the record `note`
   (`[gmail-csv <id>]`) for traceability, and the merchant into `counterParty`.

5. **CSV→account mapping is explicit config, not inferred.** The CSV `Account`
   column is a bank last-4 / identifier (`3690`, `3176`, `1008`, `0878`, `XXX383`,
   `XX6102`, …) that does not correspond to Wallet account names. A committed
   `accounts.sample.json` maps each code → Wallet account UUID (+ optional
   per-account `paymentType`); the real `accounts.json` lives in
   `config/wallet/` (git-ignored, injected via `AUTO_PACK_CONFIG_DIR`, ADR 0007).
   A `_default` entry catches empty/unmapped codes; leaving its `accountId` empty
   **skips** such rows rather than dumping them into the wrong account.

6. **Normalisation rules (locked, validated against the live CSV):**
   - `amount`: strip thousands separators (`1,500.00`), take magnitude; `Debit`
     → negative (expense), `Credit` → positive (income).
   - `recordDate`: parse `TxnDate` (`YYYY-MM-DD`, `YYYY-MM-DD HH:MM:SS`); when
     absent/unparseable, fall back to `EmailDate` (RFC1123Z, trailing `(IST)`/
     `(UTC)` stripped). Date-only values are interpreted in `WALLET_TIMEZONE`
     (default `Asia/Kolkata`).
   - Rows that yield no usable amount or date are skipped and counted, never fatal.
     (1 of 412 rows — an empty-amount row — is correctly skipped.)

## Settings chosen for the first instance

- **Label:** `source:automation-monorepo` (created if missing).
- **Default payment type:** `debit_card`; credit-card accounts overridden to
  `credit_card` in `accounts.sample.json`.
- **Timezone:** `Asia/Kolkata`.
- **Schedule:** `wallet-sync` at 07:30 IST (after gmail-extract at 07:00),
  `enabled: false` until token + `accounts.json` are in place.
- **Category:** left unset (Wallet auto-assigns Unknown income/expense);
  recategorise in-app or by a future rule. Deliberately out of scope for v1.

## Consequences

- The workspace gains a Wallet REST **API token** in `config/wallet/`
  (git-ignored), handled exactly like gmail's secrets (ADR 0007). It is revocable
  from the Wallet web app; narrower than telegram's session credential (ADR 0008).
- Writes are **idempotent and reversible-by-filter**: everything the pack creates
  is tagged, so it can be found (and, if ever needed, bulk-deleted) by label.
- **API constraints to live with:** `records.create` scope must be enabled on the
  token (the MCP connector is read-only by default — see settings); `POST /records`
  rejects `recordDate` more than 10 years in the past, so a few very old CSV rows
  (pre-2016) will report per-item failures and are skipped — bound the run with
  `--since` if that noise is unwanted. Bank-synced accounts reject created records;
  map only manual accounts.
- Complements ADR 0005 (produced data local: `state.json`), 0006 (apps as packs),
  0007 (config injection: token env + `accounts.json` file). Same shape as gmail;
  the delta captured here is the CSV→Wallet normalisation and the label-based
  provenance/rollback handle.
- The *code* is dependency-free and could later be shared/public (each user
  supplying their own token + `accounts.json`); this instance is `private` as it
  syncs personal finance data.
