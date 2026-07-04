# Global data store

Git-synced, no server. Three zones:

- **`config/`** — YAML settings and small structured data. Human-edit these.
- **`state/`** — SQLite DBs for durable/growing/queryable data. One DB per
  domain. Tracked via git-lfs (see `.gitattributes`).
- **`inbox/`** — scratch / drop zone. Git-ignored.

Rules (see `docs/PLAN.md` §7):
- One writer machine per SQLite file (git can't merge binaries). Others read.
- Schema changes go in `data/migrations/*.sql`, applied by `auto data migrate`.
- A `data-sync` job on the server commits & pushes this folder on a schedule.
