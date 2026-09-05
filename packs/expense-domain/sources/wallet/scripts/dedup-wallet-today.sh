#!/bin/bash
# Dedup wallet entries from today using the Go dedup system
# Workflow: fetch today's records → scan for duplicates → review → execute

set -e

CONFIG_PATH="${CONFIG_PATH:-$HOME/automation-monorepo-config}"
DATA_DIR="$CONFIG_PATH/data/expense-domain/wallet"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../../../../" && pwd)"

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "🎯 WALLET DEDUPLICATION - TODAY'S ENTRIES"
echo "═══════════════════════════════════════════════════════════════"
echo ""

# Verify config exists
if [ ! -f "$CONFIG_PATH/config/wallet/config.yaml" ]; then
  echo "❌ Config not found: $CONFIG_PATH/config/wallet/config.yaml"
  exit 1
fi

# Extract Wallet API token from config
WALLET_TOKEN=$(grep "WALLET_API_TOKEN:" "$CONFIG_PATH/config/wallet/config.yaml" | cut -d'"' -f2)
if [ -z "$WALLET_TOKEN" ]; then
  echo "❌ WALLET_API_TOKEN not found in config"
  exit 1
fi

WALLET_BASE_URL="https://rest.budgetbakers.com/wallet"

# Create data directory if needed
mkdir -p "$DATA_DIR"

# Get today's date range (UTC)
TODAY=$(date -u '+%Y-%m-%d')
TOMORROW=$(date -u -v+1d '+%Y-%m-%d')

echo "📁 Config: $CONFIG_PATH"
echo "📅 Date range: $TODAY to $TOMORROW (UTC)"
echo "🔐 API: $WALLET_BASE_URL"
echo ""

# Fetch today's records from Wallet API
echo "📥 Fetching today's wallet records..."
RECORDS_FILE="$DATA_DIR/records-today.jsonl"
TEMP_RESPONSE=$(mktemp)

curl -s -X GET \
  "$WALLET_BASE_URL/v1/api/records?limit=500&offset=0&recordDate=gte.$TODAY&recordDate=lt.$TOMORROW&withTotal=true" \
  -H "Authorization: Bearer $WALLET_TOKEN" \
  -H "Accept: application/json" > "$TEMP_RESPONSE"

# Extract records from response
RECORD_COUNT=$(jq '.records | length' "$TEMP_RESPONSE" 2>/dev/null || echo "0")
TOTAL=$(jq '.total // 0' "$TEMP_RESPONSE" 2>/dev/null || echo "0")

if [ "$RECORD_COUNT" = "0" ] || [ "$RECORD_COUNT" = "null" ]; then
  echo "✅ No records found for today ($TODAY)"
  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "✅ ALL CLEAR - No deduplication needed"
  echo "═══════════════════════════════════════════════════════════════"
  rm -f "$TEMP_RESPONSE"
  exit 0
fi

echo "✅ Fetched $RECORD_COUNT records from today"
echo "   (Total in wallet: $TOTAL)"
echo ""

# Save records to JSONL format (one record per line)
jq -r '.records[]' "$TEMP_RESPONSE" | while read -r record; do
  echo "$record" >> "$RECORDS_FILE"
done

rm -f "$TEMP_RESPONSE"

echo "💾 Records saved to: $RECORDS_FILE"
echo ""

# Run Go dedup scan
echo "═══════════════════════════════════════════════════════════════"
echo "🔍 SCANNING FOR DUPLICATES"
echo "═══════════════════════════════════════════════════════════════"
echo ""

# Check if dedup binary exists
DEDUP_BIN="$REPO_ROOT/packs/expense-domain/sources/wallet/dedup"
if [ ! -f "$DEDUP_BIN" ] || [ ! -x "$DEDUP_BIN" ]; then
  echo "⚠️  Dedup binary not found or not executable: $DEDUP_BIN"
  echo "   Building from source..."
  cd "$REPO_ROOT/packs/expense-domain/sources/wallet"
  go build -o dedup ./main.go dedup.go
  echo "   ✅ Built dedup binary"
fi

# Run scan
DEDUP_REPORT="$DATA_DIR/dedup-scan-$TODAY.json"
export AUTO_DATA_DIR="$DATA_DIR"
"$DEDUP_BIN" dedup scan \
  --records-file "$RECORDS_FILE" \
  --format json > "$DEDUP_REPORT"

DUPLICATE_COUNT=$(jq '.duplicateGroupsFound // 0' "$DEDUP_REPORT")

echo "✅ Scan complete"
echo "   Groups with duplicates: $DUPLICATE_COUNT"
echo "   Report saved: $DEDUP_REPORT"
echo ""

if [ "$DUPLICATE_COUNT" = "0" ] || [ "$DUPLICATE_COUNT" = "null" ]; then
  echo "═══════════════════════════════════════════════════════════════"
  echo "✅ ALL CLEAR - No duplicates found in today's records"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  exit 0
fi

# Show preview of duplicates found
echo "📊 Duplicate Groups Found:"
jq '.groups[] | {key: .duplicateKey, count: (.records | length), amounts: [.records[].amount]}' "$DEDUP_REPORT"
echo ""

# Run review (collect decisions)
echo "═══════════════════════════════════════════════════════════════"
echo "👤 REVIEWING DUPLICATES"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "⚠️  You will be prompted to decide which records to keep."
echo ""

DECISIONS_FILE="$DATA_DIR/dedup-decisions-$TODAY.json"
export AUTO_DATA_DIR="$DATA_DIR"

"$DEDUP_BIN" dedup review \
  --records-file "$RECORDS_FILE" \
  --decisions-file "$DECISIONS_FILE"

echo ""
echo "✅ Decisions saved to: $DECISIONS_FILE"
echo ""

# Show what will happen
echo "═══════════════════════════════════════════════════════════════"
echo "📋 EXECUTION PLAN"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "To execute the deduplication:"
echo ""
echo "  export AUTO_DATA_DIR=\"$DATA_DIR\""
echo "  export WALLET_API_TOKEN=\"$WALLET_TOKEN\""
echo "  \"$DEDUP_BIN\" dedup execute \\"
echo "    --records-file \"$RECORDS_FILE\" \\"
echo "    --decisions-file \"$DECISIONS_FILE\""
echo ""
echo "OR use the wrapper:"
echo ""
echo "  CONFIG_PATH=\"$CONFIG_PATH\" $(basename "$0") --execute"
echo ""
