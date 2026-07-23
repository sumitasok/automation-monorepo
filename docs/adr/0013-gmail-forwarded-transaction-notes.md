# ADR 0013 — Gmail pack: personal notes via manually-forwarded transactions

**Status:** accepted — 2026-07-23; amended same day (see Amendment below) —
decision point 6 originally rejected cross-row correlation; it's now
implemented.

## Context

The gmail pack extracts bank-alert transactions into `data/gmail/transactions.csv`
(ADR 0006) by matching each filter's `From:` query against the inbox and running
regex patterns over the body. That pipeline has no room for the user's own
commentary — e.g. "this was a reimbursement, not a real expense" or "split with
Alex" — because the source is always the bank's own email, never something the
user authored.

A cheap, no-new-infrastructure way to attach a note to a specific transaction is
to reuse email itself: forward the bank alert to a dedicated address and type
the note above the quoted original, the way anyone forwards mail. Because
`sumitasok+ai@gmail.com` is a plus-alias of the same account the extractor
already reads, no new mailbox, credential, or scope is needed — it's just
another message in the same inbox, addressed differently.

Two things this is *not*: it is not a replacement for the AI categorisation
pipeline (ADR 0010), which classifies rows already in the CSV; and it is not a
guarantee of exactly-once rows — see Consequences.

## Decision

1. **New `FORWARD_NOTE_EMAIL` config key** (`config.sample.yaml`, ADR 0007
   env-injection convention), default `sumitasok+ai@gmail.com`, overridable via
   the `-forward-note-email` flag or the env var. Configurable rather than
   hard-coded because plus-aliases are personal and the pack is shared code
   (ADR 0006).

2. **A second fetch pass in the default subcommand**, alongside the existing
   per-filter loop: query Gmail for `to:<alias>`, confirmed against the
   To/Delivered-To/X-Original-To/Cc headers (defense against `to:` search being
   looser than an exact recipient match). Reuses the existing sidecar `*.state`
   mechanism (`config/state.go`) under a synthetic path
   (`filters/_forwarded-notes.yaml.state`) for incremental fetch — no new state
   format.

3. **The preamble above the quoted original is the note.** Detected via the
   standard forward-marker boilerplate ("---------- Forwarded message
   ---------", "-----Original Message-----", "Begin forwarded message:").
   Everything else about the transaction (Amount, Type, Merchant, Account,
   dates) still comes from running the existing filter patterns against the
   forwarded body, which still contains the quoted original bank text below
   the note — the parser doesn't care that it arrived via a forward. This
   keeps one extraction path instead of a parallel manual-entry schema.

4. **Forwards with no preamble are skipped — no row is created.** If nothing
   is typed above the quote, there is nothing to add, and if the original bank
   email independently matches a filter's own `From:` query it is already a
   row from the normal pass. Creating an empty-note duplicate would be pure
   noise.

5. **`Note` is a new, additive `transactions.csv` column.** It is never
   populated by the normal per-filter pass — only by this one — so existing
   rows and consumers of the earlier 13-column schema are unaffected beyond
   the wider header.

6. ~~No cross-row correlation.~~ **Superseded — see Amendment below.**

## Amendment — 2026-07-23 (same day)

User feedback after the first pass: the note must land on the transaction the
email was forwarded from, not a separate row — "match based on which email it
was forwarded from, and associate the note with that transaction." The
original point 6 (no correlation; forwarding an already-fetched transaction
produces two rows) was rejected as not meeting that bar.

**Revised decision**: after parsing the forwarded body to recover
Type/Amount/TxnDate/Account, look for the transaction those fields identify —
first in the current run's own in-progress results (`txns`, not yet
persisted), then in `transactions.csv` itself via a new `CSVStore.Match`
(exact match on Type + Amount + TxnDate + Account) — and attach the note there
via a new `CSVStore.SetNote`, mutating the existing row instead of appending a
new one. Only when no original can be found anywhere does it fall back to a
standalone row (so the note is never silently dropped), and that fallback is
now logged unconditionally rather than only under `--debug`.

Matching is intentionally exact-string on fields the parser has already
normalised (`NormaliseDate` / `ParseWithLayout` for TxnDate) rather than fuzzy,
so it stays predictable; the cost is that it can occasionally miss on
formatting differences between an email and its forwarded/quoted copy, in
which case the fallback standalone row is the visible symptom.

`CSVStore.Write` only rewrites the CSV file when it was handed at least one
non-failed transaction; a `SetNote` call on an already-stored row is an
in-memory mutation that call wouldn't otherwise flush. `main.go` now always
calls `CSVStore.Save` once after `Write`, unconditionally, so a note attached
to an existing row (and the `Note` header itself, on a file that predates this
feature) is guaranteed to land on disk even on a run that fetched nothing new.

## Consequences

- `transactions.csv` gains a `Note` column (position depends on whichever
  columns are already present when this lands — see the pack's own commit
  history for the exact width at merge time, since another in-progress branch
  is independently adding Category/SubCategory/Labels columns).
- Forwarding is the only way to attach a note today; there is no CSV-editing or
  CLI path for annotating a row after the fact.
- No AI-assist path exists yet for forwards whose body doesn't match any known
  filter pattern — they are skipped with a `[SKIP]` log line. Extending
  `-ai-assist` to this pass is a reasonable follow-up.
- Because forwarding changes the `From:` header to the account owner, a
  forward is invisible to every existing per-bank filter's own query; it is
  only found via the new `to:<alias>` pass.

## Related

- ADR 0006 — applications as packs (gmail pack structure, `config/state.go`).
- ADR 0007 — pack config injection (`FORWARD_NOTE_EMAIL` follows the same env
  contract as `ANTHROPIC_API_KEY` / `DEEPSEEK_API_KEY`).
- ADR 0010 — AI categorisation (a separate, later enrichment pass over the same
  CSV; not touched by this decision).
