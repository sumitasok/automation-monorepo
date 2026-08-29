# ADR 0021 — `config/config.yaml`: explicit workspace-wide defaults, starting with `data_dir`

**Status:** accepted — 2026-08-29

## Context

ADR 0018 Amendment 3 traced a real `gmail-extract` failure
(`operation not permitted` writing `transactions.csv`) to the write-sandbox's
allow-list not resolving symlinks: this workspace's `data/` directory is
itself a symlink to `/Users/sumitasok/data`, kept outside the repo (likely
so produced data can live somewhere with its own backup/sync policy,
independent of the git repo's lifecycle). The immediate fix was to
`.resolve()` every write root before generating the sandbox profile — that
patches the symptom for *this specific* symlink-based layout, but the
underlying design (there is no way to *tell* `auto` where data actually
lives, other than making `data/` a symlink and hoping every code path that
touches `DATA` resolves it correctly) stays fragile. Amendment 3 already
found five call sites that needed fixing once; a future one is one grep away
from reintroducing the exact same bug.

Sumit asked, directly, for a workspace `config/config.yaml` — analogous to
the per-pack `config/<pack>/config.yaml` (ADR 0007) — where `auto` can be
*told* things like the data directory's real location, rather than having
to infer it from a symlink at a conventional path and hope every consumer
resolves it consistently.

## Decision

1. **New `load_workspace_config()`**, reading `config/config.yaml` (via
   `CONFIG_ROOT`, itself already `.resolve()`d per Amendment 3) once at
   module load, alongside `WS`/`CONFIG_ROOT`/`DATA`. Missing file → `{}` —
   every key this file can set has a defined fallback, so it's entirely
   optional, and a workspace that has never heard of ADR 0021 keeps working
   exactly as before.

2. **`data_dir:` is the first (and, for now, only) key.** `_resolve_data_dir()`
   uses it when set — absolute path, or relative to the workspace root, `~`
   expanded, `.resolve()`d for the same reason every other write root is
   (Amendment 3) — and falls back to the pre-existing `(WS / "data").resolve()`
   convention when it isn't. `DATA` is computed from this function once, so
   every consumer (`pack_data_dir()`, `AUTO_DATA_DIR` injection,
   `_sandbox_write_roots()`) automatically agrees, by construction, instead
   of each needing to know to resolve a possibly-symlinked path itself.

3. **Committed template, git-ignored real file — same split as every other
   config in this workspace.** `config/config.example.yaml` documents
   `data_dir` (and is where future workspace-wide keys get documented as
   they're added) and is versioned; `config/config.yaml` holds the real,
   machine-local value and is git-ignored (`config/*` already covered it —
   this ADR just adds the `!config/config.example.yaml` exception, mirroring
   `!config/README.md` and `config/ai/*.example.yaml`). No `auto config
   init` step for this one file — unlike per-pack config, there's no pack to
   scope an `init <pack>` subcommand to, and the file is simple enough
   (right now, one key) that `cp config/config.example.yaml config/config.yaml`
   documented in `config/README.md` is enough.

4. **The `data/` symlink itself is left in place, not migrated away.** Once
   `data_dir` is set, `auto` no longer looks at `data/` at all — but a human
   (or an editor, or a shell alias) `cd`-ing into `automation-monorepo/data/`
   out of habit still lands in the right place, since the symlink still
   points at the same real directory `data_dir` now names explicitly. It's
   redundant for `auto`'s purposes, not wrong.

## Consequences

- Verified: with no `config/config.yaml`, `DATA` still resolves to
  `/Users/sumitasok/data` (unchanged from Amendment 3 — the symlink
  fallback). With `config/config.yaml` containing
  `data_dir: /Users/sumitasok/data`, `DATA` resolves to the same value via
  the new, explicit path — confirming the override and the fallback agree
  when they're describing the same real location.
- This workspace's real `config/config.yaml` now sets
  `data_dir: /Users/sumitasok/data` explicitly. The `data/` symlink at the
  workspace root still exists (kept for manual navigation, per decision 4)
  but `auto` no longer depends on it.
- Opens the door for other workspace-wide settings (a default `--ai`
  profile, a default machine id override, ...) to live in the same file
  rather than each inventing its own env-var-or-symlink convention — nothing
  else is defined yet, but `load_workspace_config()` / `config.example.yaml`
  are the place future ones go.
- Complements ADR 0007 (the per-pack config/data_files split this mirrors at
  workspace scope), ADR 0018 (the write-sandbox whose roots this now feeds,
  Amendment 3 especially), and ADR 0019 (`data_files:`, unaffected — packs
  still declare data files the same way; only where `data/` itself resolves
  to changed).
