# Data Model: Unified Wallet Sync

**Version**: 1.0  
**Last Updated**: 2026-09-05

---

## Core Entities

### Entity: Wallet Transaction Record

**Purpose**: Represents a single financial transaction synced to BudgetBakers Wallet API.

**Fields**:

| Field | Type | Constraints | Source |
|-------|------|-----------|--------|
| `id` | UUID | Wallet-assigned, immutable | Wallet API response |
| `accountId` | UUID | Required; must exist in Wallet | routing.yaml |
| `amount` | float | Negative = expense, positive = income; 2 decimal places | Gmail/Drive extraction |
| `currency` | string | ISO 4217 code (INR, USD, etc.) | Email body |
| `recordDate` | ISO 8601 | Date-only or timestamp; stored at 12:00:00Z if date-only | Email/receipt date |
| `paymentType` | string | "Card", "UPI", "Cash", "Cheque", etc. | Extracted from email/receipt |
| `counterParty` | string | Merchant/vendor name; max 255 chars; case-insensitive for dedup | Email body |
| `categoryId` | UUID | Nullable; mapped via CATEGORY_HINTS → Wallet categories | Fuzzy-matched from merchant |
| `labelIds` | [UUID] | Array of 2–4 label IDs; max 4 per Wallet API | Hardcoded keyword match |
| `note` | string | Format: `<merchant> \| via <instrument> \| gm:<msgid> \| source:refactored-code-0905` (max 255 chars) | Constructed in sync.py |
| `createdAt` | timestamp | Wallet-assigned on creation | Wallet API response |
| `updatedAt` | timestamp | Updated on patches | Wallet API response |

**Deduplication Key (Composite)**:
- Layer 1 (Idempotency): `gm:<gmail-message-id>` in note (primary)
- Layer 2 (Fuzzy): (`recordDate`, `amount`, `counterParty` case-insensitive) if no gm: tag exists

**Validation Rules**:
- `amount` must be non-zero
- `currency` must be valid ISO 4217
- `counterParty` must be non-empty after trimming
- `note` must contain `source:refactored-code-0905` tag (mandatory)
- `labelIds` array max length = 4
- `recordDate` must be valid ISO 8601

**State Transitions**:
- **Pending**: Created in sync batch, awaiting API response
- **Created**: Successfully written to Wallet API
- **Skipped**: Duplicate (gm: found) or manual duplicate detected
- **Patched**: Updated with enrichment (e.g., drive: tag, category from bill)
- **Failed**: API call failed; logged in sync report

---

### Entity: Email Envelope (Gmail → Engine)

**Purpose**: Standardized format for passing Gmail data to extraction engine.

**Fields**:

| Field | Type | Example |
|-------|------|---------|
| `source` | string | "gmail" |
| `id` | string | Gmail message ID (e.g., "abc123def456") |
| `sender` | string | "alerts@hdfcbank.com" |
| `subject` | string | "Credit Card - Transaction Alert" |
| `date` | string | RFC 2822 format; parsed to ISO 8601 |
| `body` | string | HTML or plain text; base64 decoded from Gmail payload |

**Validation**:
- All fields required; fail if any null
- `id` must be non-empty
- `body` must be non-empty (skip if empty)

**Lifecycle**:
1. Fetched from Gmail API via `fetch_gmail_threads()`
2. Passed to `run_engine()` as batch JSONL
3. Engine returns structured transaction or "unmatched" action

---

### Entity: Engine Result

**Purpose**: Output of deterministic extraction (engine.py) or AI fallback.

**Fields**:

| Field | Type | Values | Usage |
|-------|------|--------|-------|
| `action` | string | "skip" \| "extract" \| "unmatched" \| "error" | Routing signal |
| `record` | object | Structured transaction (see below) | If action = "extract" |
| `error` | string | Error message | If action = "error" |
| `format_matched` | string | Regex pattern name (e.g., "email.hdfc.alertv1") | Audit; null if AI-parsed |

**Record Object** (when action = "extract"):

| Field | Type | Notes |
|-------|------|-------|
| `merchant` | string | Extracted from email body |
| `amount` | float | Positive; negation applied based on direction ("debit" → negative) |
| `date` | string | YYYY-MM-DD date-only |
| `currency` | string | ISO 4217 |
| `card_last4` | string | Last 4 digits; optional |
| `bank` | string | Identified bank/card issuer |
| `instrument` | string | "Credit Card X3690", "UPI", "Savings Account", etc. |
| `accountId` | string | UUID from routing.yaml for this bank |
| `category_hint` | string | Optional; merchant keyword category (e.g., "groceries") |

**Action Routing**:
- **"skip"**: Email is OTP, promo, or statement; discard
- **"extract"**: Known format matched; use `record` directly
- **"unmatched"**: Format unknown; requires AI parsing + format codification
- **"error"**: Known format is broken (regex no longer matches); fail and report

---

### Entity: Sync State (last-sync.json)

**Purpose**: Cursor + audit trail for idempotent re-runs.

**Fields**:

```json
{
  "last_email_timestamp": "2026-09-05T14:30:00Z",
  "last_run_start": "2026-09-05T14:35:00Z",
  "last_run_end": "2026-09-05T14:35:18Z",
  "last_run_status": "success",
  "last_run_summary": {
    "total_fetched": 47,
    "created": 12,
    "skipped": 35,
    "failed": 0
  },
  "processed_drive_files": [
    "file-id-abc123",
    "file-id-def456"
  ],
  "auto_created_accounts": [
    {
      "id": "uuid-xyz",
      "name": "New Bank CC",
      "bank": "newbank",
      "created_at": "2026-09-05T10:00:00Z"
    }
  ]
}
```

**Update Rules**:
- `last_email_timestamp`: Advance ONLY on success (all-or-nothing)
- `last_run_status`: Set based on outcome ("success", "partial", "failed")
- `processed_drive_files`: Append ONLY if file processed successfully
- `auto_created_accounts`: Append on account creation; stop at 10 entries
- Back up entire file before write (atomic overwrite)

**Constraints**:
- `auto_created_accounts` length ≤ 10; reject creation if ≥ 10 without approval
- File must be valid JSON after write (validate before commit)

---

### Entity: Label Cache (labels-cache.json)

**Purpose**: Runtime mapping of label slugs to Wallet API UUIDs.

**Structure**:

```json
{
  "blinkit": "uuid-1",
  "licious": "uuid-2",
  "zomato": "uuid-3",
  "swiggy": "uuid-3",
  "food-delivery": "uuid-3",
  "netflix": "uuid-4",
  "subscriptions": "uuid-4",
  "amazon": "uuid-5",
  "shopping": "uuid-6",
  "uber": "uuid-7",
  "ola": "uuid-7",
  "transport": "uuid-7",
  "hdfc": "uuid-8",
  "canara": "uuid-9",
  "icici": "uuid-10",
  "groceries": "uuid-11",
  "restaurants": "uuid-12",
  "healthcare": "uuid-13",
  "utilities": "uuid-14",
  "fuel": "uuid-15"
}
```

**Population Rules**:
- On first run: Fetch all labels from Wallet API + merge into cache
- Create missing labels for slugs in Tag Registry
- Preserve existing mappings on subsequent runs
- Refresh on demand via `apply-labels.py`

**Validation**:
- Every value must be a valid UUID
- No duplicate slug keys

---

### Entity: Account Routing (routing.yaml)

**Purpose**: Deterministic mapping of bank/card identifiers to Wallet accountIds.

**Structure**:

```yaml
accounts:
  - bank: "HDFC"
    card_last4: "3690"
    name: "HDFC Credit Card x3690"
    account_id: "97320818-c6df-4fbc-be24-baa5fbea7cc5"
    account_type: "CreditCard"

  - bank: "HDFC"
    card_last4: null
    name: "HDFC Savings x3176"
    account_id: "6cf80ab9-85bd-420a-aec4-8498005f4ce8"
    account_type: "SavingAccount"

  - bank: "Canara"
    card_last4: "6102"
    name: "Canara Credit Card x6102 (Chinju)"
    account_id: "e6f8c8a4-72d5-44a8-b1e0-5e8f3c4d9a2b"
    account_type: "CreditCard"

  # ... more entries
```

**Match Logic**:
1. Extract bank name from email (sender/subject parsing)
2. Extract card_last4 if present
3. Match bank first; if card_last4 specified in routing, also match card_last4
4. Return accountId; fail if no match

**Validation**:
- Every `account_id` must be a valid UUID
- Every `account_type` must be known in Wallet (CreditCard, SavingAccount, etc.)
- No duplicate (bank, card_last4) pairs

---

### Entity: Email Formats (formats/email.<bank>.yaml)

**Purpose**: Regex patterns for deterministic parsing of known email formats.

**Structure**:

```yaml
bank: "hdfc"
version: "1.0"
patterns:
  - name: "hdfc_cc_alert_v1"
    sender: "alerts@hdfcbank.com"
    subject_regex: "Credit Card.*Transaction"
    body_patterns:
      - field: "merchant"
        regex: "Establishment Details.*?:.*?([A-Z ]+)"
      - field: "amount"
        regex: "Amount\\s*:\\s*(?:Rs\\.?|₹)?\\s*([0-9,]+\\.?[0-9]*)"
      - field: "date"
        regex: "Date & Time.*?:.*?([0-9]{2}-[A-Z]{3}-[0-9]{4})"
      - field: "card_last4"
        regex: "Card Number.*?([0-9]{4})"
    output:
      merchant_field: "merchant"
      amount_field: "amount"
      date_format: "%d-%b-%Y"
      currency: "INR"
      direction: "debit"  # or "credit"
    test_samples:
      - path: "tests/samples/email.hdfc/alert-20260901.txt"
        expected_merchant: "AMAZON INDIA RETAIL"
        expected_amount: "1500"
```

**Validation**:
- All `body_patterns[].regex` must be valid Python re syntax
- `date_format` must be valid strptime format
- Test samples must exist and pass when pattern is applied

---

### Entity: CATEGORY_HINTS (in sync.py)

**Purpose**: Hardcoded merchant keyword → category slug mapping.

**Structure**:

```python
CATEGORY_HINTS = {
    # merchant keywords → Wallet category name keyword
    "blinkit":        "groceries",
    "licious":        "groceries",
    "zomato":         "restaurants",
    "swiggy":         "restaurants",
    "netflix":        "subscriptions",
    "youtube":        "subscriptions",
    "spotify":        "subscriptions",
    "apple":          "subscriptions",
    "google play":    "subscriptions",
    "amazon":         "shopping",
    "meesho":         "shopping",
    "decathlon":      "shopping",
    "uber":           "transport",
    "ola":            "transport",
    "rapido":         "transport",
    "fastag":         "transport",
    "irctc":          "transport",
    "indigo":         "transport",
    "hospital":       "healthcare",
    "pharmacy":       "healthcare",
    "1mg":            "healthcare",
    "apollo":         "healthcare",
    "eureka forbes":  "home",
    "urban company":  "home",
    "mygate":         "home",
    "livpure":        "utilities",
    "bescom":         "utilities",
    "fuel":           "fuel",
    "petrol":         "fuel",
}
```

**Lookup Logic**:
1. Convert merchant name to lowercase
2. Check if any keyword is substring of merchant (case-insensitive)
3. Return category slug (e.g., "groceries")
4. Use slug to look up in Wallet's category list to find categoryId

**Update Process**:
- Edit CATEGORY_HINTS dict in sync.py
- Re-run apply-labels.py to re-tag historical records
- Document change in commit message

---

## Relationships

```
Email (Gmail)
  ↓ (fetch via Gmail API)
Email Envelope
  ↓ (run through engine.py)
Engine Result
  ├─ "extract" path → Transaction Record (created)
  ├─ "unmatched" path → AI parse + format codification → Transaction Record (created)
  ├─ "skip" path → discarded
  └─ "error" path → logged, human review needed

Transaction Record
  ├─ Labels: label-id ← [Label Cache] ← CATEGORY_HINTS + hardcoded keywords
  ├─ Category: category-id ← [CATEGORY_HINTS] ← merchant lookup
  ├─ Account: accountId ← [Account Routing] ← bank/card lookup
  └─ Audit: gm:<msgid> | source:refactored-code-0905 (in note)

Sync State
  ├─ Cursor: last_email_timestamp (Gmail API after: parameter)
  ├─ Processed Files: [drive-file-ids] (Drive dedup)
  └─ Auto-Created Accounts: [account-ids] (cap at 10)

Drive Bill (PDF/image)
  ↓ (extract via engine.py or AI-OCR)
Transaction Record (enriched)
  ├─ drive:<fileId> appended to note
  ├─ Bill Note created in Obsidian
  └─ Product Prices → product-prices.jsonl
```

---

## File Storage Locations

| Entity | Location | Format | Scope |
|--------|----------|--------|-------|
| Transaction Records (fetched) | `$CONFIG_PATH/data/.../records-<date>.jsonl` | JSONL | Temporary (per run) |
| Sync State | `$CONFIG_PATH/data/.../last-sync.json` | JSON | Persistent (across runs) |
| Label Cache | `$CONFIG_PATH/data/.../labels-cache.json` | JSON | Persistent |
| Account Routing | `$CONFIG_PATH/config/.../routing.yaml` | YAML | Persistent (user config) |
| Email Formats | `$CONFIG_PATH/config/.../formats/email.<bank>.yaml` | YAML | Persistent (evolves) |
| Logs | `$CONFIG_PATH/data/.../logs/sync-<timestamp>.log` | Text | Persistent (archive) |
| Obsidian Logs | `~/sa.finances/Expenses/<year>/<YYYY-MM Month>.md` | Markdown | Persistent |
| Bill Notes | `~/sa.finances/Expenses/<year>/Bills/bill-<date>-<slug>.md` | Markdown | Persistent |
| Product Prices | `~/sa.finances/Expenses/<year>/product-prices.jsonl` | JSONL | Persistent (append-only) |

