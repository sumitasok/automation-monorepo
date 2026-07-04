# Data store

The rule (ADR 0005): **git versions what defines your automations; it does not
version the data they produce.**

- **`config/` — VERSIONED YAML.** Settings, lookup tables, small structured data
  you hand-edit. Input to jobs. Committed, syncs on `git pull`.
- **`state/` — MACHINE-LOCAL SQLite, git-ignored.** Run history, caches, scraped
  rows, metrics. Output a job accumulates; specific to this machine. Not
  committed (see `.gitignore`). Rebuilt from inputs, not carried in git.
- **`inbox/` — scratch, git-ignored.**

Need a dataset to sync across machines? Don't commit the `.sqlite`. Export to a
diffable `data/shared/<name>.jsonl` (or CSV/YAML), commit that, and rebuild the
local SQLite view from it — git merges text cleanly. If you need concurrent
multi-machine writes, graduate that dataset to a hosted Postgres on the server;
the job's manifest `data:` block means only the connection target changes.

Schema (not contents) can be versioned via `migrations/*.sql`.
