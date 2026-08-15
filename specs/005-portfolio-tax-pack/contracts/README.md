# Contracts: `portfolio` pack

Four JSON Schemas and one command-surface document. These are the pack's published interfaces —
what another pack, another person, or a future version must hold to.

| Contract | Audience | Stability |
|---|---|---|
| [lot-register.schema.json](./lot-register.schema.json) | **Cross-pack.** The `tax` pack reads this. | Highest — breaking it breaks another pack |
| [broker-profile.schema.json](./broker-profile.schema.json) | Anyone adding a broker | High — profiles are hand-written against it |
| [explorer-document.schema.json](./explorer-document.schema.json) | The page; fetch-mode consumers | Medium — versioned, page refuses unknown majors |
| [disclosure-profile.schema.json](./disclosure-profile.schema.json) | The owner, configuring sharing | Medium |
| [cli.md](./cli.md) | Job manifests, RUNBOOK, the operator | Medium — flag names are referenced by manifests |

At implementation time these are copied to `packs/portfolio/schemas/` and shipped with the pack;
this directory is the design-time record.

## Why the register is the load-bearing one

Spec FR-009 and SC-013 make the register a contract rather than an internal file, because the
future `tax` pack computes realised gains from it. Two consequences show up in its schema and
are easy to undo by accident:

**Disposals are self-contained.** Acquisition facts (`acq_date`, `cb_per_share`, `acq_fx`,
`funding`) are *copied* onto each Disposal rather than referenced through `from_lot`. A fully
consumed lot disappears from `lots[]`, so a consumer resolving `from_lot` would find nothing.
`from_lot` is for traceability only. Do not "normalise away" the duplication.

**Derived values are absent by construction.** `matures_on` and `mature` are not fields. They
are computed from `acq_date` plus the rule set, so storing them would let a register drift from
the rules that define them — a stored `mature: true` would survive a change to
`ltcg_after_months` and quietly contradict it.

## Versioning

Every contract carries `contract_version` (semver). A reader MUST refuse a major version it does
not support rather than reinterpret the data under the wrong contract (FR-055, FR-040).

- **MAJOR** — a field is removed, renamed, or changes meaning; a constraint tightens.
- **MINOR** — an optional field is added; an enum gains a member.
- **PATCH** — description or example changes only.

All five start at `1.0.0`.

## Schema subset constraint

The pack validates with its own minimal JSON Schema implementation rather than a dependency
(research R-006), so these schemas use only: `type`, `required`, `properties`,
`additionalProperties`, `items`, `enum`, `const`, `pattern`, `minimum`/`maximum`,
`exclusiveMinimum`, `minLength`, `minItems`, `minProperties`, `oneOf`, `$ref` into `#/$defs`,
`format: date` and `format: date-time`, plus non-enforced `default` and `description`.

The validator **rejects unknown keywords rather than ignoring them**, and a test asserts every
keyword used here is supported. Silently skipping an unrecognised constraint is the failure
mode this arrangement exists to prevent — a schema that looks stricter than it is would be worse
than no schema.

## Reading order

Start with `lot-register.schema.json` — every other contract is downstream of it.
`broker-profile.schema.json` is where the feature's hardest problem lives (one contract, two
export shapes; see research R-003).
