# Import Path Migration Guide

This document tracks the import path updates needed for the expense-domain restructuring.

## Changes Made

### Directory Structure
```
OLD: packs/expenses/
NEW: packs/expense-domain/engine/main/

OLD: packs/gmail/
NEW: packs/expense-domain/sources/gmail/

OLD: packs/wallet/
NEW: packs/expense-domain/sources/wallet/

OLD: packs/telegram/
NEW: packs/expense-domain/sources/telegram/
```

## Import Updates Needed

### Go Module Path
- **File**: `packs/expense-domain/engine/main/go.mod`
- **Current**: `module github.com/sumitasok/sa.automation.expenses`
- **Status**: No change required (internal module name)
- **Reason**: Module name is internal and doesn't need to reflect filesystem structure

### Hardcoded File Paths
- **File**: `packs/expense-domain/engine/main/main.go`
- **Pattern**: `../../data/config/expense-rules.yaml`
- **Update to**: Use injected `CONFIG_PATH` environment variable
- **Implementation**: Replace hardcoded paths with `os.Getenv("CONFIG_PATH")`

### Source Adapter Imports
- **Gmail adapter**: `packs/expense-domain/sources/gmail/`
  - Check for any cross-pack imports
  - Update relative paths to point to new locations

- **Wallet adapter**: `packs/expense-domain/sources/wallet/`
  - Check for any cross-pack imports
  - Update relative paths

## Configuration Injection Updates

Instead of:
```go
rulesFile := "../../data/config/expense-rules.yaml"
```

Use:
```go
configPath := os.Getenv("CONFIG_PATH")
if configPath == "" {
  configPath = os.ExpandEnv("$HOME/automation-monorepo-config")
}
rulesFile := filepath.Join(configPath, "rules", "expense-domain", "engine", "expense-rules.yaml")
```

## Environment Variables Available

When running under framework:
- `CONFIG_PATH`: Path to ~/automation-monorepo-config/
- `DOMAIN_NAME`: Domain name (expense-domain)
- `DATA_DIR`: Domain data directory
- `RULES_DIR`: Domain rules directory

## Testing Import Updates

After updates, verify:
1. `go build` succeeds
2. All imports resolve
3. Tests pass with injected paths
4. Framework can inject CONFIG_PATH correctly

## Status

- [ ] Update go.mod if needed
- [ ] Replace hardcoded paths with environment variables
- [ ] Update source adapter cross-pack imports
- [ ] Update relative paths in config references
- [ ] Test with framework configuration injection
- [ ] Update RUNBOOK.md with new paths
