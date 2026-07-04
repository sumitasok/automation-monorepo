# automation workspace

Your private working copy of the automation system. It mounts a shareable
**framework** (the parent) and one or more **packs** of jobs, and holds your
machines, data, and private automations.

**The model** (full rationale in `docs/adr/0002…` and `docs/SHARING.md`):

```
workspace (this repo, private)     machines.yaml + data/ + packs.yaml
├── framework/   → public parent: the `auto` CLI + conventions   (submodule)
└── packs/
     ├── shared/  → team library, contribute-back                (submodule)
     └── private/ → your jobs, never shared
```

A **job** is a folder with a `manifest.yaml`. Jobs live in packs. Each job has a
`visibility` (`private`/`shared`/`public`) so you can share some automations and
keep others private — enforced by `auto doctor`, guaranteed by per-repo access.

## Quickstart

```bash
./auto packs                 # what's mounted
./auto list                  # every job you can see (pack + visibility shown)
./auto list --visibility shared
./auto search backup
./auto run hello-report      # run a job (logging, timeout, history)
./auto config init gmail     # scaffold a pack's config (values live in config/, git-ignored)
./auto config gmail          # show which env/secret values are set vs missing
./auto new                   # scaffold a job into a pack (choose private/shared)
./auto catalog               # regenerate CATALOG.md
./auto share shared          # write a shareable catalog of the shared pack
./auto schedule sync --dry-run
./auto log "what I did"
./auto doctor                # validate + check for visibility leaks
```

## Sharing it with others

You share the **framework** (public) and the **shared pack** (team) — never your
workspace or `packs/private/`. To split this folder into the three real repos and
mount them as submodules: `tools/split-into-repos.sh` (dry-runs by default). Full
walkthrough and collaborator onboarding: **[docs/SHARING.md](docs/SHARING.md)**.

## Where things are

- `framework/` — the parent: `auto` CLI, template, schema, LICENSE, VERSION
- `packs/shared/`, `packs/private/` — jobs, each pack with a `pack.yaml`
- `packs.yaml` — which packs are mounted
- `machines.yaml` — your computers
- `data/` — git-synced store: `config/` (YAML), `state/` (SQLite)
- `docs/` — `PLAN.md`, `SHARING.md`, `adr/`, `worklog/`

Design & decisions: `docs/PLAN.md` and `docs/adr/`.
