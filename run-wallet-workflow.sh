#!/bin/bash
set -e

# Wallet Sync Workflow - Complete pipeline
# Runs: gmail-extract → gmail-categorize → wallet-sync-categories →
#       wallet-fetch-accounts → wallet-dedup → wallet-sync

echo "================================================================================"
echo "WALLET SYNC WORKFLOW - Starting"
echo "================================================================================"
echo "Pipeline: gmail-extract → gmail-categorize → wallet-sync-categories →"
echo "          wallet-fetch-accounts → wallet-dedup → wallet-sync"
echo "================================================================================"
echo ""

START_TIME=$(date +%s)

# Step 1: Extract Gmail transactions
echo "[1/6] GMAIL-EXTRACT: Extracting transactions from Gmail..."
echo ""
if ./auto run gmail-extract; then
    echo ""
    echo "✓ Gmail extraction complete"
else
    echo ""
    echo "✗ Gmail extraction failed"
    exit 1
fi
echo ""

# Step 2: Categorize Gmail transactions
echo "[2/6] GMAIL-CATEGORIZE: AI-categorizing transactions..."
echo ""
if ./auto run gmail-categorize; then
    echo ""
    echo "✓ Gmail categorization complete"
else
    echo ""
    echo "✗ Gmail categorization failed"
    exit 1
fi
echo ""

# Step 3: Sync categories to wallet
echo "[3/6] WALLET-SYNC-CATEGORIES: Syncing categories to wallet Unknown records..."
echo ""
if ./auto run wallet-sync-categories -- --apply; then
    echo ""
    echo "✓ Category sync complete"
else
    echo ""
    echo "✗ Category sync failed"
    exit 1
fi
echo ""

# Step 4: Fetch accounts
echo "[4/6] WALLET-FETCH-ACCOUNTS: Fetching accounts from Wallet API..."
echo ""
if ./auto run wallet-fetch-accounts; then
    echo ""
    echo "✓ Account fetch complete"
else
    echo ""
    echo "✗ Account fetch failed"
    exit 1
fi
echo ""

# Step 5: Detect and dedup duplicates
echo "[5/6] WALLET-DEDUP: Scanning for duplicate records..."
echo ""
if ./auto run wallet-dedup scan; then
    echo ""
    echo "✓ Duplicate detection complete"
else
    echo ""
    echo "✗ Duplicate detection failed"
    exit 1
fi
echo ""

# Step 6: Sync to Wallet Server
echo "[6/6] WALLET-SYNC: Pushing transactions to Wallet Server..."
echo ""
if ./auto run wallet-sync; then
    echo ""
    echo "✓ Wallet sync complete"
else
    echo ""
    echo "✗ Wallet sync failed"
    exit 1
fi
echo ""

# Calculate elapsed time
END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))
MINUTES=$((ELAPSED / 60))
SECONDS=$((ELAPSED % 60))

echo "================================================================================"
echo "✅ WORKFLOW COMPLETE"
echo "================================================================================"
echo "Total time: ${MINUTES}m ${SECONDS}s"
echo ""
echo "Pipeline executed:"
echo "  ✓ [1/6] Gmail extraction"
echo "  ✓ [2/6] Gmail categorization (AI)"
echo "  ✓ [3/6] Wallet category sync (to Unknown records)"
echo "  ✓ [4/6] Account fetch"
echo "  ✓ [5/6] Duplicate detection"
echo "  ✓ [6/6] Transaction push to Wallet Server"
echo ""
echo "Next steps:"
echo "  • Review duplicate scan results: ./auto run wallet-dedup review"
echo "  • Execute dedup if needed: ./auto run wallet-dedup execute"
echo "  • Rerun this workflow: ./run-wallet-workflow.sh"
echo "================================================================================"
