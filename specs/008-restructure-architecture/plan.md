# Implementation Plan: Multi-Domain Architecture Restructuring with Self-Improving Rules

**Spec**: 008-restructure-architecture  
**Branch**: feature/restructure-architecture  
**Created**: 2026-09-05  
**Status**: Ready for Phase 1 (Design)

---

## Technical Context

**Core Problem**: Live migration from flat pack structure to hierarchical multi-domain architecture with self-managed jobs, self-improving rules, and unified config.

**Architecture Foundation**: See ARCHITECTURE.md for complete reference
- Unified directory: ~/automation-monorepo-config/ (data/, config/, rules/)
- Domain structure: sources/, engine/, reports/, ui/, jobs/
- Framework-managed jobs: No cron/launchd
- AI-driven rule learning: YAML-based, no code changes

**Migration Scope**: 7 existing features (specs 001-007), zero regressions required

**Critical Dependencies**:
- Constitution Principles I (Packs Declare), II (packs/ Read-Only), V (Configuration Over Code)
- Framework-managed job scheduling (replaces cron/launchd)
- Domain Engine APIs (read/write domain data, trigger jobs, manage rules)
- Rule learning mechanism (AI patterns → YAML rules)
- Unified config consolidation (data/, config/, rules/ hierarchy)

---

## Constitution Check

✅ **Principle I (Packs Declare, Workspace Supplies)**
- All domain config/rules/data injected via parameter to framework
- Domains never hardcode paths; everything arrives through declared injection
- Framework supplies: config location, rules location, data location
- **Justification**: Central config consolidation enables parameterized injection, meeting Principle I exactly

✅ **Principle II (packs/ Read-Only)**
- No data generated inside packs/; all outputs go to ~/automation-monorepo-config/data/
- All rules learned go to ~/automation-monorepo-config/rules/
- packs/ contains only code, schemas, manifests, docs
- **Justification**: Clean separation prevents state drift, satisfies read-only guarantee

✅ **Principle V (Configuration Over Code)**
- Rules are YAML configuration files, not code changes
- New sources/domains added as data (config), not code paths
- Convention over Configuration as prime principle
- **Justification**: Self-improving via rules without code changes; framework applies rules without code modification

✅ **Principle VII (Local-First, Least Exposure)**
- Framework runs locally, no external dependencies for job scheduling
- Credentials in ~/automation-monorepo-config/config/, not code
- Write-back is explicit configured act
- **Justification**: Framework self-contained, no cron/launchd external dependencies

**Gate Result**: PASS - All principles satisfied, no unjustified violations

---

## Phase 0: Research & Clarifications

**Status**: COMPLETE (all clarifications resolved in earlier sessions)

All 10 clarifications from spec have been answered:
1. ✅ Multi-domain pattern (Option B + extensibility)
2. ✅ Domain interaction (Layered + Composite)
3. ✅ Shared foundation (Hybrid approach)
4. ✅ Data I/O boundaries (Parameterized injection)
5. ✅ Glossary requirement (Confirmed)
6. ✅ Migration strategy (Preserve-Shared-First, framework untouched)
7. ✅ Testing approach (BDD-driven)
8. ✅ Config structure (Unified ~/automation-monorepo-config/)
9. ✅ Convention over Configuration (Confirmed as prime principle)
10. ✅ Job scheduling (Framework-managed, no cron/launchd)
11. ✅ Rule learning (Centralized registry with domain/source scoping)
12. ✅ UI architecture (Domain-specific + framework aggregation)

No research tasks needed — all technical decisions made.

---

## Phase 1: Design Artifacts

### 1. Data Model & Entities

**Core Entities**:

```
Domain
  ├─ sources[] {name, adapter, config, manifest}
  ├─ engine {config, manifest, api-contract}
  ├─ reports[] {name, manifest}
  ├─ ui {manifest, components, api-client}
  └─ jobs[] {name, schedule, trigger-config, handlers}

Source Adapter
  ├─ fetch-job {schedule, timeout, retry}
  ├─ monitor-job {watch-path, trigger-threshold}
  ├─ extract-job {rules, output-schema}
  └─ write-back-job {targets, confirmation-rules}

Domain Engine
  ├─ process {rules, validation}
  ├─ rules-engine {learned-rules, applied-rules}
  └─ api-endpoints {read, write, query}

Rule
  ├─ name, type (categorization/validation/dedup)
  ├─ confidence, origin (ai-learned/configured)
  ├─ pattern, action, enabled
  └─ created, updated, learned-date
```

**Directory Structure**:
```
~/automation-monorepo-config/
├─ data/{domain}/{source}/ (source outputs)
├─ data/{domain}/engine/ (domain engine outputs)
├─ config/{domain}/domain.yaml (engine config)
├─ config/{domain}/{source}.yaml (adapter config)
├─ rules/{domain}/{source}/ (learned rules per source)
└─ rules/{domain}/engine/ (learned rules per engine)
```

### 2. API Contracts

**Domain Engine API** (`/api/{domain}/`):
```
GET /expenses, /expenses/{id}
PATCH /expenses/{id}, POST /expenses, DELETE /expenses/{id}
GET /rules, POST /rules, PATCH /rules/{id}
GET /sources/{source}/status
POST /jobs/{job}/trigger
GET /jobs (history & status)
```

**Source Adapter API** (`/api/{domain}/sources/{source}/`):
```
GET /data (fetch processed data)
GET /status (health, last-fetch, next-scheduled)
POST /write-back (send updates back to source)
```

**Framework API** (`/api/framework/`):
```
GET /domains (available domains)
GET /dashboard (aggregated metrics)
GET /jobs (all jobs across domains)
POST /rules/validate (cross-domain conflicts)
```

### 3. Implementation Phases

**Phase 1a: Setup Infrastructure** (weeks 1-2)
- Create ~/automation-monorepo-config/ structure
- Implement config/rules/data directory hierarchy
- Add config location as framework parameter

**Phase 1b: Migrate Shared Framework** (week 2-3)
- Move auth, jobs, lib to framework (preserve unchanged)
- Implement framework job scheduler
- Replace cron/launchd with framework job execution

**Phase 1c: Restructure expense-domain** (weeks 3-5)
- Move gmail/, wallet/, sms/, telegram/ → expense-domain/sources/
- Create expense-domain/engine/ (main application logic)
- Create expense-domain/reports/ (report generation)
- Create expense-domain/ui/ (domain-specific UI)
- Create expense-domain/jobs/ (source jobs + domain jobs)
- Implement Domain Engine API

**Phase 1d: Rule Learning System** (weeks 5-7)
- Implement AI rule discovery mechanism
- Create rule storage in ~/automation-monorepo-config/rules/
- Implement rule application in domain engine
- Create conflict resolution logic

**Phase 1e: BDD Testing** (weeks 7-8)
- Document behaviors for all 7 features
- Generate integration tests from behaviors
- Baseline tests against flat structure
- Validate tests pass against new structure

**Phase 1f: Domain UI** (weeks 8-10)
- Create expense-domain/ui/ with components
- Implement API data binding (read/write)
- Add job trigger interface
- Add rule management UI
- Create framework aggregation UI

**Phase 1g: Validation & Deployment** (weeks 10-11)
- Run full test suite
- Validate zero regressions
- Performance testing
- Deploy to production

---

## Complexity Tracking

### Highest-Risk Areas

| Area | Risk | Mitigation |
|------|------|-----------|
| **Live Migration** | 7 features must stay operational | BDD baseline tests before migration; incremental domain restructuring |
| **Framework Jobs** | Replacing cron/launchd timing-sensitive | Framework job scheduler thoroughly tested; fallback to cron during transition |
| **Rule Learning** | AI generates rules that must be applied correctly | Confidence threshold enforcement; manual review UI; conflict resolution |
| **Config Consolidation** | Moving config from 3 locations | Migration script; validation; rollback plan |
| **Write-back** | Critical for source updates | Explicit opt-in; logging; confirmation flow |

### Tradeoffs Accepted

| Tradeoff | Decision | Rationale |
|----------|----------|-----------|
| **Scope** | Restructure 7 existing features vs. greenfield | Live migration necessary to preserve functionality; reduce risk by extracting pattern after |
| **Testing** | BDD-driven before implementation | Ensures no regressions; validates behavior hasn't changed; builds confidence in migration |
| **Framework** | Self-managed jobs vs. external orchestration | Eliminates cron/launchd dependencies; makes system self-contained and portable |
| **Rules** | AI-learned YAML vs. code-based rules | Supports self-improvement without code changes; aligns with Convention over Configuration |

---

## Success Metrics

✅ All 7 existing features pass BDD tests (zero regressions)  
✅ expense-domain restructured following pattern  
✅ Framework manages all jobs (no cron/launchd)  
✅ Rules learned and applied correctly (>95% confidence)  
✅ Domain UIs functional for all domains  
✅ Framework aggregation UI shows all domains  
✅ Zero data loss during migration  

---

## Next Steps

1. **Phase 1 Design Complete** ✅
2. **Proceed to `/speckit-tasks`** — Decompose into implementation tasks
3. **Begin Phase 1a (Setup)** — Create config structure
4. **Execute tasks in sequence** — Follow 11-week timeline
5. **Monitor BDD tests** — Validate zero regressions throughout

---

## Appendix: Constitution Check Details

**Principle I Verification**:
- ✅ Framework accepts config location as parameter
- ✅ Framework injects config location to all domains
- ✅ Domains read from injected path, never hardcode
- ✅ Domains don't resolve paths; framework supplies them

**Principle II Verification**:
- ✅ No data in packs/; all outputs to ~/automation-monorepo-config/data/
- ✅ No rules in packs/; all rules to ~/automation-monorepo-config/rules/
- ✅ packs/ contains only code, schemas, manifests, docs
- ✅ Verified with directory structure documentation

**Principle V Verification**:
- ✅ Rules are configuration (YAML), not code
- ✅ New sources/domains added as config data
- ✅ Framework applies rules without code modification
- ✅ Convention over Configuration is primary principle

**Principle VII Verification**:
- ✅ Framework runs locally, self-contained
- ✅ No external cron/launchd dependencies
- ✅ Credentials in config files, not code
- ✅ Write-back is explicit configured act

