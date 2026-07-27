# Contract: Dashboard HTTP API

Served by `auto serve` on `127.0.0.1:<port>` (default 4321). Single local operator, no auth — see research §9 for why, and for the controls that replace ADR 0012's "it's read-only" justification.

## Shared rules

- **Every request** must carry a `Host` header whose hostname is `127.0.0.1`, `localhost`, or `[::1]`. Anything else → `403` (DNS-rebinding defense).
- **`GET` never has side effects.** No run can be started by loading, refreshing, or prefetching a URL (FR-029).
- **All mutations are `POST`** with a JSON body and `Content-Type: application/json`.
- Errors return a JSON body: `{"error": "<code>", "message": "<human text>"}`.

---

## `GET /`

The dashboard page: the existing packs/config/jobs/commands content, plus the new **Actions** and **Runs** sections.

## `GET /api/actions`

Everything launchable, rebuilt from source per request.

```json
{
  "tmux_available": true,
  "ai_profiles": [ {"name": "deepseek", "provider": "deepseek", "usable": true} ],
  "actions": [
    {
      "kind": "job", "id": "gmail-extract",
      "name": "Gmail transaction extract",
      "description": "Fetch bank alert emails and extract transactions to transactions.csv",
      "accepts_ai": true, "danger": false,
      "available": true, "unavailable_reason": null
    },
    {
      "kind": "command", "id": "schedule-sync",
      "name": "Install scheduled tasks",
      "description": "Write this machine's cron/launchd entries from job manifests",
      "accepts_ai": false, "danger": true,
      "available": true, "unavailable_reason": null
    }
  ]
}
```

`tmux_available: false` means no action can be launched; the UI must surface the install instruction rather than offering dead buttons (FR-017).

## `POST /api/runs`

Start a run. Returns immediately — it does not wait for the run to finish.

**Request**:
```json
{ "kind": "job", "id": "gmail-categorize", "ai_profile": "deepseek", "confirm": false }
```

`ai_profile` is optional and only honored when the action's `accepts_ai` is true. `confirm` **must** be `true` for an action whose `danger` is true — enforced server-side, so the UI dialog cannot be bypassed by posting directly (FR-005).

**Responses**:
- `201` — `{"audit_id": "r-20260726-153012-a1b2c3", "status": "running", "log_path": "logs/runs/r-….log"}`
- `409 already_running` — this action already has a run in progress (FR-006). Body names the existing `audit_id`.
- `412 confirm_required` — `danger` action posted without `confirm: true`.
- `422 unavailable` — action exists but cannot run here; body carries `unavailable_reason`.
- `404 unknown_action` — no such action.
- `503 tmux_missing` — tmux is not installed; body carries the install hint (FR-017).

## `GET /api/runs`

All runs, newest first. Reconciles stale `running` rows before returning (FR-016).

```json
{
  "runs": [
    {
      "audit_id": "r-20260726-153012-a1b2c3",
      "kind": "job", "action_id": "gmail-categorize",
      "ai_profile": "deepseek",
      "status": "running", "rc": null,
      "started_at": "2026-07-26T15:30:12Z", "ended_at": null,
      "elapsed_seconds": 42.7,
      "note": null
    }
  ]
}
```

## `GET /api/runs/{audit_id}`

One run, including orchestration step progress (empty for non-orchestration runs — FR-023).

```json
{
  "run": { "...": "as above" },
  "steps": [
    {"step_index": 0, "job": "gmail-extract", "status": "succeeded", "rc": 0},
    {"step_index": 1, "job": "gmail-categorize", "status": "running", "rc": null}
  ]
}
```

`404 unknown_run` if the id is not known.

## `GET /api/runs/{audit_id}/log?from={byte_offset}`

Incremental log fetch — the live-tail primitive (FR-019). The client polls this ~1s with the offset it last received.

```json
{ "from": 4096, "next": 8192, "eof": false, "text": "…new log content…" }
```

- `next` is the offset to send on the following poll.
- `eof: true` means the run has finished **and** the client has read to the end — the client stops polling.
- Returning `from == next` with empty `text` is normal (no new output yet).
- Works identically for a finished run, so the same endpoint serves history (FR-021).

## `POST /api/runs/{audit_id}/cancel`

Stop an in-progress run by killing its tmux session; the run is recorded as `cancelled` (FR-014).

- `200` — `{"audit_id": "…", "status": "cancelled"}`
- `409 not_running` — the run already reached a terminal status.
- `404 unknown_run`.

---

# Contract — Revision 2 (2026-07-27)

## Startup precondition (new)

`auto serve` resolves and validates the data and configuration directories **before binding a port** (FR-020). If either is missing or invalid it prints the same message the CLI gives and exits non-zero — there is no degraded mode. Every endpoint below therefore assumes valid directories; there is no `dirs_missing` runtime error.

This differs deliberately from the tmux-missing case, which *does* start and degrade: an operator with no tmux can still usefully read the dashboard, whereas one with no data directory has nothing trustworthy to show.

## `GET /api/actions` — changed

Adds a session block; `accepts_ai` is **removed** from each action (the profile is no longer per-action).

```json
{
  "tmux_available": true,
  "session": {
    "ai_profile": "deepseek",
    "data_dir": "/Users/…/automation-monorepo/data",
    "config_dir": "/Users/…/automation-monorepo/config"
  },
  "ai_profiles": [ {"name": "deepseek", "provider": "deepseek", "usable": true} ],
  "actions": [
    {
      "kind": "job", "id": "gmail-extract",
      "name": "Gmail transaction extract",
      "description": "Fetch bank alert emails and extract transactions to transactions.csv",
      "danger": false, "available": true, "unavailable_reason": null, "note": null
    }
  ]
}
```

`session.ai_profile` is the currently selected profile, or `null`. `data_dir`/`config_dir` are shown so the operator can confirm at a glance which directories the dashboard is bound to.

## `PUT /api/session/ai-profile` — new

Sets the session-wide AI profile (FR-007). A mutation, so `POST`-class rules apply: JSON body, loopback `Host` required.

**Request**: `{ "ai_profile": "deepseek" }` — or `{ "ai_profile": null }` to clear.

**Responses**:
- `200` — `{"ai_profile": "deepseek"}`
- `422 unknown_profile` — no such profile, or it exists but fails validation (FR-012). Body names the profile.

The selection lives in server memory for the lifetime of the process; it is not persisted across restarts (spec Assumptions).

## `POST /api/runs` — changed

`ai_profile` is **no longer accepted in the body**. The run uses whatever the session-wide selection is at launch time.

**Request**: `{ "kind": "job", "id": "gmail-categorize", "confirm": false }`

**New response**:
- `422 unknown_profile` — a profile was selected but no longer resolves to usable credentials; the run is refused rather than started with half-resolved credentials (FR-012).

Unchanged: `201`, `409 already_running`, `412 confirm_required`, `422 unavailable`, `404 unknown_action`, `503 tmux_missing`.

## `GET /api/runs` and `GET /api/runs/{audit_id}` — changed

Each run object gains `data_dir` and `config_dir` (FR-021). Runs recorded before this revision report `null` for both.

```json
{ "audit_id": "r-…", "ai_profile": "deepseek",
  "data_dir": "/Users/…/data", "config_dir": "/Users/…/config", "…": "…" }
```
