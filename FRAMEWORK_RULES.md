# Automation Monorepo Framework Rules

## Rule 1: Single Environment Variable for All Operations

**Principle**: All framework operations require only one environment variable: `CONFIG_PATH`

```bash
CONFIG_PATH=~/automation-monorepo-config <command> <sub-command> [--filters]
```

**No exceptions**: 
- ❌ Never reference script paths directly
- ❌ Never hardcode data directories
- ❌ Never specify credentials in commands
- ✅ Single CONFIG_PATH handles everything

## Rule 2: Command Discovery and Auto-Resolution

**How it works**:
1. Framework reads `CONFIG_PATH` (defaults to `~/automation-monorepo-config`)
2. Discovers command location automatically
3. Extracts credentials from `$CONFIG_PATH/config/{domain}/config.yaml`
4. Creates data directories as needed
5. Executes job with full context

**Search path for commands**:
```
$REPO_ROOT/packs/{domain}/sources/{service}/scripts/{command}.sh
$REPO_ROOT/scripts/{command}.sh
```

## Rule 3: Command Naming Convention

```
{service}-{action}-{scope}

Examples:
  dedup-wallet-today       # Service: dedup, domain: wallet, scope: today
  sync-gmail-calendar      # Service: sync, domain: gmail, scope: calendar
  fetch-accounts           # Service: fetch, domain: implicit, scope: accounts
```

## Rule 4: Filter Arguments

All commands accept `--filters` for parameterization:
```bash
CONFIG_PATH=~/automation-monorepo-config dedup-wallet-today --no-confirm
CONFIG_PATH=~/automation-monorepo-config dedup-wallet-today --dry-run
```

## Rule 5: Standard Output Format

All commands output:
```
🚀 Running: <command>
   Config: $CONFIG_PATH
   Domain: <domain>
   
[command-specific output]

✅ COMPLETE
```

## Implementation

The `dedup-wallet-today` command pattern:
```bash
CONFIG_PATH=~/automation-monorepo-config dedup-wallet-today
```

This single invocation:
- ✅ Loads config from ~/automation-monorepo-config
- ✅ Finds script at packs/expense-domain/sources/wallet/scripts/dedup-wallet-today.sh
- ✅ Extracts WALLET_API_TOKEN from config/wallet/config.yaml
- ✅ Creates data directories
- ✅ Sets up environment
- ✅ Executes job
- ✅ Runs to completion (scan → review → execute)

## No File Path References

**Before** (❌ Old way):
```bash
./run-job.sh dedup-wallet-today
./packs/expense-domain/sources/wallet/scripts/dedup-wallet-today.sh
export CONFIG_PATH=... WALLET_TOKEN=... AUTO_DATA_DIR=...
```

**After** (✅ Framework way):
```bash
CONFIG_PATH=~/automation-monorepo-config dedup-wallet-today
```

That's it. Everything else is auto-discovered.
