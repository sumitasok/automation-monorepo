# Feature Specification: Unified Wallet Sync with Obsidian Integration

**Feature Branch**: `feature/restructure-architecture` (Phase: Unified Wallet Sync)

**Created**: 2026-09-05

**Status**: In Progress

**Input**: User requirement to unify two competing wallet sync triggers (com.safinances vs com.sumitasok) into a single, comprehensive wallet sync system that incorporates all Obsidian logic (labeling, dedup, bills reconciliation, account routing) into the automation-monorepo code path.

---

## Clarifications *(from speckit-clarify session)*

### Session 2026-09-05

- **Q: How is the wallet sync identifying which labels to apply?**  
  **A**: Labels are identified by hardcoded merchant/bank keyword matching:
  1. Extract merchant name & bank/card from transaction
  2. Fuzzy-match against keyword list in `sync.py` (e.g., "blinkit" → label slug "blinkit", "zomato" → "food-delivery")
  3. Look up slug in `labels-cache.json` to get labelId UUID
  4. Return 2–4 unique label UUIDs per transaction
  
  **Implementation**: `label_ids_for_record(record, cache)` at line 277 of Obsidian `sync.py`

- **Q: How is the wallet sync getting the data from Gmail?**  
  **A**: Two-stage extraction:
  1. **Stage 1 (Gmail API)**: Query Gmail API for threads from `from:(bank.in)` senders, extract `sender`, `subject`, `date`, `body` (base64 decoded from Gmail payload), skip already-processed threads (marked with `claude-read` label)
  2. **Stage 2 (Engine-First Parsing)**: 
     - Feed envelope (sender, subject, date, body) to `engine.py` with regex patterns from `formats/email.<bank>.yaml`
     - If matched: Extract merchant, amount, date, currency, card_last4 directly (zero AI cost)
     - If unmatched: AI parses email body + codifies format (add new regex to formats/, then reprocess)
     - Subsequent emails from that bank use the new regex (deterministic, no AI)
  
  **Implementation**: `fetch_gmail_threads()` at line 310, `run_engine()` at line 390 of Obsidian `sync.py`

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Hourly Wallet Sync from Gmail (Priority: P1)

A user receives bank and credit card alert emails throughout the day. They want these transactions automatically imported into their Wallet app (BudgetBakers) on an hourly schedule, with:
- Automatic merchant/account routing based on bank identification
- Smart deduplication using Gmail message IDs (idempotency)
- Fuzzy matching against existing records to prevent manual duplicates
- Proper categorization and labeling based on transaction type
- Write-back to Obsidian monthly expense logs for audit trail

**Why this priority**: This is the core income flow. Broken sync means transactions aren't tracked. This is the foundation all other features depend on.

**Independent Test**: Every bank alert email since last sync (or last 7 days if fresh start) is fetched, parsed, deduplicated, routed to the correct account, labeled, and written to both Wallet API and Obsidian monthly logs without duplicates. Test passes if:
- All new transactions appear in Wallet within 1 hour
- No duplicates created (gm: idempotency key prevents re-runs)
- Records in Obsidian match Wallet
- Labels applied correctly per tag registry

**Acceptance Scenarios**:

1. **Given** a new HDFC CC alert arrives in Gmail, **When** sync runs (within 60 min), **Then** transaction appears in Wallet under "HDFC CC x3690" account with correct amount/merchant and `gm:<message-id>` in note
2. **Given** the same email is re-processed (e.g., cursor sync fails), **When** sync runs again, **Then** the existing record is found via idempotency key and NO duplicate is created
3. **Given** a manual entry exists for the same transaction (same amount, date, merchant but no gm: tag), **When** sync finds it, **Then** it's skipped as "fuzzy duplicate" with a logged warning
4. **Given** an email from an unknown bank arrives, **When** sync processes it, **Then** the transaction is either auto-routed to a matching existing account OR a new account is created (if under 10-account cap)
5. **Given** sync completes successfully, **When** the user checks Obsidian, **Then** a new row appears in `Expenses/2026/2026-09 September.md` with the transaction details and gm: ref

---

### User Story 2 - Drive Bills Reconciliation (Priority: P2)

The user uploads receipt PDFs to a "Bills Inbox" folder in Google Drive. They want these to:
- Auto-extract vendor, date, line items, total amount
- Match against existing Wallet records within ±3 days
- Enrich matched records with category/line items OR create new records if no match
- Generate bill notes in Obsidian with YAML frontmatter and itemized line items
- Track product prices for shopping optimization analysis

**Why this priority**: Bills are the richest data source (line items, vendor proof, tax info). Linking bills to transactions closes the reconciliation loop.

**Independent Test**: A receipt PDF is uploaded to Bills Inbox. Within 1 hour, if a matching transaction exists in Wallet (same date ±3 days, similar amount), the record is enriched with bill reference and category. If no match, a new record is created. A bill note file is created at `Expenses/2026/Bills/bill-260905-starbucks.md` with line items. Product prices are logged. Test passes if all three artifacts (Wallet record, Obsidian bill note, product-prices.jsonl line) are created and cross-linked.

**Acceptance Scenarios**:

1. **Given** a Starbucks receipt is uploaded to Bills Inbox on 2026-09-05, **When** sync runs, **Then** it matches to a Wallet transaction from 2026-09-04/05 and patches it with `drive:<fileId>` in note + category
2. **Given** the bill has 3 line items, **When** the bill note is created, **Then** `Expenses/2026/Bills/bill-260905-starbucks.md` contains an itemized table with qty/unit price/total
3. **Given** each line item in the bill, **When** sync completes, **Then** one JSON line is appended to `Expenses/2026/Bills/product-prices.jsonl` with normalized product name, unit price, vendor, date
4. **Given** a scanned image receipt (not a text PDF), **When** sync runs, **Then** AI-OCR is used instead of regex extraction (slower but necessary)
5. **Given** a bill total is 1500 INR but Wallet records show only 1000 INR + 500 INR split, **When** sync runs, **Then** both records are flagged for manual reconciliation (too many splits to auto-merge safely)

---

### User Story 3 - Cross-Source Reconciliation (Priority: P2)

When the same real-world transaction is recorded multiple ways (Gmail alert, Drive bill, manual entry), the system intelligently merges them:
- Preserves all information (Gmail merchant, Drive line items, manual notes)
- Appends source tags (gm:, drive:) without replacing content
- Applies richest-source logic (Drive detail > Gmail detail > manual detail)
- Prevents data loss during merge

**Why this priority**: Users often enter transactions manually AND receive email alerts. Without reconciliation, duplicates accumulate; with it, you get a single unified record with all context.

**Independent Test**: A transaction created from Gmail alert (merchant name, amount, date) is matched to a manually-entered record (same day, same amount) plus a Drive bill (line items, category). All three sources are merged into one Wallet record with fields populated from all three sources, and the Obsidian note links all three artifacts.

**Acceptance Scenarios**:

1. **Given** Gmail created a record with merchant "Amazon" but no category, Drive bill provides category "Electronics" and line items, **When** merge runs, **Then** the Wallet record has category + line items + gm: + drive: tags in note (nothing is lost)
2. **Given** a manual note on the record says "Split between 2 people", **When** merge adds Drive bill info, **Then** the manual note is preserved (appended to note, not replaced)
3. **Given** conflicting data (Gmail says 1000 INR, manual entry says 1200 INR), **When** merge runs, **Then** the transaction is flagged for manual review instead of auto-merging

---

### User Story 4 - Intelligent Label Application (Priority: P1)

Transactions are automatically tagged with 2–4 relevant labels based on:
- Category (e.g., dining, groceries, transport)
- Payment instrument (e.g., HDFC CC, Canara CC)
- User-defined rules from Tag Registry

**Why this priority**: Labels are how users organize and filter. Without them, transactions are not actionable for analytics or budgeting.

**Independent Test**: A grocery purchase from Blinkit via Canara CC is imported. The system applies labels: "groceries" + "canara-cc" + "blinkit" (optional vendor label). User can filter Wallet by label. Test passes if all labels exist and are correctly applied.

**Acceptance Scenarios**:

1. **Given** a transaction is categorized as "Dining", **When** labels-cache.json contains "dining" label ID, **Then** the record is created with `labelIds: [dining-id]`
2. **Given** a Canara CC transaction, **When** sync runs, **Then** the "canara-cc" label is always applied (instrument label)
3. **Given** a vendor-specific tag exists in `Tag Registry.md` (e.g., "zomato" for food delivery), **When** a Zomato transaction is imported, **Then** the zomato label is applied in addition to dining + instrument labels
4. **Given** a new tag is added to Tag Registry.md, **When** apply-labels.mjs is re-run, **Then** the label is created in Wallet and applied to all matching historical records

---

## Requirements *(mandatory)*

### Functional Requirements

**Architecture Alignment**:
- All wallet sync code lives in `packs/expense-domain/sources/wallet/`
- All extracted data (records, bills, prices) goes to `~/automation-monorepo-config/data/expense-domain/wallet/` (never in packs/)
- All routing rules, formats, and label definitions are configuration (YAML), not code
- Records created MUST include `source:refactored-code-0905` tag in note

**Part A: Gmail Sync (Engine-First Extraction)**

- **FR-A-001**: System MUST fetch Gmail threads via Gmail API with query: `from:(bank.in) -label:claude-read after:{cursor_timestamp}` (use `last-sync.json` → `last_email_timestamp` as cursor, default 7 days ago if null). For each thread, extract: `sender`, `subject`, `date`, `body` (base64 decoded from Gmail payload)
- **FR-A-002**: System MUST implement deterministic extraction engine (`extract-engine.py`): apply regex patterns from `formats/email.<bank>.yaml` to email body (envelope) before attempting AI parsing. Engine input: `{"sender": "...", "subject": "...", "date": "...", "body": "..."}`. Engine output: `{"action": "skip|extract|unmatched|error", "record": {...merchant, amount, date, currency, card_last4, bank, instrument, wallet_account_id...}}`
- **FR-A-003**: For `action: unmatched` results, system MUST AI-parse merchant/amount/date/currency/card_last4 from email body, THEN codify the pattern (add format block to `formats/email.<bank>.yaml`, save test email sample to `formats/tests/email.<bank>/<sender>-sample.txt`, extend `routing.yaml` if new bank, run `extract-engine.py --test` for regression, then reprocess the same batch with new format — all in one run)
- **FR-A-004**: System MUST determine transaction category: fuzzy-match merchant name against `CATEGORY_HINTS` hardcoded mapping (e.g., "blinkit" → "groceries", "zomato" → "restaurants", "amazon" → "shopping"). Then lookup the category slug in Wallet's category list to find the categoryId. If no match, categoryId is null
- **FR-A-005**: System MUST route extracted transactions to Wallet accounts via `routing.yaml` (deterministic mapping: bank/card name → accountId + accountType)
- **FR-A-006**: System MUST support auto-account creation: if a bank/card has no routing entry and no matching manual account exists, create a new Wallet account (name, accountType, currency inferred from email), add route to `routing.yaml`, track in `last-sync.json` → `auto_created_accounts` (max 10 per user before requiring approval)
- **FR-A-007**: System MUST implement dual-layer deduplication:
  - Layer 1 (idempotency): every record note MUST end with `gm:<gmail-message-id>`. Before write, check existing records for this message ID; skip if found
  - Layer 2 (fuzzy): also skip if a record with same `recordDate` + same `amount` + similar `counterParty` exists but lacks `gm:` tag (log as "skipped: manual duplicate")
- **FR-A-008**: System MUST write records with `note` format: `<merchant> | via <instrument> | <detail> gm:<message-id> | source:refactored-code-0905` (max 255 chars; truncate merchant if needed, preserve gm: tag)
- **FR-A-009**: System MUST update `last-sync.json` atomically: advance cursor, log status, count pushed, on success only
- **FR-A-010**: System MUST support rate-limit retry: on HTTP 429, exponential backoff (2s, 4s, 8s) up to 3 retries before failing

**Part B: Drive Bills Sync**

- **FR-B-001**: System MUST fetch files from Drive "Bills Inbox" folder (folderId in config); track processed files in `last-sync.json` → `processed_drive_files` (idempotent)
- **FR-B-002**: System MUST extract vendor/date/line items/total from PDFs: engine-first for text PDFs (pdftotext/pdfplumber), AI-OCR for scanned images
- **FR-B-003**: System MUST match bills to Wallet records: `wallet_list_records` filtered to date ±3 days, amount match (allow currency conversion via fx_rate)
- **FR-B-004**: On match: patch record with `drive:<fileId>` appended to note + apply bill's category/counterParty; on no match: create new record
- **FR-B-005**: System MUST create bill note: `Expenses/<year>/Bills/bill-<YYMMDD>-<vendor-slug>.md` with YAML frontmatter (bill_id, date, vendor, category, currency, amount_inr, drive_file_id, wallet_record_id) + itemized table + wallet/Obsidian cross-links
- **FR-B-006**: System MUST append product prices: for each line item, log to `product-prices.jsonl` in same folder as bill note (schema: date, product, vendor, qty, unit_price, currency, category, bill_id, bill_note)
- **FR-B-007**: System MUST support fuzzy reconciliation: if split transactions (1500 INR bill vs 1000 + 500 INR records), flag for manual review instead of forcing merge

**Part C: Cross-Source Reconciliation**

- **FR-C-001**: After Parts A/B complete, system MUST scan records created/patched in this run for matches against other sources (manual Wallet entries, records from alternate sync path)
- **FR-C-002**: Match criteria: same `recordDate` ±3 days + same or converted `amount` + similar `counterParty` (case-insensitive substring)
- **FR-C-003**: On match, system MUST merge (richest-first): Drive detail (line items, vendor, category) > Gmail detail (merchant, card, txn ref) > manual detail
- **FR-C-004**: System MUST APPEND source tags to `note` (never replace): preserve all information
- **FR-C-005**: System MUST NOT merge if conflict is irreconcilable (e.g., significantly different amounts); flag instead

**Part D: Label Tagging**

- **FR-D-001**: System MUST maintain `labels-cache.json` (slug → labelId mapping); populate on first run via `wallet_list_labels` + create missing labels from Tag Registry
- **FR-D-002**: System MUST read Tag Registry (`Tag Registry.md` in Obsidian vault) and load desired labels (category, instrument, vendor tags)
- **FR-D-003**: When creating records, system MUST apply 2–4 relevant labels using hardcoded keyword matching:
  - **Merchant matching**: Compare counterParty/merchant against keywords (e.g., "blinkit" → "blinkit" label slug, "zomato" → "food-delivery" slug, "amazon" → "amazon" slug)
  - **Bank/Card matching**: Compare account bank/card against keywords (e.g., "hdfc" → "hdfc" slug, "canara" → "canara" slug, "icici" → "icici" slug)
  - **Label lookup**: Use `labels-cache.json` to map slugs to UUID labelIds
  - **Return**: Max 4 unique label UUIDs per transaction
- **FR-D-004**: System MUST batch label patches (max 20 records per request) to respect 300 req/hr rate limit
- **FR-D-005**: System MUST support `apply-labels.py`: one-shot script to refresh label cache + re-tag historical records (dry-run support for safety)
- **FR-D-006**: System MUST maintain keyword list in code (CATEGORY_HINTS dict in sync.py) mapping merchant keywords to category slugs; these slugs are looked up in labels-cache.json to find the actual labelId

**Part E: Obsidian Write-Back (Audit Trail)**

- **FR-E-001**: For every record created in Parts A/B, system MUST write/update row in Obsidian monthly expense log: `Expenses/<year>/<YYYY-MM Month>.md`
- **FR-E-002**: Row format: `| <date> | <merchant> | <category> | <amount> | <instrument> | gm:<id> |` (or with drive: ref if bills apply)
- **FR-E-003**: Row MUST be idempotent: keyed by gm:/drive: ref; re-runs skip if row already exists
- **FR-E-004**: Create monthly log from template if missing; support Obsidian vault path from config

**Configuration Over Code**

- **FR-F-001**: Account routing: `routing.yaml` (bank/card name → accountId + accountType)
- **FR-F-002**: Email formats: `formats/email.<bank>.yaml` (regex patterns for known senders)
- **FR-F-003**: Tag registry: `Tag Registry.md` in Obsidian (slug → label name + description + rules for auto-apply)
- **FR-F-004**: Sync state: `last-sync.json` (cursor, processed files, auto-created accounts, status)
- **FR-F-005**: All paths configurable via `CONFIG_PATH` and domain config

**Framework Integration**

- **FR-G-001**: Single invocation: `CONFIG_PATH=~/automation-monorepo-config wallet-sync` (or `auto orchestrate wallet-sync`)
- **FR-G-002**: Auto-discover credentials from `$CONFIG_PATH/config/wallet/config.yaml` (Wallet API token, Gmail MCP credentials, Drive folder IDs)
- **FR-G-003**: No file path references in command line; all paths resolved from config
- **FR-G-004**: Standard output format: `🚀 Running wallet-sync, Config: <CONFIG_PATH>, ... ✅ COMPLETE`

### Key Entities

- **Transaction Record**: Wallet API record with id, accountId, amount, currency, recordDate, counterParty, categoryId, labelIds, note (includes gm:/drive: tags)
- **Email Message**: Gmail thread with sender, subject, date, body; source for Part A
- **Bill**: Drive file (PDF/image) with vendor, date, line items, total; source for Part B
- **Bill Note**: Obsidian markdown file with YAML frontmatter, itemized table, cross-links
- **Label**: BudgetBakers label (uuid, name); applied to records per Tag Registry
- **Account Routing**: Mapping entry (bank/card identifier → accountId, accountType)
- **Sync Cursor**: State in last-sync.json tracking last processed email timestamp, processed Drive files, auto-created accounts, status

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All new Gmail bank/card alerts are synced to Wallet within 60 minutes with zero duplicates (idempotency key prevents re-runs)
- **SC-002**: Drive bill uploads are matched to Wallet records within 60 minutes; matched records are enriched with bill reference and category; unmatched records are created
- **SC-003**: Every Wallet record created by sync includes `source:refactored-code-0905` tag in note
- **SC-004**: Every Wallet record has 2–4 relevant labels applied (instrument + category + optional vendor)
- **SC-005**: Obsidian monthly expense logs are kept in sync: every new Wallet record has a corresponding row in the expense log within 5 minutes
- **SC-006**: Product-prices.jsonl is populated for all bills with line items; supports price trend analysis
- **SC-007**: Cross-source merge preserves all information: no data loss when combining Gmail + Drive + manual records
- **SC-008**: Zero customer-facing errors when running hourly; all failures logged to `last-sync.json` → `last_run_status`
- **SC-009**: Sync completes in <30 seconds for typical load (50–100 new transactions/hour)
- **SC-010**: Auto-created accounts are capped at 10; further creations require user approval

## Assumptions

- **Single Wallet User**: Configuration targets a single user (sumitasok@gmail.com); multi-user support is out of scope (future work: per-user config trees)
- **Known Email Formats**: Bank alert emails follow deterministic patterns (same sender, similar subject/body structure); these are codified in `formats/email.<bank>.yaml`
- **Manual Accounts Exist**: Critical accounts (HDFC CC x3690, Canara CC x6102, etc.) are pre-created in Wallet; no full auto-account creation needed for main accounts
- **Obsidian Vault Available**: Vault path is configured; sync can read/write monthly logs and bill notes
- **No External FX Service**: Currency conversions use Wallet API's built-in support; no real-time FX lookups
- **Rate Limit Tolerance**: Wallet API rate limit of 300 req/hr is sufficient for <100 new transactions/hour
- **Stateful Cursor**: `last-sync.json` persists across runs; cursor advances only on success (idempotent, safe for retries)

