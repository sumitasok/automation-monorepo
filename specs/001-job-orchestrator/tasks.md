---

description: "Task list for the Job Orchestrator feature"
---

# Tasks: Job Orchestrator

**Input**: Design documents from `/specs/001-job-orchestrator/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/cli.md, quickstart.md (all present)

**Tests**: Not requested in the spec, and this workspace has no automated test suite for `framework/tools/auto` today (see plan.md > Technical Context > Testing). Validation instead happens via the structural `validate_orchestration()` pass (itself a functional requirement, FR-006) and the manual scenarios in `quickstart.md`, referenced directly from the relevant tasks below instead of a generated test suite.

**Organization**: Tasks are grouped by user story (P1–P4 from spec.md) so each can be delivered and demoed independently.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1–US4)
- Nearly every task below touches the single file `framework/tools/auto` (this repo's one-script CLI convention — see plan.md > Structure Decision), so most tasks are **not** parallelizable with each other even across stories; only tasks touching `orchestrator/*.yaml` fixtures, `README.md`, or `Makefile` are independent of it.

## Path Conventions

Single project, in-place extension of the existing CLI (see plan.md > Project Structure):
- `framework/tools/auto` — the CLI script gaining new functions
- `orchestrator/` — new workspace-root directory holding pipeline YAML files
- `data/state/orchestrations.sqlite` — created at runtime by the code added below, not authored by hand

---

## Phase 1: Setup

**Purpose**: Create the new directory this feature's content lives in, before any code references it.

- [X] T001 Create the `orchestrator/` directory at the workspace root with `orchestrator/README.md` documenting the pipeline YAML schema (fields, types, defaults, validation rules) by summarizing `specs/001-job-orchestrator/data-model.md` — mirrors how every pack/job directory ships its own README today.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared building blocks every user story's step-execution logic depends on. All four tasks edit the same file (`framework/tools/auto`) and must be done in this order — no `[P]`.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T002 Extract the body of the existing `cmd_run()` in `framework/tools/auto` into a new function `execute_job(job_id: str, extra_args: list[str], ai: str = "") -> tuple[int, float]` (env precedence, pack config file linking, workdir resolution, `exec`-vs-`entrypoint` command building, `subprocess.run` with timeout, returning `(returncode, duration_seconds)` — do **not** call `_record_run()` or `sys.exit()` from inside it). Update `cmd_run()` to call `execute_job()` once, then call `_record_run()` and `sys.exit(rc)` itself exactly as it does today, so `auto run`'s behavior is byte-for-byte unchanged (research.md "Reuse `cmd_run`'s execution core").
- [X] T003 In `framework/tools/auto`, add `ORCH_DIR = WS / "orchestrator"` next to the existing `DATA`/`GEN`/`WORKLOG` constants, plus `load_orchestrations() -> list[dict]` (parse every `orchestrator/*.yaml`, attach `_id` = filename stem) and `load_orchestration(name: str) -> dict | None` (return the parsed dict for `orchestrator/<name>.yaml`, or `None` if the file doesn't exist) per the `Orchestration`/`Step` field tables in `specs/001-job-orchestrator/data-model.md`.
- [X] T004 In `framework/tools/auto`, add `validate_orchestration(orch: dict) -> list[str]` returning a list of human-readable problem strings (empty list = valid): top-level must have a non-empty `steps` list; every step's `job` must resolve via the existing `load_jobs()`; every present numeric control (`retry`, `retry_delay_seconds`≥0, `timeout_seconds`, `wait_before_seconds`, `loop.max_iterations`) must be a positive integer (zero/negative is an error, per data-model.md's Validation rules and the spec's Edge Cases); `loop.until_exit_code` if present must be an integer (any sign). This function does not execute anything — it only inspects the parsed structure, satisfying FR-006's "validate before any step runs."
- [X] T005 In `framework/tools/auto`, add the `orchestrations.sqlite` schema (`orchestration_runs`, `orchestration_steps` tables exactly as specified in `specs/001-job-orchestrator/data-model.md`) plus `_record_orchestration_run(orch_id, rc, dur) -> int` (returns the new `run_id`) and `_record_orchestration_step(run_id, step_index, job, iteration, attempt, rc, dur, outcome)`, both mirroring the existing `_record_run()`'s connect/execute/commit/close pattern and writing to `DATA / "state" / "orchestrations.sqlite"`.

**Checkpoint**: Foundation ready — `execute_job()`, the loaders, the validator, and history recording all exist and are unit-callable, even though nothing invokes them from the CLI yet.

---

## Phase 3: User Story 1 - Run a named multi-step pipeline in order (Priority: P1) 🎯 MVP

**Goal**: `./auto orchestrate gmail-wallet-sync` runs `gmail-extract` then `gmail-categorize` in order, spanning packs, with validation-before-execution and clear failure reporting. `./auto orchestrate` (no name) lists available pipelines.

**Independent Test**: Author `orchestrator/gmail-wallet-sync.yaml` with the two real steps from today's manual commands, run it, and confirm both ran in order with no manual step in between (spec.md US1 Independent Test).

### Implementation for User Story 1

- [X] T006 [US1] In `framework/tools/auto`, add an `orchestrate` subparser to the existing argparse setup (`name` as an **optional** positional argument, `nargs="?"`) wired to a new `cmd_orchestrate(args)`, and add `orchestrate` to the module's top-of-file `Usage:` docstring alongside `run`/`list`/etc.
- [X] T007 [US1] In `cmd_orchestrate()` (`framework/tools/auto`), implement **list mode** (`args.name` is falsy): call `load_orchestrations()` and print one line per orchestration (id, step count, description) in the format shown in `specs/001-job-orchestrator/contracts/cli.md`; print `no orchestrations yet — add a YAML file to orchestrator/` and exit `0` when none exist (depends on T003, T006).
- [X] T008 [US1] In `cmd_orchestrate()` (`framework/tools/auto`), implement the start of **run mode** (`args.name` given): call `load_orchestration(args.name)`, printing a clear "not found" error (naming the `orchestrator/` directory searched) and exiting non-zero if it's `None`; otherwise call `validate_orchestration()` and, if it returns any problems, print every one of them and `sys.exit(2)` before executing any step (FR-006, SC-003, contracts/cli.md's exit-code convention) (depends on T003, T004, T006).
- [X] T009 [US1] Extend `cmd_orchestrate()`'s run mode (`framework/tools/auto`) with the sequential step-execution loop: for each step in `steps` (in file order), call `execute_job(step["job"], step.get("args", []), step.get("ai", ""))` exactly once (retry/timeout/wait/loop fields are read but not yet acted on — that's US2–US4); on non-zero return, mark that step `failed` and every remaining step `skipped`, and stop the loop (FR-004, FR-007, Edge Case "step fails after retries exhausted" reduces to "step fails" with zero retries here) (depends on T002, T008).
- [X] T010 [US1] Wire history recording and reporting into the same loop (`framework/tools/auto`): call `_record_orchestration_step()` once per step attempt and `_record_orchestration_run()` once at the end (rc = 0 if every step succeeded, else the failing step's rc), then print the per-step summary table and exit with the code described in `specs/001-job-orchestrator/contracts/cli.md` (0 = all succeeded, else the failing step's rc or `1`) (depends on T005, T009).
- [X] T011 [P] [US1] Author `orchestrator/gmail-wallet-sync.yaml`: two steps, `gmail-extract` with `ai: deepseek`, then `gmail-categorize` with `ai: deepseek` and `args: ["--batch-size", "0"]` — the exact pipeline from `specs/001-job-orchestrator/quickstart.md` Scenario 1, replacing today's two manual `./auto run` invocations.
- [X] T012 [US1] Manually run `specs/001-job-orchestrator/quickstart.md` Scenarios 1, 2, and 7 (sequential run replaces the manual commands, a third cross-pack step added, and bare `./auto orchestrate` discoverability) plus Scenario 1's negative check (typo'd job id fails validation before anything runs); fix anything that doesn't match the expected behavior (depends on T010, T011).

**Checkpoint**: At this point, User Story 1 is fully functional and independently testable/demoable — this is the MVP.

---

## Phase 4: User Story 2 - Recover from transient step failures via retry and per-step timeout (Priority: P2)

**Goal**: A step can declare `retry`/`retry_delay_seconds` and `timeout_seconds` so one flaky attempt doesn't fail the whole pipeline, and a runaway step gets killed on schedule.

**Independent Test**: Configure a step to fail once and succeed on retry; confirm the orchestration continues without user intervention. Configure a shorter-than-actual `timeout_seconds`; confirm the step is stopped and counted as a failed attempt (spec.md US2 Independent Test).

### Implementation for User Story 2

- [X] T013 [US2] In `cmd_orchestrate()`'s step loop (`framework/tools/auto`), wrap the single `execute_job()` call from T009 in a retry loop: up to `step.get("retry", 0)` additional attempts on non-zero exit, sleeping `step.get("retry_delay_seconds", 0)` seconds between attempts; record each attempt's outcome as `retried` (more attempts remain) or `failed` (attempts exhausted) per `data-model.md`'s `outcome` values (depends on T010).
- [X] T014 [US2] In the same loop (`framework/tools/auto`), pass a per-step `timeout_seconds` override through to `execute_job()` when the step declares one (falling back to the job's own manifest `runtime.timeout_seconds` when unset, unchanged from today), marking an expired attempt's outcome `timed_out` with `rc = 124` — matching `cmd_run`'s existing timeout convention — before it's subjected to the same retry accounting from T013 (depends on T013). *(Note: `execute_job()` from T002 already accepts a timeout internally via the job's manifest; this task adds the override parameter/plumbing, it does not change `execute_job()`'s signature contract established in T002 beyond adding an optional timeout override.)*
- [X] T015 [P] [US2] Extend `orchestrator/gmail-wallet-sync.yaml` (or add a second fixture, e.g. `orchestrator/gmail-wallet-sync-retry-demo.yaml`) with `retry`, `retry_delay_seconds`, and a short `timeout_seconds` on one step, matching `specs/001-job-orchestrator/quickstart.md` Scenarios 3–4.
- [X] T016 [US2] Manually run `quickstart.md` Scenarios 3 and 4 (retry recovers from a forced one-time failure; a short timeout kills a slow step) and confirm the run report shows the right attempt counts and outcomes (depends on T014, T015).

**Checkpoint**: User Stories 1 AND 2 both work independently — retries/timeouts layer on top of US1 without changing its no-retry-declared behavior (FR-016).

---

## Phase 5: User Story 3 - Insert explicit waits between or before steps (Priority: P3)

**Goal**: A step can declare `wait_before_seconds` to pace execution deliberately, independent of retry/failure handling.

**Independent Test**: Declare a wait on a step; confirm the measured gap between the previous step finishing and this one starting matches the declared duration (spec.md US3 Independent Test).

### Implementation for User Story 3

- [X] T017 [US3] In `cmd_orchestrate()`'s step loop (`framework/tools/auto`), before starting a step's first iteration/attempt, sleep `step.get("wait_before_seconds", 0)` seconds (a no-op when unset, so US1/US2 behavior is unaffected) (depends on T014).
- [X] T018 [P] [US3] Add a `wait_before_seconds` example to an `orchestrator/*.yaml` fixture per `specs/001-job-orchestrator/quickstart.md` Scenario 5.
- [X] T019 [US3] Manually run `quickstart.md` Scenario 5 and confirm the measured gap is ≥ the declared wait, and ~0 when no wait is declared (depends on T017, T018).

**Checkpoint**: User Stories 1–3 all work independently.

---

## Phase 6: User Story 4 - Repeat a step until a bound is reached (Priority: P4)

**Goal**: A step can declare a bounded `loop` (`max_iterations` required, `until_exit_code` optional) so a batch job can be driven through multiple passes in one orchestration invocation.

**Independent Test**: Declare a loop with a maximum of N iterations; confirm the step runs at most N times (or fewer if `until_exit_code` is hit first), with a result recorded per iteration (spec.md US4 Independent Test).

### Implementation for User Story 4

- [X] T020 [US4] In `cmd_orchestrate()`'s step loop (`framework/tools/auto`), wrap the retry-wrapped execution from T013–T014 in an outer loop bounded by `step["loop"]["max_iterations"]` when `loop` is present (default: run once, i.e. today's behavior is unchanged when `loop` is absent); after each iteration whose exit code equals `step["loop"].get("until_exit_code")`, record that iteration's outcome as `loop_stopped` and end the loop early without treating it as a failure (research.md "Loop stop-condition vocabulary", data-model.md's `outcome` values).
- [X] T021 [P] [US4] Add a `loop: {max_iterations: 3}` example (no `until_exit_code` — see caveat below) to an `orchestrator/*.yaml` fixture per `specs/001-job-orchestrator/quickstart.md` Scenario 6.
- [X] T022 [US4] Manually run `quickstart.md` Scenario 6 with `max_iterations` only (no job in this workspace emits a distinguishing `until_exit_code` today — this is the caveat flagged in `specs/001-job-orchestrator/RUNBOOK.md`'s plan entry) and confirm the step runs exactly `max_iterations` times, never more; add a note to `orchestrator/README.md` (from T001) documenting that `until_exit_code` needs job-side support that doesn't exist yet for any current job (depends on T020, T021).

**Checkpoint**: All four user stories are independently functional — the full spec (FR-001 through FR-016) is implemented.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and whole-system validation after all stories are in place.

- [X] T023 [P] Add an `auto orchestrate` section to `README.md`'s `## Quickstart` (mirroring the existing `auto run`/`auto list` lines) and mention `orchestrator/` in its `## Where things are` section.
- [X] T024 [P] Add a `make orchestrate` convenience target to `Makefile` (`./auto orchestrate $(NAME)`), mirroring the existing `make run JOB=...` pattern.
- [X] T025 Run `./auto doctor` and confirm it still reports "OK — all manifests valid, no visibility leaks" — i.e. the `execute_job()` extraction (T002) introduced no regression to existing job manifests or visibility rules.
- [X] T026 Manually run `specs/001-job-orchestrator/quickstart.md` Scenario 8 (inspect `data/state/orchestrations.sqlite` after the earlier scenarios) and confirm the run/step history is complete and correct end-to-end, satisfying SC-005.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately.
- **Foundational (Phase 2)**: Depends on Setup (T001 creates `orchestrator/`, which T003's `load_orchestrations()` reads) — BLOCKS all user stories. T002 → T003 → T004 → T005, strictly sequential (same file).
- **User Story 1 (Phase 3)**: Depends on Foundational completion. T006 → T007/T008 (both need the subparser from T006, but T007 and T008 touch different branches of the same new function so do them in ID order, not in parallel) → T009 → T010 → T012 (needs T011's fixture).
- **User Story 2 (Phase 4)**: Depends on User Story 1 completion (extends the same step loop written in T009/T010) — not on US3/US4.
- **User Story 3 (Phase 5)**: Depends on User Story 2 completion (extends the same step loop again) — could be reordered before US2 with minor rework, but as written each phase edits the loop the previous phase left off, so do them in spec priority order (P1 → P2 → P3 → P4).
- **User Story 4 (Phase 6)**: Depends on User Story 3 completion (wraps the same step loop in an outer loop).
- **Polish (Phase 7)**: Depends on all four user stories being complete.

### Why these stories aren't independently parallelizable by different people

Unlike a typical multi-file feature, every story's implementation task edits the *same function* (`cmd_orchestrate()`'s step loop) in the *same file* (`framework/tools/auto`), because this repo deliberately keeps `auto` as one script (plan.md > Structure Decision). That means, despite the spec's stories being independently *testable*, the tasks that implement them must be done in sequence by whoever is editing that file — the parallelism available here is between an implementation task and its *fixture*/*doc* task (marked `[P]`), not between two implementation tasks.

### Parallel Opportunities

- T011 (US1 fixture) can be authored any time after T001 (needs only the `orchestrator/` directory and knowledge of job ids from `packs/gmail`) — in practice, in parallel with T006–T010's code changes, since it's a different file.
- T015, T018, T021 (fixture edits for US2/US3/US4) are each `[P]` relative to their story's code task for the same reason.
- T023 and T024 (Polish docs) are `[P]` relative to each other and to T025/T026 (different files: `README.md`, `Makefile` vs. running `auto doctor`/inspecting the sqlite file).

---

## Parallel Example: User Story 1

```bash
# T011 (fixture) can be written while T006-T010 (code) are in progress —
# different files, and T011's content is already fully specified by
# quickstart.md Scenario 1, independent of the code not existing yet:
Task: "Author orchestrator/gmail-wallet-sync.yaml per quickstart.md Scenario 1"

# T006-T010 (framework/tools/auto) must stay sequential — same function, same file.
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 (T001) and Phase 2 (T002–T005).
2. Complete Phase 3 (T006–T012) — this alone replaces today's two manual `./auto run` commands with one `./auto orchestrate gmail-wallet-sync`, spanning packs, with validation and clear failure reporting.
3. **STOP and VALIDATE**: run `quickstart.md` Scenarios 1, 2, 7 (already T012). This is a fully usable increment on its own — retry/timeout/wait/loop are refinements, not prerequisites, for daily use.

### Incremental Delivery

1. Setup + Foundational → foundation ready (execute_job/loaders/validator/history, nothing wired to the CLI yet).
2. Add US1 → validate via quickstart Scenarios 1/2/7 → this is the MVP (SC-001, SC-002, SC-003 all satisfied).
3. Add US2 → validate via Scenarios 3/4 (SC-004).
4. Add US3 → validate via Scenario 5.
5. Add US4 → validate via Scenario 6, with the `until_exit_code` limitation documented rather than faked (SC-006 is still met via `max_iterations` alone).
6. Polish → validate via `auto doctor` (T025) and Scenario 8 (T026, SC-005).

---

## Notes

- No `[Story]` label on Setup/Foundational/Polish tasks, per the task-format rules — only Phase 3–6 tasks carry `[US1]`–`[US4]`.
- Almost nothing here is `[P]` across implementation tasks because this feature is, by design (plan.md > Structure Decision), a set of sequential edits to one existing script rather than new independent modules — see "Why these stories aren't independently parallelizable" above. Don't force parallelism that would just create merge conflicts in `framework/tools/auto`.
- Commit after each task or logical group, per this workspace's own auto-commit convention (global CLAUDE.md Rule 1) — this task list doesn't repeat that instruction per task.
- Every fixture/doc task (`[P]`) cites the exact `quickstart.md` scenario it exists to satisfy, so there's no ambiguity about what "done" looks like for it.
