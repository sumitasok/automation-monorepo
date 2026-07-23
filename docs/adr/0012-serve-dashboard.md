# ADR 0012 — `auto serve` / `make serve`: local read-only workspace dashboard

**Status:** accepted — 2026-07-23

## Context

The workspace's state is spread across several `auto` subcommands you run one
at a time: `auto packs` (what's mounted), `auto config <pack>` (env/secret
status, once per pack), `auto list` (every job), and `make help` (which
Make targets exist and what they do, per Makefile — root plus one per pack).
There was no single place to see all of it at once, and no way to check it
without a terminal (e.g. from a phone on the same network, or a second
monitor left open while working).

## Decision

1. **A new `auto serve [--port N]` subcommand** (default port `4321`), wrapped
   by `make serve` (`PORT=` override) the same way every other Make target
   thinly wraps an `auto` call. It starts a local HTTP server and renders one
   HTML page combining: workspace/machine info, every mounted pack with its
   config status (mirrors `auto config <pack>`, run for all packs at once),
   the full job list (mirrors `auto list`), and every `target: ## help` line
   from every Makefile in the workspace (mirrors `make help`'s own
   grep/awk, generalized to per-pack Makefiles too), plus the `auto` CLI's
   own usage block.

2. **Stdlib only — `http.server`, no new dependency.** Consistent with `auto`
   itself (PyYAML is the one existing dependency); adding Flask/FastAPI for a
   single read-only page isn't justified.

3. **Read-only, regenerated on every request, no persistence.** The page is
   built fresh from `packs.yaml` / manifests / config / Makefiles on each
   `GET`, so it's always live — same "regenerate from source, never drift"
   principle as ADR 0001 (catalog/schedules regenerate from manifests) and
   `auto list`/`catalog`. Nothing is written; there's no POST handler and no
   state to get stale.

4. **Binds to `127.0.0.1`, not `0.0.0.0` — localhost-only, no auth.** The
   dashboard surfaces which secrets/env vars are *set* (not their values, per
   `cmd_config`'s existing convention) and full job/pack layout. That's
   workspace-internal information, fine on localhost, not something to expose
   on the network without at least auth in front of it. Widening this to
   `0.0.0.0` (e.g. to view from a phone) is a future change that should add
   auth first, not a default.

5. **Request logging suppressed** (`log_message` overridden to no-op) — the
   dashboard is polled/reloaded interactively; per-request access log lines
   in the terminal add noise without adding information for a read-only,
   localhost-only tool.

## Consequences

- One command (`make serve`) replaces "run four different `auto`/`make`
  invocations and read their terminal output" when you want the full picture.
- Because content is regenerated per-request straight from `packs.yaml`,
  manifests, `config/`, and every `Makefile`, the dashboard can't drift from
  `auto list` / `auto config` / `make help` — it calls the same loader
  functions (`load_packs`, `load_jobs`, `load_pack_config`) they do.
  Automatically reflects new packs, jobs, config keys, and Make targets when the
  workspace or a submodule changes, with no separate registration step.
- No new dependency, no build step — same "plain Python script" posture as
  the rest of `auto` (see README).
- Env values and secret *contents* are never rendered, only set/missing
  status per key/file — same redaction convention `cmd_config` already uses.
- If remote (non-localhost) access is wanted later, that needs its own ADR
  covering auth — this decision explicitly scopes to localhost.
