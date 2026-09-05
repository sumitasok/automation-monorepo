# Interface Contracts: Unified Wallet Sync

**Version**: 1.0  
**Last Updated**: 2026-09-05

---

## CLI Command Contract

### Primary Command: `wallet-sync-unified.sh`

**Invocation**:
```bash
CONFIG_PATH=~/automation-monorepo-config ./scripts/wallet-sync-unified.sh [options]
```

**Environment Variables**:

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `CONFIG_PATH` | path | `~/automation-monorepo-config` | External config directory (required) |
| `DRY_RUN` | bool | `false` | Preview only, no Wallet API writes |
| `AUTO_DATA_DIR` | path | `$CONFIG_PATH/data/expense-domain/wallet` | Override data directory |
| `VERBOSE` | bool | `false` | Enable debug logging |

**Options** (command-line flags):

```
--dry-run          Simulate sync without writing to Wallet API
--since <date>     Override cursor; backfill from <date> (ISO 8601)
--force            Skip idempotency checks; re-fetch all emails
--no-obsidian      Skip Obsidian write-back (continue on vault errors)
--no-bills         Skip Drive bills processing (Part B)
--help             Print usage and exit
--version          Print version and exit
```

**Required Input Files**:

Located in `$CONFIG_PATH/config/expense-domain/wallet/`:

| File | Format | Purpose |
|------|--------|---------|
| `config.yaml` | YAML | API tokens, folder IDs, vault path |
| `routing.yaml` | YAML | Bank/card → accountId mappings |
| `tag-registry.yaml` | YAML | Label definitions (from Obsidian) |
| `email-formats/` (dir) | YAML files | Bank-specific email parsing patterns |

**Config File Structure** (`config.yaml`):

```yaml
expense-domain:
  wallet:
    # Wallet API
    api_token: "${WALLET_API_TOKEN}"  # or read from env
    api_base_url: "https://rest.budgetbakers.com/wallet/v1/api"
    
    # Gmail
    gmail_credentials_path: "~/.config/sa-finances/gmail-credentials.json"
    gmail_token_path: "~/.config/sa-finances/gmail-token.json"
    
    # Google Drive
    drive_bills_folder_id: "1DXizYKYGSg8pPO1_tbXPLTUOENOwfMR6"
    
    # Obsidian
    obsidian_vault_path: "~/sa.finances"
    
    # Behavior
    auto_account_cap: 10
    rate_limit_retries: 3
```

**Output Files**:

Created in `$CONFIG_PATH/data/expense-domain/wallet/`:

| File | Format | Purpose |
|------|--------|---------|
| `last-sync.json` | JSON | Cursor + state (updated atomically) |
| `labels-cache.json` | JSON | Label slug → UUID mapping |
| `logs/sync-<timestamp>.log` | Text | Run log |
| `records-<date>.jsonl` | JSONL | Fetched wallet records (temporary) |
| `dedup-scan-<date>.json` | JSON | Dedup analysis (if enabled) |

**Output Format** (stdout):

```
═══════════════════════════════════════════════════════════════
🎯 WALLET SYNC — Unified
═══════════════════════════════════════════════════════════════

📁 Config: /Users/sumitasok/automation-monorepo-config
🔐 API: https://rest.budgetbakers.com/wallet/v1/api
📅 Mode: Normal (will write to Wallet)

[Part A: Gmail Sync]
📥 Fetching Gmail threads (from:(bank.in))...
✅ Fetched 47 threads
🔍 Scanning for duplicates...
✅ Scan: 47 new, 0 duplicates found
📤 Creating records...
✅ Created: 12 records
⏭️  Skipped: 35 (duplicates or manual)

[Part B: Drive Bills]
📥 Fetching Drive Bills Inbox...
✅ Found 3 new PDFs
🔍 Extracting vendors/amounts...
✅ Extracted: 3 bills
✨ Enriched: 2 records (matched), 1 new record (no match)
💾 Product prices: 8 line items logged

[Part C: Cross-Source Reconciliation]
🔀 Merging records from Parts A & B...
✅ Merged: 2 records (gmail + drive)

[Part D: Label Tagging]
🏷️  Applying labels...
✅ Applied: 12 records × 2-4 labels

[Part E: Obsidian Write-Back]
📝 Updating Obsidian vault...
✅ Updated: 15 rows in 2026-09 September.md
✅ Created: 1 bill note (bill-260905-starbucks.md)

═══════════════════════════════════════════════════════════════
✅ SYNC COMPLETE
═══════════════════════════════════════════════════════════════

📊 Summary:
   Total Fetched: 47
   Created:       12
   Skipped:       35 (0 duplicate, 35 manual)
   Failed:        0
   Duration:      18.3 seconds

💾 State:
   Cursor advanced to: 2026-09-05T14:35:00Z
   Obsidian logs updated: ✅
   Labels applied: ✅
```

**Exit Codes**:

| Code | Meaning | Action |
|------|---------|--------|
| `0` | Success | All transactions synced, cursor advanced |
| `1` | Failure | No changes applied; cursor not advanced |
| `2` | Partial | Some records created, some failed; cursor NOT advanced (requires manual retry) |
| `3` | Config error | Missing/invalid config file |
| `4` | Auth error | Invalid Wallet API token or Gmail credentials |

---

## Python Module Contracts

### Module: `sync.py`

**Entrypoint**:

```python
def main(args: argparse.Namespace) -> int:
    """Execute full wallet sync (Parts A-E).
    
    Args:
        args: Parsed CLI arguments
        
    Returns:
        Exit code (0=success, 1=failure, 2=partial)
    """
```

**Public Functions**:

#### 1. Gmail Sync (Part A)

```python
def fetch_gmail_threads(
    service: googleapiclient.discovery.Resource,
    since_ts: str,
    log: Callable[[str], None]
) -> list[dict]:
    """Fetch Gmail threads from banking senders.
    
    Args:
        service: Gmail API service object
        since_ts: ISO 8601 timestamp (cursor)
        log: Logging function
        
    Returns:
        [
            {
                "thread_id": str,
                "messages": [
                    {
                        "id": str (Gmail message ID),
                        "sender": str,
                        "subject": str,
                        "date": str,
                        "body": str (base64 decoded)
                    }
                ]
            }
        ]
        
    Raises:
        Exception: On Gmail API error
    """
```

#### 2. Extraction Engine

```python
def run_engine(
    envelopes: list[dict],
    log: Callable[[str], None]
) -> list[dict]:
    """Feed envelopes through engine.py.
    
    Args:
        envelopes: [{"source": str, "id": str, "sender": str, "subject": str, "date": str, "body": str}]
        log: Logging function
        
    Returns:
        [
            {
                "action": "skip" | "extract" | "unmatched" | "error",
                "record": {...} if action="extract",
                "error": str if action="error",
                "format_matched": str | None
            }
        ]
    """
```

#### 3. Category Determination

```python
def guess_category_id(
    merchant: str,
    categories: dict[str, str]  # {category_name: category_id}
) -> str | None:
    """Fuzzy-match merchant to category.
    
    Args:
        merchant: Merchant name
        categories: Wallet categories dict
        
    Returns:
        Wallet categoryId (UUID) or None
        
    Logic:
        1. Lowercase merchant
        2. Check if any CATEGORY_HINTS keyword is substring of merchant
        3. Return matching category hint (e.g., "groceries")
        4. Lookup hint in categories dict to find categoryId
        5. Return categoryId or None
    """
```

#### 4. Label Selection

```python
def label_ids_for_record(
    record: dict,  # {"merchant": str, "bank": str, "instrument": str}
    cache: dict[str, str]  # {label_slug: uuid}
) -> list[str]:
    """Pick 2-4 label UUIDs for a transaction.
    
    Args:
        record: Transaction record (merchant, bank, instrument)
        cache: labels-cache.json mapping
        
    Returns:
        [uuid1, uuid2, ...] max 4 unique UUIDs
        
    Logic:
        1. Extract merchant, bank, instrument from record
        2. Match merchant against hardcoded merchant keywords
        3. Match bank/instrument against hardcoded bank/card keywords
        4. Look up slugs in cache to get UUIDs
        5. Deduplicate and return max 4
    """
```

#### 5. Wallet Record Creation

```python
def create_wallet_records(
    records: list[dict],  # NewRecord format
    client: requests.Session,
    token: str,
    dry_run: bool = False,
    log: Callable[[str], None] = print
) -> list[dict]:
    """POST records to Wallet API.
    
    Args:
        records: List of NewRecord objects
        client: HTTP session
        token: Wallet API Bearer token
        dry_run: If True, log but don't POST
        log: Logging function
        
    Returns:
        [
            {
                "success": bool,
                "id": str | None,
                "error": str | None
            }
        ]
        
    Rate limit handling:
        - On HTTP 429: exponential backoff (2s, 4s, 8s)
        - Max 3 retries before failing
        - Log each retry
    """
```

#### 6. Deduplication

```python
def check_duplicate(
    record: dict,
    existing_records: list[dict]
) -> tuple[bool, str]:
    """Check if record is duplicate.
    
    Returns:
        (is_duplicate: bool, reason: str)
        
    Logic:
        1. Check Layer 1: if gm:<msgid> exists in existing records → duplicate
        2. Check Layer 2: if (recordDate, amount, counterParty) matches existing record without gm: → fuzzy duplicate
        3. Return (True, reason) or (False, "")
    """
```

---

### Module: `apply-labels.py`

**Entrypoint**:

```python
def main(args: argparse.Namespace) -> int:
    """Create/refresh labels and optionally re-tag records.
    
    Args:
        args: Parsed CLI arguments (--dry-run, --since, etc.)
        
    Returns:
        Exit code (0=success, 1=failure)
    """
```

**Public Functions**:

```python
def load_labels_cache() -> dict[str, str]:
    """Load or refresh labels-cache.json from Wallet API.
    
    Returns:
        {label_slug: uuid} mapping
        
    Logic:
        1. Fetch all labels from Wallet API
        2. Merge with existing cache
        3. For each slug in Tag Registry that doesn't exist in API, create it
        4. Save updated cache to labels-cache.json
    """

def apply_labels_to_records(
    records: list[dict],
    cache: dict[str, str],
    dry_run: bool = False
) -> int:
    """Batch-patch records with labels.
    
    Args:
        records: Wallet records (fetched via API)
        cache: labels-cache.json mapping
        dry_run: If True, log but don't PATCH
        
    Returns:
        Number of records patched
        
    Rate limit handling:
        - Max 20 records per PATCH request
        - Respect 300 req/hr limit
    """
```

---

## Error Handling Contract

**All errors MUST be logged to `logs/sync-<timestamp>.log` with one of these levels**:

| Level | Meaning | Action |
|-------|---------|--------|
| INFO | Normal progress | Continue |
| WARNING | Recoverable issue (skip record, retry) | Log + continue |
| ERROR | Unrecoverable issue (auth, network) | Log + stop |

**Example Error Messages**:

```
INFO: Fetched 47 Gmail threads
WARNING: Skip gm:abc123 — duplicate (idempotency key found)
WARNING: Skip gm:def456 — fuzzy duplicate (same date/amount/merchant)
ERROR: Gmail API 401 Unauthorized — check credentials
ERROR: Wallet API 503 Service Unavailable — retrying (1/3)...
ERROR: Max retries exceeded — abort
```

---

## Idempotency Contract

**All operations are idempotent**:

- ✅ Running sync twice with same data: no duplicates created
- ✅ Re-running with cursor in middle of batch: picks up where left off
- ✅ Network interrupt mid-batch: cursor not advanced; safe to retry
- ✅ Obsidian write-back fails: cursor still advanced (bill data preserved in Wallet)

**Idempotency Keys**:

| Entity | Key |
|--------|-----|
| Gmail transaction | `gm:<gmail-message-id>` in record note |
| Drive bill | `drive:<file-id>` in record note |
| Obsidian expense log row | `gm:` or `drive:` ref in Notes column |
| Processed Drive file | File ID in `last-sync.json.processed_drive_files` |

---

## Rollback/Recovery Contract

**If sync fails**:

1. ✅ `last-sync.json` cursor is NOT advanced
2. ✅ `last-sync.json.last_run_status` = "failed"
3. ✅ Error message saved to `last-sync.json.last_run_error`
4. ✅ Backup created: `last-sync.json.backup.<timestamp>`
5. ✅ User can retry same command; sync resumes from last cursor

**If Obsidian write-back fails**:

1. ✅ Wallet records ARE created (primary goal achieved)
2. ⚠️ Obsidian logs are NOT updated
3. ⚠️ `last-sync.json.notes` logs Obsidian error (user can re-sync write-back later)
4. ✅ Sync continues (doesn't abort)

---

## Rate Limit Contract

**Wallet API**: 300 requests/hour

**Sync batching**:
- Gmail API: List threads (1 req) + Get each thread (N reqs) = N+1 reqs
- Wallet API:
  - List existing records (dedup check): 1 req per date range
  - Create records: batched by 20 (so 50 records = 3 POST reqs)
  - Patch records (enrichment): batched by 20
- Total for 100 new transactions: ~10 API calls

**Retry logic on 429**:
```
Attempt 1: Wait 2s, retry
Attempt 2: Wait 4s, retry
Attempt 3: Wait 8s, retry
Max: 3 retries; if still 429, abort
```

