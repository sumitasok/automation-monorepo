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
                            'category': {
                                'name': gmail_category,
                                'id': None  # Will need to be resolved before API call
                            }
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
        logger.info(f"APPLYING UPDATES TO WALLET API (BATCH)")
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

        # Build category name->ID map from existing wallet records
        logger.info(f"Building category name->ID map from existing wallet records...")
        category_name_to_id = {}
        with open(wallet_jsonl, 'r') as f:
            for line in f:
                if line.strip():
                    try:
                        record = json.loads(line)
                        cat = record.get('category', {})
                        cat_name = cat.get('name', '')
                        cat_id = cat.get('id', '')
                        if cat_name and cat_id:
                            category_name_to_id[cat_name] = cat_id
                    except json.JSONDecodeError:
                        pass

        logger.info(f"✓ Indexed {len(category_name_to_id)} unique categories from wallet records")

        # Filter updates by confidence if needed
        updates_to_apply = []
        for update in updates:
            if apply_high and update['match_confidence'] != 'high':
                logger.debug(f"Skipping (medium confidence): {update['wallet_current']['merchant']}")
                continue
            updates_to_apply.append(update)

        if not updates_to_apply:
            logger.warning(f"No updates to apply (all filtered by confidence level)")
            logger.info(f"\n{'='*80}")
            logger.info(f"API RESULTS")
            logger.info(f"{'='*80}")
            logger.info(f"Applied: 0")
            logger.info(f"Failed: 0")
            logger.info(f"Skipped: {len(updates)}")
        else:
            # Build batch payload: array of {id, categoryId}
            batch_payload = []
            unmapped_cats = set()
            for update in updates_to_apply:
                cat_name = update['proposed_update']['category']['name']
                cat_id = category_name_to_id.get(cat_name)
                if cat_id:
                    batch_payload.append({
                        'id': update['id'],
                        'categoryId': cat_id,
                    })
                else:
                    unmapped_cats.add(cat_name)

            if unmapped_cats:
                logger.warning(f"Could not map {len(unmapped_cats)} category names to IDs: {', '.join(sorted(unmapped_cats))}")
                logger.info(f"Only {len(batch_payload)} of {len(updates_to_apply)} updates can be applied")

            if not batch_payload:
                logger.error(f"No mappable updates - cannot proceed")
                sys.exit(1)

            logger.info(f"Sending batch updates: {len(batch_payload)} records (max 10 per request)")
            logger.debug(f"API URL: https://rest.budgetbakers.com/wallet/v1/api/records")

            applied = 0
            failed = 0
            batch_size = 10

            # Send updates in batches of max 10
            for batch_idx in range(0, len(batch_payload), batch_size):
                batch_chunk = batch_payload[batch_idx:batch_idx + batch_size]
                chunk_num = (batch_idx // batch_size) + 1
                total_chunks = (len(batch_payload) + batch_size - 1) // batch_size

                logger.info(f"Batch {chunk_num}/{total_chunks}: Sending {len(batch_chunk)} records")
                logger.debug(f"  Records {batch_idx + 1}-{min(batch_idx + batch_size, len(batch_payload))}")

                try:
                    api_url = "https://rest.budgetbakers.com/wallet/v1/api/records"
                    response = requests.patch(
                        api_url,
                        headers={
                            "Authorization": f"Bearer {token}",
                            "Content-Type": "application/json",
                        },
                        json=batch_chunk,
                        timeout=30,
                    )
                    logger.debug(f"  Response status: {response.status_code}")

                    if response.status_code in [200, 207]:
                        # Parse batch response
                        try:
                            resp_data = response.json()
                            results = resp_data.get('results', [])

                            for result in results:
                                record_id = result.get('id', '?')
                                success = result.get('success', False)
                                error = result.get('error', '')

                                # Find the original update for logging
                                orig_update = next((u for u in updates_to_apply if u['id'] == record_id), None)
                                merchant = orig_update['wallet_current']['merchant'] if orig_update else '?'
                                category = orig_update['proposed_update']['category']['name'] if orig_update else '?'

                                if success:
                                    logger.info(f"    ✅ {merchant} → {category}")
                                    applied += 1
                                else:
                                    logger.error(f"    ❌ {merchant}: {error}")
                                    failed += 1
                        except json.JSONDecodeError as e:
                            logger.error(f"  Failed to parse batch response: {e}")
                            logger.debug(f"  Response body: {response.text[:500]}")
                            failed += len(batch_chunk)
                    else:
                        logger.error(f"  Batch request failed: HTTP {response.status_code}")
                        try:
                            error_data = response.json()
                            logger.debug(f"  Error: {error_data.get('message', error_data)}")
                        except:
                            logger.debug(f"  Response: {response.text[:500]}")
                        failed += len(batch_chunk)

                except requests.exceptions.Timeout:
                    logger.error(f"  Timeout (30s) - batch {chunk_num} did not complete")
                    failed += len(batch_chunk)
                except requests.exceptions.ConnectionError as e:
                    logger.error(f"  Connection error: {e}")
                    failed += len(batch_chunk)
                except Exception as e:
                    logger.error(f"  Exception: {type(e).__name__}: {e}")
                    failed += len(batch_chunk)

            logger.info(f"\n{'='*80}")
            logger.info(f"API RESULTS")
            logger.info(f"{'='*80}")
            logger.info(f"Applied: {applied}")
            logger.info(f"Failed: {failed}")
            if apply_high:
                logger.info(f"Skipped (medium confidence): {len(updates) - len(updates_to_apply)}")
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
