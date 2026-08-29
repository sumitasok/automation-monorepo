# Dedup Record Service Contract

**Date**: 2026-08-29 | **Version**: 1.0

## Overview

This contract specifies the input/output schemas and operation protocols for the wallet record deduplication service. The service is a CLI tool invoked as `auto wallet dedup` or `go run . dedup`.

---

## Operations

### Operation 1: Scan

**Purpose**: Identify duplicate records in records.json without modifying anything.

#### Input

```bash
auto wallet dedup scan [flags]
```

**Flags**:
- `--records-file` (string, optional): Path to records.json. Default: `$AUTO_DATA_DIR/wallet/records.json`
- `--dedup-config` (string, optional): Path to dedup config file. Default: `config/wallet/config.yaml` (extracts `dedup` section)
- `--format` (string, optional): Output format. Choices: `text`, `json`. Default: `text`
- `--min-confidence` (float, optional): Include only duplicates with confidence >= this value (0.0–1.0). Default: 0.5 (show uncertain duplicates with low confidence as advisory)

#### Output (text format)

```
=== Dedup Scan Results ===
Scanned: 6329 records
Time: 2026-08-29T10:45:33Z

Duplicate Groups Found: 3
  Group 1: 2026-08-29 | -3054.5 INR | IRCTC Ticketing
    Record 65436aec (original, 2026-08-29T09:38:41Z)
    Record f7e8c3d2 (duplicate, 2026-08-29T10:15:22Z)
  
  Group 2: 2026-08-28 | -500.0 INR | Uber
    Record a1b2c3d4 (original, 2026-08-28T14:20:10Z)
    Record e5f6g7h8 (duplicate, 2026-08-28T15:45:33Z)

Total Duplicates to Review: 2
Ready for review? Run: auto wallet dedup review
```

#### Output (JSON format)

```json
{
  "timestamp": "2026-08-29T10:45:33Z",
  "recordsScanned": 6329,
  "duplicateGroupsFound": 3,
  "groups": [
    {
      "duplicateKey": "2026-08-29 | -3054.5 | IRCTC Ticketing",
      "matchType": "exact",
      "confidence": 1.0,
      "records": [
        {
          "id": "65436aec-bbd0-45a3-8687-e9501dc22c06",
          "createdAt": "2026-08-29T09:38:41.844Z",
          "isOriginal": true,
          "counterParty": "IRCTC Ticketing",
          "amount": -3054.5,
          "category": "Unknown expense"
        },
        {
          "id": "f7e8c3d2-a1b9-48f5-9e7c-3d6b9e8c2a1f",
          "createdAt": "2026-08-29T10:15:22.100Z",
          "isOriginal": false,
          "counterParty": "IRCTC Ticketing",
          "amount": -3054.5,
          "category": "Unknown expense"
        }
      ]
    }
  ],
  "totalDuplicateRecords": 2
}
```

#### Exit Codes

- `0`: Success (duplicates found or not)
- `1`: Error reading records.json or dedup config
- `2`: Invalid flag values

---

### Operation 2: Review

**Purpose**: Present duplicate groups interactively and collect user decisions on how to handle each group.

#### Input

```bash
auto wallet dedup review [flags]
```

**Flags**:
- `--records-file` (string, optional): Path to records.json. Default: `$AUTO_DATA_DIR/wallet/records.json`
- `--dedup-config` (string, optional): Path to dedup config. Default: `config/wallet/config.yaml`
- `--decisions-file` (string, optional): Path to save decisions as JSON for later review/audit. Default: `.dedup-decisions-{timestamp}.json` in current directory.
- `--dry-run` (bool, optional): Show decisions but don't save them. Default: false

#### Interactive Flow

```
=== Dedup Review ===
Duplicate groups to review: 3

Group 1 of 3: 2026-08-29 | -3054.5 INR | IRCTC Ticketing
  Confidence: exact match
  
  Record 1 (original) [2026-08-29T09:38:41Z]
    ID: 65436aec
    Amount: -3054.5 INR
    Counterparty: IRCTC Ticketing
    Category: Unknown expense
    Note: IRCTC Ticketing | via HDFC SB UPI | gmail-sync...
  
  Record 2 (duplicate) [2026-08-29T10:15:22Z]
    ID: f7e8c3d2
    Amount: -3054.5 INR
    Counterparty: IRCTC Ticketing
    Category: Unknown expense
    Note: IRCTC Ticketing | via HDFC SB UPI | gmail-sync...
  
  Action? (keep-first/custom/skip) [keep-first]:
```

**Input options**:
- `keep-first`: Keep the original (oldest), delete all others
- `custom`: Specify which to keep (interactive prompt: "Keep record IDs (comma-separated):")
- `skip`: Don't process this group

#### Decisions File (JSON)

```json
{
  "timestamp": "2026-08-29T10:45:33Z",
  "decisions": [
    {
      "duplicateKey": "2026-08-29 | -3054.5 | IRCTC Ticketing",
      "action": "keep_first_delete_rest",
      "keepRecordIds": ["65436aec-bbd0-45a3-8687-e9501dc22c06"],
      "deleteRecordIds": ["f7e8c3d2-a1b9-48f5-9e7c-3d6b9e8c2a1f"],
      "reason": "User selected keep-first"
    },
    {
      "duplicateKey": "2026-08-28 | -500.0 | Uber",
      "action": "skip",
      "reason": "User skipped this group"
    }
  ],
  "summary": {
    "totalGroupsReviewed": 2,
    "groupsToDelete": 1,
    "recordsToDelete": 1,
    "groupsSkipped": 1
  }
}
```

#### Exit Codes

- `0`: Success (decisions collected and saved)
- `1`: Error reading records.json or dedup config
- `2`: User cancelled before completing review (decisions not saved)
- `3`: Invalid user input in interactive prompt

---

### Operation 3: Execute

**Purpose**: Apply dedup decisions, delete records, create backup, and update records.json.

#### Input

```bash
auto wallet dedup execute [flags]
```

**Flags**:
- `--records-file` (string, optional): Path to records.json. Default: `$AUTO_DATA_DIR/wallet/records.json`
- `--decisions-file` (string, required if not piped): Path to decisions JSON from review step.
- `--dry-run` (bool, optional): Show what would be deleted without modifying records.json. Default: false
- `--force` (bool, optional): Skip final confirmation prompt. Default: false

#### Output (execution success)

```
=== Dedup Execute ===
Loading decisions from .dedup-decisions-20260829T104533Z.json...
  Decisions: 2 groups, 1 to delete

Creating backup: records.json.backup.20260829-104533...
  Backup size: 234KB

Deleting records: [f7e8c3d2, a1b2c3d4]...
  Deleted: 2 records

Updating records.json (6329 → 6327 records)...
  Validation: OK (valid JSON)

Recording audit trail in state.json...
  Done

=== Dedup Complete ===
Records deleted: 2
Remaining records: 6327
Backup: records.json.backup.20260829-104533
Audit trail updated: state.json
```

#### Output (execution with --dry-run)

```
=== Dedup Execute (DRY RUN) ===
[Same as above, but appends]

DRY RUN: No changes made. Run without --dry-run to apply.
```

#### Exit Codes

- `0`: Success (records deleted, backup created, audit trail updated)
- `1`: Error reading decisions or records.json
- `2`: Backup creation failed
- `3`: Validation of updated records.json failed; reverted to original
- `4`: User cancelled at final confirmation prompt

---

### Operation 4: Undo (future extension)

**Purpose**: Restore records from a backup created by a previous dedup operation.

#### Input

```bash
auto wallet dedup undo --backup <backup-file>
```

(Not implemented in Phase 1; included here for completeness.)

---

## Data Schemas

### DedupConfig (from config.yaml)

```yaml
dedup:
  enabled: true
  primaryKeys:
    - recordDate
    - amount.value
    - counterParty
  optionalKeys: []  # Can extend to [category.id, labels[].id, note]
  minConfidence: 0.5  # Flag uncertain duplicates
  backupRetention: 30  # Days to keep backups (future)
```

### Input Record (from records.json)

See [data-model.md](../data-model.md#record).

### Output: DuplicateGroup

See [data-model.md](../data-model.md#duplicate-group).

### Error Response

All operations return errors as:

```json
{
  "error": "Error message",
  "code": "error_code",
  "timestamp": "2026-08-29T10:45:33Z"
}
```

Example:

```json
{
  "error": "records.json: invalid JSON at line 42",
  "code": "invalid_json",
  "timestamp": "2026-08-29T10:45:33Z"
}
```

**Common error codes**:
- `invalid_json`: Malformed JSON in records.json or config
- `file_not_found`: records.json or decisions file does not exist
- `invalid_config`: dedup config section invalid or missing
- `backup_failed`: Backup creation failed (permissions, disk space)
- `write_failed`: Update to records.json failed (permissions, disk space)
- `user_cancelled`: User exited before completing operation

---

## Protocols

### Scan-Review-Execute Workflow

**Typical user flow**:

```bash
# Step 1: Scan (no side effects)
auto wallet dedup scan --format json | tee scan-results.json

# Step 2: Review (user decides how to handle each group)
auto wallet dedup review

# Step 3: Execute (user confirms final action)
auto wallet dedup execute --decisions-file .dedup-decisions-*.json

# Result: records.json updated, backup created, audit trail recorded
```

### Non-Interactive Workflow

**For automation** (manifest-driven):

```bash
# Create decisions file programmatically or via review
cat decisions.json | auto wallet dedup execute --decisions-file - --force
```

---

## Guarantees

### Atomicity

- **Scan operation**: Read-only; no side effects. Safe to run repeatedly.
- **Review operation**: Saves decisions to a file; original records.json unmodified. Safe to review multiple times or discard decisions.
- **Execute operation**: All-or-nothing deletion. On failure, both records.json and backup exist; recovery is possible.

### Data Integrity

- Backup is created *before* records.json is modified.
- Backup filename includes timestamp for uniqueness.
- Audit trail is appended to state.json after successful execution.
- Deleted record IDs (not full records) are logged for least-exposure.

### Idempotence

- Scanning a deduplicated dataset produces no duplicate groups.
- Running execute twice with different decisions files produces cumulative deletions (not idempotent, but expected).

---

## Configuration

Dedup behavior is configured in `config.sample.yaml` under the `dedup:` key:

```yaml
dedup:
  # Which fields define a duplicate? Primary keys always used.
  primaryKeys:
    - recordDate
    - amount.value
    - counterParty
  
  # Extended matching (optional, not used by default)
  optionalKeys: []
  
  # Confidence threshold for display (0.0 to 1.0)
  # Duplicates below this threshold are marked as "uncertain"
  minConfidence: 0.5
  
  # Future: backup retention policy
  backupRetention: 30  # days
```

Users can override via environment:

```bash
export AUTO_WALLET_DEDUP_MIN_CONFIDENCE=0.8
auto wallet dedup scan
```

Or via CLI flag:

```bash
auto wallet dedup scan --min-confidence 0.8
```
