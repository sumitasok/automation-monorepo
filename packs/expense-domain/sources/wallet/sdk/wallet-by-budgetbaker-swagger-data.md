# Wallet (BudgetBaker) — Swagger UI Data (Session Notes)

Source: copied from Swagger UI (Wallet surface).

---

## Banking

Core financial data - transaction records and accounts

### `GET /v1/api/accounts` — Get Accounts

Returns paginated list of financial accounts.

#### Parameters (query)

- `limit` (integer): Maximum number of items to return (1-200, default: 30)
  - Default: `30`
- `offset` (integer): Number of items to skip (default: 0). Used for pagination.
  - Default: `0`
- `agentHints` (boolean): Enable AI agent hints in response. When true, includes structured hints to help AI agents understand the response and take follow-up actions.
  - Default: `false`
- `withTotal` (boolean): When true, the response includes a total count of all matching items (requires an additional database query).
  - Default: `false`
- `id` (string): Filter by ID. Supports comma-separated list for multiple IDs (max 30).
- `name` (string): Filter by name. Requires text prefix.
- `accountType` (string): Filter accounts by account type. Case-insensitive.
  - Available values: `General`, `Cash`, `CurrentAccount`, `CreditCard`, `SavingAccount`, `Bonus`, `Insurance`, `Investment`, `Loan`, `Mortgage`, `Overdraft`
- `currencyCode` (string): Filter by currency code (ISO 4217). Exact match, case-insensitive.
- `createdAt` (string): Filter by creation timestamp. Requires range prefix.
- `updatedAt` (string): Filter by last sync timestamp (when entity was last updated in the API database). Requires range prefix.
- `archived` (boolean): Filter by archived status. `true`=archived only, `false`=active only, omit for all.
- `budgetId` (string): Filter items tracked by a specific budget.
- `sortBy` (string): Sort field with +/- prefix for ASC/DESC. Allowed: `createdAt`, `updatedAt`. Example: `-createdAt`

#### Responses

##### `200` — List of accounts (`application/json`)

Example value:

```json
{
  "limit": 0,
  "nextOffset": 0,
  "offset": 0,
  "total": 0,
  "accounts": [
    {
      "accountType": "CurrentAccount",
      "archived": false,
      "balance": {
        "balanceMode": "standard",
        "currencyCode": "USD",
        "currentBalance": 3343.75,
        "formula": "initial + totalIncomes - totalExpenses",
        "initial": 1000,
        "rawCurrentBalance": 3343.75
      },
      "bankAccountNumber": "CZ65 0800 0000 1920 0014 5399",
      "color": "#d32f2f",
      "createdAt": "2021-03-24T09:28:41.200Z",
      "currencyCode": "USD",
      "excludeFromStats": false,
      "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
      "isBankSync": false,
      "isInvestmentAccount": false,
      "name": "My Checking Account",
      "recordStats": {
        "createdAt": {
          "max": "2025-03-20T14:22:00.000Z",
          "min": "2024-01-15T10:30:00.000Z"
        },
        "lastUpdatedAt": "2025-03-20T14:22:31.456Z",
        "recordCount": 42,
        "recordDate": {
          "max": "2025-03-20T00:00:00.000Z",
          "min": "2024-01-15T00:00:00.000Z"
        },
        "totalExpenses": 12890.75,
        "totalIncomes": 15234.5
      },
      "updatedAt": "2021-03-24T09:28:41.200Z"
    }
  ],
  "agentHints": [
    {
      "action": {
        "url": "string"
      },
      "data": null,
      "severity": "info",
      "text": "string",
      "type": "pagination.has_more"
    }
  ]
}
```

Headers:

- `X-Last-Data-Change-At` (string): Timestamp of last data modification (ISO 8601 UTC with millisecond precision)
  - Example: `2024-01-28T14:23:45.123Z`
- `X-Last-Data-Change-Rev` (string): Revision counter for change detection. Compare values to detect data changes between requests.
  - Example: `r1234`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`
- `X-Sync-In-Progress` (string): If true, response data is valid but background sync is running — more changes may follow shortly.
  - Example: `false`

##### `400` — Bad Request (`application/json`)

```json
{
  "error": "invalid parameter"
}
```

##### `401` — Unauthorized (`application/json`)

```json
{
  "error": "Invalid or expired token"
}
```

##### `403` — Forbidden (`text/plain`)

```text
Forbidden: this token belongs to the Board app and cannot be used on the wallet surface. Use the board endpoints, or authenticate against wallet.
```

##### `429` — Rate limit exceeded (`application/json`)

```json
{
  "error": "Rate limit exceeded. Please try again later."
}
```

Headers:

- `Retry-After` (integer): Seconds to wait before retrying
  - Example: `60`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`

##### `500` — Internal server error (`application/json`)

```json
{
  "error": "Internal Error"
}
```

---

### `POST /v1/api/accounts` — Create Account

Create a new financial account.

#### Parameters

No parameters.

#### Request body (`application/json`)

Example value:

```json
{
  "accountType": "General",
  "currencyCode": "EUR",
  "initialBalance": 0,
  "name": "My new account"
}
```

#### Responses

##### `201` — Account created (`application/json`)

Example value:

```json
{
  "account": {
    "accountType": "CurrentAccount",
    "archived": false,
    "balance": {
      "balanceMode": "standard",
      "currencyCode": "USD",
      "currentBalance": 3343.75,
      "formula": "initial + totalIncomes - totalExpenses",
      "initial": 1000,
      "rawCurrentBalance": 3343.75
    },
    "bankAccountNumber": "CZ65 0800 0000 1920 0014 5399",
    "color": "#d32f2f",
    "createdAt": "2021-03-24T09:28:41.200Z",
    "currencyCode": "USD",
    "excludeFromStats": false,
    "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
    "isBankSync": false,
    "isInvestmentAccount": false,
    "name": "My Checking Account",
    "recordStats": {
      "createdAt": {
        "max": "2025-03-20T14:22:00.000Z",
        "min": "2024-01-15T10:30:00.000Z"
      },
      "lastUpdatedAt": "2025-03-20T14:22:31.456Z",
      "recordCount": 42,
      "recordDate": {
        "max": "2025-03-20T00:00:00.000Z",
        "min": "2024-01-15T00:00:00.000Z"
      },
      "totalExpenses": 12890.75,
      "totalIncomes": 15234.5
    },
    "updatedAt": "2021-03-24T09:28:41.200Z"
  },
  "agentHints": [
    {
      "action": {
        "url": "string"
      },
      "data": null,
      "severity": "info",
      "text": "string",
      "type": "pagination.has_more"
    }
  ],
  "summary": {
    "clientErrors": 0,
    "documentsWritten": 0,
    "serverErrors": 0,
    "succeeded": 0,
    "total": 0
  }
}
```

##### `400` — Bad Request (`application/json`)

```json
{
  "error": "invalid parameter"
}
```

##### `401` — Unauthorized (`application/json`)

```json
{
  "error": "Invalid or expired token"
}
```

##### `403` — Forbidden (`text/plain`)

```text
Forbidden: this token belongs to the Board app and cannot be used on the wallet surface. Use the board endpoints, or authenticate against wallet.
```

##### `429` — Rate limit exceeded (`application/json`)

```json
{
  "error": "Rate limit exceeded. Please try again later."
}
```

Headers:

- `Retry-After` (integer): Seconds to wait before retrying
  - Example: `60`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`

##### `500` — Internal server error (`application/json`)

```json
{
  "error": "Internal Error"
}
```

---

### `PATCH /v1/api/accounts` — Update Accounts

Batch update account fields. Maximum 10 accounts per request.

#### Parameters (query)

- `returnData` (boolean): When false, omit the created/updated object body from each per-item result (results still carry id, success, and error).
  - Default: `true`

#### Request body (`application/json`)

Example value:

```json
[
  {
    "archived": false,
    "excludeFromStats": false,
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "initialBalance": 0,
    "name": "My Patched Account"
  }
]
```

#### Responses

##### `200` — All updates succeeded (`application/json`)

Example value:

```json
{
  "agentHints": [
    {
      "action": {
        "url": "string"
      },
      "data": null,
      "severity": "info",
      "text": "string",
      "type": "pagination.has_more"
    }
  ],
  "results": [
    {
      "account": {
        "accountType": "CurrentAccount",
        "archived": false,
        "balance": {
          "balanceMode": "standard",
          "currencyCode": "USD",
          "currentBalance": 3343.75,
          "formula": "initial + totalIncomes - totalExpenses",
          "initial": 1000,
          "rawCurrentBalance": 3343.75
        },
        "bankAccountNumber": "CZ65 0800 0000 1920 0014 5399",
        "color": "#d32f2f",
        "createdAt": "2021-03-24T09:28:41.200Z",
        "currencyCode": "USD",
        "excludeFromStats": false,
        "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
        "isBankSync": false,
        "isInvestmentAccount": false,
        "name": "My Checking Account",
        "recordStats": {
          "createdAt": {
            "max": "2025-03-20T14:22:00.000Z",
            "min": "2024-01-15T10:30:00.000Z"
          },
          "lastUpdatedAt": "2025-03-20T14:22:31.456Z",
          "recordCount": 42,
          "recordDate": {
            "max": "2025-03-20T00:00:00.000Z",
            "min": "2024-01-15T00:00:00.000Z"
          },
          "totalExpenses": 12890.75,
          "totalIncomes": 15234.5
        },
        "updatedAt": "2021-03-24T09:28:41.200Z"
      },
      "error": "string",
      "errorType": "client_error",
      "fields": [
        "string"
      ],
      "id": "string",
      "inputIndex": 0,
      "success": true
    }
  ],
  "summary": {
    "clientErrors": 0,
    "documentsWritten": 0,
    "serverErrors": 0,
    "succeeded": 0,
    "total": 0
  }
}
```

Headers:

- `X-Last-Data-Change-At` (string): Timestamp of last data modification (ISO 8601 UTC with millisecond precision)
  - Example: `2024-01-28T14:23:45.123Z`
- `X-Last-Data-Change-Rev` (string): Revision counter for change detection. Compare values to detect data changes between requests.
  - Example: `r1234`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`
- `X-Sync-In-Progress` (string): If true, response data is valid but background sync is running — more changes may follow shortly.
  - Example: `false`

##### `207` — Mixed results (`application/json`)

Same batch response schema as `200`; check `results[]` for per-item details.

##### `400` — All items failed with client errors (`application/json`)

Same batch response schema as `200`; check `results[]` for per-item error details.

##### `401` — Unauthorized (`application/json`)

```json
{
  "error": "Invalid or expired token"
}
```

##### `403` — Forbidden (`text/plain`)

```text
Forbidden: this token belongs to the Board app and cannot be used on the wallet surface. Use the board endpoints, or authenticate against wallet.
```

##### `429` — Rate limit exceeded (`application/json`)

```json
{
  "error": "Rate limit exceeded. Please try again later."
}
```

Headers:

- `Retry-After` (integer): Seconds to wait before retrying
  - Example: `60`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`

##### `500` — All items failed with server errors (`application/json`)

Same batch response schema as `200`; retry later.

---

### `GET /v1/api/records` — Get Financial Records

Returns paginated list of financial transaction records.

Notes:

- Use `recordDate` filter with range operators to scope results. If omitted, a default 3‑month window is applied (see `appliedRecordDateFilters` in the response).
- Provide any single bound (e.g. `gte.2020-01-01`) to override the default.
- ID-based lookups (`?id=`) bypass the default window entirely.

#### Parameters (query)

- `id` (string): Filter by ID. Supports comma-separated list for multiple IDs (max 30).
- `accountId` (string): Filter by account ID(s), comma-separated (max 10). When omitted, returns records from all accounts. Use `/accounts` to find valid account IDs.
- `recordDate` (string): Filter by transaction date. Requires range prefix (e.g. `gte.2024-01-01`, `lt.2024-04-01`). If omitted, a default 3-month window is applied automatically. The actual applied filter is returned in `appliedRecordDateFilters`.
- `limit` (integer): Maximum number of items to return (1-200, default: 30)
  - Default: `30`
- `offset` (integer): Number of items to skip (default: 0). Used for pagination.
  - Default: `0`
- `agentHints` (boolean): Enable AI agent hints in response.
  - Default: `false`
- `withTotal` (boolean): When true, response includes a total count of all matching items.
  - Default: `false`
- `categoryId` (string): Filter by category ID. Comma-separated for multiple values. Special value `unknown` expands to IDs of 3 categories: Unknown income, Unknown expense, and Uncategorized. Max 10 values.
- `labelId` (string): Filter by label ID — returns records that have this label.
- `note` (string): Filter by note. Requires text prefix.
- `counterParty` (string): Filter records by counter party (payee/payer). Requires text prefix.
- `amount` (string): Filter by transaction amount. Requires range prefix. Applied to `amount.value` (decimal format, e.g., `100.50`).
- `createdAt` (string): Filter by creation timestamp. Requires range prefix.
- `updatedAt` (string): Filter by last sync timestamp (when entity was last updated in the API database). Requires range prefix.
- `recordType` (string): Filter by record type. Matches on the amount sign (expense = amount < 0, income = amount > 0); a legacy record with an amount of exactly 0 matches the type it was stored with.
  - Available values: `expense`, `income`
- `isTransfer` (boolean): Filter by transfer state: true = only transfer records (paired or unpaired), false = only non-transfer records, omit = all. Independent of `recordType`.
- `transferId` (string): Filter by transfer identity, as returned in `transfer.transferId`. Returns every record sharing it.
- `recordState` (string): Filter by record state. Comma-separated for multiple values (e.g. `reconciled,uncleared`). Valid values: `reconciled`, `cleared`, `uncleared`.
- `source` (string): Filter by record creation source. Comma-separated for multiple values (e.g. `mcp,rest`). Valid values: `android`, `ios`, `web`, `rest`, `mcp`, `backend`, `missing`.
- `sortBy` (string): Sort results by field. Prefix with `+` for ascending, `-` for descending. Supported fields: `recordDate`, `amount`, `createdAt`, `updatedAt`.
  - Default: `-recordDate` (newest first)
- `convertTo` (string): Convert record amounts to this currency. Accepts ISO-4217 code (e.g. `EUR`), `base` (profile currency), or `account` (record's account currency). When set, each record includes a `convertedAmount` field.

#### Responses

##### `200` — List of records (`application/json`)

Example value:

```json
{
  "limit": 0,
  "nextOffset": 0,
  "offset": 0,
  "total": 0,
  "agentHints": [
    {
      "action": {
        "url": "string"
      },
      "data": null,
      "severity": "info",
      "text": "string",
      "type": "pagination.has_more"
    }
  ],
  "appliedRecordDateFilters": [
    "gte.2026-02-22T00:00:00Z",
    "lt.2026-05-23T00:00:00Z"
  ],
  "records": [
    {
      "accountId": "-Account_abc123",
      "accountIsBankSync": true,
      "accountName": "Checking Account",
      "amount": {
        "currencyCode": "USD",
        "value": -42.5
      },
      "category": {
        "color": "#26c6da",
        "group": {
          "id": "food_and_drinks",
          "name": "Food & Drinks"
        },
        "id": "04f8dea1-33dc-411a-b2eb-b348c3168fa3",
        "name": "Health & beauty"
      },
      "convertedAmount": {
        "conversionPair": "EUR:CZK",
        "currencyCode": "CZK",
        "error": "string",
        "ratio": 25.42,
        "value": 2567.89
      },
      "counterParty": "string",
      "createdAt": "2021-03-24T09:28:41.200Z",
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "labels": [
        {
          "archived": false,
          "color": "#d32f2f",
          "id": "8f4e8961-3f0e-4c1e-b7a1-3d8f9c5e2a6b",
          "name": "Important"
        }
      ],
      "note": "string",
      "photos": [
        {
          "createdAt": "2021-03-24T09:28:41.200Z",
          "temporaryUrl": "string"
        }
      ],
      "place": {
        "address": "string",
        "latitude": 0.1,
        "longitude": 0.1,
        "name": "string"
      },
      "recordDate": "2021-03-24T09:28:41.200Z",
      "recordState": "cleared",
      "recordType": "expense",
      "source": "android",
      "transfer": {
        "mirrorRecord": {
          "accountId": "string",
          "amount": {
            "currencyCode": "USD",
            "value": 42.5
          },
          "counterParty": "string",
          "id": "string",
          "note": "string"
        },
        "transferId": "string",
        "type": "string"
      },
      "updatedAt": "2021-03-24T09:28:41.200Z"
    }
  ]
}
```

Headers:

- `X-Last-Data-Change-At` (string): Timestamp of last data modification (ISO 8601 UTC with millisecond precision)
  - Example: `2024-01-28T14:23:45.123Z`
- `X-Last-Data-Change-Rev` (string): Revision counter for change detection.
  - Example: `r1234`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`
- `X-Sync-In-Progress` (string): If true, response data is valid but background sync is running.
  - Example: `false`

##### `400` — Bad Request (`application/json`)

```json
{
  "error": "invalid parameter"
}
```

##### `401` — Unauthorized (`application/json`)

```json
{
  "error": "Invalid or expired token"
}
```

##### `403` — Forbidden (`text/plain`)

```text
Forbidden: this token belongs to the Board app and cannot be used on the wallet surface. Use the board endpoints, or authenticate against wallet.
```

##### `429` — Rate limit exceeded (`application/json`)

```json
{
  "error": "Rate limit exceeded. Please try again later."
}
```

Headers:

- `Retry-After` (integer): Seconds to wait before retrying
  - Example: `60`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`

##### `500` — Internal server error (`application/json`)

```json
{
  "error": "Internal Error"
}
```

---

### `PATCH /v1/api/records` — Update Financial Records

Batch update record fields (category, labels, note, amount, etc.). Maximum 10 records per request.

#### Parameters (query)

- `validation` (string): Validation mode.
  - `strict`: validate all items first; if any `client_error`, return `400` with full results (no writes).
  - Default: `lenient` (process valid items, skip invalid)
  - Available values: `strict`
- `returnData` (boolean): When false, omit the created/updated object body from each per-item result (results still carry id, success, and error).
  - Default: `true`

#### Request body (`application/json`)

Example value:

```json
[
  {
    "categoryId": "5c5c07d0-0014-8000-8000-000000000000",
    "counterParty": "Updated merchant",
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "note": "Updated note"
  }
]
```

#### Responses

##### `200` — All updates succeeded (`application/json`)

Example value:

```json
{
  "agentHints": [
    {
      "action": {
        "url": "string"
      },
      "data": null,
      "severity": "info",
      "text": "string",
      "type": "pagination.has_more"
    }
  ],
  "results": [
    {
      "error": "string",
      "errorType": "client_error",
      "fields": [
        "string"
      ],
      "id": "string",
      "inputIndex": 0,
      "record": {
        "accountId": "-Account_abc123",
        "accountIsBankSync": true,
        "accountName": "Checking Account",
        "amount": {
          "currencyCode": "USD",
          "value": -42.5
        },
        "category": {
          "color": "#26c6da",
          "group": {
            "id": "food_and_drinks",
            "name": "Food & Drinks"
          },
          "id": "04f8dea1-33dc-411a-b2eb-b348c3168fa3",
          "name": "Health & beauty"
        },
        "convertedAmount": {
          "conversionPair": "EUR:CZK",
          "currencyCode": "CZK",
          "error": "string",
          "ratio": 25.42,
          "value": 2567.89
        },
        "counterParty": "string",
        "createdAt": "2021-03-24T09:28:41.200Z",
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "labels": [
          {
            "archived": false,
            "color": "#d32f2f",
            "id": "8f4e8961-3f0e-4c1e-b7a1-3d8f9c5e2a6b",
            "name": "Important"
          }
        ],
        "note": "string",
        "photos": [
          {
            "createdAt": "2021-03-24T09:28:41.200Z",
            "temporaryUrl": "string"
          }
        ],
        "place": {
          "address": "string",
          "latitude": 0.1,
          "longitude": 0.1,
          "name": "string"
        },
        "recordDate": "2021-03-24T09:28:41.200Z",
        "recordState": "cleared",
        "recordType": "expense",
        "source": "android",
        "transfer": {
          "mirrorRecord": {
            "accountId": "string",
            "amount": {
              "currencyCode": "USD",
              "value": 42.5
            },
            "counterParty": "string",
            "id": "string",
            "note": "string"
          },
          "transferId": "string",
          "type": "string"
        },
        "updatedAt": "2021-03-24T09:28:41.200Z"
      },
      "success": true
    }
  ],
  "summary": {
    "clientErrors": 0,
    "documentsWritten": 0,
    "serverErrors": 0,
    "succeeded": 0,
    "total": 0
  }
}
```

Headers:

- `X-Last-Data-Change-At` (string): Timestamp of last data modification (ISO 8601 UTC with millisecond precision)
  - Example: `2024-01-28T14:23:45.123Z`
- `X-Last-Data-Change-Rev` (string): Revision counter for change detection.
  - Example: `r1234`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`
- `X-Sync-In-Progress` (string): If true, response data is valid but background sync is running.
  - Example: `false`

##### `207` — Mixed results (`application/json`)

Same batch response schema as `200`; check `results[]` for per-item details.

##### `400` — All items failed with client errors (`application/json`)

Same batch response schema as `200`; check `results[]` for per-item error details.

##### `401` — Unauthorized (`application/json`)

```json
{
  "error": "Invalid or expired token"
}
```

##### `403` — Forbidden (`text/plain`)

```text
Forbidden: this token belongs to the Board app and cannot be used on the wallet surface. Use the board endpoints, or authenticate against wallet.
```

##### `429` — Rate limit exceeded (`application/json`)

```json
{
  "error": "Rate limit exceeded. Please try again later."
}
```

Headers:

- `Retry-After` (integer): Seconds to wait before retrying
  - Example: `60`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`

##### `500` — All items failed with server errors (`application/json`)

Same batch response schema as `200`; retry later.

---

### `POST /v1/api/records` — Create Financial Records

Create one or more transaction records. Maximum 50 records per request.

By default each result includes the full created record; pass `returnData=false` to omit the record bodies (each result still carries `inputIndex`, `id`, `success`, and `error`).

#### Parameters (query)

- `returnData` (boolean): When false, omit the created record body from each result (keeps `inputIndex`, `id`, `success`, `error`).
  - Default: `true`

#### Request body (`application/json`)

Example value:

```json
[
  {
    "accountId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "amount": {
      "value": -42.5
    },
    "categoryId": "5c5c07d0-0014-8000-8000-000000000000",
    "counterParty": "Downtown Bistro",
    "note": "Lunch at restaurant",
    "recordDate": "2025-03-15T14:30:00Z"
  }
]
```

#### Responses

##### `200` — All records created successfully (`application/json`)

Example value:

```json
{
  "results": [
    {
      "createdMirrorRecordId": "string",
      "error": "string",
      "errorType": "client_error",
      "fields": [
        "string"
      ],
      "id": "string",
      "inputIndex": 0,
      "record": {
        "accountId": "-Account_abc123",
        "accountIsBankSync": true,
        "accountName": "Checking Account",
        "amount": {
          "currencyCode": "USD",
          "value": -42.5
        },
        "category": {
          "color": "#26c6da",
          "group": {
            "id": "food_and_drinks",
            "name": "Food & Drinks"
          },
          "id": "04f8dea1-33dc-411a-b2eb-b348c3168fa3",
          "name": "Health & beauty"
        },
        "convertedAmount": {
          "conversionPair": "EUR:CZK",
          "currencyCode": "CZK",
          "error": "string",
          "ratio": 25.42,
          "value": 2567.89
        },
        "counterParty": "string",
        "createdAt": "2021-03-24T09:28:41.200Z",
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "labels": [
          {
            "archived": false,
            "color": "#d32f2f",
            "id": "8f4e8961-3f0e-4c1e-b7a1-3d8f9c5e2a6b",
            "name": "Important"
          }
        ],
        "note": "string",
        "photos": [
          {
            "createdAt": "2021-03-24T09:28:41.200Z",
            "temporaryUrl": "string"
          }
        ],
        "place": {
          "address": "string",
          "latitude": 0.1,
          "longitude": 0.1,
          "name": "string"
        },
        "recordDate": "2021-03-24T09:28:41.200Z",
        "recordState": "cleared",
        "recordType": "expense",
        "source": "android",
        "transfer": {
          "mirrorRecord": {
            "accountId": "string",
            "amount": {
              "currencyCode": "USD",
              "value": 42.5
            },
            "counterParty": "string",
            "id": "string",
            "note": "string"
          },
          "transferId": "string",
          "type": "string"
        },
        "updatedAt": "2021-03-24T09:28:41.200Z"
      },
      "success": true
    }
  ],
  "summary": {
    "clientErrors": 0,
    "documentsWritten": 0,
    "serverErrors": 0,
    "succeeded": 0,
    "total": 0
  }
}
```

Headers:

- `X-Last-Data-Change-At` (string): Timestamp of last data modification (ISO 8601 UTC with millisecond precision)
  - Example: `2024-01-28T14:23:45.123Z`
- `X-Last-Data-Change-Rev` (string): Revision counter for change detection.
  - Example: `r1234`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`
- `X-Sync-In-Progress` (string): If true, response data is valid but background sync is running.
  - Example: `false`

##### `207` — Mixed results (`application/json`)

Same batch response schema as `200`; check `results[]` for per-item details.

##### `400` — All items failed with client errors (`application/json`)

Same batch response schema as `200`; check `results[]` for per-item error details.

##### `401` — Unauthorized (`application/json`)

```json
{
  "error": "Invalid or expired token"
}
```

##### `403` — Forbidden (`text/plain`)

```text
Forbidden: this token belongs to the Board app and cannot be used on the wallet surface. Use the board endpoints, or authenticate against wallet.
```

##### `429` — Rate limit exceeded (`application/json`)

```json
{
  "error": "Rate limit exceeded. Please try again later."
}
```

Headers:

- `Retry-After` (integer): Seconds to wait before retrying
  - Example: `60`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`

##### `500` — All items failed with server errors (`application/json`)

Same batch response schema as `200`; retry later.

---

## Classification

Data organization - categories, labels, and record rules

### `GET /v1/api/categories` — Get Categories

Returns paginated list of categories.

#### Parameters (query)

- `limit` (integer): Maximum number of items to return (1-200, default: 30)
  - Default: `30`
- `offset` (integer): Number of items to skip (default: 0). Used for pagination.
  - Default: `0`
- `agentHints` (boolean): Enable AI agent hints in response.
  - Default: `false`
- `withTotal` (boolean): When true, the response includes a total count of all matching items (requires an additional database query).
  - Default: `false`
- `id` (string): Filter by ID. Supports comma-separated list for multiple IDs (max 30).
- `name` (string): Filter by name. Requires text prefix.
- `createdAt` (string): Filter by creation timestamp. Requires range prefix.
- `updatedAt` (string): Filter by last sync timestamp (when entity was last updated in the API database). Requires range prefix.
- `customCategory` (boolean): Filter by category type. `true`=custom subcategories only, `false`=system categories only.
- `cardinality` (string): Filter by the category's spending nature, matching the value shown in the response.
  - A category with no cardinality of its own inherits its base category's value, so filtering by an inherited value also returns categories that merely inherit it.
  - `cardinality=none` returns categories explicitly set to none plus any with no value to inherit.
  - Available values: `none`, `must`, `need`, `want`
- `archived` (boolean): Filter by archived status. `true`=archived only, `false`=active only, omit for all.
- `budgetId` (string): Filter items tracked by a specific budget.
- `sortBy` (string): Sort field with +/- prefix for ASC/DESC. Allowed: `createdAt`, `updatedAt`. Example: `-createdAt`

#### Responses

##### `200` — List of categories (`application/json`)

Example value:

```json
{
  "limit": 0,
  "nextOffset": 0,
  "offset": 0,
  "total": 0,
  "agentHints": [
    {
      "action": {
        "url": "string"
      },
      "data": null,
      "severity": "info",
      "text": "string",
      "type": "pagination.has_more"
    }
  ],
  "categories": [
    {
      "archived": true,
      "cardinality": "none",
      "color": "#26c6da",
      "createdAt": "2021-03-24T09:28:41.200Z",
      "customCategory": false,
      "customName": true,
      "enabled": true,
      "group": {
        "id": "food_and_drinks",
        "name": "Food & Drinks"
      },
      "id": "04f8dea1-33dc-411a-b2eb-b348c3168fa3",
      "name": "string",
      "parentId": "string",
      "parentName": "Groceries",
      "systemId": "food_and_drinks__groceries",
      "updatedAt": "2021-03-24T09:28:41.200Z"
    }
  ]
}
```

Headers:

- `X-Last-Data-Change-At` (string): Timestamp of last data modification (ISO 8601 UTC with millisecond precision)
  - Example: `2024-01-28T14:23:45.123Z`
- `X-Last-Data-Change-Rev` (string): Revision counter for change detection.
  - Example: `r1234`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`
- `X-Sync-In-Progress` (string): If true, response data is valid but background sync is running.
  - Example: `false`

##### `400` — Bad Request (`application/json`)

```json
{
  "error": "invalid parameter"
}
```

##### `401` — Unauthorized (`application/json`)

```json
{
  "error": "Invalid or expired token"
}
```

##### `403` — Forbidden (`text/plain`)

```text
Forbidden: this token belongs to the Board app and cannot be used on the wallet surface. Use the board endpoints, or authenticate against wallet.
```

##### `429` — Rate limit exceeded (`application/json`)

```json
{
  "error": "Rate limit exceeded. Please try again later."
}
```

Headers:

- `Retry-After` (integer): Seconds to wait before retrying
  - Example: `60`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`

##### `500` — Internal server error (`application/json`)

```json
{
  "error": "Internal Error"
}
```

---

### `PATCH /v1/api/categories` — Update Categories

Batch update category fields (name, color, cardinality). Maximum 10 categories per request.

#### Parameters (query)

- `returnData` (boolean): When false, omit the created/updated object body from each per-item result (results still carry id, success, and error).
  - Default: `true`

#### Request body (`application/json`)

Example value:

```json
[
  {
    "color": "blue",
    "id": "5c5c07d0-0014-8000-8000-000000000000",
    "name": "My Custom Category Name"
  }
]
```

#### Responses

##### `200` — All updates succeeded (`application/json`)

Example value:

```json
{
  "agentHints": [
    {
      "action": {
        "url": "string"
      },
      "data": null,
      "severity": "info",
      "text": "string",
      "type": "pagination.has_more"
    }
  ],
  "results": [
    {
      "category": {
        "archived": true,
        "cardinality": "none",
        "color": "#26c6da",
        "createdAt": "2021-03-24T09:28:41.200Z",
        "customCategory": false,
        "customName": true,
        "enabled": true,
        "group": {
          "id": "food_and_drinks",
          "name": "Food & Drinks"
        },
        "id": "04f8dea1-33dc-411a-b2eb-b348c3168fa3",
        "name": "string",
        "parentId": "string",
        "parentName": "Groceries",
        "systemId": "food_and_drinks__groceries",
        "updatedAt": "2021-03-24T09:28:41.200Z"
      },
      "error": "string",
      "errorType": "client_error",
      "fields": [
        "string"
      ],
      "id": "string",
      "inputIndex": 0,
      "success": true
    }
  ],
  "summary": {
    "clientErrors": 0,
    "documentsWritten": 0,
    "serverErrors": 0,
    "succeeded": 0,
    "total": 0
  }
}
```

Headers:

- `X-Last-Data-Change-At` (string): Timestamp of last data modification (ISO 8601 UTC with millisecond precision)
  - Example: `2024-01-28T14:23:45.123Z`
- `X-Last-Data-Change-Rev` (string): Revision counter for change detection.
  - Example: `r1234`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`
- `X-Sync-In-Progress` (string): If true, response data is valid but background sync is running.
  - Example: `false`

##### `207` — Mixed results (`application/json`)

Same batch response schema as `200`; check `results[]` for per-item details.

##### `400` — All items failed with client errors (`application/json`)

Same batch response schema as `200`; check `results[]` for per-item error details.

##### `401` — Unauthorized (`application/json`)

```json
{
  "error": "Invalid or expired token"
}
```

##### `403` — Forbidden (`text/plain`)

```text
Forbidden: this token belongs to the Board app and cannot be used on the wallet surface. Use the board endpoints, or authenticate against wallet.
```

##### `429` — Rate limit exceeded (`application/json`)

```json
{
  "error": "Rate limit exceeded. Please try again later."
}
```

Headers:

- `Retry-After` (integer): Seconds to wait before retrying
  - Example: `60`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`

##### `500` — All items failed with server errors (`application/json`)

Same batch response schema as `200`; retry later.

---

### `POST /v1/api/categories/custom` — Create Subcategory

Create a custom subcategory under a system category.

#### Parameters

No parameters.

#### Request body (`application/json`)

Example value:

```json
{
  "color": "red",
  "name": "Weekend Dining",
  "parentId": "5c5c07d0-0014-8000-8000-000000000000"
}
```

#### Responses

##### `201` — Subcategory created (`application/json`)

Example value:

```json
{
  "agentHints": [
    {
      "action": {
        "url": "string"
      },
      "data": null,
      "severity": "info",
      "text": "string",
      "type": "pagination.has_more"
    }
  ],
  "category": {
    "archived": true,
    "cardinality": "none",
    "color": "#26c6da",
    "createdAt": "2021-03-24T09:28:41.200Z",
    "customCategory": false,
    "customName": true,
    "enabled": true,
    "group": {
      "id": "food_and_drinks",
      "name": "Food & Drinks"
    },
    "id": "04f8dea1-33dc-411a-b2eb-b348c3168fa3",
    "name": "string",
    "parentId": "string",
    "parentName": "Groceries",
    "systemId": "food_and_drinks__groceries",
    "updatedAt": "2021-03-24T09:28:41.200Z"
  },
  "summary": {
    "clientErrors": 0,
    "documentsWritten": 0,
    "serverErrors": 0,
    "succeeded": 0,
    "total": 0
  }
}
```

##### `400` — Bad Request (`application/json`)

```json
{
  "error": "invalid parameter"
}
```

##### `401` — Unauthorized (`application/json`)

```json
{
  "error": "Invalid or expired token"
}
```

##### `403` — Forbidden (`text/plain`)

```text
Forbidden: this token belongs to the Board app and cannot be used on the wallet surface. Use the board endpoints, or authenticate against wallet.
```

##### `429` — Rate limit exceeded (`application/json`)

```json
{
  "error": "Rate limit exceeded. Please try again later."
}
```

Headers:

- `Retry-After` (integer): Seconds to wait before retrying
  - Example: `60`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`

##### `500` — Internal server error (`application/json`)

```json
{
  "error": "Internal Error"
}
```

---

### `GET /v1/api/labels` — Get Labels

Returns paginated list of labels/hashtags. Default order: name ascending.

#### Parameters (query)

- `limit` (integer): Maximum number of items to return (1-200, default: 30)
  - Default: `30`
- `offset` (integer): Number of items to skip (default: 0). Used for pagination.
  - Default: `0`
- `agentHints` (boolean): Enable AI agent hints in response.
  - Default: `false`
- `withTotal` (boolean): When true, the response includes a total count of all matching items (requires an additional database query).
  - Default: `false`
- `id` (string): Filter by ID. Supports comma-separated list for multiple IDs (max 30).
- `name` (string): Filter by name. Requires text prefix.
- `createdAt` (string): Filter by creation timestamp. Requires range prefix.
- `updatedAt` (string): Filter by last sync timestamp (when entity was last updated in the API database). Requires range prefix.
- `archived` (boolean): Filter by archived status. true=archived only, false=active only, omit for all.
- `recordId` (string): Filter labels used by a specific record.
- `budgetId` (string): Filter items tracked by a specific budget.
- `standingOrderId` (string): Filter labels used by a specific standing order.
- `recordRuleId` (string): Filter labels used by a specific record rule.
- `sortBy` (string): Sort field with +/- prefix for ASC/DESC. Allowed: `createdAt`, `updatedAt`. Example: `-createdAt`

#### Responses

##### `200` — List of labels (`application/json`)

Example value:

```json
{
  "limit": 0,
  "nextOffset": 0,
  "offset": 0,
  "total": 0,
  "agentHints": [
    {
      "action": {
        "url": "string"
      },
      "data": null,
      "severity": "info",
      "text": "string",
      "type": "pagination.has_more"
    }
  ],
  "labels": [
    {
      "archived": false,
      "color": "#d32f2f",
      "createdAt": "2021-03-24T09:28:41.200Z",
      "id": "8f4e8961-3f0e-4c1e-b7a1-3d8f9c5e2a6b",
      "name": "string",
      "updatedAt": "2021-03-24T09:28:41.200Z"
    }
  ]
}
```

Headers:

- `X-Last-Data-Change-At` (string): Timestamp of last data modification (ISO 8601 UTC with millisecond precision)
  - Example: `2024-01-28T14:23:45.123Z`
- `X-Last-Data-Change-Rev` (string): Revision counter for change detection.
  - Example: `r1234`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`
- `X-Sync-In-Progress` (string): If true, response data is valid but background sync is running.
  - Example: `false`

##### `400` — Bad Request (`application/json`)

```json
{
  "error": "invalid parameter"
}
```

##### `401` — Unauthorized (`application/json`)

```json
{
  "error": "Invalid or expired token"
}
```

##### `403` — Forbidden (`text/plain`)

```text
Forbidden: this token belongs to the Board app and cannot be used on the wallet surface. Use the board endpoints, or authenticate against wallet.
```

##### `429` — Rate limit exceeded (`application/json`)

```json
{
  "error": "Rate limit exceeded. Please try again later."
}
```

Headers:

- `Retry-After` (integer): Seconds to wait before retrying
  - Example: `60`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`

##### `500` — Internal server error (`application/json`)

```json
{
  "error": "Internal Error"
}
```

---

### `PATCH /v1/api/labels` — Update Labels

Batch update label fields. Maximum 10 labels per request.

#### Parameters (query)

- `returnData` (boolean): When false, omit the created/updated object body from each per-item result (results still carry id, success, and error).
  - Default: `true`

#### Request body (`application/json`)

Example value:

```json
[
  {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "name": "My Patched Label"
  }
]
```

#### Responses

##### `200` — All updates succeeded (`application/json`)

Example value:

```json
{
  "agentHints": [
    {
      "action": {
        "url": "string"
      },
      "data": null,
      "severity": "info",
      "text": "string",
      "type": "pagination.has_more"
    }
  ],
  "results": [
    {
      "error": "string",
      "errorType": "client_error",
      "fields": [
        "string"
      ],
      "id": "string",
      "inputIndex": 0,
      "label": {
        "archived": false,
        "color": "#d32f2f",
        "createdAt": "2021-03-24T09:28:41.200Z",
        "id": "8f4e8961-3f0e-4c1e-b7a1-3d8f9c5e2a6b",
        "name": "string",
        "updatedAt": "2021-03-24T09:28:41.200Z"
      },
      "success": true
    }
  ],
  "summary": {
    "clientErrors": 0,
    "documentsWritten": 0,
    "serverErrors": 0,
    "succeeded": 0,
    "total": 0
  }
}
```

Headers:

- `X-Last-Data-Change-At` (string): Timestamp of last data modification (ISO 8601 UTC with millisecond precision)
  - Example: `2024-01-28T14:23:45.123Z`
- `X-Last-Data-Change-Rev` (string): Revision counter for change detection.
  - Example: `r1234`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`
- `X-Sync-In-Progress` (string): If true, response data is valid but background sync is running.
  - Example: `false`

##### `207` — Mixed results (`application/json`)

Same batch response schema as `200`; check `results[]` for per-item details.

##### `400` — All items failed with client errors (`application/json`)

Same batch response schema as `200`; check `results[]` for per-item error details.

##### `401` — Unauthorized (`application/json`)

```json
{
  "error": "Invalid or expired token"
}
```

##### `403` — Forbidden (`text/plain`)

```text
Forbidden: this token belongs to the Board app and cannot be used on the wallet surface. Use the board endpoints, or authenticate against wallet.
```

##### `429` — Rate limit exceeded (`application/json`)

```json
{
  "error": "Rate limit exceeded. Please try again later."
}
```

Headers:

- `Retry-After` (integer): Seconds to wait before retrying
  - Example: `60`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`

##### `500` — All items failed with server errors (`application/json`)

Same batch response schema as `200`; retry later.

---

### `POST /v1/api/labels` — Create Label

Create a new label/hashtag.

#### Parameters

No parameters.

#### Request body (`application/json`)

Example value:

```json
{
  "color": "blue",
  "name": "vacation"
}
```

#### Responses

##### `201` — Label created (`application/json`)

Example value:

```json
{
  "agentHints": [
    {
      "action": {
        "url": "string"
      },
      "data": null,
      "severity": "info",
      "text": "string",
      "type": "pagination.has_more"
    }
  ],
  "label": {
    "archived": false,
    "color": "#d32f2f",
    "createdAt": "2021-03-24T09:28:41.200Z",
    "id": "8f4e8961-3f0e-4c1e-b7a1-3d8f9c5e2a6b",
    "name": "string",
    "updatedAt": "2021-03-24T09:28:41.200Z"
  },
  "summary": {
    "clientErrors": 0,
    "documentsWritten": 0,
    "serverErrors": 0,
    "succeeded": 0,
    "total": 0
  }
}
```

##### `400` — Bad Request (`application/json`)

```json
{
  "error": "invalid parameter"
}
```

##### `401` — Unauthorized (`application/json`)

```json
{
  "error": "Invalid or expired token"
}
```

##### `403` — Forbidden (`text/plain`)

```text
Forbidden: this token belongs to the Board app and cannot be used on the wallet surface. Use the board endpoints, or authenticate against wallet.
```

##### `429` — Rate limit exceeded (`application/json`)

```json
{
  "error": "Rate limit exceeded. Please try again later."
}
```

Headers:

- `Retry-After` (integer): Seconds to wait before retrying
  - Example: `60`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`

##### `500` — Internal server error (`application/json`)

```json
{
  "error": "Internal Error"
}
```

---

### `GET /v1/api/record-rules` — Get Record Rules

Returns paginated list of automatic categorization rules for records. Default order: newest first (`-createdAt`).

#### Parameters (query)

- `limit` (integer): Maximum number of items to return (1-200, default: 30)
  - Default: `30`
- `offset` (integer): Number of items to skip (default: 0). Used for pagination.
  - Default: `0`
- `agentHints` (boolean): Enable AI agent hints in response.
  - Default: `false`
- `withTotal` (boolean): When true, the response includes a total count of all matching items (requires an additional database query).
  - Default: `false`
- `id` (string): Filter by ID. Supports comma-separated list for multiple IDs (max 30).
- `name` (string): Filter by name. Requires text prefix.
- `labelId` (string): Filter by label ID. Returns entities that include this label.
- `createdAt` (string): Filter by creation timestamp. Requires range prefix.
- `updatedAt` (string): Filter by last sync timestamp (when entity was last updated in the API database). Requires range prefix.

#### Responses

##### `200` — List of record rules (`application/json`)

Example value:

```json
{
  "limit": 0,
  "nextOffset": 0,
  "offset": 0,
  "total": 0,
  "agentHints": [
    {
      "action": {
        "url": "string"
      },
      "data": null,
      "severity": "info",
      "text": "string",
      "type": "pagination.has_more"
    }
  ],
  "recordRules": [
    {
      "category": {
        "color": "#26c6da",
        "group": {
          "id": "food_and_drinks",
          "name": "Food & Drinks"
        },
        "id": "04f8dea1-33dc-411a-b2eb-b348c3168fa3",
        "name": "Health & beauty"
      },
      "createdAt": "2021-03-24T09:28:41.200Z",
      "fromAccountId": "string",
      "id": "f6g7h8i9-j0k1-l2m3-n4o5-p6q7r8s9t0u1",
      "keywords": [
        "string"
      ],
      "labels": [
        {
          "archived": false,
          "color": "#d32f2f",
          "id": "8f4e8961-3f0e-4c1e-b7a1-3d8f9c5e2a6b",
          "name": "Important"
        }
      ],
      "name": "string",
      "toAccountId": "string",
      "updatedAt": "2021-03-24T09:28:41.200Z"
    }
  ]
}
```

Headers:

- `X-Last-Data-Change-At` (string): Timestamp of last data modification (ISO 8601 UTC with millisecond precision)
  - Example: `2024-01-28T14:23:45.123Z`
- `X-Last-Data-Change-Rev` (string): Revision counter for change detection.
  - Example: `r1234`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`
- `X-Sync-In-Progress` (string): If true, response data is valid but background sync is running.
  - Example: `false`

##### `400` — Bad Request (`application/json`)

```json
{
  "error": "invalid parameter"
}
```

##### `401` — Unauthorized (`application/json`)

```json
{
  "error": "Invalid or expired token"
}
```

##### `403` — Forbidden (`text/plain`)

```text
Forbidden: this token belongs to the Board app and cannot be used on the wallet surface. Use the board endpoints, or authenticate against wallet.
```

##### `429` — Rate limit exceeded (`application/json`)

```json
{
  "error": "Rate limit exceeded. Please try again later."
}
```

Headers:

- `Retry-After` (integer): Seconds to wait before retrying
  - Example: `60`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`

##### `500` — Internal server error (`application/json`)

```json
{
  "error": "Internal Error"
}
```

---

## Planning

Planning and automation - budgets, goals, and standing orders

### `GET /v1/api/budgets` — Get Budgets

Returns paginated list of budgets. Default order: newest first (`-createdAt`).

#### Parameters (query)

- `limit` (integer): Maximum number of items to return (1-200, default: 30)
  - Default: `30`
- `offset` (integer): Number of items to skip (default: 0). Used for pagination.
  - Default: `0`
- `agentHints` (boolean): Enable AI agent hints in response.
  - Default: `false`
- `withTotal` (boolean): When true, the response includes a total count of all matching items (requires an additional database query).
  - Default: `false`
- `id` (string): Filter by ID. Supports comma-separated list for multiple IDs (max 30).
- `name` (string): Filter by name. Requires text prefix.
- `currencyCode` (string): Filter by currency code (ISO 4217). Exact match, case-insensitive.
- `closed` (boolean): Filter by closed flag. `true` = only closed budgets, `false` = only open budgets. Omit to return both.
- `type` (string): Filter by budget type (exact match). `BUDGET_ALL` is a read-only legacy type that cannot be created.
  - Available values: `BUDGET_INTERVAL_WEEK`, `BUDGET_INTERVAL_MONTH`, `BUDGET_INTERVAL_YEAR`, `BUDGET_ALL`, `BUDGET_CUSTOM`
- `labelId` (string): Filter by label ID. Returns entities that include this label.
- `accountId` (string): Filter by account ID. Returns budgets that include this account.
- `categoryId` (string): Filter by category ID. Comma-separated for multiple values. Special value `unknown` expands to IDs of 3 categories: Unknown income, Unknown expense, and Uncategorized. Max 10 values.
- `startDate` (string): Filter by budget start date. Requires range prefix (e.g. `gte.2024-01-01`, `lt.2024-12-31`). Up to 2 conditions.
- `endDate` (string): Filter by budget end date. Requires range prefix (e.g. `gte.2024-01-01`, `lt.2024-12-31`). Up to 2 conditions.
- `createdAt` (string): Filter by creation timestamp. Requires range prefix.
- `updatedAt` (string): Filter by last sync timestamp (when entity was last updated in the API database). Requires range prefix.
- `spending` (string): Controls spending data depth.
  - `none` = no spending data (default for REST)
  - `current` = current period only
  - `current+N` = current period plus N past periods
  - Available values: `none`, `current`, `current+2`, `current+5`, `current+10`, `current+25`
  - Default: `none`

#### Responses

##### `200` — List of budgets (`application/json`)

Example value:

```json
{
  "limit": 0,
  "nextOffset": 0,
  "offset": 0,
  "total": 0,
  "agentHints": [
    {
      "action": {
        "url": "string"
      },
      "data": null,
      "severity": "info",
      "text": "string",
      "type": "pagination.has_more"
    }
  ],
  "budgets": [
    {
      "accountIds": [
        "string"
      ],
      "categoryIds": [
        "string"
      ],
      "closed": true,
      "closedDate": "2025-06-15",
      "createdAt": "2021-03-24T09:28:41.200Z",
      "currencyCode": "string",
      "endDate": "2027-01-01",
      "id": "1a2b3c4d-5e6f-7g8h-9i0j-k1l2m3n4o5p6",
      "labelIds": [
        "string"
      ],
      "limit": 0.1,
      "limitOverrides": [
        {
          "createdAt": "string",
          "limit": 0.1,
          "period": "string",
          "periodCount": 0,
          "periodStart": "string"
        }
      ],
      "name": "string",
      "pastLimitOverrides": [
        {
          "createdAt": "string",
          "limit": 0.1,
          "period": "string",
          "periodCount": 0,
          "periodStart": "string"
        }
      ],
      "spending": {
        "computedAt": "string",
        "current": {
          "convertedCurrencies": [
            "string"
          ],
          "effectiveLimit": 0.1,
          "error": "string",
          "excluded": {
            "debts": 0,
            "incomeCategories": 0,
            "total": 0,
            "totalAmountSum": 0.1,
            "transfers": 0,
            "unknownCategories": 0
          },
          "incomplete": true,
          "overspent": 0.1,
          "period": "string",
          "periodEnd": "string",
          "periodStart": "string",
          "progress": 0.1,
          "recordCount": 0,
          "remaining": 0.1,
          "spent": 0.1,
          "totalExpenses": 0.1,
          "totalIncomes": 0.1
        },
        "past": [
          {
            "convertedCurrencies": [
              "string"
            ],
            "effectiveLimit": 0.1,
            "error": "string",
            "excluded": {
              "debts": 0,
              "incomeCategories": 0,
              "total": 0,
              "totalAmountSum": 0.1,
              "transfers": 0,
              "unknownCategories": 0
            },
            "incomplete": true,
            "overspent": 0.1,
            "period": "string",
            "periodEnd": "string",
            "periodStart": "string",
            "progress": 0.1,
            "recordCount": 0,
            "remaining": 0.1,
            "spent": 0.1,
            "totalExpenses": 0.1,
            "totalIncomes": 0.1
          }
        ]
      },
      "startDate": "2026-01-01",
      "type": "BUDGET_INTERVAL_WEEK",
      "updatedAt": "2021-03-24T09:28:41.200Z"
    }
  ]
}
```

Headers:

- `X-Last-Data-Change-At` (string): Timestamp of last data modification (ISO 8601 UTC with millisecond precision)
  - Example: `2024-01-28T14:23:45.123Z`
- `X-Last-Data-Change-Rev` (string): Revision counter for change detection.
  - Example: `r1234`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`
- `X-Sync-In-Progress` (string): If true, response data is valid but background sync is running.
  - Example: `false`

##### `400` — Bad Request (`application/json`)

```json
{
  "error": "invalid parameter"
}
```

##### `401` — Unauthorized (`application/json`)

```json
{
  "error": "Invalid or expired token"
}
```

##### `403` — Forbidden (`text/plain`)

```text
Forbidden: this token belongs to the Board app and cannot be used on the wallet surface. Use the board endpoints, or authenticate against wallet.
```

##### `429` — Rate limit exceeded (`application/json`)

```json
{
  "error": "Rate limit exceeded. Please try again later."
}
```

Headers:

- `Retry-After` (integer): Seconds to wait before retrying
  - Example: `60`
- `X-RateLimit-Limit` (integer): Maximum request capacity (hourly limit)
  - Example: `500`
- `X-RateLimit-Remaining` (integer): Remaining request capacity in current window
  - Example: `487`

##### `500` — Internal server error (`application/json`)

```json
{
  "error": "Internal Error"
}
```

### `POST /v1/api/budgets` — Create Budget

Create a new budget.

#### Parameters

No parameters.

#### Request body (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/CreateBudgetRequest"
}
```

#### Responses

##### `201` — Budget created (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/CreateBudgetResponse"
}
```

##### `400` — Bad Request (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

##### `401` — Unauthorized (`application/json`)

```json
{
  "error": "Invalid or expired token"
}
```

##### `403` — Forbidden (`text/plain`)

```text
Forbidden: this token belongs to the Board app and cannot be used on the wallet surface. Use the board endpoints, or authenticate against wallet.
```

##### `429` — Rate limit exceeded (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

##### `500` — Internal server error (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/Error"
}
```

### `PATCH /v1/api/budgets` — Update Budgets

Batch update budget fields. Maximum 10 budgets per request.

#### Parameters (query)

- `returnData` (boolean): When false, omit the created/updated object body from each per-item result (results still carry id, success, and error). Default: true.

#### Request body (`application/json`)

Schema:

```json
{
  "items": {
    "$ref": "#/components/schemas/PatchBudgetRequest"
  },
  "maxItems": 10,
  "minItems": 1,
  "type": "array"
}
```

#### Responses

##### `200` — All updates succeeded (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/PatchBudgetsResponse"
}
```

##### `207` — Mixed results (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/PatchBudgetsResponse"
}
```

##### `400` — All items failed with client errors (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/PatchBudgetsResponse"
}
```

##### `401` — Unauthorized (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

##### `403` — Forbidden (`text/plain`)

Schema:

```json
{
  "type": "string"
}
```

##### `429` — Rate limit exceeded (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

##### `500` — All items failed with server errors (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/PatchBudgetsResponse"
}
```

### `GET /v1/api/goals` — Get Goals

Returns paginated list of financial goals. Default order: newest first (`-createdAt`).

#### Parameters (query)

- `limit` (integer): Maximum number of items to return (1-200, default: 30)
- `offset` (integer): Number of items to skip (default: 0). Used for pagination.
- `agentHints` (boolean): Enable AI agent hints in response. When true, includes structured hints to help AI agents understand the response and take follow-up actions.
- `withTotal` (boolean): When true, the response includes a total count of all matching items (requires an additional database query). Default: false.
- `id` (string): Filter by ID. Supports comma-separated list for multiple IDs (max 30).
- `name` (string): Filter by name. Requires text prefix.
- `note` (string): Filter goals by note. Requires text prefix.
- `createdAt` (string): Filter by creation timestamp. Requires range prefix.
- `updatedAt` (string): Filter by last sync timestamp (when entity was last updated in the API database). Requires range prefix.

#### Responses

##### `200` — List of goals (`application/json`)

Schema:

```json
{
  "allOf": [
    {
      "$ref": "#/components/schemas/PaginatedResponse"
    },
    {
      "properties": {
        "agentHints": {
          "$ref": "#/components/schemas/AgentHintsArray"
        },
        "goals": {
          "items": {
            "$ref": "#/components/schemas/Goal"
          },
          "type": "array"
        }
      },
      "type": "object"
    }
  ]
}
```

##### `400` — Bad Request (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

##### `401` — Unauthorized (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

##### `403` — Forbidden (`text/plain`)

Schema:

```json
{
  "type": "string"
}
```

##### `429` — Rate limit exceeded (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

##### `500` — Internal server error (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/Error"
}
```

### `GET /v1/api/standing-orders` — Get Standing Orders

Returns paginated list of recurring payments. Default order: newest first (`-createdAt`).

#### Parameters (query)

- `limit` (integer): Maximum number of items to return (1-200, default: 30)
- `offset` (integer): Number of items to skip (default: 0). Used for pagination.
- `agentHints` (boolean): Enable AI agent hints in response. When true, includes structured hints to help AI agents understand the response and take follow-up actions.
- `withTotal` (boolean): When true, the response includes a total count of all matching items (requires an additional database query). Default: false.
- `id` (string): Filter by ID. Supports comma-separated list for multiple IDs (max 30).
- `name` (string): Filter by name. Requires text prefix.
- `currencyCode` (string): Filter by currency code (ISO 4217). Exact match, case-insensitive.
- `labelId` (string): Filter by label ID. Returns entities that include this label.
- `createdAt` (string): Filter by creation timestamp. Requires range prefix.
- `updatedAt` (string): Filter by last sync timestamp (when entity was last updated in the API database). Requires range prefix.

#### Responses

##### `200` — List of standing orders (`application/json`)

Schema:

```json
{
  "allOf": [
    {
      "$ref": "#/components/schemas/PaginatedResponse"
    },
    {
      "properties": {
        "agentHints": {
          "$ref": "#/components/schemas/AgentHintsArray"
        },
        "standingOrders": {
          "items": {
            "$ref": "#/components/schemas/StandingOrder"
          },
          "type": "array"
        }
      },
      "type": "object"
    }
  ]
}
```

##### `400` — Bad Request (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

##### `401` — Unauthorized (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

##### `403` — Forbidden (`text/plain`)

Schema:

```json
{
  "type": "string"
}
```

##### `429` — Rate limit exceeded (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

##### `500` — Internal server error (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/Error"
}
```

### `GET /v1/api/standing-orders/items` — Get Standing Order Items

Returns paginated list of standing order items. Each item represents one occurrence (period) of a standing order and tracks paid/unpaid/dismissed status. Default order: newest first (`-createdAt`).

#### Parameters (query)

- `limit` (integer): Maximum number of items to return (1-200, default: 30)
- `offset` (integer): Number of items to skip (default: 0). Used for pagination.
- `agentHints` (boolean): Enable AI agent hints in response. When true, includes structured hints to help AI agents understand the response and take follow-up actions.
- `withTotal` (boolean): When true, the response includes a total count of all matching items (requires an additional database query). Default: false.
- `standingOrderId` (string): Filter items by standing order ID. Omit for cross-standing-order queries.
- `originalDate` (string): Filter by original date. Supports range operators: `eq.`, `gt.`, `gte.`, `lt.`, `lte.` (e.g. `gte.2024-01-01T00:00:00Z`). Max 2 conditions.
- `dismissed` (boolean): Filter by dismissed status (true/false).
- `recordId` (string): Filter items containing this record ID.
- `paidDate` (string): Filter by paid date. Supports range operators: `eq.`, `gt.`, `gte.`, `lt.`, `lte.` (e.g. `gte.2024-01-01T00:00:00Z`). Max 2 conditions.

#### Responses

##### `200` — List of standing order items (`application/json`)

Schema:

```json
{
  "allOf": [
    {
      "$ref": "#/components/schemas/PaginatedResponse"
    },
    {
      "properties": {
        "agentHints": {
          "$ref": "#/components/schemas/AgentHintsArray"
        },
        "standingOrderItems": {
          "items": {
            "$ref": "#/components/schemas/StandingOrderItem"
          },
          "type": "array"
        }
      },
      "type": "object"
    }
  ]
}
```

##### `400` — Bad Request (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

##### `401` — Unauthorized (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

##### `403` — Forbidden (`text/plain`)

Schema:

```json
{
  "type": "string"
}
```

##### `429` — Rate limit exceeded (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

##### `500` — Internal server error (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/Error"
}
```

---

## Data Management

Bulk operations - delete entities by type, check references before deletion

### `DELETE /v1/api/{type}` — Delete Entities

Delete one or more entities by ID. The entity type is specified in the URL path. Maximum 10 IDs per request. For categories, records, accounts, labels, and orders, use the references endpoint first to check if they are referenced by other entities. Budgets, standing-orders, and record-rules are not referenced by other entities, so no reference check is needed. Paired transfer records delete as a pair: both IDs must be in the same request, otherwise the delete is rejected and the error names the missing leg. `orders` is Board-only (Wallet returns 404).

#### Parameters (path)

- `type` (string): The entity type to delete (required)

#### Request body (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/DeleteRequest"
}
```

#### Responses

##### `200` — All deletes succeeded (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/DeleteResponse"
}
```

##### `207` — Mixed results (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/DeleteResponse"
}
```

##### `400` — All items failed with client errors (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/DeleteResponse"
}
```

##### `401` — Unauthorized (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

##### `403` — Forbidden (`text/plain`)

Schema:

```json
{
  "type": "string"
}
```

##### `429` — Rate limit exceeded (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

##### `500` — All items failed with server errors (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/DeleteResponse"
}
```

### `GET /v1/api/{type}/references` — Get Entity References (Batch)

Returns which entities reference the given entities. Accepts up to 20 comma-separated IDs via the `id` query parameter. Response is a map keyed by entity ID.

Supported types:

- `categories` (returns `records`, `standingOrders`, `budgets`, `recordRules`)
- `records` (returns `standingOrders` via `items.recordIds`)
- `accounts` (returns `records`, `standingOrders`, `budgets`, `recordRules`, `other`)
- `labels` (returns `records`, `standingOrders`, `budgets`, `recordRules`, `other`)
- `orders` (returns `other` — Template documents referencing the order; Board-only)

#### Parameters

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `type` | path | yes | `string` | The entity type to check references for. Plural forms are canonical; singular forms (category, record, account, label) are also accepted via normalization. |
| `id` | query | yes | `string` | Comma-separated entity IDs to check references for (max 20) |

#### Responses

##### `200` — Map of entity ID to references (or per-ID error object) (`application/json`)

Schema:

```json
{
  "additionalProperties": {
    "$ref": "#/components/schemas/ReferencesResponse"
  },
  "example": {
    "550e8400-e29b-41d4-a716-446655440000": {
      "budgets": {
        "field": "categoryIds",
        "hasMore": false,
        "ids": [
          "b2c3d4e5-..."
        ],
        "limit": 50,
        "total": 1
      },
      "recordRules": {
        "field": "categoryId",
        "hasMore": false,
        "ids": [],
        "limit": 50,
        "total": 0
      },
      "records": {
        "field": "categoryId",
        "hasMore": false,
        "ids": [
          "a1b2c3d4-..."
        ],
        "limit": 50,
        "total": 3
      },
      "standingOrders": {
        "field": "categoryId",
        "hasMore": false,
        "ids": [],
        "limit": 50,
        "total": 0
      }
    }
  },
  "type": "object"
}
```

##### `400` — Bad Request (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

##### `401` — Unauthorized (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

##### `403` — Forbidden (`text/plain`)

Schema:

```json
{
  "type": "string"
}
```

##### `429` — Rate limit exceeded (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

##### `500` — Internal server error (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/Error"
}
```

---

## Monitoring

API usage statistics and monitoring

### `GET /v1/api/api-usage/stats` — Get API Usage Statistics

Returns REST API usage statistics for the authenticated client.

- All successful requests are counted (except this endpoint).
- Use `period` to specify the time range (e.g., `30days`, `4weeks`) and how it's grouped by a time period (daily, weekly, monthly).
- Time ranges of each period use half-open intervals: `from` is inclusive, `to` is exclusive.

#### Parameters (query)

- `period` (string): Time period for statistics (required)
  - Formats:
    - `Xdays` → daily granularity (max `366days`)
    - `Xweeks` → weekly granularity, ISO weeks, Monday start (max `53weeks`)
    - `Xmonths` → monthly granularity (max `13months`)

#### Responses

##### `200` — API usage statistics (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/APIUsageStats"
}
```

##### `400` — Bad Request (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

##### `401` — Unauthorized (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

##### `403` — Forbidden (`text/plain`)

Schema:

```json
{
  "type": "string"
}
```

##### `429` — Rate limit exceeded (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

##### `500` — Internal server error (`application/json`)

Schema:

```json
{
  "$ref": "#/components/schemas/Error"
}
```

---

## Next additions

Paste more Swagger UI sections in chat and I’ll append them here.
