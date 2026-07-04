# ADR 0005 — Data store: version definitions, not produced data

**Status:** accepted — 2026-07-04 (supersedes the SQLite-in-git parts of ADR 0001 §7)

## Context

The initial design tracked `data/state/*.sqlite` in git (via git-lfs) so the
data store would "sync across machines." On review this is wrong for most of what
those DBs hold. `runs.sqlite` is run history — it is produced by, and specific
to, the machine that ran the job. Committing it (and DBs like it) means:

- constant conflicts: SQLite is binary, so git cannot merge two machines' writes;
- history bloat: every run rewrites the file, so every run is a new blob;
- meaningless cross-machine data: one machine's run log isn't the other's.

The user correctly flagged that this is user/run-specific data that shouldn't be
in version control.

## Decision

Split by provenance, not by file type:

- **Definitions → versioned.** Things you author to configure automations —
  `data/config/*.yaml`, manifests, schema migrations — are inputs, belong in git,
  and sync on `git pull`.
- **Produced data → machine-local, git-ignored.** `data/state/*.sqlite` (run
  history, caches, scraped rows, metrics) is output. It stays local and is
  excluded via `.gitignore`. SQLite remains the store; it just isn't carried in
  git.
- **Must-sync datasets → diffable export, not the binary.** If a specific dataset
  genuinely needs to travel, commit a text export (`data/shared/<name>.jsonl` /
  CSV / YAML) and rebuild the local SQLite view from it. Git merges text.
- **Concurrent multi-writer / large data → hosted Postgres.** The manifest
  `data:` block means only the connection target changes.

`.gitattributes` no longer LFS-tracks `*.sqlite`/`*.db`; LFS is reserved for
intentionally-shared binary *assets* (e.g. a reference PDF shipped in a pack).

## Consequences

- No binary-merge conflicts, no history bloat from run data.
- "Sync the data store" now means "sync versioned config + optional diffable
  exports," which is what git is actually good at.
- Revises ADR 0001 §7 and the one-writer-per-DB rule there: with state kept
  local, the single-writer constraint only applies to the opt-in shared-export
  path, not to every DB.
