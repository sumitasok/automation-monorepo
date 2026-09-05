#!/bin/bash
# Unified Wallet Sync - Main Orchestrator
# Single entry point: CONFIG_PATH=~/automation-monorepo-config ./scripts/wallet-sync-unified.sh

set -euo pipefail

# Configuration
CONFIG_PATH="${CONFIG_PATH:-$HOME/automation-monorepo-config}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
SCRIPT_DIR="$REPO_ROOT/packs/expense-domain/sources/wallet/jobs/wallet-sync-unified"
DATA_DIR="$CONFIG_PATH/data/expense-domain/wallet"

# Verify config exists
if [ ! -f "$CONFIG_PATH/config/expense-domain/wallet/config.yaml" ]; then
  echo "❌ Config not found: $CONFIG_PATH/config/expense-domain/wallet/config.yaml"
  exit 3
fi

# Create directories
mkdir -p "$DATA_DIR/logs"

# Setup Python environment
export PYTHONPATH="$SCRIPT_DIR:$PYTHONPATH"

# Extract Wallet API token from config
WALLET_TOKEN=$(grep "api_token:" "$CONFIG_PATH/config/expense-domain/wallet/config.yaml" | cut -d'"' -f2)
if [ -z "$WALLET_TOKEN" ]; then
  echo "❌ WALLET_API_TOKEN not configured"
  exit 4
fi

# Log file
LOG_FILE="$DATA_DIR/logs/sync-$(date +%Y%m%d-%H%M%S).log"

echo "═══════════════════════════════════════════════════════════════"
echo "🎯 WALLET SYNC — Unified"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "📁 Config: $CONFIG_PATH"
echo "📊 Mode: $([ "$DRY_RUN" = "true" ] && echo "DRY-RUN (no writes)" || echo "NORMAL (writes enabled)")"
echo ""

# Run sync.py with all required parameters
export CONFIG_PATH="$CONFIG_PATH"
export AUTO_DATA_DIR="$DATA_DIR"
export WALLET_API_TOKEN="$WALLET_TOKEN"

python3 "$SCRIPT_DIR/sync.py" \
  --config-path "$CONFIG_PATH" \
  $([ "${DRY_RUN:-false}" = "true" ] && echo "--dry-run" || echo "") \
  $([ "${SINCE:-}" != "" ] && echo "--since $SINCE" || echo "") \
  2>&1 | tee -a "$LOG_FILE"

EXIT_CODE=${PIPESTATUS[0]}

echo ""
echo "═══════════════════════════════════════════════════════════════"
if [ $EXIT_CODE -eq 0 ]; then
  echo "✅ SYNC COMPLETE"
else
  echo "❌ SYNC FAILED (exit code: $EXIT_CODE)"
fi
echo "═══════════════════════════════════════════════════════════════"
echo ""

exit $EXIT_CODE
