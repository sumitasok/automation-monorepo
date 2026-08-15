# Implementation Plan: `portfolio` pack — broker- and instrument-agnostic lot register and sell planner

**Branch**: `feature/portfolio-tax-pack` | **Date**: 2026-08-15 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/005-portfolio-tax-pack/spec.md`

## Summary

Move the forward-looking half of the `sa.finances` vault's `_db/tax/` analysis into this
workspace as a pack named `portfolio`, and turn its three hardcoded assumptions — one broker,
one instrument, one vault layout — into configuration.

The approach is **port, don't rewrite**. The existing Python is verified against manually
checked figures, and SC-001 (exact parity) is the feature's top criterion; a language change
would put the one thing that must not move at risk for no compensating benefit. The work is
therefore: relocate the data out of the pack, collapse the forked IBKR path into the single
broker-profile contract, finish the instrument seam, publish schemas, and split the page from
its data document.

The single most consequential design decision is that **the lot register stops being an
internal file and becomes a published, versioned, schema-validated contract** — because the
future `tax` pack reads it. That reframes several otherwise-internal choices (serialization,
atomicity, where annotations live) as contract decisions.

## Technical Context

**Language/Version**: Python 3.10+ (matches the vault code's existing idioms — `str | None`
unions, dataclass-free plain dicts). Jobs with `language: python` are run by `auto` with
`sys.executable`, i.e. the same interpreter running `auto` itself.

**Primary Dependencies**: PyYAML only. It is guaranteed present because `framework/tools/auto`
hard-requires it at import (`auto` exits with "PyYAML is required" otherwise) and runs python
jobs with `sys.executable`. **`ruamel.yaml` is deliberately dropped** — see research.md R-002.

**Storage**: Plain files under `data/portfolio/`. YAML for the hand-edited register, rate table
and profiles; JSON for the generated page data document. No database.

**Testing**: `unittest` from the standard library, plus a golden-figure regression test that is
the executable form of SC-001.

**Target Platform**: macOS (Sumit's machine) and Linux, run through `auto run`. The generated
page targets any modern browser and must also work opened directly from `file://`.

**Project Type**: A pack in this workspace — a CLI application invoked by the job runner, plus
static artefacts it produces.

**Performance Goals**: A full run over the current register (~15 lots, one instrument) is
interactive, under 2 seconds. Import of a broker export of ~2,000 rows completes in under 5
seconds. These are comfort thresholds, not engineering targets; nothing here is hot.

**Constraints**: No writes anywhere except through declared symlinks into `data/portfolio/`
(enforced by the write sandbox). The page must be a single self-contained file that renders
from `file://` with no network. Figures must match the vault program exactly.

**Scale/Scope**: Single user, single machine. Tens of instruments, hundreds of lots, thousands
of import rows — all comfortably small. The scaling axis that matters is **kinds of broker and
kinds of instrument**, not volume.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Answer each gate for THIS feature. `N/A` is a valid answer where a principle does not apply —
but state why in one line, do not leave it blank. Any `NO` must appear in Complexity Tracking
with the simpler alternative that was rejected and why. See `.specify/memory/constitution.md`
v1.0.0.

| # | Gate | Verdict |
|---|---|---|
| I | Does every value the pack needs (env, secrets, produced data) arrive via a `config.sample.yaml` declaration the workspace supplies — with no absolute path, workspace-relative path, or environment inspection in the pack? | **YES** — every input and output is a `data_files:` declaration. The pack has no secrets, so `files:` is empty and `env:` is empty. The vault path constants (`ROOT`, `VAULT`) that the current code carries are deleted outright. |
| II | Does the pack write **nothing** into `packs/` — every secret to `config/<pack>/`, every produced file to `data/<pack>/`, each reached through a declared symlink? | **YES** — and the atomic-write design in R-004 is specifically what keeps this true, since a naive temp-and-rename would silently replace the symlink with a real file. |
| III | If this feature has a UI: is it a static artefact under `data/<pack>/`, declared in the manifest, opening correctly from disk, with no port bound and no route owned by the pack? | **PARTIAL** — artefact, location, disk-rendering and no-port all hold (FR-061, FR-063, FR-065, FR-067). The *declaration* (FR-062, FR-064) cannot be satisfied: the workspace UI declaration contract does not exist yet. See Complexity Tracking. |
| IV | Is every derived artefact regenerated from manifests/config on demand rather than stored, with one loader per fact and no registration step? | **YES** — the page and its data document are regenerated from the register on every run, one loader is shared by planner and page, and nothing is hand-maintained. The register itself is source of truth, not a derived artefact. |
| V | Can a new instance of anything this feature handles (source, format, rule, category) be added as data, with one implementation of each shared computation and one contract covering all variants? | **YES** — and closing the existing violation (the forked IBKR engine) is user story 3. One broker-profile contract covers both flat and sectioned exports (R-003). |
| VI | Is every boundary this feature relies on enforced by the sandbox, `auto doctor`, or repo access — not by documentation or convention? | **YES** — write containment by the sandbox, symlink integrity by `auto doctor`'s `data_files:` check, visibility by pack `default_visibility: private`. |
| VII | Does this feature bind only to localhost, render no secret values, and make any data leaving the machine an explicit configured act? | **YES** — the pack binds nothing at all. Sharing is opt-in, requires confirmation (FR-050), and redaction strips the document rather than hiding it (FR-047). |

**Post-design re-check** (after Phase 1): Re-run. Gates I, II, IV, V, VI, VII unchanged and
still YES — the Phase 1 contracts strengthen V (one broker-profile schema all brokers validate
against) and II (R-004's symlink-aware atomic write is now specified, not assumed). Gate III
remains PARTIAL for the same single reason, unchanged by design work; no new violation appeared.

## Project Structure

### Documentation (this feature)

```text
specs/005-portfolio-tax-pack/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output — the published schemas
│   ├── README.md
│   ├── lot-register.schema.json
│   ├── broker-profile.schema.json
│   ├── explorer-document.schema.json
│   ├── disclosure-profile.schema.json
│   └── cli.md
├── checklists/
│   └── requirements.md
└── tasks.md             # /speckit-tasks output — NOT created here
```

### Source Code (repository root)

```text
packs/portfolio/                    # code, schemas, samples, docs — NO DATA
├── pack.yaml                       # name, description, default_visibility: private
├── config.sample.yaml              # env: {} · files: [] · data_files: [...]
├── Makefile                        # thin wrappers, matching the other packs
├── RUNBOOK.md
├── main.py                         # CLI entrypoint / subcommand dispatch
├── portfolio/
│   ├── __init__.py
│   ├── register.py                 # load/save the lot register; atomic, symlink-aware
│   ├── lots.py                     # generic lot maths — maturity, FIFO, splitting
│   ├── importer.py                 # broker export → events, driven by a profile
│   ├── profiles.py                 # broker-profile loading + the two reader shapes
│   ├── planner.py                  # break-evens, cushion, buy-back, ladder
│   ├── ratings.py                  # data-driven rating evaluation + why-templates
│   ├── document.py                 # build the page data document
│   ├── page.py                     # inject the document into the page template
│   ├── disclosure.py               # redaction profiles + derivability check
│   ├── schema.py                   # minimal JSON Schema subset validator
│   └── corpactions.py              # detect and refuse (FR-030)
├── schemas/                        # the published contracts (copied from contracts/)
│   ├── lot-register.schema.json
│   ├── broker-profile.schema.json
│   ├── explorer-document.schema.json
│   └── disclosure-profile.schema.json
├── profiles/                       # shipped broker profiles — data, not code
│   ├── schwab.yaml
│   └── ibkr.yaml
├── templates/
│   └── explorer.html               # data-free page shell
├── samples/                        # example data for a fresh install
│   ├── register.sample.yaml
│   ├── rules.sample.yaml
│   └── disclosure.sample.yaml
├── jobs/
│   ├── portfolio-import/manifest.yaml
│   ├── portfolio-report/manifest.yaml
│   └── portfolio-share/manifest.yaml
└── tests/
    ├── test_lots.py  test_importer.py  test_profiles.py
    ├── test_planner.py  test_ratings.py  test_schema.py
    ├── test_disclosure.py  test_register.py
    └── test_parity.py              # SC-001, the golden-figure regression

data/portfolio/                     # ALL data — git-ignored or versioned per file
├── .gitignore                      # records which of these are versioned
├── register.yaml                   # open lots + closed disposals (versioned)
├── fx-rates.yaml                   # dated FX with provenance (versioned)
├── rules.yaml                      # thresholds, rates, rating definitions (versioned)
├── disclosure.yaml                 # disclosure profiles (versioned)
├── explorer.html                   # THE declared page artefact (local)
├── explorer-document.json          # the document, for fetch-mode use (local)
└── shared/                         # redacted copies — never declared, never served
```

**Structure Decision**: The pack mirrors the layout of `packs/expenses` and `packs/wallet` —
`pack.yaml`, `config.sample.yaml`, `Makefile`, `RUNBOOK.md`, `jobs/<id>/manifest.yaml` — so it
is recognisable to anyone who has read another pack, differing only in being Python rather than
Go. Everything under `data/portfolio/` is reached from the pack's workdir through `data_files:`
symlinks; nothing in that column is ever a real file inside `packs/portfolio/`.

Note the flat basenames under `data/portfolio/`: `_link_pack_data_files` flattens every declared
name to its basename, so two declared files may not share one. See research.md R-005.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| **Gate III PARTIAL** — FR-062 (declare the page in the manifest) and FR-064 (conform to the workspace UI declaration contract) cannot be satisfied. | The contract does not exist. Constitution Principle III is marked aspirational; `auto serve` has no concept of a pack declaring a UI, and no ADR defines the manifest field. Spec Clarification 1 already scoped that mechanism to a separate framework feature. | **Inventing a `ui:` manifest block now** was rejected because it inverts Principle I: the pack would be dictating a workspace contract rather than declaring into one, and a guessed field would have to be migrated when the real contract lands. **Blocking the whole feature** was rejected because FR-061, FR-063 and FR-065–FR-074 are independently buildable and deliver the page regardless; only its discoverability waits. **Mitigation**: build everything else, keep the artefact at a stable declared path so the later declaration is a one-line addition, and track FR-062/FR-064 as the feature's only carried-forward debt. |

## Phase Status

- [x] Phase 0 — research.md complete, all NEEDS CLARIFICATION resolved
- [x] Phase 1 — data-model.md, contracts/, quickstart.md complete
- [x] Constitution Check re-run post-design
- [ ] Phase 2 — tasks.md (`/speckit-tasks`, not this command)
