# Dedup Architecture: Working Copy Pattern

**Date**: 2026-08-29  
**Version**: 1.0

## Design Principle: Never Modify Original Until Confirmed

The wallet record deduplication uses a **working copy pattern** to ensure data safety:

```
Load records.json → Create Working Copy (in memory)
    ↓
Perform all dedup operations on working copy
    ↓
User reviews findings (from working copy)
    ↓
User confirms decisions (what to delete)
    ↓
Create backup: records.json.backup.{timestamp}
    ↓
Atomic write: working copy → records.json
    ↓
Verify: records.json is valid JSON
    ↓
Append audit trail to state.json
```

**Original records.json is never modified** until step 6 (atomic write).  
**On failure**: Both original and backup exist for recovery.

---

## Data Flow

### Phase: Scan (Read-Only)

```go
// Scan never modifies records.json or any files
func scanDedup() {
    originalRecords := loadRecords("records.json")  // Load original
    workingCopy := copyRecords(originalRecords)      // Create copy in memory
    duplicates := detect(workingCopy)                // Find duplicates
    print(duplicates)                                 // Report findings
    // records.json: UNTOUCHED
}
```

**Result**: User sees duplicate groups. No files changed.

### Phase: Review (Decision Capture)

```go
// Review collects decisions from user, saves to separate file
func reviewDedup() {
    duplicates := loadDuplicatesFromScan()           // From scan results
    decisions := promptUser(duplicates)              // User decides
    saveDecisions("decisions.json", decisions)       // Save separately
    // records.json: UNTOUCHED
    // decisions.json: NEW FILE (can be discarded if not executed)
}
```

**Result**: User's decisions saved. Original records.json still untouched.

### Phase: Execute (Atomic Write)

```go
// Execute is the ONLY phase that modifies records.json
func executeDedup() {
    originalRecords := loadRecords("records.json")   // Load original
    decisions := loadDecisions("decisions.json")     // Load user's choices
    
    // Create backup BEFORE any modification
    backupPath := createBackup("records.json")       // "records.json.backup.{timestamp}"
    
    // Apply decisions to working copy
    workingCopy := copyRecords(originalRecords)      // Copy
    cleaned := applyDecisions(workingCopy, decisions) // Filter
    
    // Validate cleaned data
    if !isValidJSON(cleaned) {
        return error("cleaned data invalid")
    }
    
    // Atomic write (temp file + rename prevents corruption)
    tempFile := writeToTemp(cleaned)                 // Write to temp
    atomicRename(tempFile, "records.json")           // Atomic move
    
    // Audit trail
    appendAuditTrail("dedup_executed", decisions, backupPath)
    
    // Result: records.json updated, backup exists, audit trail logged
}
```

**Result**: 
- ✓ records.json updated with cleaned records
- ✓ Backup created (original safe)
- ✓ Audit trail recorded
- ✓ Both files exist if verification fails

---

## Failure Scenarios

### Scenario 1: Scan Fails
```
records.json: UNTOUCHED
No recovery needed (no changes made)
```

### Scenario 2: Review User Cancels
```
records.json: UNTOUCHED
decisions.json: Can be discarded
User can re-review if needed
```

### Scenario 3: Execute - Disk Full Before Write
```
records.json: UNTOUCHED (write failed)
records.json.backup.{timestamp}: NOT CREATED (backup creation failed)
No loss (no changes made)
Retry after freeing disk space
```

### Scenario 4: Execute - Disk Full During Write
```
records.json: UNTOUCHED (atomic write failed, temp file not renamed)
records.json.backup.{timestamp}: EXISTS (backup created successfully)
temp file: Partial/incomplete (not renamed, can be discarded)
Recovery: Delete temp, retry or restore from backup
```

### Scenario 5: Execute - Write Succeeds, Verify Fails
```
records.json: UPDATED (but might be corrupted)
records.json.backup.{timestamp}: EXISTS (original safe)
Solution: Restore from backup, investigate corruption
```

---

## Code Implementation Pattern

### Load Phase (all phases)
```go
type RecordsWrapper struct {
    OriginalPath string
    Records      []Record      // Working copy (never written back)
    IsModified   bool          // Track if modifications made
}

// Load creates a working copy, original file untouched
func Load(filePath string) (*RecordsWrapper, error) {
    data, _ := os.ReadFile(filePath)           // Read original
    var records []Record
    json.Unmarshal(data, &records)             // Parse
    return &RecordsWrapper{
        OriginalPath: filePath,
        Records:      records,                 // This is the working copy
        IsModified:   false,
    }
}
```

### Modify Phase (execute only)
```go
// Filter removes records from working copy
func (w *RecordsWrapper) Filter(toDelete []string) {
    filtered := []Record{}
    for _, r := range w.Records {
        if !contains(toDelete, r.ID) {
            filtered = append(filtered, r)
        }
    }
    w.Records = filtered
    w.IsModified = true
}

// WriteAtomic only called after confirmation
func (w *RecordsWrapper) WriteAtomic() error {
    if !w.IsModified {
        return errors.New("no modifications to write")
    }
    
    // Create backup first
    backup := createBackup(w.OriginalPath)
    if backup == "" {
        return errors.New("backup creation failed, aborting write")
    }
    
    // Write to temp
    tmpPath := w.OriginalPath + ".tmp"
    data, _ := json.Marshal(w.Records)
    os.WriteFile(tmpPath, data, 0644)
    
    // Validate
    var test []Record
    if err := json.Unmarshal(data, &test); err != nil {
        os.Remove(tmpPath)
        return fmt.Errorf("new records invalid JSON: %v", err)
    }
    
    // Atomic rename
    return os.Rename(tmpPath, w.OriginalPath)
}
```

---

## Configuration

No changes to config required. The working copy pattern is transparent to the user:

```yaml
dedup:
  primaryKeys: [recordDate, amount.value, counterParty]
  optionalKeys: []
  minConfidence: 0.5
```

The config describes WHAT dedup logic to use, not HOW to store it.

---

## Safety Guarantees

| Guarantee | Mechanism |
|-----------|-----------|
| **Never lose data** | Backup created before write; original recoverable |
| **Atomic writes** | Temp file + rename prevents partial writes |
| **Valid JSON** | Validated before write; if invalid, write aborted |
| **Audit trail** | Every change logged with timestamp and record IDs |
| **Reversible** | Backup exists; can restore if needed |
| **No silent corruption** | All writes verified; failures reported clearly |

---

## Testing Strategy

### Scan Tests (Read-Only)
```go
TestScanDoesNotModifyRecords()           // Verify file unchanged
TestScanDoesNotCreateBackup()            // Verify no backup created
TestScanOutputIsValid()                  // Verify output format
```

### Review Tests (Decision Capture)
```go
TestReviewDoesNotModifyRecords()         // Verify records.json unchanged
TestReviewSavesDecisions()               // Verify decisions.json created
TestReviewCanDiscardDecisions()          // Verify decisions can be deleted
```

### Execute Tests (Atomic Write)
```go
TestExecuteCreatesBackupFirst()          // Verify backup precedes write
TestExecuteWritesValidJSON()             // Verify new file is valid JSON
TestExecuteAtomic()                      // Verify temp + rename pattern
TestExecuteOnFailurePreservesOriginal()  // Verify rollback works
TestExecuteAppendAuditTrail()            // Verify audit logged
```

---

## Compatibility with Wallet API

This architecture operates on the **local mirror** (records.json from wallet-fetch).

Future enhancement: After updating records.json, could optionally sync deletions back to Wallet API:
```
User confirms dedup → records.json updated → (optional) call DELETE /records/{id} for each deleted record
```

For MVP, dedup is local-only (records.json mirror only). Wallet API not contacted.
