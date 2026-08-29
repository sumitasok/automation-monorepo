#!/usr/bin/env python3
"""
Wallet Sync Categories Job

Analyzes Gmail categorized transactions and syncs categories to Wallet records
that have "Unknown" categories. Stages updates in wallet/updates.jsonl without
modifying records.jsonl until explicitly applied via --apply flag.

Usage:
    python3 sync-categories.py                    # dry-run (default)
    python3 sync-categories.py --apply            # apply all updates to API
    python3 sync-categories.py --apply-high       # apply only high-confidence
    python3 sync-categories.py --review           # show detailed review
    python3 sync-categories.py --dry-run          # explicit dry-run

Respects auto framework environment variables:
    AUTO_DATA_DIR: Points to data directory (e.g., /Users/sumitasok/data)
    WALLET_API_TOKEN: Required for --apply flag
    AUTO_JOB_ID: Job identifier
"""

import csv
import json
import sys
import os
from datetime import datetime
from collections import defaultdict
from pathlib import Path


def get_data_dir():
    """Get data directory from AUTO_DATA_DIR or raise error."""
    data_dir = os.getenv('AUTO_DATA_DIR')
    if not data_dir:
        raise RuntimeError(
            "AUTO_DATA_DIR not set. This job must run via './auto run wallet-sync-categories'\n"
            "Running outside auto framework is not supported."
        )
    return Path(data_dir)


def main():
    # Parse arguments
    apply_all = '--apply' in sys.argv
    apply_high = '--apply-high' in sys.argv
    review = '--review' in sys.argv
    dry_run = '--dry-run' in sys.argv or (not apply_all and not apply_high)

    # Get data directory from auto framework
    try:
        data_dir = get_data_dir()
    except RuntimeError as e:
        print(f"❌ {e}", file=sys.stderr)
        sys.exit(1)

    gmail_csv = data_dir / 'gmail' / 'transactions.csv'
    wallet_jsonl = data_dir / 'wallet' / 'records.jsonl'
    updates_file = data_dir / 'wallet' / 'updates.jsonl'

    # Verify files exist
    if not gmail_csv.exists():
        print(f"❌ Gmail CSV not found: {gmail_csv}")
        sys.exit(1)
    if not wallet_jsonl.exists():
        print(f"❌ Wallet records not found: {wallet_jsonl}")
        sys.exit(1)

    print("=" * 80)
    print("WALLET SYNC CATEGORIES FROM GMAIL")
    print("=" * 80)
    print(f"\n📁 Data directory: {data_dir}")
    print(f"Mode: {'DRY-RUN' if dry_run else 'APPLY'}")

    # Read Gmail transactions
    gmail_data = []
    gmail_by_merchant = defaultdict(list)

    with open(gmail_csv, 'r') as f:
        reader = csv.DictReader(f)
        for row in reader:
            gmail_data.append(row)
            merchant = (row.get('Merchant', '') or row.get('Info', '')).lower().strip()
            if merchant:
                gmail_by_merchant[merchant].append(row)

    print(f"\n📧 Gmail transactions: {len(gmail_data)} loaded")

    # Read Wallet records with Unknown category
    wallet_unknown = []
    with open(wallet_jsonl, 'r') as f:
        for line in f:
            if line.strip():
                try:
                    record = json.loads(line)
                    cat = record.get('category', {}).get('name', '')
                    if cat in ['Unknown expense', 'Unknown income', 'Uncategorized']:
                        wallet_unknown.append(record)
                except json.JSONDecodeError:
                    pass

    print(f"💰 Wallet records: {len(wallet_unknown)} with Unknown category")

    # Find matches and create updates
    updates = []
    seen_wallet_ids = set()

    for wallet in wallet_unknown:
        if wallet['id'] in seen_wallet_ids:
            continue
        seen_wallet_ids.add(wallet['id'])

        wallet_merchant = wallet.get('counterParty', '').lower().strip()
        wallet_date_str = wallet.get('recordDate', '')
        wallet_amount = wallet.get('amount', {}).get('value', 0)
        wallet_cat_current = wallet.get('category', {}).get('name', '')

        if not wallet_merchant:
            continue

        if wallet_merchant in gmail_by_merchant:
            gmail_matches = gmail_by_merchant[wallet_merchant]

            try:
                wallet_dt = datetime.fromisoformat(wallet_date_str.replace('Z', '+00:00')).date()
            except (ValueError, TypeError):
                continue

            best_match = None
            best_day_diff = float('inf')

            for gmail in gmail_matches:
                gmail_dt_str = gmail.get('TxnDate', '')
                if gmail_dt_str:
                    try:
                        gmail_dt = datetime.strptime(gmail_dt_str.split()[0], '%Y-%m-%d').date()
                        day_diff = abs((wallet_dt - gmail_dt).days)

                        if day_diff <= 1 and day_diff < best_day_diff:
                            best_match = gmail
                            best_day_diff = day_diff
                    except ValueError:
                        pass

            if best_match:
                gmail_category = best_match.get('Category', '').strip()
                gmail_subcategory = best_match.get('SubCategory', '').strip()
                gmail_labels = best_match.get('Labels', '').strip()
                gmail_source = best_match.get('Source', '').strip()

                if gmail_category and gmail_category not in ['', 'Unknown', 'Unknown income', 'Unknown expense']:
                    update = {
                        'id': wallet['id'],
                        'action': 'PATCH',
                        'match_confidence': 'high' if best_day_diff == 0 else 'medium',
                        'wallet_current': {
                            'id': wallet['id'],
                            'merchant': wallet.get('counterParty'),
                            'date': wallet_date_str[:10],
                            'amount': wallet_amount,
                            'category_current': wallet_cat_current,
                        },
                        'gmail_source': {
                            'merchant': best_match.get('Merchant'),
                            'date': best_match.get('TxnDate'),
                            'category': gmail_category,
                            'subcategory': gmail_subcategory,
                            'labels': gmail_labels,
                            'source': gmail_source,
                        },
                        'proposed_update': {
                            'category': {'name': gmail_category}
                        },
                        'reason': f'Matched with Gmail transaction: {gmail_category}',
                        'timestamp': datetime.now().isoformat(),
                    }
                    updates.append(update)

    print(f"\n✅ Found {len(updates)} potential updates")

    # Group by category
    updates_by_category = defaultdict(list)
    for update in updates:
        cat = update['proposed_update']['category']['name']
        updates_by_category[cat].append(update)

    print(f"\n📊 Updates by Category:")
    for cat in sorted(updates_by_category.keys(), key=lambda x: len(updates_by_category[x]), reverse=True):
        print(f"  {cat}: {len(updates_by_category[cat])}")

    # Write staging file
    with open(updates_file, 'w') as f:
        for update in updates:
            f.write(json.dumps(update) + '\n')

    print(f"\n💾 Staged {len(updates)} updates to: {updates_file}")

    # Stats
    high_conf = sum(1 for u in updates if u['match_confidence'] == 'high')
    medium_conf = sum(1 for u in updates if u['match_confidence'] == 'medium')

    print(f"\n📈 Confidence Breakdown:")
    print(f"  High (same day): {high_conf}")
    print(f"  Medium (±1 day): {medium_conf}")

    # Handle flags
    if review:
        print(f"\n{'='*80}")
        print("DETAILED REVIEW")
        print(f"{'='*80}\n")
        for i, update in enumerate(updates[:20], 1):
            print(f"{i}. {update['wallet_current']['merchant']}")
            print(f"   Current: {update['wallet_current']['category_current']}")
            print(f"   → Propose: {update['proposed_update']['category']['name']}")
            print(f"   Confidence: {update['match_confidence'].upper()}")
            print()
        if len(updates) > 20:
            print(f"... and {len(updates) - 20} more")

    if apply_all or apply_high:
        import requests

        # Token is injected by auto framework from config/wallet/config.yaml
        token = os.getenv('WALLET_API_TOKEN')

        if not token:
            print("\n❌ WALLET_API_TOKEN not configured")
            print(f"\n   The auto framework reads from: config/wallet/config.yaml")
            print(f"   Under the 'env:' section:")
            print(f"     WALLET_API_TOKEN: <your-actual-token>")
            print(f"\n   Get your token from: https://app.budgetbakers.com/settings/api")
            print(f"   (Premium plan required)")
            sys.exit(1)

        print(f"\n🚀 APPLYING UPDATES TO WALLET API...")
        applied = 0
        failed = 0

        for update in updates:
            if apply_high and update['match_confidence'] != 'high':
                continue

            record_id = update['id']
            category = update['proposed_update']['category']['name']
            merchant = update['wallet_current']['merchant']

            print(f"  📤 {merchant} → {category}")

            try:
                response = requests.patch(
                    f"https://api.budgetbakers.com/v1/records/{record_id}",
                    headers={
                        "Authorization": f"Bearer {token}",
                        "Content-Type": "application/json",
                    },
                    json={"category": {"name": category}},
                    timeout=10,
                )
                if response.status_code in [200, 204]:
                    print(f"      ✅")
                    applied += 1
                else:
                    print(f"      ❌ HTTP {response.status_code}")
                    failed += 1
            except Exception as e:
                print(f"      ❌ {e}")
                failed += 1

        print(f"\n📊 Results:")
        print(f"  Applied: {applied}")
        print(f"  Failed: {failed}")
    else:
        print(f"\n📝 DRY-RUN MODE - No API calls made")
        print(f"   To apply updates, run:")
        print(f"   ./auto run wallet-sync-categories -- --apply")
        print(f"   Or for high-confidence only:")
        print(f"   ./auto run wallet-sync-categories -- --apply-high")

    print(f"\n✅ Complete")


if __name__ == '__main__':
    main()
