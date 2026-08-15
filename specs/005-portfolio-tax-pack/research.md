# Phase 0 Research: `portfolio` pack

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Date**: 2026-08-15

Every unknown in the plan's Technical Context is resolved below. Findings marked **⚠ trap**
are things that would silently produce a wrong result if implemented the obvious way.

---

## R-001 — Implementation language: Python, not Go

**Decision**: Python 3.10+, porting the existing vault code rather than rewriting it.

**Rationale**: SC-001 requires the pack's figures to match the vault program *exactly*, and
that program is ~4,650 lines of Python already verified against a manual review (net STCG
₹1,61,304, 667.652 AVGO units at FY end, $397.59 withholding — every row checked by hand). A
Go rewrite would re-derive tax arithmetic that is currently correct, putting the feature's top
success criterion at risk to gain consistency with the other packs. Consistency is worth
something; it is not worth re-deriving verified tax maths.

`language: python` is first-class in the runner, not a theoretical option:

- `execute_job()` maps `python` to `[sys.executable]`, i.e. the same interpreter running `auto`.
- `auto` hard-requires PyYAML at import, so `sys.executable` provably has PyYAML.
- The write sandbox already carves out `~/.cache/pip` for `language: python`.
- `auto new` offers python as its *default* language.
- `auto doctor` checks for `python3` on PATH.

**Alternatives considered**:
- *Rewrite in Go* — matches gmail/wallet/expenses, single binary, no interpreter question. Rejected on parity risk, and because the pack is a personal analysis tool where startup time and distribution are irrelevant.
- *Port incrementally, Go shell around Python core* — worst of both; two runtimes, no benefit.

---

## R-002 — Drop `ruamel.yaml`; PyYAML only

**Decision**: PyYAML alone. `ruamel.yaml` is removed from the dependency set.

**Rationale**: The vault code uses `ruamel.yaml` for one reason — round-tripping
`holdings.yaml` while preserving the owner's hand-written comments — and documents the friction
it causes (`pip3 install ruamel.yaml --break-system-packages`). Three things make it
unnecessary here:

1. **The register is now a published contract** (FR-009). A contract whose fidelity depends on
   comment preservation is not a contract. Anything that matters must be a schema'd field.
2. **The annotations already are fields.** Reading the current `holdings.yaml`, the load-bearing
   notes are `cb_review:`, `fx_review:`, `origin:`, `confirmed:` — real keys, not comments. Only
   file-header documentation is free comment, and that belongs in the schema and RUNBOOK.
3. **Zero-dependency keeps the pack portable**, consistent with the workspace's demonstrated
   posture (ADR 0012 rejected Flask for a whole dashboard; PyYAML is the framework's single
   dependency).

**Migration consequence**: the one-time migration (FR-056) must promote any surviving
free-comment annotation in `holdings.yaml` into a schema'd field before the switch, and the
verification step (FR-057) must confirm nothing was lost. This is a migration task, not a
runtime concern.

**Alternatives considered**:
- *Keep ruamel.yaml* — preserves comments, costs a non-stdlib dependency and the install friction the current README already apologises for.
- *Move the register to JSON* — trivially machine-readable, but it is hand-edited constantly and JSON has no comments at all, which is strictly worse for the human side.

---

## R-003 — One broker-profile contract covering two export shapes

**Decision**: A single `broker-profile.schema.json` with a required `reader` discriminator
taking one of two values, `tabular` or `sectioned`. Everything downstream of the reader — action
routing, event mapping, lot creation — is shared and shape-independent.

**Rationale**: This is the feature's central technical problem. The two existing profiles are
not two instances of one contract; they are different designs:

| | `schwab_csv.yaml` | `ibkr_format.yaml` |
|---|---|---|
| Shape | one flat table, title lines before the header | many sections, each with its own header |
| Row selection | find header by `header_tokens` | match `section_name`, filter on `DataDiscriminator` |
| Column mapping | `columns:` — canonical field → list of candidate names | `field_map:` — canonical field → single name, per section |
| Event decision | `actions:` — regex on the Action column, first match wins | implicit in which section the row came from |

The unifying insight is that **the difference is entirely in how you get from a file to a
stream of field-mapped rows.** Once each row is a dict of canonical fields plus a `section`
label, both brokers route identically. So the contract is:

```
reader: tabular   → header_tokens, columns (candidate lists), one implicit section
reader: sectioned → sections[] each with section_name, filter, field_map
                    ↓
        both emit: [{section, date, symbol, qty, price, amount, raw...}]
                    ↓
        actions: [{match, section?, event, funding?, lapse?}] — first match wins
```

IBKR's section identity is preserved by letting an action rule match on `section` as well as on
an action-string regex, so a `Trades` row with negative quantity becomes `sell` without needing
an Action column IBKR does not have.

**⚠ trap**: IBKR encodes buy-versus-sell in the *sign of the quantity*, while Schwab encodes it
in an Action string. The contract must therefore allow an action rule to match on a quantity
sign predicate, or every IBKR trade routes to the same event. This is the single most likely
place for the port to silently produce wrong results.

**Alternatives considered**:
- *Two contracts, one per shape* — honest about the difference but violates FR-019 and Principle V, and leaves the door open to a third shape spawning a third contract.
- *Normalise IBKR to a flat CSV in a pre-step* — keeps one reader, but the pre-step is broker-specific code, which is exactly what FR-020 forbids.

---

## R-004 — ⚠ trap: atomic writes must resolve the symlink first

**Decision**: Every register write resolves the symlink to its real target under
`data/portfolio/`, writes a temporary file **in that same directory**, then `os.replace()`s it
onto the target. Never write to, or rename onto, the symlink path in the workdir.

**Rationale**: FR-011 requires atomic writes so a reader (the future `tax` pack) never sees a
partial register. The obvious implementation — write `register.yaml.tmp` next to it and rename
— is wrong here in a way that only shows up later:

- `packs/portfolio/register.yaml` is a **symlink** into `data/portfolio/register.yaml`.
- `os.replace(tmp, "register.yaml")` **replaces the symlink itself** with a real file.
- The pack then has a real data file inside it — violating Principle II and FR-002.
- `auto doctor`'s `data_files:` check fails, and the write sandbox denies the next write.

This is precisely the credentials-drift failure of ADR 0018 reappearing as data. It is also
exactly what ADR 0018 Amendment 2 verified against, so it would be caught — but noisily, at
runtime, after the register had already moved.

Correct sequence: `target = Path(link).resolve()` → write `target.parent / (target.name +
".tmp")` → `os.replace(tmp, target)`. Both paths are inside `data/`, so the sandbox permits
both. The symlink is never touched.

Same rule applies to the page artefact and the data document.

**Alternatives considered**:
- *Write in place, no temp file* — simple, but a crash or a concurrent reader mid-write corrupts or half-reads the contract file.
- *Lock file* — solves concurrency but not atomicity, and adds a stale-lock failure mode for no gain at this scale.

---

## R-005 — ⚠ trap: declared data files are flattened to basename

**Decision**: Every declared `data_files:` entry must have a **unique basename**, and the flat
layout under `data/portfolio/` is chosen accordingly.

**Rationale**: `_link_pack_data_files` deliberately flattens — the link keeps the full relative
path the app expects, but the target is `data/<pack>/<basename>`:

```python
target = data_dir / Path(name).name     # flattened
link   = workdir / name                 # full relative path preserved
```

So declaring both `data/register.yaml` and `data/archive/register.yaml` would point two
different links at the *same* target file. The vault layout has exactly this shape today
(`data/holdings.yaml`, `data/fx_rates.yaml`, `data/rsu_slips/FY2025-26.yaml`,
`data/manual.FY2025-26.yaml`), so a naive one-for-one relocation is a live collision risk.

The plan's `data/portfolio/` layout is therefore flat with distinct names, and the pack's
internal paths are chosen to match.

Secondary note: the linker creates links that may be **dangling on first run** (it links before
the target exists). That is the mechanism behind spec edge case "pack mounted, job never run" —
the pack must treat a dangling link as *absent*, not as an error, and report it per FR-003.

---

## R-006 — JSON Schema validation without a dependency

**Decision**: Ship a small validator (`portfolio/schema.py`) supporting a documented subset of
JSON Schema draft 2020-12, and constrain the pack's own schemas to that subset.

**Supported subset**: `type`, `required`, `properties`, `additionalProperties`, `items`,
`enum`, `const`, `pattern`, `minimum`/`maximum`, `minItems`, `oneOf`, `$ref` to `#/$defs/*`,
and `format: date`. Every keyword the pack's four schemas actually use, and nothing else.

**Rationale**: The user asked for JSON schemas in the pack, and FR-052 requires validation
before any figure is computed. The reference `jsonschema` package would be a second dependency
for schemas we author ourselves and therefore fully control. A subset validator is roughly 200
lines, has no install friction, and — importantly for FR-053/FR-054 — lets us produce error
messages in our own terms (file, path, expectation, all problems at once) rather than adapting
someone else's.

The constraint is honest and checkable: a test asserts that every keyword appearing in the
shipped schemas is in the supported set, so a schema can never quietly use something the
validator ignores. **Silently ignoring an unsupported keyword is the failure mode to design
against** — the validator must reject unknown keywords rather than skip them.

**Alternatives considered**:
- *Depend on `jsonschema`* — full correctness, standard errors, one more dependency and worse error text for our purpose. Reasonable to revisit if the schemas grow beyond the subset.
- *Hand-written per-file validation, no schemas* — no dependency and no validator, but nothing publishable for the `tax` pack to read against, which fails FR-009 and SC-013.

---

## R-007 — Serialization split: YAML in, JSON out

**Decision**: Hand-edited inputs (register, FX rates, rules, profiles, disclosure) are YAML.
The generated page data document is JSON. Schemas are JSON Schema in both cases.

**Rationale**: The register is edited by hand constantly — lots get review notes, FX gets
provenance flags, bases get corrected — and YAML is materially better for that (comments where
they are still useful for humans, no quoting noise, readable diffs). The data document is
machine-generated and machine-read, embedded into HTML and parsed by `JSON.parse`, so JSON is
the natural fit and is what the existing page already consumes.

Validating YAML with JSON Schema is standard: parse to native structures, validate the
structures. The schema is the contract; YAML is one serialization of it.

**Consequence for the `tax` pack** (SC-013): a consumer needs a YAML parser to read the
register. Acceptable — the successor pack will be Python too, and the schema, not the
serialization, is what is published.

---

## R-008 — Splitting page from document without breaking `file://`

**Decision**: One page template with a `<script id="explorer-data" type="application/json">`
block, exactly as today. The build fills it for the embedded variant (the declared artefact),
or empties it and sets a fetch URL for the fetch-configured variant.

**Rationale**: The existing page already parses that block —
`JSON.parse(document.getElementById("explorer-data").textContent)` — and `refresh_explorer.py`
already fills it by string replacement. The split the spec asks for is therefore small, and the
`file://` requirement (FR-067) is already met by the embedded variant.

**⚠ trap**: browsers block `fetch()` of a `file://` URL under CORS. So the fetch variant cannot
be the artefact that must work from disk. This is independent confirmation that Clarification 5
(the declared, served artefact is the embedded one) was the right call — the alternative would
have shipped a page that breaks in the exact scenario FR-067 requires.

The document is embedded as JSON inside a `<script>` element, so the builder must escape any
`</script>` sequence in the payload. Content is numeric and symbolic, so this is a guard
against a pathological instrument name rather than a likely case, but it is a one-line
precaution against producing an unparseable page.

---

## R-009 — Corporate action detection (FR-030)

**Decision**: Detect, refuse, and say why. Never adjust.

**Detection**: (a) an import row whose action maps to a split/merger/spin-off event in the
profile; (b) a quantity or price discontinuity — a position's share count changing by a
near-integer ratio with no matching trade, or a per-share price gapping by such a ratio between
consecutive observations.

**Behaviour on detection**: refuse to produce per-share figures for the affected instrument,
name the instrument and the suspected event, and leave the register untouched. Other
instruments are unaffected and still reported.

**Rationale**: AVGO underwent a 10-for-1 split in 2024 and the vault code has no handling for
it at all, so this is a live gap rather than a hypothetical. Adjusting lots correctly requires
knowing the ratio, the effective date and the tax treatment of the specific action — real work
that the spec puts out of scope. Silently producing per-share figures across an unadjusted
split would be confidently wrong in a way the owner would not notice, which is the worst
available outcome.

---

## R-010 — Redaction and the derivability check (FR-049)

**Decision**: A disclosure profile lists withheld **field paths** in the document. Redaction
deletes those keys from the document before the page is built. A static check runs before any
output: for each withheld field, verify it cannot be reconstructed from retained fields via the
document's own known relationships.

**Known reconstruction paths to check**: `qty × cb` recovers a withheld cost total; `proceeds −
gain` recovers cost; `spot × qty` recovers market value; a per-share price plus quantity
recovers any absolute amount. So a profile withholding absolute money while retaining both
`qty` and any per-share price is rejected — which is exactly the naive profile someone would
write first.

**Rationale**: FR-047 requires redaction at the document level, and FR-049 requires the
derivability check to run before anything is produced. Encoding the relationships explicitly
(rather than inferring them) keeps the check honest and reviewable: the list of reconstruction
paths is itself data, and a new derived figure added to the document must be accompanied by its
reconstruction rule.

---

## Resolved unknowns summary

| Unknown from Technical Context | Resolution |
|---|---|
| Language | Python 3.10+, port not rewrite (R-001) |
| Dependencies | PyYAML only; `ruamel.yaml` dropped (R-002) |
| One profile contract for two export shapes | `reader: tabular \| sectioned` discriminator (R-003) |
| Atomicity for a cross-pack contract file | Resolve symlink, temp-and-replace at target (R-004) |
| Data file layout under `data/portfolio/` | Flat, unique basenames (R-005) |
| Schema validation approach | Subset validator, no dependency, rejects unknown keywords (R-006) |
| Serialization | YAML in, JSON out (R-007) |
| Page/document split vs `file://` | Embedded variant is the declared artefact (R-008) |
| Corporate actions | Detect and refuse (R-009) |
| Redaction correctness | Field-path deletion + explicit derivability rules (R-010) |

No NEEDS CLARIFICATION markers remain.
