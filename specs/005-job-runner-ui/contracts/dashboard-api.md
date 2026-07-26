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
