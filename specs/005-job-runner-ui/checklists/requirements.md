# Specification Quality Checklist: One-Click Job Runner UI

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-07-26
**Feature**: [spec.md](../spec.md)

## Content Quality

- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

## Requirement Completeness

- [x] No [NEEDS CLARIFICATION] markers remain
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

## Notes

### Re-validated after clarification session 2026-07-27

All 16 items still pass (16/16 → 16/16, no state changes). The spec grew from 29 to 39 functional requirements, 9 to 14 success criteria, and 4 to 5 user stories.

Resolved this session:

- **AI profile control (FR-007–FR-013)**: one top-right control replaces the per-action dropdowns. It sets a session-wide *default*; a pipeline step's own declared profile still wins (FR-010). This **reverses the previous FR-010**, which forbade showing a selector for actions that can't take one — recorded here so the change is not mistaken for drift.
- **Directory requirements (FR-014–FR-021)**: new section. Jobs and pipelines refuse to run without a valid data directory and configuration directory; read-only inspection actions stay usable without them (FR-019); the dashboard itself refuses to start (FR-020).
- **Validation strictness (FR-017)**: structural, not merely non-empty — chosen over the literal reading so that pointing at a plausible-but-wrong directory is caught.

⚠️ **This clarification invalidates part of the completed implementation** — see the Completion Report. The per-action AI dropdowns are built and must be replaced, and nothing currently requires or validates the directories.

### Original session 2026-07-26

- Both clarifications resolved by the user:
  - **Button surface (FR-004)**: widest option chosen — jobs, pipelines, read-only inspection commands, *and* state-changing maintenance commands. Because that last group can alter the machine outside the workspace, FR-005 and SC-008 were added to require an explicit confirmation step for them.
  - **Interactive prompts (FR-025, was FR-015)**: runs are always non-interactive; prompts are skipped exactly as on a scheduled run. Recorded as a deliberate scope boundary in Assumptions (interactive-only features stay terminal-only).
  - **Missing session tooling (FR-027, was FR-017)**: treated as a hard prerequisite with a clear startup error and explicitly no silent fallback.
- The spec deliberately avoids naming the specific session tooling in the requirements (kept technology-agnostic); the user's explicit request for it is captured in Assumptions/Dependencies instead, including the fact that it was not installed at the time (installed 2026-07-27).
- **Carried into planning**: this feature supersedes the earlier "read-only dashboard" decision (ADR 0012), which explicitly reasoned that no auth was needed *because* nothing could be triggered. That ADR needs superseding/amending as part of the plan.
