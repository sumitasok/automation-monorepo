#!/usr/bin/env python3
"""
Delete duplicate wallet records from 2026-07-21.
Keeps the newer records from 2026-08-29, removes the old ones.
"""

import json
import os
import subprocess
from collections import defaultdict
from datetime import datetime

# Get API token
API_TOKEN = os.getenv('WALLET_API_TOKEN')
if not API_TOKEN:
    print("❌ WALLET_API_TOKEN not set")
    exit(1)

BASE_URL = "https://rest.budgetbakers.com/wallet"

# Load records and find duplicates
records_by_msg_id = defaultdict(list)

with open(os.path.expanduser('~/data/wallet/records.20260830.jsonl')) as f:
    next(f)  # Skip header
    for line in f:
        if line.strip():
            rec = json.loads(line)
            note = rec.get('note', '')

            if '[gmail-csv ' in note:
                msg_id = note.split('[gmail-csv ')[-1].split(']')[0]
                created_date = rec.get('createdAt', '')[:10]

                records_by_msg_id[msg_id].append({
                    'id': rec.get('id'),
                    'created': rec.get('createdAt', '')[:19],
                    'date': created_date,
                    'amount': rec.get('amount', {}).get('value', 0),
                    'merchant': rec.get('counterParty', '')[:30]
                })

# Find records with duplicates and mark old ones for deletion
to_delete = []
for msg_id, records in records_by_msg_id.items():
    if len(records) > 1:
        # Sort by creation date
        records_sorted = sorted(records, key=lambda x: x['created'])
        # Mark all but the newest for deletion
        for rec in records_sorted[:-1]:
            if rec['date'] == '2026-07-21':
                to_delete.append(rec['id'])

print(f"Found {len(to_delete)} records to delete from 2026-07-21")
print()

if not to_delete:
    print("No records to delete!")
    exit(0)

# Confirm before deleting
print("🔴 WARNING: About to delete the following records:")
print(f"   Total: {len(to_delete)} records")
print(f"   Date: 2026-07-21")
print()
response = input("Type 'DELETE' to confirm: ")
if response != 'DELETE':
    print("❌ Cancelled")
    exit(0)

print()
print(f"🗑️  Deleting {len(to_delete)} records from Wallet API...")
print()

# Delete via API
deleted = 0
failed = 0
failed_ids = []

for i, record_id in enumerate(to_delete, 1):
    if i % 50 == 0:
        print(f"Progress: {i}/{len(to_delete)}...", end='\r')

    url = f"{BASE_URL}/v1/api/records/{record_id}"

    try:
        result = subprocess.run(
            ['curl', '-s', '-X', 'DELETE',
             '-H', f'Authorization: Bearer {API_TOKEN}',
             url],
            capture_output=True, text=True, timeout=10
        )

        if result.returncode == 0 and result.stdout.strip() in ['', '{}', 'null']:
            deleted += 1
        else:
            failed += 1
            failed_ids.append(record_id)
    except Exception as e:
        failed += 1
        failed_ids.append(record_id)

print()
print()
print(f"✅ Deletion Results:")
print(f"   Deleted: {deleted}")
print(f"   Failed:  {failed}")

if failed_ids:
    print(f"\n❌ Failed deletions (first 10):")
    for rid in failed_ids[:10]:
        print(f"   {rid}")

if deleted == len(to_delete):
    print(f"\n✅ SUCCESS: All {deleted} old records deleted!")
    print(f"   Wallet API now has clean data (no duplicates)")
else:
    print(f"\n⚠️  {failed} deletions failed - may need manual cleanup")

# Log results
log_entry = {
    'timestamp': datetime.now().isoformat(),
    'deleted': deleted,
    'failed': failed,
    'total_attempted': len(to_delete)
}

print()
print(json.dumps(log_entry, indent=2))
