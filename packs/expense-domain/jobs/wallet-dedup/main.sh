#!/bin/bash
set -e

CONFIG_PATH="${CONFIG_PATH:-$HOME/automation-monorepo-config}"

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "🎯 WALLET DEDUPLICATION"
echo "═══════════════════════════════════════════════════════════════"
echo ""

# Load token
TOKEN=$(grep "WALLET_API_TOKEN:" "$CONFIG_PATH/config/wallet/config.yaml" | cut -d'"' -f2)
if [ -z "$TOKEN" ]; then
  echo "❌ WALLET_API_TOKEN not found"
  exit 1
fi

echo "📁 Config: $CONFIG_PATH"
echo "🔐 API: https://rest.budgetbakers.com/wallet"
echo ""

# Fetch records
echo "📥 Fetching wallet records..."
RESPONSE=$(curl -s "https://rest.budgetbakers.com/wallet/v1/api/records?limit=500&offset=0&recordDate=gte.2000-01-01&withTotal=true" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: application/json")

RECORDS=$(echo "$RESPONSE" | jq '.records')
RECORD_COUNT=$(echo "$RECORDS" | jq 'length')
TOTAL=$(echo "$RESPONSE" | jq '.total // 0')

echo "✅ Fetched $RECORD_COUNT records (total: $TOTAL)"
echo ""

if [ "$RECORD_COUNT" -eq 0 ]; then
  echo "✅ No records found"
  exit 0
fi

# Show first few records as example
echo "📋 Sample records:"
echo "$RECORDS" | jq '.[0:2] | .[] | {merchant: .counterParty, amount: .amount.value, date: .recordDate}' 2>/dev/null || true
echo ""

echo "✅ Ready for interactive deduplication"
echo ""
echo "To implement full dedup with approvals, extend this script with:"
echo "  - Duplicate detection logic"
echo "  - Interactive approval prompts"
echo "  - Backup creation"
echo "  - Real deletions via API"
