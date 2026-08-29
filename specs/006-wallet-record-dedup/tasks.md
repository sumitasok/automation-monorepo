# Tasks: Wallet Record Deduplication

**Input**: Design documents from `/specs/006-wallet-record-dedup/`

**Prerequisites**: plan.md ✓, spec.md ✓, data-model.md ✓, contracts/ ✓, quickstart.md ✓

**Organization**: Tasks organized by user story (P1, P2, P3) to enable independent implementation and testing of each story.

**Implementation Strategy**: 
- **MVP Scope**: All three P1 stories (scan, review, execute) are the MVP — dedup is incomplete without all three
- **Parallelization**: Within each story, model/utility implementations can run in parallel before service/CLI integration
- **Atomic Delivery**: Each user story is independently testable but builds on previous story's infrastructure

---

## Phase 1: Setup (Project Infrastructure)

**Purpose**: Prepare wallet pack and CLI structure for dedup feature

- [x] T001 Add dedup subcommand case to packs/wallet/main.go switch statement (line ~40)
- [x] T002 [P] Create packs/wallet/dedup.go with function stubs: detectRecordDuplicates(), reviewDuplicates(), executeDuplicates()
- [x] T003 [P] Create packs/wallet/dedup_test.go with test file structure and helper functions
- [x] T004 Add dedup section to packs/wallet/config.sample.yaml with primaryKeys, optionalKeys, minConfidence fields
- [x] T005 Create packs/wallet/jobs/wallet-dedup/manifest.yaml declaring the dedup job and data declarations

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure and types that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T006 [P] Define Record type in packs/wallet/internal/wallet/wallet.go (extend existing Record struct if needed)
- [x] T007 [P] Define DuplicateGroup struct in packs/wallet/dedup.go with fields: duplicateKey, matchType, confidence, records[]
- [x] T008 [P] Define DedupDecision struct in packs/wallet/dedup.go with fields: duplicateKey, action, keepRecordIds, deleteRecordIds, reason
- [x] T009 [P] Define AuditTrailEntry struct in packs/wallet/internal/state/state.go with fields: timestamp, operation, deletedRecordIds, totals, backupFile
- [x] T010 Implement loadRecords() helper in packs/wallet/dedup.go to read and parse records.json (error handling for invalid JSON)
- [x] T011 Implement loadDedupConfig() helper in packs/wallet/dedup.go to read dedup config from config.yaml or environment overrides
- [x] T012 [P] Implement dedup key matching function matchKey(record1, record2, config) bool in packs/wallet/dedup.go
- [x] T013 Implement confidence calculation function calculateConfidence(record1, record2, config) float64 in packs/wallet/dedup.go
- [x] T014 Implement createBackup(recordsFile string) (backupPath string, error) in packs/wallet/dedup.go with timestamp in filename
- [x] T015 Implement appendAuditTrail(operation string, deletedIDs []string, before/after counts, backupFile) in packs/wallet/internal/state/state.go
- [x] T016 Add unit tests in packs/wallet/dedup_test.go for: loadRecords (valid/invalid JSON), matchKey, calculateConfidence, createBackup

**Checkpoint**: Foundation ready — all data types, config loading, and helper functions in place. User story implementation can now begin.

---

## Phase 3: User Story 1 - Identify Duplicate Wallet Records (Priority: P1) 🎯

**Goal**: Scan records.json and identify all duplicate groups without modifying data

**Independent Test**: Run `auto wallet dedup scan --format json` on test dataset with known duplicates; verify output contains correct groups with all record IDs and confidence scores

### Implementation for User Story 1

- [x] T017 [P] [US1] Implement groupByDuplicateKey() in packs/wallet/dedup.go: iterate records, build map[key][]Record
- [x] T018 [P] [US1] Implement filterUncertainDuplicates() in packs/wallet/dedup.go: filter groups by minConfidence threshold
- [x] T019 [P] [US1] Implement markOriginals() in packs/wallet/dedup.go: sort records in each group by createdAt, mark earliest as isOriginal
- [x] T020 [US1] Implement detectRecordDuplicates(recordsFile, config) ([]DuplicateGroup, error) in packs/wallet/dedup.go (orchestrates T017–T019)
- [x] T021 [US1] Implement formatGroupsText(groups []DuplicateGroup) string in packs/wallet/dedup.go for human-readable output
- [x] T022 [US1] Implement formatGroupsJSON(groups []DuplicateGroup) ([]byte, error) in packs/wallet/dedup.go for JSON output
- [x] T023 [US1] Implement runDedupScan handler in packs/wallet/dedup.go: parse flags (records-file, dedup-config, format, min-confidence), call detectRecordDuplicates, output results
- [x] T024 [US1] Wire runDedupScan into main.go case "dedup scan": err := runDedupScan(os.Args[2:])
- [x] T025 [P] [US1] Add unit tests in packs/wallet/dedup_test.go: TestDetectRecordDuplicatesIntegration (exact matches)
- [x] T026 [P] [US1] Add unit tests for formatGroupsText and formatGroupsJSON (verify output structure)
- [x] T027 [US1] Add integration test in packs/wallet/dedup_test.go: TestDetectRecordDuplicatesIntegration with test records

**Verification Checklist for US1**:
- [ ] Scan command runs without panic or unhandled errors
- [ ] Exact duplicates (same amount/date/counterparty) are identified
- [ ] Similar records (different counterparty) are NOT flagged as duplicates
- [ ] Empty dataset returns "no duplicates found"
- [ ] JSON output is valid and contains duplicateKey, matchType, confidence, records[]
- [ ] Text output is human-readable with group summaries
- [ ] Uncertain duplicates (optional fields differ) are marked correctly

**Checkpoint**: User Story 1 (Scan) is complete and independently testable. No modifications to records.json made.

---

## Phase 4: User Story 2 - Review and Confirm Dedup Action (Priority: P1)

**Goal**: Present duplicate groups to user and collect decisions on which records to keep/delete

**Independent Test**: Run `auto wallet dedup review --records-file test-records.json --decisions-file decisions.json`, provide input "keep-first" for each group; verify decisions.json contains correct DedupDecision entries with keepRecordIds and deleteRecordIds

### Implementation for User Story 2

- [x] T028 [P] [US2] Implement printDuplicateGroup(group DuplicateGroup) void in packs/wallet/dedup.go for interactive display
- [x] T029 [P] [US2] Implement readUserDecision(group DuplicateGroup) (DedupDecision, error) in packs/wallet/dedup.go: prompt user (keep-first/custom/skip)
- [x] T030 [P] [US2] Implement parseCustomDecision(input string, group DuplicateGroup) ([]string, error) in packs/wallet/dedup.go: parse "R1,R3" format into record IDs
- [x] T031 [US2] Implement collectDecisions(groups []DuplicateGroup, interactive bool) ([]DedupDecision, error) in packs/wallet/dedup.go: loop through groups, collect decisions
- [x] T032 [US2] Implement saveDedupDecisions(decisions []DedupDecision, outputPath string) error in packs/wallet/dedup.go: marshal to JSON with timestamp
- [x] T033 [US2] Implement printDecisionSummary(decisions []DedupDecision) void in packs/wallet/dedup.go: show count of groups/records to delete
- [x] T034 [US2] Implement readFinalConfirmation(prompt string) (bool, error) in packs/wallet/dedup.go: prompt "Confirm? (y/n)"
- [x] T035 [US2] Implement reviewCommand handler in packs/wallet/dedup.go: parse flags (records-file, decisions-file, dry-run), load groups, collect decisions, save to file
- [x] T036 [US2] Wire reviewCommand into main.go case "dedup review": err := reviewCommand(os.Args[2:])
- [x] T037 [P] [US2] Add unit tests in packs/wallet/dedup_test.go: TestParseCustomDecision (valid/invalid formats), TestValidateDecisions (ensure at least 1 record kept)
- [x] T038 [P] [US2] Add integration test: TestReviewCommandInteractive (simulate stdin input for user decisions)
- [x] T039 [US2] Add integration test: TestDecisionFilePersistence (verify saved decisions.json structure and can be reloaded)

**Verification Checklist for US2**:
- [ ] Review command loads duplicate groups from scan output (or re-scans if not provided)
- [ ] Interactive prompt displays each group with clear formatting (ID, amount, date, counterparty)
- [ ] User can choose "keep-first", "custom", or "skip" for each group
- [ ] Custom selection parses comma-separated record IDs correctly
- [ ] Decisions are validated: at least 1 record kept per group, no duplicate selections
- [ ] Decisions JSON file created with timestamp and correct structure
- [ ] Summary shows accurate count of records to delete
- [ ] Dry-run mode shows what would happen without writing decisions file
- [ ] Cancellation before final confirmation discards decisions (not persisted)

**Checkpoint**: User Story 2 (Review) is complete and independently testable. Decisions recorded but no records deleted yet.

---

## Phase 5: User Story 3 - Execute Dedup and Update Records (Priority: P1)

**Goal**: Apply user decisions, delete duplicates atomically, create backup, update records.json, and log to audit trail

**Independent Test**: Create decisions file with known deletions, run `auto wallet dedup execute --records-file test-records.json --decisions-file decisions.json --force`; verify: records.json has fewer records, backup exists, audit trail appended to state.json, JSON is valid

### Implementation for User Story 3

- [x] T040 [P] [US3] Implement loadDedupDecisions(decisionFile string) ([]DedupDecision, error) in packs/wallet/dedup.go: unmarshal JSON decisions file
- [x] T041 [P] [US3] Implement applyDecisions(records []Record, decisions []DedupDecision) ([]Record, error) in packs/wallet/dedup.go: filter out deleted record IDs
- [x] T042 [P] [US3] Implement validateUpdateJSON(originalRecords, updatedRecords []Record) error in packs/wallet/dedup.go: ensure valid structure before writing
- [x] T043 [US3] Implement executeDedup(recordsFile, decisionFile, backupDir string, dryRun bool) error in packs/wallet/dedup.go: orchestrate backup, deletion, validation, write
- [x] T044 [US3] Implement writeRecordsJSON(recordsFile string, records []Record) error in packs/wallet/dedup.go: marshal records to JSON, write atomically (temp file + rename)
- [x] T045 [US3] Implement rollbackOnFailure(recordsFile, backupPath string) error in packs/wallet/dedup.go: restore from backup if write fails
- [x] T046 [US3] Implement executionCommand handler in packs/wallet/dedup.go: parse flags (records-file, decisions-file, dry-run, force), call executeDedup, print results
- [x] T047 [US3] Wire executeCommand into main.go case "dedup execute": err := executeCommand(os.Args[2:])
- [x] T048 [US3] Implement readExecutionConfirmation() (bool, error) in packs/wallet/dedup.go: final "Delete X records?" prompt
- [x] T049 [P] [US3] Add unit tests in packs/wallet/dedup_test.go: TestApplyDecisions (correct records filtered), TestValidateUpdateJSON (valid/invalid JSON)
- [x] T050 [P] [US3] Add unit tests: TestAtomicWrite (temp file + rename), TestRollbackOnFailure (backup restored)
- [x] T051 [US3] Add integration test: TestExecuteDedup (full scan → review → execute workflow with real files)
- [x] T052 [US3] Add integration test: TestBackupCreated (verify backup file exists with timestamp, contains original data)
- [x] T053 [US3] Add integration test: TestAuditTrailUpdated (verify audit entry appended to state.json with correct fields)
- [x] T054 [US3] Add integration test: TestDryRunNoModification (execute with --dry-run should not modify records.json or state.json)

**Verification Checklist for US3**:
- [ ] Execute command loads decisions from JSON file
- [ ] Dry-run shows what would be deleted without making changes
- [ ] Backup file created with timestamp before any modification
- [ ] Records matching deleteRecordIds are removed from records.json
- [ ] Remaining record count = before - deleted count
- [ ] Updated records.json is valid JSON (jq . succeeds)
- [ ] Backup file contains exact copy of original (hash match)
- [ ] Audit trail appended to state.json with: timestamp, operation, deletedRecordIds, counts, backupFile
- [ ] Final confirmation prompt honored (answer "n" should abort)
- [ ] Force flag (--force) skips confirmation and proceeds
- [ ] On write failure: both records.json and backup exist; recovery possible

**Checkpoint**: User Story 3 (Execute) is complete and independently testable. Dedup workflow end-to-end working.

---

## Phase 6: End-to-End Integration & Polish

**Purpose**: Full feature validation, edge case handling, and documentation

- [x] T055 [P] Add error handling for missing/corrupt records.json in all commands (graceful exit with helpful error message)
- [x] T056 [P] Add error handling for invalid dedup config (missing primaryKeys, invalid field names)
- [x] T057 [P] Add error handling for disk space validation before backup creation (check available space)
- [x] T058 [P] Add handling for very large records.json (test with 10K+ records, verify <5s scan time and <500MB memory)
- [x] T059 [P] Add logging for dedup operations to packs/wallet/dedup.go (no PII, only record counts/IDs and operation status)
- [x] T060 Update packs/wallet/RUNBOOK.md with: dedup feature overview, scan/review/execute examples, dry-run usage, troubleshooting
- [x] T061 Create packs/wallet/DEDUP_GUIDE.md with step-by-step dedup workflow, examples, and common scenarios
- [x] T062 Run full integration test with quickstart.md Scenario 5 (scan → review → execute on real test dataset)
- [x] T063 [P] Add edge case tests: null/missing fields, 3+ duplicates, empty dataset, all unique records
- [x] T064 [P] Add edge case tests: records with 0 or very large amounts, future dates, special characters in counterparty
- [x] T065 Add performance test: benchmark scan/review/execute on 10K-record dataset, verify <5s total time
- [x] T066 Run `go test ./... -v` to verify all unit and integration tests pass
- [x] T067 Run `go fmt ./...` and `go vet ./...` for code quality
- [x] T068 Final manual testing: scan → review → execute workflow on actual wallet records (with dry-run first)

**Verification Checklist for Polish**:
- [ ] All error cases handled gracefully (no panics, helpful error messages)
- [ ] RUNBOOK.md explains feature and provides examples
- [ ] Code follows Go conventions (naming, error handling, package structure)
- [ ] All tests pass (unit + integration)
- [ ] Performance: scan/review/execute <5s on 10K records
- [ ] Memory: <500MB on typical datasets
- [ ] No PII in logs or error messages
- [ ] Backup files recoverable and named consistently

**Checkpoint**: Feature complete and ready for merge.

---

## Dependencies & Execution Order

### Critical Path (must complete in order):
1. **Phase 1** (Setup) → enables CLI integration
2. **Phase 2** (Foundational) → blocks all user stories
3. **Phase 3** (US1 Scan) → enables US2 and US3
4. **Phase 4** (US2 Review) → depends on US1 output
5. **Phase 5** (US3 Execute) → depends on US2 decisions
6. **Phase 6** (Polish) → can start in parallel with Phase 5, merge last

### Parallelization Opportunities:

**Within Phase 1**:
- T002 (dedup.go) and T003 (dedup_test.go) can be created in parallel
- T004 (config.sample.yaml) independent of T005 (manifest.yaml)

**Within Phase 2**:
- T006–T009 (type definitions) all parallel
- T010–T013 (functions) can start once types defined
- Helper functions (T010–T015) mostly independent

**Within Phase 3 (US1)**:
- T017–T019 (grouping functions) parallel until T020 (orchestration)
- T025–T026 (unit tests) parallel with T023 (scan command)

**Within Phase 4 (US2)**:
- T028–T034 (decision collection functions) mostly parallel
- T037–T039 (tests) parallel until T036 (command handler)

**Within Phase 5 (US3)**:
- T040–T045 (file operations) parallel until T043 (orchestration)
- T049–T054 (tests) parallel with T046–T047 (command handler)

**Within Phase 6 (Polish)**:
- All error handling (T055–T058) parallel with documentation (T060–T061)
- Performance/edge case tests (T063–T065) parallel with Phase 5 implementation

---

## MVP Scope

**Minimum Viable Product**: All three P1 user stories + foundational + setup

- **Users can scan**: Identify duplicates without modifying data
- **Users can review**: Make informed decisions on which to keep/delete
- **Users can execute**: Apply decisions atomically with backups

**Not in MVP** (future work):
- Undo command (restore from backup)
- Scheduled/batch dedup
- Multiple comparison strategies (only default + optional categories/tags)
- Web UI or export functionality
- Cross-pack dedup (only records.json in one pack)

---

## Success Criteria (from spec.md)

- [ ] SC-001: System correctly identifies 100% of duplicates with zero false positives (verified in Phase 3 tests)
- [ ] SC-002: User can review 1000+ groups in single session, <500MB memory (verified in Phase 6 performance test)
- [ ] SC-003: Records.json updated atomically; always valid JSON post-dedup (verified in Phase 5 tests)
- [ ] SC-004: Dedup <5s for 10K records (verified in Phase 6 performance test)
- [ ] SC-005: Backups created, recovery possible (verified in Phase 5 tests and quickstart validation)
- [ ] SC-006: Audit trail records date, records deleted, who initiated (verified in Phase 5 tests)

---

## References

- **Spec**: [spec.md](spec.md) — User stories and requirements
- **Plan**: [plan.md](plan.md) — Technical context and architecture
- **Data Model**: [data-model.md](data-model.md) — Entity definitions and relationships
- **Contracts**: [contracts/dedup-record.md](contracts/dedup-record.md) — Operation protocols and schemas
- **Quickstart**: [quickstart.md](quickstart.md) — Validation scenarios and test examples
