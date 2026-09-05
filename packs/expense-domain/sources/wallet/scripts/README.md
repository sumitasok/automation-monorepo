# Wallet Domain Scripts

Utility scripts for wallet data management, including deduplication, testing, and backup/revert operations.

## Scripts

### Deduplication

- **`safe-deduplicate-wallet.js`** — Analyze wallet for duplicates with backup & source tracking
  - Creates before-state backup
  - Tracks which code version created each record
  - Generates change log with revert instructions
  - No actual changes made (dry-run analysis)
  - Usage: `CONFIG_PATH=~/automation-monorepo-config node safe-deduplicate-wallet.js`

- **`deduplicate-real-wallet.js`** — Execute real wallet deduplication via Budget Bakers API
  - Fetches records from Wallet API
  - Analyzes for duplicates
  - Requires WALLET_API_TOKEN environment variable
  - Requires SKIP_CONFIRMATION=true to execute changes
  - Usage: `CONFIG_PATH=~/automation-monorepo-config SKIP_CONFIRMATION=true node deduplicate-real-wallet.js`

### Testing

- **`test-wallet-dedup.js`** — Unit tests for deduplication logic
  - Tests duplicate detection
  - Tests merge strategies
  - Tests report generation
  - Usage: `CONFIG_PATH=~/automation-monorepo-config node test-wallet-dedup.js`

- **`test-wallet-dedup-today.js`** — Safe dry-run test on TODAY's data only
  - Limits scope to current day
  - Prevents data loss beyond today
  - Shows merge preview
  - Usage: `CONFIG_PATH=~/automation-monorepo-config node test-wallet-dedup-today.js`

### Legacy Scripts

- **`deduplicate-wallet-real.js`** — Detailed analysis and API command generation
  - Shows exact curl commands for deletions/updates
  - Doesn't execute API calls
  - Useful for manual execution or inspection

- **`execute-wallet-dedup.sh`** — Bash wrapper for deduplication workflow
  - Creates backup directory
  - Runs analysis
  - Displays results

## Backup & Revert

For complete backup and revert documentation, see: `../../docs/BACKUP_AND_REVERT_GUIDE.md`

### Key Locations

- **Backup directory**: `~/automation-monorepo-config/backups/wallet-dedup/`
- **Before-state backup**: `wallet-before-{TIMESTAMP}.json`
- **Change log**: `wallet-changelog-{TIMESTAMP}.json`
- **Configuration**: `~/automation-monorepo-config/config/wallet/config.yaml`

## Configuration

All scripts read configuration from `~/automation-monorepo-config/`:

```bash
export CONFIG_PATH=~/automation-monorepo-config
export WALLET_API_TOKEN="your-api-token"  # For real API operations
```

## Typical Workflow

### 1. Analyze (Safe, no changes)
```bash
CONFIG_PATH=~/automation-monorepo-config node safe-deduplicate-wallet.js
```
This shows:
- Current wallet state
- Records grouped by source version
- What will be deleted
- What will be updated
- Where to find backup files

### 2. Review Results
```bash
cat ~/automation-monorepo-config/backups/wallet-dedup/wallet-before-*.json
cat ~/automation-monorepo-config/backups/wallet-dedup/wallet-changelog-*.json
```

### 3. Execute (Only when ready)
```bash
CONFIG_PATH=~/automation-monorepo-config SKIP_CONFIRMATION=true node deduplicate-real-wallet.js
```

### 4. Verify
Check the Wallet app to confirm deduplication succeeded.

### 5. Revert (If needed)
See `../../docs/BACKUP_AND_REVERT_GUIDE.md` for complete revert procedures.

## Understanding Source Code Version Tracking

Each record has metadata identifying which code version created/updated it:

- `source_code_version` — Which code version (e.g., "unknown-manual-entry", "restructure-architecture-worktree")
- `created_by` — Who created it (e.g., "manual-web-entry", "framework-gmail-sync")
- `created_at` — ISO timestamp of creation

This helps:
- Identify which operations created which records
- Trace issues back to specific code versions
- Safely handle multiple versions of the framework

## Examples

### Safe Analysis Only
```bash
cd packs/expense-domain/sources/wallet/scripts
CONFIG_PATH=~/automation-monorepo-config node safe-deduplicate-wallet.js
# Shows analysis, no changes, no API calls
```

### Test on Today's Data
```bash
CONFIG_PATH=~/automation-monorepo-config node test-wallet-dedup-today.js
# Tests only today's records, unlimited dry-run
```

### Execute Real Deduplication
```bash
export WALLET_API_TOKEN="your-premium-token"
CONFIG_PATH=~/automation-monorepo-config SKIP_CONFIRMATION=true node deduplicate-real-wallet.js
# Actually deletes duplicates, updates merged records in Wallet API
```

### Revert If Something Goes Wrong
```bash
# See BACKUP_AND_REVERT_GUIDE.md for complete procedures
cd docs
cat BACKUP_AND_REVERT_GUIDE.md | grep "Step-by-Step Revert"
```

## Notes

- All scripts read configuration from external `~/automation-monorepo-config/` directory
- Backups are NEVER deleted automatically
- Source code version tracking helps identify which code versions are operating on wallet data
- Multiple versions of the framework can safely coexist if properly tracked
