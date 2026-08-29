# Wallet Sync Workflow

Complete end-to-end pipeline for syncing financial transactions from Gmail to Wallet app.

## Quick Start

Run the complete workflow (with deduplication):
```bash
export WALLET_API_TOKEN="your-premium-api-token"
./auto orchestrate gmail-wallet-sync-with-dedup
```

This runs all 10 steps in sequence: extract → categorize → sync → dedup (scan → review → execute → finalize).

**Alternative**: Run without dedup:
```bash
./auto orchestrate gmail-wallet-sync
```

## Pipeline Overview

### With Deduplication (Recommended)
```
Wallet App
    ↓
[1] wallet-fetch
    ↓ (fetches current wallet records)
Gmail Inbox
    ↓
[2] gmail-extract
    ↓ (extracts transactions to gmail/transactions.csv)
[3] gmail-categorize
    ↓ (AI categorization with DeepSeek)
[4] wallet-sync-categories
    ↓ (syncs categories to Wallet Unknown records)
[5] wallet-fetch-accounts
    ↓ (refreshes accounts cache)
[6] wallet-sync
    ↓ (pushes to Wallet API)
[7] wallet-dedup scan
    ↓ (detects duplicates, read-only)
[8] wallet-dedup review
    ↓ (interactive: collect decisions)
[9] wallet-dedup execute
    ↓ (DELETE from Wallet API)
[10] wallet-dedup finalize
    ↓ (cleanup local records.jsonl)
Wallet App ✓
```

Run with: `./auto orchestrate gmail-wallet-sync-with-dedup`

### Without Deduplication
```
[1] wallet-fetch → [2] gmail-extract → [3] gmail-categorize → 
[4] wallet-sync-categories → [5] wallet-fetch-accounts → [6] wallet-sync → Wallet App ✓
```

Run with: `./auto orchestrate gmail-wallet-sync`

## Step-by-Step Details

### [1] Wallet Fetch
**Command:** `./auto run wallet-fetch`

Fetches all wallet records from Wallet API and saves to `data/wallet/records.jsonl`. This snapshot is used for:
- Matching unknown categories (wallet-sync-categories)
- Building category ID map
- Detecting duplicates (wallet-dedup)
- Providing baseline for sync operations

**Output:** `data/wallet/records.jsonl` (JSONL format, one record per line)

---

### [2] Gmail Extract
**Command:** `./auto run gmail-extract`

Extracts financial transactions from Gmail inbox using email pattern matching. Creates `data/gmail/transactions.csv` with:
- Transaction date
- Merchant/counterparty
- Amount
- Email subject/info
- MessageID (for deduplication)

**Output:** `data/gmail/transactions.csv`

---

### [2] Gmail Categorize
**Command:** `./auto run gmail-categorize`

Uses AI (DeepSeek) to categorize uncategorized transactions. Analyzes merchant name and transaction context to assign categories and labels.

**Input:** `data/gmail/transactions.csv`  
**Output:** `data/gmail/transactions.csv` (updated with categories)

---

### [3] Wallet Sync Categories
**Command:** `./auto run wallet-sync-categories -- --apply`

Matches Gmail categorized transactions to Wallet records with "Unknown" categories. Uses merchant name + date matching (±1 day).

- Dry-run default: `./auto run wallet-sync-categories`
- Apply all: `./auto run wallet-sync-categories -- --apply`
- High-confidence only: `./auto run wallet-sync-categories -- --apply-high`

**Input:** Gmail categories + Wallet records  
**Output:** Category updates to Wallet API

---

### [4] Wallet Fetch Accounts
**Command:** `./auto run wallet-fetch-accounts`

Syncs all Wallet accounts from API to local cache (`data/wallet/accounts-cache.json`). Used for account mapping fallback during sync.

**Input:** Wallet API  
**Output:** `data/wallet/accounts-cache.json`

---

### [5] Wallet Dedup
**Command:** `./auto run wallet-dedup scan`

Scans for potential duplicate records in Wallet. Does NOT auto-fix; requires review before execution.

Subcommands:
- `scan` — identify duplicates (safe, read-only)
- `review` — show duplicates with details
- `execute` — apply dedup decisions (destructive)
- `finalize` — cleanup after execute

**Display Duplicates:**
**Command:** `python3 show-duplicates.py`

Shows a formatted index of all duplicate groups with:
- Duplicate group number and record count
- MessageID for the group
- For each record: Date, Merchant, Amount, Category, Record ID
- Creation timestamp to identify newest vs oldest (oldest marked for KEEP)
- Summary showing total groups, records to delete, records to keep

**Flow:**
```
scan → (python3 show-duplicates.py) → review → execute → finalize
```

---

### [6] Wallet Sync
**Command:** `./auto run wallet-sync`

Main sync operation. Reads `data/gmail/transactions.csv`, maps to Wallet accounts, creates records via Wallet API.

Features:
- Automatic retry on rate limits (429) with exponential backoff
- Comprehensive logging showing every step
- Account mapping with cache fallback
- Deduplication by MessageID (state.json)
- Batch processing (max 20 per request)

**Input:** `data/gmail/transactions.csv`  
**Output:** Records created in Wallet API  
**State:** `data/wallet/state.json` (dedup ledger)

---

## Manual Step-by-Step

Run individual steps:

```bash
# Step 1: Fetch wallet records
./auto run wallet-fetch

# Step 2: Extract transactions
./auto run gmail-extract

# Step 3: Categorize with AI
./auto run gmail-categorize

# Step 4: Sync categories (dry-run first)
./auto run wallet-sync-categories
./auto run wallet-sync-categories -- --apply

# Step 5: Fetch accounts for fallback matching
./auto run wallet-fetch-accounts

# Step 6: Scan for duplicates
./auto run wallet-dedup scan
./auto run wallet-dedup review
# If duplicates found:
./auto run wallet-dedup execute
./auto run wallet-dedup finalize

# Step 7: Sync to Wallet
./auto run wallet-sync
```

## Configuration

All configuration is done via environment variables and config files:

### Environment Variables (in `config/wallet/config.yaml`)

```yaml
env:
  WALLET_API_TOKEN: "your-premium-api-token"
  WALLET_LABEL: "source:automation-monorepo"  # optional
  WALLET_LABEL_ID: "uuid"                    # optional, preferred over above
  WALLET_DEFAULT_PAYMENT_TYPE: "debit_card"  # default: debit_card
  WALLET_TIMEZONE: "Asia/Kolkata"            # default: Asia/Kolkata
  WALLET_BASE_URL: "https://rest.budgetbakers.com/wallet"  # usually unchanged
```

### Account Mapping

Create `config/wallet/accounts.json` mapping CSV account codes to Wallet UUIDs:

```json
{
  "6003": {
    "accountId": "c8806151-51be-44e8-9e49-08473cb3727f",
    "paymentType": "credit_card"
  },
  "1983": {
    "accountId": "d5a84d3e-7293-4e5e-b2f8-6c9e1d2f8a3b"
  },
  "_default": {
    "accountId": "default-account-uuid"
  }
}
```

Fallback: If an account code isn't found, the system searches `data/wallet/accounts-cache.json` (synced from Wallet API) by matching last digits.

---

## Troubleshooting

### Rate Limit (429)

Wallet API rate limits requests. The sync automatically retries with exponential backoff:
- Wait 2s, retry
- Wait 4s, retry
- Wait 8s, retry
- Fail if all retries exhausted

No action needed; just wait for retries to succeed.

### Account Unmapped

**Error:** `skip (unmapped account "XX6003")`

**Solution:**
1. Get Wallet account UUID: Use Wallet MCP's `get_accounts` tool
2. Add to `config/wallet/accounts.json`:
   ```json
   "6003": {
     "accountId": "the-uuid",
     "paymentType": "credit_card"
   }
   ```
3. Rerun `./auto run wallet-sync`

OR: System will automatically match by last digits against `accounts-cache.json`.

### Duplicate MessageID

**Warning:** `duplicate MessageID <id> with different amount (old: X, new: Y)`

Indicates the same email was processed twice with different amounts. The sync treats it as a new transaction and creates a new record. Use `wallet-dedup` to clean up if needed.

### No Gmail Transactions Found

Ensure Gmail has emails matching transaction patterns:
- Subject contains merchant + amount
- Or specific email formatting (check gmail pack docs)

Check: `data/gmail/transactions.csv` exists and has rows.

---

## Logging

All steps have comprehensive logging:

```
[1/6] GMAIL-EXTRACT: Extracting transactions from Gmail...
  Processing: inbox (500 emails)
  Extracted: 127 transactions
✓ Gmail extraction complete

[2/6] GMAIL-CATEGORIZE: AI-categorizing transactions...
  Categorizing 45 uncategorized rows via DeepSeek
  Batch 1: 45 rows in 1 API call
✓ Gmail categorization complete

[3/6] WALLET-SYNC-CATEGORIES: Syncing categories...
  Indexed 96 categories from wallet records
  Matched: 22, Unmatched: 7
  Batch 1/3: Sending 10 records...
    [1/10] ✅ merchant1 → Food & Drinks
    [10/10] ✅ merchant10 → Housing
  → 10 succeeded, 0 failed
✓ Category sync complete

[6/6] WALLET-SYNC: Pushing transactions...
  processing 2531 transaction(s) across 245 day(s)
  [1/245] 2020-01-01: 5 record(s)
    batch 1/1: sending 5 record(s)...
      [1/5] ✅ abc123 -10.00 Merchant → rec-id
      [5/5] ✅ xyz789 -50.00 Merchant → rec-id
    → 5 succeeded, 0 failed
✓ Wallet sync complete
```

---

## Requirements

- ✅ `WALLET_API_TOKEN` configured (Premium plan)
- ✅ `config/wallet/accounts.json` created with account mappings
- ✅ Gmail pack working (extract step successful)
- ✅ Internet connection (API calls)

---

## Next Steps After Workflow

After a successful sync:

1. **Review Wallet app** — New transactions should appear in Wallet
2. **Check duplicates** — `./auto run wallet-dedup scan`
3. **Schedule (optional)** — Set up auto-run via cron/launchd if desired
4. **Monitor** — Watch logs for any errors or unmapped accounts

---

## Troubleshooting Workflow Issues

If the complete workflow fails:

1. **Check which step failed** — Workflow stops at first failure
2. **Run that step manually** for detailed logs
3. **Fix the issue** (e.g., add missing account mapping)
4. **Rerun workflow** — It will continue from the beginning

Example:
```bash
# Workflow fails at wallet-sync
# Run manually with full logging:
./auto run wallet-sync

# See detailed error and fix
# Then rerun workflow:
./run-wallet-workflow.sh
```

---

## Support

For issues with specific steps, see their individual documentation:
- Gmail pack: `packs/gmail/RUNBOOK.md`
- Wallet pack: `packs/wallet/RUNBOOK.md`

For questions about the workflow orchestration, refer to this guide.
