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
import logging
from datetime import datetime
from collections import defaultdict
from pathlib import Path

try:
    import requests
except ImportError:
    requests = None

# Set up detailed logging
log_format = '%(asctime)s [%(levelname)8s] %(message)s'
logging.basicConfig(
    level=logging.DEBUG,
    format=log_format,
    datefmt='%Y-%m-%d %H:%M:%S'
)
logger = logging.getLogger(__name__)


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
    logger.info("=" * 80)
    logger.info("WALLET SYNC CATEGORIES - Starting")
    logger.info("=" * 80)

    # Parse arguments
    apply_all = '--apply' in sys.argv
    apply_high = '--apply-high' in sys.argv
    review = '--review' in sys.argv
    dry_run = '--dry-run' in sys.argv or (not apply_all and not apply_high)

    logger.debug(f"Arguments: apply_all={apply_all}, apply_high={apply_high}, review={review}, dry_run={dry_run}")

    # Get data directory from auto framework
    try:
        data_dir = get_data_dir()
        logger.info(f"Data directory: {data_dir}")
    except RuntimeError as e:
        logger.error(f"{e}")
        sys.exit(1)

    gmail_csv = data_dir / 'gmail' / 'transactions.csv'
    wallet_jsonl = data_dir / 'wallet' / 'records.jsonl'
    updates_file = data_dir / 'wallet' / 'updates.jsonl'

    logger.info(f"Gmail CSV: {gmail_csv}")
    logger.info(f"Wallet JSONL: {wallet_jsonl}")
    logger.info(f"Updates file: {updates_file}")

    # Verify files exist
    if not gmail_csv.exists():
        logger.error(f"Gmail CSV not found: {gmail_csv}")
        sys.exit(1)
    logger.debug(f"✓ Gmail CSV exists")

    if not wallet_jsonl.exists():
        logger.error(f"Wallet records not found: {wallet_jsonl}")
        sys.exit(1)
    logger.debug(f"✓ Wallet records exist")

    logger.info(f"Mode: {'DRY-RUN' if dry_run else 'APPLY'}")
    logger.info(f"Flags: review={review}")

    # Read Gmail transactions
    logger.info("Reading Gmail transactions...")
    gmail_data = []
    gmail_by_merchant = defaultdict(list)

    with open(gmail_csv, 'r') as f:
        reader = csv.DictReader(f)
        row_count = 0
        for row in reader:
            gmail_data.append(row)
            merchant = (row.get('Merchant', '') or row.get('Info', '')).lower().strip()
            if merchant:
                gmail_by_merchant[merchant].append(row)
            row_count += 1
            if row_count % 100 == 0:
                logger.debug(f"  Loaded {row_count} Gmail rows...")

    logger.info(f"✓ Gmail transactions: {len(gmail_data)} total")
    logger.debug(f"  Unique merchants indexed: {len(gmail_by_merchant)}")

    # Read Wallet records with Unknown category
    logger.info("Reading Wallet records...")
    wallet_unknown = []
    wallet_all_count = 0
    wallet_unknown_by_cat = defaultdict(int)

    with open(wallet_jsonl, 'r') as f:
        for line_num, line in enumerate(f, 1):
            if line.strip():
                try:
                    record = json.loads(line)
                    wallet_all_count += 1
                    cat = record.get('category', {}).get('name', '')
                    if cat in ['Unknown expense', 'Unknown income', 'Uncategorized']:
                        wallet_unknown.append(record)
                        wallet_unknown_by_cat[cat] += 1
                except json.JSONDecodeError as e:
                    logger.warning(f"  Line {line_num}: JSON parse error: {e}")

    logger.info(f"✓ Wallet records: {wallet_all_count} total")
    logger.info(f"✓ Wallet records with Unknown category: {len(wallet_unknown)}")
    for cat, count in sorted(wallet_unknown_by_cat.items()):
        logger.debug(f"    - {cat}: {count}")

    # Find matches and create updates
    logger.info(f"\nMatching Gmail categories with Wallet Unknown records...")
    logger.debug(f"Processing {len(wallet_unknown)} wallet records...")

    updates = []
    seen_wallet_ids = set()
    matched_count = 0
    unmatched_count = 0
    skipped_count = 0

    for wallet_idx, wallet in enumerate(wallet_unknown, 1):
        if wallet['id'] in seen_wallet_ids:
            logger.debug(f"[{wallet_idx}/{len(wallet_unknown)}] Duplicate wallet ID, skipping")
            skipped_count += 1
            continue
        seen_wallet_ids.add(wallet['id'])

        wallet_merchant = wallet.get('counterParty', '').lower().strip()
        wallet_date_str = wallet.get('recordDate', '')
        wallet_amount = wallet.get('amount', {}).get('value', 0)
        wallet_cat_current = wallet.get('category', {}).get('name', '')

        if not wallet_merchant:
            logger.debug(f"[{wallet_idx}/{len(wallet_unknown)}] No merchant, skipping")
            skipped_count += 1
            continue

        logger.debug(f"\n[{wallet_idx}/{len(wallet_unknown)}] Processing wallet record:")
        logger.debug(f"  Merchant: {wallet.get('counterParty')}")
        logger.debug(f"  Date: {wallet_date_str[:10]}")
        logger.debug(f"  Amount: {wallet_amount}")
        logger.debug(f"  Current Category: {wallet_cat_current}")

        if wallet_merchant in gmail_by_merchant:
            logger.debug(f"  ✓ Found {len(gmail_by_merchant[wallet_merchant])} matching Gmail transaction(s)")
            gmail_matches = gmail_by_merchant[wallet_merchant]

            try:
                wallet_dt = datetime.fromisoformat(wallet_date_str.replace('Z', '+00:00')).date()
                logger.debug(f"  Wallet date parsed: {wallet_dt}")
            except (ValueError, TypeError) as e:
                logger.debug(f"  ✗ Date parse error: {e}")
                unmatched_count += 1
                continue

            best_match = None
            best_day_diff = float('inf')

            for match_idx, gmail in enumerate(gmail_matches, 1):
                gmail_dt_str = gmail.get('TxnDate', '')
                if gmail_dt_str:
                    try:
                        gmail_dt = datetime.strptime(gmail_dt_str.split()[0], '%Y-%m-%d').date()
                        day_diff = abs((wallet_dt - gmail_dt).days)
                        logger.debug(f"    Gmail match {match_idx}: {gmail_dt} (diff: {day_diff} days)")

                        if day_diff <= 1 and day_diff < best_day_diff:
                            best_match = gmail
                            best_day_diff = day_diff
                            logger.debug(f"      → New best match (diff: {day_diff})")
                    except ValueError as e:
                        logger.debug(f"    Gmail match {match_idx}: Date parse error: {e}")

            if best_match:
                logger.debug(f"  ✓ Best match found (day diff: {best_day_diff})")
                gmail_category = best_match.get('Category', '').strip()
                gmail_subcategory = best_match.get('SubCategory', '').strip()
                gmail_labels = best_match.get('Labels', '').strip()
                gmail_source = best_match.get('Source', '').strip()

                logger.debug(f"    Gmail Category: {gmail_category}")
                logger.debug(f"    Gmail SubCategory: {gmail_subcategory}")
                logger.debug(f"    Gmail Labels: {gmail_labels}")
                logger.debug(f"    Gmail Source: {gmail_source}")

                if gmail_category and gmail_category not in ['', 'Unknown', 'Unknown income', 'Unknown expense']:
                    logger.info(f"  ✓ MATCH: {wallet_merchant} | {wallet_cat_current} → {gmail_category}")
                    matched_count += 1

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
                else:
                    logger.debug(f"  ✗ Gmail category not valid: '{gmail_category}'")
                    unmatched_count += 1
            else:
                logger.debug(f"  ✗ No date match found (all outside ±1 day range)")
                unmatched_count += 1
        else:
            logger.debug(f"  ✗ No Gmail merchant match: '{wallet_merchant}'")
            unmatched_count += 1

    logger.info(f"\n{'='*80}")
    logger.info(f"MATCHING RESULTS")
    logger.info(f"{'='*80}")
    logger.info(f"Matched: {matched_count}")
    logger.info(f"Unmatched: {unmatched_count}")
    logger.info(f"Skipped: {skipped_count}")
    logger.info(f"Total updates found: {len(updates)}")

    # Group by category
    logger.info(f"\nGrouping updates by category...")
    updates_by_category = defaultdict(list)
    for update in updates:
        cat = update['proposed_update']['category']['name']
        updates_by_category[cat].append(update)

    logger.info(f"Updates by Category:")
    for cat in sorted(updates_by_category.keys(), key=lambda x: len(updates_by_category[x]), reverse=True):
        logger.info(f"  {cat}: {len(updates_by_category[cat])}")

    # Write staging file
    logger.info(f"\nWriting staging file: {updates_file}")
    with open(updates_file, 'w') as f:
        for update_idx, update in enumerate(updates, 1):
            f.write(json.dumps(update) + '\n')
            if update_idx % 5 == 0:
                logger.debug(f"  Written {update_idx}/{len(updates)}...")

    logger.info(f"✓ Staged {len(updates)} updates to: {updates_file}")

    # Stats
    high_conf = sum(1 for u in updates if u['match_confidence'] == 'high')
    medium_conf = sum(1 for u in updates if u['match_confidence'] == 'medium')

    logger.info(f"\nConfidence Breakdown:")
    logger.info(f"  High (same day): {high_conf}")
    logger.info(f"  Medium (±1 day): {medium_conf}")

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
        if not requests:
            logger.error(f"requests module not installed. Install with: pip install requests")
            sys.exit(1)

        logger.info(f"\n{'='*80}")
        logger.info(f"APPLYING UPDATES TO WALLET API")
        logger.info(f"{'='*80}")

        # Token is injected by auto framework from config/wallet/config.yaml
        token = os.getenv('WALLET_API_TOKEN')

        if not token:
            logger.error(f"WALLET_API_TOKEN not configured")
            logger.error(f"The auto framework reads from: config/wallet/config.yaml")
            logger.error(f"Under the 'env:' section: WALLET_API_TOKEN: <your-actual-token>")
            logger.error(f"Get your token from: https://app.budgetbakers.com/settings/api (Premium plan required)")
            sys.exit(1)

        logger.info(f"Token loaded from config")
        logger.info(f"Mode: {'Apply HIGH-CONFIDENCE only' if apply_high else 'Apply ALL'}")

        applied = 0
        failed = 0
        skipped_confidence = 0

        for update_idx, update in enumerate(updates, 1):
            record_id = update['id']
            category = update['proposed_update']['category']['name']
            merchant = update['wallet_current']['merchant']
            confidence = update['match_confidence']

            if apply_high and confidence != 'high':
                logger.debug(f"[{update_idx}/{len(updates)}] Skipping (medium confidence): {merchant}")
                skipped_confidence += 1
                continue

            logger.info(f"[{update_idx}/{len(updates)}] Applying: {merchant} ({confidence}) → {category}")
            logger.debug(f"  Record ID: {record_id}")
            logger.debug(f"  API URL: https://api.budgetbakers.com/v1/records/{record_id}")

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
                logger.debug(f"  Response status: {response.status_code}")

                if response.status_code in [200, 204]:
                    logger.info(f"  ✅ Success")
                    applied += 1
                else:
                    logger.error(f"  ❌ HTTP {response.status_code}")
                    logger.debug(f"  Response: {response.text[:200]}")
                    failed += 1
            except requests.exceptions.Timeout:
                logger.error(f"  ❌ Timeout (10s)")
                failed += 1
            except requests.exceptions.ConnectionError as e:
                logger.error(f"  ❌ Connection error: {e}")
                failed += 1
            except Exception as e:
                logger.error(f"  ❌ Exception: {type(e).__name__}: {e}")
                failed += 1

        logger.info(f"\n{'='*80}")
        logger.info(f"API RESULTS")
        logger.info(f"{'='*80}")
        logger.info(f"Applied: {applied}")
        logger.info(f"Failed: {failed}")
        if apply_high:
            logger.info(f"Skipped (medium confidence): {skipped_confidence}")
    else:
        logger.info(f"\n{'='*80}")
        logger.info(f"DRY-RUN MODE - No API calls made")
        logger.info(f"{'='*80}")
        logger.info(f"To apply updates, run:")
        logger.info(f"  ./auto run wallet-sync-categories -- --apply")
        logger.info(f"Or for high-confidence only:")
        logger.info(f"  ./auto run wallet-sync-categories -- --apply-high")

    logger.info(f"\n{'='*80}")
    logger.info(f"Complete")
    logger.info(f"{'='*80}")


if __name__ == '__main__':
    main()
