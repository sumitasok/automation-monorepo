#!/bin/bash
# Unified Wallet Sync - Main Orchestrator
# Single entry point: gmail scan/discover -> extract -> categorize/label/describe -> wallet push
#   CONFIG_PATH=~/automation-monorepo-config ./scripts/wallet-sync.sh [--dry-run] [--since YYYY-MM-DD] [--no-ai-assist]
#
# AI-assist (for emails matching no known format: ask AI to suggest and save a
# new pattern, then retry within this same run) is ON BY DEFAULT whenever an
# AI profile exists at config/ai/<name>.yaml (ADR 0015 — the same profile
# `auto run <job> --ai <name>` uses elsewhere in this workspace). Profile name
# defaults to "deepseek" (the one profile this workspace has configured);
# override with WALLET_SYNC_AI_PROFILE=<name>. Pass --no-ai-assist to disable
# for one run (e.g. to avoid API costs on an ad-hoc/manual run), or --ai-assist
# to force it on even without a profile file (will then no-op if unconfigured).

set -euo pipefail

# Configuration
CONFIG_PATH="${CONFIG_PATH:-$HOME/automation-monorepo-config}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../.." && pwd)"
SCRIPT_DIR="$REPO_ROOT/packs/expense-domain/sources/wallet/jobs/wallet-sync-unified"
DATA_DIR="$CONFIG_PATH/data/expense-domain/wallet"
AI_PROFILE="${WALLET_SYNC_AI_PROFILE:-deepseek}"

# Parse flags
DRY_RUN=false
SINCE=""
AI_ASSIST=""   # tri-state: "" = auto (default-on if a profile exists), true/false = explicit
while [ $# -gt 0 ]; do
  case "$1" in
    --dry-run) DRY_RUN=true; shift ;;
    --since) SINCE="$2"; shift 2 ;;
    --config-path) CONFIG_PATH="$2"; shift 2 ;;
    --ai-assist) AI_ASSIST=true; shift ;;
    --no-ai-assist) AI_ASSIST=false; shift ;;
    *) shift ;;
  esac
done

# Load the named AI profile (ADR 0015) and inject the same env vars
# `auto run --ai <name>` would — AI_PROVIDER plus DEEPSEEK_*/ANTHROPIC_* —
# so extract-engine.py's existing get_ai_provider() picks it up unchanged.
AI_PROFILE_FILE="$CONFIG_PATH/config/ai/$AI_PROFILE.yaml"
AI_PROFILE_LOADED=false
if [ -f "$AI_PROFILE_FILE" ]; then
  AI_ENV="$(python3 -c "
import sys, shlex
import yaml
p = yaml.safe_load(open(sys.argv[1])) or {}
provider = str(p.get('provider') or '').lower()
api_key = str(p.get('api_key') or '')
model = str(p.get('model') or '')
api_base = str(p.get('api_base') or '')
if not provider or not api_key:
    sys.exit(0)
provider = 'claude' if provider in ('claude', 'anthropic') else 'deepseek'
print(f'export AI_PROVIDER={shlex.quote(provider)}')
if provider == 'deepseek':
    print(f'export DEEPSEEK_API_KEY={shlex.quote(api_key)}')
    if model: print(f'export DEEPSEEK_MODEL={shlex.quote(model)}')
    if api_base: print(f'export DEEPSEEK_API_BASE={shlex.quote(api_base)}')
else:
    print(f'export ANTHROPIC_API_KEY={shlex.quote(api_key)}')
    if model: print(f'export ANTHROPIC_MODEL={shlex.quote(model)}')
" "$AI_PROFILE_FILE")"
  if [ -n "$AI_ENV" ]; then
    eval "$AI_ENV"
    AI_PROFILE_LOADED=true
  fi
fi

# Default: on when a usable profile was loaded and the user didn't override.
if [ -z "$AI_ASSIST" ]; then
  AI_ASSIST="$AI_PROFILE_LOADED"
fi

# Verify config exists
if [ ! -f "$CONFIG_PATH/config/expense-domain/wallet/config.yaml" ]; then
  echo "❌ Config not found: $CONFIG_PATH/config/expense-domain/wallet/config.yaml"
  exit 3
fi

# Create directories
mkdir -p "$DATA_DIR/logs"

# Setup Python environment
export PYTHONPATH="$SCRIPT_DIR:${PYTHONPATH:-}"

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
echo "🤖 AI-assist: $([ "$AI_ASSIST" = "true" ] && echo "ON (profile: $AI_PROFILE)" || echo "off")"
echo "🐍 Script: $SCRIPT_DIR/sync.py"
echo ""

# Run sync.py with all required parameters
export CONFIG_PATH="$CONFIG_PATH"
export AUTO_DATA_DIR="$DATA_DIR"
export WALLET_AUTH_HEADER="Bearer $WALLET_TOKEN"

python3 "$SCRIPT_DIR/sync.py" \
  $([ "$DRY_RUN" = "true" ] && echo "--dry-run" || echo "") \
  $([ -n "$SINCE" ] && echo "--since $SINCE" || echo "") \
  $([ "$AI_ASSIST" = "true" ] && echo "--ai-assist" || echo "") \
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
