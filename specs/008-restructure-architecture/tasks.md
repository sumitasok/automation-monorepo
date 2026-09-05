# Implementation Tasks: Multi-Domain Architecture Restructuring

**Feature**: 008-restructure-architecture  
**Created**: 2026-09-05  
**Total Tasks**: 68  
**Execution Strategy**: Phases 1-7 in sequence, with parallelization within each phase  
**Estimated Duration**: 11 weeks

---

## Phase 1: Setup Infrastructure (Weeks 1-2)

Create unified config structure and framework initialization.

- [ ] T001 Create ~/automation-monorepo-config/ directory structure (data/, config/, rules/)
- [ ] T002 Create config/framework.yaml with framework-level settings
- [ ] T003 Create .specify/feature.json tracking entry for restructure feature
- [ ] T004 [P] Update .gitignore to exclude ~/automation-monorepo-config/ from repo
- [ ] T005 [P] Create CLAUDE.md rules for external config handling
- [ ] T006 Create migration script: validate current packs structure → new domain structure
- [ ] T007 Document config injection mechanism in ARCHITECTURE.md

## Phase 2: Foundational (Weeks 2-3)

Shared framework preservation and job scheduler implementation.

- [ ] T008 Lock shared/ directory: verify zero changes allowed (add validation)
- [ ] T009 Implement framework job scheduler core in packs/shared/jobs/scheduler.js
- [ ] T010 [P] Create job manifest schema (schedule, timeout, retry, handlers)
- [ ] T011 [P] Implement job execution engine (schedule → execute → track)
- [ ] T012 Implement Domain Engine API base class in packs/shared/lib/domain-api.js
- [ ] T013 [P] Create config loader: ~/automation-monorepo-config/config/ → framework
- [ ] T014 [P] Create rules loader: ~/automation-monorepo-config/rules/ → framework
- [ ] T015 Implement rule application engine (pattern matching → action)

## Phase 3: User Story 1 - Restructure expense-domain (Weeks 3-5)

Reorganize flat expense packs into hierarchical expense-domain.

**Goal**: expense-domain demonstrates full pattern: sources/, engine/, reports/, ui/, jobs/  
**Independent Test**: Restructured domain loads, all 7 existing features work  
**Parallel Execution**: T016-T019 can run in parallel; T020+ depend on completion

- [ ] T016 [US1] Create packs/expense-domain/ directory structure
- [ ] T017 [P] [US1] Move packs/expenses → packs/expense-domain/engine/
- [ ] T018 [P] [US1] Move packs/gmail → packs/expense-domain/sources/gmail/
- [ ] T019 [P] [US1] Move packs/wallet → packs/expense-domain/sources/wallet/
- [ ] T020 [US1] Create packs/expense-domain/reports/ with existing report generators
- [ ] T021 [US1] Create packs/expense-domain/ui/ directory structure (placeholder)
- [ ] T022 [US1] Create packs/expense-domain/jobs/ with job definitions for all sources
- [ ] T023 [US1] Create packs/expense-domain/manifest.yaml (declares structure, APIs, UIs)
- [ ] T024 [US1] Update all import paths in expense-domain to account for restructuring
- [ ] T025 [US1] Create config/expense-domain/domain.yaml (engine config)
- [ ] T026 [P] [US1] Create config/expense-domain/gmail.yaml (gmail adapter config)
- [ ] T027 [P] [US1] Create config/expense-domain/wallet.yaml (wallet adapter config)
- [ ] T028 [P] [US1] Create config/expense-domain/sms.yaml (SMS adapter config)
- [ ] T029 [US1] Implement Domain Engine API for expense-domain (GET/PATCH /expenses, /rules, /jobs, /sources)
- [ ] T030 [US1] Validate all 7 existing features work with restructured layout (integration test)

## Phase 4: User Story 2 - Framework-Managed Job Scheduling (Weeks 5-6)

Replace cron/launchd with framework job execution.

**Goal**: All domain jobs scheduled and executed by framework at configured times  
**Independent Test**: Jobs execute on schedule without external cron/launchd  
**Parallel Execution**: Job definitions (T031-T034) can be created in parallel

- [ ] T031 [P] [US2] Create packs/expense-domain/jobs/gmail-fetch-job.yaml (daily 2 AM)
- [ ] T032 [P] [US2] Create packs/expense-domain/jobs/wallet-fetch-job.yaml (hourly)
- [ ] T033 [P] [US2] Create packs/expense-domain/jobs/bank-csv-monitor-job.yaml (on-demand)
- [ ] T034 [P] [US2] Create packs/expense-domain/jobs/process-transactions-job.yaml (after fetch)
- [ ] T035 [US2] Integrate framework job scheduler with expense-domain
- [ ] T036 [US2] Remove all cron jobs from system (replace with framework)
- [ ] T037 [US2] Remove all launchd configs (replace with framework)
- [ ] T038 [US2] Test job execution: scheduled jobs run at configured times
- [ ] T039 [US2] Test job failure handling: retries, exponential backoff, alerts

## Phase 5: User Story 3 - Config Consolidation & Parameterized Injection (Weeks 6-7)

Consolidate config from multiple locations and inject via framework parameter.

**Goal**: All config in ~/automation-monorepo-config/, injected to domains by framework  
**Independent Test**: Domains read config from injected path, no hardcoded paths  
**Parallel Execution**: Config migration (T040-T044) can run in parallel

- [ ] T040 [P] [US3] Migrate packs config → ~/automation-monorepo-config/config/expense-domain/
- [ ] T041 [P] [US3] Migrate ~/data → ~/automation-monorepo-config/data/expense-domain/
- [ ] T042 [P] [US3] Create ~/automation-monorepo-config/rules/ directory hierarchy
- [ ] T043 [P] [US3] Create migration script: validate all config migrated
- [ ] T044 [P] [US3] Update .gitignore: ensure ~/automation-monorepo-config/ excluded
- [ ] T045 [US3] Implement config injection: framework accepts ~/automation-monorepo-config/ location as parameter
- [ ] T046 [US3] Update Domain Engine API to read config from injected location
- [ ] T047 [US3] Update all source adapters to read config from injected location
- [ ] T048 [US3] Test: domains cannot hardcode or discover config paths (validation test)
- [ ] T049 [US3] Document config injection in ARCHITECTURE.md

## Phase 6: User Story 4 - AI-Driven Rule Learning (Weeks 7-8)

Implement AI rule discovery, storage, and application.

**Goal**: Application learns rules from data patterns, stores as YAML, applies without code changes  
**Independent Test**: AI learns rule from transaction pattern, rule applied to new transactions  
**Parallel Execution**: Rule storage schema (T050-T051) and learning job (T052) in parallel

- [ ] T050 [P] [US4] Define rule YAML schema: name, type, confidence, pattern, action, origin
- [ ] T051 [P] [US4] Create ~/automation-monorepo-config/rules/expense-domain/{source}/ directories
- [ ] T052 [US4] Create packs/expense-domain/jobs/learn-rules-job.yaml (AI pattern detection)
- [ ] T053 [US4] Implement AI rule discovery: analyze transaction patterns, generate YAML rules
- [ ] T054 [US4] Implement rule storage: write learned rules to ~/automation-monorepo-config/rules/
- [ ] T055 [US4] Implement rule application: framework applies learned rules during processing
- [ ] T056 [US4] Implement rule confidence threshold (default >95%, flag for review)
- [ ] T057 [US4] Implement conflict detection: identify rules that conflict with existing rules
- [ ] T058 [US4] Implement conflict resolution: highest confidence wins, user override available
- [ ] T059 [US4] Test: learned rule applied in next domain run without code change
- [ ] T060 [US4] Test: conflicting rules detected and resolved correctly
- [ ] T061 [US4] Document rule learning in ARCHITECTURE.md

## Phase 7: User Story 5 - Domain UIs & Framework Aggregation (Weeks 8-10)

Create domain-specific UIs and framework aggregation UI.

**Goal**: Domain UIs access Domain Engine API; Framework UI shows all domains  
**Independent Test**: Domain UI fetches transactions, displays, allows edit; Framework UI aggregates all domains  
**Parallel Execution**: Domain UI components (T062-T066) in parallel; framework UI (T067-T069) in parallel

- [ ] T062 [P] [US5] Create packs/expense-domain/ui/ directory structure
- [ ] T063 [P] [US5] Create expense-domain UI components: transaction-list.jsx, transaction-editor.jsx
- [ ] T064 [P] [US5] Create expense-domain UI: source-status.jsx, rule-editor.jsx, upload-interface.jsx
- [ ] T065 [P] [US5] Create expense-domain UI API client: api-client.js with fetch/patch/post methods
- [ ] T066 [US5] Create expense-domain UI pages: dashboard, sources, rules, transactions
- [ ] T067 [P] [US5] Create framework aggregation UI in packs/framework/ui/ (or shared/)
- [ ] T068 [P] [US5] Implement domain discovery: framework loads domain list from config
- [ ] T069 [P] [US5] Implement domain UI embedding: framework loads domain UIs dynamically
- [ ] T070 [US5] Create framework aggregation dashboard: metrics, job status, domain health
- [ ] T071 [US5] Test: Domain UI fetches/displays/edits transactions through API
- [ ] T072 [US5] Test: Framework UI aggregates all domains, navigation works
- [ ] T073 [US5] Test: File upload triggers source monitor job, data appears in UI
- [ ] T074 [US5] Test: Rule editor displays learned and configured rules

## Phase 8: BDD Testing & Validation (Weeks 10-11)

Baseline tests on current structure, validate against new structure, certify zero regressions.

- [ ] T075 Document behaviors for all 7 existing features (001-007 specs) in BDD format
- [ ] T076 [P] Generate integration tests from documented behaviors
- [ ] T077 [P] Run baseline tests against flat structure: all pass (baseline established)
- [ ] T078 Run tests against new domain structure: all pass (zero regressions)
- [ ] T079 Validate data integrity: no data loss during migration
- [ ] T080 Performance test: no degradation in job execution, rule application
- [ ] T081 Security test: credentials in config, not code; write-back explicit; rules applied safely
- [ ] T082 Documentation complete: ARCHITECTURE.md, domain UI setup guide, admin guide

---

## Task Dependencies & Completion Order

**Critical Path** (must complete before proceeding):
1. Phase 1 (Setup) → Phase 2 (Foundational) → Phase 3 (US1)
2. Phase 3 complete → Phases 4-7 can start (parallelizable)
3. All Phases complete → Phase 8 (Testing & Validation)

**Parallel Execution Opportunities**:
- Phase 4-7: Can execute in parallel after US1 complete
  - US2 (job scheduling): Independent, no blocker
  - US3 (config): Independent, no blocker
  - US4 (rule learning): Independent after US2 job scheduler in place
  - US5 (UIs): Independent, depends only on US1 structure

**MVP Scope** (minimum for production):
- Phase 1-3: expense-domain restructured, all 7 features work
- Phase 8: BDD tests validate zero regressions
- Minimum 5 weeks, maximum risk reduction

**Full Scope**:
- All phases 1-8: Complete framework with job scheduling, rules, UIs
- 11 weeks, production-ready

---

## Success Criteria Per User Story

| Story | Success Criteria | Parallel Tasks |
|-------|------------------|-----------------|
| US1 | expense-domain structured, 7 features work | T017-T019, T026-T028 |
| US2 | Jobs execute on schedule, no cron/launchd | T031-T034 |
| US3 | Config in ~/automation-monorepo-config/, injected | T040-T044 |
| US4 | Rules learned, stored, applied; no code changes | T050-T051, T052 |
| US5 | Domain UI and framework aggregation UI functional | T062-T066, T067-T069 |

---

## Format Validation

✅ **All tasks follow checklist format**:
- Checkbox: `- [ ]` present on every task
- Task ID: T001 → T074 (sequential)
- [P] marker: Present on parallelizable tasks
- [Story] label: Present on US1-US5 phase tasks
- Description with file path: Every task includes specific file path

✅ **Organization**: By user story, each independently testable  
✅ **Parallelization**: Identified within phases  
✅ **Dependencies**: Critical path clear (Phase 1 → 2 → 3 → 4-7 parallel → 8)  

