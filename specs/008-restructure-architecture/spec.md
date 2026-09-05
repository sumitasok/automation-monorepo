# Feature Specification: Hierarchical Pack Architecture with Expense Tracker as Core

**Feature Branch**: `008-restructure-architecture`

**Created**: 2026-09-05

**Status**: Draft

**Input**: Restructure directories from flat pack model (all packs as siblings) to hierarchical model where expense tracking is the core application with data sources as subordinate components.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Reorganize packs into hierarchical structure (Priority: P1)

As an architect/developer, I need the codebase organized with expense tracking as the primary application and all external data sources (Gmail, SMS, iMessage, Wallet, etc.) clearly positioned as subordinate data sources, so that the system's primary concern (expense tracking) is architecturally distinguished from its inputs.

**Why this priority**: This is the foundational reorganization that all other improvements depend on. The current flat pack structure obscures the real problem being solved (expense tracking) and treats all components equally.

**Independent Test**: Can verify by checking directory structure - expense-tracker contains a sources/ directory with gmail/, sms/, imessage/, and wallet/ subdirectories, each responsible for fetching data from their respective platforms.

**Acceptance Scenarios**:

1. **Given** the current flat pack structure (expenses, gmail, wallet, telegram, etc.), **When** the restructuring is complete, **Then** the directory layout clearly shows expense-tracker as the core with sources/ subdirectory containing email, messaging, and wallet connectors
2. **Given** a developer new to the project, **When** they examine the root directory, **Then** they immediately understand expense tracking is the primary focus and other systems are data inputs

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

- **FR-001**: System MUST reorganize directory structure so that `packs/expense-tracker/` contains the core expense tracking application
- **FR-002**: System MUST establish `packs/expense-tracker/sources/` subdirectory containing specialized connectors: `gmail/`, `sms/`, `imessage/`, `wallet/`
- **FR-003**: System MUST create a `packs/expense-tracker/reports/` directory for report generation, analysis, and output formatting
- **FR-004**: System MUST establish native expense tracking mechanism (data models, business logic, transaction processing) as a core module within the expense-tracker pack, distinct from source connectors
- **FR-005**: Each source (gmail, sms, imessage, wallet) MUST expose a standardized interface with read operations to fetch transaction data and write-back operations where supported
- **FR-006**: System MUST maintain backward compatibility with existing orchestration scripts during migration (or provide migration path)
- **FR-007**: System MUST establish that data sources cannot directly modify core expense tracking data; all writes go through defined channels
- **FR-008**: System MUST support adding new data sources (CSV, SMS parsing, bank PDFs) by following the established source pattern without modifying core expense tracking logic

### Key Entities *(include if feature involves data)*

- **Expense Tracker Pack**: Core application responsible for transaction management, categorization, rules enforcement, and reporting
- **Data Source**: Standardized connector (gmail, sms, imessage, wallet) that fetches external transaction data and optionally writes back confirmations/updates
- **Transaction**: Individual expense record with source attribution, amount, category, and metadata
- **Report**: Aggregated view of transactions (by category, date, source, etc.) generated from expense data

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Directory structure reorganization is complete with zero broken imports or paths in existing code
- **SC-002**: Each data source module has a documented, testable interface contract (read and write-back operations)
- **SC-003**: Existing orchestration and CLI scripts continue to function without modification, or migration guide is provided and scripts are updated
- **SC-004**: Documentation clearly demonstrates how to add a new data source (e.g., bank CSV import) following the established pattern
- **SC-005**: The structure makes the expense tracking problem domain immediately clear to new developers reviewing the codebase

## Assumptions

- Existing orchestration scripts and CI/CD pipelines can be updated as part of this restructuring
- The current packs (gmail, wallet, telegram, expenses) will be reorganized into the new structure without losing functionality
- Write-back capabilities for all sources are not required in v1; read operations are primary
- Configuration files and environment variables referencing old pack paths will be migrated during implementation
- The restructuring will be done in a feature branch and merged once all tests pass

