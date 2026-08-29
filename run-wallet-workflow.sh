#!/bin/bash
set -e

# Wallet Sync Workflow - Complete pipeline
# Runs: gmail-extract → gmail-categorize → wallet-sync-categories →
#       wallet-fetch-accounts → wallet-dedup → wallet-sync

echo "================================================================================"
echo "WALLET SYNC WORKFLOW - Starting"
echo "================================================================================"
echo "Pipeline: wallet-fetch → gmail-extract → gmail-categorize →"
echo "          wallet-sync-categories → wallet-fetch-accounts → wallet-dedup →"
echo "          wallet-sync"
echo "================================================================================"
echo ""

START_TIME=$(date +%s)

# Step 1: Fetch wallet records
echo "[1/7] WALLET-FETCH: Fetching wallet records from Wallet API..."
echo ""
if ./auto run wallet-fetch; then
    echo ""
    echo "✓ Wallet fetch complete"
else
    echo ""
    echo "✗ Wallet fetch failed"
    exit 1
fi
echo ""

# Step 2: Extract Gmail transactions
echo "[2/7] GMAIL-EXTRACT: Extracting transactions from Gmail..."
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
echo "[3/7] GMAIL-CATEGORIZE: AI-categorizing transactions..."
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
echo "[4/7] WALLET-SYNC-CATEGORIES: Syncing categories to wallet Unknown records..."
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
echo "[5/7] WALLET-FETCH-ACCOUNTS: Fetching accounts from Wallet API..."
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
echo "[6/7] WALLET-DEDUP: Scanning for duplicate records..."
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
echo "[7/7] WALLET-SYNC: Pushing transactions to Wallet Server..."
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
echo "  ✓ [1/7] Wallet record fetch"
echo "  ✓ [2/7] Gmail extraction"
echo "  ✓ [3/7] Gmail categorization (AI)"
echo "  ✓ [4/7] Wallet category sync (to Unknown records)"
echo "  ✓ [5/7] Account fetch"
echo "  ✓ [6/7] Duplicate detection"
echo "  ✓ [7/7] Transaction push to Wallet Server"
echo ""
echo "Next steps:"
echo "  • Review duplicate scan results: ./auto run wallet-dedup review"
echo "  • Execute dedup if needed: ./auto run wallet-dedup execute"
echo "  • Rerun this workflow: ./run-wallet-workflow.sh"
echo "================================================================================"
