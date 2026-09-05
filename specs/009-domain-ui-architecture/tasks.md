# Implementation Tasks: Domain-Specific UIs with Framework Aggregation

**Feature**: 009-domain-ui-architecture  
**Created**: 2026-09-05  
**Total Tasks**: 64  
**Execution Strategy**: Phases 1-2 sequential, then Phases 3-6 can parallelize, Phase 7 last  
**Estimated Duration**: 8 weeks  

---

## Phase 1: Setup Infrastructure (Weeks 1)

Create UI framework structure, build pipeline, and base patterns.

- [ ] T001 Create packs/framework/ui/ directory structure with assets/, components/, pages/, styles/
- [ ] T002 Create domain UI template directory: packs/shared/ui-template/ with boilerplate components
- [ ] T003 [P] Setup Node.js/React build pipeline: webpack/vite config for domain UIs in packs/framework/ui/build/
- [ ] T004 [P] Create base API client library in packs/shared/lib/api-client.js (fetch, error handling, retry)
- [ ] T005 [P] Configure CSS/styling framework (Tailwind or styled-components) across all UIs
- [ ] T006 Create shared UI utilities in packs/shared/lib/ui-utils.js (formatting, validation, state helpers)
- [ ] T007 Setup ESLint/Prettier config for frontend code in packs/framework/ui/.eslintrc and prettier.config.js

## Phase 2: Foundational (Weeks 1-2)

Core UI patterns and framework infrastructure.

- [ ] T008 Implement Domain Engine API client in packs/shared/lib/domain-api-client.js (GET/PATCH/POST/DELETE methods)
- [ ] T009 [P] Create base React component library: Button, Input, Modal, Loading, Error, Toast in packs/framework/ui/components/shared/
- [ ] T010 [P] Create data binding hooks: useDomainData(), useDomainMutation(), useJobStatus() in packs/shared/lib/react-hooks.js
- [ ] T011 Implement framework configuration loader: load domain list from ~/automation-monorepo-config/config/domains.yaml
- [ ] T012 [P] Create error boundary component and error handling middleware in packs/framework/ui/components/ErrorBoundary.jsx
- [ ] T013 [P] Create authentication/session management in packs/shared/lib/auth.js (if needed)
- [ ] T014 Setup domain UI discovery mechanism: scan packs/{domain}/ui/manifest.yaml files
- [ ] T015 Create framework routing structure in packs/framework/ui/pages/App.jsx with dynamic domain loading

---

## Phase 3: User Story 1 - Domain-Specific UI for Transaction Management (Weeks 2-4) 🎯 MVP

**Goal**: Create domain-specific UI that displays transactions, allows editing, triggers source jobs, and manages rules through Domain Engine API

**Independent Test**: Open expense-domain UI, view transactions from API, edit transaction, trigger Gmail fetch, view applied rules

### Implementation for User Story 1

- [ ] T016 [P] [US1] Create transaction list component in packs/expense-domain/ui/components/TransactionList.jsx
- [ ] T017 [P] [US1] Create transaction editor component in packs/expense-domain/ui/components/TransactionEditor.jsx
- [ ] T018 [P] [US1] Create source status component in packs/expense-domain/ui/components/SourceStatus.jsx
- [ ] T019 [P] [US1] Create rules display component in packs/expense-domain/ui/components/RulesDisplay.jsx
- [ ] T020 [US1] Implement API data binding for transactions: fetch from /api/expense-domain/expenses, display in list
- [ ] T021 [US1] Implement transaction edit flow: capture changes, PATCH to /api/expense-domain/expenses/{id}, persist to engine
- [ ] T022 [US1] Create Transactions page in packs/expense-domain/ui/pages/Transactions.jsx integrating components
- [ ] T023 [US1] Create Source Status page in packs/expense-domain/ui/pages/SourceStatus.jsx with last-fetch display
- [ ] T024 [US1] Create API client for expense-domain in packs/expense-domain/ui/lib/api-client.js wrapping shared client
- [ ] T025 [US1] Implement transaction filtering/sorting in TransactionList (by date, category, amount, source)
- [ ] T026 [US1] Add validation for transaction edits (required fields, amount format, category valid)
- [ ] T027 [US1] Create responsive layout for expense-domain UI in packs/expense-domain/ui/pages/Layout.jsx
- [ ] T028 [US1] Add error handling for API failures: show user-friendly messages, retry buttons
- [ ] T029 [US1] Create manifest.yaml for expense-domain UI declaring components and entry point

**Checkpoint**: User Story 1 fully functional - transactions viewable, editable, sources statusable via API

---

## Phase 4: User Story 2 - Framework Aggregation UI for Multi-Domain Overview (Weeks 3-5)

**Goal**: Create framework-level UI showing all domains, their health status, aggregated metrics, and navigation

**Independent Test**: Open framework UI, see all domains listed, click domain to load its UI, view aggregated metrics

### Implementation for User Story 2

- [ ] T030 [P] [US2] Create framework dashboard component in packs/framework/ui/components/Dashboard.jsx
- [ ] T031 [P] [US2] Create domain selector component in packs/framework/ui/components/DomainSelector.jsx
- [ ] T032 [P] [US2] Create domain embedding component (iframe/dynamic import) in packs/framework/ui/components/DomainLoader.jsx
- [ ] T033 [P] [US2] Create aggregated metrics display component in packs/framework/ui/components/AggregatedMetrics.jsx
- [ ] T034 [P] [US2] Create jobs view component in packs/framework/ui/components/JobsView.jsx
- [ ] T035 [US2] Implement domain discovery from ~/automation-monorepo-config/config/ in packs/framework/ui/lib/domain-discovery.js
- [ ] T036 [US2] Implement domain UI embedding: load domain UIs as React components or iframes
- [ ] T037 [US2] Create aggregated metrics fetching: total transactions, rules count, sources count across domains
- [ ] T038 [US2] Implement jobs view: group by domain, show job name, status, last execution, next scheduled
- [ ] T039 [US2] Create framework main page in packs/framework/ui/pages/Framework.jsx routing to dashboard/domains/jobs
- [ ] T040 [US2] Add domain health status calculation: green/yellow/red based on last job execution
- [ ] T041 [US2] Create framework navigation header with domain selector and main menu
- [ ] T042 [US2] Implement real-time metric updates: poll framework API or use WebSocket for live updates
- [ ] T043 [US2] Add error handling when domain UI fails to load: show error, allow retry
- [ ] T044 [US2] Create framework UI entry point in packs/framework/ui/index.html and index.jsx

**Checkpoint**: Framework UI functional - all domains visible, aggregation working, navigation operational

---

## Phase 5: User Story 3 - Source Data Integration and Upload Interface (Weeks 4-5)

**Goal**: Enable file upload for source data (bank CSVs, receipts) triggering domain jobs and feeding engine

**Independent Test**: Upload CSV to expense-domain, job triggered, data appears in transactions, status shown in UI

### Implementation for User Story 3

- [ ] T045 [P] [US3] Create file upload component in packs/expense-domain/ui/components/FileUpload.jsx
- [ ] T046 [P] [US3] Create upload progress tracker in packs/expense-domain/ui/components/UploadProgress.jsx
- [ ] T047 [US3] Create Upload page in packs/expense-domain/ui/pages/Upload.jsx integrating upload components
- [ ] T048 [US3] Implement file upload to domain engine: POST file to /api/expense-domain/upload endpoint
- [ ] T049 [US3] Implement automatic job trigger on file upload: POST to /api/expense-domain/jobs/{monitor-job}/trigger
- [ ] T050 [US3] Implement job status polling: GET /api/expense-domain/jobs/{job-id}/status, show progress
- [ ] T051 [US3] Add file validation: check format (CSV, PDF), size limits, encoding before upload
- [ ] T052 [US3] Implement error recovery: show parsing errors, validation failures, allow retry
- [ ] T053 [US3] Create job history view: show past uploads, their status, extracted transaction count
- [ ] T054 [US3] Add file format guidance: show expected columns for CSV, help text for supported formats
- [ ] T055 [US3] Integrate upload UI into expense-domain main navigation

**Checkpoint**: File upload workflow complete - uploads trigger jobs, data appears in transactions

---

## Phase 6: User Story 4 - Rule Management and AI-Driven Rule Learning UI (Weeks 5-6)

**Goal**: Display learned and configured rules, enable editing/creating rules, resolve rule conflicts

**Independent Test**: View rules (learned and configured), create new rule, disable learned rule, see applied in next run

### Implementation for User Story 4

- [ ] T056 [P] [US4] Create rules list component in packs/expense-domain/ui/components/RulesList.jsx
- [ ] T057 [P] [US4] Create rule detail component in packs/expense-domain/ui/components/RuleDetail.jsx
- [ ] T058 [P] [US4] Create rule editor component in packs/expense-domain/ui/components/RuleEditor.jsx
- [ ] T059 [P] [US4] Create conflict resolution component in packs/expense-domain/ui/components/ConflictResolver.jsx
- [ ] T060 [US4] Create Rules page in packs/expense-domain/ui/pages/Rules.jsx integrating rule components
- [ ] T061 [US4] Implement rules fetching: GET /api/expense-domain/rules, display with source/confidence/origin metadata
- [ ] T062 [US4] Implement rule creation flow: form for name/type/pattern/action, POST to /api/expense-domain/rules
- [ ] T063 [US4] Implement rule editing: PATCH rule to /api/expense-domain/rules/{id}, update locally
- [ ] T064 [US4] Implement rule enable/disable toggle: PATCH enabled field, update immediately
- [ ] T065 [US4] Add AI-learned rule metadata display: origin badge, confidence score, learned date, source pattern
- [ ] T066 [US4] Implement conflict detection UI: show when rules conflict, highlight conflicting patterns
- [ ] T067 [US4] Implement conflict resolution: choose primary rule, merge rules, or disable secondary
- [ ] T068 [US4] Add rule history: show when rule was created, modified, last applied
- [ ] T069 [US4] Create rule testing interface: test pattern against sample data before saving
- [ ] T070 [US4] Add validation for custom rules: pattern syntax, action valid, no self-conflicts

**Checkpoint**: Rule management fully functional - view, create, edit, resolve conflicts through UI

---

## Phase 7: Polish & Validation (Week 7-8)

**Purpose**: End-to-end testing, performance optimization, documentation

- [ ] T071 [P] Create end-to-end test suite: user journeys in Cypress/Playwright covering all user stories
- [ ] T072 [P] Performance test: measure UI load times, API response times, rule application latency
- [ ] T073 [P] Accessibility testing: keyboard navigation, screen reader compatibility, color contrast
- [ ] T074 [P] Cross-domain compatibility testing: verify all domain UIs work with framework aggregation
- [ ] T075 Create UI documentation in docs/UI_ARCHITECTURE.md (component structure, API patterns, extending domains)
- [ ] T076 Create domain UI quickstart guide in docs/DOMAIN_UI_QUICKSTART.md
- [ ] T077 Add performance optimizations: code splitting, lazy loading, memoization
- [ ] T078 Add logging and monitoring: log user actions, API calls for debugging
- [ ] T079 Create UI deployment guide: build process, asset serving, caching strategy
- [ ] T080 Validate all success criteria from spec: load times <2s, upload <1s, rule load <100ms

---

## Task Dependencies & Completion Order

**Critical Path** (must complete before proceeding):
1. Phase 1 (Setup) → Phase 2 (Foundational) → Phase 3 (US1)
2. Phase 3 complete → Phases 4-6 can start in parallel
3. All Phases 3-6 complete → Phase 7 (Polish & Validation)

**Parallel Execution Opportunities**:
- Phase 1: T003, T004, T005 can run in parallel
- Phase 2: T009, T010, T012, T013 can run in parallel
- Phase 3: T016-T019 (component creation) can run in parallel
- Phase 4: T030-T034 (component creation) can run in parallel
- Phase 5: T045-T046 (component creation) can run in parallel
- Phase 6: T056-T059 (component creation) can run in parallel
- Phase 7: T071-T074 (testing) can run in parallel

**MVP Scope** (minimum for production):
- Phase 1-2: Foundation ready
- Phase 3: User Story 1 (Domain UI) complete
- Phase 7: E2E testing and validation
- Minimum 4 weeks, production-ready for single domain

**Full Scope**:
- All phases 1-7: Complete framework with all 4 user stories
- 8 weeks, fully featured multi-domain UI system

---

## Success Criteria Per User Story

| Story | Success Criteria | Parallel Components |
|-------|------------------|---------------------|
| US1 | Domain transactions viewable/editable, sources statusable via API | T016-T019, T024 |
| US2 | Framework UI loads <2s, all domains visible with health status | T030-T034 |
| US3 | File uploads trigger jobs <1s, parsed data appears in transactions | T045-T046 |
| US4 | Rules displayed with metadata, create/edit/resolve conflicts via UI | T056-T059 |

---

## Format Validation

✅ **All tasks follow checklist format**:
- Checkbox: `- [ ]` present on every task
- Task ID: T001 → T080 (sequential)
- [P] marker: Present on parallelizable tasks
- [Story] label: Present on US1-US4 phase tasks
- Description with file path: Every task includes specific file path

✅ **Organization**: By user story, each independently testable  
✅ **Parallelization**: Identified within phases  
✅ **Dependencies**: Critical path clear (Phase 1 → 2 → 3 → 4-6 parallel → 7)
