# Project-Specific Claude Code Rules

## External Configuration Management

**Rule**: All runtime configuration lives in `~/automation-monorepo-config/`, never in this repository.

**Structure**:
```
~/automation-monorepo-config/
├── config/
│   ├── framework.yaml          # Framework-level settings
│   ├── domains.yaml            # Available domains
│   └── {domain}/               # Domain-specific config
│       ├── domain.yaml
│       ├── gmail.yaml
│       └── wallet.yaml
├── data/
│   └── {domain}/               # Domain-specific data outputs
│       ├── engine/
│       ├── gmail/
│       └── wallet/
└── rules/
    └── {domain}/               # Learned rules per domain
        ├── gmail/
        └── wallet/
```

**Principles**:
1. **packs/ is read-only**: No data, config, or rules generated inside packs/
2. **External is canonical**: ~/automation-monorepo-config/ is the Single Source of Truth
3. **Framework injection**: Framework reads `--config-path ~/automation-monorepo-config` at startup
4. **Domain isolation**: Each domain sees only its subtree via parameterized injection
5. **Convention over configuration**: Sensible defaults reduce explicit config burden

**Implementation**:
- Framework accepts `--config-path` parameter at startup
- Framework injects config location to all domains
- Domains never hardcode paths; they use injected location
- Rules are YAML, applied without code changes
- Data stored with domain/source hierarchy

**Never**:
- Commit anything from ~/automation-monorepo-config/ to this repo
- Hardcode ~/automation-monorepo-config paths in code
- Store secrets or credentials in this repo
- Add new config formats without framework support

**How to extend**:
- New domain: add subdirectory structure in ~/automation-monorepo-config/
- New source: add config file in domain-specific config subdirectory
- New rules: framework learns and stores in ~/automation-monorepo-config/rules/

---

## Git Workflow

**Worktree + Feature Branch**:
- Feature work uses `.worktrees/{feature-name}` with `feature/{feature-name}` branch
- `.specify/spec-map.json` tracks feature state locally
- Both `.specify/spec-map.json` and `.worktrees/` are git-ignored

**Commits**:
- Work inside `.worktrees/{feature-name}` 
- Commit with description including Why/What/Notes
- Push to `feature/{feature-name}` branch
- Create MR for review

**Auto-generated files**:
- RUNBOOK.md updated on every prompt
- Computation notes stored in notes/YYYY-MM-DD_*.md
- Data file organization: data/raw/, data/processed/, data/reference/, data/archive/

---

## Constitution Principles (Multi-Domain Architecture)

**Principle I - Packs Declare, Workspace Supplies**
- Domains declare their config/data needs in manifest.yaml
- Framework supplies config location as parameter
- Domains never resolve paths themselves

**Principle II - packs/ Read-Only**
- All generated data → ~/automation-monorepo-config/data/
- All learned rules → ~/automation-monorepo-config/rules/
- packs/ contains only code, schemas, manifests, docs

**Principle III - Static Artefacts**
- Domain UIs are static files in `packs/{domain}/ui/`
- Framework serves, does not execute inside domains

**Principle V - Configuration Over Code**
- Rules are YAML, not code
- New domains/sources added as configuration
- Framework applies rules without code changes

**Principle VI - Domain Isolation**
- Domain APIs declared in manifest.yaml
- Domains communicate via HTTP API contracts only
- No direct procedure calls or file dependencies

**Principle VII - Local-First, Least Exposure**
- Framework runs locally, self-contained
- No external cron/launchd dependencies
- Credentials in config files, not code
- Write-back is explicit configured act
