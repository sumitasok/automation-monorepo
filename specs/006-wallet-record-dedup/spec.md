# Feature Specification: Wallet Record Deduplication

**Feature Branch**: `006-wallet-record-dedup`

**Created**: 2026-08-29

**Status**: Draft

**Input**: User description: "have a mechanism to dedup the records in pack/wallet already stored in records.json. then update those record/delete the duplicate after confirmation"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Identify Duplicate Wallet Records (Priority: P1)

A user managing their personal finances discovers that the wallet pack's records.json contains duplicate transaction records (likely from failed imports, retry logic, or data migrations). They need a way to detect these duplicates so they can clean them up without manually inspecting the raw JSON.

**Why this priority**: Data quality directly affects financial accuracy and user trust. Duplicates skew totals, budgets, and analytics. This is the foundational capability that must work before any cleanup action.

**Independent Test**: The system can scan records.json, identify records that match based on key fields (amount, date, counterparty, category), and report findings to the user. Test passes if all genuine duplicates are found and false positives are minimal.

**Acceptance Scenarios**:

1. **Given** records.json contains two identical records (same amount, date, counterparty), **When** the dedup scan runs, **Then** both records are flagged as duplicates
2. **Given** records.json contains similar but not identical records (same amount/date but different counterparty), **When** the dedup scan runs, **Then** they are not flagged as duplicates
3. **Given** records.json is empty or has no duplicates, **When** the dedup scan runs, **Then** the user is informed no duplicates were found
4. **Given** a record was manually edited after import, **When** the dedup scan runs, **Then** the near-duplicate is suggested but marked as uncertain

---

### User Story 2 - Review and Confirm Dedup Action (Priority: P1)

After duplicates are identified, the user must be able to review them in a clear format and explicitly confirm which records to keep/delete before any data is modified. This prevents accidental data loss.

**Why this priority**: Irreversible data deletion requires explicit user consent. A P1 blocking requirement for any dedup tool.

**Independent Test**: The system shows identified duplicates in a human-readable format (date, amount, counterparty, tags), groups them, and requires explicit user confirmation (yes/no/skip) for each group. Test passes if the user can review all duplicates and make informed decisions without touching the raw data.

**Acceptance Scenarios**:

1. **Given** the dedup scan found 3 duplicate groups, **When** presented to the user, **Then** each group is clearly labeled with record IDs, amounts, dates, and counterparties
2. **Given** a duplicate group is shown, **When** the user chooses "keep first, delete rest", **Then** the system records this decision for confirmation
3. **Given** the user reviews multiple groups, **When** they confirm all decisions, **Then** the system asks for final confirmation ("Delete X records?")
4. **Given** the user cancels before final confirmation, **When** they exit, **Then** no records are modified and the list of decisions is discarded

---

### User Story 3 - Execute Dedup and Update Records (Priority: P1)

After user confirmation, the wallet pack's records.json is updated: duplicate records are removed and any retained duplicates (if the user chose to keep some) remain intact. The operation is atomic so partial failures don't corrupt the file.

**Why this priority**: The dedup has no value if the cleanup doesn't actually persist. This is the delivery mechanism.

**Independent Test**: After dedup confirmation, records.json is updated to remove duplicates selected for deletion. Retained records are unchanged. A backup of the original is created and the operation completes without corrupting the JSON structure. Test passes if records.json is valid JSON post-update and contains exactly the records the user confirmed.

**Acceptance Scenarios**:

1. **Given** the user confirmed deletion of 2 duplicate records, **When** the update runs, **Then** records.json contains the remaining unique records only
2. **Given** records.json had 100 records with 5 duplicates removed, **When** the update completes, **Then** records.json contains exactly 95 valid records
3. **Given** the update fails partway (e.g., disk full), **When** an error occurs, **Then** records.json is reverted to its pre-update state and the error is reported
4. **Given** the dedup completes successfully, **When** the user reviews records.json, **Then** a backup file is available at `records.json.backup.{timestamp}` for recovery if needed

---

### Edge Cases

- What happens when the same record appears 3+ times (more than 2 duplicates)?
- How does the system handle records with missing or NULL fields (e.g., no category or tags)?
- How does it distinguish between a legitimate duplicate (same transaction processed twice) and two separate transactions that happen to have the same amount on the same day but different merchants?
- What if a record has been partially edited post-duplicate (e.g., tags were added to one copy but not the other)?
- How does the system handle very large records.json files (10K+ records) without hanging or consuming excessive memory?

## Requirements *(mandatory)*

### Functional Requirements

**Constitution Alignment (Principle II & VII):**
- All dedup operations MUST read from and write to `data/wallet/records.json` (wallet pack's declared data file), never directly modify `packs/wallet/`
- Backups and working state MUST be created in `data/wallet/` only
- The dedup mechanism MUST not expose record contents in logs or debug output (Principle VII: Least Exposure)

**Deduplication Logic:**

- **FR-001**: System MUST identify duplicate records by matching on: transaction amount, transaction date, and counterparty name (primary dedup key)
- **FR-002**: System MUST support user-configurable comparison logic: optionally include/exclude category, tags, or notes in the duplicate match (configuration over code, Principle V)
- **FR-003**: System MUST flag records as "uncertain duplicates" if some fields match but others diverge (e.g., same amount/date/counterparty, different category or notes)
- **FR-004**: System MUST group duplicate records by transaction and present all copies of each transaction together
- **FR-005**: System MUST indicate which record in each group is the "original" (oldest by createdAt) and which are "duplicates"

**User Confirmation & Safety:**

- **FR-006**: System MUST present duplicate groups in a tabular or structured format showing: record ID, date, amount, counterparty, category, tags, and createdAt timestamp
- **FR-007**: System MUST require explicit user confirmation for each duplicate group (e.g., "Keep which? Delete which?")
- **FR-008**: System MUST provide a summary of all deletions with a "confirm all" step before executing any data modification
- **FR-009**: System MUST allow the user to skip or exclude specific duplicate groups from dedup

**Data Integrity:**

- **FR-010**: System MUST create a timestamped backup of records.json before any modification (`records.json.backup.{YYYYMMDD-HHMMSS}`)
- **FR-011**: System MUST write deduplicated records atomically (all-or-nothing) so partial failures don't corrupt records.json
- **FR-012**: System MUST validate the deduplicated records.json is valid JSON before confirming success
- **FR-013**: System MUST log all dedup operations (what was deleted, when, by whom) in a structured audit trail, not in records.json but in a separate log file

**CLI Integration:**

- **FR-014**: The dedup mechanism MUST be callable from the `auto` CLI as a subcommand: `auto wallet dedup` or similar
- **FR-015**: The CLI MUST support dry-run mode (identify duplicates, show summary, exit without modifying records.json)
- **FR-016**: The CLI MUST support non-interactive mode (read a manifest of dedup decisions from a file and apply them without prompting)

### Key Entities

- **Record**: A transaction record in records.json with fields: id, date, amount, counterparty, category, tags, notes, createdAt, updatedAt
- **Duplicate Group**: A set of 2+ records that match on the dedup key (amount, date, counterparty)
- **Dedup Decision**: User's choice for a duplicate group (keep which record, delete which)
- **Audit Trail**: Timestamped log of all dedup operations performed

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: System correctly identifies 100% of genuine duplicate records (same amount, date, counterparty) with zero false positives in test datasets
- **SC-002**: User can review and confirm dedup of 1000 duplicate groups in a single session without the tool timing out or consuming >500MB memory
- **SC-003**: Records.json is updated atomically: either all confirmed deletions succeed or none occur; post-dedup JSON is always valid
- **SC-004**: Dedup operation completes for 10K-record files in under 5 seconds (scan + confirmation + write)
- **SC-005**: User never loses data due to dedup: backups are created for every run and recovery instructions are clear
- **SC-006**: Dedup audit trail captures date, time, records deleted, and who initiated (or note "non-interactive mode" if CLI-driven)

## Assumptions

- **Duplicate Definition**: "Duplicate" is defined as two records with identical amount, date, and counterparty. Records with matching amount/date but different counterparties are NOT duplicates (legitimate separate transactions).
- **Primary Key**: The record ID (id field) is immutable and unique within records.json; it can be used to reference records in logs and audit trails.
- **User Interaction Model**: The user has access to a CLI and can make yes/no decisions when prompted. If true CLI cannot be used, non-interactive mode with a manifest file is acceptable.
- **Data Format**: records.json is a valid JSON file (array or object) with standard transaction record structure. Pre-existing corruption or malformed JSON is out of scope; the tool assumes valid input and fails loudly if input is invalid.
- **Scope**: Dedup applies to records.json only. Other wallet data (accounts, budgets, categories) are not affected by this feature.
- **Configuration Scope**: Category and tags are optional dedup key extensions; default behavior uses amount + date + counterparty only. Per-instance support for a new comparison rule is added as data (config), not code.
- **Performance**: System has sufficient disk space to create a backup copy of records.json and sufficient RAM to hold all records in memory during dedup.
- **Backwards Compatibility**: The feature adds new CLI commands but does not change existing wallet pack interfaces or record structure; no migration needed for existing users.
