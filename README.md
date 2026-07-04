# automation-monorepo

One home for every script that makes your life easier — discoverable,
version-controlled, self-scheduling, self-documenting, and backed by a shared
git-synced data store, across macOS / Linux / Windows / server.

**The one idea:** every script is a *job* — a folder under `jobs/` with a
`manifest.yaml`. The manifest says what it is, how to run it, when, and where.
Everything else (the catalog, the OS schedules, the docs) is generated from
manifests, so nothing drifts out of date.

## Quickstart

```bash
# See everything you have
./tools/auto list
./tools/auto search <term>

# Start a new job (interactive scaffold from jobs/_template)
./tools/auto new

# Run one now (uniform logging, timeout, run-history)
./tools/auto run hello-report

# Install this machine's schedules from the manifests
./tools/auto schedule sync --dry-run   # preview
./tools/auto schedule sync             # apply

# Regenerate the browsable index
./tools/auto catalog

# Journal what you did
./tools/auto log "set up the monorepo"

# Health-check: validate manifests, find drift
./tools/auto doctor
```

## New machine, from zero

```bash
git clone <remote> automation-monorepo && cd automation-monorepo
./tools/auto bootstrap        # git-lfs hook + runtime checks
./tools/auto schedule sync    # install schedules meant for this machine
```

## Where things are

- `jobs/` — your scripts, one folder each (`manifest.yaml` + code + README)
- `lib/` — shared code (python/node/bash/go)
- `data/` — the global data store: `config/` (YAML), `state/` (SQLite), `inbox/` (scratch)
- `docs/` — `PLAN.md` (the full design), `worklog/` (dated journal), `adr/`
- `tools/` — the `auto` CLI and generators
- `machines.yaml` — your registered computers
- `CATALOG.md` — auto-generated index of all jobs

Full design and rationale: **[docs/PLAN.md](docs/PLAN.md)**.
