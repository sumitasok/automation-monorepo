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
| `entrypoint` | yes | file to run, relative to the job folder |
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
