# Phase 0 Research: Job Orchestrator

No `NEEDS CLARIFICATION` markers were left in the spec's Technical Context (all resolved directly with reasonable defaults consistent with the existing `framework/tools/auto` codebase). This document instead records the design decisions made while translating the spec into an implementable shape, each with rationale and rejected alternatives, so `/speckit-tasks` and `/speckit-implement` have a single source of truth instead of re-deriving them.

## Decision: `orchestrator/` as a new workspace-root directory

**Decision**: Orchestration YAML files live in a new top-level `orchestrator/` directory, sibling to `packs/`, `schedules/`, `config/`, `data/`.

**Rationale**: Orchestrations intentionally span multiple packs (FR-005). Placing them inside any one pack (e.g. `packs/shared/orchestrations/`) would tie a cross-pack concern to one pack's visibility/ownership rules, which conflicts with the spec directly. A workspace-root directory has no pack-visibility rules to conflict with, matching how `schedules/generated/` already holds a workspace-level, pack-spanning artifact.

**Alternatives considered**:
- `packs/shared/orchestrations/` — rejected: couples a cross-pack concern to one specific pack's ownership/visibility, and `auto share`/`auto doctor`'s per-pack visibility-leak checks would need special-casing to ignore it.
- Inside `framework/` — rejected: `framework/` is the tool's own code (a git submodule per the README), not workspace-owned data; orchestrations are user content like jobs are, not framework internals.

## Decision: CLI shape — `auto orchestrate` (bare = list, `auto orchestrate <name>` = run)

**Decision**: `auto orchestrate` with no argument lists available orchestrations (id, step count, source file). `auto orchestrate <name>` validates and runs that orchestration. No separate `run`/`list` subcommand keywords.

**Rationale**: The user's own example was the flat form `./auto orchestrate gmail-wallet-sync`, not `auto orchestrate run gmail-wallet-sync`. `auto` already has precedent for "bare command = list": `auto packs` lists packs with no subcommand. Mirroring that means no new sub-verb vocabulary, and no reserved orchestration name (like `list`) is needed.

**Alternatives considered**:
- `auto orchestrate run <name>` / `auto orchestrate list` — rejected: adds a sub-verb the user didn't ask for and breaks the requested flat invocation.
- `auto orchestrate <name> --list` or a `--list` flag on the same command — rejected: less discoverable than the bare-command convention `auto` already uses elsewhere, for no benefit.

## Decision: Reuse `cmd_run`'s execution core via extraction, not subprocess-of-subprocess

**Decision**: Extract the body of the current `cmd_run()` (env precedence, pack config file linking, workdir resolution, `exec`-vs-`entrypoint` command building, `subprocess.run` with timeout, `_record_run()` call) into a plain function, e.g. `execute_job(job_id: str, extra_args: list[str], ai: str = "") -> tuple[int, float]`, returning `(returncode, duration_seconds)`. `cmd_run()` becomes a thin CLI wrapper calling it once and exiting with its return code; `cmd_orchestrate()` calls it once per step (per attempt, per loop iteration).

**Rationale**: FR-016 requires a step with no retry/timeout/wait/loop to behave *exactly* like `auto run` today. The only way to guarantee that by construction (not by keeping two implementations in sync by hand) is one shared code path. It also avoids double process overhead (a subprocess spawning `python3 framework/tools/auto run <id>` as a child, which itself spawns the job's own process) and avoids awkward exit-code/timeout translation across two process boundaries.

**Alternatives considered**:
- Shell out to `./auto run <id> -- <args>` as a subprocess per step — rejected: doubles process spawning per step, and a step-level timeout would then need to race against a *parent* process wrapping a *child* process, complicating the "stopped and counted as a failed attempt" behavior (FR-009) for no benefit.
- Duplicate the run logic inline in a new `cmd_orchestrate` — rejected: guarantees drift between `auto run` and orchestrated runs over time, directly risking FR-016.

## Decision: Separate SQLite file for orchestration history

**Decision**: New file `data/state/orchestrations.sqlite` with two tables — `orchestration_runs` (one row per `auto orchestrate <name>` invocation) and `orchestration_steps` (one row per step attempt/iteration) — rather than extending the existing `runs.sqlite` schema used by single-job `auto run`.

**Rationale**: The existing `runs.sqlite` schema (`runs(ts, job, host, rc, dur)`) models a single flat attempt with no concept of "which pipeline, which step index, which iteration, which attempt number." Bending that schema to also carry nested orchestration/step/attempt/iteration structure would either require nullable columns bolted onto an unrelated table or a schema migration of data already in use by `auto doctor`/the dashboard. A new file needs zero `.gitignore` changes (the existing `data/state/*.sqlite` glob already excludes it) and can't corrupt or migrate the existing table.

**Alternatives considered**:
- Add columns to `runs.sqlite` (`orchestration`, `step_index`, `iteration`, `attempt`, nullable for plain job runs) — rejected: conflates two different record shapes in one table, and every existing reader of `runs.sqlite` would need to learn to ignore the new nullable columns.
- No persistence, stdout-only reporting — rejected: fails SC-005 (user can tell what happened in a *completed* run without reading per-job logs), which implies the run's own record must be inspectable after the fact, not just visible in the terminal that happened to be open at the time.

## Decision: Loop stop-condition vocabulary limited to `max_iterations` + optional `until_exit_code`

**Decision**: A looping step MUST declare `max_iterations` (a positive integer bound). It MAY additionally declare `until_exit_code: N`, meaning the loop stops (without being treated as a failure) as soon as an iteration's exit code equals `N`. No other condition forms are supported in v1.

**Rationale**: FR-011/FR-012 require a bound and allow an early-stop condition, but no job in this workspace today emits any richer signal (structured JSON, a sentinel file, etc.) to evaluate a more expressive condition against. Exit code is the one signal every job already produces. Building a general expression language for a P4 (lowest-priority) capability, before any real step needs one, is exactly the kind of premature complexity the spec's own Assumptions and this workspace's conventions (single-script CLI, no framework-for-a-framework) argue against.

**Alternatives considered**:
- A boolean expression DSL over step stdout/stderr/exit code — rejected: no current job produces output structured enough to query, and it's substantially more implementation and validation surface for a capability nothing yet needs.
- No early-stop condition at all, `max_iterations` only — rejected as *too* minimal: FR-012 explicitly requires supporting an early-stop condition, and `until_exit_code` satisfies it with negligible added surface.

## Decision: Sequential, single-machine, single-threaded execution only

**Decision**: `auto orchestrate` runs steps one at a time, in a single process, on whichever machine invoked it. No parallel steps, no dispatch to other `machines.yaml` entries.

**Rationale**: Directly matches the spec's own Assumptions (parallel/fan-out and cross-machine orchestration are explicitly out of scope for v1) and every current job's `runs_on.machines` semantics already assume a job is invoked on one machine at a time — there is no existing remote-dispatch mechanism in `auto` to build on top of.

**Alternatives considered**: None seriously considered — this is a direct implementation of an explicit spec constraint, not an open design question.
