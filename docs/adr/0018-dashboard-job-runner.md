# ADR 0018 — `auto serve` becomes a job runner (supersedes 0012's read-only decision)

**Status:** accepted — 2026-07-26
**Amended by:** [ADR 0019](0019-explicit-workspace-directories.md) — the per-action AI profile
dropdown described in point 8 below is replaced by a single session-wide control, applied by
environment injection so a pipeline step's own `ai:` still wins.

**Supersedes:** the read-only posture of [ADR 0012](0012-serve-dashboard.md) (points 3 and 5). The rest of 0012 — stdlib-only, regenerate-per-request, localhost bind, quiet logging — still stands.

## Context

ADR 0012 gave the workspace a local dashboard that read the same sources
`auto list` / `auto config` / `make help` do and rendered them as one page. It
was deliberately read-only: *"Nothing is written; there's no POST handler and
no state to get stale"*, and it justified having no authentication precisely
because nothing could be triggered through it.

That has become the limitation. Running anything still meant switching to a
terminal and remembering the exact invocation — which command, which flags,
which directory, and whether the AI profile goes on `auto run` (it does) or on
`auto orchestrate` (it does not — a real source of confusion). Meanwhile the
dashboard already knew about every job and pipeline; it simply refused to act
on that knowledge.

Spec `005-job-runner-ui` asked for the obvious next step: one-click buttons for
everything runnable, an AI-profile dropdown, isolated tmux sessions per run,
audit-tagged log files, and Jenkins-style live log viewing.

## Decision

1. **The dashboard executes actions.** `GET /` still renders the read-only
   content, and two new sections appear: **Run something** (a described button
   per job, pipeline, and safe `auto` command) and **Runs** (every run, with a
   live-tailing log view). A JSON API backs them (`contracts/dashboard-api.md`).

2. **Every run executes in its own detached tmux session**, named
   `auto-<audit_id>`, torn down when the run ends. tmux's default
   `remain-on-exit off` does the teardown by itself; liveness is derived from
   `tmux has-session` rather than a stored PID, so a recycled PID can't be
   mistaken for a live run.

3. **Runs are always non-interactive: stdin is redirected from `/dev/null`.**
   This is the single most important detail here. tmux allocates a real PTY per
   pane, which makes the packs' `isInteractive()` check (ADR 0017 / spec 003)
   return true — so `gmail-categorize` would stop at its rule-capture prompt and
   hang until it timed out, waiting on input no browser user can supply.
   Redirecting stdin puts dashboard runs on exactly the path those features
   already take on an unattended cron run. **A regression test asserts stdin is
   not a TTY**; if it ever fails, dashboard runs will hang. The consequence is
   accepted deliberately: the interactive-only features (`--suggest-similar`,
   rule capture) remain terminal-only.

4. **A run re-enters the real CLI rather than calling `execute_job()`
   in-process.** The wrapper (`auto _exec-run <audit_id>`, internal, absent from
   the usage block) shells out to `auto run <id> [--ai <p>]` /
   `auto orchestrate <name>` / `auto <command>`. A UI-launched run is therefore
   identical to a hand-typed one by construction — same history recording, pack
   config linking, profile injection, timeouts, and orchestration retry/loop
   semantics — instead of a second execution path that can drift.

5. **Audit ID on every log line.** Each run gets `r-<YYYYMMDD>-<HHMMSS>-<hex>`,
   used as the primary key, the tmux session suffix, and the log filename. Every
   line in `logs/runs/<audit_id>.log` is written as
   `<ISO-8601 UTC> <audit_id> <text>`, so concatenating or grepping several
   runs' logs never leaves a line ambiguous. `logs/` was already git-ignored.

6. **Live logs by byte-offset polling**, ~1s, over the existing
   `ThreadingHTTPServer` — no SSE, no WebSocket, no dependency. The same
   endpoint serves a finished run's history.

7. **New tables, not modified ones.** `ui_runs` / `ui_run_steps` sit alongside
   the existing `runs` table in `data/state/runs.sqlite`; `_record_run` is
   untouched, so terminal run history keeps working exactly as before.

8. **Security controls replace the "it's read-only" justification.** Since that
   premise is now false, the reasoning is re-derived rather than inherited:
   - still bound to `127.0.0.1` (0012 point 4 stands);
   - **all mutations are `POST`** — no run can be started by loading,
     refreshing, or prefetching a URL;
   - the **`Host` header must be a loopback name** — the standard defense
     against DNS rebinding, where a hostile page resolves its own domain to
     `127.0.0.1` and posts to a local service;
   - **state-changing maintenance actions** (`schedule sync`, `bootstrap`)
     require an explicit `confirm` in the request body, enforced server-side so
     the UI dialog cannot be bypassed;
   - **credential values never leave the server** — only profile *names* and
     providers are sent to the browser or written to logs.

   Still deliberately absent: accounts, passwords, TLS. Disproportionate for a
   single-operator local tool; if remote access is ever wanted, 0012's rule
   holds — that needs its own ADR covering auth.

9. **`runs_on` does not gate manual runs.** It is a *scheduling* constraint:
   `cmd_schedule` uses it to decide what to install on this machine, and
   `cmd_run` never consults it. The dashboard mirrors the CLI, showing machine
   pinning as an informational note rather than disabling the button — otherwise
   machine-pinned jobs like `gmail-extract` and `wallet-sync` would have been
   unclickable on the very laptop they're normally run from by hand.

## Consequences

- Launching anything is one click, with the AI profile picked from a dropdown of
  profiles that actually exist (`*.example.yaml` templates are excluded, since
  their placeholder keys would fail confusingly at run time).
- The dropdown appears **only for jobs**. Pipelines set their AI profile per
  step in their own YAML and `auto orchestrate` has no `--ai` flag, so offering
  one there would imply an override that does not exist.
- Several jobs read-modify-write the same shared CSVs, so a second concurrent
  run of the *same* action is refused with the in-flight run's audit id.
- A run whose session vanishes (machine sleep, out-of-band `kill-session`,
  dashboard restart) is reconciled to a definite status rather than showing
  "running" forever.
- `tmux` is now a hard prerequisite for launching anything. Its absence is
  detected and reported with an install hint; there is deliberately **no silent
  fallback** to a plain subprocess, because that would quietly change the
  isolation guarantee this ADR is built on.
- The action list is still derived per request, so a newly added job or pipeline
  appears as a button with no registration step — 0012's anti-drift property is
  preserved.
