# ADR 0011 — Expenses pack: AI-clustered "events" over gmail-extracted transactions

**Status:** accepted — 2026-07-16

## Context

The gmail pack (ADR 0006) extracts bank/UPI transactions into
`data/gmail/transactions.csv` and, per ADR 0010, enriches each row with a fixed
Category/SubCategory/Labels taxonomy. That taxonomy is *static* (mirrors the
finance app's category list) and per-row. Sumit also wants a second, orthogonal
grouping: an **event** — an ad-hoc cluster of transactions that belong together
in time/story but not in category (e.g. a "Goa Trip" event spans flights,
hotels, and food across several days and several Wallet categories; a "Diwali
Shopping" event spans clothing and gifts). Unlike the taxonomy, the set of
events is not known in advance — it has to be discovered from the transactions
themselves and grow over time, with later transactions recognised as belonging
to an event created by an earlier run.

This is a different shape of problem from ADR 0010's categorisation: there the
taxonomy is fixed and the model picks one of a known set. Here the "known set"
(events) is itself built by the model, incrementally, and must stay stable
across runs — a transaction seen next month should match "Goa Trip" again
rather than spawning "Goa Trip 2".

## Decision

1. **A new `expenses` app-backed pack (ADR 0006), pure-stdlib Go — no external
   dependencies, builds offline (same choice and rationale as wallet, ADR
   0009).** This is why the registry below is JSON, not YAML: `encoding/json`
   is stdlib, `gopkg.in/yaml.v3` (as gmail uses) is not. First subcommand:
   `update-event` (`go run . update-event`), invoked as the
   `expenses-update-event` job. The subcommand dispatch in `main.go` is a plain
   switch, matching wallet's shape, so adding subcommands later (e.g.
   `list-events`, `merge-events`) is additive.

2. **`expenses` only reads `transactions.csv`; it never writes to it.**
   gmail owns that file's schema and is its only writer (its own `categorize`
   subcommand enriches it in place, ADR 0010). A second pack writing columns
   into a file it doesn't own would duplicate schema knowledge across two
   independently-versioned repos and violate the single-writer-per-file
   discipline ADR 0005 establishes for shared data. Instead, `expenses` keeps
   its own output, keyed by the CSV's `MessageID` — the same join-by-id
   approach the wallet pack already uses to consume the same file read-only.

3. **Two separate files, split by provenance (ADR 0005), not one:**
   - **`config/events.json` — the event registry, versioned.** One entry per
     event: `id` (stable kebab-case slug), `name`, `description`, `keywords`
     (free-form hints), `created_at`, `updated_at`, `txn_count`. Although the
     model proposes new entries, the registry itself must be identical across
     machines for matching to be consistent — the same reasoning ADR 0005
     applies to "must-sync datasets" (commit a diffable text export; pretty-
     printed JSON is as diffable as YAML). This is also precedented by the
     gmail pack's `aiassist.FilterRepository`, which already appends
     AI-discovered patterns into a committed file. New events are appended,
     never rewritten wholesale, so diffs stay small.
   - **`state.json` — the per-transaction assignment ledger, local and
     git-ignored.** Maps `MessageID -> {EventID, Confidence, AssignedAt}`.
     Same shape and purpose as the wallet pack's dedupe ledger: it makes
     `update-event` idempotent (a transaction already assigned is skipped on
     re-run) without needing the registry file to record every individual
     transaction.

4. **DeepSeek, reusing the existing provider convention.** Same env contract
   as gmail (`DEEPSEEK_API_KEY`, `DEEPSEEK_MODEL`, deepseek-v4-flash default)
   and the same Strategy-interface shape as gmail's `categorize.Assigner`
   (named `Matcher` here). Only DeepSeek is implemented today.

5. **One call per run sends the full registry plus every unassigned
   transaction; the model returns, per transaction, either a match against an
   existing event id with a confidence score, or a proposed new event.**
   Mirrors ADR 0010 decision 4 (repeat the full reference list every call,
   `--batch-size` to split large backlogs). A **probability threshold**
   (`--threshold`, default `0.6`) decides the outcome: confidence at or above
   threshold accepts the matched existing event; below threshold (or no
   matching id returned) the transaction is treated as belonging to a new
   event.

6. **New-event proposals are de-duplicated within a batch before being
   created.** Several transactions in the same run can plausibly originate the
   same new event (e.g. five Goa-trip transactions all missing a match in the
   same call). The model is instructed to reuse an identical
   `new_event_name` across such rows in its response; the code then groups
   unmatched rows by normalised proposed name and creates **one** registry
   entry per group, not one per row, before assigning ids. This is the crux of
   "autogenerate the event and keep track of it so future records match the
   same event" — without this grouping step every run would fragment one real
   event into many near-duplicate registry entries.

7. **Every assignment is validated against the registry before being
   recorded.** A returned `event_id` that isn't in the loaded registry is
   treated as no-match (falls through to the new-event path) rather than
   trusted blindly — same defensive posture as ADR 0010's taxonomy
   validation.

## Consequences

- `transactions.csv` is untouched by this pack; event membership lives in
  `packs/expenses/state.json` joined by `MessageID`, and the catalog of known
  events lives in `packs/expenses/config/events.json`.
- The registry can only grow (or have descriptions/keywords amended) — nothing
  in this pack deletes or merges events; a `merge-events` subcommand is left
  for a future ADR if duplicate events are observed in practice.
- Event-matching quality depends on DeepSeek and on the registry accumulating
  useful `keywords`/`description` text over time; `--dry-run` previews
  assignments (and proposed new events) before anything is written.
- Complements ADR 0005 (definitions vs. produced data), 0006 (apps as packs),
  0007 (config injection: `DEEPSEEK_API_KEY` env), 0009 (wallet's
  read-only-consumer + local-ledger pattern), and 0010 (DeepSeek provider
  convention, full-reference-per-call, validate-before-write).
