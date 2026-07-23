# AI provider profiles (ADR 0015)

A profile bundles everything an AI provider client needs — provider, API
key/token, and an optional model or endpoint override — into one YAML file,
selected by name. This is workspace-wide: any pack's job can use it, and
`auto run` is the primary way to.

## Setup

```bash
cp config/ai/deepseek.example.yaml config/ai/deepseek.yaml
# edit config/ai/deepseek.yaml and fill in your real api_key
```

`config/ai/*.yaml` is gitignored (real keys); only the `*.example.yaml`
templates and this README are committed.

## Usage — via `auto run` (any pack, any job)

```bash
./auto run gmail-extract --ai deepseek -- --ai-assist
./auto run gmail-categorize --ai deepseek
```

`auto run <job> --ai <name>` loads `config/ai/<name>.yaml` and injects
`DEEPSEEK_API_KEY`/`DEEPSEEK_MODEL`/`DEEPSEEK_API_BASE` (or the `ANTHROPIC_*`
equivalents) plus `AI_PROVIDER` into the job's process environment — the same
env vars every provider client already reads. A pack needs **zero code
changes** to benefit: if it already reads `DEEPSEEK_API_KEY` the way
`packs/gmail` does, `--ai <name>` just works. This overrides any ambient
shell env var of the same name — the explicitly-named profile wins.

## Usage — direct (bypassing `auto`, pack-local dev loops)

Packs that ship their own `--ai=<name>` flag for direct `go run .` /
equivalent local runs (see e.g. `packs/gmail`) point at this same directory by
default, so a profile you set up here works both ways without duplicating it
per pack.

## Schema

| Field | Required | Meaning |
|-------|----------|---------|
| `provider` | yes | `deepseek` or `claude` (aka `anthropic`) |
| `api_key` | yes | the provider's bearer token/API key |
| `model` | no | overrides the provider's built-in default model |
| `api_base` | no | overrides the provider's API endpoint (proxies/self-hosted only) |

## Multiple profiles

Name files however you like — `deepseek.yaml`, `deepseek-personal.yaml`,
`claude.yaml` — and select one by its name (without `.yaml`):
`--ai deepseek-personal`.
