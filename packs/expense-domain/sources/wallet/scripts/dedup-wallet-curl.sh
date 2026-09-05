#!/bin/bash
# Wallet Deduplication using curl (works around Node.js HTTPS header issues)

set -e

CONFIG_PATH="${CONFIG_PATH:-$HOME/automation-monorepo-config}"

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "🎯 WALLET DEDUPLICATION - CURL VERSION"
echo "═══════════════════════════════════════════════════════════════"
echo ""

# Load token from config
if [ ! -f "$CONFIG_PATH/config/wallet/config.yaml" ]; then
  echo "❌ Config not found: $CONFIG_PATH/config/wallet/config.yaml"
  exit 1
fi

WALLET_TOKEN=$(grep "WALLET_API_TOKEN:" "$CONFIG_PATH/config/wallet/config.yaml" | cut -d'"' -f2)
WALLET_BASE_URL="https://rest.budgetbakers.com/wallet"

if [ -z "$WALLET_TOKEN" ]; then
  echo "❌ WALLET_API_TOKEN not found in config"
  exit 1
fi

echo "📁 Config: $CONFIG_PATH"
echo "🔐 API: $WALLET_BASE_URL"
echo ""

# Fetch records
echo "📥 Fetching wallet records from API..."
RECORDS=$(curl -s -X GET \
  "$WALLET_BASE_URL/v1/api/records?limit=500&offset=0&recordDate=gte.2000-01-01&withTotal=true" \
  -H "Authorization: Bearer $WALLET_TOKEN" \
  -H "Accept: application/json")

RECORD_COUNT=$(echo "$RECORDS" | jq '.records | length')
TOTAL=$(echo "$RECORDS" | jq '.total // 0')

echo "✅ Fetched $RECORD_COUNT records (total in wallet: $TOTAL)"
echo ""

# Save records to temp file for processing
TEMP_RECORDS=$(mktemp)
echo "$RECORDS" | jq '.records' > "$TEMP_RECORDS"

echo "═══════════════════════════════════════════════════════════════"
echo "📋 RECORDS FETCHED"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "Run Node.js deduplication after checking records:"
echo ""
echo "  cat $TEMP_RECORDS | jq '.' | head -20"
echo ""
echo "Then continue with deduplication:"
echo ""
echo "  CONFIG_PATH=$CONFIG_PATH WALLET_TOKEN='$WALLET_TOKEN' \\"
echo "    node packs/expense-domain/sources/wallet/scripts/dedup-wallet-process.js"
echo ""
echo "Records saved to: $TEMP_RECORDS"
echo ""
