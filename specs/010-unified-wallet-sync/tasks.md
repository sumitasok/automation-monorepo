# Implementation Tasks: Unified Wallet Sync

**Phase**: 1 (Migrate Obsidian Code to Repo)  
**Status**: In Progress  
**Last Updated**: 2026-09-05  
**Target Completion**: Week 1

---

## Phase 1: Migrate Obsidian Code to Repo

### Setup Tasks

**T1.1: Create Directory Structure** [SETUP]
- [ ] Create `packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/` directory
- [ ] Create `packs/expense-domain/sources/wallet/scripts/` directory
- [ ] Create `packs/expense-domain/sources/wallet/config/` directory (template configs)
- [ ] Create external config structure in `~/automation-monorepo-config/config/expense-domain/wallet/`
- [ ] Create external data structure in `~/automation-monorepo-config/data/expense-domain/wallet/` with subdirs: logs/, formats/

**T1.2: Set Up Git Ignore** [SETUP]
- [ ] Update .gitignore to exclude `~/.specify/`, `.worktrees/`, external config directory
- [ ] Verify no secrets committed (check for .env files, credentials)

---

### Migration Tasks (Sequential)

**T1.3: Copy Core sync.py** [CORE, SEQUENTIAL]
- Depends on: T1.1
- [ ] Copy `/Users/sumitasok/Library/Mobile Documents/iCloud~md~obsidian/Documents/sa.finances/_db/wallet-sync/sync.py` → `packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/sync.py`
- [ ] Review all hardcoded paths in sync.py; replace with environ.get() calls for CONFIG_PATH injection
- [ ] Update imports for local module paths (extract-engine.py, etc.)
- [ ] Verify function signatures match contracts.md

**T1.4: Update Source Tag in sync.py** [CORE, SEQUENTIAL]
- Depends on: T1.3
- [ ] Locate `buildNote()` function or equivalent record creation logic
- [ ] Append `source:refactored-code-0905` to every record's note
- [ ] Verify note format: `<merchant> | via <instrument> | gm:<msgid> | source:refactored-code-0905` (max 255 chars)
- [ ] Add truncation logic if note exceeds 255 chars (preserve gm: tag)
- [ ] Test with sample data to ensure formatting correct

**T1.5: Copy apply-labels Logic** [CORE, SEQUENTIAL]
- Depends on: T1.1
- [ ] Copy `/Users/sumitasok/Library/Mobile Documents/iCloud~md~obsidian/Documents/sa.finances/_db/wallet-sync/apply-labels.mjs` → `packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/apply-labels.py` (convert to Python)
- [ ] OR keep as .mjs and add Node.js wrapper if preferred
- [ ] Review label selection logic; ensure it matches contracts.md
- [ ] Update paths for labels-cache.json and tag-registry.yaml

**T1.6: Copy Helper Scripts** [CORE, SEQUENTIAL]
- Depends on: T1.1
- [ ] Copy `_gen_payload.py` → `packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/gen-payload.py`
- [ ] Copy `_prep_records.py` → `packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/prep-records.py`
- [ ] Update paths and imports for new location

**T1.7: Copy/Create Extraction Engine** [CORE, SEQUENTIAL]
- Depends on: T1.1
- [ ] Check if engine.py exists in Obsidian `_db/extract/`
- [ ] Copy or create `packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/extract-engine.py`
- [ ] Ensure engine.py accepts envelope format (sender, subject, date, body)
- [ ] Verify engine.py returns structured result with action + record fields

**T1.8: Migrate Email Formats** [CORE, SEQUENTIAL]
- Depends on: T1.1
- [ ] Copy `/Users/sumitasok/Library/Mobile Documents/iCloud~md~obsidian/Documents/sa.finances/_db/extract/formats/` → `~/automation-monorepo-config/config/expense-domain/wallet/email-formats/`
- [ ] Copy test samples directory if present
- [ ] Verify YAML syntax in all format files
- [ ] Update any hardcoded paths in formats to use CONFIG_PATH

**T1.9: Create Routing Config Template** [CORE, SEQUENTIAL]
- Depends on: T1.1
- [ ] Create `routing.yaml` template in `~/automation-monorepo-config/config/expense-domain/wallet/`
- [ ] Populate with entries from Obsidian runbook (HDFC, Canara, ICICI, etc.)
- [ ] Verify UUID format for accountIds
- [ ] Document routing.yaml structure in README

**T1.10: Create Tag Registry from Obsidian** [CORE, SEQUENTIAL]
- Depends on: T1.1
- [ ] Extract tag definitions from Obsidian `Wallet/Tag Registry.md`
- [ ] Create `tag-registry.yaml` in `~/automation-monorepo-config/config/expense-domain/wallet/`
- [ ] Map each tag slug → UUID (initially null; will be populated on first run)
- [ ] Include category tags, instrument tags, vendor tags

**T1.11: Create Sync State Template** [CORE, SEQUENTIAL]
- Depends on: T1.1
- [ ] Create template `last-sync.json` in `~/automation-monorepo-config/data/expense-domain/wallet/`
- [ ] Initialize with:
  - `last_email_timestamp`: "2026-09-04T00:00:00Z"
  - `last_run_status`: "success"
  - `processed_drive_files`: []
  - `auto_created_accounts`: []
- [ ] Document all fields in README

**T1.12: Create Main Config File** [CORE, SEQUENTIAL]
- Depends on: T1.1
- [ ] Create `config.yaml` in `~/automation-monorepo-config/config/expense-domain/wallet/`
- [ ] Include placeholders for: WALLET_API_TOKEN, gmail_credentials_path, drive_bills_folder_id, obsidian_vault_path
- [ ] Document all configuration options in README

---

### Validation Tasks (Parallel)

**T1.13: Syntax Check Python Files** [P, VALIDATION]
- Depends on: T1.3, T1.5, T1.6, T1.7
- [ ] Run `python3 -m py_compile` on all .py files
- [ ] Check for import errors: `python3 -c "import sys; sys.path.insert(0, 'packs/...'); import sync"`
- [ ] Verify no hardcoded Obsidian paths in code

**T1.14: Validate YAML Configs** [P, VALIDATION]
- Depends on: T1.8, T1.9, T1.10, T1.11, T1.12
- [ ] Run `python3 -m yaml` validation on all .yaml files
- [ ] Check routing.yaml has valid UUIDs
- [ ] Check tag-registry.yaml has proper structure
- [ ] Check email-formats/*.yaml have valid regex patterns

**T1.15: Verify Directory Structure** [P, VALIDATION]
- Depends on: T1.1 through T1.12
- [ ] Confirm all expected files exist in packs/expense-domain/sources/wallet/
- [ ] Confirm all expected files exist in ~/automation-monorepo-config/
- [ ] Check file permissions (executables should be 755)

---

### Finalization Tasks (Sequential)

**T1.16: Update Git and Create Initial Commit** [FINALIZE, SEQUENTIAL]
- Depends on: T1.13, T1.14, T1.15 (all validation tasks)
- [ ] Run `git status` to verify only intended files are staged
- [ ] Add all new files: `git add packs/expense-domain/sources/wallet/ ~automation-monorepo-config/`
- [ ] Commit with message: "feat(wallet): Phase 1 - migrate Obsidian sync.py and configs to repo"
- [ ] Verify commit includes all migration files

**T1.17: Create README for Migrated Code** [FINALIZE, SEQUENTIAL]
- Depends on: T1.16
- [ ] Create `packs/expense-domain/sources/wallet/README.md` documenting:
  - Purpose (unified wallet sync)
  - File structure (jobs/, scripts/, config/)
  - Configuration requirements
  - Running sync locally
  - Testing procedures
- [ ] Include examples of CONFIG_PATH usage

---

## Phase 2-5 Task Stubs (For Future)

**P2: Integrate with Framework** (Week 1, Phase 2)
- Framework integration tasks (orchestration, auto-discovery)

**P3: Feature Implementation** (Week 2, Phase 3)
- Gmail sync, Drive bills, reconciliation, labeling, Obsidian write-back

**P4: Configuration & Triggers** (Week 2, Phase 4)
- Update launchd plist, disable old triggers

**P5: Validation & Cutover** (Week 3, Phase 5)
- Run quickstart scenarios, merge to main, deploy

---

## Task Execution Rules

**Sequential (SEQUENTIAL)**: Must complete in order; previous task must pass before next starts  
**Parallel [P]**: Can run simultaneously; independent of other parallel tasks  
**Dependencies**: Always check "Depends on" field; cannot start until dependency completed

**Stop Condition**: If any non-parallel task FAILS, halt execution and report error  
**Partial Failure**: If parallel [P] task fails, continue with others, report failure at end

---

## Success Criteria

**Phase 1 Complete When**:
- ✅ All directory structures created
- ✅ All Obsidian code copied to repo with updated imports
- ✅ Source tag ("source:refactored-code-0905") added to records
- ✅ All config files created and validated
- ✅ No hardcoded paths in code; all use CONFIG_PATH
- ✅ All syntax checks pass
- ✅ All YAML validation passes
- ✅ Initial commit created with migration changes
- ✅ README created documenting migrated code

---

## Notes

- **Obsidian Paths**: All source files are in `/Users/sumitasok/Library/Mobile Documents/iCloud~md~obsidian/Documents/sa.finances/_db/wallet-sync/`
- **Config Format**: All configs use YAML; no JSON for config files (except last-sync.json which tracks state)
- **Git Commit**: One focused commit per phase; include all files for that phase
- **Testing**: After Phase 1, code should be syntactically valid; functional testing in Phase 2

