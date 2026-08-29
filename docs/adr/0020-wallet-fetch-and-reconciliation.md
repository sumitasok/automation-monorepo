# ADR 0020 — Wallet fetch: a local mirror of Wallet records, for reconciliation

**Status:** accepted — 2026-08-29

## Context

Sumit noticed that records landing in BudgetBakers Wallet have duplicates and
many sit in the Unknown expense/income category. Investigating the live
account (`get_records`, Wallet MCP): 6,328 total records, 177 of them
categoryId=unknown.

Two things were already true of this workspace before this ADR:

1. **`wallet-detect-duplicates` (existing) is blind to actual Wallet state.**
   It only cross-checks `data/gmail/transactions.csv` against this pack's own
   `state.json` dedupe ledger — i.e. it can only catch duplicates *this
   pack's own `wallet-sync` job* might have introduced. It never calls the
   Wallet API and has no idea what the account actually contains.
2. **The duplicates plausibly don't all come from this pack.** Inspecting a
   sample Unknown record: `note: "Blinkit | via Canara CC x6102 | gmail-sync
   gm:1a04a87ede29274d"`. This pack's own `wallet-sync` (internal/sync
   `buildNote`) produces notes shaped `"<info> [gmail-csv <shortId>]"` — a
   different format. Something else, outside this monorepo, is also writing
   gmail-derived transactions into the same Wallet account. Whatever resolves
   duplicates/Unknown categories therefore cannot assume every affected
   record was created by `wallet-sync` or is traceable through
   `state.json` — it has to work from what Wallet itself reports.

That means duplicate-resolution and Unknown-category recategorization need a
read path against real Wallet data first. This ADR covers only that read
path — **fetching and storing records** — as step one of a larger,
explicitly incremental effort. Resolution logic (merging/deleting duplicates,
assigning real categories to Unknown records) is deliberately **out of
scope here** and will get its own ADR once the approach is worked out.

## Decision

1. **Extend the existing `wallet` pack with a `fetch` subcommand**, rather
   than a new pack. It shares the same Wallet REST client, the same
   `WALLET_API_TOKEN`/`WALLET_BASE_URL` config (ADR 0007), and the same
   `data_files:`/write-sandbox contract (ADR 0018/0019) that `sync` and
   `detect-duplicates` already use — there's no new credential shape or
   config surface to introduce.

2. **`GET /records` is added to `internal/wallet/wallet.go`** as
   `Client.GetRecords(updatedSince string) ([]Record, error)`, paginated 200
   at a time (matching `GetAccounts`/`GetLabels`), with an optional
   `updatedAt=gte.<value>` filter for future incremental fetches.
   **`Record` is a raw `map[string]any`, not a fixed struct.** Every other
   type in this client (`Account`, `Label`, `NewRecord`) is a concrete struct
   because this pack controls those shapes (write-side) or only reads a
   handful of stable fields. Records are different: the write consumers of
   this fetch (duplicate resolution, recategorization — not yet built) don't
   exist yet, so it isn't yet known which fields they'll need, and a rigid
   struct risks silently dropping a field (e.g. `transfer`, `place`,
   `photos`) that later turns out to matter. A generic map round-trips
   whatever the API returns; the cost is deferring type safety to whatever
   reads `records.json` next, which is an acceptable trade for a fetch-only
   step.

3. **Full fetch, every record, unfiltered.** Per the Context above, the
   duplicate/Unknown problem isn't provably confined to this pack's own
   writes, so scoping the fetch to `source:automation-monorepo`-labeled
   records (or to `categoryId=unknown`) risks missing exactly the records
   this effort cares about. A full fetch (6,328 records ÷ 200/page ≈ 32
   requests) is cheap against Wallet's rate limit (300 req/hour) and keeps
   `fetch` simple — filtering can always happen client-side against the
   local mirror.

4. **Stored as one JSON snapshot, `data/wallet/records.json`** (via the
   `data_files:` symlink contract, ADR 0019 — the pack writes the relative
   path `records.json` in its own workdir, `auto run` symlinks it to
   `data/wallet/records.json`): `{fetchedAt, since, count, records: [...]}`.
   JSON, not CSV (unlike `transactions.csv`), because records carry nested
   structure — `category{id,name,group}`, `labels[]`, `transfer{...}` — that
   duplicate/category logic will need intact; flattening to CSV now would
   just mean re-fetching later. Written via temp-file + rename (same pattern
   as `state.json` in `internal/state`), so an interrupted fetch never
   leaves a truncated `records.json` for the next reader.

5. **No dry-run mode for `fetch`.** Unlike `sync` (which can preview
   entirely from the CSV without touching the API), `fetch` **is** the API
   call — there's nothing to preview. It still requires `WALLET_API_TOKEN`
   like every other Wallet REST endpoint.

6. **New job `wallet-fetch`** (`jobs/wallet-fetch/manifest.yaml`), scheduled
   off (`enabled: false`) like `wallet-sync`/`wallet-detect-duplicates` were
   at their own first commit — not added to `orchestrator/gmail-wallet-sync.yaml`
   yet, since the pipeline step ordering depends on what the (not yet built)
   resolution job needs to run before/after it.

## Consequences

- `packs/wallet` now exposes three subcommands (`sync`, `fetch`,
  `detect-duplicates`) and the pack's own doc/comment surfaces (`main.go`
  package comment, `usage()`, `pack.yaml`, `RUNBOOK.md`) are updated to list
  all three.
- `data/wallet/` gains a second produced-data file, `records.json`, alongside
  `state.json` — same ADR 0005/0019 treatment (local, git-ignored, symlinked
  from the pack workdir).
- A full fetch briefly holds ~6k records (a few MB of JSON) in memory and on
  disk. Not a concern at this scale; if the account grows enough for this to
  matter, `--since` (already wired) lets a future run fetch incrementally
  against the last `fetchedAt`.
- This unblocks — but does not implement — the actual duplicate-resolution
  and Unknown-category recategorization jobs. Those are follow-up work with
  their own ADRs: what makes two Wallet records "the same" (this pack's own
  `sync` job already learned the hard way, in the 2026-08-27 duplicate fix,
  that MessageID alone under-catches — a records-level definition of
  "duplicate" has to be worked out fresh here, not assumed to carry over),
  and what recategorization does with a merchant/note that gives no reliable
  category signal.
- Complements ADR 0007 (config injection — token, unchanged), ADR 0009
  (wallet pack — this is pack's second job), ADR 0018/0019 (write sandbox /
  data files — `records.json` follows the same contract as `state.json`).

## Correction — 2026-08-29: reads live under /v1/api, not the bare path

Decision 2 assumed `GET /records` lived at the same un-prefixed path as
`GET /accounts`, `GET /labels`, and `POST /records` (all confirmed working
there in production). First real run of `wallet-fetch` proved this wrong:
`GET /records?limit=200&offset=0` returned `HTTP 404: {"message":"no Route
matched with those values"}`.

Sumit confirmed the correct route directly from the Wallet API's own
reference/try-it panel: `GET https://rest.budgetbakers.com/wallet/v1/api/records?limit=30`
returns records correctly. So this API splits its surface: write operations
and simple listing (`POST /records`, `GET /accounts`, `GET /labels`) live at
the un-prefixed path, while the richer, filterable "User Data" read
endpoints (pagination + `recordDate=gte.…`/`lt.…`-style range filters, per
the API's quick-reference doc) live under `/v1/api/`. Nothing else in this
API's public docs states this split explicitly — it was only discoverable by
hitting the endpoint for real.

Fix: `GetRecords` now requests `/v1/api/records` (`internal/wallet/wallet.go`).
No other change — pagination (`limit`/`offset`/`nextOffset`) and the response
envelope (`{records: [...], total: N}`) matched what was already assumed.

**Open question for whoever builds the follow-up duplicate-resolution job:**
the `--since`/`updatedAt=gte.` incremental-fetch filter this pack sends was
never verified against the live API (only `recordDate` range filtering is
confirmed in the docs) — check it works before relying on it, or fall back
to a full fetch.

## Correction — 2026-08-29: no-filter fetch silently applied an implicit lookback window

The open question above was answered sooner than expected, by the very next
real run. With the `/v1/api/records` route fixed, `wallet-fetch` completed
successfully — no error, no non-zero exit — but wrote only **504** records
to `records.json`, against a real total of **6,328** (per both the earlier
Wallet MCP read and the API's own `total` field, once the fetch code was
changed to capture it — see below). Every one of the 504 records fell within
roughly the prior 90 days.

Two separate bugs, in combination, produced this:

1. `--since` filtered on `updatedAt=gte.<value>`, not `recordDate`. This
   pack's own quick-reference doc only documents `recordDate=gte./lt.` range
   filtering; `updatedAt` as a filter dimension was assumed by analogy, not
   confirmed. The live API accepted the parameter without error but appears
   to have ignored it.
2. With `--since` empty (the default, for a full fetch), `GetRecords` sent
   no `recordDate` filter at all — on the theory that omitting a filter
   parameter would return everything. Instead, the live API applied an
   undocumented default lookback window (~90 days) when `recordDate` was
   absent. Nothing in the API's public docs states this; it was only
   discoverable by comparing a real "full" fetch's output against the known
   total.

Fix (`internal/wallet/wallet.go`, `fetch.go`):

- `GetRecords` now always sends an explicit `recordDate=gte.<value>` — the
  documented filter dimension, not `updatedAt`. When the caller passes no
  `--since`, it defaults to a floor date (`farPastFloor = "2000-01-01"`)
  instead of omitting the parameter, so the request never relies on
  whatever the API does by default when the filter is missing.
- `GetRecords` now also returns the API's own reported `total` (from the
  last page's response envelope) alongside the fetched records. `fetch.go`
  compares `len(records)` against this total and logs a `WARNING` if they
  don't match, so a future silent truncation — from this cause or a new one
  — surfaces immediately instead of looking like a clean success. This is
  recorded in `records.json` as `apiTotal`, next to `count`.
- `--since`'s doc comment and flag help text now say explicitly that it
  filters on `recordDate` (the record's own date), not `updatedAt`
  (last-modified time) — the two are not interchangeable for incremental
  fetches, and a future incremental-fetch design should pick deliberately
  between them rather than assume.

Verified in isolation (fake HTTP server tests asserting the exact query
string sent, plus a deliberate total-mismatch case exercising the new
WARNING) before being pushed to the device; **not yet re-verified against
the live API** — that requires another `./auto run wallet-fetch` and a
count check against the ~6,328 total.

## Addendum — 2026-08-29: confirmed against the actual OpenAPI spec, plus one more bug it surfaced

The correction above was written from behavioral inference (comparing a real
run's output to the known total). Sumit then pulled the actual `getRecords`
operation out of the Swagger UI directly (`/wallet/openapi/ui#/Banking/getRecords`
— the page itself is a JS-rendered SPA that WebFetch cannot execute, so this
needed a human to read it) and pasted the real parameter list and response
schema. It confirms the diagnosis exactly, with two corrections and one new
finding:

- The implicit window is **3 months**, stated explicitly in the spec:
  *"Use recordDate filter with range operators to scope results. If omitted,
  a default 3-month window is applied (see appliedRecordDateFilters in the
  response). Provide any single bound (e.g. gte.2020-01-01) to override."*
  ("~90 days" above was a close estimate from the data, not a documented
  number — 3 months is the real, documented behavior.)
- The response also carries `appliedRecordDateFilters` (the filter the
  server actually used, echoed back) — not currently captured by this pack;
  useful for a future debug/verbose mode, not added now since nothing here
  needs it.
- **New finding: `total` is not populated by default.** The spec says so
  directly: *"withTotal boolean (query) — When true, the response includes a
  total count of all matching items (requires an additional database query).
  Default: false."* Without it, the example response shows `"total": 0`. The
  self-check added in the correction above (`apiTotal` from `page.Total`,
  compared against `len(records)`) would therefore have compared the fetched
  count against a permanently-zero `total` and either logged a spurious
  WARNING on every run (`len(records) > 0 != apiTotal(0)`) or, worse, silently
  skipped the check entirely depending on how the zero-guard was written —
  either way the self-check would not have done its job.

Fix: `GetRecords` now sends `withTotal=true` on the first page only (offset
0) — the total shouldn't change mid-fetch, so only one page pays the
"additional database query" cost the spec warns about. `apiTotal` is
captured from that first page's `total` and reused across the whole fetch.

Also confirmed directly from the spec, matching what was already assumed:
`limit` accepts 1–200 (default 30 — this pack always passes 200 explicitly);
pagination is `offset`/`nextOffset`; the response envelope is
`{records, total, nextOffset, offset, limit, ...}`; ID-based lookups (`?id=`)
bypass the default window entirely (not used by this pack — full-collection
fetch only).

Verified in isolation (fake HTTP server tests now also assert `withTotal=true`
on the first page and its absence on later pages) before being pushed to the
device; **still not yet re-verified against the live API** — that requires
another `./auto run wallet-fetch` and a count check against the ~6,328 total.
