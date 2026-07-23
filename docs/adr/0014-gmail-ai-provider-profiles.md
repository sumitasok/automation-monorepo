# ADR 0014 — Gmail pack: named AI provider profiles (--ai=&lt;name&gt;)

**Status:** accepted — 2026-07-23; **superseded same day by ADR 0015.** The
profile *format* (`provider`/`api_key`/`model`/`api_base`) and the gmail
pack's own `--ai=<name>` flag for direct/local runs are unchanged and still
described below — what moved is *where the profiles live* and *who injects
them*. Feedback: this was meant to be core `auto` functionality available to
every pack, not a gmail-only mechanism. `config/ai/<name>.yaml` is now the
workspace root (shared), and `auto run <job> --ai <name>` is the primary way
to use a profile; gmail's own flag now points at that same shared directory
by default instead of a gmail-local copy. See ADR 0015 for the full picture.

## Context

The gmail pack already calls out to an AI provider in two places: the
`--ai-assist` parse-failure recovery path (`aiassist` package, DeepSeek or
Claude via the `Recognizer` Strategy) and the `categorize` subcommand (ADR
0010, its own small DeepSeek `Assigner`). Both read credentials the same way —
a provider name via `--ai-provider` / `AI_PROVIDER`, and a bearer key via a
provider-specific env var (`DEEPSEEK_API_KEY`, `ANTHROPIC_API_KEY`), with an
optional model override via another env var (`DEEPSEEK_MODEL`,
`ANTHROPIC_MODEL`).

That's workable through `auto run` (ADR 0007 injects `config/gmail/config.yaml`
values as env vars before the job runs), but it's friction for direct/local
runs (`go run .`, `make run`): three separate knobs (provider flag, key env
var, optional model env var) to set by hand every time, no way to keep more
than one named credential set (e.g. two DeepSeek accounts, or a DeepSeek and a
Claude profile) side by side, and no single place that says "this is what
`deepseek` means."

## Decision

1. ~~`config/ai/<name>.yaml` profile files lives inside the pack
   (`packs/gmail/config/ai/`).~~ **Moved to the workspace root by ADR 0015** —
   see that ADR. The file format itself (`provider`/`api_key`/`model`/
   `api_base`) is unchanged.

2. **New `--ai=<name>` flag**, on the default (fetch) subcommand and on
   `categorize`. It loads `<ai-profiles-dir>/<name>.yaml` (default now the
   workspace-root `config/ai/`, per ADR 0015 — was `./config/ai` i.e.
   pack-local when this ADR was first written) and exports its fields as the
   *same* env vars the existing provider clients already read
   (`DEEPSEEK_API_KEY`/`DEEPSEEK_MODEL`/`DEEPSEEK_API_BASE` or
   `ANTHROPIC_API_KEY`/`ANTHROPIC_MODEL`/`ANTHROPIC_API_BASE`), then resolves
   `provider` from the profile. This means zero changes to the `Recognizer`/
   `Assigner` Strategy interfaces or their DeepSeek/Claude implementations —
   only three lines added to each (`deepseek.go` ×2, `claude.go`) to read an
   optional `*_API_BASE` override that previously didn't exist as a knob at
   all.

3. **`--ai` takes final precedence** over `--ai-provider` / `AI_PROVIDER` and
   any `*_API_KEY` already in the process environment. It's the most specific,
   most recently-stated selection — an explicit `--ai=deepseek` should not be
   silently second-guessed by a stray env var.

4. **Real profiles are gitignored; `*.example.yaml` templates are committed.**
   `config/ai/*.yaml` holds a real API key, so it follows the same pattern as
   `credentials.json`/`token.json` (gitignored, real values local-only).
   `config/ai/deepseek.example.yaml` and `config/ai/claude.example.yaml` are
   committed, documented templates — copy and fill in, same motion as
   `config.sample.yaml` → `config/gmail/config.yaml` at the workspace level.

5. **`categorize` still only implements DeepSeek** (ADR 0010 is unchanged by
   this decision). A `provider: claude` profile resolves fine for
   `--ai-assist` but `categorize --ai=<claude-profile>` fails with the same
   "unsupported provider" error `--ai-provider=claude` already produced —
   this ADR doesn't add a Claude `Assigner`.

## Consequences

- Three flags become one for the common case: `--ai-provider foo
  DEEPSEEK_API_KEY=bar go run .` becomes `go run . --ai=foo` once
  `config/ai/foo.yaml` exists.
- Multiple named credential sets can coexist (`config/ai/deepseek.yaml`,
  `config/ai/deepseek-personal.yaml`, `config/ai/claude.yaml`), selected per
  invocation — not possible with a single `*_API_KEY` env var.
- `--ai` is local-CLI-only; it is not wired into any `jobs/*/manifest.yaml`
  `runtime.env` list, since scheduler runs go through `auto run` and ADR 0007
  env-injection, not `--ai=<name>`. Someone who wants a profile-driven
  scheduled run would need to either export the profile's env vars ahead of
  time or extend the manifest/job wrapper — out of scope here.
- `DEEPSEEK_API_BASE` / `ANTHROPIC_API_BASE` are now real, if rarely-used,
  overrides on all three provider clients — previously the API URLs were
  build-time constants with no override at all.

## Related

- ADR 0007 — pack config injection (the env-var contract `--ai` profiles feed
  into, without replacing).
- ADR 0010 — AI categorisation (`categorize`'s DeepSeek-only `Assigner`,
  unchanged).
- ADR 0013 — forwarded transaction notes (same day, unrelated feature; not
  touched by this decision).
