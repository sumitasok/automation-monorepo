# ADR 0022 — Wallet fetch: incremental by default, relative to the last downloaded record

**Status:** accepted — 2026-08-29

## Context

ADR 0020 shipped `wallet fetch` as a full-history-only operation: every run
called `GET /v1/api/records` for the entire account (~6,328 records) and
overwrote `data/wallet/records.json` from scratch. That's fine for the very
first pull, but Sumit asked for a way to fetch just what's new "relative to
the last downloaded record" — running a full ~6,328-record fetch every time
just to pick up a handful of new gmail-synced transactions is wasteful, and
will only get more so as the account grows.

The Wallet API's own OpenAPI spec (confirmed 2026-08-29 against
`/wallet/openapi/ui#/Banking/getRecords`, the same read that grounded the
ADR 0020 correction/addendum) documents `updatedAt` as a genuine, separate
range-filter dimension from `recordDate`: *"Filter by last sync timestamp
(when entity was last updated in the API database). Requires range
prefix."* That's the natural cursor for "what changed since last time" —
and, importantly, it also catches **edits** to old records, not just newly
created ones. That distinction matters here specifically: this pack's local
mirror exists to support two follow-up jobs (duplicate resolution and
Unknown-category recategorization, still unbuilt) that will themselves patch
Wallet records — changing a category doesn't change a record's `recordDate`,
but it does bump `updatedAt`. An incremental fetch keyed on `recordDate`
alone would never notice its own follow-up jobs' edits on the next pass.

## Decision

1. **`fetch` gains three modes, chosen automatically:**
   - **incremental** (the new default, once `records.json` already exists):
     compute the latest `updatedAt` already on disk across all downloaded
     records, subtract a 5-minute safety overlap, and fetch
     `updatedAt=gte.<cursor>` — new records and edits to old ones — then
     merge the result into the existing snapshot by `id` (an id already on
     disk is replaced by the fresher copy just fetched; a new id is
     appended). Re-fetching a record already on disk is harmless — the merge
     overwrites it with an identical copy — so the overlap errs toward
     re-fetching a little rather than risking a gap.
   - **full** (the default when no usable `records.json` exists yet, or
     `--full` is passed): unchanged from ADR 0020 — fetch everything, no
     `updatedAt` filter, overwrite the file fresh.
   - **since** (when `--since YYYY-MM-DD` is passed): unchanged from ADR
     0020 — an explicit ad-hoc `recordDate`-filtered query, written fresh
     (not merged with what's on disk). This is for a human asking "show me
     everything from March on", not for the routine incremental path.
2. **An incremental fetch still always sends `recordDate=gte.<farPastFloor>`**
   (the ADR 0020 fix), even though the actual filter doing the work is
   `updatedAt`. The API's documented default-window behavior is tied to
   `recordDate` specifically, independent of any other filter present in the
   request — so an incremental call that left `recordDate` unset and relied
   only on `updatedAt` could silently miss a record dated eight months ago
   that was only just recategorized: its `updatedAt` would match the filter,
   but its (old) `recordDate` would fall outside the implicit 3-month
   window and the record would never be returned. Pinning `recordDate` wide
   open neutralizes that window entirely, leaving `updatedAt` as the only
   real constraint.
3. **The cursor comes from the data, not from `fetchedAt` metadata.** Rather
   than trusting the previous run's recorded timestamp, `maxUpdatedAt` scans
   every record actually in the existing snapshot and takes the latest
   `updatedAt` field found. This is self-healing: if a previous run's
   metadata were ever wrong or missing, the cursor still reflects what's
   really on disk. If no record has a parseable `updatedAt` at all (empty or
   corrupt file), `fetch` logs a warning and falls back to a full fetch
   rather than either failing or silently fetching nothing.
4. **Merge is a plain map keyed by `id`** (`merge.go`, package `main`, not
   `internal/wallet` — it's fetch-specific assembly logic, not a Wallet API
   concern). Records without an `id` field are dropped rather than risked as
   silent duplicates (shouldn't happen against the real API; matters for
   malformed test fixtures). Existing order is preserved for surviving ids;
   genuinely new ids are appended at the end. No attempt is made to sort by
   `recordDate` or anything else — nothing downstream needs an ordering
   guarantee yet.
5. **`records.json`'s shape grows two fields**: `mode` (`"full"` |
   `"since"` | `"incremental"`) records which path produced the file, and
   `updatedSince` / `deltaFetched` (incremental only) record the cursor used
   and how many records that pass actually fetched before merging — kept
   separate from `count` (the merged total) and `apiTotal` (the API's own
   reported total for the filter just sent) so a reader can tell "how many
   changed this run" from "how many exist in total" at a glance.

## Consequences

- Routine re-runs of `./auto run wallet-fetch` (no flags) are now cheap:
  proportional to what changed since the last run, not the whole account.
  `--full` remains available for a deliberate from-scratch rebuild (e.g. if
  the mirror is ever suspected to have drifted).
- The mirror now tracks Wallet-side edits, not just new records — which is
  the property the not-yet-built duplicate-resolution and
  recategorization jobs will need once they start writing back to Wallet
  and want their own edits reflected in the next fetch.
- `--since`'s existing behavior (ADR 0020: an explicit ad-hoc query,
  overwriting the file, not merged) is preserved unchanged, but is no longer
  the routine way to do a lightweight update — incremental now is. `--since`
  remains useful for one-off "show me this date range" queries.
- Verified in isolation only so far (unit tests on `mergeRecords`/
  `maxUpdatedAt`, `GetRecords`'s new `updatedAt` parameter, and an
  end-to-end smoke test against a fake HTTP server proving the
  old-recordDate/new-updatedAt scenario is caught correctly). **Not yet
  verified against the live API** — the next real `./auto run wallet-fetch`
  after this change will be the first real incremental run, and should show
  `mode: "incremental"` with a small `deltaFetched` relative to the existing
  ~6,328-record `count`.
- Complements ADR 0020 (this pack's `fetch` subcommand and `records.json`
  contract) and ADR 0009 (the wallet pack itself).
