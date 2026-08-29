# Feature Specification: Workspace Config Defaults (`config/config.yaml`)

**Feature Branch**: `005-workspace-config-defaults` (implemented directly against `main`, not a dedicated worktree/branch — see Assumptions)

**Created**: 2026-08-29

**Status**: Implemented

**Input**: User description: a same-day follow-up to a bug fix. First: forwarded output of a failed `gmail-extract` run (`writing CSV: opening CSV for write: open transactions.csv: operation not permitted`), diagnosed to a write-sandbox bug (ADR 0018 Amendment 3) — the sandbox's allow-listed `data/` root wasn't `.resolve()`d, so a `data/` symlink pointing outside the repo silently made every write under it fail. Then, verbatim: "lets create a config/config.yaml and start adding some defauult configs that auto asumes when no extra params are passed. so we will define data directory location there instead of the symlink based assumtion."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Point `auto` at real data without a symlink trick (Priority: P1)

As the workspace owner, I want to tell `auto` explicitly where my produced data lives (it's kept outside the repo, e.g. for its own backup/sync policy), instead of having to make `data/` a symlink at a conventional path and trust that every code path resolving `DATA` follows it correctly.

**Why this priority**: This is the entire ask. The symlink convention was never declared anywhere `auto` could check it against — it just happened to work until the write-sandbox (ADR 0018) generated Seatbelt rules from the unresolved symlink path and every write under `data/` started failing.

**Independent Test**: With no `config/config.yaml` present, confirm `auto` still finds data via the legacy `data/` symlink (no regression). Create `config/config.yaml` with `data_dir: /Users/sumitasok/data` and confirm `auto` now resolves data through that explicit value instead — with `pack_data_dir('gmail')`, `AUTO_DATA_DIR`, and the write-sandbox's allow-list all agreeing.

**Acceptance Scenarios**:

1. **Given** no `config/config.yaml` exists, **When** `auto` computes where data lives, **Then** it falls back to `(workspace)/data`, resolved — unchanged from before this feature.
2. **Given** `config/config.yaml` sets `data_dir: /Users/sumitasok/data`, **When** `auto` computes where data lives, **Then** it uses that path directly, and every consumer (`pack_data_dir()`, the `AUTO_DATA_DIR` env var injected into jobs, the write-sandbox's allow-listed roots) agrees on the same resolved location.
3. **Given** `data_dir` is set to a path relative to the workspace root, or one starting with `~`, **When** `auto` resolves it, **Then** it's expanded/joined against the workspace root and resolved to an absolute, symlink-free path — the same guarantee the fallback already had (ADR 0018 Amendment 3).

---

### User Story 2 - A place for future workspace-wide defaults (Priority: P2)

As the workspace owner, I want one file where settings that belong to the workspace itself (not to any single pack) can be declared going forward, rather than each new one inventing its own env-var-or-symlink convention.

**Why this priority**: Named directly in the request ("start adding some default configs that auto assumes") — `data_dir` is the first key, not the only one this file is meant to ever hold.

**Independent Test**: Confirm `config/config.example.yaml` (committed) documents the file's purpose and its one current key in a way a future key can be added to without restructuring anything.

**Acceptance Scenarios**:

1. **Given** a workspace has never created `config/config.yaml`, **When** `auto` runs any command, **Then** nothing breaks — every key this file can set has a defined fallback.
2. **Given** someone wants to add a new workspace-wide default later, **When** they look at `config/config.yaml`, **Then** they find one file, one loader (`load_workspace_config()`), and one documented template (`config/config.example.yaml`) to extend — not a new symlink-and-hope convention.

### Edge Cases

- `config/config.yaml` exists but `data_dir` is empty/unset (e.g. someone copied the example verbatim without filling it in): treated the same as the key being absent — falls back to the `data/` symlink convention, not an error.
- `data_dir` points at a path that doesn't exist yet: not validated at load time; the existing `pack_data_dir(name).mkdir(parents=True, exist_ok=True)` call in `execute_job()` creates it (and pack subdirectories) on first use, same as before this feature.
- `data_dir` itself turns out to be (or contain) a symlink: still safe — the value is `.resolve()`d before becoming `DATA`, same guarantee as the legacy fallback (ADR 0018 Amendment 3).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `auto` MUST read `config/config.yaml` at the workspace root, if it exists, before computing where produced data lives.
- **FR-002**: `auto` MUST use `data_dir:` from that file, when set and non-empty, as the produced-data root — accepting an absolute path, a workspace-relative path, or a `~`-prefixed path.
- **FR-003**: `auto` MUST resolve the produced-data root (whether from `data_dir:` or the legacy fallback) to an absolute, symlink-free path, so the write-sandbox's allow-list (ADR 0018), `pack_data_dir()`, and the `AUTO_DATA_DIR` env var injected into every job's process all agree on the identical location.
- **FR-004**: `auto` MUST fall back to the pre-existing `data/`-directory-at-workspace-root convention when `config/config.yaml` is absent or does not set `data_dir` — a workspace that has not adopted this feature keeps working unchanged.
- **FR-005**: The workspace MUST ship a committed template (`config/config.example.yaml`) documenting every key `auto` reads from this file, and the real file (`config/config.yaml`) MUST be git-ignored, matching this workspace's existing config/pack-config split (ADR 0007) — nothing machine-local versioned, nothing secret or path-specific left undocumented for the next person.

### Key Entities

- **Workspace config (`config/config.yaml`)**: workspace-wide settings `auto` assumes when nothing more specific (a pack's own config, a CLI flag, an env var) overrides them. Distinct from `config/<pack>/config.yaml` (per-pack secrets/env, ADR 0007) and `config/ai/<name>.yaml` (named AI provider profiles, ADR 0015). One key today (`data_dir`); the intended home for future ones.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: On this workspace, `DATA` (and everything derived from it) resolves to `/Users/sumitasok/data` whether `config/config.yaml` is present (explicit `data_dir`) or absent (symlink fallback) — both paths converge on the same real location, confirmed by direct module inspection.
- **SC-002**: `auto doctor` and a full `python3 -m py_compile` of `framework/tools/auto` pass with `config/config.yaml` both absent and present.
- **SC-003**: `gmail-extract` (and any other job writing under `data/`) no longer depends on a `data/` symlink existing at the workspace root to find its real data directory.

## Assumptions

- This was implemented as a direct, same-session follow-up to a live production bug fix (ADR 0018 Amendment 3), not run through the full `/speckit-plan` → `/speckit-tasks` → `/speckit-implement` pipeline — there is no `plan.md`/`tasks.md`/`research.md` alongside this `spec.md`. The design rationale lives in `docs/adr/0021-workspace-config-yaml.md`; the session-by-session implementation log lives in `RUNBOOK.md` (2026-08-29 entries).
- No dedicated git branch or worktree was created for this change (unlike `001`–`004`, which reference `feature/...` branches in `RUNBOOK.md`) — it was made directly against `main` in the same session as the bug fix it follows from.
- `data_dir` is the only key this feature introduces. Other workspace-wide defaults mentioned as a future direction (e.g. a default `--ai` profile, a default machine id) are explicitly out of scope here — `config/config.example.yaml` and `load_workspace_config()` exist to make adding them straightforward later, but none are defined yet.
- The pre-existing `data/` symlink at the workspace root was deliberately left in place rather than removed — it's redundant for `auto`'s purposes now, not wrong, and still useful for a human `cd`-ing into it out of habit.
