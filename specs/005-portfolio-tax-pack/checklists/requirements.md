# Specification Quality Checklist: `portfolio` pack — broker- and instrument-agnostic lot register and sell planner

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-08-15
**Last Updated**: 2026-08-15 (iteration 2 — clarifications resolved)
**Feature**: [spec.md](../spec.md)

## Content Quality

- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

## Requirement Completeness

- [x] No [NEEDS CLARIFICATION] markers remain — all 3 resolved in iteration 2
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable
- [x] Success criteria are technology-agnostic (no implementation details)
- [x] All acceptance scenarios are defined
- [x] Edge cases are identified
- [x] Scope is clearly bounded
- [x] Dependencies and assumptions identified

## Feature Readiness

- [x] All functional requirements have clear acceptance criteria
- [x] User scenarios cover primary flows
- [x] Feature meets measurable outcomes defined in Success Criteria
- [x] No implementation details leak into specification

**Status: all 17 items pass. Ready for `/speckit-plan`.**

## Validation Notes

### Iteration 1 — findings and fixes applied

- *Success criteria were initially framed around the existing program's structure.* Rewritten as owner-observable outcomes (SC-003 "under 10 minutes to add an instrument", SC-006 "byte-for-byte unchanged", SC-007 "zero rows dropped unaccounted").
- *The broker-independence requirement was too weak to prevent the failure that already happened.* The existing analysis grew a second broker by duplicating the program, and the two brokers' format descriptions do not even share a shape. FR-019 (one profile contract covering both flat and sectioned exports), FR-020 (no per-broker program path) and FR-021 (retire the existing duplicate) were added to make that specific regression untestable-to-pass-by-accident.
- *Corporate actions were missing entirely.* The existing analysis has no handling for splits or mergers, and the tracked instrument has undergone one. Added as an edge case, as FR-030 (detect and refuse), and as an explicit Out of Scope entry so the gap is a decision rather than an oversight.
- *Unverified-value flags were treated as an implementation detail.* They are the mechanism by which the owner knows a figure rests on a guess, so they were promoted to FR-029 and SC-015.
- *"Pack holds no data" needed an enforcement story, not just a statement.* FR-003/FR-004 now tie to the workspace's declared-path and write-sandbox mechanisms, and SC-002 makes it checkable on demand.

### Iteration 2 — clarifications resolved, spec restructured

**Q1 — scope. Answer: two separate packs, delivered in sequence.** The owner corrected the framing rather than picking from the options: planning (before a sale) and calculation (against actual sales) are different tools, not two halves of one. Changes made:

- Retitled and rescoped the feature to the `portfolio` pack — the forward-looking half only (FR-007).
- Added a *Two packs, one register* section to the Overview making the split and the delivery order explicit.
- Promoted the lot register from an internal structure to a **published contract between the two packs** — FR-008 (single writer), FR-009 (versioned schema readable without this pack's behaviour), FR-010 (must carry everything the `tax` pack needs so it never re-derives from broker exports), FR-011 (atomic writes so a concurrent reader never sees a partial state).
- Added SC-013 as the gate: a consumer written only against the published schema can read the register. This is the precondition for specifying the `tax` pack at all.
- Moved the tax-return computation to Out of Scope with a named successor rather than leaving it ambiguous.
- Rewrote the parity requirement (FR-035) and SC-001 — the earlier version measured against fiscal-year ITR reference figures, which now belong to the other pack. Parity is now register-and-page figures against the vault program from identical inputs.
- Noted in Assumptions that compensation slips and per-fiscal-year facts stay in the vault and travel with the `tax` pack, since only that pack consumes them.

**Q2 — sharing. Answer: support both, local by default, hosted as a deliberate opt-in with redaction.** Added User Story 5 and requirements FR-043 through FR-050. Two decisions worth flagging for planning:

- *Redaction is applied to the data document, not the display* (FR-047). A page that merely hides figures still ships them, and this is the one irreversible mistake available in this feature — hence SC-011 verifies by searching the delivered bytes, not by looking at the rendered page.
- *A disclosure profile is rejected if a withheld figure is reconstructible from retained ones* (FR-049). Withholding absolute amounts while retaining quantities and per-share prices would withhold nothing in practice. This was added as an edge case first, then promoted to a requirement.

**Q3 — vault program. Answer: read-only cross-check for one filing cycle, then delete.** Added FR-058 through FR-060. The cycle is bounded concretely — through the FY2026-27 return rather than "one cycle" — because an open-ended retention is how the duplicate this feature exists to remove came about. FR-059 forbids editing the vault copy during the period, so it stays a fixed oracle rather than becoming a second maintained implementation. The vault program is also now listed as a Dependency, since SC-001 is measured against it.

### Deliberate assumptions taken instead of asking

Single jurisdiction; reporting currency unchanged; first-in-first-out disposal convention retained; page stays a single self-contained file; existing figures are the correctness oracle; pack visibility follows the existing convention for personal-financial packs. All recorded in the spec's Assumptions section.

### Carried into planning, not blocking

- Pack name proposed as `portfolio` (data at `data/portfolio/`), matching the single-word convention of gmail/wallet/expenses. Successor pack proposed as `tax`. Neither is registered in `packs.yaml` yet.
- Implementation language undecided. This would be the first non-Go pack in the workspace; the job manifest's `language:` field already accommodates that.
- The cross-pack read pattern has direct precedent — the `gmail` pack owns `transactions.csv` and both `wallet` and `expenses` read it without writing it — so FR-008 through FR-011 should follow that convention rather than inventing one.
