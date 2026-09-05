#!/usr/bin/env python3
import json
from datetime import datetime

# Read the batch file
batch_records = []
with open('_db/extract/batch-2026-08-24-part1.jsonl') as f:
    for line in f:
        batch_records.append(json.loads(line))

# Read labels cache
with open('_db/wallet-sync/labels-cache.json') as f:
    labels_cache = json.load(f)

# Prepare records for wallet_create_records
records_to_create = []

for rec in batch_records:
    gm_id = rec['id']
    date_str = rec['date']
    body = rec['body']

    # Parse date for recordDate
    dt = datetime.fromisoformat(date_str.replace('Z', '+00:00'))
    record_date = dt.strftime('%Y-%m-%d')

    # Extract transaction details
    if 'Krishna Murthy' in body:
        amount = -309
        merchant = 'Krishna Murthy'
        account_id = '6cf80ab9-85bd-420a-aec4-8498005f4ce8'  # HDFC SB x3176
        cat_label = 'sumit'  # Transfer to family member
    elif 'Maresh' in body:
        amount = -251
        merchant = 'Maresh'
        account_id = '41618874-d161-4b2e-86d1-1d077c75cc60'  # Canara CC x6003
        cat_label = 'dining'
    elif 'HungerBox' in body:
        if 'INR 137' in body:
            amount = -137
        elif 'INR 40' in body:
            amount = -40
        elif 'INR 100' in body:
            amount = -100
        else:
            amount = -137
        merchant = 'HungerBox'
        account_id = '41618874-d161-4b2e-86d1-1d077c75cc60'  # Canara CC x6003
        cat_label = 'food-delivery'
    elif 'FirstClub' in body:
        amount = -1832
        merchant = 'FirstClub'
        account_id = 'e6f8c8a4-72d5-44a8-b1e0-5e8f3c4d9a2b'  # Canara CC x6102
        cat_label = 'dining'
    else:
        continue

    # Get label IDs
    label_ids = []
    if cat_label in labels_cache:
        label_ids.append(labels_cache[cat_label])

    # Add payment method label for Canara CC
    if account_id in ['41618874-d161-4b2e-86d1-1d077c75cc60', 'e6f8c8a4-72d5-44a8-b1e0-5e8f3c4d9a2b']:
        if 'canara-cc' in labels_cache:
            label_ids.append(labels_cache['canara-cc'])

    # Use Unknown expense category (standard Wallet API category)
    unknown_expense_cat = '5c5c32c9-0082-8000-8000-000000000000'

    record = {
        'accountId': account_id,
        'amount': amount,
        'currency': 'INR',
        'recordDate': record_date,
        'paymentType': 'card_payment',
        'counterParty': merchant,
        'categoryId': unknown_expense_cat,
        'note': f'{merchant} | via credit card | gmail-sync gm:{gm_id}',
    }

    if label_ids:
        record['labelIds'] = label_ids

    records_to_create.append(record)

# Output for use in wallet_create_records
print(json.dumps(records_to_create))
