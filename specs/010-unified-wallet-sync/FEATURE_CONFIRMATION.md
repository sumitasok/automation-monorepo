# Feature Confirmation: Unified Wallet Sync

**Date**: 2026-09-05  
**Status**: Ready for Implementation  
**Confirmed By**: User (via `/speckit-specify` + explicit request to bring Obsidian logic into repo)

---

## ✅ ALL OBSIDIAN FEATURES INCLUDED

### Part A: Gmail Bank Alert Sync
- ✅ **Hourly schedule** (every :07 minute via launchd)
- ✅ **Deterministic email parsing** (engine-first extraction using regex formats)
- ✅ **AI fallback** for new email formats + automatic format codification
- ✅ **Account routing** from bank/card identification to Wallet accountId
- ✅ **Auto-account creation** with 10-account cap + tracking
- ✅ **Dual-layer deduplication**:
  - Idempotency key: `gm:<gmail-message-id>` in record note
  - Fuzzy fallback: skip if same date + amount + merchant exists (manual duplicate detection)
- ✅ **Smart merging** of matched records (don't create duplicates, preserve existing data)
- ✅ **Rate limit handling** (exponential backoff on HTTP 429, max 3 retries)
- ✅ **Cursor state management** (advance only on success, safe for hourly re-runs)

### Part B: Drive Bills Reconciliation
- ✅ **PDF receipt upload to Bills Inbox folder**
- ✅ **Automatic vendor/date/line items extraction** (engine-first for text, AI-OCR for scans)
- ✅ **Fuzzy matching** against Wallet records (date ±3 days, amount ±tolerance)
- ✅ **Record enrichment** when matched (append drive:<fileId>, apply category, add line items)
- ✅ **Bill note creation** in Obsidian with YAML frontmatter (bill_id, date, vendor, category, line items, cross-links)
- ✅ **Product price ledger** (append to product-prices.jsonl per line item)
- ✅ **Complex split detection** (flag 1500₹ bill vs 1000+500₹ records for manual review)
- ✅ **File tracking** (processed_drive_files in last-sync.json prevents reprocessing)

### Part C: Cross-Source Reconciliation
- ✅ **Intelligent merging** of Gmail + Drive + manual records describing same transaction
- ✅ **Richest-source logic** (Drive line items + category > Gmail merchant > manual notes)
- ✅ **Data preservation** (APPEND source tags gm:/drive: to note, never replace)
- ✅ **Conflict detection** (flag irreconcilable differences for manual review)
- ✅ **Obsidian note consolidation** (monthly expense log shows one row per merged transaction)

### Part D: Label Tagging
- ✅ **Labels cache** (labels-cache.json maintains slug → labelId mapping)
- ✅ **Tag Registry integration** (read from Obsidian Wallet/Tag Registry.md)
- ✅ **Automatic label creation** (missing labels created in Wallet on first run)
- ✅ **Smart label application** (2–4 labels per transaction: instrument + category + optional vendor)
- ✅ **Batch patching** (max 20 records per request to respect 300 req/hr rate limit)
- ✅ **Historical re-tagging** (apply-labels.py script for one-shot refresh of all records)

### Part E: Obsidian Write-Back
- ✅ **Monthly expense log updates** (Expenses/<year>/<YYYY-MM Month>.md)
- ✅ **Row idempotency** (keyed by gm: or drive: ref, re-runs skip existing rows)
- ✅ **Template support** (create monthly logs from Template/Expense Log.md if missing)
- ✅ **Cross-links** (expense log row links bill notes; bill notes link Wallet records and gm: refs)
- ✅ **Audit trail** (every synced transaction has an Obsidian trail)

---

## ✅ REFACTORED CODE TAG

**All records created by unified sync will include**:
```
source:refactored-code-0905
```
in the record note (appended after gm: tag).

This distinguishes refactored-code records from:
- Manual Wallet app entries (no tag)
- Legacy sync paths (different source tag)

---

## ✅ UNIFIED CONFIGURATION

**Single entry point**:
```bash
CONFIG_PATH=~/automation-monorepo-config wallet-sync-unified.sh
```

All configuration lives in `~/automation-monorepo-config/`:
```
~/automation-monorepo-config/
├── config/expense-domain/wallet/
│   ├── config.yaml                  # Wallet API token, Gmail/Drive creds
│   ├── routing.yaml                 # Bank/card → accountId mappings
│   ├── tag-registry.yaml            # Label definitions (from Obsidian Tag Registry.md)
│   └── email-formats/               # Bank-specific email parsing rules
├── data/expense-domain/wallet/
│   ├── last-sync.json               # Sync state (cursor, auto-created accounts)
│   ├── labels-cache.json            # slug → labelId cache
│   ├── product-prices.jsonl         # Shopping optimization ledger
│   └── logs/                        # Sync run logs
```

**No file path references in command**. Framework discovers everything via CONFIG_PATH.

---

## ✅ SINGLE LAUNCHD TRIGGER

**Before**: Two competing triggers
- com.safinances.wallet-sync.plist → Obsidian Claude Code (hourly, more features)
- com.sumitasok.wallet-sync.plist → automation-monorepo (every 4h, fewer features)
- **Result**: Duplicate records with different note formats

**After**: One unified trigger
- com.safinances.wallet-sync.plist → Repo version with ALL Obsidian features (hourly)
- com.sumitasok.wallet-sync.plist → **DISABLED**
- **Result**: Single sync job, no duplicates, all features preserved + refactored

---

## 🎯 What User Gets

| Feature | Before | After |
|---------|--------|-------|
| **Email sync frequency** | Hourly (Obsidian) | Hourly ✅ |
| **Email dedup** | gm: idempotency key | gm: + fuzzy ✅ |
| **Account routing** | Obsidian config | Repo config ✅ |
| **Auto-account creation** | Yes (Obsidian) | Yes (repo) ✅ |
| **Drive bills matching** | Yes (Obsidian) | Yes (repo) ✅ |
| **Bill notes** | Yes (Obsidian) | Yes (repo) ✅ |
| **Product prices** | Yes (Obsidian) | Yes (repo) ✅ |
| **Cross-source merge** | Yes (Obsidian) | Yes (repo) ✅ |
| **Label tagging** | Yes (Obsidian) | Yes (repo) ✅ |
| **Obsidian write-back** | Yes (Obsidian) | Yes (repo) ✅ |
| **Source tagging** | No | Yes ✅ `source:refactored-code-0905` |
| **Duplicate records** | YES (two triggers) | NO (unified) ✅ |
| **Codebase location** | Obsidian vault | Repo ✅ |
| **Maintainability** | Manual in Obsidian | Version controlled ✅ |

---

## 📋 Implementation Phases

1. **Phase 1**: Migrate Obsidian code (sync.py, label logic, formats, routing) → repo
2. **Phase 2**: Integrate with framework (CONFIG_PATH, auto-discovery, no hardcoded paths)
3. **Phase 3**: Feature implementation (Parts A–E, all 40+ requirements)
4. **Phase 4**: Unified trigger configuration (update plist, disable old trigger)
5. **Phase 5**: Cutover & cleanup (migrate state, test, commit, document)

---

## ✅ Confirmation Checklist

- ✅ **Spec created** with all 40+ functional requirements (FR-A through FR-G)
- ✅ **Implementation checklist** created with 50+ tasks across 5 phases
- ✅ **All Obsidian features** documented for inclusion
- ✅ **Source tag** specified: `source:refactored-code-0905`
- ✅ **Single trigger** confirmed: com.safinances unified, com.sumitasok disabled
- ✅ **Config structure** defined (routing.yaml, email-formats/, labels-cache.json)
- ✅ **Parts A–E** scoped (Gmail sync, Bills, Reconciliation, Labeling, Obsidian write-back)
- ✅ **Ready for implementation**

---

## 🚀 Next Steps

1. Read back the Obsidian `sync.py` to understand the implementation patterns
2. Start Phase 1: Create directory structure in packs/expense-domain/sources/wallet/
3. Port sync.py and apply logic to add source:refactored-code-0905 tag
4. Create orchestrator script (wallet-sync-unified.sh)
5. Test end-to-end (manual + launchd)
6. Update plist and disable old trigger
7. Commit and document

