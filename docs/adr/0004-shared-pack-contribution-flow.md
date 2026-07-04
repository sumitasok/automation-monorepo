# ADR 0004 — Shared pack: contribute-back flow

**Status:** accepted — 2026-07-04

## Context

Collaborators should be able to **use** shared automations and **contribute**
their own back (the chosen "read + contribute back" model), across a public
framework and a team-only shared pack.

## Decision

- The **shared pack is its own git repo** that collaborators clone/mount as a
  submodule and to which they have push or fork-and-PR access.
- Contributing a job = add a job folder under the shared pack's `jobs/`, set
  `visibility: shared` (or `public`), and open a PR against the shared-pack repo.
- The shared-pack repo runs CI: `auto doctor` (valid manifests, no private
  leaks, unique ids) must pass before merge.
- Job `id`s are globally unique across packs; doctor enforces this so two
  contributors can't collide.
- Shared jobs must be self-contained or depend only on the shared pack's own
  `lib/`, so they run in any workspace that mounts the pack.

## Consequences

- The shared pack becomes a living team library; the framework stays stable and
  public underneath it.
- A collaborator's private experiments live in *their* workspace's private pack
  until they choose to PR them to the shared pack.
- Versioning: workspaces pin the shared pack to a submodule commit, so a bad
  contribution can't break everyone instantly — each person updates the pointer
  when ready.
