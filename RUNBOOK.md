# RUNBOOK

Newest entries first. Each entry: timestamp, prompt summary, files affected, steps taken, outcome, caveats.

---

## 2026-07-23 22:33 — Tasks: Job Orchestrator (`/speckit-tasks`)

**Prompt summary**: `/speckit-tasks and /speckit-implement` — generate the task breakdown for the planned job orchestrator, then proceed straight into implementation.

**Files affected**:
- `specs/001-job-orchestrator/tasks.md` (new) — 26 tasks across 7 phases

**Steps taken**:
1. Confirmed no `.specify/extensions.yml` — before/after-tasks hooks skipped silently.
2. Ran `.specify/scripts/bash/setup-tasks.sh --json`, confirming `research.md`, `data-model.md`, `contracts/`, `quickstart.md` are all available inputs.
3. Organized tasks by the spec's own 4 user stories (P1 sequential run/list/validate, P2 retry+timeout, P3 wait, P4 bounded loop), preceded by Setup (create `orchestrator/`) and Foundational (extract `execute_job()`, add loaders/validator/SQLite recording) phases, followed by Polish.
4. Called out explicitly that almost no implementation task is `[P]` here — every story's code task edits the same function in the same single-script file (`framework/tools/auto`, per plan.md's Structure Decision), so parallelism only exists between a code task and its YAML-fixture/doc task, not between two code tasks.
5. Tied every fixture/manual-validation task directly to a numbered `quickstart.md` scenario so "done" is unambiguous.
6. Re-flagged the `until_exit_code` limitation (no job in this workspace emits a matching exit code yet) as its own task (T022) rather than letting it hide inside the loop-implementation task.

**Outcome**: 26 tasks (Setup 1, Foundational 4, US1 7, US2 4, US3 3, US4 3, Polish 4). MVP = Setup + Foundational + US1 (T001-T012) — already replaces today's two manual commands with one, spanning packs, with validation-before-execution.

**Caveats**: proceeding directly into `/speckit-implement` per this prompt's own instruction — see the next entry for implementation results.

---

## 2026-07-23 22:21 — Plan: Job Orchestrator (`/speckit-plan`)

**Prompt summary**: `/speckit-plan` with no additional input — proceed with planning against the existing `specs/001-job-orchestrator/spec.md`.

**Files affected**:
- `specs/001-job-orchestrator/plan.md` — filled in (Summary, Technical Context, Constitution Check + post-design re-check, Project Structure, Structure Decision)
- `specs/001-job-orchestrator/research.md` (new) — 6 design decisions with rationale + rejected alternatives
- `specs/001-job-orchestrator/data-model.md` (new) — YAML schema for Orchestration/Step/loop, SQLite schema for orchestration run history
- `specs/001-job-orchestrator/contracts/cli.md` (new) — `auto orchestrate` command contract (bare = list, `<name>` = run), exit codes
- `specs/001-job-orchestrator/quickstart.md` (new) — 8 manual validation scenarios covering every user story + success criterion

**Steps taken**:
1. Confirmed no `.specify/extensions.yml` — before/after-plan hooks skipped silently.
2. Ran `.specify/scripts/bash/setup-plan.sh --json`, which copied the plan template and confirmed branch `001-job-orchestrator`.
3. Re-read `spec.md` in full and confirmed the (unfilled) constitution has no ratified principles to gate against.
4. Inspected `framework/tools/auto`'s existing `cmd_run`/`_record_run` implementation, `.gitignore`'s `data/state/*.sqlite` rule, and `schedules/` as the precedent for a workspace-root, pack-spanning directory — used these to ground every plan decision in what already exists rather than inventing new patterns.
5. Made and documented 6 key decisions in research.md: `orchestrator/` location, flat `auto orchestrate [name]` CLI shape (list vs. run), extracting a shared `execute_job()` core instead of shelling out, a new `orchestrations.sqlite` file instead of extending `runs.sqlite`, a minimal `max_iterations`+`until_exit_code` loop vocabulary, and sequential-only v1 execution.
6. Wrote data-model.md (authoritative field names/types/defaults for the YAML schema and SQLite tables) and contracts/cli.md (command behavior, exit codes, compatibility notes tying back to FR-016).
7. Wrote quickstart.md with one manual scenario per user story (plus discoverability and history-inspection scenarios), since this workspace has no automated test suite.

**Outcome**: Phase 0 + Phase 1 planning complete. No code written yet — `framework/tools/auto` is unmodified; `orchestrator/` directory doesn't exist on disk yet. That's `/speckit-tasks` → `/speckit-implement`.

**Caveats**:
- The loop `until_exit_code` convention (e.g. "job exits 2 when nothing left to categorize") is a *contract this feature introduces*, not something `gmail-categorize` already does — implementing loop support (US4/P4) will need either a small change to that job to emit such a code, or the loop capability will only be exercised with `max_iterations` alone until that job-side change happens. Flagged here so `/speckit-tasks` doesn't assume it's free.
- Same Obsidian-note caveat as the previous entry: no known vault path, so that global-memory instruction was not applied.

---

## 2026-07-23 22:15 — Spec: Job Orchestrator (`/speckit-specify`)

**Prompt summary**: User currently runs `./auto run gmail-extract -- --ai=deepseek` then `./auto run gmail-categorize -- --ai=deepseek --batch-size 0` by hand. Wants an `orchestrator/` directory holding pipeline YAML files (e.g. `gmail-wallet-sync.yaml`), run via `./auto orchestrate gmail-wallet-sync`, executing steps in order. This must become a spec-kit feature (not just an ad-hoc script) because future capabilities are planned: loop, wait, retry, timeout — and pipelines must span multiple packs and be fully user-controllable via YAML.

**Files affected**:
- `specs/001-job-orchestrator/spec.md` (new) — feature specification
- `specs/001-job-orchestrator/checklists/requirements.md` (new) — spec quality checklist (all items passed)
- `.specify/feature.json` (new) — records `feature_directory: specs/001-job-orchestrator` for downstream `/speckit-plan` etc.
- `RUNBOOK.md` (new, this file)

**Steps taken**:
1. Inspected the repo: `framework/tools/auto` (the CLI), `packs.yaml`, and the three relevant Gmail job manifests (`gmail-extract`, `gmail-categorize`, plus `gmail-discover`/`gmail-recategorize` for context) to understand the existing job model (id, pack, `exec`/`entrypoint`, `runtime.timeout_seconds`, `runs_on`, `schedule`) that any orchestrator step would wrap rather than replace.
2. Confirmed no `orchestrate` command, no `orchestrator/` directory, and no `specs/` directory exist yet — this is feature `001`.
3. Confirmed `.specify/extensions.yml` does not exist, so before/after-specify hooks were skipped silently per the command's own pre/post-execution check rules.
4. Read `.specify/init-options.json` (`feature_numbering: sequential`) and the (unfilled template) constitution — no project-specific constitutional constraints to apply.
5. Wrote `specs/001-job-orchestrator/spec.md` using the resolved `spec-template.md`, with 4 prioritized user stories (P1 sequential run, P2 retry/timeout, P3 wait, P4 bounded loop), 16 functional requirements, 3 key entities, 6 measurable success criteria, and documented assumptions in place of `[NEEDS CLARIFICATION]` markers (all open questions had reasonable, low-risk defaults).
6. Generated and validated the spec quality checklist — all items passed on the first pass, no clarification markers needed.
7. Wrote `.specify/feature.json` pointing downstream commands at the feature directory.

**Outcome**: Spec is complete and passed its own quality checklist on the first iteration. No implementation code was written — this command only produces the specification artifact, per its contract.

**Caveats**:
- This is a spec only. `./auto orchestrate`, the `orchestrator/` directory, and any YAML schema/parser do not exist yet — that's `/speckit-plan` → `/speckit-tasks` → `/speckit-implement`.
- V1 scope is sequential, single-machine execution; parallel/fan-out steps and cross-machine orchestration were explicitly deferred (see spec Assumptions).
- The global CLAUDE.md instruction to maintain an Obsidian note per query was not applied — no Obsidian vault path is known in this environment or in prior memory, so nothing was written there to avoid guessing a wrong location.
