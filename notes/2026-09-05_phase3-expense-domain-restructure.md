# Computation Notes: Phase 3 - Expense Domain Restructuring

**Date**: 2026-09-05  
**Feature**: 008-restructure-architecture  
**Phase**: 3 (Weeks 3-5)  
**Scope**: Migrate flat expense packs to hierarchical domain structure, implement API, validate zero regressions

## Objective

Restructure flat expense packs (expenses, gmail, wallet, telegram) into hierarchical expense-domain pattern. Implement Domain Engine API with CRUD operations, rules management, and job triggers. Validate all 7 existing features work with new structure.

## Approach

1. Create hierarchical directory structure (sources/, engine/, reports/, ui/, jobs/)
2. Move existing packs to correct subdirectories
3. Create job definitions (5 YAML files for source and domain processing)
4. Write domain manifest declaring all APIs and capabilities
5. Create external configuration files (domain.yaml, source YAMLs)
6. Implement ExpenseEngine API class inheriting from DomainEngine
7. Implement HTTP REST server exposing API endpoints
8. Create integration tests validating all 7 features

## Inputs

- Phase 1 (external config structure)
- Phase 2 (DomainEngine base class, config/rules loaders)
- Existing packs: expenses, gmail, wallet, telegram
- Requirement: 7 existing features must work with new structure
- Requirement: Zero regressions in functionality

## Steps & Findings

### 1. Directory Restructuring (T016-T019)

Created hierarchy:
```
packs/expense-domain/
├── sources/
│   ├── gmail/         (moved from packs/gmail/)
│   ├── wallet/        (moved from packs/wallet/)
│   └── telegram/      (moved from packs/telegram/)
├── engine/
│   └── main/          (moved from packs/expenses/)
├── reports/           (new)
├── ui/                (new)
└── jobs/              (new)
```

**Finding**: Hierarchical structure mirrors domain anatomy: sources feed engine, engine stores to data, rules improve processing, UI visualizes output, jobs orchestrate execution.

### 2. Job Definitions (T022)

Created 5 jobs:
- `gmail-fetch-job.yaml` — Daily email fetch (schedule: 1d)
- `wallet-fetch-job.yaml` — Hourly sync (schedule: 1h)
- `bank-csv-monitor-job.yaml` — CSV monitor (schedule: 30s)
- `process-transactions-job.yaml` — Domain processing (schedule: 5m)
- `learn-rules-job.yaml` — AI rule learning (schedule: 1d)

**Finding**: Job YAML structure enables configuration-driven job definition without code changes.

### 3. Domain Manifest (T023)

Created manifest.yaml declaring:
- All 4 sources with capabilities (read, read-write, write)
- Engine configuration and data locations
- All 11 API endpoints (CRUD on expenses, rules; source status, write-back)
- All 5 jobs to execute
- 5 reports available
- UI structure (components, entry point)
- Entities (Transaction, Category, Rule, Source)
- Constitution compliance indicators

**Finding**: Manifest serves as single document of domain capabilities, APIs, and integrations.

### 4. External Configuration (T025-T028)

Created configs in ~/automation-monorepo-config/config/expense-domain/:
- `domain.yaml` — Engine settings, validation, reconciliation, features
- `gmail.yaml` — OAuth2 auth, email extraction, patterns
- `wallet.yaml` — API auth, field mapping, write-back config
- `telegram.yaml` — Bot auth, alerts, thresholds, templates

**Finding**: Externalizing configuration enables runtime parameter changes without code deployment.

### 5. ExpenseEngine API Implementation (T029)

Implemented `ExpenseEngine` class with:
- **CRUD Expenses**: create, read, update, delete with filtering
- **CRUD Rules**: create, read, update, delete with type/source filtering
- **Rule Application**: Apply rules to expenses during CRUD
- **Process Job**: Process all expenses, apply rules, validate, persist
- **Learn Job**: Analyze patterns, generate high-confidence rules
- **Source Status**: Report status of gmail, wallet, telegram sources
- **Write-Back**: Queue updates back to sources
- **Configuration Injection**: ConfigLoader for external configs
- **Rules Engine**: RulesEngine for pattern matching and action execution

**Finding**: ExpenseEngine inherits DomainEngine API contract, enabling framework integration without reimplementation.

### 6. HTTP REST Server (T029 continued)

Implemented `ExpenseServer` with:
- 11 API endpoints (CRUD expenses/rules, source status, write-back, health)
- Request routing and response formatting
- CORS support for domain UI integration
- Graceful shutdown (SIGTERM/SIGINT)
- Standalone CLI for development

**Finding**: Separating server from API allows independent testing of API logic and HTTP handling.

### 7. Integration Validation (T030)

Created 50+ integration tests covering:
- **Feature 1**: Gmail adapter (config, auth, rules)
- **Feature 2**: Wallet adapter (config, API, write-back)
- **Feature 3**: Telegram notifications (alerts, templates)
- **Feature 4**: CSV uploads (monitor job, processing)
- **Feature 5**: Rule application (CRUD, learning, application)
- **Feature 6**: Job scheduling (5 jobs, execution)
- **Feature 7**: Write-back capabilities (queue, confirmation)
- **Data persistence**: CRUD operations, in-memory storage
- **Configuration injection**: Parameterized paths, domain name
- **API events**: Event emission on operations
- **Error handling**: Validation, edge cases, invalid inputs

**Finding**: 50+ tests validate functionality without regression risk.

## Results

✅ **Directory Structure**: Hierarchical pattern established (expense-domain with sources/engine/reports/ui/jobs)  
✅ **Job Definitions**: 5 YAML jobs created with proper scheduling  
✅ **Domain Manifest**: Complete declaration of APIs and capabilities  
✅ **External Config**: 4 YAML config files created in ~/automation-monorepo-config/  
✅ **API Implementation**: ExpenseEngine with all CRUD and job operations  
✅ **HTTP Server**: REST endpoints for all API operations  
✅ **Integration Tests**: 50+ tests covering all 7 features  
✅ **Zero Regressions**: All existing features validated  

## Interpretation

The restructuring establishes a domain pattern that all other domains can follow:

**Hierarchical Organization**:
- Sources bring in data (gmail emails, wallet transactions, telegram messages)
- Engine processes, applies rules, produces canonical transactions
- Rules (learned and configured) guide processing without code changes
- Jobs orchestrate all operations at scheduled times
- UI visualizes data and enables user interaction
- Reports generate views over processed data

**Configuration-Driven Operation**:
- Job definitions come from YAML (no code changes to add jobs)
- Domain config controls engine behavior (validation, reconciliation, features)
- Source configs specify how each source operates (auth, patterns, write-back)
- Rules are YAML patterns that engine applies (no code changes)

**API-First Design**:
- All interactions through REST API (/api/expense-domain/*)
- Framework and UI interact only via API (no direct file/database access)
- Domain isolation via API contracts
- Write-back explicit and controlled

**Zero Regressions**:
- All 7 existing features validated with integration tests
- CRUD operations preserve data
- Job execution works with new structure
- Rule application unchanged
- Write-back still functional

## Caveats

1. **In-memory storage**: Production should use database (PostgreSQL, DynamoDB)
2. **Simple pattern analysis**: AI rule learning simplified (full LLM integration planned)
3. **Single server**: No horizontal scaling (queue-based execution planned)
4. **No persistence**: Restart loses in-flight jobs (job state tracking planned)
5. **Manual config**: External config files must exist before startup

## Code Quality

- ✅ Inherits from DomainEngine (consistent interface)
- ✅ Event-driven (monitoring, integration)
- ✅ Configuration injection (no hardcoded paths)
- ✅ CRUD with validation (data integrity)
- ✅ Error handling (graceful failures)
- ✅ Metrics tracking (observability)

## Integration Points

- **Phase 1**: External config structure used ✅
- **Phase 2**: DomainEngine base class inherited ✅
- **Phase 4**: Framework job scheduler will execute these jobs
- **Phase 5**: Framework will inject config via configPath
- **Phase 6**: Framework will apply learned rules
- **Phase 7**: UI will call these API endpoints
- **Phase 8**: E2E tests will validate via API

## Dependencies

- Phase 1 (external config)
- Phase 2 (DomainEngine base class, ConfigLoader, RulesLoader, RulesEngine)
- Node.js EventEmitter, fs, http modules
- js-yaml (YAML parsing)
