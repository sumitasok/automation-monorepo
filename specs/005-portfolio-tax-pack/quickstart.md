# Quickstart: validating the `portfolio` pack

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

Runnable scenarios that prove the feature works end to end. Each maps to a user story and the
success criteria it discharges. Run them in order — scenario 1 establishes the parity baseline
everything else is measured against.

## Prerequisites

- The pack mounted and registered in `packs.yaml`
- `data/portfolio/` populated by the migration (see scenario 0)
- The vault program still present, read-only, as the correctness oracle (FR-058)
- `python3` with PyYAML — guaranteed, since `auto` itself requires it

```bash
./auto doctor          # must be green before starting
./auto sandbox-check   # must pass on this machine before trusting write containment
```

---

## Scenario 0 — Migration (FR-056, FR-057)

**Proves**: the relocation loses nothing and is reversible until proven.

```bash
cd packs/portfolio
python3 main.py migrate --from ~/Claude/Projects/sa.finances/_db/tax --dry-run
python3 main.py migrate --from ~/Claude/Projects/sa.finances/_db/tax
python3 main.py validate
```

**Expected**: dry run lists every lot, rate and disposal it would move and writes nothing. The
real run populates `data/portfolio/` and leaves the vault untouched. `validate` passes clean.

**Check**: no free-comment annotation was dropped — the migration report must list every YAML
comment it found in `holdings.yaml` and either promote it to a field or state it was
documentation (research R-002).

---

## Scenario 1 — Parity (US1, SC-001) ⭐ the gate

**Proves**: the move changed no figure. Everything else is worthless if this fails.

```bash
cd packs/portfolio
python3 -m unittest tests.test_parity -v
python3 main.py report --format text > /tmp/pack.txt
cd ~/Claude/Projects/sa.finances/_db/tax
python3 refresh_explorer.py --dry-run > /tmp/vault.txt
diff <(grep -E 'lot|break|cushion' /tmp/pack.txt) <(grep -E 'lot|break|cushion' /tmp/vault.txt)
```

**Expected**: `test_parity` green, `diff` empty. Every lot's break-even, cushion, rating and
maturity identical to the vault program's.

**If it fails, stop.** A parity failure is not a rounding question — it means a tax convention
was lost in the port. Do not proceed to other scenarios.

---

## Scenario 2 — No data inside the pack (US1, SC-002, FR-002/003/004)

**Proves**: the pack/data boundary is real, not documented.

```bash
./auto doctor                                    # data_files must all be symlinks
find packs/portfolio -type f \
     \( -name '*.yaml' -o -name '*.json' -o -name '*.html' \) \
     -not -path '*/schemas/*' -not -path '*/profiles/*' \
     -not -path '*/samples/*' -not -path '*/templates/*' -not -name 'pack.yaml' \
     -not -name 'config.sample.yaml' -not -path '*/jobs/*'
```

**Expected**: `doctor` green; the `find` returns nothing. Every hit is a data file that escaped
into the pack.

**Negative check** — the sandbox actually stops a stray write:

```bash
./auto run portfolio-report -- --probe-write packs/portfolio/rogue.tmp
```

**Expected**: denied. If it succeeds, containment is not in effect — check `auto sandbox-check`.

---

## Scenario 3 — Second instrument, zero code change (US2, SC-003)

**Proves**: the instrument seam is real.

```bash
# Add a position to data/portfolio/register.yaml by hand — a ticker, a market
# quote, two lots. No file under packs/portfolio/ is touched.
python3 main.py validate
python3 main.py report --format text
python3 main.py report --ticker AVGO --format text
git -C . status --porcelain packs/portfolio    # must be empty
```

**Expected**: both instruments appear, rated and priced; `--ticker` scopes to one; the pack
directory is unmodified. Under 10 minutes end to end.

**Edge check**: remove the `market` block from the new position and re-run — it must report as
*unpriced* and be excluded from value totals, never valued at zero (FR-016).

---

## Scenario 4 — Second broker by profile alone (US3, SC-004, SC-005)

**Proves**: the hardest requirement — that the forked IBKR path is genuinely gone.

```bash
python3 main.py import --file <ibkr-export.csv> --profile ibkr --dry-run
python3 main.py import --file <ibkr-export.csv> --profile ibkr
python3 main.py validate
grep -ril 'ibkr\|schwab' portfolio/          # code dir, not profiles/
```

**Expected**: the sectioned IBKR export imports correctly through the same contract as the
tabular Schwab one; the register matches what `ibkr_engine.py` produces; the `grep` returns
**nothing** — no broker name anywhere in the code.

**The trap to verify explicitly** (research R-003): IBKR encodes buy vs sell in the *sign of the
quantity*, not an action string. Confirm a negative-quantity `Trades` row became a disposal and
a positive one became a lot. If every trade routed to the same event, `qty_sign` is not being
applied.

**Unrecognised rows**: feed an export containing an action string no rule matches. It must be
reported, not dropped (FR-022, SC-007).

---

## Scenario 5 — Idempotent import (SC-006)

```bash
cp data/portfolio/register.yaml /tmp/before.yaml
python3 main.py import --file <same-export.csv> --profile schwab
diff /tmp/before.yaml data/portfolio/register.yaml
```

**Expected**: `diff` empty — byte-for-byte unchanged. Then repeat with an *overlapping* export
(a wider date range covering the same rows): old rows skipped, only genuinely new ones imported.

---

## Scenario 6 — Page renders three ways (US4, SC-008)

```bash
python3 main.py report --format page
open data/portfolio/explorer.html          # 1. from disk, embedded
```

Then, with networking disabled, reload — it must still render completely (FR-037).

```bash
python3 main.py report --format json       # regenerate document only
```

**Expected**: (1) the declared artefact renders complete from `file://` with no network and no
companion file (FR-065, FR-073). (2) A fetch-configured variant pointed at a served document
renders from it. (3) With the document unreachable, that variant falls back to embedded and
*says so*, naming how old the fallback is (FR-039).

**Version check**: hand-edit the document's `contract_version` to a higher major and reload —
the page must refuse plainly rather than render partial figures (FR-040).

---

## Scenario 7 — Redaction removes, not hides (US5, SC-011) ⭐ the privacy gate

**Proves**: the one irreversible mistake available in this feature cannot happen.

```bash
python3 main.py share --profile figures-hidden
grep -c '"qty"' data/portfolio/shared/figures-hidden-*.html      # expect 0
python3 -c "import re,sys,json; ..."   # extract the embedded JSON and assert absence
```

**Expected**: withheld figures appear **nowhere in the delivered bytes** — not merely hidden by
CSS or omitted from the rendered view. Search the artefact itself, not the page.

**Derivability check**: write a profile withholding absolute money while retaining `qty` and a
per-share price. It must be **refused before anything is produced** (FR-049, exit 2), because
`qty × cb` reconstructs what it claims to withhold.

**Confirmation**: `share --profile full` must demand an explicit confirmation naming what is
about to leave the machine, and must refuse rather than default to yes when non-interactive
(FR-050).

**Not served**: confirm nothing under `shared/` is declared or reachable from the workspace
index (FR-070, SC-017).

---

## Scenario 8 — Schemas catch real mistakes (US6, SC-012)

```bash
python3 main.py validate            # clean baseline
# then, one at a time, introduce into a copy:
#   a misspelled key · a missing required field · a date where a number belongs
#   · a rating predicate with an unknown metric · an unsupported schema keyword
python3 main.py validate
```

**Expected**: each rejected with the file, the location within it, and what was expected
(FR-053). All problems reported together, not just the first (FR-054). No figure computed
(FR-052, SC-014).

**Consumer check** (SC-013): read `data/portfolio/register.yaml` using only
`contracts/lot-register.schema.json` and a YAML parser, with no reference to the pack's code.
This is the precondition for specifying the `tax` pack.

---

## Scenario 9 — Corporate action refused (FR-030, R-009)

```bash
# Craft an export containing a split, or a position whose share count jumps
# by a near-integer ratio with no matching trade.
python3 main.py import --file <split-export.csv> --profile schwab --dry-run
```

**Expected**: refused with the instrument named and the suspected event described; exit 2; the
register untouched. Other instruments still report normally. It must **not** silently produce
per-share figures across an unadjusted split.

---

## Scenario 10 — Flags reach every output (SC-015)

```bash
# Ensure a lot carries cost_basis_unverified and another fx_interpolated.
python3 main.py report --format text | grep -i 'unverified\|interpolated'
python3 main.py report --format page   # then inspect the rendered page
python3 main.py validate --strict      # must fail while any flag remains
```

**Expected**: the flag is visible in the text report, on the page, and in the register — every
output the owner can see. `--strict` fails, which is the check to run before trusting figures
for filing.

---

## Known gap

**Scenarios do not cover FR-062 or FR-064** — declaring the page in the manifest and conforming
to the workspace UI declaration contract. That contract does not exist yet; see plan.md
Complexity Tracking. When the framework feature lands, add a scenario asserting SC-016
(exactly one cell, renders with no companion file, no pack code executed) and SC-018 (discovered
by mounting alone; disappears on unmount).

## Coverage map

| Scenario | Story | Criteria |
|---|---|---|
| 0 Migration | — | FR-056, FR-057 |
| 1 Parity ⭐ | US1 | SC-001 |
| 2 Boundary | US1 | SC-002, FR-002/003/004 |
| 3 Instrument | US2 | SC-003, FR-016 |
| 4 Broker | US3 | SC-004, SC-005, SC-007 |
| 5 Idempotence | US3 | SC-006 |
| 6 Page | US4 | SC-008, SC-009 |
| 7 Redaction ⭐ | US5 | SC-010, SC-011, SC-017 |
| 8 Schemas | US6 | SC-012, SC-013, SC-014 |
| 9 Corporate action | — | FR-030 |
| 10 Flags | — | SC-015 |
| *(deferred)* | US4 | SC-016, SC-018 |
