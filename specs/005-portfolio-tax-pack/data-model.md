# Phase 1 Data Model: `portfolio` pack

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Contracts**: [contracts/](./contracts/)

Field names and types are normative; the JSON Schemas in `contracts/` are the machine-checkable
form of this document. Where the two disagree, the schema wins.

## Entity overview

```
Register (the published contract, one file)
├── positions{ticker} ──── Position
│                          ├── market ──────── MarketQuote
│                          ├── lots[] ──────── Lot ──── funding → FundingClass
│                          └── closed[] ────── Disposal ── from_lot → Lot.id
├── funding{key} ───────── FundingClass
└── meta ───────────────── contract version, generated_at, source fingerprints

RateTable (separate file)      — dated FX with provenance
RuleSet (separate file)        — thresholds, rates, LotRating[] definitions
BrokerProfile (per broker)     — the only broker-specific knowledge
DisclosureProfile (per audience) — what a shared copy may reveal

ExplorerDocument (generated)   — a flattened, computed projection for the page
ImportBatch (transient)        — the record of one ingestion
```

---

## Register entities

### Position

One instrument the owner holds. Keyed by ticker within `positions`.

| Field | Type | Required | Notes |
|---|---|---|---|
| `ticker` | string | yes | The map key; uppercase, `^[A-Z0-9.\-]{1,12}$` |
| `broker` | string | yes | Primary custodian; individual lots may differ |
| `currency` | string | yes | ISO 4217 the instrument trades in. Default `USD` |
| `market` | MarketQuote | no | Absent ⇒ unpriced (FR-016) |
| `lots` | Lot[] | yes | May be empty |
| `closed` | Disposal[] | yes | May be empty |

**Rule**: a Position with no `market` is reported unpriced and excluded from value-dependent
totals — never valued at zero (FR-016).

### MarketQuote

| Field | Type | Required | Notes |
|---|---|---|---|
| `spot` | number > 0 | yes | Price per share in the position's currency |
| `as_of` | date | yes | When observed. Staleness is surfaced, not corrected |
| `ath`, `low_52w`, `high_52w`, `avg_52w` | number | no | Context only; never enter a tax figure |

### Lot

One acquisition. The core entity.

| Field | Type | Required | Notes |
|---|---|---|---|
| `id` | string | yes | Unique within the position, e.g. `JUN25-RSU` |
| `broker` | string | yes | Where this lot sits (FR-023) |
| `acq_date` | date | yes | Effective acquisition date; drives maturity |
| `qty` | number > 0 | yes | Fractional allowed |
| `cb_per_share` | number ≥ 0 | yes | Tax cost basis in the position's currency |
| `price_paid_per_share` | number ≥ 0 | yes | Cash actually parted with; `0` for granted shares |
| `acq_fx` | number > 0 | yes | Reporting-currency rate on `acq_date` |
| `funding` | string | yes | Key into `funding` (FR-019) |
| `src` | string | yes | Provenance fingerprint; the dedupe key (FR-026) |
| `confirmed` | boolean | yes | `false` ⇒ at least one flag must be present |
| `flags` | Flag[] | no | Unverified values (FR-029) |
| `origin` | string | no | Free-text human note |
| `fifo_from` | date | no | Hidden from FIFO before this date |

**Derived, never stored**: `matures_on` = `acq_date` + rules `ltcg_after_months`; `mature` =
`matures_on ≤ valuation_date`. Storing either would let it drift from the rules file.

**Rule**: `cb_per_share` for compensation-funded lots comes from the employer's valuation, not
the broker export. An import that creates such a lot MUST attach a `cost_basis_unverified`
flag (FR-029).

### Disposal

A closed lot or the closed part of one.

| Field | Type | Required | Notes |
|---|---|---|---|
| `from_lot` | string | yes | The `Lot.id` consumed. Survives the lot's removal |
| `qty` | number > 0 | yes | ≤ the lot's quantity at the time |
| `acq_date` | date | yes | Copied from the lot; makes a Disposal self-contained |
| `cb_per_share`, `acq_fx`, `funding` | — | yes | Likewise copied |
| `disp_date` | date | yes | ≥ `acq_date` |
| `disp_price` | number ≥ 0 | yes | Per share, position currency |
| `disp_fx` | number > 0 | yes | Reporting-currency rate on `disp_date` |
| `fees` | number ≥ 0 | no | Default `0` |
| `holding_days` | integer ≥ 0 | yes | `disp_date − acq_date` |
| `long_term` | boolean | yes | Whether it matured before disposal |
| `lapse` | boolean | no | Specific-identification vest match, not FIFO |
| `src` | string | yes | Provenance fingerprint |

**Why the copies**: a Disposal is the unit the future `tax` pack computes realised gains from
(FR-010). Making it self-contained means that pack never has to resolve `from_lot` back to a
Lot that may have been fully consumed and removed. `from_lot` is kept for traceability only.

### FundingClass

| Field | Type | Required | Notes |
|---|---|---|---|
| `label` | string | yes | e.g. "RSU vest" |
| `own_money` | enum `none`\|`partial`\|`full` | yes | Drives the own-money-vs-compensation split |
| `desc` | string | yes | Shown on the page |

Shipped keys `RSU`, `ESPP`, `MARKET` are data, not code — a new class is a register edit.

### Flag

The mechanism by which an unverified figure stays visible (FR-029, SC-015).

| Field | Type | Required | Notes |
|---|---|---|---|
| `code` | enum | yes | `cost_basis_unverified`, `fx_interpolated`, `no_basis_on_transfer`, `corporate_action_suspected` |
| `note` | string | yes | Human explanation |
| `raised_by` | string | yes | `import` \| `migration` \| `manual` |

**Rule**: a flag propagates to every output that shows a figure derived from the flagged value —
register, page, and any shared copy. It is cleared only by a human edit.

---

## Standalone files

### RateTable — `fx-rates.yaml`

Entries of `{date, rate, source}` where `source` ∈ `sbi-tt` | `interpolated` | `manual`.
A lookup that resolves to a non-`sbi-tt` source MUST raise `fx_interpolated` on the consuming
lot. Lookup is exact-date first, then nearest prior entry.

### RuleSet — `rules.yaml`

| Field | Type | Notes |
|---|---|---|
| `capital_gains.<asset_class>.ltcg_after_months` | integer | `24` for foreign equity |
| `capital_gains.<asset_class>.stcg_rate`, `.ltcg_rate` | number | Effective rates |
| `lot_matching.method` | enum `fifo` | Extension point |
| `lot_matching.same_date_tiebreak` | string[] | Funding keys in priority order |
| `lot_matching.lapse_sale_window_days` | integer | `5` |
| `lot_ratings` | LotRating[] | Evaluated top-down, first match wins |

### LotRating

| Field | Type | Notes |
|---|---|---|
| `code`, `label`, `icon` | string | Identity and display |
| `tone` | enum `go`\|`ok`\|`caution`\|`stop`\|`neutral`\|`buy` | One meaning across pill, border and gauge (FR-034) |
| `when` | object | Metric+comparator predicates, ANDed. Empty ⇒ always matches |
| `cond`, `plain` | string | Legend text. MUST NOT contain `{tokens}` |
| `why` | string | Per-lot template. MUST contain at least one `{token}` |

Predicate keys are `<metric>_<cmp>` where metric ∈ `mature`, `cushion`, `price_vs_basis`,
`gain_frac`, `wait_days` and cmp ∈ `lt`, `lte`, `gt`, `gte`, `eq`. Adding a rating is a data
edit (FR-032); an unknown metric or comparator is a validation error, never a silent skip.

### BrokerProfile — `profiles/<name>.yaml`

See [contracts/broker-profile.schema.json](./contracts/broker-profile.schema.json) and research
R-003. Structure: `reader` discriminator, then either `tabular` (header tokens + candidate
column names) or `sectioned` (per-section field maps), then a shared `actions[]` routing list
and money/date parsing conventions.

### DisclosureProfile — `disclosure.yaml`

| Field | Type | Notes |
|---|---|---|
| `name` | string | Audience label |
| `withhold` | string[] | Document field paths to delete |
| `note` | string | Shown on the shared copy stating what was withheld (FR-048) |

Shipped profiles: `full` (withholds nothing) and `figures-hidden` (withholds absolute money and
quantities, retains ratings, percentage cushions and maturity) — FR-046.

---

## Generated: ExplorerDocument

The page's sole input (FR-036). A *projection*, not a second source of truth — regenerated on
every run, never hand-edited.

| Field | Type | Notes |
|---|---|---|
| `contract_version` | string | Semver; the page refuses unsupported majors (FR-040) |
| `generated_at` | date-time | Drives the staleness display (FR-066) |
| `valuation_date` | date | What "today" meant for this build |
| `reporting_currency` | string | e.g. `INR` |
| `disclosure` | string | Profile name; `full` for the declared artefact |
| `withheld` | string[] | Empty unless redacted (FR-048) |
| `rules` | object | `stcg`, `ltcg`, `ltcg_months` — enough to recompute in-page |
| `funding` | object | Copied from the register |
| `ratings` | object[] | Definitions incl. `tone`, for the legend |
| `positions` | object[] | Per instrument: ticker, currency, spot, fx, lots, closed |
| `flags_present` | boolean | True if any lot carries a flag |

Each document lot carries its computed figures — `mat` (maturity date), `rating`, `why`
(template already filled), `breakeven`, `cushion`, `buyback` — so the page renders without
re-deriving tax logic, while retaining `qty`, `cb`, `afx` so it can recompute at a
user-chosen price and date (the existing `calc()` behaviour).

**Rule**: a withheld field is *absent*, not null or zeroed (FR-047) — a null still reveals that
the field exists and, in aggregate, can leak structure.

---

## Transient: ImportBatch

Not persisted as a file; reported to the operator and summarised in the run log.

| Field | Type | Notes |
|---|---|---|
| `source` | string | Export path |
| `profile` | string | Broker profile used |
| `created` | object[] | Lots created |
| `matched` | object[] | Disposals matched, with the lots consumed |
| `skipped` | object[] | Rows already seen, by `src` fingerprint |
| `unrecognised` | object[] | Rows matching no action rule — MUST be reported (FR-022) |
| `warnings` | string[] | Near-maturity disposals (FR-028), suspected corporate actions |

**Rule**: `len(rows) == len(created) + len(matched) + len(skipped) + len(unrecognised) +
len(ignored)`. This identity is the executable form of SC-007 and MUST be asserted at the end
of every import.

---

## Fingerprints and identity

`src` is the dedupe key and the reason re-import is safe (FR-026). Format:

```
<broker>:<date>:<event>:<qty>@<price>#<n>
```

`n` disambiguates identical rows within one file. Dedupe compares `src`, never `(date, qty)` —
a lapse sale shrinks the vest lot it came from, so quantity stops matching after the first run
and the vest would be re-imported. This is inherited from the vault design and is load-bearing;
the parity test must cover it.

## State transitions

```
                  import (buy/vest/espp)
                            │
                            ▼
   (absent) ─────────────► OPEN ──────── partial disposal ──────► OPEN (reduced qty)
                            │                     │
                            │                     └──► new Disposal (closed[])
                            │
                            └──── full disposal ──► removed from lots[]
                                                    └──► new Disposal (closed[])
```

A Lot never moves back from closed to open. Correcting a mistaken import is a register edit plus
a re-run, not a state transition — which is why `--dry-run` (FR-027) exists.

## Validation summary

Enforced by schema: types, required fields, enums, date formats, numeric bounds, unique lot ids.

Enforced by code beyond schema:

1. `funding` on every lot resolves to a defined FundingClass.
2. `from_lot` on every Disposal refers to an id that existed.
3. `confirmed: false` ⇒ at least one flag present.
4. Every `why` template contains a token; no `cond`/`plain` does.
5. Every rating predicate uses a known metric and comparator.
6. Disposal quantities never exceed what the lot held at that point in the replay.
7. Every keyword used in a shipped schema is in the validator's supported subset (R-006).
8. The ImportBatch row-count identity above.
