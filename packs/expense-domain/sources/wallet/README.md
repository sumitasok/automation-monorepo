# Unified Wallet Sync

**Purpose**: Synchronize financial transactions from multiple sources (Gmail bank alerts, Google Drive receipts, manual entries) to BudgetBakers Wallet API with automatic deduplication, categorization, and labeling.

**Status**: Phase 1 (Migration) Complete | Phase 2 (Framework Integration) Pending

---

## Quick Start

### Prerequisites

```bash
# Ensure config directory exists with required files
ls ~/automation-monorepo-config/config/expense-domain/wallet/
# Expected: config.yaml, routing.yaml, tag-registry.yaml, email-formats/

# Ensure Obsidian vault is accessible
ls ~/sa.finances/Expenses/
# Expected: Year folders and monthly logs
```

### Running Sync Locally (Current Phase)

```bash
# Navigate to repo root
cd /Users/sumitasok/Claude/Projects/automation-monorepo/.worktrees/restructure-architecture

# Run sync.py directly (Phase 2 will create wrapper script)
python3 packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/sync.py \
  --config-path ~/automation-monorepo-config \
  --dry-run
```

### Framework Integration (Phase 2)

```bash
# Once Phase 2 is complete:
CONFIG_PATH=~/automation-monorepo-config wallet-sync-unified.sh
```

---

## File Structure

```
packs/expense-domain/sources/wallet/
├── jobs/
│   └── wallet-sync-unified/          # Main sync job
│       ├── sync.py                   # Core sync logic (920 lines)
│       ├── apply-labels.mjs          # Label application script
│       ├── extract-engine.py         # Deterministic email/PDF parsing
│       ├── gen-payload.py            # Wallet payload generation
│       └── prep-records.py           # Record preprocessing
├── scripts/                          # CLI entry points (Phase 2)
│   └── wallet-sync-unified.sh        # Main orchestrator (TBD)
├── config/                           # Template configs
│   └── (populated in Phase 2)
└── README.md                         # This file
```

**External Config** (not in repo, per constitution Principle II):

```
~/automation-monorepo-config/config/expense-domain/wallet/
├── config.yaml                       # API tokens, paths, behavior
├── routing.yaml                      # Bank/card → accountId mappings
├── tag-registry.yaml                 # Label definitions
├── email-formats/                    # Regex patterns for known email senders
│   ├── email.hdfc.yaml
│   ├── email.icici.yaml
│   └── ...
└── (more as needed)

~/automation-monorepo-config/data/expense-domain/wallet/
├── last-sync.json                    # Cursor + sync state
├── labels-cache.json                 # slug → UUID mappings
├── logs/                             # Sync run logs
└── records-YYYY-MM-DD.jsonl         # Fetched records (temporary)
```

---

## Key Features

### Part A: Gmail Bank Alert Sync ✅
- Fetch emails from bank/card alert senders via Gmail API
- Engine-first extraction: deterministic regex patterns before AI
- Auto-account creation if bank not in routing table (capped at 10)
- Dual-layer deduplication:
  - Layer 1: `gm:<message-id>` idempotency key
  - Layer 2: Fuzzy match on date + amount + merchant (skip manual duplicates)
- Rate-limited Wallet API calls (300 req/hr, batch by 20)

### Part B: Drive Bills Reconciliation 🚧
- Upload receipts to Google Drive Bills Inbox
- Extract vendor, date, line items, total amount
- Match to existing Wallet records (±3 days, ±tolerance)
- Enrich matched records with category, line items
- Create bill notes in Obsidian with YAML frontmatter
- Log product prices for shopping optimization

### Part C: Cross-Source Reconciliation 🚧
- Merge Gmail + Drive + manual entries describing same transaction
- Richest-first logic: Drive details > Gmail details > manual notes
- Preserve all information (append tags, never replace)
- Flag ambiguous matches for manual review

### Part D: Intelligent Label Tagging ✅
- Hardcoded merchant/bank keyword matching
- Apply 2–4 labels per transaction (instrument + category + vendor)
- Labels-cache.json for slug → UUID mapping
- Tag Registry integration from Obsidian

### Part E: Obsidian Write-Back 🚧
- Update monthly expense logs with synced records
- Create bill notes with cross-links to Wallet
- Idempotent by `gm:` / `drive:` tags (re-runs don't duplicate)

---

## Configuration

### config.yaml (Required)

```yaml
wallet:
  api_token: "${WALLET_API_TOKEN}"  # Or set WALLET_API_TOKEN env var
  api_base_url: "https://rest.budgetbakers.com/wallet/v1/api"
  gmail_credentials_path: "~/.config/sa-finances/gmail-credentials.json"
  gmail_token_path: "~/.config/sa-finances/gmail-token.json"
  drive_bills_folder_id: "1DXizYKYGSg8pPO1_tbXPLTUOENOwfMR6"
  obsidian_vault_path: "~/sa.finances"
  auto_account_cap: 10
```

### routing.yaml (Required)

Maps bank/card identifiers to Wallet accountIds:

```yaml
accounts:
  - bank: "HDFC"
    card_last4: "3690"
    account_id: "97320818-c6df-4fbc-be24-baa5fbea7cc5"
  - ...
```

### tag-registry.yaml (Required)

Defines available labels with category/instrument grouping:

```yaml
tags:
  groceries:
    name: "Groceries"
    uuid: null  # Populated on first run
  hdfc:
    name: "HDFC"
    uuid: null
  # ...
```

### email-formats/ (Required)

Directory of `email.<bank>.yaml` files defining regex patterns for known senders.

Example: `email.hdfc.yaml`

```yaml
bank: "hdfc"
patterns:
  - name: "cc_alert_v1"
    sender: "alerts@hdfcbank.com"
    subject_regex: "Credit Card.*Transaction"
    body_patterns:
      - field: "merchant"
        regex: "Establishment.*?([A-Z ]+)"
      # ...
```

---

## Record Format

All records created include source tag: `source:refactored-code-0905`

**Note Field Format**:
```
<merchant> | via <instrument> | gm:<message-id> | source:refactored-code-0905
```

**Example**:
```
BLINKIT | via HDFC CC x3690 | gm:abc123def456 | source:refactored-code-0905
```

---

## Development Phases

| Phase | Task | Status | Timeline |
|-------|------|--------|----------|
| **1** | Migrate Obsidian code | ✅ Complete | 2026-09-05 |
| **2** | Framework integration | 🚧 Pending | Week 1 |
| **3** | Feature implementation | 🚧 Pending | Week 2 |
| **4** | Trigger configuration | 🚧 Pending | Week 2 |
| **5** | Validation & cutover | 🚧 Pending | Week 3 |

See `specs/010-unified-wallet-sync/` for detailed specification and tasks.

---

## Testing

### Manual Testing (Phase 1)

```bash
# Syntax check
python3 -m py_compile packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/sync.py

# Import check
python3 -c "import sys; sys.path.insert(0, 'packs/expense-domain/sources/wallet/jobs/wallet-sync-unified'); import sync"
```

### End-to-End Testing (Phase 2+)

See `specs/010-unified-wallet-sync/QUICKSTART.md` for 5 validation scenarios:
1. Fresh Sync (New Bank Alert)
2. Deduplication (Re-run Same Sync)
3. Drive Bill Upload & Enrichment
4. Cross-Source Merge
5. Hourly Schedule (Launchd)

---

## Troubleshooting

### Import Errors

```bash
# Ensure CONFIG_PATH is set
export CONFIG_PATH=~/automation-monorepo-config

# Check Python path
python3 -c "import sys; print('\\n'.join(sys.path))"
```

### Gmail API Issues

```bash
# Re-authorize Gmail
python3 packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/sync.py --auth

# Check credentials
ls ~/.config/sa-finances/
```

### Config File Issues

```bash
# Validate YAML syntax
python3 -c "import yaml; yaml.safe_load(open('~/automation-monorepo-config/config/expense-domain/wallet/routing.yaml'))"
```

---

## References

- **Specification**: `/specs/010-unified-wallet-sync/spec.md`
- **Implementation Plan**: `/specs/010-unified-wallet-sync/plan.md`
- **Data Model**: `/specs/010-unified-wallet-sync/architecture/data-model.md`
- **Interface Contracts**: `/specs/010-unified-wallet-sync/architecture/contracts.md`
- **Quickstart Validation**: `/specs/010-unified-wallet-sync/QUICKSTART.md`
- **Implementation Tasks**: `/specs/010-unified-wallet-sync/tasks.md`

---

## Notes

- All config is external (~/automation-monorepo-config/) per Automation Workspace Constitution Principle II
- No hardcoded paths in code; all use CONFIG_PATH injection
- Source tag "source:refactored-code-0905" on all created records for provenance tracking
- Framework integration in Phase 2 will provide `wallet-sync-unified.sh` wrapper and auto-discovery

