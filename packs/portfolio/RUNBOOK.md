# portfolio — runbook

Broker- and instrument-agnostic lot register, sell planner and explorer page.

**What it answers:** for each batch of shares you hold, is selling it now cheap
or expensive, and how far can the price fall before waiting stops paying?

**What it is not:** investment advice. Every figure is arithmetic on your own
data — no forecast, no analyst input, no view on any company.

## Quick reference

```bash
./auto run portfolio-report                       # regenerate the page
./auto run portfolio-import -- --file X --dry-run # preview an import
./auto run portfolio-share  -- --profile figures-hidden
```

Directly, from `packs/portfolio/`:

```bash
python3 main.py validate                  # check every data file
python3 main.py report --format text      # the sell ladder, on stdout
python3 main.py report                    # write the page + document
python3 main.py report --ticker AVGO      # one instrument
python3 main.py report --as-of 2027-01-01 # roll the clock forward
python3 main.py report --spot 455 --fx 86 # what-if (never written back)
python3 main.py report --fetch-from URL    # fetch variant, for your own use
```

The default page is **self-contained**: it embeds its data and opens from disk.
`--fetch-from` builds a variant that pulls its document from a URL at load time
and falls back to the embedded copy (saying so) when that fails. Browsers refuse
`fetch()` on a `file://` URL, so the fetch variant needs to be served — keep the
default build for opening from disk.

## Where things live

| What | Where | Versioned |
|---|---|---|
| Code, schemas, broker profiles, page template | `packs/portfolio/` | yes |
| Register, FX rates, rules, disclosure profiles | `data/portfolio/` | yes |
| Generated page + document | `data/portfolio/` | no |
| Redacted copies | `data/portfolio/shared/` | no |

**No data file ever lives inside `packs/portfolio/`.** The `data/` directory in
the pack holds only symlinks that `auto` creates before each run (ADR 0019).
`auto doctor` checks this.

## Adding an instrument

Edit `data/portfolio/register.yaml`: add a key under `positions` with a
`broker`, `currency`, `market` and its `lots`. Run `validate`, then `report`.
**No code changes.** A position with no `market` block is reported as unpriced
and excluded from value totals — it is not treated as worthless.

## Adding a broker

Copy `profiles/schwab.yaml` (flat CSV) or `profiles/ibkr.yaml` (sectioned
statement), change the columns and actions, run with `--profile <name>`.
**No code changes.** If you find yourself wanting to add a branch to the Python,
the profile contract is wrong — extend the contract instead.

Watch for: a broker that encodes buy vs sell in the **sign of the quantity**
rather than an action string. Use `qty_sign: positive|negative` on the action
rule. Without it every trade routes to one event, silently.

## Importing

```bash
python3 main.py import --file ~/Downloads/export.csv --profile schwab --dry-run
python3 main.py import --file ~/Downloads/export.csv --profile schwab
```

**Always dry-run first.** It reports every lot it would create and disposal it
would match, and writes nothing. It simulates against a copy, so what it shows
is exactly what the real run does.

Import is idempotent by provenance fingerprint — re-running the same file, or a
wider export overlapping one already imported, changes nothing. Every row is
accounted for: created, matched, skipped, ignored or **unrecognised**. An
unrecognised row means the profile needs a rule; it is never dropped silently.

A disposal landing shortly before its lot would have matured is warned about.
That cost is real and invisible afterwards.

## Sharing

```bash
python3 main.py share --profile figures-hidden
```

Redaction **deletes** withheld figures from the data document; it does not hide
them in the page. Before producing anything, two checks run:

1. **Derivability** — a profile is refused if what it withholds can be
   reconstructed from what it keeps (quantity × a per-share price recovers a
   basis).
2. **Byte-level survival** — every withheld value is searched for in the
   finished copy. This is what catches figures hiding in prose: a rating
   explains itself using that lot's own numbers, and a migrated cost-basis
   review note quotes them outright.

A full-disclosure copy needs an interactive `yes` and is refused on an
unattended run.

## Flags — when a figure rests on a guess

| Flag | Means |
|---|---|
| `cost_basis_unverified` | The basis came from the broker, but compensation shares are valued by the employer. Reconcile before filing. |
| `fx_interpolated` | The FX rate is estimated, not a verified same-day rate. |
| `no_basis_on_transfer` | Shares arrived by transfer with no cost basis. Not zero — unknown. |
| `corporate_action_suspected` | Per-share figures across this would be wrong. |

Flags reach every output. `validate --strict` fails while any remains — run it
before trusting figures for filing.

## Corporate actions

**Detected and refused, never adjusted.** A split, merger or spin-off changes
quantities and per-share basis for every lot, and adjusting correctly needs the
ratio, the effective date and the tax treatment. Producing per-share figures
across an unadjusted split would be confidently wrong in a way you would not
notice — so the import stops and names the instrument.

## Known gap

The page is not yet listed in the workspace index. That needs the workspace UI
declaration contract, which does not exist (constitution Principle III is
aspirational). Open the page directly: `data/portfolio/explorer.html`. It is
self-contained and renders from disk with no server.

## Verifying against the vault

The original program in `sa.finances/_db/tax/` is retained read-only through the
FY2026-27 filing cycle as the correctness oracle. `tests/test_parity.py` freezes
its figures for all 19 lots; that test is the gate. A failure there is not a
rounding question — it means a tax convention was lost.

```bash
python3 -m unittest discover -s tests -t .
```
