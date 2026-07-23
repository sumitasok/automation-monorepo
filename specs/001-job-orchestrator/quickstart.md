# Quickstart: Job Orchestrator

Manual validation scenarios for this feature. Run these after implementation to confirm the feature works end-to-end — this workspace has no automated test suite (see plan.md > Technical Context > Testing), so this is the acceptance check.

## Prerequisites

- `packs/gmail` mounted with working `gmail-extract`/`gmail-categorize` jobs (already true in this workspace today).
- A `config/ai/deepseek.yaml` AI profile in place (already required to run today's manual commands).
- `orchestrator/` directory exists at the workspace root.

## Scenario 1 — P1: sequential run replaces the two manual commands

**Setup**: create `orchestrator/gmail-wallet-sync.yaml`:
```yaml
name: gmail-wallet-sync
description: Extract Gmail transactions, then categorize them.
steps:
  - job: gmail-extract
    ai: deepseek
  - job: gmail-categorize
    ai: deepseek
    args: ["--batch-size", "0"]
```

**Run**: `./auto orchestrate gmail-wallet-sync`

**Expected**: `gmail-extract` runs to completion first; only after it exits does `gmail-categorize` start. Final output shows both steps succeeded. Matches spec Acceptance Scenario US1.1 and SC-001.

**Negative check**: rename the first step's `job` to a typo (`gmail-extractt`). Re-run. Expected: validation error naming the unresolvable job id, exit non-zero, and neither step's process starts (SC-003, Edge Case "unknown job id").

## Scenario 2 — P1: multi-pack pipeline

**Setup**: add a third step to the same file referencing a job from a different pack (e.g. a `packs/wallet` job).

**Run**: `./auto orchestrate gmail-wallet-sync`

**Expected**: all three steps run in order in one invocation, without needing to pass `--pack` anywhere (SC-002, Acceptance Scenario US1.2).

## Scenario 3 — P2: retry recovers from a transient failure

**Setup**: temporarily point a step at a job known to fail once (or simulate by editing a job to exit 1 on its first invocation only, then restore it), with:
```yaml
  - job: gmail-categorize
    ai: deepseek
    retry: 2
    retry_delay_seconds: 5
```

**Run**: `./auto orchestrate gmail-wallet-sync`

**Expected**: the run report shows the step succeeded after 1 retry; the orchestration proceeds to any later steps; nothing required a manual re-run (SC-004, Acceptance Scenario US2.1).

**Exhaustion check**: force every attempt to fail. Expected: run stops, reports the step failed after N attempts, exit code non-zero (Acceptance Scenario US2.2).

## Scenario 4 — P2: timeout stops a slow step

**Setup**: `timeout_seconds: 5` on a step that normally takes longer.

**Run**: `./auto orchestrate gmail-wallet-sync`

**Expected**: the step is killed at ~5s, recorded as `timed_out` (rc 124), and retried if attempts remain or the run fails if not (Acceptance Scenario US2.3).

## Scenario 5 — P3: wait between steps

**Setup**: `wait_before_seconds: 10` on the second step.

**Run**: `./auto orchestrate gmail-wallet-sync`, timing the gap between the first step's process exiting and the second's starting (visible in the printed per-step timestamps).

**Expected**: gap is ≥10s. Removing the field: gap is ~0s (Acceptance Scenarios US3.1/US3.2).

## Scenario 6 — P4: bounded loop

**Setup**:
```yaml
  - job: gmail-categorize
    ai: deepseek
    args: ["--batch-size", "50"]
    loop:
      max_iterations: 3
      until_exit_code: 2   # convention: job exits 2 when nothing left to categorize
```

**Run**: `./auto orchestrate gmail-wallet-sync`

**Expected**: the step runs up to 3 times; if an iteration exits `2`, the loop stops early (marked `loop_stopped`, not a failure) and the orchestration proceeds; if no iteration exits `2`, it stops after exactly 3 iterations regardless (SC-006, Acceptance Scenarios US4.1-3).

## Scenario 7 — discoverability

**Run**: `./auto orchestrate` (no argument)

**Expected**: lists `gmail-wallet-sync` with its step count and description, the same way `./auto packs`/`./auto list` already list their respective things (FR-015).

## Scenario 8 — history is inspectable after the fact

**Run**: after Scenario 1, inspect `data/state/orchestrations.sqlite` (e.g. `sqlite3 data/state/orchestrations.sqlite "select * from orchestration_runs; select * from orchestration_steps;"`).

**Expected**: one `orchestration_runs` row for the invocation, one `orchestration_steps` row per step showing its outcome — readable without opening any per-job log (SC-005).
