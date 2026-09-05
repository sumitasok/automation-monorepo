# Implementation Checklist: Unified Wallet Sync

**Status**: In Progress
**Last Updated**: 2026-09-05

## Phase 1: Migrate Obsidian Code to Repo ✅

- [ ] **Directory Structure**
  - [ ] Create `packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/` 
  - [ ] Create `packs/expense-domain/sources/wallet/scripts/wallet-sync-unified.sh`
  - [ ] Create `packs/expense-domain/sources/wallet/config/` for YAML configs
  - [ ] Create `data/` directories in ~/automation-monorepo-config (external config)

- [ ] **Port Obsidian Python Logic** (from sa.finances `_db/wallet-sync/`)
  - [ ] Copy `sync.py` → `packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/sync.py`
  - [ ] Copy `apply-labels.mjs` → `packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/apply-labels.js` (convert to Python or keep JS)
  - [ ] Copy `_gen_payload.py` → `packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/gen-payload.py`
  - [ ] Copy `_prep_records.py` → `packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/prep-records.py`
  - [ ] Create extraction engine: `packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/extract-engine.py`
  - [ ] Migrate extraction formats: copy `formats/` and `routing.yaml` to external config

- [ ] **Update Source Tag**
  - [ ] Modify `sync.py`: append `source:refactored-code-0905` to every record note
  - [ ] Verify note format: `<merchant> | via <instrument> | gm:<msgid> | source:refactored-code-0905` (max 255 chars)

- [ ] **Config Files**
  - [ ] Create `routing.yaml` template in `~/automation-monorepo-config/config/expense-domain/wallet/`
  - [ ] Create `tag-registry.yaml` from Obsidian `Tag Registry.md`
  - [ ] Create `email-formats/` directory for `email.<bank>.yaml` patterns
  - [ ] Create template `last-sync.json` (cursor, state, auto-created accounts)

---

## Phase 2: Integrate with Framework ✅

- [ ] **Environment & Credentials**
  - [ ] Modify `sync.py`: read `WALLET_API_TOKEN` from `$CONFIG_PATH/config/wallet/config.yaml`
  - [ ] Read Gmail MCP credentials from framework (via MCP tool registry)
  - [ ] Read Drive folder IDs and Obsidian vault path from config
  - [ ] NO hardcoded paths in code; all derived from `CONFIG_PATH`

- [ ] **Orchestration Script**
  - [ ] Create `scripts/wallet-sync-unified.sh`: orchestrator that:
    - [ ] Sets CONFIG_PATH (default ~/automation-monorepo-config)
    - [ ] Calls sync.py with all required arguments
    - [ ] Handles rate limit retries
    - [ ] Logs output to ~/automation-monorepo-config/data/expense-domain/wallet/logs/
    - [ ] Reports status to `last-sync.json`

- [ ] **Auto Discovery Integration**
  - [ ] Register in framework's job discovery (manifest.yaml if needed)
  - [ ] Enable `auto orchestrate wallet-sync` or `CONFIG_PATH=~/automation-monorepo-config wallet-sync` invocation
  - [ ] Auto-discover script location from packs/ or scripts/ directories

---

## Phase 3: Feature Implementation ✅

- [ ] **Part A: Gmail Sync**
  - [ ] Gmail MCP integration: fetch threads via search query
  - [ ] Extraction engine: apply regex patterns from formats/ before AI
  - [ ] AI fallback: parse unmatched emails, codify format, re-run
  - [ ] Account routing: map bank/card to accountId via routing.yaml
  - [ ] Auto-account creation: create new accounts if no routing match (max 10 cap)
  - [ ] Dual-layer dedup:
    - [ ] Layer 1: check note for gm:<message-id>, skip if found
    - [ ] Layer 2: check same date + amount + merchant, skip if exists without gm:
  - [ ] Record creation: POST to Wallet API with note format + labels
  - [ ] Cursor update: advance last_email_timestamp on success

- [ ] **Part B: Drive Bills Sync**
  - [ ] Drive MCP integration: fetch files from Bills Inbox folder
  - [ ] PDF extraction: engine-first (regex) or AI-OCR
  - [ ] Wallet record matching: date ±3 days, amount ±tolerance
  - [ ] Bill note creation: YAML frontmatter + itemized table + markdown
  - [ ] Product price logging: append to product-prices.jsonl
  - [ ] Fuzzy reconciliation: flag complex splits for manual review
  - [ ] File tracking: processed_drive_files in last-sync.json

- [ ] **Part C: Cross-Source Reconciliation**
  - [ ] Scan new/patched records from Parts A/B
  - [ ] Fuzzy match against other sources (manual Wallet, prior sync path)
  - [ ] Merge logic: richest-first detail, append tags, never replace notes
  - [ ] Conflict detection: flag irreconcilable differences for manual review

- [ ] **Part D: Label Tagging**
  - [ ] Load labels-cache.json or refresh via wallet_list_labels
  - [ ] Create missing labels from Tag Registry
  - [ ] Apply 2–4 labels per record (instrument + category + vendor)
  - [ ] Batch patch records (max 20 per request)
  - [ ] Implement apply-labels.py script (one-shot + dry-run support)

- [ ] **Part E: Obsidian Write-Back**
  - [ ] Read Obsidian vault path from config
  - [ ] Fetch or create monthly expense log: `Expenses/<year>/<YYYY-MM Month>.md`
  - [ ] Append row to expense log table (idempotent by gm: ref)
  - [ ] Link bill notes from Drive/Bills reconciliation
  - [ ] Handle missing templates (create from Template/Expense Log.md)

---

## Phase 4: Unified Trigger Configuration ✅

- [ ] **Update Launchd Plist**
  - [ ] Modify `com.safinances.wallet-sync.plist`: point to repo version instead of Obsidian
  - [ ] Update command to: `CONFIG_PATH=~/automation-monorepo-config /path/to/scripts/wallet-sync-unified.sh`
  - [ ] Disable `com.sumitasok.wallet-sync.plist` (the old automation-monorepo version)
  - [ ] Keep hourly schedule (every :07 minute)

- [ ] **Testing & Verification**
  - [ ] Manual test: run `CONFIG_PATH=~/automation-monorepo-config wallet-sync-unified.sh`
  - [ ] Verify records appear in Wallet within 60 seconds
  - [ ] Verify gm: tag is present in record note
  - [ ] Verify source:refactored-code-0905 tag is present
  - [ ] Verify labels are applied
  - [ ] Verify Obsidian monthly log is updated
  - [ ] Test dedup: re-run same email, verify no duplicate created
  - [ ] Test auto-account: send email from unknown bank, verify account created
  - [ ] Test rate limit: verify 429 retry logic works
  - [ ] Test Drive bill matching: upload receipt, verify matched to Wallet record
  - [ ] Test cross-source merge: verify Gmail + Drive + manual records merge without data loss

---

## Phase 5: Cutover & Cleanup 🎯

- [ ] **Disable Old Triggers**
  - [ ] Disable com.sumitasok.wallet-sync.plist (automation-monorepo version)
  - [ ] Keep com.safinances.wallet-sync.plist (but now pointing to repo)
  - [ ] Verify no duplicate sync runs on launchd schedule

- [ ] **Data Migration** (if needed)
  - [ ] Export last-sync state from Obsidian `last-sync.json` if any pending state
  - [ ] Move labels-cache.json to ~/automation-monorepo-config/ if not already there
  - [ ] Copy routing.yaml from Obsidian to ~/automation-monorepo-config/

- [ ] **Documentation**
  - [ ] Update FRAMEWORK_RULES.md to document `wallet-sync` command
  - [ ] Create WALLET_SYNC_RUNBOOK.md in repo with same Parts A–E structure
  - [ ] Document Tag Registry location (Obsidian file path reference)
  - [ ] Document config structure (routing.yaml, formats/, etc.)

- [ ] **Git Commit**
  - [ ] Stage all new files: scripts, jobs, config templates
  - [ ] Commit with message: "feat(wallet): unified sync with Obsidian integration, labeling, bills reconciliation"
  - [ ] Push to feature/restructure-architecture

---

## Known Risks & Mitigation

| Risk | Mitigation |
|------|-----------|
| Duplicate sync from two triggers (old + new) | Disable com.sumitasok.plist immediately after cutover |
| Missing config files break sync | Provide template configs in repo; fail loudly if missing |
| Obsidian vault unavailable during sync | Make write-back optional; log warning but continue |
| Cursor corruption → infinite sync | Validate cursor before advancing; back up last-sync.json before update |
| Rate limit exceeded | Implement exponential backoff + 429 retry logic |
| Product prices for millions of items | Prune product-prices.jsonl periodically (archive old entries) |

---

## Notes

- **Tag Registry Reference**: Currently in Obsidian at `Wallet/Tag Registry.md`. During migration, convert to YAML at `~/automation-monorepo-config/config/expense-domain/wallet/tag-registry.yaml` or keep reference to Obsidian file path.
- **Obsidian Write-Back**: Requires read/write access to vault. Recommend setting vault path in config and validating path exists at startup.
- **Email Format Codification**: First run against new bank will require AI + manual format definition. Subsequent runs use regex (zero tokens). Expected learning curve: 2–3 emails per new bank before patterns stabilize.
- **Product Prices**: `product-prices.jsonl` files accumulate in Expenses/ folders. Add yearly archival task to purge old entries if storage becomes concern.

