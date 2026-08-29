# Wallet Sync: Duplicate Records & Timestamp Fix

**Date**: 2026-08-27  
**Issue**: Duplicate transaction records in wallet app + lost purchase timestamps  
**Status**: RESOLVED

## Problem Statement

Users reported two related issues:
1. **Duplicates**: Some transactions were appearing twice in the wallet despite deduplication logic
2. **Timestamps**: All wallet entries showed 7:30 AM (sync schedule time) instead of actual purchase time from bank emails

## Root Causes

### Duplicates Issue
- Deduplication used only MessageID as the key
- If same Gmail message ID appeared with different amounts (e.g., data corruption, duplicate emails), the second wouldn't be caught
- Race conditions during sync could create multiple wallet records before state.json was persisted

### Timestamp Issue  
- Date resolution logic prioritized TxnDate over EmailDate
- TxnDate often contains only date (2026-06-26) without time
- EmailDate contains full timestamp from bank alert (e.g., "Fri, 26 Jun 2026 14:27:56 +0530")
- Code was using date-only version, losing time information

## Solution Implemented

### 1. Composite Key Deduplication
**File**: `internal/state/state.go`
- Enhanced `Entry` struct to store `Amount` alongside MessageID
- Added `Has(messageID, amount)` function to check both fields match
- Added `HasMessageID(messageID)` to detect data anomalies
- State now tracks: MessageID → (RecordID, Amount, Date, PushedAt)

**Effect**: Same transaction (same MessageID + Amount) can never be synced twice

### 2. Enhanced Sync Detection
**File**: `internal/sync/sync.go`
- Updated deduplication check to use both MessageID AND Amount
- Added warning when same MessageID appears with different amounts (catches data corruption)
- Updated `applyResults()` to store amount in state for future reference

**Example warning**:
```
WARN: duplicate MessageID 19f03266b38f17e1 with different amount (old: 414.72, new: 450.00)
```

### 3. Timestamp Preservation
**File**: `internal/csvtxn/csvtxn.go`
- Changed `resolveDate()` to prefer EmailDate when available
- EmailDate always has full timestamp from bank alert
- TxnDate is only used as fallback when EmailDate is missing

**Priority**: EmailDate (with time) > TxnDate (date-only fallback)

### 4. Duplicate Detection Tool
**File**: `detect-duplicates.go`, `main.go`
- Added `detect-duplicates` subcommand to find existing issues
- Reports: CSV duplicates, state duplicates, cross-check issues
- Usage: `go run . detect-duplicates [--format json]`

**Example output**:
```
=== Duplicate Detection Report ===
CSV Duplicates Found (0 sets):
State Duplicates Found (0 sets):
Total Duplicate Records: 0
Recommendations:
✓ No duplicates detected. Deduplication is working correctly.
```

## Testing

All existing tests updated and passing:
- `TestResolveDate_PrefersEmailDateWithTime`: Verifies EmailDate is used when available
- `TestResolveDate_FallsBackToTxnDateWhenEmailDateMissing`: Verifies fallback works
- `TestSaveAndReload`: Verifies amount is persisted in state
- `TestRunner_*`: Sync tests pass with new composite-key deduplication

## Usage

### Run sync with new deduplication:
```bash
./auto run wallet-sync --dry-run  # Preview without API calls
./auto run wallet-sync            # Create records with timestamp preservation
```

### Detect existing duplicates:
```bash
./auto run wallet-detect-duplicates
./auto run wallet-detect-duplicates --format json
```

### Check specific date range:
```bash
./auto run wallet-sync --since 2026-06-01 --until 2026-06-30
```

## Migration for Existing Data

If wallet app contains old duplicates:
1. Run `detect-duplicates` to see what exists
2. Review results to understand extent of issue
3. Manually remove duplicates in wallet app UI (they're tagged with `source:automation-monorepo` label)
4. Re-run `wallet-sync` - will now prevent new duplicates

New syncs will use composite key: `StateEntry { MessageID, Amount }` preventing future duplicates

## State File Format (Before → After)

**Before** (MessageID-only):
```json
{
  "pushed": {
    "gmail:user@gmail.com:19f03266b38f17e1": {
      "recordId": "cc484e3a-9b79-4445-abbc-e1a0d9caf64b",
      "date": "2026-06-26",
      "pushedAt": "2026-07-20T17:50:11Z"
    }
  }
}
```

**After** (Composite key with amount):
```json
{
  "pushed": {
    "gmail:user@gmail.com:19f03266b38f17e1": {
      "recordId": "cc484e3a-9b79-4445-abbc-e1a0d9caf64b",
      "date": "2026-06-26",
      "pushedAt": "2026-07-20T17:50:11Z",
      "amount": 414.72
    }
  }
}
```

Old state files will load correctly (amount defaults to 0 for old entries, won't cause false positives).

## Backward Compatibility

✓ Old state.json files load without error (empty Amount field defaults to 0)  
✓ First sync run will update old entries with their amounts  
✓ No need to reset state or re-sync everything

## Caveats

- If same MessageID appears with TRULY different amounts (not a duplicate, but data issue in email), only first will sync. Review and add to exclusion filters if needed.
- EmailDate parsing requires valid RFC1123Z format from email. Corrupted dates still fall back to TxnDate.
- `detect-duplicates` is read-only. Manual removal of wallet records required for cleanup.

## Related Issues

- [[Pack boundaries by question answered]]: Wallet sync is separate pack from Gmail extract
- ADR 0005: Data files (transactions.csv, state.json) are local, not in git
- ADR 0009: Label management via API (attempted but Wallet API doesn't support labels via REST)
