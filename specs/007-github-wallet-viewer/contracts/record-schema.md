# Contract: Transaction Record Schema (JSONL)

## Overview

Transaction records are serialized as newline-delimited JSON (JSONL). Each line is a valid JSON object representing a wallet transaction.

**File Location**: `data/wallet/records.jsonl` in the GitHub repository

**Format**: 
- Line 1: Metadata header (special structure with `fetchedAt`, `recordCount`, `apiTotal`)
- Lines 2+: Transaction record objects (one per line)

## Metadata Header (Line 1)

```json
{
  "fetchedAt": "2026-08-29T15:30:45.123Z",
  "recordCount": 6329,
  "apiTotal": 6329
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `fetchedAt` | ISO 8601 datetime string | Yes | When records were fetched from Wallet API |
| `recordCount` | integer | Yes | Total records in this file |
| `apiTotal` | integer | Yes | Total records in Wallet API (for verification) |

## Transaction Record (Lines 2+)

### Full Schema

```json
{
  "id": "txn_8d4a7f2c-1e9b-4d3f-9a2e-5c8b1d6f3a4e",
  "amount": {
    "value": -2500,
    "currencyCode": "INR"
  },
  "recordDate": "2026-08-28T14:22:00Z",
  "counterParty": "Blinkit",
  "category": {
    "name": "Groceries",
    "id": "cat_g001",
    "group": {
      "id": "cg_001",
      "name": "Shopping"
    },
    "color": "#4CAF50"
  },
  "account": {
    "id": "acc_s001",
    "name": "SBI Checking",
    "isBankSync": true
  },
  "labels": [
    {
      "id": "lbl_a001",
      "name": "weekly-groceries",
      "color": "#FFC107",
      "archived": false
    }
  ],
  "notes": "Weekly grocery shopping",
  "createdAt": "2026-08-28T14:25:00Z",
  "updatedAt": "2026-08-28T14:25:00Z",
  "recordState": "cleared",
  "recordType": "expense"
}
```

### Fields Reference

| Field | Type | Required | Description | Notes |
|-------|------|----------|-------------|-------|
| `id` | string (UUID) | Yes | Unique transaction ID | Format: `txn_<uuid>` |
| `amount.value` | number | Yes | Transaction amount | Negative = expense, positive = income |
| `amount.currencyCode` | string | Yes | Currency code | ISO 4217 (e.g., "INR", "USD") |
| `recordDate` | ISO 8601 datetime | Yes | Transaction date/time | When transaction occurred |
| `counterParty` | string | Yes | Merchant or payee name | Display name, may be normalized |
| `category.name` | string | Yes | Category name | e.g., "Groceries", "Salary" |
| `category.id` | string | Yes | Category unique ID | UUID format |
| `category.group.id` | string | No | Category group ID | Parent grouping |
| `category.group.name` | string | No | Category group name | Parent grouping name |
| `category.color` | string | No | Hex color code | Display color for category |
| `account.id` | string | Yes | Account ID | UUID format |
| `account.name` | string | Yes | Account display name | e.g., "SBI Checking" |
| `account.isBankSync` | boolean | No | Is account bank-synced | true = auto-synced from bank |
| `labels[]` | array | No | Transaction labels/tags | Array of label objects |
| `labels[].id` | string | Yes | Label unique ID | UUID format |
| `labels[].name` | string | Yes | Label name | e.g., "weekly-groceries" |
| `labels[].color` | string | No | Hex color code | Display color for label |
| `labels[].archived` | boolean | No | Is label archived | true = archived, false = active |
| `notes` | string | No | Transaction notes/memo | User-entered description |
| `createdAt` | ISO 8601 datetime | Yes | Record creation timestamp | When added to Wallet |
| `updatedAt` | ISO 8601 datetime | No | Last update timestamp | When record was last modified |
| `recordState` | string | No | Transaction state | Values: "cleared", "pending", "draft", "void" |
| `recordType` | string | No | Transaction type | Values: "expense", "income", "transfer" |

## Parsing Rules

1. **Line 1**: Always a metadata object (skip it when loading for UI display)
2. **Lines 2+**: Each line is a complete Transaction object
3. **Missing fields**: Treat as null/undefined; UI should handle gracefully
4. **Amount sign**: Negative = expense, positive = income (trust the sign)
5. **Dates**: All ISO 8601 UTC; parse and format for local display if needed
6. **Labels**: Can be empty array if transaction has no labels

## Validation

- Each transaction MUST have `id`, `amount`, `recordDate`, `counterParty`, `category`, `account`
- Dates MUST be valid ISO 8601 format
- Amount value MUST be a number (can be negative)
- Category and account objects MUST contain required subfields
- Total record count (lines 2+ count) MUST match metadata `recordCount`

## Size Considerations

- Typical file: 6000+ records, ~2-5MB uncompressed
- Peak record size: ~500 bytes per transaction
- Parsing must handle line-by-line streaming for memory efficiency
