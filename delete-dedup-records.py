#!/usr/bin/env python3
"""
Delete duplicate records from Wallet API based on dedup decisions.
Reads record IDs from decisions file and calls Wallet API DELETE for each.
"""

import json
import os
import sys
import requests
from pathlib import Path

WALLET_API_TOKEN = os.getenv("WALLET_API_TOKEN", "").strip()
WALLET_BASE_URL = os.getenv("WALLET_BASE_URL", "https://rest.budgetbakers.com/wallet").rstrip("/")
DECISIONS_FILE = Path.home() / "data" / "wallet" / ".dedup-decisions-20260830-023902.json"

def load_delete_record_ids():
    """Extract all record IDs to delete from decisions file."""
    delete_ids = []
    with open(DECISIONS_FILE) as f:
        for i, line in enumerate(f):
            if line.strip():
                data = json.loads(line)
                if 'deleteRecordIds' in data:
                    delete_ids.extend(data['deleteRecordIds'])
    return delete_ids

def delete_records(record_ids):
    """Delete records from Wallet API."""
    if not WALLET_API_TOKEN:
        print("Error: WALLET_API_TOKEN not set")
        sys.exit(1)

    headers = {
        "Authorization": f"Bearer {WALLET_API_TOKEN}",
        "Content-Type": "application/json"
    }

    success = 0
    failed = 0

    for i, record_id in enumerate(record_ids, 1):
        url = f"{WALLET_BASE_URL}/v1/api/records/{record_id}"
        try:
            resp = requests.delete(url, headers=headers, timeout=10)
            if resp.status_code in [200, 204]:
                print(f"[{i}/{len(record_ids)}] ✅ Deleted {record_id}")
                success += 1
            else:
                print(f"[{i}/{len(record_ids)}] ❌ Failed to delete {record_id}: HTTP {resp.status_code}")
                if resp.text:
                    print(f"      Error: {resp.text[:200]}")
                failed += 1
        except Exception as e:
            print(f"[{i}/{len(record_ids)}] ❌ Error deleting {record_id}: {e}")
            failed += 1

    print(f"\n✓ Deleted: {success} | ✗ Failed: {failed}")
    return failed == 0

if __name__ == '__main__':
    print(f"Loading decisions from: {DECISIONS_FILE}")
    record_ids = load_delete_record_ids()
    print(f"Found {len(record_ids)} records to delete\n")

    if not record_ids:
        print("No records to delete")
        sys.exit(0)

    print(f"Deleting from: {WALLET_BASE_URL}")
    all_success = delete_records(record_ids)

    if all_success:
        print("\n✅ All records deleted successfully!")
        print("Next: ./auto run wallet-dedup finalize")
    else:
        print("\n⚠️ Some deletions failed. Check errors above.")
        sys.exit(1)
