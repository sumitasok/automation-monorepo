# Specification Quality Checklist: Portfolio pack — broker- and instrument-agnostic lot register, capital-gains planner and explorer

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-08-15
**Feature**: [spec.md](../spec.md)

## Content Quality

- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

## Requirement Completeness

- [ ] No [NEEDS CLARIFICATION] markers remain — **3 open** (FR-031 scope, FR-032 fetch audience, FR-047 vault disposition)
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable
- [x] Success criteria are technology-agnostic (no implementation details)
- [x] All acceptance scenarios are defined
- [x] Edge cases are identified
- [x] Scope is clearly bounded — pending resolution of FR-031
- [x] Dependencies and assumptions identified

## Feature Readiness

- [x] All functional requirements have clear acceptance criteria
- [x] User scenarios cover primary flows
- [x] Feature meets measurable outcomes defined in Success Criteria
- [x] No implementation details leak into specification

## Validation Notes

**Iteration 1 — findings and fixes applied:**

- *Success criteria were initially framed around the existing program's structure.* Rewritten as owner-observable outcomes (SC-003 "under 10 minutes to add an instrument", SC-006 "byte-for-byte unchanged", SC-007 "zero rows dropped unaccounted").
- *The broker-independence requirement was too weak to prevent the failure that already happened.* The existing analysis grew a second broker by duplicating the program, and the two brokers' format descriptions do not even share a shape. FR-014 (one profile contract), FR-015 (no per-broker program path) and FR-016 (retire the existing duplicate) were added to make that specific regression untestable-to-pass-by-accident.
- *Corporate actions were missing entirely.* The existing analysis has no handling for splits or mergers, and the tracked instrument has undergone one. Added as an edge case, as FR-025 (detect and refuse), and as an explicit Out of Scope entry so the gap is a decision rather than an oversight.
- *Unverified-value flags were treated as an implementation detail.* They are the mechanism by which the owner knows a tax figure rests on a guess, so they were promoted to FR-024 and SC-012.
- *"Pack holds no data" needed an enforcement story, not just a statement.* FR-003/FR-004 now tie to the workspace's declared-path and write-sandbox mechanisms, and SC-002 makes it checkable on demand.

**Deliberate assumptions taken instead of asking** (recorded in the spec's Assumptions section): single jurisdiction, reporting currency unchanged, first-in-first-out disposal convention retained, page stays a single self-contained file, fetching opt-in and local-first, existing figures are the correctness oracle.

**Open items requiring the owner's decision** — all three change scope or the security posture, and none has a defensible default:

| Marker | Question | Why it cannot be defaulted |
|---|---|---|
| FR-031 | Full fiscal-year tax-return computation, or forward-looking half only? | Roughly doubles the surface area; the two halves share only the lot register |
| FR-032 | Is the fetched document ever read by someone other than the owner? | Determines whether redaction and access control are requirements at all |
| FR-047 | Delete the vault-resident program after migration, or keep it running? | Keeping it re-creates the duplicated-arithmetic problem FR-015 exists to remove |

- Items marked incomplete require spec updates before `/speckit-plan`.
