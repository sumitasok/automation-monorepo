# Implementation Plan: One-Click Job Runner UI

**Branch**: `005-job-runner-ui` | **Date**: 2026-07-27 (rev. 2) | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/005-job-runner-ui/spec.md`

> **Revision 2 (2026-07-27)** — the clarification session added mandatory, validated `--data-dir` / `--config-dir` (FR-014–FR-021) and replaced the per-action AI dropdowns with one session-wide control (FR-007–FR-013). Rev. 1's design shipped and is green (36 tasks, 46 tests); this revision plans only the **delta** on top of it. Sections unchanged from rev. 1 are marked *(unchanged)*.

## Summary

Rev. 1 turned the read-only `auto serve` dashboard into a launcher: described buttons for every job/pipeline/safe command, per-run tmux sessions, audit-ID-tagged logs, live tailing. Rev. 2 adds two things: every action that performs workspace work must be told explicitly (and validly) where the data and config directories are or it refuses to run, and the AI profile becomes one session-wide control instead of a picker per job.

## Technical Context

*(unchanged from rev. 1)* — Python 3.14, stdlib only (`http.server`, `sqlite3`, `subprocess`, `uuid`, `json`), PyYAML as the one pre-existing dependency, tmux as an external binary (installed 2026-07-27). Storage: `data/state/runs.sqlite` + `logs/runs/*.log`. Tests: `python3 -m unittest` in `framework/tools/test_auto.py`.

**New constraint (rev. 2)**: `DATA` and the config directory are currently *module-level constants derived at import* (`DATA = WS / "data"`, `WS / "config" / …`). They must become values resolved from an explicit source and validated before any work — without breaking the read-only commands that legitimately need neither.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is still the unfilled template — no ratified principles. Falling back to this repo's observable conventions, as in rev. 1:

| Convention | Status |
|---|---|
| **Stdlib only** | **PASS** — no new dependency; validation is `pathlib` checks. |
| **Regenerate from source, never drift** | **PASS** — the action catalog and profile list stay per-request derived. |
| **Reuse existing code over parallel paths** | **PASS** — runs still re-enter the real CLI; the AI default rides the *existing* env-precedence in `execute_job` rather than adding override logic. |
| **Additive schema changes only** | **PASS** — `ui_runs` gains two nullable columns; existing rows and the untouched `runs` table stay readable. |
| **Localhost-only, compensating controls for a now-executable dashboard** | **PASS** — unchanged from rev. 1 / ADR 0018. |

**New tension — deliberate breaking change.** Rev. 2 removes an implicit default that every existing invocation relies on. This is the one place the plan knowingly breaks a working behaviour, at the user's explicit instruction ("without which the run should fail"). It is *not* waved through: §5 and §6 of research.md exist specifically to stop it silently breaking the scheduler and the `./auto` shim. **PASS with the mitigations tracked as tasks.**

## Project Structure

### Documentation (this feature)

```text
specs/005-job-runner-ui/
├── plan.md              # This file (rev. 2)
├── research.md          # Phase 0 — rev. 2 appends §11–§16
├── data-model.md        # Phase 1 — rev. 2 adds the directory pair + new columns
├── quickstart.md        # Phase 1 — rev. 2 adds scenarios 15–22
├── contracts/
│   └── dashboard-api.md # rev. 2 revises /api/actions and POST /api/runs
└── tasks.md             # Phase 2 (/speckit-tasks) — rev. 2 appends a new task block
```

### Source Code (repository root)

```text
framework/tools/
└── auto                 # all changes; new/edited sections:
                         #   § workspace dirs  — NEW: resolve + validate data/config dirs
                         #   § arg parsing     — extract --data-dir/--config-dir pre-argparse
                         #   § run store       — ui_runs gains data_dir/config_dir columns
                         #   § action catalog  — actions declare whether they need dirs
                         #   § _exec-run       — pass dirs + AI env through to the child CLI
                         #   § serve           — refuse to start on invalid dirs; single AI control
                         #   § schedule sync   — embed dirs in generated scheduler entries

framework/tools/test_auto.py   # new cases for validation, precedence, and the AI default

docs/adr/0019-explicit-workspace-directories.md   # NEW — records the breaking change
docs/adr/0018-dashboard-job-runner.md             # amended: AI control is now session-wide
```

**Structure Decision**: *(unchanged)* single project — everything lands in the existing `framework/tools/auto`, in the same commented-section style the file already uses.

## Complexity Tracking

| Deviation | Why needed | Simpler alternative rejected because |
|---|---|---|
| Removing an implicit default that currently works | User requirement FR-014/FR-018: a run against the wrong directory can corrupt shared CSV/SQLite data, and the failure is silent today | Keeping a fallback default (e.g. `WS/data` when unset) would mean nothing ever fails — the requirement would be satisfied only on paper |
| Mutable module-level `DATA`/`CONFIG_DIR` set after arg parsing | 5 call sites already read `DATA` as a global inside functions; threading a context object through every one is a far larger diff for no behavioural gain in a single-file script | Passing an explicit context parameter everywhere — larger blast radius, and inconsistent with how `WS`/`ORCH_DIR` already work |
