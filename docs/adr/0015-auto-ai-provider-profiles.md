# ADR 0015 — auto: named AI provider profiles for any pack (--ai &lt;name&gt;)

**Status:** accepted — 2026-07-23

## Context

ADR 0014 (same day) added `config/ai/<name>.yaml` profiles and a `--ai=<name>`
flag to the **gmail pack only** — a profile bundling provider + API key +
optional model/endpoint override, so a user didn't have to set `--ai-provider`
and a `*_API_KEY` env var by hand every time.

Feedback: this should have been core `auto` functionality from the start.
`auto run` (ADR 0007) already owns env injection for every pack via
`config/<pack>/config.yaml`; a second, pack-local, pack-specific mechanism for
one particular kind of credential (AI provider keys) is exactly the kind of
thing that belongs in the framework once a second pack would plausibly want
it too (`telegram`, `wallet`, `expenses` — any pack that calls an LLM).

## Decision

1. **`config/ai/<name>.yaml` moves to the workspace root**, alongside
   `config/<pack>/` (ADR 0007) — not inside any one pack. It's genuinely
   shared, not gmail-specific. Format is unchanged from ADR 0014: `provider`
   (`deepseek` | `claude`/`anthropic`), `api_key` (required), optional `model`
   / `api_base`. Gitignored like the rest of `config/*`; `*.example.yaml`
   templates and `config/ai/README.md` are the committed, versioned part
   (`.gitignore`: `!config/ai/`, `config/ai/*`, `!config/ai/*.example.yaml`,
   `!config/ai/README.md` — un-ignore the directory first, then re-ignore its
   contents, then re-allow the specific committed files, since a bare
   `config/*` pattern would otherwise hide the whole subtree from the
   negations).

2. **`auto run <job> --ai <name>` is the primary way to use one**, on `auto`
   itself, not on the packs. `auto` loads `config/ai/<name>.yaml`
   (`ai_profile_env` in `framework/tools/auto`) and injects
   `DEEPSEEK_API_KEY`/`DEEPSEEK_MODEL`/`DEEPSEEK_API_BASE` (or the
   `ANTHROPIC_*` equivalents) plus `AI_PROVIDER` into the job's subprocess
   environment — the *same* env vars provider clients already read. **Zero
   code changes are required in a pack to benefit**: if it already reads
   `DEEPSEEK_API_KEY` the way `packs/gmail` did before ADR 0014 even existed,
   `--ai <name>` just works. Precedence: pack `config.sample.yaml` defaults <
   workspace `config/<pack>/config.yaml` override < ambient shell env <
   `--ai <name>` — the explicitly-named profile wins over everything,
   matching ADR 0014's original precedence call.

3. **`--ai` is extracted from argv by hand, before argparse, only for the
   `run` subcommand** (`_extract_ai_flag`), rather than declared as a normal
   `argparse` option alongside the existing `extra` (`nargs='*'`) positional.
   Verified in this sandbox (Python 3.10): declaring `--ai` as a second named
   option on the `run` subparser breaks argparse's usual bare-`--`-separator
   handling — `auto run gmail-extract --ai=deepseek -- --ai-assist` fails
   with "unrecognized arguments: -- --ai-assist" even though the equivalent
   command with no `--ai` flag defined works fine. (Upstream argparse fixed
   `--` handling more thoroughly in Python 3.13 — bpo-53580 — but this
   script's `#!/usr/bin/env python3` shebang can't assume that version.) Hand
   -extracting `--ai NAME` / `--ai=NAME` out of `sys.argv[1:]` before
   `argparse.parse_args()` sidesteps the whole issue and was verified against
   Python's actual argparse behavior in this environment, not just reasoned
   about.

4. **`packs/gmail`'s own `--ai=<name>` flag (ADR 0014) is kept**, for direct
   `go run .` / `make run` local dev loops that don't go through `auto run` at
   all — but its default `--ai-profiles-dir` now points at the shared root
   (`../../config/ai`, relative from `packs/gmail/`) instead of a gmail-local
   copy. One file, two access paths (direct Go flag, or `auto run --ai`),
   never two copies of a real key.

## Consequences

- Any pack's job gets AI-credential support via `--ai <name>` with no
  per-pack code, as long as it already reads the standard provider env var
  names — which is the existing convention, not a new one this ADR
  introduces.
- `--ai <name>` is CLI-only, evaluated at `auto run` time; it is not a
  `runtime.env` manifest declaration and has no effect outside `auto run`
  (e.g. it does nothing for `auto schedule sync`-installed cron/launchd
  entries unless those themselves invoke `auto run ... --ai <name>`).
- `framework/tools/auto` gained ~50 lines and one new CLI flag; no other
  command's argument parsing changed.
- This ADR does not add a Claude `Assigner` to `packs/gmail/categorize` (ADR
  0010 still DeepSeek-only) or otherwise change what any individual pack's AI
  integration can do — it only changes how credentials reach it.

## Related

- ADR 0007 — pack config injection (`auto run`'s existing env pipeline; `--ai`
  is a new, higher-precedence input into the same `env` dict `cmd_run`
  builds).
- ADR 0014 — gmail-local named AI provider profiles (superseded by this ADR
  for *where profiles live and who injects them*; the profile file format and
  gmail's own direct-run flag are unchanged and described there).
