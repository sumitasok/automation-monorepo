#!/bin/bash
# Wallet sync orchestration with Telegram notifications
# Runs via launchd every 4 hours

set -e

# (zshrc is sourced by launchd)

# Set working directory
cd /Users/sumitasok/Claude/Projects/automation-monorepo

# Log file with timestamp
LOG_FILE="/tmp/wallet-sync-$(date +%Y%m%d-%H%M%S).log"

echo "Starting wallet sync orchestration at $(date)" | tee -a "$LOG_FILE"

# Run orchestration and capture result
if ./auto orchestrate gmail-wallet-sync >> "$LOG_FILE" 2>&1; then
    SUCCESS=true
    MSG="✅ Wallet sync completed successfully at $(date '+%Y-%m-%d %H:%M:%S')"
else
    SUCCESS=false
    MSG="❌ Wallet sync failed at $(date '+%Y-%m-%d %H:%M:%S') - Check logs: $LOG_FILE"
fi

# Send Telegram notification
if command -v telegram_message &> /dev/null; then
    telegram_message "$MSG"
else
    echo "Warning: telegram_message command not found, skipping notification" | tee -a "$LOG_FILE"
fi

# Exit with proper code
if [ "$SUCCESS" = true ]; then
    echo "Wallet sync completed successfully" | tee -a "$LOG_FILE"
    exit 0
else
    echo "Wallet sync failed" | tee -a "$LOG_FILE"
    exit 1
fi
