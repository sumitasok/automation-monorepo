---

description: "Task list for feature implementation"
---

# Tasks: One-Click Job Runner UI

**Input**: Design documents from `/specs/005-job-runner-ui/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/dashboard-api.md, quickstart.md (all present)

**Tests**: This feature introduces the first tests for `framework/tools/auto` (none exist today). They are written alongside each unit of logic, matching this repo's convention in the Go packs (tests beside the change, not a separate TDD gate).

**Organization**: Grouped by user story (US1/US2 = P1, US3/US4 = P2) so each is independently deliverable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable (different file or independent region, no ordering dependency)
- **[Story]**: US1–US4 per spec.md
- Nearly all work lands in the single file `framework/tools/auto`, so tasks touching it are mostly **sequential by necessity**; parallel markers are used only where a task touches a genuinely different file or an isolated new section.

---

## Phase 1: Setup

- [X] T001 Verify `tmux` is present and record its version (`tmux -V`); confirm `logs/` and `*.log` are already covered by `.gitignore` so run logs are never committed (FR-027).
- [X] T002 Create `framework/tools/test_auto.py` with a stdlib `unittest` skeleton that imports the `auto` script by path (it has no `.py` extension, so use `importlib.util.spec_from_file_location`), plus a shared helper that points `AUTO_WORKSPACE` at a temporary throwaway workspace so tests never touch the real `data/state/runs.sqlite`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The run store, log plumbing, and tmux control that every user story builds on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T003 In `framework/tools/auto`, add a `# ---------- run store ----------` section: `_runs_db()` (reuse `DATA/"state"/"runs.sqlite"`), `_ui_runs_schema(conn)` creating `ui_runs` and `ui_run_steps` exactly per data-model.md, and `new_audit_id()` producing `r-<YYYYMMDD>-<HHMMSS>-<6 hex>`. The pre-existing `runs` table and `_record_run` must not be touched.
- [X] T004 [P] Add tests in `framework/tools/test_auto.py` for `new_audit_id()` (format, chronological string sort, uniqueness across rapid calls) and for schema creation being idempotent (running it twice is safe, and it leaves an existing `runs` table intact).
- [X] T005 In `framework/tools/auto`, add run-record CRUD to the run store section: `create_run(...)`, `finish_run(audit_id, rc, status, note=None)`, `get_run(audit_id)`, `list_runs(limit)`, `set_run_status(...)`. Statuses per data-model.md's transition diagram; `rc == 124` maps to `timed_out` to match `execute_job`'s existing timeout convention.
- [X] T006 Add a `# ---------- run logs ----------` section to `framework/tools/auto`: `run_log_path(audit_id)` → `logs/runs/<audit_id>.log` (creating the directory), `format_log_line(audit_id, text)` → `<ISO-8601 UTC> <audit_id> <text>` per data-model.md, and `read_log_slice(path, offset)` returning `(text, next_offset, size)` for byte-offset tailing.
- [X] T007 [P] Add tests for `format_log_line` (exact shape, audit ID present on every line, multi-line input handled) and `read_log_slice` (offset past EOF is safe, partial reads resume correctly, a missing file yields empty rather than raising).
- [X] T008 Add a `# ---------- tmux runner ----------` section to `framework/tools/auto`: `tmux_available()`, `tmux_session_name(audit_id)` → `auto-<audit_id>`, `tmux_has_session(name)`, `tmux_kill_session(name)`, and `tmux_launch(name, argv, cwd, env)` which builds a detached session. **`tmux_launch` MUST redirect the command's stdin from `/dev/null`** — this is the non-interactive guarantee (FR-015, research §1); without it, tmux's PTY makes the packs' `isInteractive()` true and runs hang on prompts.
- [X] T009 [P] Add an integration test that `tmux_launch` runs a trivial command to completion, that the session disappears on its own afterwards (research §2), that `tmux_kill_session` terminates a long-running one, and — critically — a test asserting the launched command sees a **non-TTY stdin**, locking in FR-015 against regression.
- [X] T010 In `framework/tools/auto`, add `reconcile_running_runs()` to the run store section: for every row with `status == 'running'`, if its tmux session is gone and `rc` is NULL, set status `failed` with a `note` explaining it ended abnormally (FR-016). Add a test that a run whose session vanished is moved off `running`.

**Checkpoint**: `./auto` still behaves exactly as before (no user-visible change yet); tests pass.

---

## Phase 3: User Story 1 - Launch a job with one click (Priority: P1) 🎯 MVP

**Goal**: Every job, orchestration, and safe command appears as a described button and starts with one click.

**Independent Test**: Open the dashboard, see the actions with descriptions, click a fast job (`hello-report`), and confirm it actually runs to completion.

- [X] T011 [US1] Add a `# ---------- action catalog ----------` section to `framework/tools/auto`: `list_actions()` returning Action dicts per data-model.md, built live from `load_jobs()` + `load_orchestrations()` + the fixed command table from research §7. Set `accepts_ai` true only for jobs, `danger` true for `schedule-sync`/`bootstrap`, and compute `available`/`unavailable_reason` from `job_runs_here()` and `validate_orchestration()` (FR-001, FR-003, FR-004).
- [X] T012 [P] [US1] Add tests for `list_actions()`: every loaded job and orchestration appears; `accepts_ai` is true for jobs and false for orchestrations (FR-010, research §7); `danger` is set only for the two maintenance commands; a job whose `runs_on` excludes this OS is `available: false` with a reason.
- [X] T013 [US1] Add the hidden `auto _exec-run <audit_id>` subcommand to `framework/tools/auto` (wire it into `main()`'s dispatch but deliberately **omit it from the module docstring's usage block** — it is a dashboard implementation detail, research §3). It loads the run row, builds the real CLI argv for that action (`auto run <id> [--ai <p>]`, `auto orchestrate <name>`, or `auto <command…>`), runs it as a subprocess with unbuffered output, writes every output line through `format_log_line` to the run's log, and calls `finish_run` with the true exit code.
- [X] T014 [US1] Add `start_run(kind, action_id, ai_profile, confirm)` to `framework/tools/auto`: validate the action exists and is available; enforce the duplicate guard (FR-006) and the `confirm` gate for `danger` actions (FR-005); create the run row; then `tmux_launch` a session running `python3 -u <auto> _exec-run <audit_id>` with `AUTO_RUN_AUDIT_ID` exported. Return the audit id or a typed error matching contracts/dashboard-api.md.
- [X] T015 [P] [US1] Add tests for `start_run`: refuses a second concurrent run of the same action (FR-006); refuses a `danger` action without `confirm` (FR-005); refuses an unavailable action; and refuses everything with a clear error when tmux is absent (FR-017).
- [X] T016 [US1] Extend `cmd_serve`'s handler in `framework/tools/auto` with a `do_POST` method plus JSON routing, and add `GET /api/actions` and `POST /api/runs` per contracts/dashboard-api.md. Enforce the shared rules: `Host` header allowlist (403 otherwise) and side-effect-free `GET` (FR-028, FR-029, research §9).
- [X] T017 [US1] In `_dashboard_html()`, add an **Actions** section rendering each action as a button with its name and description, grouped by kind, with unavailable ones visibly disabled showing their reason, and `danger` ones visually distinguished (FR-005). Show a prominent banner instead of buttons when tmux is missing (FR-017).
- [X] T018 [US1] Add the client-side JavaScript in `_dashboard_html()` to POST a launch, show a confirmation dialog first for `danger` actions, and surface the returned audit id with a link to the run — plus readable handling of the `409`/`412`/`422`/`503` error bodies.

**Checkpoint**: US1 fully usable — buttons launch real runs (quickstart scenarios 1, 2, 8, 9, 13).

---

## Phase 4: User Story 2 - Watch a run's progress and logs live (Priority: P1)

**Goal**: A Runs list with live status, and a per-run Jenkins-style log view that updates as output is produced.

**Independent Test**: Launch a job that emits output over time; watch its log grow without reloading; confirm it reaches a terminal status.

- [X] T019 [US2] Add `GET /api/runs`, `GET /api/runs/{audit_id}`, and `GET /api/runs/{audit_id}/log?from=` to the serve section of `framework/tools/auto` per contracts/dashboard-api.md. The list endpoint MUST call `reconcile_running_runs()` first (FR-016), and the log endpoint returns `{from,next,eof,text}` for incremental tailing (FR-019, FR-021).
- [X] T020 [US2] Add `POST /api/runs/{audit_id}/cancel`: kill the tmux session and record the run as `cancelled` (FR-014); return `409 not_running` when already terminal.
- [X] T021 [US2] In `_dashboard_html()`, add a **Runs** section listing runs newest-first with action name, audit id, start time, elapsed, AI profile, and status badge (FR-018), plus a Cancel control for in-progress runs.
- [X] T022 [US2] Add the live-tail JavaScript: poll the log endpoint ~1s from the last byte offset and append (SC-003), auto-scroll unless the user has scrolled up, stop polling on `eof`, and keep each run's view isolated from every other run's (FR-020). Cap the rendered buffer so a very chatty run cannot freeze the page (spec edge case).
- [X] T023 [US2] Add step-level progress for orchestration runs: in `execute_job`, emit a machine-readable step marker **only when `AUTO_RUN_AUDIT_ID` is set** (research §10, so terminal output is byte-for-byte unchanged); have `_exec-run` parse those markers into `ui_run_steps`; surface them via `GET /api/runs/{audit_id}` and render them in the run view (FR-023).
- [X] T024 [P] [US2] Add tests: the log endpoint's offset arithmetic is correct across successive polls; `reconcile_running_runs` is invoked by the list path; cancelling sets `cancelled`; and `execute_job` emits **no** step marker when `AUTO_RUN_AUDIT_ID` is unset (guarding the "terminal behavior unchanged" requirement).

**Checkpoint**: US1 + US2 complete — the core Jenkins-style experience (quickstart scenarios 3, 10, 11, 12).

---

## Phase 5: User Story 3 - Choose an AI profile before running (Priority: P2)

**Goal**: A dropdown of real, usable AI profiles, applied to the run and recorded with it.

**Independent Test**: Select `deepseek`, launch an AI-using job, and confirm from the run's own log and record that the profile was applied.

- [X] T025 [US3] Add `list_ai_profiles()` to `framework/tools/auto`: scan `config/ai/*.yaml`, exclude `*.example.yaml`, validate each via the existing `load_ai_profile`, and return `{name, provider, usable}` — **never any credential value** (FR-007, FR-008, FR-011, research §8).
- [X] T026 [P] [US3] Add tests: `*.example.yaml` files are excluded; a profile missing `api_key` is reported `usable: false` rather than dropped silently; and — asserting SC-007 — no returned structure contains the api_key string.
- [X] T027 [US3] Include `ai_profiles` in the `GET /api/actions` response and render a dropdown next to each action whose `accepts_ai` is true, with **no dropdown at all** for orchestrations and commands (FR-010). Pass the selection through on launch and display it on the run record (FR-009).

**Checkpoint**: quickstart scenario 4 passes.

---

## Phase 6: User Story 4 - Audit and trace runs after the fact (Priority: P2)

**Goal**: Every log line attributable to exactly one run, and past runs retrievable by audit id.

**Independent Test**: Run three actions concurrently, then confirm from the log files that every line is unambiguously attributable.

- [X] T028 [US4] Surface the audit id prominently in both the Runs list and the run detail view, and make it selectable/copyable (FR-024).
- [X] T029 [US4] Add a lookup affordance: entering an audit id opens that run's detail and full log, including for completed runs (FR-026). Return the contract's `404 unknown_run` for an unknown id.
- [X] T030 [P] [US4] Add a concurrency test: start three runs at once, assert each log file contains only its own audit id, that no line lacks an id, and that the three runs' records are all complete and distinct (FR-025, SC-004).

**Checkpoint**: all four user stories independently functional (quickstart scenarios 5, 6).

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T031 Write `docs/adr/0018-dashboard-job-runner.md` superseding ADR 0012's read-only decision: record why the "no auth because nothing can be triggered" justification no longer holds, and the controls that replace it (localhost bind retained, POST-only mutations, Host-header allowlist, server-side confirm gate). Add a "Superseded by ADR 0018" note to `docs/adr/0012-*.md` itself.
- [X] T032 [P] Update the `auto` module docstring usage block with the dashboard's new capability, and update root `README.md`'s Quickstart line for `./auto serve` to say it can now launch jobs — not just display them.
- [X] T033 [P] Add a RUNBOOK entry describing the runner: how to start it, where logs live, what an audit id is, the tmux prerequisite, and the deliberate non-interactive limitation (interactive-only pack features stay terminal-only).
- [X] T034 Run the full test suite (`python3 -m unittest discover -s framework/tools -p 'test_*.py'`) and fix any failures.
- [X] T035 Execute every quickstart.md validation scenario (1–14) against a real `./auto serve`, including the credential-leak grep (scenario 6), the non-interactive check on `gmail-categorize` (scenario 7), and reconciliation after an out-of-band `tmux kill-session` (scenario 12). Record the outcome.
- [X] T036 Confirm no stray `auto-*` tmux sessions and no committed log files remain after all validation (`tmux ls`, `git status`).

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (P1)** → **Foundational (P2)** blocks everything: the run store, log plumbing, and tmux control are used by every story.
- **US1 (P3)** depends only on Foundational. Delivers the MVP.
- **US2 (P4)** depends on Foundational and on US1's serve/JSON scaffolding (T016) and run records.
- **US3 (P5)** depends on Foundational and US1's action catalog + launch path.
- **US4 (P6)** depends on Foundational's log format; largely surfacing work on top of US1/US2.
- **Polish (P7)** depends on all stories being complete.

### Critical sequencing note

**T008's `/dev/null` stdin redirect must land before any real job is launched from the UI.** Launching `gmail-categorize` without it will hang on an interactive prompt with no way to answer (research §1). Do not defer it.

### Parallel Opportunities

- Test tasks (T004, T007, T009, T012, T015, T024, T026, T030) live in `test_auto.py` and can be written alongside their implementation counterparts.
- Within Phase 7, T032 and T033 touch different files and are independent of each other.
- Because the bulk of implementation edits one file (`framework/tools/auto`), implementation tasks within a phase are largely sequential — this is expected, not an oversight.

---

## Implementation Strategy

### MVP First

1. Phases 1–2 (Setup + Foundational).
2. Phase 3 (US1) — buttons that launch real runs.
3. **STOP and VALIDATE**: quickstart scenarios 1, 2, 8, 9, 13.

### Incremental Delivery

1. Foundational → nothing user-visible, everything else rests on it.
2. US1 → one-click launching works (MVP).
3. US2 → live logs and cancellation; this is where it starts feeling like Jenkins.
4. US3 → AI profile dropdown.
5. US4 → audit lookup and parallel-run traceability.
6. Polish → ADR supersession, docs, full validation sweep.

## Notes

- Commit after each phase, per this repo's RUNBOOK-per-run convention.
- **Do not** modify the existing `runs` table, `_record_run`, or `execute_job`'s terminal-visible output — the only change to `execute_job` is a step marker gated behind `AUTO_RUN_AUDIT_ID` (T023).
- Runs must never be launched by a `GET`; keep every mutation on `POST` (FR-029).
