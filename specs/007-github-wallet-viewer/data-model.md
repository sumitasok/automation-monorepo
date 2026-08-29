# Data Model: GitHub Wallet Records Viewer

## Transaction Record

Represents a wallet transaction fetched from records.jsonl. All fields are displayed as-is (no transformation).

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string (UUID) | Unique transaction identifier |
| `amount` | object | `{value: number (negative for expense), currencyCode: string}` |
| `recordDate` | string (ISO 8601) | Transaction date/time |
| `counterParty` | string | Merchant or payee name |
| `category` | object | `{name: string, id: string, group: {id, name}, color: string}` |
| `account` | object | `{id: string, name: string, isBankSync: boolean}` |
| `labels` | array | Array of label objects `{id, name, color, archived}` |
| `notes` | string | Transaction memo or description |
| `createdAt` | string (ISO 8601) | When record was created in Wallet |
| `recordState` | string | Transaction state ("cleared", "pending", etc.) |
| `updatedAt` | string (ISO 8601) | Last update timestamp |
| `recordType` | string | "expense", "income", "transfer" |

### Relationships

- A Transaction belongs to one Account
- A Transaction has one Category
- A Transaction can have multiple Labels

## UI State

Represents the current view state of the application.

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `records` | Transaction[] | All loaded records from GitHub |
| `filteredRecords` | Transaction[] | Records after applying search/filters |
| `searchQuery` | string | Counterparty text search (case-insensitive) |
| `dateRange` | [string, string] \| null | [start, end] ISO dates or null for no filter |
| `amountRange` | [number, number] \| null | [min, max] amounts or null for no filter |
| `sortColumn` | string | Current sort field (e.g., "recordDate", "amount") |
| `sortDirection` | "asc" \| "desc" | Sort order |
| `selectedRecordId` | string \| null | ID of record in drill-down view |
| `isLoading` | boolean | Loading indicator (during GitHub fetch) |
| `error` | string \| null | Error message if any |

### Derived Fields

- `filteredRecords` = `records.filter(search).filter(dateRange).filter(amountRange).sort(sortColumn, sortDirection)`

## Storage & Persistence

### Browser Cookies

- **`wallet_github_pat`**: Read-only GitHub PAT
  - Flags: httpOnly (no JS access), secure (HTTPS only), SameSite=Strict
  - Expires: Session (or 30 days if configured)
  - Purpose: Authenticate GitHub API requests

### Session Storage

- **Filter State** (optional, for UX continuity):
  - `searchQuery`, `dateRange`, `amountRange`, `sortColumn`, `sortDirection`
  - Cleared when tab closes
  - Optional: can be preserved in URL query params for shareable filtered views

## Constraints & Validation

- All records are read-only (no editing, deleting, exporting)
- Amount values: can be positive (income) or negative (expense)
- Dates: ISO 8601 format, may be timezone-aware
- Transaction IDs: Must be unique within a session
- Filter ranges: Must handle edge cases (empty results, single record, type mismatches)
