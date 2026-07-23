# CLI Contract: `auto orchestrate`

This is the interface contract for the feature — the only interface it exposes is a command added to the existing `auto` CLI (`framework/tools/auto`), plus the YAML file format it reads (fully specified in [data-model.md](../data-model.md)). There is no network/API surface.

## `auto orchestrate` (no argument) — list

**Input**: none.

**Behavior**: Scans the `orchestrator/` directory for `*.yaml` files, parses each far enough to read `name`/`description`/`steps` count (does **not** perform full job-id validation — that only happens on run, per FR-006 tying validation to execution). Prints one line per orchestration.

**Output** (stdout, human-readable, mirrors `auto list`'s style):
```
gmail-wallet-sync   2 steps   Gmail extract + categorize, then wallet sync
```

**Exit code**: `0`. If `orchestrator/` doesn't exist or is empty: prints `no orchestrations yet — add a YAML file to orchestrator/` and exits `0` (matches `auto list`'s "no jobs yet" convention, not an error).

## `auto orchestrate <name>` — run

**Input**: `name` — the filename stem of a file in `orchestrator/<name>.yaml`.

**Behavior**:
1. Resolve `orchestrator/<name>.yaml`. If missing: print a clear "not found" error listing the orchestrator directory searched, exit non-zero. No step runs.
2. Parse and validate the whole file per the rules in [data-model.md](../data-model.md) (structure, every `job` resolves via `load_jobs()`, all numeric bounds positive). If validation fails: print every problem found, exit non-zero. No step runs (FR-006, SC-003).
3. Execute steps strictly in order (FR-004). For each step:
   - If `wait_before_seconds` is set, sleep that long first.
   - For each loop iteration (1 if no `loop`, up to `loop.max_iterations` otherwise): run the job via the shared `execute_job()` core (same code path as `auto run`), retrying up to `retry` additional times on non-zero exit (waiting `retry_delay_seconds` between attempts). If `loop.until_exit_code` matches the iteration's exit code, stop looping after recording that iteration — it does not count as a failure.
   - Record every attempt/iteration to `orchestration_steps` (see data-model.md).
   - If the step's final outcome (after all retries/iterations) is a failure, mark all remaining steps `skipped`, stop the run.
4. Record the whole run to `orchestration_runs`.
5. Print a per-step summary table (step, job, iterations, attempts, outcome) and the overall result.

**Exit code**:
- `0` — every step succeeded (or ended via `until_exit_code`, which is a success).
- Non-zero, matching the failing step's last exit code where available (falls back to `1`) — a step failed after exhausting retries/loop.
- `2` (reserved, matches `auto doctor`'s failure convention) — validation failed before any step ran. This is distinguishable from a step failure so scripts/CI wrapping `auto orchestrate` can tell "bad pipeline definition" apart from "pipeline ran and a step failed."

**Side effects**: identical to whatever the invoked jobs themselves do (writes to `transactions.csv`, etc. per each job's own manifest `data.writes`) — the orchestrator itself only writes to `data/state/orchestrations.sqlite`.

## Compatibility notes

- A step with none of `retry`, `timeout_seconds`, `wait_before_seconds`, `loop` set behaves exactly as `auto run <job> -- <args>` (`--ai <ai>` if set) does today — same env precedence, same config linking, same manifest timeout, same run recording call for the underlying job (FR-016). This is a direct consequence of both commands calling the same `execute_job()` function (see research.md).
- `auto orchestrate` does not accept `--` passthrough args itself (unlike `auto run <id> -- args`) — every argument a step needs is declared in its `args:` list in the YAML, per FR-014 ("fully configurable from the YAML file alone").
- Reserved word: none. An orchestration cannot be named in a way that collides with the bare `auto orchestrate` listing form, because listing is triggered by the *absence* of a name argument, not by a specific string (see research.md's CLI-shape decision).
