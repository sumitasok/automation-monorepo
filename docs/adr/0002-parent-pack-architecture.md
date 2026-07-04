# ADR 0002 — Split into a framework "parent" + pluggable packs

**Status:** accepted — 2026-07-04

## Context

The single monorepo (ADR 0001) works for one person, but the goal is now to
**share** the system with others so they can (a) use a common set of
automations, (b) add their own, and (c) keep some automations private while
sharing others. A single private repo can't do this: you can't hand someone the
repo without handing them everything in it.

## Decision

Split the system into three layers, each with its own git repo and its own
access control on the git host:

1. **`framework` (the parent) — public.** The `auto` CLI, the job template, the
   manifest schema, and the conventions. Contains no jobs, no data, no secrets,
   so it is safe to publish. Versioned (`framework/VERSION`), MIT-licensed.
2. **`packs` — one repo per pack.** A pack is a folder of jobs
   (`jobs/**/manifest.yaml`) plus `pack.yaml`. The **shared** pack is a
   team-accessible repo; a **private** pack lives only in the owner's workspace;
   **community** packs are public and consumed read-only.
3. **`workspace` — the owner's private repo.** Holds `machines.yaml`, `data/`,
   and `packs.yaml`, and mounts the framework and packs as git **submodules**.

A job carries a `visibility` field (`private` | `shared` | `public`), defaulting
to its pack's `default_visibility`. `auto` discovers jobs across every mounted
pack and shows/handles them accordingly.

## Why separate repos (submodules) over one-repo-with-flags

The two candidates were (a) one repo where a `visibility` flag plus `.gitignore`
export tooling decides what ships, and (b) separate repos linked by submodules.

We chose **(b)** because with "Both" audiences (public framework, team-only
shared pack) and contribute-back collaboration, the security boundary must be
**structural, not procedural**. With separate repos, a private automation
physically cannot leave the private workspace repo — the git host enforces it by
repository permissions. With one-repo-plus-flags, one wrong `.gitignore` line or
a forced `git add` leaks private scripts, and there is no second line of defense.
Submodules also give each pack independent history, access control, and
contribution flow (PRs against the shared pack), which is exactly what
"read + contribute back" requires.

Cost: submodules add a small amount of git ceremony (`--recursive` clones,
pinned commits). We accept this; `auto bootstrap` wraps the commands.

## Defense in depth

`visibility` + `auto doctor` is a **second** safety net on top of the repo
boundary: doctor fails if a `private` job appears inside a shared/public pack, so
a mistaken move is caught before it's committed.

## Consequences

- The framework can be open-sourced; anyone can adopt it without seeing anyone's
  automations.
- Collaborators mount the shared pack and contribute via PR; their private work
  stays in their own workspace.
- Migration: the current monorepo becomes the reference *workspace*; `framework/`
  and `packs/shared/` are designed to be promoted to standalone repos and
  re-mounted as submodules (see `docs/SHARING.md`).
- See also [[0003-job-visibility-model]], [[0004-shared-pack-contribution-flow]].
