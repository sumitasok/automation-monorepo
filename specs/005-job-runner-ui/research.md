# Phase 0 Research: One-Click Job Runner UI

Decisions marked **[verified]** were tested against this machine before being written down, not assumed.

## 1. Making runs non-interactive inside a PTY — the central risk **[verified]**

- **Decision**: Launch the job's command with **stdin redirected from `/dev/null`** inside the tmux pane.
- **Rationale**: This is the crux of FR-015. Both packs gate their interactive flows on `isInteractive()`, which tests whether stdin is a character device (`os.Stdin.Stat()` / `ModeCharDevice`). tmux allocates a real PTY per pane, so a naive `tmux new-session "auto run gmail-categorize"` makes stdin a TTY, `isInteractive()` returns **true**, and the rule-capture / suggest-similar prompts would block forever on input no browser user can supply. Verified directly:

  | Launch form | stdin is a TTY? | `isInteractive()` |
  |---|---|---|
  | `tmux new-session -d -s X "probe"` | yes | **true** → would hang |
  | `tmux new-session -d -s X "probe < /dev/null"` | no | **false** → prompts skipped |

  Redirecting from `/dev/null` puts UI runs on exactly the same code path those features already take on an unattended cron run — the behavior spec 003 designed for — so nothing new has to be added to either pack.
- **Alternatives considered**: Passing an explicit `--non-interactive` flag to each job — rejected, no such flag exists and it would mean editing every pack. Not using tmux at all — rejected, the user explicitly required tmux and chose no fallback.

## 2. tmux session lifecycle **[verified]**

- **Decision**: One detached session per run, named `auto-<audit_id>`. Rely on tmux's default `remain-on-exit off` for teardown (FR-013); cancel via `tmux kill-session`; test liveness via `tmux has-session`.
- **Rationale**: Verified that a session created with `tmux new-session -d -s X "cmd"` disappears on its own the moment `cmd` exits — so FR-013/SC-006 need no reaper for the normal path. An explicit sweep still runs for the abnormal path (§5).
- **Alternatives considered**: `remain-on-exit on` plus manual cleanup — rejected; it keeps dead panes around and directly contradicts "each session should be deleted after the run".

## 3. Where the wrapper logic lives

- **Decision**: A new hidden subcommand, `auto _exec-run <audit-id>`, is what tmux actually launches. It reads the run's row from SQLite, executes the underlying action, prefixes and writes every output line to the run's log, and records the final status.
- **Rationale**: Keeps all logic in Python rather than a fragile shell pipeline inside a tmux argument string. A shell pipe (`cmd | prefixer >> log`) would also lose the job's real exit code (the pipeline reports the *last* command's status), requiring `PIPESTATUS`/`pipefail` gymnastics quoted through tmux. The wrapper owns the exit code directly.
- **Underscore prefix** marks it as internal — it is not advertised in the CLI usage block, because it is an implementation detail of the dashboard, not something a user should invoke.

## 4. Not forking the execution path

- **Decision**: `_exec-run` shells out to the workspace's own `auto run <job>` / `auto orchestrate <name>` / `auto <command>` rather than calling `execute_job()` in-process.
- **Rationale**: The spec's dependency note requires a UI-launched run to behave identically to a terminal-launched one. Re-invoking the real CLI guarantees that by construction — including `_record_run`'s existing history write, AI profile injection, pack config linking, and timeouts — instead of reimplementing them and drifting. It also means orchestrations get their existing retry/loop/wait semantics for free.
- **Alternatives considered**: Importing and calling `execute_job` directly — rejected; it would bypass `cmd_run`'s history recording and require duplicating orchestration control flow.

## 5. Status reconciliation for abnormally-ended runs

- **Decision**: A run is `running` only while its tmux session exists. On every runs-list query (and at dashboard start), any row still marked `running` whose session is gone and which has no recorded exit code is reconciled to `failed`, with a note that it ended abnormally.
- **Rationale**: Satisfies FR-016/SC-005 without a background thread. The normal path never needs this — `_exec-run` writes the final status itself; this only catches a killed session, a machine sleep, or a dashboard restart mid-run. Deriving liveness from tmux rather than a stored PID avoids the classic PID-reuse false positive.

## 6. Log format and live streaming

- **Decision**: One file per run at `logs/runs/<audit-id>.log`. Every line is written as `<ISO-8601 timestamp> <audit-id> <text>`. The browser polls a JSON endpoint with a byte offset (`?from=<n>`) roughly once a second and appends whatever is new.
- **Rationale**: The per-line audit ID is exactly what the user asked for and is what makes concurrent runs disambiguable when logs are grepped together (FR-025/SC-004). Byte-offset polling is the simplest thing that meets SC-003's 2-second bar on `http.server`, needs no new dependency, survives dropped connections, and works unchanged for a finished run (the same endpoint serves history). `logs/` and `*.log` are **already** in `.gitignore`, so FR-027 holds with no change.
- **Alternatives considered**: Server-Sent Events / WebSockets — rejected as more moving parts than a local single-user tool needs, and awkward on `ThreadingHTTPServer`. Streaming with `tmux pipe-pane` — rejected; capturing the wrapper's own stdout is more direct and avoids terminal escape sequences in the log.
- **Unbuffered output** is forced (`python3 -u`, `PYTHONUNBUFFERED=1`) so lines reach the log as they are produced rather than at process exit.

## 7. Action catalog and AI-profile applicability

> **Partially superseded by §11 (rev. 2).** The per-kind "AI profile?" column below described rev. 1's per-action dropdowns. The AI profile is now a single session-wide default injected via the environment, so it reaches every kind of action; a pipeline step's own `ai:` still wins. The `danger` column and the exclusion list remain accurate.

- **Decision**: Actions are derived per-request from `load_jobs()` and `load_orchestrations()`, plus a small hardcoded table of `auto` commands. Each action declares whether it accepts an AI profile and whether it is state-changing.

  | Kind | Source | AI profile? | Danger |
  |---|---|---|---|
  | job | `load_jobs()` | yes (`--ai`) | normal |
  | orchestration | `load_orchestrations()` | **no** | normal |
  | command: `list`, `packs`, `doctor`, `catalog`, `config <pack>` | fixed table | no | read-only |
  | command: `schedule sync`, `bootstrap` | fixed table | no | **confirm required** |

- **Rationale**: Jobs take `--ai` (`_extract_ai_flag` only applies it to `run`). Orchestrations deliberately do **not** — each step names its own `ai:` in the orchestration YAML, so offering a dropdown would imply an override that does not exist. This is precisely FR-010's "no misleading dropdown", and matches the confusion the user already hit when `auto orchestrate --ai=deepseek` was rejected by the CLI.
- **Excluded**: `auto new` (interactive scaffolding that prompts for every field — unusable non-interactively), `auto log` (needs a free-text message), `auto share` and `auto search` (need an argument). These are not "feasible as buttons" in the user's sense.

## 8. Real vs. example AI profiles

- **Decision**: Offer `config/ai/*.yaml` excluding anything matching `*.example.yaml`, and additionally validate each candidate loads with a non-empty `provider` and `api_key`; profiles failing validation are listed as unusable rather than silently dropped.
- **Rationale**: FR-008. The directory today holds `deepseek.yaml` (real) alongside `claude.example.yaml` / `deepseek.example.yaml` (templates whose `api_key` is a placeholder). Offering a template would produce a confusing auth failure at run time.
- **Only the profile name is ever sent to the browser or written to a log** (FR-011/SC-007) — the loaded credential values stay inside the run's subprocess environment, exactly as `auto run --ai` already does.

## 9. Security controls replacing "it's read-only so it needs no auth"

- **Decision**: Keep the `127.0.0.1` bind; require `POST` for every mutation; validate the `Host` header against a localhost allowlist; require an explicit `confirm` field in the POST body for state-changing actions.
- **Rationale**: ADR 0012 §4 justified having no auth *because* nothing could be triggered. That premise is now false, so the justification is re-derived rather than inherited. `Host` validation is the standard defense against DNS rebinding, where a malicious page resolves its own hostname to `127.0.0.1` and POSTs to a local service. `GET` stays free of side effects so no link, prefetch, or refresh can start a run (FR-029). The server-side `confirm` requirement means the gate cannot be bypassed by crafting a request directly — the UI dialog alone would be cosmetic.
- **Deliberately not added**: user accounts, passwords, or TLS — disproportionate for a single-operator local tool, and the user's model throughout this workspace is single-user-on-own-machine.

## 10. Preserving terminal behavior while emitting step progress

- **Decision**: Add a structured step marker to `execute_job`'s existing output, emitted **only** when the `AUTO_RUN_AUDIT_ID` environment variable is set (i.e. only under a UI-launched run).
- **Rationale**: FR-023 needs to show which step of an orchestration is executing. `execute_job` already prints a human line per step (`>> running <id> [<pack>] ...`), but parsing prose is brittle. A guarded machine-readable marker gives reliable step events while leaving ordinary terminal output byte-for-byte unchanged — which is what the "behaves the same as from the terminal" dependency requires.
- **Alternatives considered**: Parsing the existing `>> running` lines — rejected as fragile. A sidecar events file — rejected as more state to keep consistent when the log already carries the events in order.

---

# Phase 0 Research — Revision 2 (2026-07-27)

Decisions for the clarification-session delta: mandatory validated directories (FR-014–FR-021) and the single session-wide AI control (FR-007–FR-013). As in rev. 1, **[verified]** marks claims tested on this machine rather than assumed.

## 11. How the session-wide AI default reaches a run — and why per-step still wins **[verified]**

- **Decision**: The dashboard does **not** pass `--ai`. It injects the chosen profile's credential environment variables into the environment of the `_exec-run` child process. Nothing else changes.
- **Rationale**: `execute_job` already builds `env = {**cfg_env, **os.environ, **ai_env, …}`. Injecting the default into `os.environ` therefore gives exactly the semantics FR-009/FR-010 ask for, with **no new override logic at all**:

  | Situation | `ai_env` | Winner | Matches |
  |---|---|---|---|
  | Job, no profile of its own | `{}` | the injected session default | FR-009 |
  | Orchestration step declaring `ai:` | set from that step | the **step's** profile | FR-010 |

  Verified by reproducing the precedence expression directly: with no step `ai:` the session default wins; with one, the step wins. This also sidesteps the fact that `auto orchestrate` has no `--ai` flag — the reason the user's earlier `./auto orchestrate gmail-wallet-sync --ai=deepseek` was rejected — without adding one.
- **Alternatives considered**: Adding `--ai` to `orchestrate` — rejected; it would have to *override* per-step `ai:` values (contradicting FR-010) or be ignored (confusing). Passing `--ai` for jobs and env for pipelines — rejected as two mechanisms for one concept.
- **Consequence**: the run's log won't carry `execute_job`'s `(ai) profile 'X' loaded` line for a session-default job, because that line is gated on the `--ai` parameter. The profile is recorded on the run record instead (FR-009), and `_exec-run` writes an explicit header line naming it, so provenance is not lost.

## 12. Where `--data-dir` / `--config-dir` are parsed

- **Decision**: Extract both from `argv` **before** argparse, alongside the existing `_extract_ai_flag`, generalising that helper into one that pulls a set of `--name value` / `--name=value` options.
- **Rationale**: `run` takes `extra` as `nargs="*"` after a bare `--`. `_extract_ai_flag`'s docstring already records that combining `nargs='*'` with another named option on the same subparser has unreliable `--` handling across Python versions (broken on 3.10, fixed in 3.13) and that this script's shebang can't assume a version. The same hazard applies verbatim to two more options, so the same proven workaround is reused rather than re-litigated.
- **Alternatives considered**: Adding them to each subparser — rejected for the `--`/`nargs='*'` hazard above. A global `argparse` parent parser — same hazard, since the conflict is with `run`'s positional, not with where the option is declared.

## 13. Making `DATA` and the config directory resolved rather than derived

- **Decision**: Keep module-level names (`DATA`, and a new `CONFIG_DIR`) but assign them from a `resolve_workspace_dirs()` call early in `main()`, instead of deriving them at import. `pack_config_dir()` and `ai_profile_dir()` read `CONFIG_DIR`.
- **Rationale**: There are only 5 `DATA` reads and 2 config reads, all *inside functions*, so late assignment is safe and the diff stays small. Threading an explicit context object through every call site would be a far larger change with no behavioural gain in a single-file script whose other roots (`WS`, `ORCH_DIR`, `GEN`) are already module-level.
- **Alternatives considered**: A context object / class — rejected as disproportionate. Leaving `DATA` derived and validating separately — rejected: the derived value would still be silently used by any path that forgot to consult the validated one, which is precisely the bug class this feature exists to remove.

## 14. What "valid" means, precisely

- **Decision**: Both directories must exist and be directories. Additionally the **data** directory must contain `state/` and `config/`; the **config** directory must contain `ai/` **or** at least one mounted pack's subdirectory.
- **Rationale**: Chosen over a mere non-empty check in clarification, because a non-empty check accepts a home directory or an unrelated project — the realistic misconfiguration. These specific children are what the code actually reads: `DATA/state/*.sqlite` (run history), `DATA/config/expense-rules.yaml` (shared rules, per the gmail pack), `CONFIG_DIR/ai/<name>.yaml` (profiles), `CONFIG_DIR/<pack>/config.yaml` (pack config).
- **Accepted cost**: a legitimately fresh, uninitialised workspace fails validation until those subdirectories exist. Recorded in the spec's Assumptions as intended — the alternative is silently operating on an unprepared directory.

## 15. Which commands require the directories, and the scheduler consequence

- **Decision**: `run`, `orchestrate`, `serve`, and **`schedule sync`** require them. `list`, `packs`, `search`, `doctor`, `catalog`, `config`, `new`, `log`, `share`, `bootstrap` do not.
- **Rationale**: The first three do or launch workspace work (FR-014/FR-020). `schedule sync` is the non-obvious one: it *writes scheduler entries that will later run jobs*, so it must know the directories in order to embed them — see §16. The exempt set is exactly the read-only inspection group of FR-019 plus commands that touch neither directory; keeping them working is what lets an operator diagnose a bad configuration (FR-019).

## 16. Not silently breaking the scheduler and the shim **[verified]**

- **Finding**: `_auto_cmd()` currently emits `"<python> <auto> run <job_id>"` — no directories, and cron/launchd run it with a bare environment. The `./auto` shim exports only `AUTO_WORKSPACE`. So the moment the directories become mandatory, **every scheduled job on this machine would start failing**, and so would every bare `./auto run …`.
- **Decision**:
  1. `_auto_cmd()` must embed `--data-dir` / `--config-dir` (the validated values from the `schedule sync` invocation) into the generated entries, making them self-sufficient. Re-running `schedule sync` is therefore required after upgrading — called out in the ADR and RUNBOOK.
  2. The `./auto` shim is **deliberately left alone**. Having it auto-export `AUTO_DATA_DIR=$here/data` would mechanically satisfy the requirement while restoring exactly the implicit behaviour the feature removes — nothing would ever fail, and the requirement would hold only on paper. The operator sets the variables once in their shell profile, or passes the flags.
- **Rationale**: This is the difference between the change being real and being decorative. It is also the single most likely way to ship a regression, so it is planned explicitly rather than discovered later.

## 17. Recording the directories on each run

- **Decision**: Add nullable `data_dir` / `config_dir` columns to `ui_runs`; `_ui_runs_schema` adds them to new databases, and an idempotent `ALTER TABLE … ADD COLUMN` guarded by a `PRAGMA table_info` check upgrades an existing one.
- **Rationale**: FR-021/SC-014. Nullable, additive, and guarded means the rows written by rev. 1 (already on disk in `data/state/runs.sqlite`) keep loading — consistent with the repo's additive-schema-only convention.
