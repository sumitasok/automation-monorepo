---
description: "Task list for the portfolio pack"
---

# Tasks: `portfolio` pack — broker- and instrument-agnostic lot register and sell planner

**Input**: [spec.md](./spec.md), [plan.md](./plan.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: Included. The spec makes parity (SC-001) the top criterion, so the regression test is
not optional polish — it is the gate.

**Organization**: Grouped by user story so each is independently deliverable.

## Format: `[ID] [P?] [Story] Description`

- **[P]** — can run in parallel (different files, no dependency)
- Paths are relative to the repo root

---

## Phase 1: Setup

- [X] T001 Create `packs/portfolio/` skeleton: `portfolio/`, `schemas/`, `profiles/`, `templates/`, `samples/`, `jobs/`, `tests/`
- [X] T002 Write `packs/portfolio/pack.yaml` — name, description, `default_visibility: private`, maintainers
- [X] T003 Write `packs/portfolio/config.sample.yaml` — empty `env:`/`files:`, `data_files:` with unique basenames (research R-005)
- [X] T004 [P] Register the pack in `packs.yaml`
- [X] T005 [P] Create `data/portfolio/.gitignore` recording versioned vs local per file (Principle I, FR-006)
- [X] T006 [P] Write `packs/portfolio/Makefile` and `RUNBOOK.md`

## Phase 2: Foundational (blocking)

**⚠️ No user story work can begin until this phase is complete.**

- [X] T007 Copy the four schemas from `specs/.../contracts/` into `packs/portfolio/schemas/`
- [X] T008 Implement `portfolio/schema.py` — JSON Schema subset validator that **rejects unknown keywords** rather than ignoring them (R-006); collects all errors, each naming file, path and expectation (FR-053/054)
- [X] T009 Implement `portfolio/register.py` — load, validate, and **symlink-aware atomic save**: resolve the link, temp-and-replace at the target inside `data/portfolio/` (R-004, FR-011). Treat a dangling link as absent, not an error (R-005)
- [X] T010 Implement `portfolio/lots.py` — `add_months`, maturity, FIFO with same-date tiebreak and lapse-window specific identification, lot splitting (FR-025)
- [X] T011 Implement `portfolio/rules.py` — load rules, derive effective rates (`ltcg × (1+surcharge_cap) × (1+cess)`), resolve FX with provenance flagging (FR-029)

## Phase 3: User Story 1 — Parity and the data boundary (P1) 🎯 MVP

- [X] T012 [US1] Implement `portfolio/planner.py` — net proceeds, break-even, cushion, buy-back at both rates, per-lot tax now vs at maturity
- [X] T013 [US1] Implement `portfolio/ratings.py` — data-driven predicate evaluation, first match wins, `{token}` template filling; unknown metric/comparator is an error, never a silent skip
- [X] T014 [US1] Implement `portfolio/document.py` — build the ExplorerDocument projection per its schema
- [X] T015 [US1] Implement `portfolio/page.py` — inject the document into the template's `<script id="explorer-data">`, escaping `</script>` (R-008)
- [X] T016 [US1] Port `templates/explorer.html` from the vault, data-free, with staleness display (FR-066) and contract-version refusal (FR-040)
- [X] T017 [US1] Implement `main.py report` per [contracts/cli.md](./contracts/cli.md)
- [X] T018 [US1] Implement `main.py migrate` — vault → `data/portfolio/`, `--dry-run`, comment audit (R-002), reversible until verified (FR-057)
- [X] T019 [US1] Write `jobs/portfolio-report/manifest.yaml`
- [X] T020 [US1] **Parity test** `tests/test_parity.py` — golden figures from the vault program (SC-001) 🎯 the gate
- [X] T021 [P] [US1] `tests/test_store.py` (the symlink trap, atomicity). Lot maths is
  covered by `test_parity.py` and `test_importer.py` rather than a separate `test_lots.py`

## Phase 4: User Story 3 — One broker contract (P2)

- [X] T022 [US3] Implement `portfolio/profiles.py` — `tabular` and `sectioned` readers behind one contract (R-003)
- [X] T023 [US3] Implement `portfolio/importer.py` — action routing incl. **`qty_sign`** (R-003 trap), `src` fingerprints, idempotence, dry-run, near-maturity warning, row-count identity assertion (SC-007)
- [X] T024 [P] [US3] Write `profiles/schwab.yaml` (tabular)
- [X] T025 [P] [US3] Write `profiles/ibkr.yaml` (sectioned, with `qty_sign` rules)
- [X] T026 [US3] Implement `main.py import`; `jobs/portfolio-import/manifest.yaml`
- [X] T027 [US3] Delete the forked broker path — verify no broker name appears anywhere in `portfolio/` (FR-020, SC-005)
- [X] T028 [P] [US3] `tests/test_importer.py` — routing, qty_sign, idempotence, splitting,
  row accounting, and profile validation (folded in, no separate test_profiles.py)

## Phase 5: User Story 2 — Any instrument (P2)

- [X] T029 [US2] Verify multi-instrument end to end; `--ticker` scoping; unpriced positions excluded from value totals, never zeroed (FR-016)
- [X] T030 [P] [US2] `tests/test_multi_instrument.py`

## Phase 6: User Story 5 — Share without the numbers (P3)

- [X] T031 [US5] Implement `portfolio/disclosure.py` — field-path deletion from the document (FR-047) plus the derivability check (FR-049, R-010)
- [X] T032 [P] [US5] Write `samples/disclosure.sample.yaml` — `full` and `figures-hidden` (FR-046)
- [X] T033 [US5] Implement `main.py share` with `--preview` and full-disclosure confirmation (FR-050, FR-072)
- [X] T034 [P] [US5] `tests/test_disclosure.py` — asserts withheld figures absent from **delivered bytes** (SC-011)

## Phase 7: User Story 6 — Schemas enforced (P3)

- [X] T035 [US6] Implement `main.py validate` with `--strict`; all problems at once (FR-054)
- [X] T036 [P] [US6] `tests/test_schema.py` — incl. the assertion that every keyword in the shipped schemas is supported (R-006)

## Phase 8: Cross-cutting and polish

- [~] T037 Detect and refuse corporate actions (FR-030) — **PARTIAL**. Profile-declared
  detection is done: a `corporate_action` action rule refuses the whole import, named and
  tested, in both shipped profiles. R-009's *second* detector — inferring a split from a
  quantity/price discontinuity with no matching trade — is NOT built, so a split that the
  broker does not label slips through. No separate `corpactions.py`; the check lives in
  `importer.run` where the routing decision already happens.
- [X] T038 [P] Corporate-action tests — folded into `tests/test_importer.py`
  (`TestCorporateActionRefused`) rather than a separate file
- [X] T039 Run the full suite; `auto doctor`; confirm no data file inside `packs/portfolio/` (SC-002)
- [X] T040 Update `RUNBOOK.md` with the real command surface

## Deferred — blocked, not skipped

- [ ] T041 [US4] **FR-062** declare the page in the manifest — *blocked*: the workspace UI declaration contract does not exist
- [ ] T042 [US4] **FR-064** conform to that contract; add quickstart scenarios for SC-016/SC-018 — *blocked, same reason*

See plan.md Complexity Tracking. Everything else ships without these.

## Dependencies

```
Phase 1 → Phase 2 → ┬→ Phase 3 (US1, MVP)
                    ├→ Phase 4 (US3) ──→ Phase 5 (US2)
                    ├→ Phase 6 (US5)
                    └→ Phase 7 (US6)
                          all → Phase 8
```

Phase 3 alone is a working tool. Phases 4/6/7 are independent of each other.
