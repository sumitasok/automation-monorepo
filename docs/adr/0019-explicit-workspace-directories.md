# ADR 0019 — `--data-dir` / `--config-dir` are required and validated

**Status:** accepted — 2026-07-27
**Amends:** [ADR 0018](0018-dashboard-job-runner.md) (the AI profile control is now session-wide, not per-action)

## Context

`auto` derived both of its roots from wherever the workspace happened to sit on disk:

```python
DATA = WS / "data"          # WS = nearest ancestor containing packs.yaml
… WS / "config" / <pack>    # pack config
… WS / "config" / "ai"      # AI profiles
```

That is convenient and silently wrong in the cases that matter. `WS` falls back to a guess when `packs.yaml` isn't found; a job run from an unexpected directory reads an empty or foreign data set and reports success on nothing; and several jobs (gmail, expenses, wallet) read-modify-write the *same* shared CSV and SQLite files, so operating on the wrong copy can destroy real records. Nothing failed loudly — the failure mode was a confidently empty result or a corrupted file.

The operator asked for the opposite default: *"make all the commands request `--data-dir` without which the run should fail … but either should have a valid path with files in it."*

## Decision

1. **Both directories must be supplied explicitly**, by `--data-dir` / `--config-dir` or by `AUTO_DATA_DIR` / `AUTO_CONFIG_DIR`. Neither source is privileged for validity; the explicit option wins when both are given. There is **no fallback** — that is the entire point.

2. **Validation is structural, not "non-empty".** The data directory must contain `state/` and `config/`; the config directory must contain `ai/` or at least one mounted pack's directory. A non-empty check was considered and rejected: it accepts a home directory, an unrelated project, or a half-prepared workspace — precisely the realistic misconfigurations. These specific children are the paths the code actually reads (`DATA/state/*.sqlite`, `DATA/config/expense-rules.yaml`, `CONFIG_DIR/ai/<name>.yaml`, `CONFIG_DIR/<pack>/config.yaml`).

   *Accepted cost*: a genuinely fresh, uninitialised workspace fails until those subdirectories exist. Better than silently operating on an unprepared directory.

3. **Only commands that do real work require them**: `run`, `orchestrate`, `serve`, `schedule`, and the internal `_exec-run`. The read-only inspection commands — `list`, `packs`, `search`, `doctor`, `catalog`, `config`, `new`, `log`, `share`, `bootstrap` — never resolve them and keep working. This is deliberate: those are exactly the commands you need in order to *diagnose* a bad configuration, so making them fail too would be self-defeating.

   `schedule` is in the required set even though it is not itself a "run": the entries it writes must embed the directories to be self-sufficient (see 5).

4. **Failures are refusals, not partial work.** Resolution happens in `main()` before any command body executes, so nothing is read or written when it fails. The message names the offending path, what was expected of it, and how to supply a correct one.

5. **Generated scheduler entries embed the directories.** `_auto_cmd()` now emits `run <job> --data-dir … --config-dir …`. Without this, every cron/launchd entry would break the moment this change shipped — the scheduler runs with a bare environment, so entries relying on an exported variable would fail every time. **Re-run `auto schedule sync` after upgrading.**

6. **The `./auto` shim is deliberately not changed.** Having it export `AUTO_DATA_DIR="$here/data"` would mechanically satisfy the requirement while restoring exactly the implicit behaviour being removed — nothing would ever fail and the decision would be decorative. Operators set the variables once in their shell profile, or pass the flags.

7. **`auto serve` refuses to start** on invalid directories rather than starting degraded. This differs from the tmux-missing case (which starts and disables the buttons) because a dashboard with no trustworthy data directory has nothing worth showing, whereas one without tmux is still a useful read-only view.

8. **Each run records the directories it used** (`ui_runs.data_dir` / `.config_dir`, nullable, added by a guarded additive migration so rows written before this change keep loading).

## Amendment to ADR 0018 — the AI profile control

ADR 0018 gave each job action its own AI profile dropdown and deliberately gave pipelines none, reasoning that `auto orchestrate` has no `--ai` flag and each step names its own `ai:`. That is replaced by **one session-wide control**, applying to everything launched from the dashboard.

The mechanism matters: the dashboard **injects the profile's credential environment variables** into the run's process instead of passing `--ai`. `execute_job` already builds `env = {**cfg_env, **os.environ, **ai_env, …}`, so:

- a job with no profile of its own picks up the injected session default;
- an orchestration step declaring `ai:` overrides it, because `ai_env` is applied last.

"Session default, per-step override" therefore falls out of the *existing* precedence chain with no new override logic, and without adding an `--ai` flag to `orchestrate` that would have had to either contradict per-step configuration or be silently ignored. The selection is process-lifetime only and is re-validated at launch, so a profile deleted or broken after selection refuses the run rather than starting it with half-resolved credentials.

## Consequences

- **This is a breaking change.** Bare `./auto run <job>` no longer works. Scheduled entries must be regenerated. This was the explicit instruction, and it is the difference between the guard being real and being decorative.
- Misconfiguration now fails in under a second with a message naming the path, instead of producing an empty result or corrupting a shared file.
- Diagnosis still works when configuration is broken, because the inspection commands are exempt.
- `DATA` and `CONFIG_DIR` are now assigned after argument parsing rather than at import. Any future code path that reads them without having resolved them first is a bug — the module-level values are conventional placeholders, not authority.
- The test fixture had to gain `data/config/` to model a real workspace; its absence made every dashboard-run test hang, which is exactly how a half-prepared real workspace behaves.
