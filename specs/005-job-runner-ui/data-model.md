# Data Model: One-Click Job Runner UI

## Storage locations

| What | Where | Notes |
|---|---|---|
| Run records | `data/state/runs.sqlite`, **new tables** | Same DB `_record_run` already uses. The existing `runs` table is **not** modified. |
| Run logs | `logs/runs/<audit_id>.log` | Plain text. `logs/` and `*.log` are already in `.gitignore` (FR-027 needs no change). |

## Entity: Action (derived, not stored)

Rebuilt from source on every request (ADR 0012 §3's "never drift" rule), so a newly added job or orchestration appears with no registration step.

| Field | Meaning |
|---|---|
| `kind` | `job` \| `orchestration` \| `command` |
| `id` | Job id, orchestration name, or command key |
| `name` | Human-readable label for the button |
| `description` | Shown under the button (from the manifest / orchestration YAML / fixed table) |
| `accepts_ai` | True only for `kind == "job"` (research §7) |
| `danger` | True for `schedule sync` and `bootstrap` — requires confirmation (FR-005) |
| `available` | False when the action cannot run here |
| `unavailable_reason` | Why, shown to the user before they click (FR-003) |

**Availability rules** (FR-003): a job is unavailable when `job_runs_here()` is false (wrong OS or wrong machine, per its `runs_on`) or its pack directory is missing. An orchestration is unavailable when `validate_orchestration()` returns problems. Commands are always available.

## Entity: Run (stored)

```sql
CREATE TABLE IF NOT EXISTS ui_runs(
  audit_id     TEXT PRIMARY KEY,   -- e.g. "r-20260726-153012-a1b2c3"
  kind         TEXT NOT NULL,      -- job | orchestration | command
  action_id    TEXT NOT NULL,
  ai_profile   TEXT,               -- profile NAME only, never a credential
  status       TEXT NOT NULL,      -- running | succeeded | failed | timed_out | cancelled
  rc           INTEGER,            -- NULL while running
  started_at   TEXT NOT NULL,      -- ISO-8601 UTC
  ended_at     TEXT,               -- NULL while running
  host         TEXT NOT NULL,
  tmux_session TEXT NOT NULL,      -- "auto-<audit_id>"
  log_path     TEXT NOT NULL,      -- relative to workspace root
  note         TEXT                -- e.g. abnormal-termination reason
);
CREATE INDEX IF NOT EXISTS ui_runs_started ON ui_runs(started_at DESC);
```

**Status transitions**:

```
                        ┌──> succeeded   (wrapper recorded rc == 0)
running ────────────────┼──> failed      (rc != 0, OR session vanished with no rc — FR-016)
   │                    ├──> timed_out   (rc == 124, matching execute_job's timeout convention)
   │                    └──> cancelled   (user stopped it — FR-014)
   └── only while the tmux session exists (research §5)
```

`running` is never trusted on its own: any row reading `running` whose tmux session is absent and whose `rc` is NULL is reconciled to `failed` with a `note` (FR-016/SC-005).

## Entity: Run Step (stored, orchestrations only)

Populated from the guarded step markers described in research §10. Individual jobs have no steps — for them, "progress" is the live log plus status (per the spec's Assumptions).

```sql
CREATE TABLE IF NOT EXISTS ui_run_steps(
  audit_id   TEXT NOT NULL,
  step_index INTEGER NOT NULL,
  job        TEXT NOT NULL,
  status     TEXT NOT NULL,   -- running | succeeded | failed
  started_at TEXT NOT NULL,
  ended_at   TEXT,
  rc         INTEGER,
  PRIMARY KEY (audit_id, step_index, started_at)
);
```

## Entity: AI Profile (derived, not stored)

| Field | Meaning |
|---|---|
| `name` | File stem of `config/ai/<name>.yaml` |
| `provider` | `deepseek` \| `claude` — shown as a hint next to the name |
| `usable` | False if the file fails `load_ai_profile` validation |

Excludes `*.example.yaml` (research §8). **Credential values are never included in this structure** — only the name and provider ever leave the server (FR-011/SC-007).

## Audit ID format

`r-<YYYYMMDD>-<HHMMSS>-<6 random hex>` — e.g. `r-20260726-153012-a1b2c3`.

Chosen so that IDs sort chronologically as plain strings, read legibly in a log line, and stay collision-free for runs started in the same second. It is the primary key, the tmux session suffix, and the log filename stem — one identifier the user can follow across all three (FR-024).

## Log line format

```
2026-07-26T15:30:12.482Z r-20260726-153012-a1b2c3 >> running gmail-extract [gmail] in /…: go run . 
```

`<ISO-8601 UTC timestamp> <SPACE> <audit_id> <SPACE> <original line>`. Every line carries the ID so that concatenating or grepping several runs' logs together never leaves a line ambiguous (FR-025/SC-004).

---

# Data Model — Revision 2 (2026-07-27)

## Entity: Workspace Directory Pair (new, derived per invocation)

Resolved once per process, before any work. Not persisted as its own record — it is captured onto each Run (below).

| Field | Source (in precedence order) | Validation |
|---|---|---|
| `data_dir` | `--data-dir` option → `AUTO_DATA_DIR` env | exists · is a directory · contains `state/` **and** `config/` |
| `config_dir` | `--config-dir` option → `AUTO_CONFIG_DIR` env | exists · is a directory · contains `ai/` **or** ≥1 mounted pack's subdirectory |

Resolution outcomes:

```
neither option nor env set        -> refuse: "missing", name the option and the env var
path absent / not a directory     -> refuse: "unusable", name the path and what it is
path present, structure wrong     -> refuse: "not a workspace <data|config> directory",
                                     naming the subdirectory that was expected
all checks pass                   -> bind DATA / CONFIG_DIR for this process
```

A refusal happens **before** any read or write (FR-018/SC-010) and always names the offending path plus what was expected (FR-011 → SC-011).

**Which commands resolve them** (research §15): `run`, `orchestrate`, `serve`, `schedule sync` require them. `list`, `packs`, `search`, `doctor`, `catalog`, `config`, `new`, `log`, `share`, `bootstrap` never resolve them and are unaffected (FR-019/SC-012).

## Entity: Run — added fields

```sql
ALTER TABLE ui_runs ADD COLUMN data_dir   TEXT;   -- nullable; absent on rev. 1 rows
ALTER TABLE ui_runs ADD COLUMN config_dir TEXT;   -- nullable; absent on rev. 1 rows
```

Both nullable and added via a `PRAGMA table_info`-guarded migration, so the run rows rev. 1 already wrote to `data/state/runs.sqlite` keep loading (repo convention: additive schema changes only). New runs always populate them (FR-021/SC-014).

## Entity: AI Profile — selection semantics changed

Unchanged in shape (`name`, `provider`, `usable`; credentials never leave the server). What changed is how a selection is applied:

| Rev. | Selection scope | Applied by | Pipeline step with its own `ai:` |
|---|---|---|---|
| 1 | one dropdown per job action | `--ai <name>` on `auto run` | unreachable — pipelines had no dropdown |
| **2** | **one session-wide control** | credential env vars injected into the run's process | **step's own profile wins** (research §11) |

At most one profile is selected at a time. "None selected" is valid and means the run uses whatever the environment already provides (FR-011). The selection is re-validated at launch and the run is refused if it no longer resolves (FR-012).
