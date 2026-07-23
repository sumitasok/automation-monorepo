# orchestrator/

Multi-step pipelines that run existing `auto` jobs in order, potentially spanning
several packs. Each `*.yaml` file here is one named pipeline; the filename stem
is its id.

```
./auto orchestrate                  # list pipelines defined here
./auto orchestrate <name>            # validate, then run <name>.yaml's steps in order
```

See `specs/001-job-orchestrator/data-model.md` for the full field reference.
Summary:

## Orchestration (top level of the file)

| Field | Required | Notes |
|---|---|---|
| `name` | no | label shown in listing; defaults to the filename stem |
| `description` | no | shown in listing |
| `steps` | yes | non-empty list, executed strictly in order |

## Step (each entry under `steps:`)

| Field | Required | Default | Notes |
|---|---|---|---|
| `job` | yes | — | an existing job id, from any mounted pack (see `auto list`) |
| `args` | no | `[]` | passed through exactly like `auto run <job> -- <args...>` |
| `ai` | no | `""` | AI profile name, like `auto run <job> --ai <name>` |
| `retry` | no | `0` | extra attempts after a failure (`2` = up to 3 total attempts) |
| `retry_delay_seconds` | no | `0` | pause between a failed attempt and its retry |
| `timeout_seconds` | no | job's own manifest timeout | overrides it for this invocation only |
| `wait_before_seconds` | no | `0` | pause before this step starts |
| `loop.max_iterations` | required if `loop` present | — | hard bound; the step never runs more than this many times |
| `loop.until_exit_code` | no | — | loop stops early (not a failure) when an iteration exits with this code |

A step with none of `retry`/`timeout_seconds`/`wait_before_seconds`/`loop` set
behaves exactly like running that job directly with `auto run` — the
orchestrator adds no implicit defaults.

**Known limitation**: `loop.until_exit_code` requires the underlying job to
actually emit a distinguishing exit code for "nothing left to process" (or
whatever condition should stop the loop early). No job in this workspace does
that today, so loops are currently only usable bounded by `max_iterations`
alone until a job is updated to support it.

## Example

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

Run history (steps, attempts, iterations, outcomes) is recorded in
`data/state/orchestrations.sqlite` (git-ignored, same as `data/state/runs.sqlite`).
