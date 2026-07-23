# Implementation Plan: Job Orchestrator

**Branch**: `001-job-orchestrator` | **Date**: 2026-07-23 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-job-orchestrator/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

Add a `auto orchestrate` command to the existing `framework/tools/auto` CLI that runs a named, multi-step pipeline declared in a YAML file under a new workspace-root `orchestrator/` directory. Each step references an existing job id (from any mounted pack) and reuses the exact execution path `auto run` already uses today, with optional per-step retry count, timeout, wait-before delay, and a bounded loop. The whole pipeline is validated (structure + job-id resolution) before any step executes, and every run is recorded (steps, attempts, iterations, outcomes) in a new SQLite file alongside the existing per-job run history.

## Technical Context

**Language/Version**: Python 3 (same interpreter/file as the existing `framework/tools/auto`; the file already uses `dict | None`-style union type hints, so Python 3.10+ is required — matches the installed 3.14.3)

**Primary Dependencies**: PyYAML (already a hard dependency of `auto`, used for manifest/pack/orchestration YAML parsing); Python stdlib only otherwise (`argparse`, `subprocess`, `sqlite3`, `datetime`, `time`, `pathlib`) — no new third-party dependency introduced

**Storage**: SQLite — new file `data/state/orchestrations.sqlite` (git-ignored by the existing `data/state/*.sqlite` rule, no `.gitignore` change needed), holding orchestration-run and per-step-attempt history, alongside (not replacing) the existing `data/state/runs.sqlite` used by single-job `auto run`

**Testing**: No automated test suite exists in this workspace today (the existing `framework/tools/auto` ships without one; correctness is validated via `auto doctor`-style structural checks plus manual scenario walk-throughs). This feature follows the same convention: validation-before-execution acts as the structural test, and `quickstart.md` documents the manual end-to-end scenarios that must pass

**Target Platform**: macOS/Linux, whichever single machine invokes `auto orchestrate` (matches `this_os()`/`runs_on.os` already used by jobs) — no cross-machine dispatch in v1

**Project Type**: Single project — this is an in-place extension of the existing CLI tool (`framework/tools/auto`), not a new service, library, or frontend

**Performance Goals**: N/A in the throughput sense — this is a personal batch-automation tool. The relevant goal is correctness of ordering/retry/timeout/loop bookkeeping, not speed

**Constraints**: Must not duplicate the job-execution logic already in `cmd_run` (env precedence, pack config linking, per-job timeout, run recording) — that logic is extracted into a shared function so `auto orchestrate` and `auto run` invoke one code path, which is what makes FR-016 ("identical behavior when no controls are set") true by construction rather than by convention. Sequential-only execution (no threads/async) in v1, matching the spec's Assumptions

**Scale/Scope**: A personal workspace with ~6 packs and a handful of jobs each; orchestrations are expected to have on the order of 2-10 steps. No need to design for large fan-out or high orchestration counts

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is still the unfilled template (`[PROJECT_NAME] Constitution`, all placeholder principles) — no principles have been ratified for this project yet, so there are no constitutional gates to evaluate against. Nothing to justify in Complexity Tracking as a result. This plan instead holds itself to the two constraints the spec already states explicitly: reuse the existing job-execution path (no parallel job-runner mechanism invented) and keep every new behavior YAML-configurable with no code changes needed per pipeline (FR-014).

**Post-Design Re-check** (after Phase 1 research/data-model/contracts/quickstart): still no ratified constitution to gate against. The Phase 1 design didn't introduce anything that would need justifying even under this project's own unwritten conventions — no new dependency, no new top-level module or service, no schema change to existing tables (`orchestrations.sqlite` is additive and new, `runs.sqlite` is untouched), and `execute_job()` extraction is a refactor of existing logic, not new complexity. Gate still passes.

## Project Structure

### Documentation (this feature)

```text
specs/001-job-orchestrator/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
# Single project: in-place extension of the existing framework/tools/auto CLI.
# No new top-level app/service is created — the orchestrator is a new command
# and a new data directory inside the workspace that already hosts packs/,
# schedules/, config/, data/.

framework/
└── tools/
    └── auto                      # existing CLI script — gains:
                                   #   - execute_job(): shared execution core,
                                   #     extracted from the current cmd_run()
                                   #     body so both `auto run` and
                                   #     `auto orchestrate` call one path
                                   #   - load_orchestrations() / load_orchestration(name):
                                   #     discover + parse orchestrator/*.yaml
                                   #   - cmd_orchestrate(): validate, then run
                                   #     (or list, if no name given) — FR-001..FR-016
                                   #   - _record_orchestration_run() / _record_orchestration_step():
                                   #     SQLite history, mirroring _record_run()

orchestrator/                     # NEW — workspace-root directory, sibling to
│                                  # packs/ and schedules/ (pipelines span packs,
│                                  # so they don't belong inside any one pack)
└── gmail-wallet-sync.yaml        # first orchestration: gmail-extract -> gmail-categorize

data/
└── state/
    └── orchestrations.sqlite     # NEW — git-ignored (existing data/state/*.sqlite
                                   # rule already covers it), separate from the
                                   # existing runs.sqlite used by single-job `auto run`

# No tests/ tree: this workspace has none today for framework/tools/auto;
# this feature follows the same convention (see Technical Context > Testing).
```

**Structure Decision**: Single project, in-place extension — there is no
frontend/backend split and no new deployable unit. The orchestrator lives
entirely inside the existing `framework/tools/auto` script (new functions, not
a new file/module, to keep the single-script-CLI convention this repo already
follows) plus one new workspace-root directory (`orchestrator/`) for pipeline
YAML files, exactly parallel to how `schedules/` and `packs/` already sit at
the workspace root for their respective concerns.

## Complexity Tracking

*No constitution violations to justify — see Constitution Check above (no
ratified principles exist yet to violate). Table intentionally omitted.*
