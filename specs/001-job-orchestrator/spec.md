# Feature Specification: Job Orchestrator

**Feature Branch**: `[001-job-orchestrator]`

**Created**: 2026-07-23

**Status**: Draft

**Input**: User description: "Right now I run `./auto run gmail-extract -- --ai=deepseek` then `./auto run gmail-categorize -- --ai=deepseek --batch-size 0`. I want to create an orchestrator directory where I can add gmail-wallet-sync.yaml, then run `./auto orchestrate gmail-wallet-sync`. The yaml should define how these tasks run, one after another. Make this orchestrator a spec for your reference, because we need to add many capabilities to this orchestrator like loop, wait, retry, timeout. This orchestrator will allow us to batch processes spanning multiple packs. This is fully user controllable."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run a named multi-step pipeline in order (Priority: P1)

Today the user manually runs `gmail-extract` then `gmail-categorize` as two separate commands, remembering the order and flags each time. Instead, the user wants to describe that sequence once in a file and trigger it with a single named command, spanning jobs from any mounted pack (not just one).

**Why this priority**: This is the entire reason the feature exists — without ordered, hands-off execution of a declared sequence, there is no orchestrator, just a naming convention. Every other capability (retry, wait, loop, timeout) is a refinement of this core loop.

**Independent Test**: Create an orchestration file listing `gmail-extract` (with `--ai=deepseek`) followed by `gmail-categorize` (with `--ai=deepseek --batch-size 0`), invoke it by name, and confirm both jobs ran in that order with their declared arguments, with no manual step in between.

**Acceptance Scenarios**:

1. **Given** an orchestration file declaring steps A then B, **When** the user runs it by name, **Then** step A completes fully before step B starts, and both run with the arguments declared in the file.
2. **Given** an orchestration file whose steps reference jobs from two different packs, **When** the user runs it, **Then** both steps execute successfully in one invocation without the user needing to know which pack each job lives in.
3. **Given** step A fails, **When** the orchestration is running, **Then** step B does not start, and the run is reported as failed at step A.
4. **Given** the orchestration file lists an unknown job id, **When** the user runs it, **Then** the system reports the invalid reference before executing any step (no partial run against real data).

---

### User Story 2 - Recover from transient step failures via retry and per-step timeout (Priority: P2)

Steps like `gmail-categorize` call an external AI provider and can fail transiently (rate limits, network blips). The user wants to declare how many times a failed step should be retried, and how long a step is allowed to run, before the whole pipeline is treated as failed — without having to re-run the entire pipeline by hand for a one-off hiccup.

**Why this priority**: Once ordered execution exists, the next biggest source of manual babysitting is a pipeline that dies on a single flaky step. This directly reduces that toil, and is a prerequisite building block for the wait/loop capabilities that follow.

**Independent Test**: Configure a step with a retry count, force that step to fail on its first attempt and succeed on a later attempt, and confirm the orchestration continues to the next step without user intervention. Separately, configure a step with a timeout shorter than the job would naturally take, and confirm the step is stopped and counted as a failed attempt.

**Acceptance Scenarios**:

1. **Given** a step declares up to N retry attempts, **When** the step fails on attempt 1 but succeeds on attempt 2, **Then** the orchestration proceeds to the next step and the run report shows the step succeeded after 1 retry.
2. **Given** a step declares up to N retry attempts, **When** every attempt fails, **Then** the orchestration stops and reports the step as failed after N attempts.
3. **Given** a step declares a timeout shorter than its actual run time, **When** the step exceeds that timeout, **Then** the step is stopped, counted as a failed attempt, and retried (if attempts remain) or the run fails (if not).
4. **Given** a step declares no retry or timeout, **When** it runs, **Then** it behaves exactly as `auto run` does today for that job (no surprise new default limits).

---

### User Story 3 - Insert explicit waits between or before steps (Priority: P3)

Some sequences need a deliberate pause — e.g. waiting a fixed duration before the next step, or waiting until a downstream system is likely ready — rather than retrying a failure. The user wants to declare a wait as an explicit part of the pipeline.

**Why this priority**: Useful for pacing batch/rate-limited work across packs, but the pipeline is already usable and safe (via P1/P2) without it — this refines timing control rather than enabling a new class of work.

**Independent Test**: Configure an orchestration with a declared wait between two steps, run it, and confirm the measured gap between step A finishing and step B starting matches the declared wait.

**Acceptance Scenarios**:

1. **Given** an orchestration declares a wait of duration D before a step, **When** the orchestration runs, **Then** the system pauses for at least D before starting that step.
2. **Given** an orchestration declares no wait for a step, **When** it runs, **Then** the next step starts immediately after the previous one finishes (no implicit delay).

---

### User Story 4 - Repeat a step until a bound is reached (Priority: P4)

Some jobs are naturally batched (e.g. `gmail-categorize --batch-size 0` processing everything in one pass today, but a future batch size might process a fixed chunk per call). The user wants a step or group of steps to repeat — up to a declared maximum number of iterations — so a pipeline can drive multiple batches of the same job without the user re-invoking the orchestrator each time.

**Why this priority**: Highest-value once the simpler sequencing, resilience, and pacing capabilities exist; looping compounds with all three, so it depends on them rather than blocking them.

**Independent Test**: Configure a step to loop up to N times, run the orchestration, and confirm the step executes N times (or fewer if a stated stop condition is met earlier) and the run report lists each iteration's outcome.

**Acceptance Scenarios**:

1. **Given** a step declares a loop with a maximum of N iterations, **When** the orchestration runs, **Then** the step executes at most N times and the run report shows a result for each iteration.
2. **Given** a looping step declares a stop condition that becomes true before reaching the maximum, **When** that condition is met, **Then** the loop ends early and the orchestration proceeds to the next step.
3. **Given** a looping step never satisfies its stop condition, **When** it reaches the declared maximum iterations, **Then** the loop ends at that maximum rather than running indefinitely.

---

### Edge Cases

- What happens when the named orchestration file does not exist in the orchestrator directory? System reports a clear "not found" error and does not attempt to run anything.
- What happens when the orchestration YAML is malformed or missing required fields (e.g. a step with no job id)? System reports a validation error before executing any step.
- What happens when two steps in the same file reference the same job id with different arguments (e.g. run `gmail-categorize` twice with different batch sizes)? This must be supported — steps are independent entries, not deduplicated by job id.
- What happens if the same orchestration is invoked twice concurrently? Each invocation runs independently; the system does not need to prevent concurrent runs of the same or different orchestrations, but must not corrupt shared run-history records when it happens.
- What happens when a step's job itself is scheduled (has `schedule.enabled: true`) and also invoked via orchestration? Orchestration invokes the job directly, on the same terms as `auto run`, independent of its own schedule.
- What happens when a retry, wait, or loop bound is declared as zero or negative? System treats it as invalid configuration and reports it at validation time, not at execution time.
- What happens when a step in the middle of the pipeline partially wrote data before failing (e.g. `gmail-extract` wrote half a batch to `transactions.csv`)? Orchestration does not attempt to undo a step's side effects — recovery from partial writes is the responsibility of the underlying job (as it already is for a manual `auto run` failure today).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST let users define a named orchestration as a single YAML file placed in a dedicated orchestrator directory, independent of any individual pack.
- **FR-002**: System MUST provide a command that runs a named orchestration by resolving it to its YAML file in the orchestrator directory.
- **FR-003**: Each orchestration file MUST declare an ordered list of steps, where each step references an existing job id (from any mounted pack) plus any arguments/flags to pass to that job.
- **FR-004**: System MUST execute an orchestration's steps strictly in the order declared in the file, completing each step before starting the next.
- **FR-005**: System MUST allow a single orchestration's steps to span jobs from more than one pack in the same run.
- **FR-006**: System MUST validate an orchestration file (structure, required fields, and that every referenced job id exists) before executing any of its steps, and MUST report validation errors clearly without partially running the pipeline.
- **FR-007**: System MUST stop an orchestration run at the first step that fails after all its retry attempts (if any) are exhausted, and MUST report which step failed.
- **FR-008**: System MUST let each step optionally declare a maximum retry count; a step is only considered failed once that many attempts have all failed.
- **FR-009**: System MUST let each step optionally declare a timeout; a step exceeding it is stopped and treated as a failed attempt (subject to that step's retry count).
- **FR-010**: System MUST let an orchestration optionally declare an explicit wait before a given step or between two steps.
- **FR-011**: System MUST let a step optionally declare a loop with a required maximum iteration count, so repeated execution of that step is always bounded.
- **FR-012**: System MUST let a looping step optionally declare an early stop condition, evaluated after each iteration, that ends the loop before its maximum count is reached.
- **FR-013**: System MUST record, for every orchestration run, which steps executed, each step's outcome (succeeded, failed, retried — with attempt count, or skipped because an earlier step failed), and the run's overall outcome.
- **FR-014**: System MUST make every orchestration behavior — step order, arguments, retries, timeouts, waits, and loop bounds — configurable from the YAML file alone, with no code changes required to add or modify a pipeline.
- **FR-015**: System MUST let users list the orchestrations available in the orchestrator directory, so pipelines are discoverable the same way jobs already are.
- **FR-016**: A step declaring none of retry, timeout, wait, or loop MUST behave exactly as running that job directly (e.g. via the existing single-job run command) — the orchestrator MUST NOT impose new implicit defaults for these controls.

### Key Entities

- **Orchestration**: A named, user-authored pipeline definition (one YAML file in the orchestrator directory). Holds an ordered list of steps and any pipeline-level defaults.
- **Step**: One entry within an orchestration. References a job id, the arguments to pass it, and optional per-step controls: retry count, timeout, wait-before, and loop (max iterations plus optional stop condition).
- **Orchestration Run**: One execution of a named orchestration. Holds the outcome of each step it reached (including retry attempts and loop iterations) and the run's overall success/failure state.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can turn today's two manual commands into a single named pipeline invocation, with zero manual steps between the first job finishing and the second starting.
- **SC-002**: Adding a new pipeline that spans multiple packs requires writing exactly one YAML file — no changes to any existing job, pack, or the orchestrator's own code.
- **SC-003**: When a pipeline is misconfigured (unknown job, malformed file, invalid bound), the user sees a clear error before any step runs, so an invalid pipeline never produces partial side effects.
- **SC-004**: A single transient failure in one step that succeeds on retry no longer requires the user to notice the failure and manually re-run the whole pipeline.
- **SC-005**: Given a completed orchestration run, the user can tell — without reading any single job's internal logs — which steps ran, in what order, how many attempts each took, and where (if anywhere) the run stopped.
- **SC-006**: A user can bound a repeated step to a fixed maximum number of iterations, and the pipeline never runs that step beyond that bound even if its stop condition never becomes true.

## Assumptions

- The orchestrator directory sits at the workspace root (sibling to `packs/`, `schedules/`, `config/`), consistent with how other workspace-level, pack-spanning concerns are already organized.
- V1 orchestrations run their steps sequentially only, on a single machine per invocation; parallel/fan-out step execution and cross-machine orchestration are out of scope for this spec and are candidate future capabilities.
- An orchestration step invokes an existing job exactly as `auto run` does today (same visibility rules, same config/AI-profile resolution) — the orchestrator introduces no new job type or execution mechanism, only a sequencing/control layer on top of the existing one.
- Default behavior when retry/timeout/wait/loop are omitted from a step matches today's single-job `auto run` behavior exactly (fail-fast, no implicit retry, job's own manifest timeout applies) — see FR-016.
- Run history for orchestrations follows the same spirit as existing per-job run recording, extended to also capture which orchestration and which step a run/attempt belongs to.
- The orchestrator does not attempt to roll back or undo a step's partial side effects on failure; that remains the responsibility of the underlying job, matching today's behavior for a manually failed `auto run`.
- Concurrent runs (same or different orchestrations at once) are permitted; the only guarantee required is that run-history recording is not corrupted by concurrency, not that runs are serialized.
