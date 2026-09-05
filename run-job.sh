#!/bin/bash
# Universal job runner - specify config path once, then just pass job name
# Usage:
#   ./run-job.sh wallet-dedup-today
#   ./run-job.sh wallet-dedup-today --execute

set -e

# Get config path (prompt if not set)
if [ -z "$CONFIG_PATH" ]; then
  read -p "Config path [~/automation-monorepo-config]: " CONFIG_PATH
  CONFIG_PATH="${CONFIG_PATH:-$HOME/automation-monorepo-config}"
fi

# Expand ~ if needed
CONFIG_PATH="${CONFIG_PATH/\~/$HOME}"

# Verify config exists
if [ ! -d "$CONFIG_PATH" ]; then
  echo "❌ Config directory not found: $CONFIG_PATH"
  exit 1
fi

# Get job name from first argument
JOB_NAME="$1"
if [ -z "$JOB_NAME" ]; then
  echo "Usage: $0 <job-name> [args...]"
  echo ""
  echo "Examples:"
  echo "  $0 wallet-dedup-today"
  echo "  $0 wallet-dedup-today --execute"
  exit 1
fi

# Shift args to pass remaining to job script
shift

# Auto-discover job script location
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$SCRIPT_DIR"

# Try to find the job script
JOB_SCRIPT=""
if [ -f "$SCRIPT_DIR/packs/expense-domain/sources/wallet/scripts/$JOB_NAME.sh" ]; then
  JOB_SCRIPT="$SCRIPT_DIR/packs/expense-domain/sources/wallet/scripts/$JOB_NAME.sh"
elif [ -f "$SCRIPT_DIR/scripts/$JOB_NAME.sh" ]; then
  JOB_SCRIPT="$SCRIPT_DIR/scripts/$JOB_NAME.sh"
else
  echo "❌ Job script not found: $JOB_NAME"
  echo ""
  echo "Searched:"
  echo "  - $SCRIPT_DIR/packs/expense-domain/sources/wallet/scripts/$JOB_NAME.sh"
  echo "  - $SCRIPT_DIR/scripts/$JOB_NAME.sh"
  exit 1
fi

# Load credentials from config
if [ ! -f "$CONFIG_PATH/config/wallet/config.yaml" ]; then
  echo "❌ Wallet config not found: $CONFIG_PATH/config/wallet/config.yaml"
  exit 1
fi

# Extract Wallet API token
WALLET_TOKEN=$(grep "WALLET_API_TOKEN:" "$CONFIG_PATH/config/wallet/config.yaml" | cut -d'"' -f2)
if [ -z "$WALLET_TOKEN" ]; then
  echo "❌ WALLET_API_TOKEN not found in config"
  exit 1
fi

# Set up environment for the job
export CONFIG_PATH="$CONFIG_PATH"
export WALLET_API_TOKEN="$WALLET_TOKEN"
export AUTO_DATA_DIR="$CONFIG_PATH/data/expense-domain/wallet"
export REPO_ROOT="$REPO_ROOT"

# Create data directories if needed
mkdir -p "$AUTO_DATA_DIR"

# Run the job
echo "🚀 Running job: $JOB_NAME"
echo "   Config: $CONFIG_PATH"
echo "   Data: $AUTO_DATA_DIR"
echo ""

exec "$JOB_SCRIPT" "$@"
