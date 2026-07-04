# ADR 0001 — Manifest-driven monorepo for personal automation

**Status:** accepted — 2026-07-04

**Context.** Scripts were scattered across machines, undocumented, scheduled by
hand-edited crontabs, and forgotten. No index, no shared data, no history.

**Decision.** One monorepo. Every script is a *job* folder with a
`manifest.yaml` that is the single source of truth. The catalog, per-OS
schedules, and docs are generated from manifests. Data lives as git-synced YAML
(config) + SQLite (state).

**Consequences.**
- Discovery, scheduling, and docs can't drift — they regenerate from manifests.
- One `git pull` syncs code + data to every machine.
- SQLite-over-git needs a single writer machine per DB; revisit with hosted
  Postgres only if concurrent multi-writer or large data emerges.
