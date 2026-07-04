# automation-framework (the parent)

The shareable core of the automation system: the `auto` CLI, the job template,
the manifest schema, and the conventions. **No jobs, no data, no secrets** live
here — those live in *packs* and *workspaces*, so this framework is safe to make
public and share with anyone.

## The model

```
framework  (this repo, public)        the parent: CLI + conventions
   ▲
   │ used by
workspace  (your private repo)        machines.yaml + data/ + packs.yaml
   ├── framework/   (submodule → this repo)
   └── packs/
        ├── shared/   (submodule → team pack, contribute-back)
        └── private/  (your jobs, never shared)
```

A **pack** is a folder of jobs (`jobs/**/manifest.yaml`) plus a `pack.yaml`. A
**workspace** mounts one or more packs via `packs.yaml`. The `auto` CLI
discovers jobs across every mounted pack and respects each job's `visibility`.

## Sharing rules (enforced by `auto doctor`)

- A job's `visibility` is `private`, `shared`, or `public` (defaults to its
  pack's `default_visibility`).
- A `private` job may **not** live in a `shared`/`public` pack — this is what
  stops personal automations from leaking when you share.
- The framework and shared pack are separate repos, so access is controlled by
  your git host, not by hoping a `.gitignore` is correct.

See `docs/SHARING.md` in a workspace for onboarding and the exact git commands.

Version: see `VERSION`. Licensed MIT (`LICENSE`).
