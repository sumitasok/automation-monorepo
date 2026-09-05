#!/bin/bash
# Execute Real Wallet Deduplication
# This script fetches wallet records and removes duplicates

set -e

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "🗑️  WALLET DEDUPLICATION - EXECUTION SCRIPT"
echo "═══════════════════════════════════════════════════════════════"
echo ""

# Check for Wallet API key
if [ -z "$WALLET_API_KEY" ]; then
  echo "❌ WALLET_API_KEY not set"
  echo ""
  echo "Set it with:"
  echo "  export WALLET_API_KEY='your-api-key'"
  echo ""
  exit 1
fi

WALLET_API="${WALLET_API_BASE:-https://api.wallet.example.com}"
OUTPUT_DIR="./wallet-dedup-backup"

echo "📁 Creating backup directory: $OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"
echo ""

echo "📥 Fetching current wallet records..."
RECORDS_FILE="$OUTPUT_DIR/records-$(date +%Y%m%d-%H%M%S).json"

curl -s "$WALLET_API/records" \
  -H "Authorization: Bearer $WALLET_API_KEY" \
  -H "Accept: application/json" \
  > "$RECORDS_FILE"

if [ ! -s "$RECORDS_FILE" ]; then
  echo "❌ Failed to fetch records"
  exit 1
fi

RECORD_COUNT=$(jq 'length' "$RECORDS_FILE")
echo "✅ Fetched $RECORD_COUNT records"
echo "   Saved to: $RECORDS_FILE"
echo ""

echo "🔍 Analyzing for duplicates..."
echo ""

# Run deduplication analysis
node scripts/deduplicate-wallet-real.js "$RECORDS_FILE" > "$OUTPUT_DIR/dedup-analysis.txt"

echo "📊 Analysis results:"
grep -E "(DELETE|UPDATE|Merchant|Labels)" "$OUTPUT_DIR/dedup-analysis.txt" | head -20

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "⚠️  REVIEW BEFORE PROCEEDING"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "Full analysis saved to: $OUTPUT_DIR/dedup-analysis.txt"
echo ""
echo "Review the DELETE and UPDATE commands before executing!"
echo ""
echo "To see full details:"
echo "  cat $OUTPUT_DIR/dedup-analysis.txt"
echo ""
echo "To execute deduplication, you will need to:"
echo "  1. Review each DELETE command"
echo "  2. Review each UPDATE command"
echo "  3. Execute them in order"
echo ""
echo "After execution, verify:"
echo "  curl $WALLET_API/records \\
    -H \"Authorization: Bearer \$WALLET_API_KEY\" | jq '.[] | select(._is_duplicate == true)'"
echo ""
