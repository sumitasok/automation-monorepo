# Quickstart: Unified Wallet Sync Validation

**Purpose**: End-to-end validation that the unified wallet sync feature works correctly.

**Time Required**: ~20 minutes  
**Prerequisites**: Config files set up, Gmail API credentials, Wallet API token, Obsidian vault accessible

---

## Prerequisites Checklist

```bash
# 1. Config directory exists
[ -d ~/automation-monorepo-config ] && echo "✅ Config dir exists"

# 2. Required config files present
[ -f ~/automation-monorepo-config/config/expense-domain/wallet/config.yaml ] && echo "✅ config.yaml"
[ -f ~/automation-monorepo-config/config/expense-domain/wallet/routing.yaml ] && echo "✅ routing.yaml"
[ -d ~/automation-monorepo-config/config/expense-domain/wallet/email-formats ] && echo "✅ email-formats dir"

# 3. Wallet API token set
grep -q "WALLET_API_TOKEN" ~/automation-monorepo-config/config/expense-domain/wallet/config.yaml && echo "✅ Wallet token configured"

# 4. Obsidian vault accessible
[ -d ~/sa.finances/Expenses ] && echo "✅ Obsidian vault found"

# 5. Gmail credentials exist
[ -f ~/.config/sa-finances/gmail-credentials.json ] && echo "✅ Gmail credentials"
[ -f ~/.config/sa-finances/gmail-token.json ] && echo "✅ Gmail token"
```

---

## Scenario 1: Fresh Sync (New Bank Alert)

**Goal**: Verify that a new Gmail bank alert is synced to Wallet with correct tags.

### Setup

```bash
# Clear the sync cursor to backfill from a known time
cat > ~/.test-last-sync.json << 'EOF'
{
  "last_email_timestamp": "2026-09-04T00:00:00Z",
  "last_run_status": "success",
  "processed_drive_files": [],
  "auto_created_accounts": []
}
EOF

# Backup current state
cp ~/automation-monorepo-config/data/expense-domain/wallet/last-sync.json \
   ~/automation-monorepo-config/data/expense-domain/wallet/last-sync.json.backup

# Replace with test cursor
cp ~/.test-last-sync.json \
   ~/automation-monorepo-config/data/expense-domain/wallet/last-sync.json
```

### Generate Test Email

Send a test email to yourself from an HDFC bank alert address (or wait for next real alert):

```
From: alerts@hdfcbank.com
Subject: Credit Card - Transaction Alert [Test Alert]
Body:
  HDFC Bank Credit Card Alert
  Card Number: XXXXXXXXXXXX3690
  Amount: Rs. 1500.00
  Transaction Date: 05-Sep-2026
  Establishment Details: BLINKIT
  ...
```

### Run Sync

```bash
CONFIG_PATH=~/automation-monorepo-config ./scripts/wallet-sync-unified.sh
```

### Validate Results

**✅ Check 1: Record created in Wallet**

```bash
# Fetch latest records
curl -s -H "Authorization: Bearer $(grep WALLET_API_TOKEN ~/automation-monorepo-config/config/expense-domain/wallet/config.yaml | cut -d'"' -f2)" \
  "https://rest.budgetbakers.com/wallet/v1/api/records?limit=5&offset=0" | jq '.records[0]'

# Expected output should contain:
# - "counterParty": "BLINKIT"
# - "amount": -1500 (negative for expense)
# - "note": "... gm:... source:refactored-code-0905"
# - "labelIds": ["uuid-blinkit", "uuid-hdfc"] (2-4 labels)
```

**Expected Fields**:
- ✅ `counterParty`: "BLINKIT"
- ✅ `amount`: -1500 (negative)
- ✅ `currency`: "INR"
- ✅ `note`: Contains `gm:<message-id>`
- ✅ `note`: Contains `source:refactored-code-0905`
- ✅ `labelIds`: Array with 2–4 UUIDs
- ✅ `categoryId`: UUID (from CATEGORY_HINTS lookup)
- ✅ `accountId`: Matches HDFC CC x3690 from routing.yaml

**✅ Check 2: Obsidian expense log updated**

```bash
# Check if row was added to monthly log
grep "BLINKIT" ~/sa.finances/Expenses/2026/2026-09*

# Expected output:
# | 2026-09-05 | BLINKIT | Groceries | 1500 | HDFC CC x3690 | gm:abc123def456 |
```

**✅ Check 3: Sync state advanced**

```bash
cat ~/automation-monorepo-config/data/expense-domain/wallet/last-sync.json | jq '.last_email_timestamp'

# Expected: Timestamp should be ≥ test email time (cursor advanced)
```

**✅ Check 4: Labels applied correctly**

```bash
# Verify label cache was populated
cat ~/automation-monorepo-config/data/expense-domain/wallet/labels-cache.json | jq '.blinkit'

# Expected: UUID for "blinkit" label
```

### Restore State

```bash
# Restore backup
cp ~/automation-monorepo-config/data/expense-domain/wallet/last-sync.json.backup \
   ~/automation-monorepo-config/data/expense-domain/wallet/last-sync.json
```

---

## Scenario 2: Deduplication (Re-run Same Sync)

**Goal**: Verify that running sync twice doesn't create duplicates.

### Setup

Same as Scenario 1; assume Scenario 1 succeeded and Wallet has the BLINKIT record.

### Run Sync Again

```bash
# Reset cursor to same time as Scenario 1
cp ~/.test-last-sync.json \
   ~/automation-monorepo-config/data/expense-domain/wallet/last-sync.json

CONFIG_PATH=~/automation-monorepo-config ./scripts/wallet-sync-unified.sh
```

### Validate Results

**✅ Check 1: No duplicate created**

```bash
# Fetch all BLINKIT records
curl -s -H "Authorization: Bearer $(cat ~/token.txt)" \
  "https://rest.budgetbakers.com/wallet/v1/api/records?limit=100" | jq '.records[] | select(.counterParty == "BLINKIT") | .id'

# Expected: Single record (same ID as before), not 2
```

**✅ Check 2: Sync output shows "skipped"**

```bash
tail -20 ~/automation-monorepo-config/data/expense-domain/wallet/logs/sync-*.log | grep -i "skip"

# Expected output: "Skipped: 1 (duplicate, idempotency key found)"
```

**✅ Check 3: Obsidian row unchanged**

```bash
# Count BLINKIT rows in expense log
grep -c "BLINKIT" ~/sa.finances/Expenses/2026/2026-09*

# Expected: Still 1 (no duplicate row added)
```

---

## Scenario 3: Drive Bill Upload & Enrichment

**Goal**: Verify that uploading a receipt PDF to Drive Bills folder enriches the Wallet record.

### Setup

Ensure a matching transaction exists in Wallet from Scenarios 1–2 (BLINKIT, 1500 INR, 2026-09-05).

### Upload Receipt

```bash
# Create test PDF (or use real receipt)
# Upload to Google Drive Bills Inbox folder (folderId from config.yaml)
# File: blinkit-receipt-20260905.pdf
# Expected: Should extract vendor="Blinkit", date="2026-09-05", total=1500, line items
```

**Or via CLI** (if gcloud is configured):

```bash
gcloud drive files upload ~/Downloads/test-receipt.pdf \
  --parent=1DXizYKYGSg8pPO1_tbXPLTUOENOwfMR6 \
  --name="blinkit-receipt-20260905.pdf"
```

### Run Sync

```bash
CONFIG_PATH=~/automation-monorepo-config ./scripts/wallet-sync-unified.sh
```

### Validate Results

**✅ Check 1: Wallet record enriched**

```bash
# Fetch BLINKIT record (should have drive: tag added)
curl -s -H "Authorization: Bearer $(cat ~/token.txt)" \
  "https://rest.budgetbakers.com/wallet/v1/api/records?limit=5" | jq '.records[0] | {id, note, categoryId}'

# Expected:
# - note: Contains "drive:<file-id>"
# - categoryId: May be updated if bill provided category
```

**✅ Check 2: Bill note created in Obsidian**

```bash
# Check if bill note was created
ls -la ~/sa.finances/Expenses/2026/Bills/bill-260905-blinkit.md

# Expected: File exists with YAML frontmatter + itemized table
# Check contents:
cat ~/sa.finances/Expenses/2026/Bills/bill-260905-blinkit.md | head -20

# Expected:
# - Frontmatter: bill_id, date, vendor, category, amount_inr
# - Cross-link to gm: ref
# - Itemized table if PDF has line items
```

**✅ Check 3: Product prices logged**

```bash
# Check product-prices.jsonl
tail -5 ~/sa.finances/Expenses/2026/Bills/product-prices.jsonl | jq .

# Expected:
# - date: "2026-09-05"
# - product: "..."
# - vendor: "Blinkit"
# - unit_price: (extracted from line item)
# - category: (from bill or inferred)
```

---

## Scenario 4: Cross-Source Merge (Gmail + Drive + Manual)

**Goal**: Verify that Gmail + Drive + manual entries merge without data loss.

### Setup

1. Create a transaction manually in Wallet:
   - Amount: 2500 INR
   - Date: 2026-09-05
   - Merchant: "Zomato"
   - Note: "Manual dinner order"

2. Ensure a Zomato Gmail alert exists for same date/amount

3. Upload a Zomato receipt PDF to Bills Inbox

### Run Sync

```bash
CONFIG_PATH=~/automation-monorepo-config ./scripts/wallet-sync-unified.sh
```

### Validate Results

**✅ Check 1: Single merged record in Wallet**

```bash
curl -s -H "Authorization: Bearer $(cat ~/token.txt)" \
  "https://rest.budgetbakers.com/wallet/v1/api/records?limit=100" | jq '.records[] | select(.counterParty | contains("Zomato"))'

# Expected: Single record with:
# - note: Contains both "gm:..." AND "drive:..." tags
# - note: Still contains manual note text ("Manual dinner order")
# - categoryId: From bill or Gmail classification (richest source)
```

**✅ Check 2: Obsidian log shows single row with cross-links**

```bash
grep "Zomato\|zomato" ~/sa.finances/Expenses/2026/2026-09*

# Expected:
# | 2026-09-05 | Zomato | Restaurants | 2500 | Credit Card | gm:abc123 drive:xyz789 |
```

**✅ Check 3: No duplicate records created**

```bash
curl -s -H "Authorization: Bearer $(cat ~/token.txt)" \
  "https://rest.budgetbakers.com/wallet/v1/api/records?limit=100" | jq '[.records[] | select(.counterParty | contains("Zomato"))] | length'

# Expected: 1 (not 2 or 3)
```

---

## Scenario 5: Hourly Schedule (Launchd)

**Goal**: Verify that the sync runs automatically via launchd every hour.

### Setup

Ensure plist is installed:

```bash
# Check if plist is loaded
launchctl list | grep com.safinances.wallet-sync

# Expected output: Shows job with PID (if currently running) or `-` (if not)
```

### Verify Plist Installed

```bash
# Check plist location
cat /Users/sumitasok/Library/LaunchAgents/com.safinances.wallet-sync.plist | grep -A 5 "ProgramArguments"

# Expected: Points to repo version of wallet-sync-unified.sh, not Obsidian run-sync.sh
```

### Wait for Next Hour

Wait until next hour at :07 minute (e.g., if it's 14:35, wait until 15:07).

### Validate Results

**✅ Check 1: Sync ran automatically**

```bash
# Check logs
ls -lrt ~/automation-monorepo-config/data/expense-domain/wallet/logs/sync-*.log | tail -1

# Expected: Timestamp ≈ current time (within last few minutes)
```

**✅ Check 2: Log shows successful sync**

```bash
tail -20 ~/automation-monorepo-config/data/expense-domain/wallet/logs/sync-*.log | grep "COMPLETE"

# Expected: "✅ COMPLETE" message
```

**✅ Check 3: Verify com.sumitasok.plist is DISABLED**

```bash
# Check if old trigger is running
launchctl list | grep com.sumitasok.wallet-sync

# Expected: Should NOT appear (disabled)
```

---

## Troubleshooting

| Issue | Check | Fix |
|-------|-------|-----|
| `Config not found` | `ls ~/automation-monorepo-config/config/expense-domain/wallet/config.yaml` | Create config.yaml with WALLET_API_TOKEN |
| `Gmail API 401 Unauthorized` | `cat ~/.config/sa-finances/gmail-token.json` | Re-run auth: `python3 sync.py --auth` |
| `Wallet API 403 Forbidden` | Check token expiration in Wallet app settings | Regenerate token, update config.yaml |
| `No records found` | Check cursor in `last-sync.json` | Ensure cursor is before your test email timestamp |
| `Duplicate records created` | Check if idempotency key (gm:) is in note | Verify sync.py appends gm:<msgid> |
| `Obsidian write-back failed` | Check vault path in config.yaml | Ensure vault path is correct and accessible |

---

## Success Criteria

All scenarios pass when:

1. ✅ **Scenario 1**: New bank alert synced with gm: tag + source tag + labels
2. ✅ **Scenario 2**: Re-run detects duplicate via idempotency key, skips
3. ✅ **Scenario 3**: Drive bill matched, record enriched, bill note created
4. ✅ **Scenario 4**: Gmail + Drive + manual merge without data loss
5. ✅ **Scenario 5**: Hourly schedule runs automatically, old trigger disabled

**Feature is READY for production when all 5 scenarios pass.**

