# Feature Specification: Domain-Specific UIs with Framework Aggregation

**Feature Branch**: `009-domain-ui-architecture`

**Created**: 2026-09-05

**Status**: Draft

**Input**: UI architecture for multi-domain framework: domain-specific UIs access domain engines via API (read/write domain data, trigger source jobs, manage rules); framework aggregation UI unifies all domains; data binding patterns enable UI↔Engine communication; source data flows through domain engine; write-back capabilities available through engine API.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Domain-Specific UI for Transaction Management (Priority: P1)

As a user working with the expense domain, I need a domain-specific UI that displays transactions, lets me edit them, trigger data source syncs, and manage categorization rules—all through a clean interface communicating with the domain engine API.

**Why this priority**: Domain UIs are the primary user interaction point. Without them, users cannot interact with domain data. Expense transactions are the most visible use case.

**Independent Test**: Can verify by opening expense-domain UI, viewing transactions from domain engine API, editing a transaction and confirming update persists, triggering Gmail fetch job, and viewing applied rules.

**Acceptance Scenarios**:

1. **Given** expense-domain UI loaded, **When** user navigates to Transactions tab, **Then** UI fetches and displays transactions from `/api/expense-domain/expenses`
2. **Given** a transaction displayed, **When** user clicks Edit and updates the category, **Then** UI sends PATCH to `/api/expense-domain/expenses/{id}` and engine persists change
3. **Given** Gmail source available, **When** user clicks "Sync Gmail", **Then** UI triggers POST `/api/expense-domain/jobs/gmail-fetch-job/trigger` and polls job status
4. **Given** multiple data sources (Gmail, Wallet, SMS), **When** user views Source Status tab, **Then** UI displays status for each source, last fetch time, next scheduled fetch
5. **Given** rules have been learned, **When** user views Rules tab, **Then** UI displays learned and configured rules, allows editing, adding new rules

---

### User Story 2 - Framework Aggregation UI for Multi-Domain Overview (Priority: P1)

As an administrator, I need a framework-level UI that shows all domains available, their status, key metrics, and provides navigation to domain-specific UIs—giving a unified view without needing to know each domain's details.

**Why this priority**: Framework UI is the entry point for users. It must discover and load domain UIs dynamically, providing navigation and high-level visibility.

**Independent Test**: Can verify by opening framework UI, seeing all available domains listed, clicking on a domain to navigate to its UI, viewing aggregated metrics (total transactions, rules, sources, jobs).

**Acceptance Scenarios**:

1. **Given** framework UI loaded, **When** user views Dashboard, **Then** UI displays aggregated metrics: total transactions across all domains, total learned rules, active sources count
2. **Given** multiple domains configured, **When** user views Domain Selector, **Then** UI lists all available domains with health status (green/yellow/red based on last job execution)
3. **Given** expense-domain in list, **When** user clicks on it, **Then** UI loads expense-domain UI as embedded component/iframe and routes navigation appropriately
4. **Given** framework UI viewing Jobs, **When** user sees job listing, **Then** jobs are grouped by domain and show: job name, last execution time, status, next scheduled run
5. **Given** rule conflicts possible across domains, **When** framework detects conflict, **Then** aggregation UI alerts user and provides conflict resolution interface

---

### User Story 3 - Source Data Integration and Upload Interface (Priority: P2)

As a user needing to upload bank statements or other data sources, I need the domain UI to provide an upload interface that triggers source monitor jobs, feeds data to the domain engine, and shows processing status—enabling manual data ingestion alongside automatic syncs.

**Why this priority**: Supports manual data entry (bank CSVs, receipts) which is critical for expense tracking and other domains. Enables flexibility for sources requiring manual upload.

**Independent Test**: Can verify by uploading a CSV file through domain UI, job being triggered, file being processed, data appearing in domain transactions, and user receiving feedback on success/failure.

**Acceptance Scenarios**:

1. **Given** bank CSV upload interface in expense-domain UI, **When** user selects and uploads a CSV file, **Then** UI sends file to domain engine's upload endpoint
2. **Given** file uploaded, **When** framework detects upload, **Then** bank-csv-monitor-job is automatically triggered (or user-triggered, depending on config)
3. **Given** job processing file, **When** user views job status, **Then** UI shows real-time processing status (parsing, extracting, feeding domain engine)
4. **Given** processing completes, **When** user returns to Transactions tab, **Then** newly parsed transactions appear in the list with source attribution
5. **Given** processing fails, **When** user views status, **Then** UI displays error message and logs with details (parsing error, validation failure, etc.)

---

### User Story 4 - Rule Management and AI-Driven Rule Learning UI (Priority: P2)

As a power user, I need to view learned rules, create custom rules, manage rule conflicts, and enable/disable rules—giving me full control over how the domain processes data while understanding which rules are AI-learned vs. manually configured.

**Why this priority**: Rule management is core to self-improving application. Users need visibility and control over rules to ensure correctness.

**Independent Test**: Can verify by viewing rules list (learned and configured), creating a new rule through UI, disabling a learned rule, and confirming rule application in next data processing run.

**Acceptance Scenarios**:

1. **Given** Rules tab open in domain UI, **When** user views rule list, **Then** UI displays all rules with columns: name, type (categorization/validation/dedup), source/engine, confidence, created date, enabled/disabled toggle
2. **Given** learned rule displayed, **When** user hovers over it, **Then** UI shows origin badge (AI-learned, confidence %, learned date) and source data pattern that triggered learning
3. **Given** two rules conflict, **When** framework detects conflict, **Then** UI displays conflict resolution: shows both rules, highlights conflicting patterns, lets user choose primary rule or merge them
4. **Given** user wants custom rule, **When** user clicks "Create Rule", **Then** UI provides form: name, type, conditions (pattern matching), action, enabled checkbox
5. **Given** custom rule created, **When** user saves, **Then** rule is persisted to engine config and applied on next data processing run

---

### Edge Cases

- What happens when domain engine is unavailable? UI shows error, offers retry, queues actions locally
- What happens when source job fails mid-upload? UI displays partial progress, allows retry, preserves user's file
- What happens when two concurrent edits happen to same transaction? Engine applies last-write-wins or shows conflict to user
- What happens when rule learns but has <95% confidence? Framework flags for user review before applying
- What happens when UI tries to access domain that doesn't exist? Framework UI shows error, removes invalid domain from list

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Domain UI MUST fetch domain data from Domain Engine API at `/api/{domain}/{resource}`; all read operations go through API, not direct file access
- **FR-002**: Domain UI MUST support writing updates back to domain engine through API (PATCH/POST), which may trigger write-back to sources
- **FR-003**: Domain UI MUST provide interface to trigger source jobs (fetch now, monitor, upload file) through `/api/{domain}/jobs/{job-name}/trigger`
- **FR-004**: Domain UI MUST display source status (health, last fetch, next scheduled fetch) retrieved from source adapter APIs
- **FR-005**: Domain UI MUST support file upload interface for sources requiring manual data input (bank CSV, receipts, etc.); uploaded files trigger monitor jobs
- **FR-006**: Domain UI MUST display learned rules with metadata: origin (AI-learned vs. configured), confidence, learned date, source pattern
- **FR-007**: Domain UI MUST allow rule management: create, edit, enable/disable, delete custom rules; detect and resolve conflicts
- **FR-008**: Domain UI MUST show real-time job execution status; user can monitor processing, see progress, view errors
- **FR-009**: Framework Aggregation UI MUST discover available domains from framework config; dynamically load and embed domain UIs without requiring UI code changes
- **FR-010**: Framework Aggregation UI MUST provide navigation between domains; display aggregated metrics (transactions count, rules count, sources count, job status)
- **FR-011**: Framework Aggregation UI MUST show framework-level job scheduler; jobs grouped by domain, showing schedule, last execution, next run
- **FR-012**: Framework Aggregation UI MUST resolve cross-domain rule conflicts; display conflict resolution interface when rules from different domains conflict
- **FR-013**: UI MUST be accessible through browser; domain UIs are static artefacts under `packs/{domain}/ui/`, served by framework (not domain itself)
- **FR-014**: UI MUST provide error handling: clear error messages, retry mechanisms, graceful degradation when API unavailable
- **FR-015**: UI data binding MUST be bidirectional: read from domain engine, write back to engine, trigger jobs, receive status updates

### Key Entities

- **Domain UI**: Component-based interface for a specific domain (expense-domain/ui/, stock-domain/ui/, etc.); accesses domain via API
- **Framework Aggregation UI**: Unified interface showing all domains; discovers domain UIs dynamically; provides navigation and aggregation
- **Data Binding**: Pattern connecting UI components to domain engine API (fetch on load, update on change, poll for status)
- **Job Trigger**: User action that initiates a job through framework job scheduler (fetch now, upload, process, learn rules)
- **Rule Manager**: UI component displaying, creating, editing, and resolving rules
- **Conflict Resolution**: UI flow when rules conflict; shows options and lets user choose resolution

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Domain UIs fully functional for all domains (expense, stock, trip); users can view, edit, delete domain data through UI
- **SC-002**: Framework aggregation UI loads in <2 seconds; discovers all available domains automatically
- **SC-003**: File upload interface works for bank CSVs and other source documents; jobs triggered within 1 second of upload
- **SC-004**: Source status displays current health; updates within 5 seconds of last fetch completion
- **SC-005**: Rule management UI displays all rules (learned and configured) without performance degradation; <100ms to load rule list
- **SC-006**: Learned rules with <95% confidence flagged for user review; user can approve or reject before applying
- **SC-007**: Cross-domain rule conflicts detected and resolved through UI within 5 minutes of conflict detection
- **SC-008**: Domain UI↔Engine API communication has zero data loss; all edits persisted, all triggers confirmed
- **SC-009**: Framework aggregation shows accurate aggregated metrics (update within 30 seconds of change in any domain)
- **SC-010**: Error handling tested: API unavailable, network timeout, job failure all show appropriate UI messages with retry options
- **SC-011**: Domain UI accessible through browser at framework-provided URLs; no special setup required per domain

## Assumptions

- Domain Engine API is implemented and stable before UI development begins
- Framework has job scheduler implemented and exposed via API
- Each domain follows the same directory structure (sources/, engine/, ui/, jobs/); new domains can be added without UI code changes
- Browser has JavaScript enabled; responsive design targets desktop and tablet (mobile out of scope for v1)
- User has permissions to access data in domain (authentication/authorization handled by framework)
- Rule learning produces YAML rules that can be displayed and edited through UI
- Concurrent edits to same resource use last-write-wins or explicit user conflict resolution
- Framework provides domain discovery mechanism (config-based or API-based); UI queries framework to find available domains

