# Feature Specification: Multi-Domain Architecture with Reusable Pattern

**Feature Branch**: `008-restructure-architecture`

**Created**: 2026-09-05

**Status**: Draft

**Input**: Restructure from flat pack model to hierarchical domain-based architecture where each domain (expense tracking, stock portfolio, trip planning) is an independent, reusable problem space with standardized sources/, reports/, and core mechanism. Domains share a common architectural pattern but not code; data I/O is parameterized and never written to the repository.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Establish reusable domain architecture pattern (Priority: P1)

As an architect/developer, I need a reusable hierarchical structure for independent domains (expense tracking, stock portfolio, trip planning), where each domain has its own sources/, reports/, and core mechanism, so that new problem spaces can be added following the same pattern without code duplication.

**Why this priority**: This is the foundational reorganization that enables the workspace to scale to multiple problem domains. The current flat pack structure obscures domain boundaries and prevents pattern reuse across different problem spaces.

**Independent Test**: Can verify by checking directory structure - each domain (e.g., `packs/expense-domain/`) contains sources/, reports/, and core mechanism. Pattern can be applied to stock-domain and trip-domain without modification, proving reusability.

**Acceptance Scenarios**:

1. **Given** the current flat pack structure, **When** restructuring is complete, **Then** `packs/expense-domain/` demonstrates the pattern with sources/ (gmail/, sms/, imessage/, wallet/), reports/, and core logic clearly separated
2. **Given** the expense-domain pattern established, **When** a developer wants to add stock-domain, **Then** they can mirror the expense structure (sources/ for IBKR, Zerodha, Sharpe; reports/ for portfolio analysis) without new architectural decisions
3. **Given** multiple domains coexist, **When** a composite domain (e.g., financial-planning) needs data from both expense and stock domains, **Then** it can declare dependencies and consume their outputs

---

### User Story 2 - Support data source abstraction and write-back capabilities (Priority: P2)

As a developer, I need each data source (email, SMS, iMessage, Wallet) to follow a consistent interface pattern, so that new data sources can be added without changing core expense tracking logic, and write-back operations are available when supported.

**Why this priority**: This enables extensibility and maintains separation of concerns. Once the structure is in place, the abstraction pattern ensures the system can grow to support new sources (CSV, PDFs, bank APIs) without architectural changes.

**Independent Test**: Can verify by confirming each source has standardized read and write-back interface definitions, and that expense tracker consumes these through a consistent adapter pattern.

**Acceptance Scenarios**:

1. **Given** Gmail, SMS, iMessage, and Wallet sources, **When** examining their module exports, **Then** each exposes consistent read() and writeBack() methods with clear contracts
2. **Given** a new data source needed (e.g., bank CSV), **When** following the source pattern, **Then** it integrates with expense tracker without modifying core expense tracking code

---

### User Story 3 - Enable parallel pack instantiation pattern (Priority: P2)

As a system designer, I need the organization to demonstrate a scalable pattern, so that we can implement parallel packs (e.g., CSV/PDF repository for bank transactions) following the same hierarchical structure and principles.

**Why this priority**: The restructuring establishes a pattern that can be replicated. CSV repository pack would follow the same sources/reports/core mechanism pattern as expense tracking.

**Independent Test**: Can verify by demonstrating how a CSV/PDF bank transaction pack would mirror the expense tracker structure with its own sources (uploaded PDFs, CSV imports) and reports.

**Acceptance Scenarios**:

1. **Given** the expense-tracker pack structure with sources/ subdirectory, **When** designing a csv-bank-transactions pack, **Then** it follows the same pattern with sources/ for PDF uploads and CSV imports
2. **Given** the restructured architecture, **When** documentation is created, **Then** it explicitly shows the pattern can be repeated for additional domain problems

---

### Edge Cases

- What happens to existing orchestration scripts that reference the old pack paths (expenses/, gmail/, wallet/)?
- How are circular dependencies prevented if native expense tracking mechanism needs to reference a source?
- What happens to CLI utilities and configuration that explicitly reference pack paths?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Each domain MUST have a hierarchical structure: `packs/<domain-name>/` containing `sources/`, `reports/`, and `core/` subdirectories
- **FR-002**: Each domain's `sources/` subdirectory MUST contain specialized source connectors (e.g., expense-domain: gmail/, sms/, imessage/, wallet/; stock-domain: ibkr/, zerodha/, sharpe/)
- **FR-003**: Each domain MUST create a `reports/` directory for domain-specific report generation, analysis, and output formatting
- **FR-004**: Each domain MUST establish a core mechanism (data models, business logic, domain-specific processing) as a distinct module within the domain, separate from source connectors
- **FR-005**: Each source connector MUST expose a standardized interface with read operations to fetch domain-specific data and optional write-back operations where supported
- **FR-006**: System MUST enforce data I/O boundaries: domains NEVER write data directly to the repository; all configuration, rules, and data MUST be passed as parameters during initialization
- **FR-007**: System MUST implement all domain outputs to `data/<domain-name>/` (not inside `packs/<domain-name>/`), ensuring packs/ remains read-only per Constitution Principle II
- **FR-008**: Composite domains (e.g., financial-planning) MUST be able to declare dependencies on other domains and consume their output data via published schemas
- **FR-009**: System MUST maintain backward compatibility with existing orchestration scripts during migration (or provide migration path)
- **FR-010**: System MUST create a root-level glossary documenting: domain, source, source connector, composite domain, cluster, sources pattern, reports pattern, and all architectural terms used throughout the workspace
- **FR-011**: System MUST support adding new sources and new domains by following the established pattern without modifying code in other domains

### Key Entities *(include if feature involves data)*

- **Domain**: Independent problem space (expense tracking, stock portfolio, trip planning) with its own sources/, reports/, and core logic; follows a reusable pattern; outputs to data/<domain>/
- **Source Connector**: Standardized component within a domain's sources/ that fetches external data from a specific platform (e.g., gmail, IBKR, MakeMyTrip) and optionally writes back confirmations
- **Composite Domain**: A meta-domain that declares dependencies on other domains and consumes their published output data (e.g., financial-planning depends on expense + stock domains)
- **Domain Output**: Data produced by a domain, written to `data/<domain>/`, never to `packs/`; passed to dependent domains via published schema
- **Sources Pattern**: Reusable architectural pattern: sources/ subdirectory containing pluggable source connectors that follow a standardized interface
- **Reports Pattern**: Reusable architectural pattern: reports/ subdirectory containing analysis and output generation logic specific to the domain
- **Glossary**: Root-level documentation defining all architectural terms (domain, source connector, composite domain, cluster, sources pattern, reports pattern, etc.) for workspace clarity

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: expense-domain restructuring is complete with zero broken imports or paths in existing code
- **SC-002**: expense-domain demonstrates the pattern: sources/ with 4+ connectors (gmail, sms, imessage, wallet), reports/, and core/ — all following the domains pattern
- **SC-003**: Each source connector has a documented, testable interface contract (read and write-back operations) adhering to the sources pattern
- **SC-004**: Existing orchestration and CLI scripts continue to function without modification, or migration guide is provided and scripts are updated
- **SC-005**: Root-level glossary is created documenting domain, source connector, composite domain, sources pattern, reports pattern, and all architectural terms
- **SC-006**: Data I/O boundaries are enforced: zero data written to packs/<domain>/; all config/rules/data passed as parameters; outputs verified to land in data/<domain>/ only
- **SC-007**: Documentation clearly demonstrates how to: (a) add a new source to expense-domain, (b) create stock-domain following the pattern, (c) add a composite domain depending on other domains
- **SC-008**: Architecture makes domain boundaries and the sources/reports/core pattern immediately clear to new developers; pattern is proven reusable (documented for stock-domain, trip-domain)

## Assumptions

- Existing orchestration scripts and CI/CD pipelines can be updated as part of this restructuring
- The current packs (gmail, wallet, telegram, expenses) will be reorganized into the new domain structure (expense-domain) without losing functionality
- Write-back capabilities for all sources are optional; read operations are the primary requirement for v1
- Configuration files and environment variables referencing old pack paths will be migrated during implementation
- The restructuring will be done in a feature branch and merged once all tests pass
- Future domains (stock-domain, trip-domain) will follow the same pattern once expense-domain is complete; they are not part of this feature
- Constitution Principle II (packs/ Is Read-Only) and VII (Local-First, Least Exposure) will be enforced during implementation
- Domains will follow Constitution Principle I (Packs Declare, the Workspace Supplies) — all config/rules/data will be injected as parameters, not discovered

## Clarifications

### Session 2026-09-05

- **Q: Should we establish a general multi-domain pattern or focus only on expense restructuring?** → **A: Option B + Extensibility** — Restructure expense-domain first (P1); extract and document the reusable pattern; architecture must prove it supports stock, trip, and composite domains without modification.
- **Q: Should domains be isolated or able to reference each other?** → **A: Layered with Shared Foundation + Composite Domains** — Domains are independent with shared foundation (sources/reports patterns); composite domains can declare dependencies and consume outputs from other domains.
- **Q: What should go into the shared foundation?** → **A: Hybrid: Core Abstractions + Organic Growth** — Establish sources/ pattern, domain manifest schema, composite domain dependency contract now; allow other patterns to emerge as domains are built.
- **Q: What are the critical data I/O constraints?** → **A: Domains are read-only in repo; all config/rules/data passed as parameters; outputs → data/<domain>/; packs/ remains pristine per Constitution Principle II.**
- **Q: Should we document architectural terminology?** → **A: Yes, create root-level glossary** documenting domain, source connector, composite domain, sources pattern, reports pattern, and all related terms for workspace clarity.

