# Manifest schema

Every job has a `manifest.yaml` — the single source of truth. Fields:

| field | required | notes |
|-------|----------|-------|
| `id` | yes | unique across ALL packs, kebab-case, never change once set |
| `name` | yes | human-readable |
| `description` | yes | 1–2 sentences |
| `category` | no | groups jobs in the catalog/on disk |
| `owner` | no | who maintains it |
| `language` | yes | `python` \| `bash` \| `node` \| `go` |
| `entrypoint` | one of | SCRIPT job: file to run, relative to the job folder |
| `exec` | one of | APP-BACKED job: shell command to run (e.g. `go run . discover`) |
| `workdir` | no | for `exec`: directory to run in, relative to the **pack root** (default = pack root) |
| `tags` | no | list, used by `auto search` |
| `visibility` | no | `private` \| `shared` \| `public`; inherits pack default if omitted |
| `runs_on.os` | no | platforms; default all three |
| `runs_on.machines` | no | `[any]` or explicit ids from `machines.yaml` |
| `schedule.cron` | no | 5-field cron; empty = manual only |
| `schedule.timezone` | no | IANA tz |
| `schedule.enabled` | no | `auto schedule sync` installs only enabled jobs |
| `runtime.timeout_seconds` | no | enforced by the run wrapper |
| `runtime.env` | no | names of required env vars (values live outside git) |
| `data.reads` / `data.writes` | no | documents what it touches in `data/` |
| `health.expect_run_every` | no | e.g. `1d`; blank = not health-checked |

**Visibility rule (enforced by `auto doctor`):** a job with `visibility:
private` may only live in a pack whose `default_visibility` is `private`. This is
what prevents private automations from leaking into a shared/public pack.

## Two job shapes

A job must define **exactly one** way to run:

- **Script job** — `entrypoint: main.py`. A few files, one entrypoint. Runs in
  the job folder. Good for small standalone scripts.
- **App-backed job** — `exec: "go run . discover"` + optional `workdir`. The job
  is a thin descriptor over a whole application (its own repo/pack, many
  directories, its own build). The manifest doesn't *contain* the app — it
  points at a runnable command inside it. One app typically exposes several jobs,
  one per subcommand. See `docs/adr/0006` and `docs/CASE-STUDY-gmail-app.md`.
