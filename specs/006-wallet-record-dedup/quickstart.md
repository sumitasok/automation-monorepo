# Quickstart: Wallet Record Deduplication

**Date**: 2026-08-29 | **Version**: 1.0

This guide demonstrates how to validate the wallet record deduplication feature end-to-end. It covers prerequisites, runnable scenarios, and expected outcomes.

---

## Prerequisites

1. **Working wallet pack**: `packs/wallet/` with Go 1.20+
2. **Valid records.json**: Contains fetched Wallet records (`data/wallet/records.json`)
3. **Dedup subcommand**: Implemented in `main.go` and callable as:
   ```bash
   go run . dedup [operation] [flags]
   # or via auto CLI:
   auto wallet dedup [operation] [flags]
   ```
4. **Test data**: Dataset with intentional duplicates (see "Preparing Test Data" below)

---

## Preparing Test Data

### Option A: Create a synthetic test records.json

For deterministic testing, create a test file with known duplicates:

```bash
cat > /tmp/test-records.json <<'EOF'
{
  "fetchedAt": "2026-08-29T10:00:00Z",
  "count": 5,
  "apiTotal": 5,
  "records": [
    {
      "id": "record-1-original",
      "recordDate": "2026-08-25T12:00:00Z",
      "amount": {"currencyCode": "INR", "value": -1000},
      "counterParty": "Uber",
      "category": {"id": "cat-1", "name": "Transport"},
      "createdAt": "2026-08-25T08:00:00Z"
    },
    {
      "id": "record-1-dup",
      "recordDate": "2026-08-25T12:00:00Z",
      "amount": {"currencyCode": "INR", "value": -1000},
      "counterParty": "Uber",
      "category": {"id": "cat-1", "name": "Transport"},
      "createdAt": "2026-08-25T09:30:00Z"
    },
    {
      "id": "record-2-original",
      "recordDate": "2026-08-26T12:00:00Z",
      "amount": {"currencyCode": "INR", "value": -500},
      "counterParty": "Starbucks",
      "category": {"id": "cat-2", "name": "Food"},
      "createdAt": "2026-08-26T07:00:00Z"
    },
    {
      "id": "record-2-dup",
      "recordDate": "2026-08-26T12:00:00Z",
      "amount": {"currencyCode": "INR", "value": -500},
      "counterParty": "Starbucks",
      "category": {"id": "cat-2", "name": "Food"},
      "createdAt": "2026-08-26T08:00:00Z"
    },
    {
      "id": "record-3-unique",
      "recordDate": "2026-08-27T12:00:00Z",
      "amount": {"currencyCode": "INR", "value": 5000},
      "counterParty": "Employer Inc",
      "category": {"id": "cat-3", "name": "Income"},
      "createdAt": "2026-08-27T06:00:00Z"
    }
  ]
}
EOF
```

### Option B: Use live records.json with intentional duplicates

If working with the actual wallet data:

1. Fetch fresh records:
   ```bash
   auto wallet fetch
   ```
2. Manually duplicate one or two records by copying their JSON objects to test dedup logic.

---

## Scenario 1: Scan for Duplicates (No Modifications)

**Objective**: Verify that the scan operation correctly identifies duplicate groups without modifying records.json.

**Steps**:

```bash
# Setup: Copy test records
cp /tmp/test-records.json data/wallet/records.json

# Run scan
go run . dedup scan --records-file data/wallet/records.json --format text

# Expected output (text):
# === Dedup Scan Results ===
# Scanned: 5 records
# Time: [timestamp]
#
# Duplicate Groups Found: 2
#   Group 1: 2026-08-25 | -1000 INR | Uber
#     Record record-1-original (original, 2026-08-25T08:00:00Z)
#     Record record-1-dup (duplicate, 2026-08-25T09:30:00Z)
#
#   Group 2: 2026-08-26 | -500 INR | Starbucks
#     Record record-2-original (original, 2026-08-26T07:00:00Z)
#     Record record-2-dup (duplicate, 2026-08-26T08:00:00Z)
#
# Total Duplicates to Review: 2
```

**Verification**:
- [ ] Output identifies 2 duplicate groups
- [ ] Each group shows correct record IDs
- [ ] Original records (oldest createdAt) are marked correctly
- [ ] records.json is unchanged (run `wc -l` or hash before/after)

---

## Scenario 2: Scan Output Formats (JSON)

**Objective**: Verify that JSON output format is valid and structured as per contract.

**Steps**:

```bash
# Run scan with JSON output
go run . dedup scan \
  --records-file data/wallet/records.json \
  --format json > scan-results.json

# Validate JSON structure
jq . scan-results.json  # Should pretty-print without errors

# Verify key fields
jq '.duplicateGroupsFound' scan-results.json    # Should output: 2
jq '.groups | length' scan-results.json          # Should output: 2
jq '.groups[0].matchType' scan-results.json      # Should output: "exact"
jq '.groups[0].records | length' scan-results.json # Should output: 2
```

**Verification**:
- [ ] JSON is valid (jq parses without error)
- [ ] `duplicateGroupsFound` matches group count
- [ ] Each group has `matchType` and `confidence` fields
- [ ] Record IDs in JSON match scan output

---

## Scenario 3: Edge Case — Null/Missing Fields

**Objective**: Verify dedup handles records with missing or null optional fields.

**Steps**:

```bash
# Create test file with missing fields
cat > /tmp/test-missing-fields.json <<'EOF'
{
  "fetchedAt": "2026-08-29T10:00:00Z",
  "count": 2,
  "apiTotal": 2,
  "records": [
    {
      "id": "record-no-category",
      "recordDate": "2026-08-25T12:00:00Z",
      "amount": {"currencyCode": "INR", "value": -1000},
      "counterParty": "Uber",
      "createdAt": "2026-08-25T08:00:00Z"
    },
    {
      "id": "record-no-category-dup",
      "recordDate": "2026-08-25T12:00:00Z",
      "amount": {"currencyCode": "INR", "value": -1000},
      "counterParty": "Uber",
      "createdAt": "2026-08-25T09:00:00Z"
    }
  ]
}
EOF

# Scan
go run . dedup scan --records-file /tmp/test-missing-fields.json --format json

# Expected: Should identify as duplicates despite missing category
```

**Verification**:
- [ ] Scan completes without error
- [ ] Two records identified as duplicate (missing category doesn't break matching)
- [ ] Output gracefully handles missing optional fields

---

## Scenario 4: Interactive Review (User Confirms Dedup Decisions)

**Objective**: Verify that review operation collects user decisions and saves them correctly.

**Steps**:

```bash
# Setup: Use test-records.json
cp /tmp/test-records.json data/wallet/records.json

# Run review (simulating user input)
# Note: In actual testing, you would provide interactive input or use --decisions-file
echo "keep-first" | go run . dedup review \
  --records-file data/wallet/records.json \
  --dry-run \
  --decisions-file /tmp/decisions-test.json

# Check decisions file was created
ls -lah /tmp/decisions-test.json

# Verify decisions structure
jq . /tmp/decisions-test.json
# Should output decisions with actions and record IDs
```

**Verification**:
- [ ] Decisions file created at specified path
- [ ] File contains valid JSON
- [ ] Each decision has `duplicateKey`, `action`, `keepRecordIds`, `deleteRecordIds`
- [ ] `deleteRecordIds` contains records to remove
- [ ] Summary section shows total groups reviewed and records to delete

---

## Scenario 5: Execute Dedup (Delete Duplicates with Backup)

**Objective**: Verify that execute operation deletes records, creates backup, and validates output.

**Steps**:

```bash
# Setup: Use test-records.json
cp /tmp/test-records.json data/wallet/records.json
BEFORE_COUNT=$(jq '.records | length' data/wallet/records.json)

# Create minimal decisions file
cat > /tmp/decisions.json <<'EOF'
{
  "timestamp": "2026-08-29T10:00:00Z",
  "decisions": [
    {
      "duplicateKey": "2026-08-25 | -1000 | Uber",
      "action": "keep_first_delete_rest",
      "keepRecordIds": ["record-1-original"],
      "deleteRecordIds": ["record-1-dup"],
      "reason": "Test deletion"
    }
  ],
  "summary": {
    "totalGroupsReviewed": 1,
    "groupsToDelete": 1,
    "recordsToDelete": 1,
    "groupsSkipped": 0
  }
}
EOF

# Execute with --dry-run first
go run . dedup execute \
  --records-file data/wallet/records.json \
  --decisions-file /tmp/decisions.json \
  --dry-run

# Expected: Shows what would be deleted, no actual changes

# Then execute for real
go run . dedup execute \
  --records-file data/wallet/records.json \
  --decisions-file /tmp/decisions.json \
  --force

# Verify results
AFTER_COUNT=$(jq '.records | length' data/wallet/records.json)
echo "Before: $BEFORE_COUNT, After: $AFTER_COUNT"
# Expected: After = Before - 1 (one duplicate removed)

# Check backup was created
ls -lah data/wallet/records.json.backup.*
# Expected: File exists with timestamp

# Verify records.json is valid JSON
jq . data/wallet/records.json > /dev/null && echo "Valid JSON"
```

**Verification**:
- [ ] Dry-run shows correct record to delete
- [ ] After execute, record count decreases by 1
- [ ] Backup file created with timestamp in filename
- [ ] Updated records.json is valid JSON
- [ ] Deleted record ID no longer appears in records.json
- [ ] records.json structure and other records intact

---

## Scenario 6: Large Dataset Performance

**Objective**: Verify dedup performs acceptably on larger datasets.

**Steps**:

```bash
# Fetch actual wallet records
auto wallet fetch

# Time the scan
time go run . dedup scan --records-file data/wallet/records.json --format json > /dev/null

# Example output:
# real    0m0.250s
# user    0m0.180s
# sys     0m0.070s
```

**Verification**:
- [ ] Scan completes in <5 seconds
- [ ] No timeout or out-of-memory errors
- [ ] Output is complete (all records scanned)

---

## Scenario 7: Data Integrity — Backup and Recovery

**Objective**: Verify that backup allows recovery if dedup partially fails.

**Steps**:

```bash
# Setup
cp /tmp/test-records.json data/wallet/records.json
ORIGINAL_HASH=$(sha256sum data/wallet/records.json | cut -d' ' -f1)

# Create decisions and execute
cat > /tmp/decisions.json <<'EOF'
{
  "timestamp": "2026-08-29T10:00:00Z",
  "decisions": [
    {
      "duplicateKey": "2026-08-25 | -1000 | Uber",
      "action": "keep_first_delete_rest",
      "keepRecordIds": ["record-1-original"],
      "deleteRecordIds": ["record-1-dup"],
      "reason": "Test deletion"
    }
  ],
  "summary": {
    "totalGroupsReviewed": 1,
    "groupsToDelete": 1,
    "recordsToDelete": 1,
    "groupsSkipped": 0
  }
}
EOF

go run . dedup execute \
  --records-file data/wallet/records.json \
  --decisions-file /tmp/decisions.json \
  --force

# Find backup
BACKUP_FILE=$(ls -t data/wallet/records.json.backup.* | head -1)

# Verify backup contains original data
BACKUP_HASH=$(sha256sum "$BACKUP_FILE" | cut -d' ' -f1)
echo "Original: $ORIGINAL_HASH"
echo "Backup: $BACKUP_HASH"
# Expected: Hashes match (backup is copy of original)

# Verify updated records.json has fewer records
UPDATED_COUNT=$(jq '.records | length' data/wallet/records.json)
BACKUP_COUNT=$(jq '.records | length' "$BACKUP_FILE")
echo "Backup record count: $BACKUP_COUNT"
echo "Updated record count: $UPDATED_COUNT"
# Expected: Updated < Backup
```

**Verification**:
- [ ] Backup file created and contains pre-dedup data
- [ ] Backup is exact copy of original (hashes match)
- [ ] Updated records.json has fewer records
- [ ] Backup file is accessible for recovery

---

## Scenario 8: Audit Trail Logging

**Objective**: Verify that dedup operations are logged in state.json.

**Steps**:

```bash
# After executing dedup (from Scenario 5):
jq . data/wallet/state.json | tail -50

# Expected: Last entry should show dedup operation:
# {
#   "timestamp": "2026-08-29T10:00:00Z",
#   "operation": "dedup_executed",
#   "deletedRecordIds": ["record-1-dup"],
#   "totalRecordsBefore": 5,
#   "totalRecordsAfter": 4,
#   "backupFile": "records.json.backup.20260829-100000"
# }
```

**Verification**:
- [ ] Audit entry appended to state.json
- [ ] Timestamp recorded
- [ ] Deleted record IDs logged (not full records, to maintain privacy)
- [ ] Before/after counts match reality

---

## Scenario 9: Idempotence — Running Dedup on Deduplicated Data

**Objective**: Verify that running dedup again on a deduplicated dataset produces no groups.

**Steps**:

```bash
# After Scenario 5, run scan again
go run . dedup scan --records-file data/wallet/records.json --format json | jq '.duplicateGroupsFound'

# Expected output: 0
```

**Verification**:
- [ ] Scan finds 0 duplicate groups
- [ ] No changes needed

---

## Scenario 10: Error Handling — Invalid records.json

**Objective**: Verify graceful error handling for malformed input.

**Steps**:

```bash
# Create invalid JSON
echo '{ "records": [invalid json' > /tmp/invalid.json

# Run scan
go run . dedup scan --records-file /tmp/invalid.json --format json

# Expected: Error message with code, no crash
# {
#   "error": "records.json: invalid JSON at line 1",
#   "code": "invalid_json",
#   "timestamp": "..."
# }
```

**Verification**:
- [ ] Tool exits with non-zero code (1 or 2)
- [ ] Error message is clear and actionable
- [ ] No partial output or corrupted state
- [ ] Tool does not crash

---

## Test Suite Structure

After implementation, the test suite should cover:

### Unit Tests (`dedup_test.go`)

- Duplicate detection logic (primary key matching)
- Duplicate grouping (records grouped correctly)
- Confidence calculation (exact vs. uncertain)
- Edge cases (null fields, empty dataset, all unique)

### Integration Tests

- Scan operation on valid records.json
- Review operation with user decisions
- Execute operation with backup creation
- Audit trail appended to state.json
- Error handling for malformed input

### End-to-End Tests

- Full scan → review → execute workflow
- Large dataset performance
- Backup recovery
- Idempotence (dedup on already-deduped data)

---

## Troubleshooting

### "records.json not found"

```bash
# Ensure records.json exists and is readable
ls -la data/wallet/records.json
# If missing, fetch from Wallet API:
auto wallet fetch
```

### "Invalid JSON in records.json"

```bash
# Validate JSON syntax
jq . data/wallet/records.json
# If error, check last few entries for incomplete or malformed records
tail -50 data/wallet/records.json | jq .
```

### "Dedup subcommand not found"

```bash
# Ensure dedup case is added to main.go switch statement
grep -n "dedup" packs/wallet/main.go
# If not present, implement per spec
```

### "Backup creation failed"

```bash
# Check permissions on data/wallet/
ls -ld data/wallet/
# Ensure directory is writable
chmod u+w data/wallet/
```

---

## References

- [Data Model](data-model.md) — Entity definitions and relationships
- [Service Contract](contracts/dedup-record.md) — Operations, schemas, protocols
- [Feature Specification](spec.md) — Full requirements and user stories
