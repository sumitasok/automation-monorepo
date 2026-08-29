# Data Model: Wallet Record Deduplication

**Date**: 2026-08-29 | **Version**: 1.0

## Overview

The dedup system operates on `records.json` (fetched Wallet API dataset) and tracks dedup operations in `state.json` (audit trail). The data model defines how duplicate groups are detected, represented, and acted upon.

## Primary Entities

### Record (from records.json)

A single transaction record fetched from the Wallet API.

```json
{
  "id": "65436aec-bbd0-45a3-8687-e9501dc22c06",
  "accountId": "6cf80ab9-85bd-420a-aec4-8498005f4ce8",
  "accountName": "HDFC SB x3176",
  "recordDate": "2026-08-29T12:00:00.000Z",
  "recordType": "expense",
  "recordState": "cleared",
  "amount": {
    "currencyCode": "INR",
    "value": -3054.5
  },
  "counterParty": "IRCTC Ticketing",
  "category": {
    "id": "5c5c32c9-0082-8000-8000-000000000000",
    "name": "Unknown expense"
  },
  "labels": [
    {
      "id": "2f09784e-661b-4521-b567-f3cf2b238ddd",
      "name": "Trip"
    }
  ],
  "note": "IRCTC Ticketing | via HDFC SB UPI | gmail-sync gm:1a04cc148200bfe2",
  "createdAt": "2026-08-29T09:38:41.844Z",
  "updatedAt": "2026-08-29T09:38:41.844Z"
}
```

**Key fields for deduplication**:
- `id`: Unique record ID (immutable, used as reference)
- `recordDate`: Transaction date (part of primary dedup key)
- `amount.value`: Transaction amount (part of primary dedup key)
- `counterParty`: Merchant/counterparty name (part of primary dedup key)
- `createdAt`: Record creation timestamp (determines "original" vs "duplicate")
- `category.id`, `labels[]`, `note`: Optional fields for extended dedup matching
- `accountId`: Account this record belongs to

**Validation rules**:
- `id` is required and globally unique within records.json
- `recordDate` must be a valid ISO 8601 timestamp
- `amount.value` can be positive (income) or negative (expense); zero amounts are invalid
- `counterParty` must be a non-empty string
- `createdAt` must be a valid ISO 8601 timestamp; record with earliest createdAt in a duplicate group is "original"

---

### Duplicate Group (dedup output)

A grouping of 2+ records that match on the primary dedup key (amount + date + counterparty).

```json
{
  "duplicateKey": "2026-08-29 | -3054.5 | IRCTC Ticketing",
  "records": [
    {
      "id": "65436aec-bbd0-45a3-8687-e9501dc22c06",
      "createdAt": "2026-08-29T09:38:41.844Z",
      "isOriginal": true,
      "counterParty": "IRCTC Ticketing",
      "amount": -3054.5,
      "category": "Unknown expense"
    },
    {
      "id": "f7e8c3d2-a1b9-48f5-9e7c-3d6b9e8c2a1f",
      "createdAt": "2026-08-29T10:15:22.100Z",
      "isOriginal": false,
      "counterParty": "IRCTC Ticketing",
      "amount": -3054.5,
      "category": "Unknown expense"
    }
  ],
  "matchType": "exact",
  "confidence": 1.0
}
```

**Fields**:
- `duplicateKey`: String representation of the matching key (date | amount | counterparty)
- `records[]`: Array of matching Record summaries (enough info for user review)
  - `id`: Record ID
  - `createdAt`: Creation timestamp (determines original order)
  - `isOriginal`: Boolean — true if this is the oldest record (keep by default)
  - `counterParty`, `amount`, `category`: Displayed to user for confirmation
- `matchType`: "exact" (all dedup key fields match) or "uncertain" (some optional fields differ)
- `confidence`: Float 0.0–1.0 indicating certainty (1.0 = definite duplicate, <1.0 = uncertain)

**Validation rules**:
- A duplicate group MUST contain 2+ records
- Records in a group MUST match on (date, amount, counterParty)
- Exactly ONE record per group has `isOriginal: true` (earliest createdAt)
- `matchType` and `confidence` inform the user review (uncertain duplicates may be skipped)

---

### Dedup Decision (user input)

The user's choice for how to handle a duplicate group.

```json
{
  "duplicateKey": "2026-08-29 | -3054.5 | IRCTC Ticketing",
  "action": "keep_first_delete_rest",
  "keepRecordIds": ["65436aec-bbd0-45a3-8687-e9501dc22c06"],
  "deleteRecordIds": ["f7e8c3d2-a1b9-48f5-9e7c-3d6b9e8c2a1f"],
  "reason": "User confirmed — keep oldest, delete newer duplicates"
}
```

**Fields**:
- `duplicateKey`: Reference to the duplicate group
- `action`: One of:
  - `"keep_first_delete_rest"` — Keep the original (oldest), delete all others
  - `"custom"` — User specified which records to keep/delete
  - `"skip"` — Don't process this group
- `keepRecordIds[]`: IDs of records to retain
- `deleteRecordIds[]`: IDs of records to delete
- `reason`: Human-readable explanation (for audit trail)

**Validation rules**:
- At least one record must be kept (no "delete all" action allowed)
- `keepRecordIds` and `deleteRecordIds` must be disjoint
- Union of keep + delete must equal all records in the group

---

### Audit Trail Entry (state.json)

A timestamped record of a dedup operation.

```json
{
  "timestamp": "2026-08-29T10:45:33.500Z",
  "operation": "dedup_executed",
  "deletedRecordIds": [
    "f7e8c3d2-a1b9-48f5-9e7c-3d6b9e8c2a1f",
    "a9b8c7d6-e5f4-3c2b-1a09-f8e7d6c5b4a3"
  ],
  "totalRecordsBefore": 6329,
  "totalRecordsAfter": 6327,
  "backupFile": "records.json.backup.20260829-104533"
}
```

**Fields**:
- `timestamp`: ISO 8601 timestamp of the operation
- `operation`: Type of operation (e.g., "dedup_executed", "dedup_scanned", "dedup_skipped")
- `deletedRecordIds[]`: List of record IDs that were deleted (not the full records, just IDs to maintain least-exposure)
- `totalRecordsBefore`, `totalRecordsAfter`: Count before and after (verify consistency)
- `backupFile`: Name of the backup file created (for recovery)

**Validation rules**:
- `timestamp` must be a valid ISO 8601 timestamp
- `deletedRecordIds` must be non-empty for "executed" operations
- `totalRecordsAfter` must equal `totalRecordsBefore` - len(deletedRecordIds)

---

## Data Relationships

```
records.json
├── Record[0]
├── Record[1]
├── Record[2] (duplicate of Record[0])
└── ...

Dedup Logic:
  Group records by (date, amount, counterparty) → DuplicateGroup[]

User Review:
  DuplicateGroup[] → [DedupDecision]

Execution:
  DedupDecision[] → Delete records → Backup + Update records.json → Append to state.json

state.json (audit trail):
├── AuditEntry (previous dedup from earlier session)
└── AuditEntry (this dedup operation)
```

---

## State Transitions

A record's state in the dedup lifecycle:

```
Original Record (in records.json)
    ↓
[Dedup Scan] → Grouped into DuplicateGroup
    ↓
[User Review] → Included in DedupDecision
    ↓
[Execution] → One of:
    ├─→ Keep (no change, remains in records.json)
    └─→ Delete (removed from records.json, ID recorded in audit trail)
```

---

## Edge Cases & Constraints

### Null/Missing Fields

- **Null counterParty**: Treated as a distinct value; two records with null counterParty and identical date/amount are duplicates.
- **Missing category/labels**: These fields can be absent; don't prevent dedup matching.
- **Null amount**: Invalid; records with null amount are skipped with a warning.

### Ordering & Stability

- **Duplicate groups are ordered**: Records within a group are sorted by createdAt (earliest first) for consistent "original" selection.
- **Idempotence**: Running dedup twice on the same dataset produces the same groups (assuming no new records added). Deleted records don't reappear.

### Scale & Performance

- **Large datasets**: 10,000+ records loaded into memory as a slice; grouped by map. Complexity O(n).
- **Backup atomicity**: Backup written *before* records.json is modified. If write fails, original records.json + backup both exist; recovery is possible.

### Multi-User / Concurrency

- **No locking**: Dedup assumes single-user CLI execution (wallet is a personal finance app). No concurrent dedup writers expected.
- **Rerun safety**: User can re-run dedup on a deduplicated dataset; no duplicates found, no changes made.

---

## Contracts

See [contracts/dedup-record.md](contracts/dedup-record.md) for the service contract (API-like specification of dedup operations, input/output schemas, error conditions).
