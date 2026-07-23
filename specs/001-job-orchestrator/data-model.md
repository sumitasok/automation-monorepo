# Data Model: Job Orchestrator

Three entities from the spec (Orchestration, Step, Orchestration Run), realized as: a YAML file format for the first two, and a SQLite schema for the third. Field names below are the authoritative names for `/speckit-tasks` and implementation — treat this file, not prose elsewhere, as the source of truth for shape.

## Orchestration (YAML file)

One file per orchestration, in `orchestrator/<id>.yaml`. The filename stem (without `.yaml`) is the orchestration's id used on the CLI (`auto orchestrate <id>`).

| Field | Type | Required | Default | Notes |
|---|---|---|---|---|
| `name` | string | no | filename stem | Human-readable label shown in `auto orchestrate` (bare) listing. If present, does not need to equal the filename — the filename stem is always the id used to invoke it. |
| `description` | string | no | `""` | Free text, shown in listing output. |
| `steps` | list of **Step** | yes, non-empty | — | Executed strictly in list order (FR-004). |

**Validation rules** (checked before any step executes — FR-006):
- File must parse as YAML and top-level must be a mapping containing `steps`.
- `steps` must be a non-empty list.
- Every step's `job` must resolve to a job id known to `load_jobs()` across all mounted packs (FR-005) — unknown job id is a validation error, not a runtime error (Edge Case: unknown job id).
- Every numeric control (`retry`, `timeout_seconds`, `wait_before_seconds`, `loop.max_iterations`) must be a positive integer if present; zero/negative is a validation error (Edge Case: invalid bound).
- `loop.until_exit_code`, if present, must be an integer (any sign allowed — exit codes can be used unconventionally by a job).

## Step (YAML, nested under `steps:`)

| Field | Type | Required | Default | Maps to |
|---|---|---|---|---|
| `job` | string | yes | — | Existing job id (FR-003), resolved via the same `load_jobs()` used by `auto run`/`auto list`. |
| `args` | list of string | no | `[]` | Passed through exactly as `auto run <job> -- <args...>` would (FR-003). |
| `ai` | string | no | `""` | Optional AI profile name, equivalent to `auto run <job> --ai <name>` (matches today's `--ai` flag; e.g. `deepseek`). |
| `retry` | integer > 0 | no | `0` (no retry) | Max additional attempts after the first failure (FR-008). A value of `2` means up to 3 total attempts. |
| `retry_delay_seconds` | integer ≥ 0 | no | `0` | Pause between a failed attempt and the next retry. Simple fixed delay — no backoff curve (kept minimal; not required by any FR, but needed to make retries usable against rate limits without inventing a bigger scheduling model). |
| `timeout_seconds` | integer > 0 | no | job's own `runtime.timeout_seconds` from its manifest | Per-step override, this invocation only — never mutates the job's manifest (FR-009). |
| `wait_before_seconds` | integer > 0 | no | `0` (no wait) | Pause before this step starts, satisfying both "wait before a step" and "wait between two steps" from FR-010 (a wait before step N *is* the wait between step N-1 and N). |
| `loop` | object | no | absent (run once) | See below (FR-011/FR-012). |

### Step.loop (nested)

| Field | Type | Required | Default | Notes |
|---|---|---|---|---|
| `max_iterations` | integer > 0 | yes, if `loop` present | — | Hard upper bound (FR-011); the loop never executes more than this many iterations, condition or not (Edge Case: condition never satisfied). |
| `until_exit_code` | integer | no | absent (no early stop) | If present, the loop stops after an iteration whose exit code equals this value, and that iteration is **not** treated as a failure (FR-012). See research.md "Loop stop-condition vocabulary" for why this is the only condition form in v1. |

**Interaction of retry and loop**: they compose per-iteration, not across the whole loop — each loop iteration gets its own fresh retry budget (a step's `retry` describes how many times *one attempt at running the job* is retried; `loop.max_iterations` describes how many times the whole retry-wrapped attempt is repeated). This keeps the two controls orthogonal and independently testable, matching how User Story 2 and User Story 4 are written as independent stories in the spec.

## Orchestration Run (SQLite: `data/state/orchestrations.sqlite`)

Mirrors the existing `runs.sqlite` pattern (`_record_run()`), extended with the run/step/attempt/iteration structure a single job run doesn't need.

```sql
CREATE TABLE IF NOT EXISTS orchestration_runs (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  ts            TEXT,     -- UTC ISO8601, run start
  orchestration TEXT,      -- orchestration id (filename stem)
  host          TEXT,      -- socket.gethostname(), matches _record_run()
  rc            INTEGER,   -- 0 = every step succeeded; else the failing step's rc
  dur           REAL       -- total wall-clock seconds for the whole run
);

CREATE TABLE IF NOT EXISTS orchestration_steps (
  run_id       INTEGER,   -- FK -> orchestration_runs.id
  step_index   INTEGER,   -- 0-based position in the orchestration's steps list
  job          TEXT,      -- job id for this step
  iteration    INTEGER,   -- 0-based loop iteration (always 0 for non-looping steps)
  attempt      INTEGER,   -- 0-based retry attempt within this iteration
  rc           INTEGER,   -- this attempt's exit code (124 = timed out, matching cmd_run's existing convention)
  dur          REAL,      -- this attempt's duration in seconds
  outcome      TEXT       -- one of: succeeded | failed | retried | timed_out | skipped | loop_stopped
);
```

**`outcome` values** (per FR-013's "succeeded, failed, retried — with attempt count, or skipped"):
- `succeeded` — attempt exited 0 (or matched `until_exit_code`, ending its loop early without being a failure).
- `failed` — attempt exited non-zero and no retry attempts remain, or the step doesn't retry.
- `retried` — attempt exited non-zero but a retry attempt follows.
- `timed_out` — attempt was killed for exceeding `timeout_seconds`; still subject to `retried`/`failed` classification above via `rc = 124`.
- `skipped` — step never ran because an earlier step already failed the run (FR-013's "skipped").
- `loop_stopped` — recorded once on the iteration where `until_exit_code` ended the loop early, distinguishing "loop ended by condition" from "loop ended by exhausting `max_iterations`" when reading history back (supports SC-005's "how many attempts each took, and where the run stopped").

**Read path**: an `auto orchestrate` run report (stdout) and any future `auto orchestrate history <name>` command both read from these two tables, joined on `run_id` — no other read path is needed for v1 (no dashboard integration required by the spec).
