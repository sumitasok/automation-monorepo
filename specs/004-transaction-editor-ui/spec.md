# Feature Specification: Gmail Transactions Editor UI

**Feature Branch**: `004-transaction-editor-ui`

**Created**: 2026-07-25

**Status**: Draft

**Input**: User description: "lets add a UI capability where a tab is dedicated to data in data/gmail/transactions. i should be able to editthe values of the transactions in the ui. shwo the latest event first"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Review transactions in chronological order (Priority: P1)

As the person reconciling bank transactions extracted from Gmail, I want to open a dedicated Transactions view and immediately see my most recent transactions at the top, so I can quickly check what's just come in without hunting through a spreadsheet sorted the wrong way.

**Why this priority**: Without a reliably and correctly ordered view of the data, editing is moot — this is the foundation everything else in this feature builds on.

**Independent Test**: Open the Transactions tab against the existing transaction data; confirm the top row is the transaction with the most recent date and ordering descends from there.

**Acceptance Scenarios**:

1. **Given** the transaction data spans multiple months, **When** the user opens the Transactions tab, **Then** the transaction list is sorted with the most recent transaction date first and the oldest last.
2. **Given** two transactions share the same date, **When** the list is rendered, **Then** their relative order is stable and does not change between page loads.

---

### User Story 2 - Edit a transaction's details (Priority: P1)

As the person maintaining transaction records, I want to correct or annotate a transaction's details directly in the UI, so I don't have to open a separate spreadsheet tool to fix a miscategorized or mislabeled entry.

**Why this priority**: this is the core value proposition requested — editing capability is the reason for building the UI at all.

**Independent Test**: Select a transaction row, change an editable field's value, save, and confirm the change is reflected both in the UI and in the underlying transaction data.

**Acceptance Scenarios**:

1. **Given** a transaction row displayed in the UI, **When** the user changes an editable field and saves, **Then** the new value is persisted to the underlying transaction data and is visible after the page is reloaded.
2. **Given** a transaction row is being edited, **When** the user cancels without saving, **Then** no changes are written and the original value remains.
3. **Given** a save is attempted with an invalid value (e.g., a non-numeric amount), **When** the user submits, **Then** the UI rejects the change with a clear message and the stored value is unchanged.

---

### User Story 3 - Find a specific transaction (Priority: P2)

As the person managing hundreds of transactions, I want to search/filter the list, so I can jump straight to the transaction I need to edit instead of scrolling through everything.

**Why this priority**: with hundreds of rows and growing, unaided scrolling doesn't scale; this materially speeds up the edit workflow introduced in Story 2.

**Independent Test**: Enter a merchant name or date range in a filter control and confirm only matching rows are shown, still ordered newest-first.

**Acceptance Scenarios**:

1. **Given** the full transaction list is loaded, **When** the user filters by merchant name, **Then** only transactions matching that merchant are shown, most recent first.
2. **Given** a filter yields no matches, **When** applied, **Then** the UI shows a clear empty state rather than an error.

---

### Edge Cases

- What happens when the transaction data has been modified externally (e.g. by the Gmail sync/categorization pipeline) while the UI is open? The system should detect staleness and prompt a refresh rather than silently overwrite newer data on save.
- How does the system handle two edits to the same row happening close together (e.g. two browser tabs open)? Last-write-wins is acceptable, but the underlying data must never be left corrupted (e.g. partial rows, broken structure).
- What happens if a transaction record is missing a date? It should sort to one consistent end of the list (e.g. last) rather than break the ordering or crash.
- What happens when there are zero transactions to show, or the data hasn't been synced yet? Show a clear empty-state message rather than a blank or broken view.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a dedicated Transactions view (tab) that displays every transaction record currently stored in `data/gmail/transactions.csv`.
- **FR-002**: The Transactions view MUST order transactions with the most recent transaction date shown first (descending), and MUST re-apply this ordering whenever the underlying data changes.
- **FR-003**: The system MUST allow a user to edit a transaction's annotation/classification fields — Category, SubCategory, Labels, Note, and UserComment — and persist that change. Fields extracted from the source email (e.g. Amount, Account, TxnDate, Merchant, EmailDate, Type, Info, AvailableBalance, Subject, BankFrom, FilterName, FilterQuery, Source) remain read-only, so a transaction record can never silently diverge from the email it was extracted from.
- **FR-004**: The system MUST NOT allow editing of a transaction's unique identifier or any other extracted/read-only field (see FR-003), since these are the link back to the source email and must remain stable.
- **FR-005**: The system MUST validate an edited value against the expected format for that field (e.g. Category/SubCategory must be non-empty text, Labels must follow the existing label format used by the categorization pipeline) before saving, and MUST reject invalid input with a clear, actionable message without saving it.
- **FR-006**: The system MUST persist a saved edit such that the change is visible on next load and to any other process reading the same transaction data (e.g. the categorization pipeline).
- **FR-007**: Users MUST be able to cancel an in-progress edit with no change persisted.
- **FR-008**: The system MUST allow users to search/filter the transaction list (e.g. by merchant, category, or date range) while preserving the newest-first ordering.
- **FR-009**: The system MUST show a clear empty state when there are no transactions to display, whether because there is no data or because a filter has no matches.
- **FR-010**: The system MUST detect when the underlying transaction data has changed since it was loaded (e.g. an external sync ran) and prompt the user to refresh before a save could overwrite newer data.

### Key Entities *(include if feature involves data)*

- **Transaction**: A single bank transaction record extracted from Gmail (existing entity — currently stored in `data/gmail/transactions.csv`). Relevant attributes for this feature: a stable identifier (not editable), a transaction date used for ordering, a set of extracted, read-only fields (Amount, Account, Merchant, TxnDate, EmailDate, Type, Info, AvailableBalance, Subject, BankFrom, FilterName, FilterQuery, Source), and a set of user-editable annotation fields (Category, SubCategory, Labels, Note, UserComment).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can open the Transactions view and see their most recent transaction at the top within 2 seconds of loading, for a dataset of at least 500 transactions.
- **SC-002**: A user can locate and successfully save an edit to a specific transaction in under 30 seconds without leaving the view.
- **SC-003**: 100% of saved edits are reflected correctly in the underlying transaction data with no data loss or record corruption, verified across at least 50 consecutive edit operations.
- **SC-004**: Invalid edits (e.g. malformed dates or non-numeric amounts) are rejected before being saved, with zero invalid values ever reaching the stored data, across all editable fields.

## Assumptions

- The UI operates on `data/gmail/transactions.csv`, the canonical, actively-synced transaction file — not the dated snapshot files or the backup file that also exist alongside it in `data/gmail/`.
- This is a single-user, local-use tool, consistent with the rest of this personal automation project — no multi-user authentication or authorization is required.
- "Latest event first" means ordering by the transaction's transaction date (falling back to the email date when the transaction date is missing), descending.
- Saving an edit is an explicit user action (e.g. a Save control per row/field) rather than continuous autosave on every keystroke, to avoid partial or accidental writes to the pipeline's source-of-truth data.
- Undo/version history relies on the existing version control history of the transaction data rather than requiring new in-app versioning.
- This Transactions view is the first tab of what may become a multi-tab UI; no other tabs are in scope for this feature.
