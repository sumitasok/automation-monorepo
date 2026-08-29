#!/usr/bin/env python3
"""
Display and analyze duplicate wallet records found by wallet-dedup.
Shows duplicate groups with merchant, date, amount, and category info.
"""

import json
import sys
from collections import defaultdict
from datetime import datetime
from pathlib import Path

DATA_DIR = Path.home() / "data" / "wallet"
RECORDS_FILE = DATA_DIR / "records.jsonl"

def load_records():
    """Load all wallet records."""
    records = {}
    if not RECORDS_FILE.exists():
        print(f"Error: {RECORDS_FILE} not found")
        sys.exit(1)

    with open(RECORDS_FILE) as f:
        for line_num, line in enumerate(f, 1):
            if not line.strip():
                continue
            record = json.loads(line)
            # Skip metadata line (first line has apiTotal, not id)
            if 'id' in record:
                records[record['id']] = record

    return records

def find_duplicates(records):
    """Find duplicate records by MessageID."""
    by_msg_id = defaultdict(list)

    for record_id, record in records.items():
        # Extract MessageID from note field
        note = record.get('note', '')
        if 'gmail-csv' in note:
            # Extract the MessageID from "[gmail-csv xxxx]"
            try:
                msg_id = note.split('[gmail-csv ')[-1].split(']')[0]
                by_msg_id[msg_id].append(record_id)
            except:
                pass

    # Find duplicates (MessageID appears multiple times)
    duplicates = {msg_id: ids for msg_id, ids in by_msg_id.items() if len(ids) > 1}
    return duplicates

def format_record(record, record_id):
    """Format a record for display."""
    date = record.get('recordDate', '')[:10]
    merchant = record.get('counterParty', '')[:40]
    amount = record.get('amount', {}).get('value', 0)
    category = record.get('category', {}).get('name', 'Unknown')

    return {
        'id': record_id,
        'date': date,
        'merchant': merchant,
        'amount': f"{amount:.2f}",
        'category': category,
    }

def main():
    print("Loading wallet records...")
    records = load_records()
    print(f"✓ Loaded {len(records)} records")

    print("\nFinding duplicates by MessageID...")
    duplicates = find_duplicates(records)
    print(f"✓ Found {len(duplicates)} duplicate groups")

    total_records = sum(len(ids) for ids in duplicates.values())
    total_dupes = sum(len(ids) - 1 for ids in duplicates.values())

    print(f"  Total duplicate records: {total_records}")
    print(f"  Records to delete: {total_dupes}")
    print(f"  Records to keep: {len(duplicates)}")

    # Display duplicates
    print(f"\n{'='*120}")
    print(f"DUPLICATE GROUPS")
    print(f"{'='*120}\n")

    for group_idx, (msg_id, record_ids) in enumerate(sorted(duplicates.items()), 1):
        print(f"[Group {group_idx}/{len(duplicates)}] {len(record_ids)} records with same MessageID")
        print(f"  MessageID: {msg_id}")
        print(f"  {'─'*100}")

        formatted = []
        for rid in record_ids:
            record = records[rid]
            formatted.append({
                **format_record(record, rid),
                'created': record.get('createdAt', '')[:19]
            })

        # Sort by created date to show which is oldest (keep) vs newer (delete)
        formatted_sorted = sorted(formatted, key=lambda x: x['created'])

        for idx, rec in enumerate(formatted_sorted, 1):
            action = "KEEP (oldest)" if idx == 1 else "DELETE"
            print(f"  [{action:15}] {rec['date']} | {rec['merchant']:40} | {rec['amount']:>10} | {rec['category']}")
            print(f"                   ID: {rec['id']} | Created: {rec['created']}")

        print()

    print(f"{'='*120}")
    print(f"Summary: {len(duplicates)} groups | {total_records} duplicate records | {total_dupes} to delete | {len(duplicates)} to keep")
    print(f"\nTo execute dedup: ./auto run wallet-dedup execute")
    print(f"To finalize: ./auto run wallet-dedup finalize")

if __name__ == '__main__':
    main()
