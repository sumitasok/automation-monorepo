# Wallet API

**Version:** `2.0.0`

**OpenAPI:** `3.1.0`

REST API for accessing Wallet financial data. Provides endpoints for financial records, accounts, budgets, and other financial entities for the Wallet application.

<br>
<b id="query-filter-prefixes">Query Filter Prefixes</b> <br>
• certain query parameters require inputs with <b>filter prefixes</b><br>
• <b>Range filters:</b> <code>gt.</code> <code>gte.</code> <code>lt.</code> <code>lte.</code> <code>eq.</code> — comparisons (>, ≥, <, ≤, =) <br>
• <b>Text filters:</b> <br>
&nbsp; &nbsp; <code>eq.</code> — exact match <br>
&nbsp; &nbsp; <code>contains.</code> — substring (case-sensitive) <br>
&nbsp; &nbsp; <code>contains-i.</code> — substring (case-insensitive) <br>
• see the <a href="https://rest.budgetbakers.com/wallet/reference#query-filter-prefixes">API Reference</a> for more details and examples

<br><a href="/wallet/openapi/ui?v=1.5.0">previous version: 1.5.0</a>

## Servers
- `https://rest.budgetbakers.com/wallet` — Wallet API server

## Tags
- **Banking**: Core financial data - transaction records and accounts
- **Classification**: Data organization - categories, labels, and record rules
- **Planning**: Planning and automation - budgets, goals, and standing orders
- **Data Management**: Bulk operations - delete entities by type, check references before deletion
- **Monitoring**: API usage statistics and monitoring

## Endpoints

### `/v1/api/accounts`

#### GET `/v1/api/accounts`
- **Summary:** Get Accounts
- **Description:** Returns paginated list of financial accounts
- **Operation ID:** `getAccounts`
- **Tags:** Banking

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `limit` | query | no | `integer` | Maximum number of items to return (1-200, default: 30) |
| `offset` | query | no | `integer` | Number of items to skip (default: 0). Used for pagination. |
| `agentHints` | query | no | `boolean` | Enable AI agent hints in response. When true, includes structured hints to help AI agents understand the response and take follow-up actions. |
| `withTotal` | query | no | `boolean` | When true, the response includes a total count of all matching items (requires an additional database query). Default: false. |
| `id` | query | no | `string` | Filter by ID. Supports comma-separated list for multiple IDs (max 30). |
| `name` | query | no | `string` | Filter by name. Requires text <a href="#query-filter-prefixes">prefix</a>. |
| `accountType` | query | no | `string` | Filter accounts by account type. Case-insensitive. |
| `currencyCode` | query | no | `string` | Filter by currency code (ISO 4217). Exact match, case-insensitive. |
| `createdAt` | query | no | `string` | Filter by creation timestamp. Requires range <a href="#query-filter-prefixes">prefix</a>. |
| `updatedAt` | query | no | `string` | Filter by last sync timestamp (when entity was last updated in the API database). Requires range <a href="#query-filter-prefixes">prefix</a>. |
| `archived` | query | no | `boolean` | Filter by archived status. true=archived only, false=active only, omit for all. |
| `budgetId` | query | no | `string` | Filter items tracked by a specific budget. |
| `sortBy` | query | no | `string` | Sort field with +/- prefix for ASC/DESC. Allowed: createdAt, updatedAt. Example: -createdAt |

**Responses**

- **200**: List of accounts
  - Content-Type: `application/json`
```json
{
  "allOf": [
    {
      "$ref": "#/components/schemas/PaginatedResponse"
    },
    {
      "properties": {
        "accounts": {
          "items": {
            "$ref": "#/components/schemas/Account"
          },
          "type": "array"
        },
        "agentHints": {
          "$ref": "#/components/schemas/AgentHintsArray"
        }
      },
      "type": "object"
    }
  ]
}
```

- **400**: Bad Request - Invalid parameter value
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: Internal server error. The request was valid but processing failed due to a temporary issue. Retry the request later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/Error"
}
```

#### POST `/v1/api/accounts`
- **Summary:** Create Account
- **Description:** Create a new financial account.
- **Operation ID:** `createAccount`
- **Tags:** Banking

**Request Body**
- **Required:** yes
- **Content-Type:** `application/json`
```json
{
  "$ref": "#/components/schemas/CreateAccountRequest"
}
```

**Responses**

- **201**: Account created
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/CreateAccountResponse"
}
```

- **400**: Bad Request - Invalid parameter value
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: Internal server error. The request was valid but processing failed due to a temporary issue. Retry the request later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/Error"
}
```

#### PATCH `/v1/api/accounts`
- **Summary:** Update Accounts
- **Description:** Batch update account fields. Maximum 10 accounts per request.
- **Operation ID:** `patchAccounts`
- **Tags:** Banking

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `returnData` | query | no | `boolean` | When false, omit the created/updated object body from each per-item result (results still carry id, success, and error). Default: true. |

**Request Body**
- **Required:** yes
- **Content-Type:** `application/json`
```json
{
  "items": {
    "$ref": "#/components/schemas/PatchAccountItem"
  },
  "maxItems": 10,
  "minItems": 1,
  "type": "array"
}
```

**Responses**

- **200**: All updates succeeded
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchAccountsResponse"
}
```

- **207**: Mixed results — at least one item succeeded and at least one failed, or both client and server errors occurred. Check results array for per-item details.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchAccountsResponse"
}
```

- **400**: All items failed with client errors (bad input, not found, validation). Body uses the same batch response schema — check results array for per-item error details.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchAccountsResponse"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: All items failed with server errors. Body uses the same batch response schema — check results array for per-item error details. Retry later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchAccountsResponse"
}
```

### `/v1/api/api-usage/stats`

#### GET `/v1/api/api-usage/stats`
- **Summary:** Get API Usage Statistics
- **Description:** ▪️Returns REST API usage statistics for the authenticated client.<br>
▪️All successful requests are counted (except this endpoint).<br>
▪️Use <code>period</code> to specify the time range (e.g., 30days, 4weeks) and how it's grouped by a time period (daily, weekly, monthly).<br>
▪️Time ranges of each period use half-open intervals: <code>from</code> is inclusive, <code>to</code> is exclusive.
- **Operation ID:** `getApiUsageStats`
- **Tags:** Monitoring

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `period` | query | yes | `string` | Time period for statistics.  **Formats:** - `Xdays` → daily granularity (max `366days`) - `Xweeks` → weekly granularity, ISO weeks, Monday start (max `53weeks`) - `Xmonths` → monthly granularity (max `13months`) |

**Responses**

- **200**: API usage statistics
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/APIUsageStats"
}
```

- **400**: Bad Request - Invalid parameter value
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: Internal server error. The request was valid but processing failed due to a temporary issue. Retry the request later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/Error"
}
```

### `/v1/api/budgets`

#### GET `/v1/api/budgets`
- **Summary:** Get Budgets
- **Description:** Returns paginated list of budgets. Default order: newest first (-createdAt).
- **Operation ID:** `getBudgets`
- **Tags:** Planning

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `limit` | query | no | `integer` | Maximum number of items to return (1-200, default: 30) |
| `offset` | query | no | `integer` | Number of items to skip (default: 0). Used for pagination. |
| `agentHints` | query | no | `boolean` | Enable AI agent hints in response. When true, includes structured hints to help AI agents understand the response and take follow-up actions. |
| `withTotal` | query | no | `boolean` | When true, the response includes a total count of all matching items (requires an additional database query). Default: false. |
| `id` | query | no | `string` | Filter by ID. Supports comma-separated list for multiple IDs (max 30). |
| `name` | query | no | `string` | Filter by name. Requires text <a href="#query-filter-prefixes">prefix</a>. |
| `currencyCode` | query | no | `string` | Filter by currency code (ISO 4217). Exact match, case-insensitive. |
| `closed` | query | no | `boolean` | Filter by closed flag. true = only closed budgets, false = only open budgets. Omit to return both. |
| `type` | query | no | `string` | Filter by budget type (exact match). BUDGET_ALL is a read-only legacy type that cannot be created. |
| `labelId` | query | no | `string` | Filter by label ID. Returns entities that include this label. |
| `accountId` | query | no | `string` | Filter by account ID. Returns budgets that include this account. |
| `categoryId` | query | no | `string` | Filter by category ID. Comma-separated for multiple values. Special value 'unknown' expands to IDs of 3 categories: Unknown income, Unknown expense, and Uncategorized. Max 10 values. |
| `startDate` | query | no | `string` | Filter by budget start date. Requires range prefix (e.g. gte.2024-01-01, lt.2024-12-31). Up to 2 conditions. |
| `endDate` | query | no | `string` | Filter by budget end date. Requires range prefix (e.g. gte.2024-01-01, lt.2024-12-31). Up to 2 conditions. |
| `createdAt` | query | no | `string` | Filter by creation timestamp. Requires range <a href="#query-filter-prefixes">prefix</a>. |
| `updatedAt` | query | no | `string` | Filter by last sync timestamp (when entity was last updated in the API database). Requires range <a href="#query-filter-prefixes">prefix</a>. |
| `spending` | query | no | `string` | Controls spending data depth. 'none' = no spending data (default for REST). 'current' = current period only. 'current+N' = current period plus N past periods. |

**Responses**

- **200**: List of budgets
  - Content-Type: `application/json`
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
        "budgets": {
          "items": {
            "$ref": "#/components/schemas/Budget"
          },
          "type": "array"
        }
      },
      "type": "object"
    }
  ]
}
```

- **400**: Bad Request - Invalid parameter value
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: Internal server error. The request was valid but processing failed due to a temporary issue. Retry the request later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/Error"
}
```

#### POST `/v1/api/budgets`
- **Summary:** Create Budget
- **Description:** Create a new budget.
- **Operation ID:** `createBudget`
- **Tags:** Planning

**Request Body**
- **Required:** yes
- **Content-Type:** `application/json`
```json
{
  "$ref": "#/components/schemas/CreateBudgetRequest"
}
```

**Responses**

- **201**: Budget created
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/CreateBudgetResponse"
}
```

- **400**: Bad Request - Invalid parameter value
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: Internal server error. The request was valid but processing failed due to a temporary issue. Retry the request later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/Error"
}
```

#### PATCH `/v1/api/budgets`
- **Summary:** Update Budgets
- **Description:** Batch update budget fields. Maximum 10 budgets per request.
- **Operation ID:** `patchBudgets`
- **Tags:** Planning

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `returnData` | query | no | `boolean` | When false, omit the created/updated object body from each per-item result (results still carry id, success, and error). Default: true. |

**Request Body**
- **Required:** yes
- **Content-Type:** `application/json`
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

**Responses**

- **200**: All updates succeeded
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchBudgetsResponse"
}
```

- **207**: Mixed results — at least one item succeeded and at least one failed, or both client and server errors occurred. Check results array for per-item details.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchBudgetsResponse"
}
```

- **400**: All items failed with client errors (bad input, not found, validation). Body uses the same batch response schema — check results array for per-item error details.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchBudgetsResponse"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: All items failed with server errors. Body uses the same batch response schema — check results array for per-item error details. Retry later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchBudgetsResponse"
}
```

### `/v1/api/categories`

#### GET `/v1/api/categories`
- **Summary:** Get Categories
- **Description:** Returns paginated list of categories
- **Operation ID:** `getCategories`
- **Tags:** Classification

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `limit` | query | no | `integer` | Maximum number of items to return (1-200, default: 30) |
| `offset` | query | no | `integer` | Number of items to skip (default: 0). Used for pagination. |
| `agentHints` | query | no | `boolean` | Enable AI agent hints in response. When true, includes structured hints to help AI agents understand the response and take follow-up actions. |
| `withTotal` | query | no | `boolean` | When true, the response includes a total count of all matching items (requires an additional database query). Default: false. |
| `id` | query | no | `string` | Filter by ID. Supports comma-separated list for multiple IDs (max 30). |
| `name` | query | no | `string` | Filter by name. Requires text <a href="#query-filter-prefixes">prefix</a>. |
| `createdAt` | query | no | `string` | Filter by creation timestamp. Requires range <a href="#query-filter-prefixes">prefix</a>. |
| `updatedAt` | query | no | `string` | Filter by last sync timestamp (when entity was last updated in the API database). Requires range <a href="#query-filter-prefixes">prefix</a>. |
| `customCategory` | query | no | `boolean` | Filter by category type. true=custom subcategories only, false=system categories only. |
| `cardinality` | query | no | `string` | Filter by the category's spending nature, matching the value shown in the response. A category with no cardinality of its own inherits its base category's value, so filtering by an inherited value also returns categories that merely inherit it. cardinality=none returns categories explicitly set to none plus any with no value to inherit. |
| `archived` | query | no | `boolean` | Filter by archived status. true=archived only, false=active only, omit for all. |
| `budgetId` | query | no | `string` | Filter items tracked by a specific budget. |
| `sortBy` | query | no | `string` | Sort field with +/- prefix for ASC/DESC. Allowed: createdAt, updatedAt. Example: -createdAt |

**Responses**

- **200**: List of categories
  - Content-Type: `application/json`
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
        "categories": {
          "items": {
            "$ref": "#/components/schemas/Category"
          },
          "type": "array"
        }
      },
      "type": "object"
    }
  ]
}
```

- **400**: Bad Request - Invalid parameter value
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: Internal server error. The request was valid but processing failed due to a temporary issue. Retry the request later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/Error"
}
```

#### PATCH `/v1/api/categories`
- **Summary:** Update Categories
- **Description:** Batch update category fields (name, color, cardinality). Maximum 10 categories per request.
- **Operation ID:** `patchCategories`
- **Tags:** Classification

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `returnData` | query | no | `boolean` | When false, omit the created/updated object body from each per-item result (results still carry id, success, and error). Default: true. |

**Request Body**
- **Required:** yes
- **Content-Type:** `application/json`
```json
{
  "items": {
    "$ref": "#/components/schemas/PatchCategoryRequest"
  },
  "maxItems": 10,
  "minItems": 1,
  "type": "array"
}
```

**Responses**

- **200**: All updates succeeded
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchCategoriesResponse"
}
```

- **207**: Mixed results — at least one item succeeded and at least one failed, or both client and server errors occurred. Check results array for per-item details.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchCategoriesResponse"
}
```

- **400**: All items failed with client errors (bad input, not found, validation). Body uses the same batch response schema — check results array for per-item error details.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchCategoriesResponse"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: All items failed with server errors. Body uses the same batch response schema — check results array for per-item error details. Retry later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchCategoriesResponse"
}
```

### `/v1/api/categories/custom`

#### POST `/v1/api/categories/custom`
- **Summary:** Create Subcategory
- **Description:** Create a custom subcategory under a system category.
- **Operation ID:** `createCategory`
- **Tags:** Classification

**Request Body**
- **Required:** yes
- **Content-Type:** `application/json`
```json
{
  "$ref": "#/components/schemas/CreateSubcategoryRequest"
}
```

**Responses**

- **201**: Subcategory created
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/CreateCategoryResponse"
}
```

- **400**: Bad Request - Invalid parameter value
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: Internal server error. The request was valid but processing failed due to a temporary issue. Retry the request later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/Error"
}
```

### `/v1/api/goals`

#### GET `/v1/api/goals`
- **Summary:** Get Goals
- **Description:** Returns paginated list of financial goals. Default order: newest first (-createdAt).
- **Operation ID:** `getGoals`
- **Tags:** Planning

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `limit` | query | no | `integer` | Maximum number of items to return (1-200, default: 30) |
| `offset` | query | no | `integer` | Number of items to skip (default: 0). Used for pagination. |
| `agentHints` | query | no | `boolean` | Enable AI agent hints in response. When true, includes structured hints to help AI agents understand the response and take follow-up actions. |
| `withTotal` | query | no | `boolean` | When true, the response includes a total count of all matching items (requires an additional database query). Default: false. |
| `id` | query | no | `string` | Filter by ID. Supports comma-separated list for multiple IDs (max 30). |
| `name` | query | no | `string` | Filter by name. Requires text <a href="#query-filter-prefixes">prefix</a>. |
| `note` | query | no | `string` | Filter goals by note. Requires text <a href="#query-filter-prefixes">prefix</a>. |
| `createdAt` | query | no | `string` | Filter by creation timestamp. Requires range <a href="#query-filter-prefixes">prefix</a>. |
| `updatedAt` | query | no | `string` | Filter by last sync timestamp (when entity was last updated in the API database). Requires range <a href="#query-filter-prefixes">prefix</a>. |

**Responses**

- **200**: List of goals
  - Content-Type: `application/json`
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

- **400**: Bad Request - Invalid parameter value
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: Internal server error. The request was valid but processing failed due to a temporary issue. Retry the request later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/Error"
}
```

### `/v1/api/labels`

#### GET `/v1/api/labels`
- **Summary:** Get Labels
- **Description:** Returns paginated list of labels/hashtags. Default order: name ascending.
- **Operation ID:** `getLabels`
- **Tags:** Classification

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `limit` | query | no | `integer` | Maximum number of items to return (1-200, default: 30) |
| `offset` | query | no | `integer` | Number of items to skip (default: 0). Used for pagination. |
| `agentHints` | query | no | `boolean` | Enable AI agent hints in response. When true, includes structured hints to help AI agents understand the response and take follow-up actions. |
| `withTotal` | query | no | `boolean` | When true, the response includes a total count of all matching items (requires an additional database query). Default: false. |
| `id` | query | no | `string` | Filter by ID. Supports comma-separated list for multiple IDs (max 30). |
| `name` | query | no | `string` | Filter by name. Requires text <a href="#query-filter-prefixes">prefix</a>. |
| `createdAt` | query | no | `string` | Filter by creation timestamp. Requires range <a href="#query-filter-prefixes">prefix</a>. |
| `updatedAt` | query | no | `string` | Filter by last sync timestamp (when entity was last updated in the API database). Requires range <a href="#query-filter-prefixes">prefix</a>. |
| `archived` | query | no | `boolean` | Filter by archived status. true=archived only, false=active only, omit for all. |
| `recordId` | query | no | `string` | Filter labels used by a specific record. |
| `budgetId` | query | no | `string` | Filter items tracked by a specific budget. |
| `standingOrderId` | query | no | `string` | Filter labels used by a specific standing order. |
| `recordRuleId` | query | no | `string` | Filter labels used by a specific record rule. |
| `sortBy` | query | no | `string` | Sort field with +/- prefix for ASC/DESC. Allowed: createdAt, updatedAt. Example: -createdAt |

**Responses**

- **200**: List of labels
  - Content-Type: `application/json`
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
        "labels": {
          "items": {
            "$ref": "#/components/schemas/Label"
          },
          "type": "array"
        }
      },
      "type": "object"
    }
  ]
}
```

- **400**: Bad Request - Invalid parameter value
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: Internal server error. The request was valid but processing failed due to a temporary issue. Retry the request later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/Error"
}
```

#### POST `/v1/api/labels`
- **Summary:** Create Label
- **Description:** Create a new label/hashtag.
- **Operation ID:** `createLabel`
- **Tags:** Classification

**Request Body**
- **Required:** yes
- **Content-Type:** `application/json`
```json
{
  "$ref": "#/components/schemas/CreateLabelRequest"
}
```

**Responses**

- **201**: Label created
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/CreateLabelResponse"
}
```

- **400**: Bad Request - Invalid parameter value
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: Internal server error. The request was valid but processing failed due to a temporary issue. Retry the request later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/Error"
}
```

#### PATCH `/v1/api/labels`
- **Summary:** Update Labels
- **Description:** Batch update label fields. Maximum 10 labels per request.
- **Operation ID:** `patchLabels`
- **Tags:** Classification

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `returnData` | query | no | `boolean` | When false, omit the created/updated object body from each per-item result (results still carry id, success, and error). Default: true. |

**Request Body**
- **Required:** yes
- **Content-Type:** `application/json`
```json
{
  "items": {
    "$ref": "#/components/schemas/PatchLabelItem"
  },
  "maxItems": 10,
  "minItems": 1,
  "type": "array"
}
```

**Responses**

- **200**: All updates succeeded
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchLabelsResponse"
}
```

- **207**: Mixed results — at least one item succeeded and at least one failed, or both client and server errors occurred. Check results array for per-item details.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchLabelsResponse"
}
```

- **400**: All items failed with client errors (bad input, not found, validation). Body uses the same batch response schema — check results array for per-item error details.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchLabelsResponse"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: All items failed with server errors. Body uses the same batch response schema — check results array for per-item error details. Retry later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchLabelsResponse"
}
```

### `/v1/api/record-rules`

#### GET `/v1/api/record-rules`
- **Summary:** Get Record Rules
- **Description:** Returns paginated list of automatic categorization rules for records. Default order: newest first (-createdAt).
- **Operation ID:** `getRecordRules`
- **Tags:** Classification

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `limit` | query | no | `integer` | Maximum number of items to return (1-200, default: 30) |
| `offset` | query | no | `integer` | Number of items to skip (default: 0). Used for pagination. |
| `agentHints` | query | no | `boolean` | Enable AI agent hints in response. When true, includes structured hints to help AI agents understand the response and take follow-up actions. |
| `withTotal` | query | no | `boolean` | When true, the response includes a total count of all matching items (requires an additional database query). Default: false. |
| `id` | query | no | `string` | Filter by ID. Supports comma-separated list for multiple IDs (max 30). |
| `name` | query | no | `string` | Filter by name. Requires text <a href="#query-filter-prefixes">prefix</a>. |
| `labelId` | query | no | `string` | Filter by label ID. Returns entities that include this label. |
| `createdAt` | query | no | `string` | Filter by creation timestamp. Requires range <a href="#query-filter-prefixes">prefix</a>. |
| `updatedAt` | query | no | `string` | Filter by last sync timestamp (when entity was last updated in the API database). Requires range <a href="#query-filter-prefixes">prefix</a>. |

**Responses**

- **200**: List of record rules
  - Content-Type: `application/json`
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
        "recordRules": {
          "items": {
            "$ref": "#/components/schemas/RecordRule"
          },
          "type": "array"
        }
      },
      "type": "object"
    }
  ]
}
```

- **400**: Bad Request - Invalid parameter value
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: Internal server error. The request was valid but processing failed due to a temporary issue. Retry the request later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/Error"
}
```

### `/v1/api/records`

#### GET `/v1/api/records`
- **Summary:** Get Financial Records
- **Description:** ▪️Returns paginated list of financial transaction records.<br>
▪️Use <code>recordDate</code> filter with range operators to scope results. If omitted, a default 3-month window is applied (see <code>appliedRecordDateFilters</code> in the response). Provide any single bound (e.g. <code>gte.2020-01-01</code>) to override.<br>
▪️ID-based lookups (<code>?id=</code>) bypass the default window entirely.
- **Operation ID:** `getRecords`
- **Tags:** Banking

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `id` | query | no | `string` | Filter by ID. Supports comma-separated list for multiple IDs (max 30). |
| `accountId` | query | no | `string` | Filter by account ID(s), comma-separated (max 10). When omitted, returns records from all accounts. Use /accounts to find valid account IDs. |
| `recordDate` | query | no | `string` | Filter by transaction date. Requires range <a href="#query-filter-prefixes">prefix</a> (e.g. <code>gte.2024-01-01</code>, <code>lt.2024-04-01</code>). If omitted, a default 3-month window is applied automatically. Provide any single bound to override the default. The actual applied filter is returned in <code>appliedRecordDateFilters</code>. |
| `limit` | query | no | `integer` | Maximum number of items to return (1-200, default: 30) |
| `offset` | query | no | `integer` | Number of items to skip (default: 0). Used for pagination. |
| `agentHints` | query | no | `boolean` | Enable AI agent hints in response. When true, includes structured hints to help AI agents understand the response and take follow-up actions. |
| `withTotal` | query | no | `boolean` | When true, the response includes a total count of all matching items (requires an additional database query). Default: false. |
| `categoryId` | query | no | `string` | Filter by category ID. Comma-separated for multiple values. Special value 'unknown' expands to IDs of 3 categories: Unknown income, Unknown expense, and Uncategorized. Max 10 values. |
| `labelId` | query | no | `string` | Filter by label ID - returns records that have this label |
| `note` | query | no | `string` | Filter by note. Requires text <a href="#query-filter-prefixes">prefix</a>. |
| `counterParty` | query | no | `string` | Filter records by counter party (payee/payer). Requires text <a href="#query-filter-prefixes">prefix</a>. |
| `amount` | query | no | `string` | Filter by transaction amount. Requires range <a href="#query-filter-prefixes">prefix</a>. Applied to amount.value (decimal format, e.g., 100.50). |
| `createdAt` | query | no | `string` | Filter by creation timestamp. Requires range <a href="#query-filter-prefixes">prefix</a>. |
| `updatedAt` | query | no | `string` | Filter by last sync timestamp (when entity was last updated in the API database). Requires range <a href="#query-filter-prefixes">prefix</a>. |
| `recordType` | query | no | `string` | Filter by record type. Matches on the amount sign (expense = amount < 0, income = amount > 0); a legacy record with an amount of exactly 0 matches the type it was stored with. |
| `isTransfer` | query | no | `boolean` | Filter by transfer state: true = only transfer records (paired or unpaired), false = only non-transfer records, omit = all. Independent of recordType. |
| `transferId` | query | no | `string` | Filter by transfer identity, as returned in transfer.transferId. Returns every record sharing it: both legs of a pair, or the single record of an unpaired transfer. Comma-separated for multiple values. |
| `recordState` | query | no | `string` | Filter by record state. Comma-separated for multiple values (e.g. recordState=reconciled,uncleared). Valid values: reconciled, cleared, uncleared. |
| `source` | query | no | `string` | Filter by record creation source. Comma-separated for multiple values (e.g. source=mcp,rest). Valid values: android, ios, web, rest, mcp, backend, missing. |
| `sortBy` | query | no | `string` | Sort results by field. Prefix with + for ascending, - for descending. Supported fields: recordDate, amount, createdAt, updatedAt. Default: -recordDate (newest first).  Examples: - +recordDate (oldest first) - -recordDate (newest first) - +amount (smallest first) - -amount (largest first) - -createdAt (most recently created first) - -updatedAt (most recently synced first) |
| `convertTo` | query | no | `string` | Convert record amounts to this currency. Accepts ISO-4217 code (e.g. EUR, CZK), 'base' (profile currency), or 'account' (record's account currency). When set, each record includes a convertedAmount field with value, ratio, currencyCode, and conversionPair. |

**Responses**

- **200**: List of records
  - Content-Type: `application/json`
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
        "appliedRecordDateFilters": {
          "description": "The actual recordDate filter applied to this request. Array of operator-prefixed strings (e.g. [\"gte.2026-02-22T...\", \"lt.2026-05-23T...\"]) when a date filter is active (including the implicit 3-month default). Null when no date filter is applied (id-based lookup).",
          "example": [
            "gte.2026-02-22T00:00:00Z",
            "lt.2026-05-23T00:00:00Z"
          ],
          "items": {
            "type": "string"
          },
          "nullable": true,
          "type": "array"
        },
        "records": {
          "items": {
            "$ref": "#/components/schemas/Record"
          },
          "type": "array"
        }
      },
      "type": "object"
    }
  ]
}
```

- **400**: Bad Request - Invalid parameter value
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: Internal server error. The request was valid but processing failed due to a temporary issue. Retry the request later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/Error"
}
```

#### POST `/v1/api/records`
- **Summary:** Create Financial Records
- **Description:** Create one or more transaction records. Maximum 50 records per request. By default each result includes the full created record; pass returnData=false to omit the record bodies (each result still carries inputIndex, id, success, and error).
- **Operation ID:** `createRecords`
- **Tags:** Banking

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `returnData` | query | no | `boolean` | When false, omit the created record body from each result (keeps inputIndex, id, success, error). Default: true. |

**Request Body**
- **Required:** yes
- **Content-Type:** `application/json`
```json
{
  "items": {
    "$ref": "#/components/schemas/CreateRecordRequest"
  },
  "maxItems": 50,
  "minItems": 1,
  "type": "array"
}
```

**Responses**

- **200**: All records created successfully
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/CreateRecordsResponse"
}
```

- **207**: Mixed results — at least one item succeeded and at least one failed, or both client and server errors occurred. Check results array for per-item details.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/CreateRecordsResponse"
}
```

- **400**: All items failed with client errors (bad input, not found, validation). Body uses the same batch response schema — check results array for per-item error details.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/CreateRecordsResponse"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: All items failed with server errors. Body uses the same batch response schema — check results array for per-item error details. Retry later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/CreateRecordsResponse"
}
```

#### PATCH `/v1/api/records`
- **Summary:** Update Financial Records
- **Description:** Batch update record fields (category, labels, note, amount, etc.). Maximum 10 records per request.
- **Operation ID:** `patchRecords`
- **Tags:** Banking

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `validation` | query | no | `string` | Validation mode. 'strict': validate all items first; if any client_error, return 400 with full results (no writes). Default: lenient (process valid items, skip invalid). |
| `returnData` | query | no | `boolean` | When false, omit the created/updated object body from each per-item result (results still carry id, success, and error). Default: true. |

**Request Body**
- **Required:** yes
- **Content-Type:** `application/json`
```json
{
  "items": {
    "$ref": "#/components/schemas/PatchRecordItem"
  },
  "maxItems": 10,
  "minItems": 1,
  "type": "array"
}
```

**Responses**

- **200**: All updates succeeded
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchRecordsResponse"
}
```

- **207**: Mixed results — at least one item succeeded and at least one failed, or both client and server errors occurred. Check results array for per-item details.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchRecordsResponse"
}
```

- **400**: All items failed with client errors (bad input, not found, validation). Body uses the same batch response schema — check results array for per-item error details.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchRecordsResponse"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: All items failed with server errors. Body uses the same batch response schema — check results array for per-item error details. Retry later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/PatchRecordsResponse"
}
```

### `/v1/api/standing-orders`

#### GET `/v1/api/standing-orders`
- **Summary:** Get Standing Orders
- **Description:** Returns paginated list of recurring payments. Default order: newest first (-createdAt).
- **Operation ID:** `getStandingOrders`
- **Tags:** Planning

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `limit` | query | no | `integer` | Maximum number of items to return (1-200, default: 30) |
| `offset` | query | no | `integer` | Number of items to skip (default: 0). Used for pagination. |
| `agentHints` | query | no | `boolean` | Enable AI agent hints in response. When true, includes structured hints to help AI agents understand the response and take follow-up actions. |
| `withTotal` | query | no | `boolean` | When true, the response includes a total count of all matching items (requires an additional database query). Default: false. |
| `id` | query | no | `string` | Filter by ID. Supports comma-separated list for multiple IDs (max 30). |
| `name` | query | no | `string` | Filter by name. Requires text <a href="#query-filter-prefixes">prefix</a>. |
| `currencyCode` | query | no | `string` | Filter by currency code (ISO 4217). Exact match, case-insensitive. |
| `labelId` | query | no | `string` | Filter by label ID. Returns entities that include this label. |
| `createdAt` | query | no | `string` | Filter by creation timestamp. Requires range <a href="#query-filter-prefixes">prefix</a>. |
| `updatedAt` | query | no | `string` | Filter by last sync timestamp (when entity was last updated in the API database). Requires range <a href="#query-filter-prefixes">prefix</a>. |

**Responses**

- **200**: List of standing orders
  - Content-Type: `application/json`
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

- **400**: Bad Request - Invalid parameter value
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: Internal server error. The request was valid but processing failed due to a temporary issue. Retry the request later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/Error"
}
```

### `/v1/api/standing-orders/items`

#### GET `/v1/api/standing-orders/items`
- **Summary:** Get Standing Order Items
- **Description:** Returns paginated list of standing order items. Each item represents one occurrence (period) of a standing order and tracks paid/unpaid/dismissed status. Default order: newest first (-createdAt).
- **Operation ID:** `getStandingOrderItems`
- **Tags:** Planning

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `limit` | query | no | `integer` | Maximum number of items to return (1-200, default: 30) |
| `offset` | query | no | `integer` | Number of items to skip (default: 0). Used for pagination. |
| `agentHints` | query | no | `boolean` | Enable AI agent hints in response. When true, includes structured hints to help AI agents understand the response and take follow-up actions. |
| `withTotal` | query | no | `boolean` | When true, the response includes a total count of all matching items (requires an additional database query). Default: false. |
| `standingOrderId` | query | no | `string` | Filter items by standing order ID. Omit for cross-standing-order queries. |
| `originalDate` | query | no | `string` | Filter by original date. Supports range operators: eq., gt., gte., lt., lte. (e.g., gte.2024-01-01T00:00:00Z). Max 2 conditions. |
| `dismissed` | query | no | `boolean` | Filter by dismissed status (true/false). |
| `recordId` | query | no | `string` | Filter items containing this record ID. |
| `paidDate` | query | no | `string` | Filter by paid date. Supports range operators: eq., gt., gte., lt., lte. (e.g., gte.2024-01-01T00:00:00Z). Max 2 conditions. |

**Responses**

- **200**: List of standing order items
  - Content-Type: `application/json`
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

- **400**: Bad Request - Invalid parameter value
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: Internal server error. The request was valid but processing failed due to a temporary issue. Retry the request later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/Error"
}
```

### `/v1/api/{type}`

#### DELETE `/v1/api/{type}`
- **Summary:** Delete Entities
- **Description:** Delete one or more entities by ID. The entity type is specified in the URL path. Maximum 10 IDs per request. For categories, records, accounts, labels, and orders, use the references endpoint first to check if they are referenced by other entities. Budgets, standing-orders, and record-rules are not referenced by other entities, so no reference check is needed. Paired transfer records delete as a pair: both IDs must be in the same request, otherwise the delete is rejected and the error names the missing leg. `orders` is Board-only (Wallet returns 404).
- **Operation ID:** `deleteByType`
- **Tags:** Data Management

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `type` | path | yes | `string` | The entity type to delete |

**Request Body**
- **Required:** yes
- **Content-Type:** `application/json`
```json
{
  "$ref": "#/components/schemas/DeleteRequest"
}
```

**Responses**

- **200**: All deletes succeeded
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/DeleteResponse"
}
```

- **207**: Mixed results — at least one item succeeded and at least one failed, or both client and server errors occurred. Check results array for per-item details.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/DeleteResponse"
}
```

- **400**: All items failed with client errors (bad input, not found, validation). Body uses the same batch response schema — check results array for per-item error details.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/DeleteResponse"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: All items failed with server errors. Body uses the same batch response schema — check results array for per-item error details. Retry later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/DeleteResponse"
}
```

### `/v1/api/{type}/references`

#### GET `/v1/api/{type}/references`
- **Summary:** Get Entity References (Batch)
- **Description:** Returns which entities reference the given entities. Accepts up to 20 comma-separated IDs via the id query parameter. Response is a map keyed by entity ID. Supported types: categories (returns records, standingOrders, budgets, recordRules), records (returns standingOrders via items.recordIds), accounts (returns records, standingOrders, budgets, recordRules, other), labels (returns records, standingOrders, budgets, recordRules, other), orders (returns other — Template documents referencing the order; Board-only).
- **Operation ID:** `getReferences`
- **Tags:** Data Management

**Parameters**

| Name | In | Required | Type | Description |
|---|---|---:|---|---|
| `type` | path | yes | `string` | The entity type to check references for. Plural forms are canonical; singular forms (category, record, account, label) are also accepted via normalization. |
| `id` | query | yes | `string` | Comma-separated entity IDs to check references for (max 20) |

**Responses**

- **200**: Map of entity ID to references (or per-ID error object)
  - Content-Type: `application/json`
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

- **400**: Bad Request - Invalid parameter value
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/ValidationError"
}
```

- **401**: Unauthorized
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/UnauthorizedError"
}
```

- **403**: Forbidden — the token belongs to the other app. Tokens carry the flavor of the login that minted them, and may only be used on that flavor's endpoints (a Wallet token on /wallet/..., a Board token on /board/...). Re-authenticating against the same app will not help; use the other prefix, or a token issued for this one.
  - Content-Type: `text/plain`
```json
{
  "type": "string"
}
```

- **429**: Rate limit exceeded. Check `Retry-After` header for when to retry.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/RateLimitError"
}
```

- **500**: Internal server error. The request was valid but processing failed due to a temporary issue. Retry the request later.
  - Content-Type: `application/json`
```json
{
  "$ref": "#/components/schemas/Error"
}
```

## Components: schemas

### `APIUsageStats`
```json
{
  "properties": {
    "from": {
      "description": "Start of the queried date range, inclusive (ISO 8601 UTC timestamp)",
      "format": "date-time",
      "type": "string"
    },
    "granularity": {
      "description": "Time granularity of the usage entries",
      "enum": [
        "daily",
        "weekly",
        "monthly"
      ],
      "type": "string"
    },
    "period": {
      "description": "The requested period (e.g., '30days', '4weeks')",
      "type": "string"
    },
    "to": {
      "description": "End of the queried date range, exclusive (ISO 8601 UTC timestamp)",
      "format": "date-time",
      "type": "string"
    },
    "total": {
      "description": "Total API requests in the period",
      "type": "integer"
    },
    "usage": {
      "description": "Per-period request totals. Periods with zero usage are omitted.",
      "items": {
        "$ref": "#/components/schemas/APIUsageStatsEntry"
      },
      "type": "array"
    }
  },
  "required": [
    "period",
    "granularity",
    "from",
    "to",
    "total",
    "usage"
  ],
  "type": "object"
}
```

### `APIUsageStatsEntry`
```json
{
  "properties": {
    "from": {
      "description": "Start of period, inclusive (ISO 8601 UTC timestamp, e.g., '2025-01-07T00:00:00Z')",
      "format": "date-time",
      "type": "string"
    },
    "to": {
      "description": "End of period, exclusive (ISO 8601 UTC timestamp). This is the start of the next period, so the range is [from, to). For daily granularity, 'to' is 'from' + 1 day.",
      "format": "date-time",
      "type": "string"
    },
    "total": {
      "description": "Total API requests in this period",
      "type": "integer"
    }
  },
  "required": [
    "from",
    "to",
    "total"
  ],
  "type": "object"
}
```

### `Account`
```json
{
  "properties": {
    "accountType": {
      "description": "Account type. Creatable: General (general purpose), Cash (physical cash), CurrentAccount (checking/current), SavingAccount (savings), Insurance (policy). Read-only (bank-sync/mobile only): CreditCard, Investment, Loan, Mortgage, Overdraft, Bonus (rewards/legacy)",
      "enum": [
        "General",
        "Cash",
        "CurrentAccount",
        "CreditCard",
        "SavingAccount",
        "Bonus",
        "Insurance",
        "Investment",
        "Loan",
        "Mortgage",
        "Overdraft"
      ],
      "example": "CurrentAccount",
      "type": "string"
    },
    "archived": {
      "example": false,
      "type": "boolean"
    },
    "balance": {
      "allOf": [
        {
          "$ref": "#/components/schemas/AccountBalance"
        }
      ],
      "description": "Computed account balance with credit card/overdraft corrections applied. Read-only. For non-credit accounts rawCurrentBalance equals currentBalance and credit fields are absent. For credit/overdraft accounts, balanceMode identifies the formula variant and formula shows the exact computation.",
      "example": {
        "balanceMode": "standard",
        "currencyCode": "USD",
        "currentBalance": 3343.75,
        "formula": "initial + totalIncomes - totalExpenses",
        "initial": 1000,
        "rawCurrentBalance": 3343.75
      },
      "nullable": true
    },
    "bankAccountNumber": {
      "example": "CZ65 0800 0000 1920 0014 5399",
      "type": "string"
    },
    "color": {
      "description": "Hex color code",
      "example": "#d32f2f",
      "type": "string"
    },
    "createdAt": {
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "type": "string"
    },
    "currencyCode": {
      "description": "The account's currency (ISO 4217). Also repeated inside balance so amounts are self-describing — the two values are always identical. Empty string when the currency could not be resolved.",
      "example": "USD",
      "type": "string"
    },
    "excludeFromStats": {
      "example": false,
      "type": "boolean"
    },
    "id": {
      "description": "Unique identifier",
      "example": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
      "type": "string"
    },
    "isBankSync": {
      "description": "Whether this account is connected to a bank via automatic sync (e.g. SaltEdge, Plaid). Read-only.",
      "example": false,
      "type": "boolean"
    },
    "isInvestmentAccount": {
      "description": "Whether this account contains investment portfolio data (stocks, funds, etc.). Read-only.",
      "example": false,
      "type": "boolean"
    },
    "name": {
      "example": "My Checking Account",
      "type": "string"
    },
    "recordStats": {
      "$ref": "#/components/schemas/AccountStats",
      "example": {
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
      "nullable": true
    },
    "updatedAt": {
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "type": "string"
    }
  },
  "type": "object"
}
```

### `AccountBalance`
```json
{
  "properties": {
    "availableCredit": {
      "format": "double",
      "type": "number"
    },
    "balanceDisplayOption": {
      "type": "string"
    },
    "balanceMode": {
      "description": "Computed balance mode derived from accountType, isBankSync, and credit card settings. Determines which formula variant is used. Read-only.",
      "enum": [
        "standard",
        "creditCardManual",
        "creditCardManualInverse",
        "creditCardBank",
        "creditCardBankReversed",
        "overdraftManual",
        "overdraftBank"
      ],
      "example": "standard",
      "type": "string"
    },
    "balanceModeFormula": {
      "description": "Formula pattern showing both creditBalance and availableCredit variants for this balance mode. Read-only.",
      "example": "creditBalance = -(initial + totalIncomes - totalExpenses); availableCredit = creditLimit + creditBalance",
      "type": "string"
    },
    "creditBalance": {
      "format": "double",
      "type": "number"
    },
    "creditLimit": {
      "format": "double",
      "type": "number"
    },
    "currencyCode": {
      "type": "string"
    },
    "currentBalance": {
      "format": "double",
      "type": "number"
    },
    "error": {
      "type": "string"
    },
    "formula": {
      "type": "string"
    },
    "initial": {
      "format": "double",
      "type": "number"
    },
    "rawCurrentBalance": {
      "format": "double",
      "type": "number"
    }
  },
  "type": "object"
}
```

### `AccountStats`
```json
{
  "properties": {
    "createdAt": {
      "$ref": "#/components/schemas/StatDateRange"
    },
    "error": {
      "type": "string"
    },
    "errorAt": {
      "type": "string"
    },
    "lastUpdatedAt": {
      "type": "string"
    },
    "recordCount": {
      "type": "integer"
    },
    "recordDate": {
      "$ref": "#/components/schemas/StatDateRange"
    },
    "totalExpenses": {
      "format": "double",
      "type": "number"
    },
    "totalIncomes": {
      "format": "double",
      "type": "number"
    }
  },
  "type": "object"
}
```

### `AgentHint`
```json
{
  "description": "Structured hint for AI agents to understand API responses and take appropriate actions",
  "properties": {
    "action": {
      "description": "Action data for instruction hints. Only present when severity=instruction. Contains url for fetching next page.",
      "nullable": true,
      "properties": {
        "url": {
          "description": "URL for next page (pagination.has_more)",
          "type": "string"
        }
      },
      "type": "object"
    },
    "data": {
      "additionalProperties": true,
      "description": "Context data specific to the hint type. Structure varies based on the 'type' field. See 'text' for human-readable explanation.",
      "example": null,
      "nullable": true,
      "type": "object"
    },
    "severity": {
      "description": "Indicates the nature/importance of the hint",
      "enum": [
        "info",
        "warning",
        "instruction"
      ],
      "type": "string",
      "x-enum-descriptions": {
        "info": "Neutral informational hint (e.g., all results returned, parameter inferred)",
        "instruction": "Agent should take action (e.g., fetch next page via action.url)",
        "warning": "Alert about potential issues (e.g., rate limit approaching)"
      }
    },
    "text": {
      "description": "Human/AI-readable description of the hint",
      "type": "string"
    },
    "type": {
      "description": "Hint category using dot-notation",
      "enum": [
        "pagination.has_more",
        "result.partial_match",
        "result.empty",
        "rate_limit.warning"
      ],
      "type": "string",
      "x-enum-descriptions": {
        "pagination.has_more": "More results available beyond this page. Use action.url to fetch next page.",
        "rate_limit.warning": "API rate limit is approaching. See data.remaining and data.reset_at for details.",
        "result.empty": "No records found. If filters were applied, data.appliedFilters shows them; otherwise, no records of this type exist yet.",
        "result.partial_match": "Some requested IDs were not found. Check data.missingIds for the list."
      }
    }
  },
  "required": [
    "type",
    "severity",
    "text"
  ],
  "type": "object"
}
```

### `AgentHintsArray`
```json
{
  "description": "Array of hints for AI agents. Only present when agentHints=true query parameter is set.",
  "items": {
    "$ref": "#/components/schemas/AgentHint"
  },
  "nullable": true,
  "type": "array"
}
```

### `AmountWithCurrency`
```json
{
  "properties": {
    "currencyCode": {
      "description": "Currency code (ISO 4217)",
      "example": "USD",
      "type": "string"
    },
    "value": {
      "description": "Amount value in decimal format (e.g., 1234.56 for one thousand two hundred thirty-four dollars and fifty-six cents). Not in cents.",
      "example": 1234.56,
      "format": "double",
      "type": "number"
    }
  },
  "type": "object"
}
```

### `BatchSummary`
```json
{
  "description": "Summary of batch operation results",
  "properties": {
    "clientErrors": {
      "description": "Items that failed due to invalid input (client errors)",
      "type": "integer"
    },
    "documentsWritten": {
      "description": "Documents actually written or removed, counted from confirmed write outcomes and deduplicated. NOT a copy of `succeeded`: on delete, a repeated id is idempotent, so `succeeded` counts input rows and this counts DISTINCT documents removed — and a retried delete of an already-deleted document reports success with 0 here, because that call wrote nothing. On create it can EXCEED `total`: an auto-created transfer mirror, a rewritten pairing target, and a CRM order document rewritten by an orderId bind are all counted here without being items.",
      "type": "integer"
    },
    "serverErrors": {
      "description": "Items that failed due to internal errors (server errors)",
      "type": "integer"
    },
    "succeeded": {
      "description": "Successfully processed items",
      "type": "integer"
    },
    "total": {
      "description": "Total items in request",
      "type": "integer"
    }
  },
  "required": [
    "total",
    "succeeded",
    "clientErrors",
    "serverErrors",
    "documentsWritten"
  ],
  "type": "object"
}
```

### `Budget`
```json
{
  "properties": {
    "accountIds": {
      "items": {
        "type": "string"
      },
      "type": "array"
    },
    "categoryIds": {
      "description": "Array of category IDs. System categories use deterministic IDs, custom categories use standard IDs.",
      "items": {
        "type": "string"
      },
      "type": "array"
    },
    "closed": {
      "nullable": true,
      "type": "boolean"
    },
    "closedDate": {
      "description": "Date the budget was closed (YYYY-MM-DD). Present only when closed=true.",
      "example": "2025-06-15",
      "format": "date",
      "type": "string"
    },
    "createdAt": {
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "type": "string"
    },
    "currencyCode": {
      "type": "string"
    },
    "endDate": {
      "example": "2027-01-01",
      "format": "date",
      "type": "string"
    },
    "id": {
      "example": "1a2b3c4d-5e6f-7g8h-9i0j-k1l2m3n4o5p6",
      "type": "string"
    },
    "labelIds": {
      "items": {
        "type": "string"
      },
      "type": "array"
    },
    "limit": {
      "format": "double",
      "nullable": true,
      "type": "number"
    },
    "limitOverrides": {
      "description": "Limit overrides in force now or later. EFFECTIVE-FROM: each entry sets the limit for its period and every period after it, until a later entry takes over, so the first entry may have a periodStart in the past — that is the override currently in force. Each entry: {limit (number), periodStart (YYYY-MM-DD), period (YYYY-MM / YYYY-Www / YYYY), periodCount (int, how many periods this entry governs — absent means open-ended, nothing after it takes over), createdAt (datetime, optional)}. Empty array means uniform limit across all periods.",
      "items": {
        "$ref": "#/components/schemas/BudgetChangeEntry"
      },
      "type": "array"
    },
    "name": {
      "type": "string"
    },
    "pastLimitOverrides": {
      "description": "Limit overrides that a later entry has already superseded — they no longer affect any period. Same shape as limitOverrides. Empty array means no superseded overrides.",
      "items": {
        "$ref": "#/components/schemas/BudgetChangeEntry"
      },
      "type": "array"
    },
    "spending": {
      "$ref": "#/components/schemas/BudgetSpending"
    },
    "startDate": {
      "example": "2026-01-01",
      "format": "date",
      "type": "string"
    },
    "type": {
      "description": "Budget interval type. BUDGET_ALL is a read-only legacy type.",
      "enum": [
        "BUDGET_INTERVAL_WEEK",
        "BUDGET_INTERVAL_MONTH",
        "BUDGET_INTERVAL_YEAR",
        "BUDGET_ALL",
        "BUDGET_CUSTOM"
      ],
      "type": "string"
    },
    "updatedAt": {
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "type": "string"
    }
  },
  "type": "object"
}
```

### `BudgetChangeEntry`
```json
{
  "properties": {
    "createdAt": {
      "type": "string"
    },
    "limit": {
      "format": "double",
      "type": "number"
    },
    "period": {
      "type": "string"
    },
    "periodCount": {
      "type": "integer"
    },
    "periodStart": {
      "type": "string"
    }
  },
  "type": "object"
}
```

### `BudgetPeriodSpending`
```json
{
  "properties": {
    "convertedCurrencies": {
      "items": {
        "type": "string"
      },
      "type": "array"
    },
    "effectiveLimit": {
      "format": "double",
      "type": "number"
    },
    "error": {
      "type": "string"
    },
    "excluded": {
      "$ref": "#/components/schemas/ExcludedBreakdown",
      "nullable": true
    },
    "incomplete": {
      "type": "boolean"
    },
    "overspent": {
      "format": "double",
      "nullable": true,
      "type": "number"
    },
    "period": {
      "type": "string"
    },
    "periodEnd": {
      "nullable": true,
      "type": "string"
    },
    "periodStart": {
      "nullable": true,
      "type": "string"
    },
    "progress": {
      "format": "double",
      "nullable": true,
      "type": "number"
    },
    "recordCount": {
      "type": "integer"
    },
    "remaining": {
      "format": "double",
      "nullable": true,
      "type": "number"
    },
    "spent": {
      "format": "double",
      "nullable": true,
      "type": "number"
    },
    "totalExpenses": {
      "format": "double",
      "nullable": true,
      "type": "number"
    },
    "totalIncomes": {
      "format": "double",
      "nullable": true,
      "type": "number"
    }
  },
  "type": "object"
}
```

### `BudgetSpending`
```json
{
  "properties": {
    "computedAt": {
      "type": "string"
    },
    "current": {
      "$ref": "#/components/schemas/BudgetPeriodSpending",
      "nullable": true
    },
    "past": {
      "items": {
        "$ref": "#/components/schemas/BudgetPeriodSpending"
      },
      "type": "array"
    }
  },
  "type": "object"
}
```

### `Category`
```json
{
  "properties": {
    "archived": {
      "type": "boolean"
    },
    "cardinality": {
      "enum": [
        "none",
        "must",
        "need",
        "want"
      ],
      "type": "string"
    },
    "color": {
      "description": "Hex color code",
      "example": "#26c6da",
      "type": "string"
    },
    "createdAt": {
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "nullable": true,
      "type": "string"
    },
    "customCategory": {
      "example": false,
      "type": "boolean"
    },
    "customName": {
      "type": "boolean"
    },
    "enabled": {
      "example": true,
      "type": "boolean"
    },
    "group": {
      "$ref": "#/components/schemas/CategoryGroup",
      "nullable": true
    },
    "id": {
      "example": "04f8dea1-33dc-411a-b2eb-b348c3168fa3",
      "type": "string"
    },
    "name": {
      "type": "string"
    },
    "parentId": {
      "description": "UUID of the parent base category. Present for custom categories only.",
      "type": "string"
    },
    "parentName": {
      "description": "Display name of the parent base category, exactly as GET /categories renders it. Present for custom categories only.",
      "example": "Groceries",
      "type": "string"
    },
    "systemId": {
      "description": "System category identifier (envelope slug). Present for system categories.",
      "example": "food_and_drinks__groceries",
      "type": "string"
    },
    "updatedAt": {
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "nullable": true,
      "type": "string"
    }
  },
  "type": "object"
}
```

### `CategoryEmbed`
```json
{
  "nullable": true,
  "properties": {
    "color": {
      "description": "Hex color code",
      "example": "#26c6da",
      "type": "string"
    },
    "group": {
      "$ref": "#/components/schemas/CategoryGroup"
    },
    "id": {
      "description": "Category ID",
      "example": "04f8dea1-33dc-411a-b2eb-b348c3168fa3",
      "type": "string"
    },
    "name": {
      "description": "Category name",
      "example": "Health & beauty",
      "type": "string"
    }
  },
  "type": "object"
}
```

### `CategoryGroup`
```json
{
  "properties": {
    "id": {
      "description": "Category group slug — stable identifier for the root-level group (e.g. 'food_and_drinks', 'shopping', 'income')",
      "example": "food_and_drinks",
      "type": "string"
    },
    "name": {
      "description": "Category group display name (e.g. 'Food & Drinks')",
      "example": "Food & Drinks",
      "type": "string"
    }
  },
  "type": "object"
}
```

### `ConvertedAmount`
```json
{
  "properties": {
    "conversionPair": {
      "description": "Currency pair direction: recordCurrency:targetCurrency",
      "example": "EUR:CZK",
      "type": "string"
    },
    "currencyCode": {
      "description": "Target currency code (ISO 4217)",
      "example": "CZK",
      "type": "string"
    },
    "error": {
      "description": "Error message when rate is unavailable (other fields omitted except currencyCode)",
      "type": "string"
    },
    "ratio": {
      "description": "Exchange rate used (1 unit of record currency = ratio units of target currency)",
      "example": 25.42,
      "format": "double",
      "type": "number"
    },
    "value": {
      "description": "Converted amount value (record amount × ratio)",
      "example": 2567.89,
      "format": "double",
      "type": "number"
    }
  },
  "type": "object"
}
```

### `CreateAccountRequest`
```json
{
  "example": {
    "accountType": "General",
    "currencyCode": "EUR",
    "initialBalance": 0,
    "name": "My new account"
  },
  "properties": {
    "accountType": {
      "enum": [
        "General",
        "Cash",
        "CurrentAccount",
        "SavingAccount",
        "Insurance"
      ],
      "type": "string"
    },
    "bankAccountNumber": {
      "description": "Bank account number or IBAN (max 80 chars)",
      "maxLength": 80,
      "type": "string"
    },
    "color": {
      "description": "Universal color name. Randomly assigned if omitted.",
      "enum": [
        "Blue",
        "Charcoal",
        "CoffeeBrown",
        "DarkPurple",
        "Green",
        "LightBlue",
        "Lime",
        "Orange",
        "PineGreen",
        "Pink",
        "Purple",
        "Red",
        "Stone",
        "Turquoise",
        "Wine",
        "Yellow"
      ],
      "type": "string"
    },
    "currencyCode": {
      "description": "3-letter fiat currency code",
      "type": "string"
    },
    "excludeFromStats": {
      "default": false,
      "type": "boolean"
    },
    "initialBalance": {
      "description": "Initial balance amount (e.g. 1500.50). INPUT ONLY — it is not returned on the account object; read it back as balance.initial (with balance.currencyCode alongside).",
      "type": "number",
      "writeOnly": true
    },
    "name": {
      "maxLength": 80,
      "type": "string"
    }
  },
  "required": [
    "name",
    "accountType",
    "currencyCode",
    "initialBalance"
  ],
  "type": "object"
}
```

### `CreateAccountResponse`
```json
{
  "properties": {
    "account": {
      "$ref": "#/components/schemas/Account"
    },
    "agentHints": {
      "$ref": "#/components/schemas/AgentHintsArray"
    },
    "summary": {
      "$ref": "#/components/schemas/BatchSummary",
      "description": "Standard write summary (A7): total 1, succeeded 1, documentsWritten 1 on the success this envelope reports."
    }
  },
  "type": "object"
}
```

### `CreateBudgetRequest`
```json
{
  "example": {
    "categoryIds": [
      "550e8400-e29b-41d4-a716-446655440000"
    ],
    "currencyCode": "USD",
    "limit": 300,
    "name": "Monthly Food",
    "type": "BUDGET_INTERVAL_MONTH"
  },
  "properties": {
    "accountIds": {
      "description": "Account IDs to track",
      "items": {
        "type": "string"
      },
      "type": "array"
    },
    "categoryIds": {
      "description": "Category IDs to track (system or custom)",
      "items": {
        "type": "string"
      },
      "type": "array"
    },
    "currencyCode": {
      "description": "3-letter currency code (e.g. USD, EUR)",
      "type": "string"
    },
    "endDate": {
      "description": "End date (YYYY-MM-DD). Required for BUDGET_CUSTOM.",
      "format": "date",
      "type": "string"
    },
    "labelIds": {
      "description": "Label IDs to filter transactions",
      "items": {
        "type": "string"
      },
      "type": "array"
    },
    "limit": {
      "description": "Spending limit (must be > 0)",
      "example": 500,
      "type": "number"
    },
    "name": {
      "description": "Budget display name",
      "type": "string"
    },
    "startDate": {
      "description": "Start date (YYYY-MM-DD). Required for BUDGET_CUSTOM.",
      "format": "date",
      "type": "string"
    },
    "type": {
      "description": "Budget interval type. BUDGET_CUSTOM requires startDate and endDate. Type is immutable after creation.",
      "enum": [
        "BUDGET_INTERVAL_WEEK",
        "BUDGET_INTERVAL_MONTH",
        "BUDGET_INTERVAL_YEAR",
        "BUDGET_CUSTOM"
      ],
      "type": "string"
    }
  },
  "required": [
    "name",
    "currencyCode",
    "type",
    "limit"
  ],
  "type": "object"
}
```

### `CreateBudgetResponse`
```json
{
  "properties": {
    "agentHints": {
      "$ref": "#/components/schemas/AgentHintsArray"
    },
    "budget": {
      "$ref": "#/components/schemas/Budget"
    },
    "summary": {
      "$ref": "#/components/schemas/BatchSummary",
      "description": "Standard write summary (A7): total 1, succeeded 1, documentsWritten 1 on the success this envelope reports."
    }
  },
  "type": "object"
}
```

### `CreateCategoryResponse`
```json
{
  "properties": {
    "agentHints": {
      "$ref": "#/components/schemas/AgentHintsArray"
    },
    "category": {
      "$ref": "#/components/schemas/Category"
    },
    "summary": {
      "$ref": "#/components/schemas/BatchSummary",
      "description": "Standard write summary (A7): total 1, succeeded 1, documentsWritten 1 on the success this envelope reports."
    }
  },
  "type": "object"
}
```

### `CreateLabelRequest`
```json
{
  "example": {
    "color": "blue",
    "name": "vacation"
  },
  "properties": {
    "color": {
      "description": "Universal color name. Random color assigned if omitted.",
      "enum": [
        "Blue",
        "Charcoal",
        "CoffeeBrown",
        "DarkPurple",
        "Green",
        "LightBlue",
        "Lime",
        "Orange",
        "PineGreen",
        "Pink",
        "Purple",
        "Red",
        "Stone",
        "Turquoise",
        "Wine",
        "Yellow"
      ],
      "type": "string"
    },
    "name": {
      "description": "Label display name",
      "maxLength": 80,
      "type": "string"
    }
  },
  "required": [
    "name"
  ],
  "type": "object"
}
```

### `CreateLabelResponse`
```json
{
  "properties": {
    "agentHints": {
      "$ref": "#/components/schemas/AgentHintsArray"
    },
    "label": {
      "$ref": "#/components/schemas/Label"
    },
    "summary": {
      "$ref": "#/components/schemas/BatchSummary",
      "description": "Standard write summary (A7): total 1, succeeded 1, documentsWritten 1 on the success this envelope reports."
    }
  },
  "type": "object"
}
```

### `CreateRecordRequest`
```json
{
  "example": {
    "accountId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "amount": {
      "value": -42.5
    },
    "categoryId": "5c5c07d0-0014-8000-8000-000000000000",
    "counterParty": "Downtown Bistro",
    "note": "Lunch at restaurant",
    "recordDate": "2025-03-15T14:30:00Z"
  },
  "properties": {
    "accountId": {
      "description": "Account ID to associate the record with",
      "type": "string"
    },
    "amount": {
      "description": "Transaction amount. currencyCode is optional (defaults to account's currency). If provided, must match account's currency.",
      "properties": {
        "currencyCode": {
          "description": "Currency code (e.g. USD). Optional — defaults to account's currency. Must match if provided.",
          "type": "string"
        },
        "value": {
          "description": "Amount value (negative=expense, positive=income). Must not be zero.",
          "type": "number"
        }
      },
      "required": [
        "value"
      ],
      "type": "object"
    },
    "categoryId": {
      "description": "Category ID. System categories use deterministic IDs, custom categories use standard IDs.",
      "type": "string"
    },
    "counterParty": {
      "description": "Payee/merchant name",
      "maxLength": 255,
      "type": "string"
    },
    "labelIds": {
      "items": {
        "type": "string"
      },
      "type": "array"
    },
    "note": {
      "maxLength": 255,
      "type": "string"
    },
    "place": {
      "$ref": "#/components/schemas/PlaceInput",
      "description": "Location data (optional)"
    },
    "recordDate": {
      "description": "Record date in ISO 8601 format. Must not be more than 24 hours in the future.",
      "format": "date-time",
      "type": "string"
    },
    "recordState": {
      "description": "Record state (default: cleared).",
      "enum": [
        "reconciled",
        "cleared",
        "uncleared"
      ],
      "type": "string"
    },
    "transfer": {
      "description": "Account movement settings. Omit for regular records. Requires pairingMode: 'new' (paired, two linked records), 'existing' (pair to existing record), or 'unpaired'.",
      "properties": {
        "accountId": {
          "description": "Target account (pairingMode 'new').",
          "type": "string"
        },
        "counterAmount": {
          "description": "Amount on the counter account (cross-currency 'new' only). Opposite sign to record amount. Omit to auto-convert via FX.",
          "properties": {
            "currencyCode": {
              "description": "Must match counter account's currency code.",
              "type": "string"
            },
            "value": {
              "description": "Numeric amount (opposite sign to record amount).",
              "type": "number"
            }
          },
          "required": [
            "value",
            "currencyCode"
          ],
          "type": "object"
        },
        "pairingMode": {
          "description": "Required. 'new' = create mirror on target account, 'existing' = bind to existing record, 'unpaired' = no mirror.",
          "enum": [
            "new",
            "existing",
            "unpaired"
          ],
          "type": "string"
        },
        "recordId": {
          "description": "Existing record to bind to (pairingMode 'existing'). Target must be on a different account, have opposite amount sign, and not already be paired.",
          "type": "string"
        }
      },
      "required": [
        "pairingMode"
      ],
      "type": "object"
    }
  },
  "required": [
    "accountId",
    "amount",
    "recordDate"
  ],
  "type": "object"
}
```

### `CreateRecordsResponse`
```json
{
  "properties": {
    "results": {
      "description": "Per-item results, one entry per input record",
      "items": {
        "properties": {
          "createdMirrorRecordId": {
            "description": "ID of the auto-created B-side of a paired transfer — present only when THIS call wrote that document (pairingMode \"new\"). Absent when the item is not a paired transfer, or when mirror creation failed (best-effort — a transfer.mirror_failed agent hint reports that case and the item still counts as succeeded). Survives returnData=false; the mirror's body is not returned — read transfer.mirrorRecord on the source, or fetch by this id.",
            "type": "string"
          },
          "error": {
            "description": "Error message (on failure)",
            "type": "string"
          },
          "errorType": {
            "description": "Error classification: client_error (bad input) or server_error (internal failure)",
            "enum": [
              "client_error",
              "server_error"
            ],
            "type": "string"
          },
          "fields": {
            "description": "Input field names the error is attributable to; absent when the error belongs to the item as a whole",
            "items": {
              "type": "string"
            },
            "type": "array"
          },
          "id": {
            "description": "Record ID (present only when created)",
            "type": "string"
          },
          "inputIndex": {
            "description": "0-based index in the input records array",
            "type": "integer"
          },
          "record": {
            "$ref": "#/components/schemas/Record",
            "description": "Full record object (on success)"
          },
          "success": {
            "type": "boolean"
          }
        },
        "type": "object"
      },
      "type": "array"
    },
    "summary": {
      "$ref": "#/components/schemas/BatchSummary"
    }
  },
  "type": "object"
}
```

### `CreateSubcategoryRequest`
```json
{
  "example": {
    "color": "red",
    "name": "Weekend Dining",
    "parentId": "5c5c07d0-0014-8000-8000-000000000000"
  },
  "properties": {
    "cardinality": {
      "description": "Defaults to parent's cardinality",
      "enum": [
        "none",
        "must",
        "need",
        "want"
      ],
      "type": "string"
    },
    "color": {
      "description": "Universal color name. Defaults to parent's color if omitted.",
      "enum": [
        "Blue",
        "Charcoal",
        "CoffeeBrown",
        "DarkPurple",
        "Green",
        "LightBlue",
        "Lime",
        "Orange",
        "PineGreen",
        "Pink",
        "Purple",
        "Red",
        "Stone",
        "Turquoise",
        "Wine",
        "Yellow"
      ],
      "type": "string"
    },
    "name": {
      "description": "Custom subcategory name",
      "maxLength": 80,
      "type": "string"
    },
    "parentId": {
      "description": "Parent base category ID. Use GET /categories to find available parent IDs.",
      "type": "string"
    }
  },
  "required": [
    "name",
    "parentId"
  ],
  "type": "object"
}
```

### `DeleteRequest`
```json
{
  "example": {
    "ids": [
      "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "b2c3d4e5-f6a7-8901-bcde-f12345678901"
    ]
  },
  "properties": {
    "ids": {
      "items": {
        "type": "string"
      },
      "maxItems": 10,
      "minItems": 1,
      "type": "array"
    }
  },
  "required": [
    "ids"
  ],
  "type": "object"
}
```

### `DeleteResponse`
```json
{
  "properties": {
    "agentHints": {
      "$ref": "#/components/schemas/AgentHintsArray"
    },
    "results": {
      "items": {
        "properties": {
          "error": {
            "type": "string"
          },
          "errorType": {
            "description": "Error classification: client_error (bad input) or server_error (internal failure)",
            "enum": [
              "client_error",
              "server_error"
            ],
            "type": "string"
          },
          "fields": {
            "description": "Input field names the error is attributable to; absent for item-level errors",
            "items": {
              "type": "string"
            },
            "type": "array"
          },
          "id": {
            "type": "string"
          },
          "inputIndex": {
            "description": "0-based position of the input item this result answers — the uniform correlation key across every write endpoint.",
            "type": "integer"
          },
          "success": {
            "type": "boolean"
          }
        },
        "type": "object"
      },
      "type": "array"
    },
    "summary": {
      "$ref": "#/components/schemas/BatchSummary"
    }
  },
  "type": "object"
}
```

### `Error`
```json
{
  "example": {
    "error": "Internal Error"
  },
  "properties": {
    "error": {
      "description": "Error message describing what went wrong",
      "type": "string"
    }
  },
  "type": "object"
}
```

### `ExcludedBreakdown`
```json
{
  "properties": {
    "debts": {
      "type": "integer"
    },
    "incomeCategories": {
      "type": "integer"
    },
    "total": {
      "type": "integer"
    },
    "totalAmountSum": {
      "format": "double",
      "nullable": true,
      "type": "number"
    },
    "transfers": {
      "type": "integer"
    },
    "unknownCategories": {
      "type": "integer"
    }
  },
  "type": "object"
}
```

### `Goal`
```json
{
  "properties": {
    "color": {
      "description": "Hex color code",
      "example": "#673ab7",
      "type": "string"
    },
    "createdAt": {
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "type": "string"
    },
    "desiredDate": {
      "description": "Target completion date",
      "example": "2025-12-31T00:00:00.000Z",
      "format": "date-time",
      "type": "string"
    },
    "id": {
      "example": "9f8e7d6c-5b4a-3210-fedc-ba9876543210",
      "type": "string"
    },
    "initialAmount": {
      "allOf": [
        {
          "$ref": "#/components/schemas/AmountWithCurrency"
        }
      ],
      "description": "Initial amount in user's base currency",
      "example": {
        "currencyCode": "USD",
        "value": 500
      }
    },
    "name": {
      "type": "string"
    },
    "note": {
      "type": "string"
    },
    "state": {
      "description": "Goal state",
      "enum": [
        "active",
        "paused",
        "reached"
      ],
      "type": "string"
    },
    "stateUpdatedAt": {
      "description": "When the state was last changed",
      "example": "2025-06-15T10:30:00.000Z",
      "format": "date-time",
      "type": "string"
    },
    "targetAmount": {
      "allOf": [
        {
          "$ref": "#/components/schemas/AmountWithCurrency"
        }
      ],
      "description": "Target amount in user's base currency",
      "example": {
        "currencyCode": "USD",
        "value": 10000
      }
    },
    "updatedAt": {
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "type": "string"
    }
  },
  "type": "object"
}
```

### `Label`
```json
{
  "properties": {
    "archived": {
      "example": false,
      "type": "boolean"
    },
    "color": {
      "description": "Hex color code",
      "example": "#d32f2f",
      "type": "string"
    },
    "createdAt": {
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "type": "string"
    },
    "id": {
      "example": "8f4e8961-3f0e-4c1e-b7a1-3d8f9c5e2a6b",
      "type": "string"
    },
    "name": {
      "type": "string"
    },
    "updatedAt": {
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "type": "string"
    }
  },
  "type": "object"
}
```

### `LabelEmbed`
```json
{
  "properties": {
    "archived": {
      "description": "Whether the label is archived",
      "example": false,
      "type": "boolean"
    },
    "color": {
      "description": "Hex color code",
      "example": "#d32f2f",
      "type": "string"
    },
    "id": {
      "description": "Label ID",
      "example": "8f4e8961-3f0e-4c1e-b7a1-3d8f9c5e2a6b",
      "type": "string"
    },
    "name": {
      "description": "Label name",
      "example": "Important",
      "type": "string"
    }
  },
  "type": "object"
}
```

### `LimitOverrideInput`
```json
{
  "properties": {
    "limit": {
      "description": "New spending limit (must be > 0). Mutually exclusive with setBaseline.",
      "type": "number"
    },
    "period": {
      "description": "Starting period. Format: YYYY-MM (monthly), YYYY-Www (weekly), YYYY (yearly). Must be >= current period.",
      "type": "string"
    },
    "periodCount": {
      "description": "How many periods this override lasts, COUNTING ITS OWN: 1 = this period only. Omit for an open-ended override. Requires limit; not allowed with setBaseline. The server writes the entry that ends the override, restoring the limit actually in force at that period. Rejected if another entry in the same call starts inside the span.",
      "maximum": 120,
      "minimum": 1,
      "type": "integer"
    },
    "setBaseline": {
      "description": "If true, writes an entry holding the budget's current limit, ending any earlier override from this period onward. Entries carry a fixed value, so a later change to the budget's limit does not move it — use resetLimit to clear all overrides instead. Mutually exclusive with limit and with periodCount.",
      "type": "boolean"
    }
  },
  "required": [
    "period"
  ],
  "type": "object"
}
```

### `MirrorRecordEmbed`
```json
{
  "properties": {
    "accountId": {
      "description": "Account ID of the linked transfer record",
      "type": "string"
    },
    "amount": {
      "allOf": [
        {
          "$ref": "#/components/schemas/AmountWithCurrency"
        }
      ],
      "description": "Amount of the linked transfer record",
      "example": {
        "currencyCode": "USD",
        "value": 42.5
      },
      "nullable": true
    },
    "counterParty": {
      "description": "Counter party of the linked transfer record",
      "type": "string"
    },
    "id": {
      "description": "ID of the linked transfer record (other side of the transfer)",
      "type": "string"
    },
    "note": {
      "description": "Note from the linked transfer record",
      "type": "string"
    }
  },
  "type": "object"
}
```

### `PaginatedResponse`
```json
{
  "properties": {
    "limit": {
      "description": "Number of items per page (applied limit)",
      "type": "integer"
    },
    "nextOffset": {
      "description": "Offset for next page (only present if more pages exist)",
      "type": "integer"
    },
    "offset": {
      "description": "Starting position (applied offset)",
      "type": "integer"
    },
    "total": {
      "description": "Total number of items matching the query filters. Only present when withTotal=true is set.",
      "nullable": true,
      "type": "integer"
    }
  },
  "type": "object"
}
```

### `PatchAccountItem`
```json
{
  "example": {
    "archived": false,
    "excludeFromStats": false,
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "initialBalance": 0,
    "name": "My Patched Account"
  },
  "properties": {
    "archived": {
      "type": "boolean"
    },
    "bankAccountNumber": {
      "description": "Bank account number or IBAN (max 80 chars). Empty string clears the value.",
      "maxLength": 80,
      "type": "string"
    },
    "color": {
      "description": "Universal color name.",
      "enum": [
        "Blue",
        "Charcoal",
        "CoffeeBrown",
        "DarkPurple",
        "Green",
        "LightBlue",
        "Lime",
        "Orange",
        "PineGreen",
        "Pink",
        "Purple",
        "Red",
        "Stone",
        "Turquoise",
        "Wine",
        "Yellow"
      ],
      "type": "string"
    },
    "excludeFromStats": {
      "type": "boolean"
    },
    "id": {
      "type": "string"
    },
    "initialBalance": {
      "description": "Initial balance amount (e.g. 1000.00). Omit = no change. INPUT ONLY — it is not returned on the account object; read it back as balance.initial.",
      "type": "number",
      "writeOnly": true
    },
    "name": {
      "maxLength": 80,
      "type": "string"
    }
  },
  "required": [
    "id"
  ],
  "type": "object"
}
```

### `PatchAccountsResponse`
```json
{
  "properties": {
    "agentHints": {
      "$ref": "#/components/schemas/AgentHintsArray"
    },
    "results": {
      "items": {
        "properties": {
          "account": {
            "$ref": "#/components/schemas/Account",
            "description": "Full account object on success (same shape as GET response)"
          },
          "error": {
            "description": "Error message (on failure only)",
            "type": "string"
          },
          "errorType": {
            "description": "Error classification: client_error (bad input) or server_error (internal failure)",
            "enum": [
              "client_error",
              "server_error"
            ],
            "type": "string"
          },
          "fields": {
            "description": "Input field names the error is attributable to; absent when the error belongs to the item as a whole",
            "items": {
              "type": "string"
            },
            "type": "array"
          },
          "id": {
            "type": "string"
          },
          "inputIndex": {
            "description": "0-based position of the input item this result answers — the uniform correlation key across every write endpoint.",
            "type": "integer"
          },
          "success": {
            "type": "boolean"
          }
        },
        "type": "object"
      },
      "type": "array"
    },
    "summary": {
      "$ref": "#/components/schemas/PatchSummary"
    }
  },
  "type": "object"
}
```

### `PatchBudgetRequest`
```json
{
  "properties": {
    "accountIds": {
      "anyOf": [
        {
          "items": {
            "type": "string"
          },
          "type": "array"
        },
        {
          "additionalProperties": false,
          "minProperties": 1,
          "properties": {
            "add": {
              "items": {
                "type": "string"
              },
              "type": "array"
            },
            "remove": {
              "items": {
                "type": "string"
              },
              "type": "array"
            }
          },
          "type": "object"
        }
      ],
      "description": "Account IDs to track. Array = replace all. Object {add, remove} = partial update."
    },
    "categoryIds": {
      "anyOf": [
        {
          "items": {
            "type": "string"
          },
          "type": "array"
        },
        {
          "additionalProperties": false,
          "minProperties": 1,
          "properties": {
            "add": {
              "items": {
                "type": "string"
              },
              "type": "array"
            },
            "remove": {
              "items": {
                "type": "string"
              },
              "type": "array"
            }
          },
          "type": "object"
        }
      ],
      "description": "Category IDs to track (system or custom). Array = replace all. Object {add, remove} = partial update."
    },
    "closed": {
      "description": "true to close (auto-sets closedDate to today UTC); false to reopen (clears closedDate)",
      "type": "boolean"
    },
    "endDate": {
      "description": "End date. Only patchable on BUDGET_CUSTOM.",
      "format": "date",
      "type": "string"
    },
    "id": {
      "description": "Budget ID (UUID)",
      "type": "string"
    },
    "labelIds": {
      "anyOf": [
        {
          "items": {
            "type": "string"
          },
          "type": "array"
        },
        {
          "additionalProperties": false,
          "minProperties": 1,
          "properties": {
            "add": {
              "items": {
                "type": "string"
              },
              "type": "array"
            },
            "remove": {
              "items": {
                "type": "string"
              },
              "type": "array"
            }
          },
          "type": "object"
        }
      ],
      "description": "Label IDs to filter transactions. Array = replace all. Object {add, remove} = partial update."
    },
    "limitOverrides": {
      "description": "Array of limit overrides (max 50), EFFECTIVE-FROM: each entry sets the limit for its period and every period after it, until the next entry takes over. To bound an override to N periods, set periodCount on the entry (periodCount 1 = that period only). Replaces all current+future overrides — one you do not re-send is deleted, not merged, including any part beyond your own entries (past overrides are preserved). Not allowed on BUDGET_CUSTOM.",
      "items": {
        "$ref": "#/components/schemas/LimitOverrideInput"
      },
      "maxItems": 50,
      "type": "array"
    },
    "name": {
      "description": "Budget display name",
      "maxLength": 80,
      "type": "string"
    },
    "resetLimit": {
      "description": "Clears ALL period overrides (past + future), sets uniform baseline limit. Works on all budget types. Can be combined with limitOverrides (resetLimit applies first).",
      "type": "number"
    },
    "startDate": {
      "description": "Start date. Only patchable on BUDGET_CUSTOM.",
      "format": "date",
      "type": "string"
    }
  },
  "required": [
    "id"
  ],
  "type": "object"
}
```

### `PatchBudgetsResponse`
```json
{
  "properties": {
    "agentHints": {
      "$ref": "#/components/schemas/AgentHintsArray"
    },
    "results": {
      "items": {
        "properties": {
          "budget": {
            "$ref": "#/components/schemas/Budget",
            "description": "Full budget object on success"
          },
          "error": {
            "description": "Error message (on failure only)",
            "type": "string"
          },
          "errorType": {
            "description": "Error classification",
            "enum": [
              "client_error",
              "server_error"
            ],
            "type": "string"
          },
          "fields": {
            "description": "Input field names the error is attributable to; absent when the error belongs to the item as a whole",
            "items": {
              "type": "string"
            },
            "type": "array"
          },
          "id": {
            "type": "string"
          },
          "inputIndex": {
            "description": "0-based position of the input item this result answers — the uniform correlation key across every write endpoint.",
            "type": "integer"
          },
          "success": {
            "type": "boolean"
          }
        },
        "type": "object"
      },
      "type": "array"
    },
    "summary": {
      "$ref": "#/components/schemas/PatchSummary"
    }
  },
  "type": "object"
}
```

### `PatchCategoriesResponse`
```json
{
  "properties": {
    "agentHints": {
      "$ref": "#/components/schemas/AgentHintsArray"
    },
    "results": {
      "items": {
        "properties": {
          "category": {
            "$ref": "#/components/schemas/Category",
            "description": "Full category object on success (same shape as GET response)"
          },
          "error": {
            "description": "Error message (on failure only)",
            "type": "string"
          },
          "errorType": {
            "description": "Error classification: client_error (bad input) or server_error (internal failure)",
            "enum": [
              "client_error",
              "server_error"
            ],
            "type": "string"
          },
          "fields": {
            "description": "Input field names the error is attributable to; absent when the error belongs to the item as a whole",
            "items": {
              "type": "string"
            },
            "type": "array"
          },
          "id": {
            "type": "string"
          },
          "inputIndex": {
            "description": "0-based position of the input item this result answers — the uniform correlation key across every write endpoint.",
            "type": "integer"
          },
          "success": {
            "type": "boolean"
          }
        },
        "type": "object"
      },
      "type": "array"
    },
    "summary": {
      "$ref": "#/components/schemas/PatchSummary"
    }
  },
  "type": "object"
}
```

### `PatchCategoryRequest`
```json
{
  "example": {
    "color": "blue",
    "id": "5c5c07d0-0014-8000-8000-000000000000",
    "name": "My Custom Category Name"
  },
  "properties": {
    "cardinality": {
      "description": "New cardinality ('none' to clear)",
      "enum": [
        "none",
        "must",
        "need",
        "want"
      ],
      "type": "string"
    },
    "color": {
      "description": "Universal color name.",
      "enum": [
        "Blue",
        "Charcoal",
        "CoffeeBrown",
        "DarkPurple",
        "Green",
        "LightBlue",
        "Lime",
        "Orange",
        "PineGreen",
        "Pink",
        "Purple",
        "Red",
        "Stone",
        "Turquoise",
        "Wine",
        "Yellow"
      ],
      "type": "string"
    },
    "id": {
      "description": "Category ID (system or custom)",
      "type": "string"
    },
    "name": {
      "description": "New name (sets customName=true). Cannot be used together with reset:[\"name\"].",
      "maxLength": 80,
      "type": "string"
    },
    "reset": {
      "description": "Restore fields to the base (system) category default. Base categories only — custom subcategories have no defaults. A field cannot be both set and reset in the same call. reset:[\"color\"] restores the colour of the category's ENVELOPE GROUP, shared by every category in that group — not the colour this category had before you changed it. A colour that arrived from one of the mobile or web apps outside the 16 universal colours is therefore not recoverable: reset writes the group colour, and the original cannot be written back because color only accepts the 16. Read the current colour first if you may want it again.",
      "items": {
        "enum": [
          "name",
          "cardinality",
          "color"
        ],
        "type": "string"
      },
      "type": "array"
    }
  },
  "required": [
    "id"
  ],
  "type": "object"
}
```

### `PatchLabelItem`
```json
{
  "example": {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "name": "My Patched Label"
  },
  "properties": {
    "archived": {
      "description": "Archive status (true/false)",
      "type": "boolean"
    },
    "color": {
      "description": "Universal color name.",
      "enum": [
        "Blue",
        "Charcoal",
        "CoffeeBrown",
        "DarkPurple",
        "Green",
        "LightBlue",
        "Lime",
        "Orange",
        "PineGreen",
        "Pink",
        "Purple",
        "Red",
        "Stone",
        "Turquoise",
        "Wine",
        "Yellow"
      ],
      "type": "string"
    },
    "id": {
      "description": "Label ID (UUID)",
      "type": "string"
    },
    "name": {
      "description": "Label display name",
      "maxLength": 80,
      "type": "string"
    }
  },
  "required": [
    "id"
  ],
  "type": "object"
}
```

### `PatchLabelsResponse`
```json
{
  "properties": {
    "agentHints": {
      "$ref": "#/components/schemas/AgentHintsArray"
    },
    "results": {
      "items": {
        "properties": {
          "error": {
            "description": "Error message (on failure only)",
            "type": "string"
          },
          "errorType": {
            "description": "Error classification: client_error (bad input) or server_error (internal failure)",
            "enum": [
              "client_error",
              "server_error"
            ],
            "type": "string"
          },
          "fields": {
            "description": "Input field names the error is attributable to; absent when the error belongs to the item as a whole",
            "items": {
              "type": "string"
            },
            "type": "array"
          },
          "id": {
            "type": "string"
          },
          "inputIndex": {
            "description": "0-based position of the input item this result answers — the uniform correlation key across every write endpoint.",
            "type": "integer"
          },
          "label": {
            "$ref": "#/components/schemas/Label",
            "description": "Full label object on success (same shape as GET response)"
          },
          "success": {
            "type": "boolean"
          }
        },
        "type": "object"
      },
      "type": "array"
    },
    "summary": {
      "$ref": "#/components/schemas/PatchSummary"
    }
  },
  "type": "object"
}
```

### `PatchRecordItem`
```json
{
  "example": {
    "categoryId": "5c5c07d0-0014-8000-8000-000000000000",
    "counterParty": "Updated merchant",
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "note": "Updated note"
  },
  "properties": {
    "$clear": {
      "description": "Fields to clear/reset. Example: [\"note\", \"place\"]. 'transfer' cancels transfer state (sets category to Unknown). Cannot be combined with an explicit value for the same field.",
      "items": {
        "enum": [
          "categoryId",
          "labelIds",
          "note",
          "counterParty",
          "place",
          "transfer"
        ],
        "type": "string"
      },
      "type": "array"
    },
    "accountId": {
      "description": "Move record to a different account (UUID). Both accounts must not be bank-synced and must have the same currency.",
      "type": "string"
    },
    "amount": {
      "description": "Transaction amount. Omit = no change. Negative=expense, positive=income. Auto-updates recordType. Patching amount ALWAYS sets the record's currency to the account's currency, regardless of the record's original currency. If amount is omitted from the patch, the original currency is preserved.",
      "properties": {
        "currencyCode": {
          "description": "Reserved for future use. Currently must match the account's currency if provided.",
          "type": "string"
        },
        "value": {
          "description": "Amount value (negative=expense, positive=income). Must not be zero.",
          "type": "number"
        }
      },
      "required": [
        "value"
      ],
      "type": "object"
    },
    "categoryId": {
      "description": "Category ID (system or custom)",
      "type": "string"
    },
    "counterParty": {
      "maxLength": 255,
      "type": "string"
    },
    "id": {
      "type": "string"
    },
    "labelIds": {
      "anyOf": [
        {
          "items": {
            "type": "string"
          },
          "type": "array"
        },
        {
          "additionalProperties": false,
          "minProperties": 1,
          "properties": {
            "add": {
              "items": {
                "type": "string"
              },
              "type": "array"
            },
            "remove": {
              "items": {
                "type": "string"
              },
              "type": "array"
            }
          },
          "type": "object"
        }
      ],
      "description": "Label IDs. Array = replace all (null=no change, []=clear all). Object {add, remove} = partial update (add to / remove from the existing set)."
    },
    "note": {
      "maxLength": 255,
      "type": "string"
    },
    "place": {
      "$ref": "#/components/schemas/PlaceInput",
      "description": "Location data. Omit=no change. Full replace with {address, latitude, longitude, name}. Use $clear to erase."
    },
    "recordDate": {
      "format": "date-time",
      "type": "string"
    },
    "recordState": {
      "description": "Transaction state: reconciled (verified), cleared (processed), uncleared (pending)",
      "enum": [
        "reconciled",
        "cleared",
        "uncleared"
      ],
      "type": "string"
    }
  },
  "required": [
    "id"
  ],
  "type": "object"
}
```

### `PatchRecordsResponse`
```json
{
  "properties": {
    "agentHints": {
      "$ref": "#/components/schemas/AgentHintsArray"
    },
    "results": {
      "items": {
        "properties": {
          "error": {
            "description": "Error message (on failure only)",
            "type": "string"
          },
          "errorType": {
            "description": "Error classification: client_error (bad input) or server_error (internal failure)",
            "enum": [
              "client_error",
              "server_error"
            ],
            "type": "string"
          },
          "fields": {
            "description": "Input field names the error is attributable to; absent when the error belongs to the item as a whole",
            "items": {
              "type": "string"
            },
            "type": "array"
          },
          "id": {
            "type": "string"
          },
          "inputIndex": {
            "description": "0-based position of the input item this result answers — the uniform correlation key across every write endpoint.",
            "type": "integer"
          },
          "record": {
            "$ref": "#/components/schemas/Record",
            "description": "Full record object on success (same shape as GET response)"
          },
          "success": {
            "type": "boolean"
          }
        },
        "type": "object"
      },
      "type": "array"
    },
    "summary": {
      "$ref": "#/components/schemas/PatchSummary"
    }
  },
  "type": "object"
}
```

### `PatchSummary`
```json
{
  "description": "Summary of patch/update operation results",
  "properties": {
    "clientErrors": {
      "description": "Items that failed due to invalid input (client errors)",
      "type": "integer"
    },
    "documentsWritten": {
      "description": "Documents actually written, counted from confirmed write outcomes and deduplicated — independent of item success: an item that failed AFTER its document committed still counts its document, and the error text says what landed. A no-op item (fields already hold the sent values) succeeds with 0 here. Can EXCEED `total` on records: a created mirror, severing, orphaning, converting a target, propagating to a mirror, or rewriting a CRM order document for an orderId bind each write another document, and every one of them is counted here.",
      "type": "integer"
    },
    "serverErrors": {
      "description": "Items that failed due to internal errors (server errors)",
      "type": "integer"
    },
    "succeeded": {
      "description": "Successfully updated items",
      "type": "integer"
    },
    "total": {
      "description": "Total items in request",
      "type": "integer"
    }
  },
  "required": [
    "total",
    "succeeded",
    "clientErrors",
    "serverErrors",
    "documentsWritten"
  ],
  "type": "object"
}
```

### `PlaceInput`
```json
{
  "description": "Location data. Full replace on PATCH (no merge). Send empty object to erase, or use $clear with place.",
  "properties": {
    "address": {
      "description": "Street/full address",
      "maxLength": 255,
      "type": "string"
    },
    "latitude": {
      "description": "GPS latitude",
      "maximum": 90,
      "minimum": -90,
      "type": "number"
    },
    "longitude": {
      "description": "GPS longitude",
      "maximum": 180,
      "minimum": -180,
      "type": "number"
    },
    "name": {
      "description": "Place name",
      "maxLength": 255,
      "type": "string"
    }
  },
  "type": "object"
}
```

### `RateLimitError`
```json
{
  "description": "Returned when rate limit is exceeded. Check `X-RateLimit-Remaining` header before making requests to avoid hitting the limit.",
  "example": {
    "error": "Rate limit exceeded. Please try again later."
  },
  "properties": {
    "error": {
      "type": "string"
    }
  },
  "type": "object"
}
```

### `Record`
```json
{
  "properties": {
    "accountId": {
      "example": "-Account_abc123",
      "type": "string"
    },
    "accountIsBankSync": {
      "nullable": true,
      "type": "boolean"
    },
    "accountName": {
      "description": "Account display name (read-only, derived from account)",
      "example": "Checking Account",
      "type": "string"
    },
    "amount": {
      "allOf": [
        {
          "$ref": "#/components/schemas/AmountWithCurrency"
        }
      ],
      "description": "Transaction amount in its original currency.",
      "example": {
        "currencyCode": "USD",
        "value": -42.5
      }
    },
    "category": {
      "allOf": [
        {
          "$ref": "#/components/schemas/CategoryEmbed"
        }
      ],
      "description": "Full category data embedded in response"
    },
    "convertedAmount": {
      "$ref": "#/components/schemas/ConvertedAmount"
    },
    "counterParty": {
      "description": "Counter party name (payee for expenses, payer for income)",
      "type": "string"
    },
    "createdAt": {
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "type": "string"
    },
    "id": {
      "description": "Unique identifier",
      "example": "550e8400-e29b-41d4-a716-446655440000",
      "type": "string"
    },
    "labels": {
      "description": "Labels attached to this record",
      "items": {
        "$ref": "#/components/schemas/LabelEmbed"
      },
      "type": "array"
    },
    "note": {
      "type": "string"
    },
    "photos": {
      "description": "Photos attached to this record",
      "items": {
        "$ref": "#/components/schemas/RecordPhoto"
      },
      "type": "array"
    },
    "place": {
      "$ref": "#/components/schemas/RecordPlace"
    },
    "recordDate": {
      "description": "Date of the transaction",
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "type": "string"
    },
    "recordState": {
      "description": "Transaction state: reconciled (verified against bank statement), cleared (processed by bank), uncleared (pending), void (cancelled/reversed), waitForAssign (awaiting categorization)",
      "enum": [
        "reconciled",
        "cleared",
        "uncleared",
        "void",
        "waitForAssign"
      ],
      "example": "cleared",
      "type": "string"
    },
    "recordType": {
      "description": "Transaction type: income (money received) or expense (money spent). Derived from the amount sign (amount < 0 = expense); a legacy record with an amount of exactly 0 keeps the type it was stored with.",
      "enum": [
        "income",
        "expense"
      ],
      "example": "expense",
      "type": "string"
    },
    "source": {
      "description": "Client that created this record. android/ios = mobile app, web = web app, rest = REST API, mcp = MCP tool, backend = bank-sync or system-generated. Read-only.",
      "enum": [
        "android",
        "ios",
        "web",
        "rest",
        "mcp",
        "backend",
        "missing"
      ],
      "type": "string"
    },
    "transfer": {
      "allOf": [
        {
          "$ref": "#/components/schemas/TransferOutput"
        }
      ],
      "description": "Transfer state. Null for regular records. 'paired' = a linked mirror record exists between the user's own accounts (mirrorRecord present). 'unpaired' = transfer with no linked mirror — either to/from an external account the user doesn't track, or between own accounts where the link was severed or never established. transferId is the primary transfer identity, present on every transfer: absent = not a transfer, held by one record = unpaired, shared by two = paired. Pairing follows that cardinality. A record whose pair was severed keeps its transferId as a singleton and is simply an unpaired transfer. Read-only, never accepted as input.",
      "nullable": true
    },
    "updatedAt": {
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "type": "string"
    }
  },
  "type": "object"
}
```

### `RecordPhoto`
```json
{
  "properties": {
    "createdAt": {
      "description": "Timestamp when the photo was created",
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "type": "string"
    },
    "temporaryUrl": {
      "description": "Temporary signed URL of the photo (valid for limited time)",
      "type": "string"
    }
  },
  "type": "object"
}
```

### `RecordPlace`
```json
{
  "properties": {
    "address": {
      "type": "string"
    },
    "latitude": {
      "description": "GPS latitude coordinate",
      "format": "double",
      "type": "number"
    },
    "longitude": {
      "description": "GPS longitude coordinate",
      "format": "double",
      "type": "number"
    },
    "name": {
      "type": "string"
    }
  },
  "type": "object"
}
```

### `RecordRule`
```json
{
  "properties": {
    "category": {
      "$ref": "#/components/schemas/CategoryEmbed"
    },
    "createdAt": {
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "type": "string"
    },
    "fromAccountId": {
      "type": "string"
    },
    "id": {
      "example": "f6g7h8i9-j0k1-l2m3-n4o5-p6q7r8s9t0u1",
      "type": "string"
    },
    "keywords": {
      "items": {
        "type": "string"
      },
      "type": "array"
    },
    "labels": {
      "items": {
        "$ref": "#/components/schemas/LabelEmbed"
      },
      "type": "array"
    },
    "name": {
      "type": "string"
    },
    "toAccountId": {
      "type": "string"
    },
    "updatedAt": {
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "type": "string"
    }
  },
  "type": "object"
}
```

### `ReferenceResult`
```json
{
  "properties": {
    "field": {
      "description": "Field name used for the reference",
      "type": "string"
    },
    "hasMore": {
      "description": "True if total exceeds limit",
      "type": "boolean"
    },
    "ids": {
      "description": "IDs of referencing entities (up to limit)",
      "items": {
        "type": "string"
      },
      "type": "array"
    },
    "limit": {
      "description": "Maximum IDs returned",
      "type": "integer"
    },
    "total": {
      "description": "Total count of references",
      "type": "integer"
    }
  },
  "type": "object"
}
```

### `ReferencesResponse`
```json
{
  "description": "Per-ID entry: either the references that point to the target entity, or an error object when the ID is absent or of the wrong type.",
  "properties": {
    "actualType": {
      "description": "On 'type_mismatch', the entity type the ID actually exists under (account, category, record, or label).",
      "type": "string"
    },
    "budgets": {
      "$ref": "#/components/schemas/ReferenceResult"
    },
    "error": {
      "description": "Set when the ID could not be resolved: 'not found' (absent) or 'type_mismatch' (the ID exists but is a different entity type than requested).",
      "type": "string"
    },
    "message": {
      "description": "Human-readable explanation accompanying 'type_mismatch' (names the actual type and notes the ID may be incorrect).",
      "type": "string"
    },
    "recordRules": {
      "$ref": "#/components/schemas/ReferenceResult"
    },
    "records": {
      "$ref": "#/components/schemas/ReferenceResult"
    },
    "standingOrders": {
      "$ref": "#/components/schemas/ReferenceResult"
    }
  },
  "type": "object"
}
```

### `StandingOrder`
```json
{
  "properties": {
    "accountId": {
      "example": "-Account_abc123",
      "type": "string"
    },
    "amount": {
      "example": 1234.56,
      "format": "double",
      "type": "number"
    },
    "categoryId": {
      "example": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "type": "string"
    },
    "counterParty": {
      "type": "string"
    },
    "createdAt": {
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "type": "string"
    },
    "currencyCode": {
      "description": "Currency code (ISO 4217)",
      "example": "USD",
      "type": "string"
    },
    "dueDate": {
      "example": "2025-12-01T00:00:00Z",
      "format": "date-time",
      "type": "string"
    },
    "dueDateNotificationEnabled": {
      "example": true,
      "type": "boolean"
    },
    "generateFromDate": {
      "type": "string"
    },
    "id": {
      "example": "e5f6a7b8-c9d0-4e1a-2f34-a5b6c7d8e9f0",
      "type": "string"
    },
    "labels": {
      "items": {
        "$ref": "#/components/schemas/LabelEmbed"
      },
      "type": "array"
    },
    "manualPayment": {
      "example": false,
      "type": "boolean"
    },
    "name": {
      "type": "string"
    },
    "note": {
      "type": "string"
    },
    "paymentCount": {
      "description": "Number of payment occurrences tracked for this standing order",
      "example": 12,
      "type": "integer"
    },
    "recurrenceRule": {
      "description": "RRULE format",
      "type": "string"
    },
    "reminder": {
      "enum": [
        "due_today",
        "none",
        "1_day",
        "3_days",
        "7_days"
      ],
      "example": "3_days",
      "type": "string"
    },
    "threeDaysBeforeNotificationEnabled": {
      "example": true,
      "type": "boolean"
    },
    "type": {
      "enum": [
        "income",
        "expense"
      ],
      "example": "expense",
      "type": "string"
    },
    "updatedAt": {
      "example": "2021-03-24T09:28:41.200Z",
      "format": "date-time",
      "type": "string"
    }
  },
  "type": "object"
}
```

### `StandingOrderItem`
```json
{
  "properties": {
    "alignedDate": {
      "example": "2025-01-03T00:00:00Z",
      "format": "date-time",
      "type": "string"
    },
    "dismissed": {
      "example": false,
      "type": "boolean"
    },
    "id": {
      "example": "a1b2c3d4-e5f6-4789-abcd-ef0123456789",
      "type": "string"
    },
    "originalDate": {
      "example": "2025-01-01T00:00:00Z",
      "format": "date-time",
      "type": "string"
    },
    "paidDate": {
      "example": "2025-01-01T00:00:00Z",
      "format": "date-time",
      "type": "string"
    },
    "paidFromAppDate": {
      "example": "2025-01-01T00:00:00Z",
      "format": "date-time",
      "type": "string"
    },
    "recordIds": {
      "example": [
        "rec-id-1",
        "rec-id-2"
      ],
      "items": {
        "type": "string"
      },
      "type": "array"
    },
    "standingOrderId": {
      "example": "e5f6a7b8-c9d0-4e1a-2f34-a5b6c7d8e9f0",
      "type": "string"
    }
  },
  "type": "object"
}
```

### `StatDateRange`
```json
{
  "properties": {
    "max": {
      "type": "string"
    },
    "min": {
      "type": "string"
    }
  },
  "type": "object"
}
```

### `TransferOutput`
```json
{
  "properties": {
    "mirrorRecord": {
      "$ref": "#/components/schemas/MirrorRecordEmbed"
    },
    "transferId": {
      "type": "string"
    },
    "type": {
      "type": "string"
    }
  },
  "type": "object"
}
```

### `UnauthorizedError`
```json
{
  "description": "Returned when authentication fails (invalid/expired token or missing Authorization header)",
  "example": {
    "error": "Invalid or expired token"
  },
  "properties": {
    "error": {
      "type": "string"
    }
  },
  "type": "object"
}
```

### `ValidationError`
```json
{
  "description": "Returned when request validation fails. Common errors: invalid filter format, limit must be between 1 and 200.",
  "example": {
    "error": "invalid parameter"
  },
  "properties": {
    "error": {
      "type": "string"
    }
  },
  "type": "object"
}
```

## Components: headers

### `X-Last-Data-Change-At`
```json
{
  "description": "Timestamp of last data modification (ISO 8601 UTC with millisecond precision)",
  "schema": {
    "example": "2024-01-28T14:23:45.123Z",
    "format": "date-time",
    "type": "string"
  }
}
```

### `X-Last-Data-Change-Rev`
```json
{
  "description": "Revision counter for change detection. Compare values to detect data changes between requests.",
  "schema": {
    "example": "r1234",
    "type": "string"
  }
}
```

### `X-RateLimit-Limit`
```json
{
  "description": "Maximum request capacity (hourly limit)",
  "schema": {
    "example": 500,
    "type": "integer"
  }
}
```

### `X-RateLimit-Remaining`
```json
{
  "description": "Remaining request capacity in current window",
  "schema": {
    "example": 487,
    "type": "integer"
  }
}
```

### `X-Sync-In-Progress`
```json
{
  "description": "If `true`, response data is valid but background sync is running — more changes may follow shortly.",
  "schema": {
    "enum": [
      "true",
      "false"
    ],
    "example": "false",
    "type": "string"
  }
}
```

## Components: securitySchemes

### `BearerAuth`
```json
{
  "bearerFormat": "JWT",
  "description": "JWT token obtained from authentication endpoint. The 'sub' claim in the JWT identifies the client_id.",
  "scheme": "bearer",
  "type": "http"
}
```