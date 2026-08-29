# automation workspace

Your private working copy of the automation system. It mounts a shareable
**framework** (the parent) and one or more **packs** of jobs, and holds your
machines, data, and private automations.

**The model** (full rationale in `docs/adr/0002…` and `docs/SHARING.md`):

```
workspace (this repo, private)     machines.yaml + data/ + packs.yaml
├── framework/   → public parent: the `auto` CLI + conventions   (submodule)
└── packs/
     ├── shared/  → team library, contribute-back                (submodule)
     └── private/ → your jobs, never shared
```

A **job** is a folder with a `manifest.yaml`. Jobs live in packs. Each job has a
`visibility` (`private`/`shared`/`public`) so you can share some automations and
keep others private — enforced by `auto doctor`, guaranteed by per-repo access.

## Quickstart

```bash
./auto packs                 # what's mounted
./auto list                  # every job you can see (pack + visibility shown)
./auto list --visibility shared
./auto search backup
./auto run hello-report      # run a job (logging, timeout, history)
./auto run gmail-extract --ai deepseek -- --ai-assist   # inject a named AI credential profile (config/ai/deepseek.yaml)
./auto orchestrate           # list multi-step pipelines defined in orchestrator/
./auto orchestrate gmail-wallet-sync   # run one: steps in order, spanning packs, with retry/timeout/wait/loop
./auto config init gmail     # scaffold a pack's config (values live in config/, git-ignored)
./auto config gmail          # show which env/secret values are set vs missing
./auto new                   # scaffold a job into a pack (choose private/shared)
./auto catalog               # regenerate CATALOG.md
./auto share shared          # write a shareable catalog of the shared pack
./auto schedule sync --dry-run
./auto log "what I did"
./auto doctor                # validate + check for visibility leaks
./auto serve                 # local dashboard: packs, config status, jobs, command help
#   or: make serve  (PORT=... to override the default 4321)
./auto sandbox-check         # verify a job's writes are actually confined to config/+data/ on this machine
```

Every job `auto run`/`auto orchestrate` executes runs inside a write-sandbox
(macOS `sandbox-exec`, Linux `bwrap`): it can read anywhere (its own code
lives in `packs/`), but can only *write* inside the workspace's
`config/`+`data/` and a couple of toolchain-cache carve-outs jobs need to
actually run — **packs/ itself is read-only**, including a pack's own
directory. `auto`'s own commands (catalog/log/new/schedule) are unaffected —
only the job process itself is wrapped. `auto run <id> --no-sandbox` is an
escape hatch for debugging; it prints a warning every time it's used.
Design in `docs/adr/0018-write-sandbox-for-job-execution.md`.

Anything a job needs to persist — secrets or produced data — is declared in
its `config.sample.yaml` and symlinked in by `auto run`: `files:` for
secrets (→ `config/<pack>/`, ADR 0007) and `data_files:` for everything else
a pack itself would otherwise write internally, like a registry or dedupe
ledger (→ `data/<pack>/`, ADR 0019). `auto doctor` checks both lists: every
declared file must be a symlink, never a real file sitting in the pack
directory.

### Wallet sync (gmail transactions → BudgetBakers Wallet)

The `wallet` pack reads the gmail pack's `data/gmail/transactions.csv` and creates
one Wallet record per transaction (day by day, deduped, tagged with the
`source:automation-monorepo` label). Full setup in
**[packs/wallet/RUNBOOK.md](packs/wallet/RUNBOOK.md)** and **[WALLET-WORKFLOW.md](WALLET-WORKFLOW.md)**; design in `docs/adr/0009`.

#### Complete Workflow (Recommended)

```bash
export WALLET_API_TOKEN="your-premium-api-token"
./auto orchestrate gmail-wallet-sync-with-dedup
```

This runs 10 steps in sequence:
1. **wallet-fetch** — Fetches all wallet records from Wallet API → `data/wallet/records.jsonl`
2. **gmail-extract** — Extracts financial transactions from Gmail inbox → `data/gmail/transactions.csv`
3. **gmail-categorize** — AI-categorizes transactions using DeepSeek
4. **wallet-sync-categories** — Syncs Gmail categories to Wallet records with "Unknown" category (±1 day date matching)
5. **wallet-fetch-accounts** — Refreshes accounts cache from Wallet API → `data/wallet/accounts-cache.json`
6. **wallet-sync** — Pushes new transactions to Wallet API with automatic retry on rate limits
7. **wallet-dedup scan** — Scans for duplicate records by MessageID (read-only)
8. **wallet-dedup review** — Interactive: reviews duplicates and collects keep/delete decisions
9. **wallet-dedup execute** — Deletes duplicate records from Wallet API
10. **wallet-dedup finalize** — Removes deleted records from local `records.jsonl`, creates backup

#### Individual Commands

**Setup**:
```bash
./auto config init wallet                    # Scaffold config/wallet/ (set WALLET_API_TOKEN)
# Then edit config/wallet/accounts.json to map account codes to Wallet UUIDs
```

**Fetch & Extract**:
```bash
./auto run wallet-fetch                      # Fetch current wallet records (for reference)
./auto run gmail-extract                     # Extract transactions from Gmail
./auto run gmail-categorize                  # AI-categorize transactions (requires DEEPSEEK_API_KEY)
```

**Categorize in Wallet**:
```bash
./auto run wallet-sync-categories            # Dry-run: show what categories would sync
./auto run wallet-sync-categories -- --apply # Apply all category updates (all confidence levels)
./auto run wallet-sync-categories -- --apply-high  # Apply only high-confidence matches (same-day)
./auto run wallet-fetch-accounts             # Refresh accounts cache for fallback matching
```

**Sync to Wallet**:
```bash
./auto run wallet-sync                       # Push transactions to Wallet API (with rate-limit retry)
```

**Deduplication Workflow**:
```bash
./auto run wallet-dedup scan                 # Detect duplicates (read-only)
./auto run wallet-dedup review               # Interactive: review and collect decisions
# Generates: data/wallet/.dedup-decisions-{timestamp}.json

./auto run wallet-dedup execute --decisions-file data/wallet/.dedup-decisions-{timestamp}.json
# DELETE from Wallet API + create backup + save results

./auto run wallet-dedup finalize             # Remove deleted records from local records.jsonl
```

**Orchestrations**:
```bash
./auto orchestrate gmail-wallet-sync                   # Extract → sync (6 steps, no dedup)
./auto orchestrate gmail-wallet-sync-with-dedup       # Extract → sync → dedup (10 steps, full workflow)
```

#### What Each Command Achieves

| Command | Input | Output | Purpose |
|---------|-------|--------|---------|
| **wallet-fetch** | Wallet API | `records.jsonl` | Mirror all wallet records locally for reference/matching |
| **gmail-extract** | Gmail inbox | `transactions.csv` | Extract financial transactions from emails |
| **gmail-categorize** | `transactions.csv` | `transactions.csv` (enhanced) | AI-assign categories using DeepSeek |
| **wallet-sync-categories** | Gmail CSV + Wallet records | Wallet API updates | Match Gmail categories to Wallet "Unknown" records |
| **wallet-fetch-accounts** | Wallet API | `accounts-cache.json` | Cache account mappings for fallback matching |
| **wallet-sync** | `transactions.csv` | Wallet API records | Create records in Wallet, dedupe by MessageID, rate-limit retry |
| **wallet-dedup scan** | `records.jsonl` | Console output | Identify duplicate records (same MessageID, different amounts/dates) |
| **wallet-dedup review** | `records.jsonl` | `decisions.json` | Interactive: decide which duplicates to keep/delete |
| **wallet-dedup execute** | `decisions.json` | Wallet API deletes | Actually DELETE duplicate records from Wallet |
| **wallet-dedup finalize** | `records.jsonl` + deletion results | `records.jsonl` (updated) | Remove deleted records from local copy, backup original |

#### Key Features

- **Deduplication by MessageID**: Each Gmail MessageID tracked in `state.json` — re-runs never duplicate
- **Rate-limit retry**: Automatic exponential backoff (2s, 4s, 8s) for HTTP 429 errors
- **Account mapping**: Fallback to `accounts-cache.json` for unmapped account codes
- **Category matching**: High confidence (same-day) and medium (±1 day), reviewable before applying
- **Comprehensive logging**: Every step shows progress, per-record results, success/failure counts
- **Audit trail**: Dedup decisions and results saved to JSON for review
- **Backups**: Automatic backups created before any destructive operations

#### Configuration

**Required**:
- `WALLET_API_TOKEN` — Premium plan API token from https://web.budgetbakers.com/settings/rest-api
- `config/wallet/accounts.json` — Map CSV account codes to Wallet UUIDs

**Optional**:
- `WALLET_BASE_URL` — Wallet API endpoint (default: `https://rest.budgetbakers.com/wallet`)
- `WALLET_TIMEZONE` — Timezone for record dates (default: `Asia/Kolkata`)
- `WALLET_LABEL_ID` — Label UUID for new records (or use `WALLET_LABEL` name)

See `config/wallet/config.sample.yaml` for all options.

### Expenses events (AI clustering of transactions into trips, festivals, …)

The `expenses` pack reads the gmail pack's `transactions.csv` (read-only) and, via
DeepSeek, matches each not-yet-assigned transaction against a versioned registry
of known "events" (`config/events.json`) or proposes a new one — so a
transaction seen next month is recognised as the same trip/festival instead of
spawning a duplicate. Full setup in
**[packs/expenses/RUNBOOK.md](packs/expenses/RUNBOOK.md)**; design in `docs/adr/0011`.

```bash
cd packs/expenses && go run . update-event --dry-run   # preview matches/new events
./auto config init expenses                            # scaffold config/expenses/ (set DEEPSEEK_API_KEY)
./auto run expenses-update-event                        # run for real (env-injected, scheduler path)
```

## Sharing it with others

You share the **framework** (public) and the **shared pack** (team) — never your
workspace or `packs/private/`. To split this folder into the three real repos and
mount them as submodules: `tools/split-into-repos.sh` (dry-runs by default). Full
walkthrough and collaborator onboarding: **[docs/SHARING.md](docs/SHARING.md)**.

## Where things are

- `framework/` — the parent: `auto` CLI, template, schema, LICENSE, VERSION
- `packs/shared/`, `packs/private/` — jobs, each pack with a `pack.yaml`
- `packs.yaml` — which packs are mounted
- `machines.yaml` — your computers
- `orchestrator/` — multi-step pipeline YAML files (`auto orchestrate <name>`), can span jobs from any pack; see `orchestrator/README.md` and `specs/001-job-orchestrator/`
- `data/` — git-synced store: `config/` (YAML), `state/` (SQLite)
- `config/ai/` — named AI provider profiles (`<name>.yaml`: provider/api_key/model/api_base), used via `auto run <job> --ai <name>` (any pack); see `config/ai/README.md` and `docs/adr/0015`
- `docs/` — `PLAN.md`, `SHARING.md`, `adr/`, `worklog/`

Design & decisions: `docs/PLAN.md` and `docs/adr/`.

**The rules those decisions follow: [`.specify/memory/constitution.md`](.specify/memory/constitution.md).**
Seven principles governing what belongs to the workspace and what belongs to a
pack — packs declare and the workspace supplies, `packs/` is read-only, the
workspace serves and packs render, derived artifacts regenerate, configuration
over code, boundaries are structural, local-first. Read it before adding a pack
or changing where anything lives; every `/speckit-plan` is gated against it.
