# Feature Specification: GitHub Wallet Records Viewer

**Feature Branch**: `feature/github-wallet-viewer`

**Created**: 2026-08-29

**Status**: Ready for Planning

## User Scenarios & Testing

### User Story 1 - Load Records from GitHub (Priority: P1)

User authenticates with a GitHub Personal Access Token (read-only) to fetch wallet transaction records stored in a GitHub repository. The PAT is securely stored in browser cookies for subsequent requests without re-authentication.

**Why this priority**: This is the foundation—without data access, nothing else works. It's the core integration point.

**Independent Test**: User provides a valid GitHub PAT, records.jsonl is successfully fetched from the configured repo, and subsequent page reloads don't require re-authentication (PAT is restored from secure cookie).

**Acceptance Scenarios**:

1. **Given** user visits the UI for the first time, **When** they paste a valid GitHub PAT in the auth form, **Then** records are fetched from the GitHub repo and displayed.
2. **Given** records have been loaded and user refreshes the page, **When** the page reloads, **Then** records remain visible without prompting for the PAT again (restored from secure cookie).
3. **Given** user provides an invalid PAT, **When** they submit, **Then** an error message clearly indicates auth failure and prompts for a valid token.

**Edge Cases**:
- What happens when the GitHub repo or records.jsonl file doesn't exist?
- What happens when the token is revoked or expired?
- What happens when network is interrupted mid-fetch?

---

### User Story 2 - View and Search Records (Priority: P1)

User views all fetched transaction records in an interactive, sortable table with search and filter capabilities. They can find specific transactions by date, amount, or counterparty.

**Why this priority**: After data is loaded, users need a way to browse and locate records. This is essential for the primary use case.

**Independent Test**: With records loaded, user can search by counterparty name, sort by date/amount in ascending/descending order, and the displayed results update instantly without page reload.

**Acceptance Scenarios**:

1. **Given** records are loaded, **When** user types "Blinkit" in the search box, **Then** only transactions with "Blinkit" as counterparty are shown.
2. **Given** records are displayed, **When** user clicks the "Date" column header, **Then** records are sorted by date (ascending on first click, descending on second).
3. **Given** records are displayed, **When** user enters an amount in the "Amount" filter, **Then** only records matching that amount (or range) are shown.
4. **Given** 6000+ records are loaded, **When** user applies a filter, **Then** results are shown within 1 second with no lag.

**Edge Cases**:
- What happens when search returns zero results?
- What happens when sorting 6000+ records?
- How are negative amounts (expenses) displayed vs. positive (income)?

---

### User Story 3 - View Record Details (Priority: P2)

User clicks on a record to see full details (all fields: amount, date, category, labels, notes, account info). Details are shown in an expanded view or modal without leaving the table.

**Why this priority**: While core functionality works without this, drilling into transaction details is important for reconciliation and verification use cases.

**Independent Test**: User clicks on any record row, and a detail panel/modal opens showing all transaction metadata. User can close it and continue browsing other records.

**Acceptance Scenarios**:

1. **Given** a record row is visible, **When** user clicks it, **Then** a detail view shows all fields for that transaction.
2. **Given** detail view is open, **When** user closes it, **Then** they return to the table view with their search/filter state preserved.

---

---

## Requirements

### Functional Requirements

- **FR-001**: System MUST fetch records.jsonl from the automation-monorepo GitHub repository using a read-only PAT provided by the user.
- **FR-002**: System MUST read the repository URL from the repo's .git/config file (or provide it as a fallback default).
- **FR-003**: System MUST securely store the GitHub PAT in browser cookies (httpOnly flag, secure flag for HTTPS) so it persists across page reloads without re-authentication.
- **FR-004**: System MUST display all records in a sortable table with all fields as-is from records.jsonl (amount, date, category, counterparty, labels, account, notes, etc.).
- **FR-005**: System MUST support filtering records by date range, amount range, and counterparty (text search/contains).
- **FR-006**: System MUST support case-insensitive search across counterparty names, notes, and other text fields.
- **FR-007**: System MUST allow users to view full transaction details (drill-down) on-demand without page navigation or external links.
- **FR-008**: System MUST handle 6000+ records efficiently (lazy loading, pagination, or virtual scrolling to avoid performance lag).
- **FR-009**: System MUST display clear error messages when auth fails, network fails, or records.jsonl format is invalid.
- **FR-010**: System MUST never expose the PAT in console logs, network requests visibility, or HTML attributes—it's used in Authorization header only.
- **FR-011**: UI is read-only—no editing, modifying, or exporting records. Analysis only (sorting, filtering, viewing).

### Key Entities

- **Transaction Record**: Represents a wallet transaction with fields: id, amount (value + currency), recordDate, counterParty, category, account, labels, notes, createdAt, recordState.
- **GitHub PAT**: Read-only Personal Access Token stored securely for API access.
- **Filter State**: User's current search query, date range, amount range, sort column, and sort direction (preserved in URL or sessionStorage if possible).

## Success Criteria

### Measurable Outcomes

- **SC-001**: Users can authenticate and load records within 10 seconds of providing a valid GitHub PAT.
- **SC-002**: Search and filter results are displayed in under 500ms for 6000+ records after applying a filter.
- **SC-003**: 95% of records display correctly (no parsing errors, all fields visible on demand).
- **SC-004**: Page remains usable (sortable, searchable, no crashes) with a 2MB records.jsonl file (6000+ records).
- **SC-005**: Users can drill into any record's details instantly without page reload or network fetch.

## Assumptions

- **GitHub API Access**: Repository is accessible via GitHub API with a valid read-only PAT from the authenticated user.
- **Repository**: automation-monorepo GitHub repo (URL read from .git/config or provided as default).
- **Records Format**: records.jsonl contains one JSON object per line (standard JSONL format) with a metadata header on first line; all fields are displayed as-is (no transformation).
- **Data Source**: UI reads from ~/data/wallet/records.jsonl (which syncs to GitHub repo); this IS the repo data, not a transformed copy.
- **Browser Support**: Target modern browsers (Chrome, Firefox, Safari, Edge) with ES6+ support and cookie support.
- **HTTPS Only**: PAT storage assumes HTTPS is used in production (secure cookie flag requires HTTPS).
- **Single User Context**: UI is a personal analysis tool; no per-user record filtering or access control beyond repo access.
- **Read-Only Analysis**: Users cannot edit, delete, or export records. UI is for browser-based analysis only (sort, filter, drill-down).
- **No Scheduled Sync**: Records are fetched on-demand when user loads the UI; no background refresh or polling.
- **UI Deployment**: Static HTML artifact deployed to packs/wallet/index.html (GitHub Pages); embedded in wallet pack; no backend server needed.

## Clarifications Resolved

**Q1 - Authentication**: ✅ Manual PAT entry only (GitHub OAuth support deferred to future iteration)

**Q2 - Data Handling**: ✅ No export feature. UI displays data as-is from ~/data/wallet/records.jsonl (which IS the GitHub repo data). Read-only analysis only.

**Q3 - Deployment**: ✅ Static HTML artifact at `packs/wallet/index.html`, embedded in wallet pack, deployed to GitHub Pages.
