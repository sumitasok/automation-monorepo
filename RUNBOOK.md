# RUNBOOK

Newest entries first. Each entry: timestamp, prompt summary, files affected, steps taken, outcome, caveats.

---

## 2026-07-23 23:45 — Implement: Expense Classification Rules Engine (`/speckit-plan and /speckit-tasks and /speckit-implement`)

**Prompt summary**: Chained `/speckit-plan and /speckit-tasks and /speckit-implement` against the already-written `specs/002-expense-rules-engine/spec.md` — design, break into tasks, then build all four user stories for real.

**Files affected**:
- `specs/002-expense-rules-engine/{plan,research,data-model,quickstart}.md`, `contracts/{expense-rules.schema,cli}.md`, `tasks.md` (new) — design artifacts and a 28-task breakdown across 7 phases.
- `data/config/expense-rules.yaml` (new) — the shared, versioned rules file both packs read; committed with two real, validated rules (`hungerbox-workplace-food`, `uber-weekday-afternoon-commute`).
- `packs/gmail` (submodule, `sa.automation.gmail`) — `categorize/rules.go` + `rules_test.go` (new): `ExpenseRule`/`ExpenseRules`/`MatchCondition`/`Outcome` types, ordered first-match-wins evaluation, five condition types (merchant/keyword/day-of-week/time-of-day/amount), load-time validation. `categorize.go`: `Run()` evaluates rules before the AI assigner; a fully-resolved match skips the AI entirely. `store/csv.go`: new additive `Source` column. `main.go`: new `--rules-file` flag. `RUNBOOK.md`: new entry.
- `packs/expenses/internal/event/rules.go` + `rules_test.go` (new) — an independent duplicate of the gmail-side engine (different repo/module boundary, same shared file). `updateevent.go`: `Run()` evaluates `event`-scoped rules before the AI matcher; a `routine` outcome marks the transaction as not event-worthy with zero AI calls. `state.go`: new additive `Source` field on `AssignmentEntry`; `Mark()` call sites across `updateevent.go`/`fillsimilar.go`/`bulkassign.go` all updated. `main.go`: new `--rules-file` flag. `go.mod`/`go.sum`: first external dependency, `gopkg.in/yaml.v3`. `RUNBOOK.md`: new section.
- `docs/adr/0016-expense-rules-engine.md` (new).
- Both jobs' `manifest.yaml` — `data.reads` gained the shared rules file.

**Steps taken**:
1. **Plan**: researched the actual current state of both consumer packs (categorize.go, updateevent.go, taxonomy validation, DeepSeek provider Strategy pattern) before designing, rather than guessing. Key finding during research: corrected a wrong assumption from the spec phase — `gmail-recategorize` operates on a completely different file/domain (`email_catalog.csv`'s sender-domain classification) than transaction category re-classification, so "reuse gmail-recategorize for retroactive re-application" was factually wrong; documented the correction in `research.md` rather than silently reusing the bad assumption.
2. Chose `data/config/expense-rules.yaml` (read via the already-injected-but-previously-unused `AUTO_DATA_DIR` env var) over both root `config/<pack>/` (git-ignored secrets, wrong meaning) and a pack-local file (not actually shared) — this is the first real consumer of a convention the workspace had documented but nothing used yet.
3. Chose to duplicate the rule-loading/matching Go code between `packs/gmail` (independently-versioned git submodule) and `packs/expenses` (separate in-repo Go module) rather than share a Go import — following the exact precedent ADR 0011 already set for the DeepSeek-provider Strategy interface.
4. **Tasks**: broke the plan into Setup → Foundational (only gates US3's new `yaml.v3` dependency) → US1 (P1, merchant rule, MVP) → US2 (P2, time/day conditions) → US3 (P2, event-relevance) → US4 (P3, decision-source auditability) → Polish.
5. **Implement**: built the full condition evaluator (merchant/keyword/day-of-week/time-of-day/amount, first-match-wins, taxonomy validation, fail-closed time matching) and the `Source`-tagging integration into `Run()` in one coherent pass per pack, rather than artificially splitting a single function's edit across phases — noted this explicitly in `tasks.md` rather than leaving the phase-to-code mapping misleading.
6. Manually validated every quickstart.md scenario against scratch CSVs/state files at each phase boundary: zero-rules regression (byte-identical failure point to pre-feature behavior), merchant rule, weekday-afternoon-Uber time/day matching (including fail-closed on date-only timestamps), rule outcome vs. taxonomy rejection, event-relevance routine-marking, and `Source` column/field on real (non-dry-run) writes.
7. Committed incrementally: gmail submodule (US1+US2 code, then the RUNBOOK entry) pushed to its own remote (`sa.automation.gmail`) before bumping the submodule pointer in this repo; expenses (not a submodule) committed directly; parent-repo commits for the shared rules file, manifests, ADR, and tasks.md tracking throughout.

**Outcome**: All 28 tasks across 7 phases complete. `go test ./...` passes in both packs (53 gmail tests, 14 expenses tests, including 32 new rules-engine tests). Both `gmail categorize` and `expenses update-event` gained a `--rules-file` flag; a confirmed rule match now decides Category/SubCategory/Labels (gmail) or event-relevance (expenses) deterministically with zero AI calls, and every decision (rule- or AI-made) is now auditable via a new `Source` column/field.

**Caveats**:
- Rule-decided rows are held in memory and only persisted at the same single `Save()` call the AI-decided rows already used — consistent with pre-existing behavior, but it means if the AI batch portion of a run errors out, rule-decided rows from that same run are *not* persisted either (matches today's existing all-or-nothing-on-batch-error semantics; not a regression, but worth knowing if a run fails partway).
- Only two rules are committed so far (`hungerbox-workplace-food`, `uber-weekday-afternoon-commute`) — both validated against real scratch data, but neither has been run against real financial data yet; do that deliberately when ready, same caveat as the orchestrator feature's `gmail-wallet-sync.yaml`.
- No repo-wide test runner exists in this workspace (same finding as the job-orchestrator feature) — verification was per-pack `go test ./...` plus manual quickstart scenarios.

---

## 2026-07-23 23:10 — Specify: Expense Classification Rules Engine (`/speckit-specify`)

**Prompt summary**: User wants a rules engine — human-authored rules like "afternoon Uber = office-to-home work travel" or "merchant HungerBox = workplace food" — to be the basis of how `gmail-categorize` and `expenses-update-event` classify transactions, instead of the AI re-guessing the same recurring patterns every run.

**Files affected**:
- `specs/002-expense-rules-engine/spec.md` (new) — feature spec: 4 prioritized user stories (merchant rule, time+pattern rule, rules informing event-matching, decision-source auditability), 13 functional requirements, 5 success criteria, and an Assumptions section.
- `specs/002-expense-rules-engine/checklists/requirements.md` (new) — quality checklist, all items passing.
- `.specify/feature.json` — repointed `feature_directory` to `specs/002-expense-rules-engine`.

**Steps taken**:
1. Confirmed no `.specify/extensions.yml` — pre/post-specify hooks skipped silently.
2. Read both consumer jobs' manifests and Go source (`packs/gmail/categorize/categorize.go`, `packs/expenses/internal/event/matcher.go`) to ground the spec in what data is actually available (merchant, description/subject, amount, TxnDate — no location/GPS) and how each job currently prompts its AI provider.
3. Confirmed `config/taxonomy.yaml` already has categories/labels (e.g. Transportation/Business trips, "Work" label) that a "work expense" outcome can map onto — no new taxonomy needed.
4. Resolved what would otherwise be 2-3 [NEEDS CLARIFICATION] markers (rule-vs-AI precedence; conflict resolution between rules) as documented Assumptions instead, since reasonable defaults existed: the user's own global CLAUDE.md instruction to codify recurring decisions to avoid unnecessary AI calls (→ confirmed rule match deterministically skips the AI call), and the existing ordered per-bank filter-file pattern already in `packs/gmail/filters/` (→ first-match-wins precedence).
5. Ran the spec-quality checklist — all items passed on the first pass, no iteration needed.
6. Used the top-level `specs/` directory (sequential numbering, next after `001-job-orchestrator`) rather than `packs/gmail`'s own scoped spec-kit instance, since the feature is cross-cutting across both the gmail and expenses packs.

**Outcome**: Spec ready for `/speckit-plan`. No open clarification questions.

**Caveats**:
- Spec deliberately leaves storage format/location, matching implementation, and how rule evaluation integrates into each job's existing batch-call flow to the planning phase — this is a WHAT/WHY spec only.
- The "office to home" direction inference is documented as a time-of-day heuristic only (assumption), since no GPS/route data exists in the transaction extract today.

---

## 2026-07-23 22:44 — Implement: Job Orchestrator (`/speckit-implement`)

**Prompt summary**: Chained from `/speckit-tasks and /speckit-implement` — generate tasks, then implement all 26 of them against the approved plan/spec.

**Files affected**:
- `framework/tools/auto` — extracted `execute_job()` from `cmd_run()`; added `ORCH_DIR`, `load_orchestrations()`/`load_orchestration()`, `validate_orchestration()`, the `orchestrations.sqlite` schema + `_record_orchestration_run()`/`_record_orchestration_step()`, `_run_step_once_with_retry()`, `_run_step_with_loop()`, `_print_orchestrations()`, `cmd_orchestrate()`; wired the `orchestrate` subcommand into argparse and the usage docstring.
- `orchestrator/README.md` (new) — schema reference for anyone authoring a pipeline.
- `orchestrator/gmail-wallet-sync.yaml` (new) — the real two-step pipeline (`gmail-extract` → `gmail-categorize`) replacing today's two manual commands.
- `README.md` — added `auto orchestrate` to Quickstart and `orchestrator/` to "Where things are."
- `Makefile` — added `make orchestrate NAME=...` (mirrors `make run JOB=...`).
- `specs/001-job-orchestrator/tasks.md` — all 26 tasks marked `[X]`.

**Steps taken**:
1. Confirmed no `.specify/extensions.yml` (hooks skipped) and that the spec-quality checklist was 16/16 complete — proceeded without pausing for confirmation.
2. Implemented Setup + Foundational (T001–T005) as pure additions/refactors to `framework/tools/auto`, keeping `cmd_run()`'s observable behavior byte-for-byte identical (verified via `python3 -m py_compile` and a real `auto run hello-report`).
3. Implemented US1 (T006–T012): `orchestrate` subcommand, list mode, run mode with validation-before-execution, sequential step loop, history recording, and the real `gmail-wallet-sync.yaml` fixture.
4. Implemented US2 (T013–T016, retry + per-step timeout), US3 (T017–T019, wait-before), US4 (T020–T022, bounded loop with `until_exit_code`) directly on top of the same step-execution loop, in that order.
5. Implemented Polish (T023–T026): README/Makefile docs, `auto doctor` regression check, history inspection.
6. **Manual validation** (in place of an automated suite, since none exists in this workspace): created four throwaway jobs under `packs/private/jobs/scratch/` (`orch-test-fail`, `orch-test-flaky`, `orch-test-slow`, `orch-test-loop`) plus scratch `orchestrator/_test-*.yaml` files, and exercised every quickstart.md scenario against them — sequential run + list + unknown-job validation, fail-then-skip-remaining, retry-recovers, retry-exhausted, timeout-kills-and-counts-as-failed-attempt, wait-before measured at ~3.1s for a 3s wait, loop stopping early on `until_exit_code` at iteration 2 of 5, and loop stopping at `max_iterations` (2) when the condition never matched. Inspected `data/state/orchestrations.sqlite` directly and confirmed run/step rows matched every scenario. Ran `./auto doctor` — still reports OK, confirming the `execute_job()` extraction didn't regress existing manifest/visibility checks.
7. **Cleaned up all scratch test artifacts** (`packs/private/jobs/scratch/`, `orchestrator/_test-*.yaml`, `/tmp/orch-test-*.state`) before committing — none of it is part of the shipped feature.

**Outcome**: All 26 tasks complete. `./auto orchestrate` (list) and `./auto orchestrate gmail-wallet-sync` (run) are live. Every user story (P1–P4) validated manually per quickstart.md. `auto doctor` and `auto run` behavior confirmed unregressed.

**Caveats**:
- `orchestrator/gmail-wallet-sync.yaml` was validated structurally and via equivalent scratch fixtures, but was **not executed for real** — doing so would trigger live Gmail API reads and billed DeepSeek API calls against real financial data, which this session didn't take without the user explicitly asking for that specific run. Run `./auto orchestrate gmail-wallet-sync` yourself when ready to replace the two manual commands for real.
- The `until_exit_code` loop-stop mechanism works exactly as designed (proven with the scratch `orch-test-loop` fixture), but `gmail-categorize` itself doesn't yet emit a distinguishing "nothing left" exit code — so looping `gmail-categorize` today only makes sense bounded by `max_iterations` alone, as flagged in the plan. Documented in `orchestrator/README.md`'s "Known limitation."
- No automated regression suite was added (matches this workspace's existing convention for `framework/tools/auto`) — re-run the scratch-fixture scenarios above if `cmd_orchestrate`'s step loop is touched again in the future.

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
