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
```

### Wallet sync (gmail transactions → BudgetBakers Wallet)

The `wallet` pack reads the gmail pack's `data/gmail/transactions.csv` and creates
one Wallet record per transaction (day by day, deduped, tagged with the
`source:automation-monorepo` label). Full setup in
**[packs/wallet/RUNBOOK.md](packs/wallet/RUNBOOK.md)**; design in `docs/adr/0009`.

```bash
cd packs/wallet && make dry-run   # preview mappings — no token, no API calls
./auto config init wallet         # scaffold config/wallet/ (set WALLET_API_TOKEN)
#   ...then copy accounts.sample.json → config/wallet/accounts.json and fill UUIDs
./auto run wallet-sync            # sync for real (env-injected, scheduler path)
```

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
- `data/` — git-synced store: `config/` (YAML), `state/` (SQLite)
- `config/ai/` — named AI provider profiles (`<name>.yaml`: provider/api_key/model/api_base), used via `auto run <job> --ai <name>` (any pack); see `config/ai/README.md` and `docs/adr/0015`
- `docs/` — `PLAN.md`, `SHARING.md`, `adr/`, `worklog/`

Design & decisions: `docs/PLAN.md` and `docs/adr/`.
