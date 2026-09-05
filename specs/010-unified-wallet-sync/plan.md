# Implementation Plan: Unified Wallet Sync

**Feature**: 010-unified-wallet-sync  
**Branch**: feature/restructure-architecture  
**Created**: 2026-09-05  
**Status**: Phase 0 (Research) → Phase 1 (Design)

---

## Technical Context

| Component | Status | Details |
|-----------|--------|---------|
| **Gmail extraction** | ✅ Clear | Engine-first with AI fallback; patterns in `formats/email.<bank>.yaml` |
| **Label selection** | ✅ Clear | Hardcoded merchant/bank keywords → labels-cache.json lookup |
| **Category mapping** | ✅ Clear | CATEGORY_HINTS dict (hardcoded) + fuzzy lookup in Wallet categories |
| **Drive Bills sync** | ✅ Clear | PDF extraction (pdftotext/pdfplumber) + engine.py parsing |
| **Obsidian write-back** | ✅ Clear | Monthly logs at `Expenses/<year>/<YYYY-MM Month>.md` |
| **Cross-source merge** | ✅ Clear | Richest-first logic; append tags, never replace |
| **Framework integration** | ✅ Clear | Single CONFIG_PATH, auto-discovery, no hardcoded paths |
| **Launchd trigger** | ✅ Clear | Update com.safinances.plist, disable com.sumitasok.plist |

**All clarifications resolved in prior `/speckit-clarify` session**

---

## Constitution Check (Pre-Design Gates)

**Alignment with Automation Workspace Constitution**:

✅ **Principle I: Packs Declare, Workspace Supplies**
- Pack declares needs in manifest.yaml: WALLET_API_TOKEN, Gmail credentials, Drive folder ID, Obsidian vault path
- Workspace supplies via CONFIG_PATH injection (~/automation-monorepo-config/)
- No hardcoded paths in code; all derived from CONFIG_PATH
- **Implementation**: All config reads use environ.get() with CONFIG_PATH prefix

✅ **Principle II: packs/ Is Read-Only**
- Pack contains: Python code (sync.py, apply-labels.py, extract-engine.py), formats/, routing.yaml template, manifest.yaml, schemas
- No secrets, no output data, no state files generated in packs/
- All state (last-sync.json, labels-cache.json, logs) → ~/automation-monorepo-config/data/
- **Implementation**: All writes go to external config path, never packs/

✅ **Principle IV: Derived Artifacts Regenerate, Never Drift**
- Email format codification (formats/email.<bank>.yaml) is re-run on every sync
- Labels-cache regenerates on first run + stays in sync with Wallet API
- last-sync.json cursor advances atomically (all-or-nothing)
- **Implementation**: No caching of credentials, formats reload on each sync

✅ **Principle V: Configuration Over Code**
- Account routing: routing.yaml (no hardcoding in code)
- Email formats: YAML patterns in formats/email.<bank>.yaml (not regex in code)
- Tag registry: Tag Registry.md + labels-cache.json (loaded at runtime)
- **Implementation**: All YAML files loaded fresh on each run, no bundled defaults

✅ **Principle VII: Local-First, Least Exposure**
- Sync runs locally with no external cron/launchd from third-party services
- Credentials never logged or exposed
- Scope-limited Gmail API scopes (labels, modify only for claude-read)
- **Implementation**: All sensitive data logged only to local file at INFO level

**No gate violations detected. Proceeding to design.**

---

## Phase 0: Research Tasks

**Status**: ✅ Complete (no unresolved unknowns from clarifications session)

All critical ambiguities resolved:
- Gmail extraction: Two-stage (API fetch + engine-first parsing) 
- Label identification: Hardcoded keywords + cache lookup
- Category determination: CATEGORY_HINTS dict + Wallet category fuzzy-find
- Framework integration: CONFIG_PATH single variable
- Configuration structure: routing.yaml, formats/, labels-cache.json

No new research tasks generated; proceeding directly to Phase 1 design.

---

## Phase 1: Design

### 1. Data Model

**Entity: Transaction Record**
```
{
  id: string (UUID, Wallet API assigned)
  accountId: string (UUID, from routing.yaml)
  amount: float (negative = expense)
  currency: string (ISO 4217, e.g., "INR", "USD")
  recordDate: string (ISO 8601, e.g., "2026-09-05T00:00:00Z")
  paymentType: string (e.g., "Card", "UPI")
  counterParty: string (merchant name, max 255 chars)
  categoryId: string (UUID, nullable)
  labelIds: [string] (UUID array, max 4)
  note: string (format: "<merchant> | via <instrument> | gm:<msgid> | source:refactored-code-0905", max 255 chars)
  sourceId: string (internal, gm:<msgid> or drive:<fileId> for dedup)
  createdAt: timestamp
  updatedAt: timestamp
}
```

**Entity: Email Envelope (Gmail → Engine)**
```
{
  source: "gmail"
  id: string (Gmail message ID)
  sender: string (from: header)
  subject: string (subject: header)
  date: string (date: header)
  body: string (base64 decoded HTML/plain)
}
```

**Entity: Engine Result**
```
{
  action: "skip" | "extract" | "unmatched" | "error"
  record: {
    merchant: string
    amount: float
    date: string (YYYY-MM-DD)
    currency: string
    card_last4: string (optional)
    bank: string
    instrument: string
    accountId: string (via routing.yaml)
  }
  error: string (if action: "error")
}
```

**Entity: Sync State (last-sync.json)**
```
{
  last_email_timestamp: string (ISO 8601, cursor)
  last_run_start: timestamp
  last_run_status: "success" | "partial" | "failed"
  last_run_summary: {
    total_fetched: number
    created: number
    skipped: number
    failed: number
  }
  processed_drive_files: [string] (Drive file IDs, for idempotency)
  auto_created_accounts: [
    {
      id: string (UUID)
      name: string
      created_at: timestamp
    }
  ]
}
```

**Entity: Label Cache (labels-cache.json)**
```
{
  "blinkit": "uuid-1",
  "licious": "uuid-2",
  "food-delivery": "uuid-3",
  "hdfc": "uuid-4",
  "canara": "uuid-5",
  "icici": "uuid-6",
  ... (slug → UUID mapping)
}
```

### 2. Interface Contracts

#### 2.1 CLI Command Contract

**Command**: `CONFIG_PATH=~/automation-monorepo-config wallet-sync-unified.sh`

**Environment**:
- `CONFIG_PATH`: Path to external config directory (default: ~/automation-monorepo-config)
- `AUTO_DATA_DIR`: (optional) Override data directory location
- `DRY_RUN`: (optional) "true" to preview without modifying Wallet API

**Input Files** (from `CONFIG_PATH/config/expense-domain/wallet/`):
- `config.yaml`: Contains `WALLET_API_TOKEN`, `GMAIL_CREDS_PATH`, `DRIVE_FOLDER_ID`, `OBSIDIAN_VAULT_PATH`
- `routing.yaml`: Bank/card → accountId mappings
- `tag-registry.yaml`: Label definitions
- `email-formats/`: Directory of `email.<bank>.yaml` pattern files

**Output Files** (to `CONFIG_PATH/data/expense-domain/wallet/`):
- `last-sync.json`: Cursor + state (updated atomically)
- `labels-cache.json`: slug → UUID mapping
- `logs/sync-<timestamp>.log`: Run log
- `records-<date>.jsonl`: Fetched wallet records
- `product-prices.jsonl`: (appended to) for bills

**Output (stdout)**:
```
🚀 Running: wallet-sync-unified
   Config: /Users/sumitasok/automation-monorepo-config
   Domain: expense-domain
   
[progress output]

✅ COMPLETE — Synced 47 transactions
   Created: 12 | Skipped: 35 (duplicates/manual)
   Duration: 18.3s
```

**Exit Code**:
- `0`: Success
- `1`: Failure (no changes applied)
- `2`: Partial failure (some records created, some failed)

#### 2.2 Python Module Contracts

**Module**: `packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/sync.py`

**Public Functions**:
```python
def main(args: argparse.Namespace) -> dict:
    """Execute full wallet sync (Parts A-E).
    
    Returns:
    {
      "status": "success" | "failed" | "partial",
      "stats": {
        "fetched": int,
        "created": int,
        "skipped": int,
        "failed": int
      },
      "errors": [str]
    }
    """

def fetch_gmail_threads(service, since_ts: str) -> list[dict]:
    """Fetch Gmail threads from senders matching 'from:(bank.in)'.
    
    Returns: [{"thread_id": str, "messages": [{"id", "sender", "subject", "date", "body"}]}]
    """

def run_engine(envelopes: list[dict]) -> list[dict]:
    """Feed envelopes through engine.py.
    
    Returns: [{"action": "skip|extract|unmatched|error", "record": {...}}]
    """

def guess_category_id(merchant: str, categories: dict) -> str | None:
    """Fuzzy-match merchant to CATEGORY_HINTS, lookup in Wallet categories."""

def label_ids_for_record(record: dict, cache: dict) -> list[str]:
    """Pick 2-4 label UUIDs from hardcoded merchant/bank keywords."""
```

**Module**: `packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/apply-labels.py`

**Public Functions**:
```python
def main(args: argparse.Namespace) -> dict:
    """Create missing labels + optionally re-tag historical records.
    
    Returns: {"created": int, "patched": int}
    """

def load_labels_cache() -> dict:
    """Load or refresh labels-cache.json from Wallet API."""

def apply_labels_to_records(records: list[dict], cache: dict, dry_run=False) -> int:
    """Batch-patch records with correct labels. Returns count patched."""
```

### 3. Quickstart Validation Guide

**Goal**: Demonstrate that unified wallet sync works end-to-end.

#### 3.1 Prerequisites

```bash
# 1. Ensure config exists
ls -la ~/automation-monorepo-config/config/expense-domain/wallet/
# Expected: config.yaml, routing.yaml, tag-registry.yaml, email-formats/ directory

# 2. Ensure Obsidian vault is accessible
ls -la ~/sa.finances/Expenses/
# Expected: Year folders (2026/, etc.) and monthly logs

# 3. Verify Gmail API credentials
ls -la ~/.config/sa-finances/
# Expected: gmail-credentials.json, gmail-token.json

# 4. Check Wallet API token
grep WALLET_API_TOKEN ~/automation-monorepo-config/config/wallet/config.yaml
# Expected: Token present, not empty
```

#### 3.2 Validation Scenario 1: Fresh Sync (No Existing Records)

**Setup**:
1. Clear `last-sync.json` cursor: `{"last_email_timestamp": "2026-09-04T00:00:00Z", ...}`
2. Send a test bank alert email to sumitasok@gmail.com from HDFC (or any known bank)
3. Wait 2–5 minutes for email to arrive in Gmail

**Run**:
```bash
CONFIG_PATH=~/automation-monorepo-config ./scripts/wallet-sync-unified.sh
```

**Expected Outcomes**:
1. ✅ Script runs without errors
2. ✅ Transaction appears in Wallet app within 60 seconds
3. ✅ Record contains gm:<message-id> in note
4. ✅ Record contains source:refactored-code-0905 in note
5. ✅ Record has appropriate labels applied (e.g., "groceries" + "hdfc")
6. ✅ New row appears in `Expenses/2026/2026-09 September.md`
7. ✅ `last-sync.json` cursor advanced to email timestamp

**Validation Commands**:
```bash
# Check Wallet record
curl -s -H "Authorization: Bearer $WALLET_TOKEN" \
  "https://rest.budgetbakers.com/wallet/v1/api/records?limit=5&offset=0" \
  | jq '.records[] | select(.note | contains("source:refactored-code-0905"))'

# Check Obsidian expense log
grep "gm:" ~/sa.finances/Expenses/2026/2026-09*

# Check sync state
cat ~/automation-monorepo-config/data/expense-domain/wallet/last-sync.json | jq .last_run_status
```

#### 3.3 Validation Scenario 2: Dedup (Re-run Same Sync)

**Setup**: Run the sync script again without changing last-sync.json cursor

**Run**:
```bash
CONFIG_PATH=~/automation-monorepo-config ./scripts/wallet-sync-unified.sh
```

**Expected Outcomes**:
1. ✅ Script detects existing records via gm: idempotency key
2. ✅ No duplicate records created in Wallet
3. ✅ Output shows "skipped: 1 (already synced)"
4. ✅ Obsidian expense log row unchanged (already has gm: ref)

#### 3.4 Validation Scenario 3: Drive Bill Upload

**Setup**:
1. Create test receipt PDF (or use existing one)
2. Upload to "Bills Inbox" folder in Google Drive

**Run**:
```bash
CONFIG_PATH=~/automation-monorepo-config ./scripts/wallet-sync-unified.sh
```

**Expected Outcomes**:
1. ✅ Bill PDF extracted: vendor, date, line items, total
2. ✅ Matched to Wallet transaction (same day ±3 days, similar amount)
3. ✅ Wallet record enriched with drive:<fileId> in note + category
4. ✅ Bill note created at `Expenses/2026/Bills/bill-<date>-<vendor>.md`
5. ✅ Product prices appended to product-prices.jsonl

#### 3.5 Validation Scenario 4: Label Application

**Setup**: Ensure labels-cache.json is populated

**Run**:
```bash
CONFIG_PATH=~/automation-monorepo-config ./scripts/wallet-sync-unified.sh
```

**Check Labels**:
```bash
# Fetch record and check labelIds
curl -s -H "Authorization: Bearer $WALLET_TOKEN" \
  "https://rest.budgetbakers.com/wallet/v1/api/records?limit=1" \
  | jq '.records[0].labelIds'

# Expected: Array of 2–4 UUIDs
# e.g., ["uuid-groceries", "uuid-hdfc"]
```

#### 3.6 Validation Scenario 5: Hourly Schedule (Launchd)

**Setup**: Verify com.safinances.wallet-sync.plist is installed

**Run**:
```bash
# Check if plist is loaded
launchctl list | grep com.safinances.wallet-sync

# Wait for hourly trigger (next :07 minute)
# Monitor logs
tail -f ~/automation-monorepo-config/data/expense-domain/wallet/logs/sync-*.log
```

**Expected Outcomes**:
1. ✅ launchctl shows plist loaded
2. ✅ Sync runs automatically every hour at :07 minute
3. ✅ Log shows "✅ COMPLETE" message

---

## Implementation Roadmap

### Phase 2: Implementation (Tasks)

*See `tasks.md` for detailed task breakdown across 5 phases:*
1. **Phase 1**: Migrate Obsidian code to repo
2. **Phase 2**: Integrate with framework
3. **Phase 3**: Feature implementation (Parts A–E)
4. **Phase 4**: Unified trigger configuration
5. **Phase 5**: Cutover & cleanup

### Phase 3: Validation

Run all 5 validation scenarios above to confirm end-to-end functionality.

### Phase 4: Merge & Cutover

1. Merge feature/restructure-architecture → main
2. Update com.safinances.plist on production machine
3. Disable com.sumitasok.plist
4. Monitor first 3 hourly runs

---

## Success Criteria (from spec)

✅ **SC-001**: All new Gmail bank/card alerts synced within 60 minutes, zero duplicates  
✅ **SC-002**: Drive bills matched and enriched within 60 minutes  
✅ **SC-003**: Every record includes source:refactored-code-0905 tag  
✅ **SC-004**: Every record has 2–4 labels applied  
✅ **SC-005**: Obsidian monthly logs updated within 5 minutes  
✅ **SC-006**: Product-prices.jsonl populated for all bills  
✅ **SC-007**: Cross-source merge preserves all information  
✅ **SC-008**: Zero customer-facing errors; all failures logged  
✅ **SC-009**: Sync completes in <30 seconds for typical load  
✅ **SC-010**: Auto-created accounts capped at 10  

---

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Duplicate sync from old triggers | Disable com.sumitasok.plist immediately after cutover |
| Email format changes break parsing | Engine-first design + AI fallback ensures graceful degradation |
| Missing Obsidian vault during sync | Make write-back optional; warn but continue if unavailable |
| Cursor corruption → infinite sync | Validate cursor before advancing; back up last-sync.json |
| Rate limit exceeded (>300 req/hr) | Implement exponential backoff + 3-retry logic |
| Product prices file grows unbounded | Add yearly archival task (separate feature) |

