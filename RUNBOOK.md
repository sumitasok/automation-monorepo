# RUNBOOK

Newest entries first. Each entry: timestamp, prompt summary, files affected, steps taken, outcome, caveats.

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
