# automation-monorepo: organization, scheduling, and data plan

Goal: every script you write lives in one discoverable place, is version-controlled, documents itself automatically, runs on schedule on whichever machine it belongs to, and leaves a queryable trail of what ran and when — across macOS, Linux, and Windows.

## 1. Repository layout

One monorepo, cloned identically on every machine. Scripts are grouped by purpose, not by language, since you use Python/Bash/Node/Go interchangeably.

```
automation-monorepo/
├── README.md                  # auto-generated catalog (see §3) — this is your homepage
├── bin/
│   └── automate               # single CLI entrypoint (Python), on PATH on every machine
├── scripts/
│   ├── backup/
│   │   └── nightly-restic/
│   │       ├── manifest.yaml  # metadata (required for every script)
│   │       ├── run.sh
│   │       └── README.md      # optional longer notes
│   ├── finance/
│   ├── devops/
│   ├── media/
│   └── system/
├── lib/                       # shared helpers, one subfolder per language
│   ├── python/
│   ├── sh/
│   ├── node/
│   └── go/
├── registry/
│   ├── scripts.yaml           # auto-generated: aggregated catalog of every manifest
│   └── schedules.yaml         # source of truth for what runs when, on which host(s)
├── data/
│   ├── config/
│   │   └── machines/
│   │       ├── mac-mini.yaml       # per-machine overrides (paths, enabled scripts)
│   │       ├── work-laptop.yaml
│   │       └── linux-box.yaml
│   └── automation.db          # SQLite run history — gitignored, local per machine (§6)
├── logs/                      # raw stdout/stderr, rotated, gitignored
├── docs/
│   └── worklog/                # optional dated notes for decisions, not code
├── tools/
│   ├── build_catalog.py       # regenerates registry/scripts.yaml + README table
│   ├── validate_manifests.py  # schema check, run in pre-commit and CI
│   └── schedule_sync.py       # reconciles schedules.yaml with the native OS scheduler
├── install.sh                 # bootstrap a new machine (§7)
├── .pre-commit-config.yaml
├── .gitignore                 # data/automation.db, logs/, .env, secrets*
└── .github/workflows/ci.yml   # lint + validate manifests + rebuild catalog on push
```

Rule: **a script is not "done" until it has a `manifest.yaml`.** That one rule is what prevents the current problem — scripts you forget about are scripts with no metadata anywhere describing what they do.

## 2. Script anatomy

Every script folder gets a manifest. This is the metadata that makes discovery, scheduling, and documentation automatic instead of something you have to remember to do.

```yaml
# scripts/backup/nightly-restic/manifest.yaml
name: nightly-restic-backup
description: Backs up ~/Documents and ~/Projects to Backblaze B2 via restic.
language: bash
entrypoint: run.sh
tags: [backup, restic, storage]
owner: sumit
created: 2026-07-04
last_reviewed: 2026-07-04
machines: [mac-mini, work-laptop]     # or "all"
schedule: "0 2 * * *"                  # cron syntax, translated per-OS by schedule_sync.py
timeout_minutes: 30
env_required: [B2_ACCOUNT_ID, B2_ACCOUNT_KEY]
depends_on: [lib/sh/notify.sh]
notes: >
  Restores tested manually on 2026-06-01. See README.md for restore steps.
```

`tools/validate_manifests.py` rejects a commit if a script under `scripts/` lacks a manifest or the manifest fails schema validation (missing name/description/entrypoint). Wire this into `.pre-commit-config.yaml` so it's enforced automatically, not by memory.

## 3. Discovery: catalog + CLI

`tools/build_catalog.py` walks `scripts/**/manifest.yaml`, aggregates everything into `registry/scripts.yaml`, and renders a markdown table into `README.md` (grouped by tag, with description, schedule, and machines). This runs as a pre-commit hook, so the README is always current — you never hand-maintain an index.

`bin/automate` is a thin CLI (Python, ~150 lines) wrapping the registry:

- `automate list` — table of every script, filterable by tag/machine
- `automate search <term>` — grep over name/description/tags
- `automate run <name>` — runs it locally through the logging wrapper (§5)
- `automate history <name>` — last N runs from SQLite, with exit codes and duration
- `automate schedule sync` — reconciles this machine's cron/launchd/Task Scheduler entries with `registry/schedules.yaml`

This is the single command you reach for instead of hunting through folders: "did I already write something for X" becomes `automate search X`.

## 4. Version control workflow

Trunk-based, single `main` branch, no long-lived feature branches for something this size. Small commits per script, conventional-commit style so `git log` doubles as a changelog: `feat(backup): add nightly restic backup`, `fix(finance): correct timezone in csv parser`.

Pre-commit hooks (via `pre-commit` framework, works identically on macOS/Linux/Windows-with-WSL-or-Git-Bash) run on every commit: `validate_manifests.py`, `build_catalog.py` (auto-regenerates and re-stages README/registry), and language linters (`shellcheck`, `ruff`, `eslint`, `gofmt`) scoped to changed files. This means the repo is self-documenting and lint-clean by construction — you can't commit a script without it becoming discoverable.

Secrets never enter git: a checked-in `secrets.example.yaml` shows the shape; real values live in `.env` files per machine (gitignored) or an OS keychain, referenced by `env_required` in the manifest.

## 5. Cross-platform scheduling

`registry/schedules.yaml` is the one place schedules are declared (cron syntax, plus a `machines` field), decoupled from any single OS's scheduler:

```yaml
- script: backup/nightly-restic
  cron: "0 2 * * *"
  machines: [mac-mini, work-laptop]
- script: finance/monthly-report
  cron: "0 9 1 * *"
  machines: [all]
```

`tools/schedule_sync.py` reads this and reconciles the native scheduler on whichever machine it runs on:

- **macOS** → generates a launchd `.plist` per script into `~/Library/LaunchAgents`, `launchctl load`s it
- **Linux** → writes a systemd user timer + service unit (falls back to crontab if systemd isn't available)
- **Windows** → shells out to `schtasks.exe /Create` (or generates Task Scheduler XML for more complex triggers)

Run `automate schedule sync` after every `git pull` (or hook it into a login/boot script) so a schedule change made on one machine propagates everywhere the next time you sync. This is idempotent — it diffs what's currently installed against `schedules.yaml` and only adds/removes what changed.

## 6. Data storage

Two stores, used for different things:

**SQLite (`data/automation.db`)** — structured, queryable run history. One table is enough to start:

```sql
CREATE TABLE runs (
  id INTEGER PRIMARY KEY,
  script_name TEXT NOT NULL,
  host TEXT NOT NULL,
  started_at TEXT NOT NULL,
  ended_at TEXT,
  exit_code INTEGER,
  duration_s REAL,
  git_commit TEXT,
  log_path TEXT
);
```

Every script is invoked through a thin runner (`lib/*/run_wrapper.*`) instead of directly by the scheduler. The wrapper records a row before/after every run — hostname, start/end time, exit code, the repo's current git commit, and a pointer to the full stdout/stderr in `logs/`. This is what solves "document every single work I do": the documentation happens automatically on every run, with zero manual effort, queryable via `automate history`.

**YAML** — for anything you'd want to hand-edit or diff in git: the manifest catalog, `schedules.yaml`, and per-machine config (`data/config/machines/*.yaml` — which scripts are enabled on that host, local path overrides). YAML stays in git because it's small and human-authored; the SQLite file does not (see §7 for why).

If later you want richer queries (joins, aggregations across many scripts) you can add tables without changing this design — SQLite scales fine to tens of thousands of rows, which is more than a personal automation habit will produce in years.

## 7. Multi-machine sync

The one thing that doesn't fit cleanly into git is the SQLite file — binary, and concurrent writers from different machines will conflict. Recommended approach: **each machine keeps its own local `automation.db`**, never committed (it's in `.gitignore`). Everything that needs to travel between machines already does, through git: the code, the manifests, the schedules, the config.

If you later want a single cross-machine view of run history (e.g., "show me every failed run this week across all three machines"), the lightest add-on is **Litestream**, which continuously streams SQLite's WAL to an S3-compatible bucket (Backblaze B2, which you may already be using for backups per §2, works well) — each machine gets its own replicated file, and a periodic job can merge them into one queryable view without you managing a server. This is optional; skip it until you actually feel the need to query across machines.

## 8. Bootstrapping a new machine

`install.sh` (POSIX shell, with a `.ps1` twin for Windows) does, in order: clone the repo, install `pre-commit` and language toolchains if missing, create an empty local `automation.db` with the schema from §6, prompt for/copy this machine's `.env` and `data/config/machines/<hostname>.yaml`, and run `automate schedule sync`. A brand-new machine goes from bare OS to fully scheduled and logging in one script run.

## 9. Rollout — suggested order

1. Scaffold the directory structure and empty `registry/`, `tools/`, `bin/` above; init the SQLite schema.
2. Write `tools/build_catalog.py` and `tools/validate_manifests.py` first — they're what makes every later step self-enforcing.
3. Migrate 2-3 of your most-used existing scripts into `scripts/<category>/<name>/` with manifests, as a pattern to copy.
4. Write `lib/*/run_wrapper.*` and point `bin/automate run` through it so history logging starts immediately.
5. Write `tools/schedule_sync.py` for whichever OS you're on first, then extend to the other two.
6. Set up the GitHub private repo, push, wire up `.github/workflows/ci.yml` and pre-commit hooks.
7. Write `install.sh`, then bootstrap your second and third machines with it.
8. Backfill the rest of your existing scripts, one at a time, into the structure.

I can scaffold steps 1-4 directly in this repo now if you want — say the word and I'll generate the actual files (CLI, wrapper, schema, a couple of migrated example scripts) rather than just the plan.
