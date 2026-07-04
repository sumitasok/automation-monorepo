# ADR 0006 — Applications are packs; their subcommands are jobs

**Status:** accepted — 2026-07-04

## Context

The "job = a folder with a `manifest.yaml` and one entrypoint script" model
(ADR 0001) fits small scripts. But a real automation is often a full
**application**: e.g. `sa.automation.gmail` is a Go program with `main.go`,
several packages (`discover/`, `parser/`, `auth/`, `store/`, `config/`), its own
`go.mod`, its own git repo (`git@github.com:sumitasok/sa.automation.gmail.git`),
its own config directory (`filters/`), its own build, and multiple runnable
subcommands (`discover`, `recategorize`, and a default transaction extractor).

The question this raised: *doesn't a single application span many directories —
so how does it live in a "monorepo" of one-folder jobs?*

## Decision

Do **not** flatten an application into the job layout. Instead:

1. **An application is a pack.** It stays its own git repo with its own internal
   structure, build, and history, and is mounted into a workspace as a submodule
   (ADR 0002) — exactly like any other pack. The framework never dictates an
   app's internal directory layout.
2. **Each runnable entrypoint of the app is a job.** A thin `manifest.yaml`
   *describes* how to run one subcommand; it does not contain the app. One app
   → several jobs (`gmail-extract`, `gmail-discover`, `gmail-recategorize`).
3. **Manifests use `exec` + `workdir` for app-backed jobs** (added this ADR):
   `exec` is the command to run (`go run . discover`, or a built binary),
   `workdir` is where to run it, relative to the pack (app) root. This replaces
   the single-`entrypoint`-file assumption. A job now defines *exactly one* of
   `entrypoint` (script) or `exec` (app-backed); `auto doctor` enforces this.

So a "monorepo" here means **one workspace that composes many independently-
versioned application repos as packs** — not one giant flat tree. An app being
spread across many directories is correct and expected; it's a pack.

## Where the manifests live

Two placements, both supported:

- **App-owned (preferred when you own the app):** put `pack.yaml` + a `jobs/`
  tree *inside* the app repo. The app self-describes and its automations travel
  with it. Used for `sa.automation.gmail`.
- **Overlay (for third-party apps you don't own):** keep the app pristine; add a
  pack in your workspace whose job manifests `exec` into an external checkout
  via `workdir`. Nothing is added to the upstream repo.

## Visibility for app-backed packs

Visibility is evaluated per job/pack as usual, but an application cleanly
separates **code** (often shareable) from **private inputs** (credentials,
personal config, produced data). The pattern: the app/pack can be `shared` or
`public` (the code), while each user supplies their own secrets and config
locally and keeps produced data out of git (ADR 0005). This is why
`sa.automation.gmail` already `.gitignore`s `credentials.json`, `token.json`,
`output/`, `*.state`, and `email_catalog.csv`.

## Consequences

- Large, multi-directory applications are first-class citizens, not awkward
  exceptions.
- The catalog lists an app's subcommands as discrete, individually schedulable
  jobs.
- The run wrapper (logging, timeout, history) works uniformly for a one-file
  script and a compiled Go app alike.
- Existing apps adopt the framework additively: add `pack.yaml` + manifests, no
  code changes. See `docs/CASE-STUDY-gmail-app.md`.
