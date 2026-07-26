# Implementation Plan: One-Click Job Runner UI

**Branch**: `005-job-runner-ui` | **Date**: 2026-07-26 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/005-job-runner-ui/spec.md`

## Summary

Turn the existing read-only `auto serve` dashboard into a launcher: render every job, orchestration, and safe `auto` command as a described button; launch each one inside its own detached tmux session via a new hidden `auto _exec-run` wrapper that streams the run's output into an audit-ID-tagged log file and records status in SQLite; and add a Runs section that lists every run and streams its log live by polling from a byte offset. All additions are stdlib-only inside the existing single-file `framework/tools/auto`.

## Technical Context

**Language/Version**: Python 3.14 (existing `framework/tools/auto` CLI — a single stdlib script)

**Primary Dependencies**: Python stdlib only — `http.server` (already used by `cmd_serve`), `sqlite3` (already used by run history), `subprocess`, `uuid`, `json`. PyYAML is the module's one pre-existing dependency. **No new Python dependency.** External binary: `tmux` (3.7b confirmed installed).

**Storage**: `data/state/runs.sqlite` — the DB `_record_run` already writes to. This feature adds new tables alongside the existing `runs` table (never altering it). Run logs as plain files under `logs/runs/`.

**Testing**: `python3 -m unittest` — no test suite exists in this repo today for `auto`; this feature introduces the first, covering the pure//logic-level helpers (action catalog, audit IDs, log line prefixing, status reconciliation) plus a tmux round-trip integration test.

**Target Platform**: The operator's own machine (macOS/Linux), single user, `http://127.0.0.1:4321`.

**Project Type**: Single project — extends the existing workspace CLI; no new application, service, or package.

**Performance Goals**: Live log latency under 2s (SC-003) via ~1s client polling; dashboard interactions feel immediate at the scale of tens of concurrent runs and a few thousand log lines.

**Constraints**: Must not change how `auto run` / `auto orchestrate` behave when invoked from a terminal (a UI-launched run must take the identical execution path); must stay bound to `127.0.0.1`; must never render or log credential values; must not commit run logs.

**Scale/Scope**: 11 jobs, 1 orchestration, ~6 command actions today — all discovered dynamically, so the count grows with the workspace. A handful of concurrent runs.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is still the unfilled template — no ratified principles, so no project-specific gates. Falling back to this repo's observable, documented conventions (chiefly ADR 0012, which governs the very component being changed):

| Convention | Status |
|---|---|
| **Stdlib only** — "adding Flask/FastAPI for a single read-only page isn't justified" (ADR 0012 §2) | **PASS** — polling over `http.server`, no new dependency. tmux is an external binary the user explicitly requested, declared as a hard prerequisite. |
| **Regenerate from source, never drift** (ADR 0012 §3, ADR 0001) | **PASS** — the action catalog is derived live from `load_jobs()`/`load_orchestrations()` on every request, so new jobs appear with no registration step. |
| **Localhost-only, no auth** (ADR 0012 §4) | **PASS with amendment** — still `127.0.0.1`-bound, but the stated *justification* for needing no auth ("read-only, no POST handler") no longer holds. Compensating controls added: POST-only mutations, `Host` header validation against DNS rebinding, and a confirmation gate on machine-altering actions. |
| **Reuse existing code over adding parallel paths** | **PASS** — runs go through the existing `execute_job` / `cmd_orchestrate`; no second execution path. |
| **Additive schema changes only** | **PASS** — new SQLite tables; the existing `runs` table and `_record_run` are untouched. |

**ADR 0012 must be superseded** by a new ADR recording the read-only → executable transition. Tracked as a task, not a violation.

## Project Structure

### Documentation (this feature)

```text
specs/005-job-runner-ui/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit-tasks — not created here)
```

### Source Code (repository root)

```text
framework/tools/
└── auto                       # the single-file CLI — all changes live here, in new
                               # clearly-delimited sections mirroring how the
                               # orchestrator section was added:
                               #   § run store    — SQLite tables, audit IDs, status
                               #   § run logs     — log paths, line prefixing, tailing
                               #   § tmux runner  — session launch/liveness/kill/reap
                               #   § action catalog — jobs + orchestrations + commands
                               #   § _exec-run    — hidden in-tmux wrapper subcommand
                               #   § serve        — extended: POST handlers + JSON endpoints
                               #   § _dashboard_html — extended: buttons, runs list, live log

framework/tools/test_auto.py   # NEW — first tests for the CLI (stdlib unittest)

logs/runs/<audit-id>.log       # NEW at runtime — per-run logs (already git-ignored)

docs/adr/0018-dashboard-job-runner.md   # NEW — supersedes ADR 0012's read-only decision
```

**Structure Decision**: Single project — everything lands in the existing `framework/tools/auto`. This follows the file's own established pattern of adding a self-contained, commented section per capability (as the orchestrator did, spec 001). A separate module was rejected: `auto` is a standalone executable script with no package structure, so importing a sibling would need `sys.path` manipulation for no benefit to a single-consumer tool.

## Complexity Tracking

*No constitution violations requiring justification. The one deviation — the dashboard ceasing to be read-only — is the explicit purpose of the feature and is handled by superseding ADR 0012 with compensating controls (see Constitution Check).*
