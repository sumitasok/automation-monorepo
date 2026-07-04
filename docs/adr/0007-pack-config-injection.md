# ADR 0007 — Unified pack config: sample in the pack, values in the workspace, injected as env

**Status:** accepted — 2026-07-04

## Context

Packs (especially app-backed ones like `gmail`) need configuration and secrets to
run: API keys, OAuth `credentials.json`, tokens. These can't live in the pack repo
(it may be shared/public) and shouldn't live in the workspace repo either (it's
versioned). The `gmail-discover` run failed with `credentials.json: no such file`
precisely because there was no unified way to get config *into* a pack before it
runs. We need one mechanism, identical for every pack.

## Decision

A three-part contract, with the pack **declaring** its needs and the workspace
**supplying** the values, injected into the process environment before the pack
is called:

1. **Sample in the pack (committed).** Each pack ships `config.sample.yaml` at its
   root:
   ```yaml
   env:                     # env vars the pack reads, with placeholders
     ANTHROPIC_API_KEY: ""
   files:                   # secret files the pack reads from its workdir
     - credentials.json
     - token.json
   ```
   It contains no secrets — only declarations — so it is safe to commit and
   travels with the pack for anyone who mounts it.

2. **Values in the workspace (git-ignored).** Real values live in
   `config/<pack>/` in the workspace: `config.yaml` for env overrides, plus the
   actual secret files. The workspace `.gitignore` excludes `config/*` (all but
   its README), so these values are **never** versioned in either repo.
   `auto config init <pack>` scaffolds this from the sample.

3. **Injected as env before the pack runs.** `auto run <job>`:
   - merges env, precedence low→high: pack sample defaults < workspace
     `config/<pack>/config.yaml` < the ambient shell environment;
   - exports the merged env plus `AUTO_PACK_CONFIG_DIR` into the job process;
   - symlinks each declared `file` from `config/<pack>/` into the job's workdir,
     so file-based apps (which read e.g. `./credentials.json`) find them, and can
     write through (e.g. `token.json`) so runtime state persists in the workspace
     config dir across submodule re-clones.

Introspection: `auto config <pack>` reports which env keys and files are set vs.
missing.

## Why env (and symlinked files) rather than editing the pack

The pack must stay generic and shareable. Injecting via environment means the
pack is configured *from the outside* at call time — no pack code changes, no
per-user forks. For apps that read files rather than env (like gmail's OAuth
files), the symlink bridges the same workspace config dir into the workdir
without copying secrets into any repo. Apps that already read a path from env
(e.g. `ANTHROPIC_API_KEY`) need nothing extra.

## Consequences

- One identical config flow for every pack: `config.sample.yaml` → `auto config
  init` → fill `config/<pack>/` → `auto run`.
- Secrets live in exactly one place (`config/<pack>/`), git-ignored, and reach
  the pack only at runtime.
- A pack must `.gitignore` its declared `files` in its own repo (gmail already
  does) so a symlinked secret is never committed upstream.
- Complements ADR 0005 (produced data local) and ADR 0006 (apps as packs): code
  ships in the pack, values stay in the workspace.
