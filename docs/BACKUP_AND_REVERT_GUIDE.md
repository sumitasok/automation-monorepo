# Backup & Revert Guide for Wallet Deduplication

**Purpose**: This guide explains how the backup/caching system works and how to execute a complete revert if needed.

**Audience**: AI agents (Claude) performing wallet operations or reverting changes.

---

## Overview

The wallet deduplication system creates two backup files before making ANY changes:

1. **Before-state backup** (`wallet-before-*.json`) — Complete snapshot of wallet records BEFORE deduplication
2. **Change log** (`wallet-changelog-*.json`) — Detailed record of what WAS CHANGED and HOW TO REVERT each change

This allows complete reversal of any deduplication operation.

---

## Backup Directory Structure

```
~/automation-monorepo-config/backups/wallet-dedup/
├── wallet-before-2026-09-05T13-35-53-193Z.json     ← Before state
├── wallet-before-2026-09-05T14-22-10-456Z.json     ← Another operation
├── wallet-changelog-2026-09-05T13-35-53-193Z.json  ← Change log for first op
├── wallet-changelog-2026-09-05T14-22-10-456Z.json  ← Change log for second op
└── ... (one pair per operation)
```

**File Naming Convention**:
- Timestamp format: `YYYY-MM-DDTHH-MM-SS-MMMZ` (ISO 8601 with `-` replacing `:`)
- Before and change log share the SAME timestamp
- Multiple backup pairs = multiple historical operations

---

## Backup File Format

### Before-State Backup (`wallet-before-*.json`)

```json
{
  "timestamp": "2026-09-05T13:35:53.193Z",
  "description": "Wallet state before deduplication",
  "total_records": 6,
  "records": [
    {
      "id": "39629ad1-dfe9-47a8-bddd-aca5daf90318",
      "merchant": "Zomato",
      "amount": 868.76,
      "date": "2026-09-05",
      "category": "Food & Drinks",
      "description": "Zomato | via Canara CC x6102 | gmail-sync gm:1a07051c0102f33e",
      "labels": ["Chinju", "Ordering in outside food", "Dinner"],
      "source_code_version": "unknown-manual-entry",
      "created_by": "manual-web-entry",
      "created_at": "2026-09-05T07:39:56.622Z"
    },
    ... (5 more records)
  ],
  "analysis": {
    "by_source": {
      "unknown-manual-entry": [
        { "id": "39629ad1...", "merchant": "Zomato", "amount": 868.76, "created_by": "manual-web-entry" },
        ... (2 more manual entries)
      ],
      "restructure-architecture-worktree": [
        { "id": "e5b1d60e...", "merchant": "Blinkit", "amount": 607, "created_by": "framework-gmail-sync" },
        ... (2 more automation entries)
      ]
    },
    "by_category": {
      "Food & Drinks": 3,
      "Unknown expense": 3
    }
  }
}
```

**Key Fields**:
- `timestamp` — Exact moment backup was created (matches `wallet-changelog-*.json` timestamp)
- `total_records` — Number of records in wallet at backup time
- `records[]` — Complete record objects with ALL attributes
  - `id` — Unique record identifier (used in DELETE/PATCH API calls)
  - `source_code_version` — Which code version created this record
  - `created_by` — Who created it (manual-web-entry vs framework-gmail-sync)
  - `created_at` — ISO timestamp of creation
- `analysis.by_source` — Records grouped by `source_code_version` (aids identifying which operations to revert)
- `analysis.by_category` — Category distribution (confirms data integrity)

---

### Change Log (`wallet-changelog-*.json`)

```json
{
  "timestamp": "2026-09-05T13:35:53.193Z",
  "description": "Change log from deduplication",
  "total_changes": 3,
  "deletions": [
    {
      "id": "e8156d87-ff75-4a7e-9fcb-660850e08f30",
      "merchant": "Blinkit",
      "amount": 607,
      "created_by": "manual-web-entry",
      "source_version": "unknown-manual-entry",
      "created_at": "2026-09-05T12:38:45.989Z",
      "reason": "Duplicate without source:automation-monorepo label",
      "revert_command": "curl -X POST https://api.wallet.example.com/records -d '{...record-json...}'"
    },
    ... (2 more deletions)
  ],
  "updates": [
    {
      "id": "e5b1d60e-be9a-438c-9705-fa16e24a1dfe",
      "merchant": "Blinkit",
      "merged_with_id": "e8156d87-ff75-4a7e-9fcb-660850e08f30",
      "new_description": "Blinkit | better details from manual record",
      "new_labels": ["Blinkit", "from-manual-source"],
      "revert_command": "curl -X PATCH https://api.wallet.example.com/records/e5b1d60e... -d '{...original-data...}'"
    },
    ... (2 more updates)
  ],
  "revert_instructions": {
    "note": "If deduplication causes issues, use these commands to restore",
    "step_1": "Delete the updated records (if created)",
    "step_2": "Restore deleted records from backup JSON",
    "backup_file": "/Users/sumitasok/automation-monorepo-config/backups/wallet-dedup/wallet-before-2026-09-05T13-35-53-193Z.json",
    "sql_restore": "DELETE FROM wallet_records WHERE id IN ('e8156d87...', 'e32dcffe...', '39629ad1...');",
    "restore_from_backup": "cat backup.json | jq '.records[] | select(.id | IN(...))' > restore.json"
  }
}
```

**Key Fields**:
- `timestamp` — Matches the `wallet-before-*.json` timestamp (links them together)
- `total_changes` — Number of records modified (deleted or updated)
- `deletions[]` — Records that WILL BE deleted
  - `id` — Record to DELETE via API
  - `reason` — Why this record is a duplicate
  - `revert_command` — Exact curl command to RE-CREATE this record
- `updates[]` — Records that WILL BE updated/merged
  - `id` — Record to UPDATE via API (the one KEPT)
  - `merged_with_id` — ID of the record being deleted from
  - `new_description` — New description after merge
  - `new_labels` — New labels after merge
  - `revert_command` — Exact curl command to RESTORE original values
- `revert_instructions` — Complete guide for reverting ALL changes

---

## How to Identify a Specific Backup

**If you need to revert a specific operation**, use the timestamp to match files:

```bash
# 1. List all backups with timestamps
ls -lh ~/automation-monorepo-config/backups/wallet-dedup/

# 2. Find the timestamp you want to revert
# Example: wallet-before-2026-09-05T13-35-53-193Z.json
#          wallet-changelog-2026-09-05T13-35-53-193Z.json

# 3. The matching pair is the one with the SAME timestamp
# Do NOT mix timestamps from different operations!
```

---

## Step-by-Step Revert Procedure

### **Step 1: Identify What Changed**

Read the change log to understand the operation:

```bash
cat ~/automation-monorepo-config/backups/wallet-dedup/wallet-changelog-2026-09-05T13-35-53-193Z.json | jq '.deletions, .updates' | less
```

### **Step 2: Restore Deleted Records**

For each deletion, use the `revert_command` to RE-CREATE the record:

```bash
# Extract the revert commands
cat ~/automation-monorepo-config/backups/wallet-dedup/wallet-changelog-2026-09-05T13-35-53-193Z.json | jq '.deletions[] | .revert_command' -r

# Example output (three separate commands):
curl -X POST https://api.wallet.example.com/records -d '{"id":"e8156d87...","merchant":"Blinkit",...}'
curl -X POST https://api.wallet.example.com/records -d '{"id":"e32dcffe...","merchant":"ZOMATO",...}'
curl -X POST https://api.wallet.example.com/records -d '{"id":"39629ad1...","merchant":"Zomato",...}'

# Execute each one (or batch them):
for cmd in $(cat ... | jq '.deletions[] | .revert_command' -r); do
  eval "$cmd"
  echo "Created: $?"
done
```

### **Step 3: Restore Updated Records**

For each update, use the `revert_command` to RESTORE original values:

```bash
# Extract the revert commands
cat ~/automation-monorepo-config/backups/wallet-dedup/wallet-changelog-2026-09-05T13-35-53-193Z.json | jq '.updates[] | .revert_command' -r

# Example (updates three records back to original state):
curl -X PATCH https://api.wallet.example.com/records/e5b1d60e... -d '{"description":"Blinkit...","labels":[...]}'
curl -X PATCH https://api.wallet.example.com/records/f3893584... -d '{"description":"ZOMATO...","labels":[...]}'
curl -X PATCH https://api.wallet.example.com/records/67fc052c... -d '{"description":"Zomato...","labels":[...]}'

# Execute each one:
for cmd in $(cat ... | jq '.updates[] | .revert_command' -r); do
  eval "$cmd"
  echo "Updated: $?"
done
```

### **Step 4: Verify Revert Succeeded**

Fetch records and compare against backup:

```bash
# Fetch current state
curl https://api.wallet.example.com/records \
  -H "Authorization: Bearer $WALLET_API_TOKEN" | jq '.' > current-state.json

# Compare against backup
diff <(jq '.records | sort_by(.id)' wallet-before-*.json) \
     <(jq '.' current-state.json | jq 'sort_by(.id)')

# If diff output is empty, revert was successful!
```

---

## Understanding Record Identity

**Critical**: The `id` field is the record's unique identifier across the Wallet API.

```
id: "e8156d87-ff75-4a7e-9fcb-660850e08f30"
   └─ This is what you DELETE/PATCH against in API calls
   └─ This is what the backup uses to track which record was changed
   └─ This NEVER changes, even after category updates
```

**All API operations use this ID**:
```bash
# DELETE a record
curl -X DELETE https://api.wallet.example.com/records/e8156d87... \
  -H "Authorization: Bearer $WALLET_API_TOKEN"

# UPDATE a record
curl -X PATCH https://api.wallet.example.com/records/e8156d87... \
  -H "Authorization: Bearer $WALLET_API_TOKEN" \
  -d '{"description":"..."}'
```

---

## Source Code Version Tracking

The `source_code_version` field tells you which code version created a record:

| Version | Meaning | Creator |
|---------|---------|---------|
| `unknown-manual-entry` | Manual web entry, not from automation | `manual-web-entry` |
| `restructure-architecture-worktree` | Created by current feature branch automation | `framework-gmail-sync` |
| `main-branch-version` | Created by production automation | `framework-gmail-sync` |
| (future versions) | Different branches/versions of framework | varies |

**Use this to understand**:
- Which records are duplicates of which version's work
- Whether a revert should target only certain versions
- Which feature branch caused an issue (if any)

---

## Real-World Revert Examples

### Example 1: Revert All Changes from One Operation

```bash
#!/bin/bash
set -e

BACKUP_DIR=~/automation-monorepo-config/backups/wallet-dedup
TIMESTAMP="2026-09-05T13-35-53-193Z"
CHANGELOG="$BACKUP_DIR/wallet-changelog-$TIMESTAMP.json"
API_BASE="https://api.wallet.example.com"

echo "🔄 Reverting all changes from operation $TIMESTAMP..."

# Step 1: Restore deleted records
echo "📝 Restoring deleted records..."
jq -r '.deletions[] | .revert_command' "$CHANGELOG" | while read cmd; do
  eval "$cmd"
  echo "  ✅ Restored"
done

# Step 2: Restore updated records
echo "📝 Restoring updated records..."
jq -r '.updates[] | .revert_command' "$CHANGELOG" | while read cmd; do
  eval "$cmd"
  echo "  ✅ Restored"
done

echo "✅ Revert complete!"
echo "⚠️  Verify in Wallet app that all records are back to original state"
```

### Example 2: Revert Only Deletions

```bash
#!/bin/bash
# If updates succeeded but deletions caused an issue

CHANGELOG="$BACKUP_DIR/wallet-changelog-$TIMESTAMP.json"

echo "🔄 Restoring only deleted records..."
jq -r '.deletions[] | .revert_command' "$CHANGELOG" | while read cmd; do
  eval "$cmd"
  echo "  ✅ Restored: $(echo $cmd | jq '.id')"
done

echo "✅ Deleted records restored!"
```

### Example 3: Find Which Version Caused the Issue

```bash
# Check which records are problematic
cat ~/automation-monorepo-config/backups/wallet-dedup/wallet-before-*.json | \
  jq '.records[] | select(.merchant == "Zomato") | {id, source_code_version, created_by, category}'

# Output:
# {
#   "id": "39629ad1-dfe9-47a8-bddd-aca5daf90318",
#   "source_code_version": "unknown-manual-entry",
#   "created_by": "manual-web-entry",
#   "category": "Food & Drinks"
# }
# {
#   "id": "67fc052c-c633-4965-95ff-eb25d93c330e",
#   "source_code_version": "restructure-architecture-worktree",
#   "created_by": "framework-gmail-sync",
#   "category": "Food & Drinks"
# }

# Now you know: manual entry vs automation, different sources
```

---

## Integrity Checks

### Verify Backup Integrity

```bash
# Check both files exist and match timestamps
ls -lh ~/automation-monorepo-config/backups/wallet-dedup/ | grep "2026-09-05T13-35-53-193Z"

# Output should show:
# wallet-before-2026-09-05T13-35-53-193Z.json
# wallet-changelog-2026-09-05T13-35-53-193Z.json

# Check JSON validity
jq empty ~/automation-monorepo-config/backups/wallet-dedup/wallet-before-*.json
jq empty ~/automation-monorepo-config/backups/wallet-dedup/wallet-changelog-*.json
# (no errors = valid JSON)
```

### Verify Revert Completeness

After reverting, check that record counts match:

```bash
# Count records in backup
BEFORE_COUNT=$(jq '.total_records' wallet-before-*.json)

# Count records in current Wallet API
CURRENT_COUNT=$(curl -s https://api.wallet.example.com/records \
  -H "Authorization: Bearer $WALLET_API_TOKEN" | jq 'length')

if [ "$BEFORE_COUNT" -eq "$CURRENT_COUNT" ]; then
  echo "✅ Record count matches backup: $BEFORE_COUNT records"
else
  echo "❌ Count mismatch! Before: $BEFORE_COUNT, Now: $CURRENT_COUNT"
  exit 1
fi
```

---

## File Locations Reference

| Purpose | Path | Format |
|---------|------|--------|
| Backups stored here | `~/automation-monorepo-config/backups/wallet-dedup/` | Directory |
| Before-state snapshot | `wallet-before-{TIMESTAMP}.json` | JSON |
| Change instructions | `wallet-changelog-{TIMESTAMP}.json` | JSON |
| Script creating backups | `.worktrees/restructure-architecture/scripts/safe-deduplicate-wallet.js` | JavaScript |
| Execution script | `.worktrees/restructure-architecture/scripts/deduplicate-real-wallet.js` | JavaScript |
| Configuration | `~/automation-monorepo-config/config/wallet/config.yaml` | YAML |
| This guide | `.worktrees/restructure-architecture/docs/BACKUP_AND_REVERT_GUIDE.md` | Markdown |

---

## Environment Variables Required for Revert

To execute revert commands, you need:

```bash
# Required for Wallet API access
export WALLET_API_TOKEN="your-premium-api-token"
export WALLET_BASE_URL="https://rest.budgetbakers.com/wallet"  # Optional, has default

# Optional but recommended
export CONFIG_PATH=~/automation-monorepo-config
```

---

## Quick Reference: Complete Revert Script

```bash
#!/bin/bash
# Complete revert for a wallet deduplication operation
# Usage: ./revert.sh <TIMESTAMP>

TIMESTAMP="${1:-2026-09-05T13-35-53-193Z}"
BACKUP_DIR=~/automation-monorepo-config/backups/wallet-dedup
CHANGELOG="$BACKUP_DIR/wallet-changelog-$TIMESTAMP.json"
BEFORE="$BACKUP_DIR/wallet-before-$TIMESTAMP.json"

if [ ! -f "$CHANGELOG" ]; then
  echo "❌ Changelog not found: $CHANGELOG"
  exit 1
fi

echo "🔄 REVERTING WALLET DEDUPLICATION"
echo "Timestamp: $TIMESTAMP"
echo "Changelog: $CHANGELOG"
echo ""

# Check API token
if [ -z "$WALLET_API_TOKEN" ]; then
  echo "❌ WALLET_API_TOKEN not set. Set with: export WALLET_API_TOKEN='...'"
  exit 1
fi

# Restore deleted records
echo "📝 Restoring deleted records..."
jq -r '.deletions[] | .revert_command' "$CHANGELOG" | while read cmd; do
  if eval "$cmd" > /dev/null 2>&1; then
    echo "  ✅ Restored"
  else
    echo "  ❌ Failed"
  fi
done

# Restore updated records
echo "📝 Restoring updated records..."
jq -r '.updates[] | .revert_command' "$CHANGELOG" | while read cmd; do
  if eval "$cmd" > /dev/null 2>&1; then
    echo "  ✅ Restored"
  else
    echo "  ❌ Failed"
  fi
done

echo ""
echo "✅ Revert complete!"
echo "📋 Backup reference: $BEFORE"
echo "⚠️  Verify in Wallet app that records are restored correctly"
```

---

## Summary for AI Agents

**When asked to revert wallet deduplication**:

1. Find the correct timestamp (match before + changelog files)
2. Read the changelog to understand what changed
3. Execute `deletions[].revert_command` to restore deleted records (POST)
4. Execute `updates[].revert_command` to restore updated records (PATCH)
5. Verify record count matches `before.total_records`
6. Confirm in Wallet app that all records appear with original values

**Critical**: Always use the SAME timestamp for both files. Never mix timestamps.

