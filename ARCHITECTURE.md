# Automation Monorepo — Complete Architecture

**Version**: 1.0.0  
**Date**: 2026-09-05  
**Status**: Framework Architecture Specification (Pre-Implementation)

---

## Executive Summary

The automation monorepo is a **self-managing, self-improving multi-domain framework** that:
- Organizes independent problem spaces (domains) with reusable pattern
- Each domain orchestrates data sources through a Domain Engine
- Framework manages job execution, rule learning, and UI aggregation
- Data flows: Sources → Source Jobs → Domain Engine → Domain Rules → Domain UI → Framework Aggregation UI
- Self-improving: AI learns rules from data patterns, stores as YAML, applies without code changes

---

## Directory Structure

### Root-Level Organization

```
~/automation-monorepo/
├── packs/                          # Framework and domains (git-tracked, read-only)
│   ├── shared/                     # Framework utilities (untouched during migration)
│   │   ├── auth/                   # Shared authentication
│   │   ├── jobs/                   # Job scheduling engine
│   │   └── lib/                    # Shared utilities
│   ├── expense-domain/             # Example domain (restructured)
│   ├── stock-domain/               # Future domain (same pattern)
│   └── trip-domain/                # Future domain (same pattern)
│
├── .specify/                       # Feature spec infrastructure (gitignored)
├── docs/                           # Documentation (git-tracked)
├── schedules/                      # Job definitions (git-tracked)
└── .gitignore                      # Updated to include ~/automation-monorepo-* dirs
```

### External Configuration Directories (Outside Repository)

```
~/automation-monorepo-config/       # UNIFIED CONFIG ROOT (external to repo)
├── data/                           # Domain and source outputs
│   ├── expense-domain/
│   │   ├── gmail/                  # Gmail adapter outputs
│   │   ├── wallet/                 # Wallet adapter outputs
│   │   ├── sms/                    # SMS adapter outputs
│   │   └── engine/                 # Domain engine outputs (transactions, processed data)
│   ├── stock-domain/
│   │   ├── ibkr/
│   │   ├── zerodha/
│   │   └── engine/
│   └── trip-domain/
│       ├── makemytrip/
│       └── engine/
│
├── config/                         # Domain and source configuration
│   ├── expense-domain/
│   │   ├── domain.yaml             # Domain engine config (rules, settings, features)
│   │   ├── gmail.yaml              # Gmail adapter config (API keys, fetch schedule)
│   │   ├── wallet.yaml
│   │   └── sms.yaml
│   ├── stock-domain/
│   │   ├── domain.yaml
│   │   ├── ibkr.yaml
│   │   └── zerodha.yaml
│   └── framework.yaml              # Framework-wide settings (job scheduler, aggregation)
│
└── rules/                          # LEARNED RULES (AI-generated YAML)
    ├── expense-domain/
    │   ├── gmail/
    │   │   ├── email-categorization-rules.yaml
    │   │   └── transaction-extraction-rules.yaml
    │   ├── wallet/
    │   │   └── transaction-normalization-rules.yaml
    │   ├── sms/
    │   │   └── sms-transaction-rules.yaml
    │   └── engine/
    │       ├── categorization-rules.yaml
    │       ├── duplicate-detection-rules.yaml
    │       └── validation-rules.yaml
    ├── stock-domain/
    │   ├── ibkr/
    │   │   └── statement-parsing-rules.yaml
    │   └── zerodha/
    │       └── settlement-rules.yaml
    └── trip-domain/
        ├── makemytrip/
        │   └── booking-extraction-rules.yaml
        └── engine/
            └── expense-assignment-rules.yaml
```

### Domain Structure (Inside packs/)

```
packs/expense-domain/
├── sources/                        # Source adapters (read-only code)
│   ├── gmail/
│   │   ├── index.js                # Gmail adapter (reads emails, extracts transactions)
│   │   ├── config.sample.yaml      # Config template
│   │   └── manifest.yaml           # Declares dependencies, jobs, outputs
│   ├── wallet/
│   │   ├── index.js
│   │   └── manifest.yaml
│   ├── sms/
│   │   ├── index.js
│   │   └── manifest.yaml
│   └── shared-adapters/            # Shared adapter patterns
│
├── engine/                         # Domain Engine (core application logic)
│   ├── index.js                    # Domain engine entry point
│   ├── processor.js                # Processes data from sources
│   ├── rules-engine.js             # Applies learned rules to data
│   ├── api.js                      # Exposes API for UI to read/write domain state
│   ├── manifest.yaml               # Declares what engine exposes, writes back to sources
│   └── config.sample.yaml
│
├── reports/                        # Report generation (read-only code)
│   ├── expense-summary.js
│   ├── category-breakdown.js
│   ├── monthly-trends.js
│   └── manifest.yaml
│
├── ui/                             # Domain-specific UI (read-only code)
│   ├── components/
│   │   ├── expense-list.jsx
│   │   ├── expense-editor.jsx
│   │   ├── source-status.jsx
│   │   └── rule-editor.jsx
│   ├── pages/
│   │   ├── dashboard.jsx
│   │   ├── sources.jsx
│   │   └── rules.jsx
│   ├── api-client.js               # Client library for Domain Engine API
│   ├── index.html
│   ├── package.json
│   └── manifest.yaml               # Declares UI outputs, what data it needs
│
├── jobs/                           # Domain and source jobs
│   ├── source-jobs/
│   │   ├── gmail-fetch-job.yaml    # Fetch emails daily
│   │   ├── gmail-monitor-job.yaml  # Monitor for new emails
│   │   ├── wallet-fetch-job.yaml   # Fetch transactions hourly
│   │   └── bank-csv-monitor-job.yaml # Monitor for uploaded CSVs
│   ├── domain-jobs/
│   │   ├── process-transactions-job.yaml      # Process fetched data through engine
│   │   ├── learn-rules-job.yaml               # AI learns rules from data patterns
│   │   ├── reconciliation-job.yaml            # Daily reconciliation
│   │   └── report-generation-job.yaml         # Generate reports
│   └── manifest.yaml
│
├── README.md                       # Domain documentation
├── package.json
└── manifest.yaml                   # Domain manifest (declares structure, APIs, capabilities)
```

---

## Configuration Injection Mechanism

### Overview

The framework uses **parameterized configuration injection** to satisfy Constitution Principle I (Packs Declare, Workspace Supplies). All runtime configuration lives external to the repository in `~/automation-monorepo-config/`, injected to the framework at startup via a command-line parameter.

### Startup

Framework is invoked with:
```bash
node packs/framework/index.js --config-path ~/automation-monorepo-config
```

Or via environment variable:
```bash
CONFIG_PATH=~/automation-monorepo-config node packs/framework/index.js
```

### Framework Initialization

1. **Framework starts** with `--config-path` parameter
2. **Framework loads** `{config_path}/config/framework.yaml`
3. **Framework discovers domains** from `framework.yaml` domains list
4. **Framework injects** `{config_path}` to all domain engines
5. **Framework starts** job scheduler with injected config path
6. **Framework loads** UI components and passes injected path

### Domain Configuration

Each domain receives:
```javascript
// In domain engine initialization
const configPath = '~/automation-monorepo-config';

// Domain engine reads:
// - {configPath}/config/{domain}/domain.yaml
// - {configPath}/config/{domain}/{source}.yaml for each source
// - {configPath}/data/{domain}/ for outputs
// - {configPath}/rules/{domain}/ for learned rules
```

### Source Adapter Configuration

Source adapters never hardcode paths. They receive injected config:

```javascript
// In source adapter initialization
SourceAdapter {
  constructor(configPath, domainName, sourceName) {
    this.configPath = configPath;
    this.domainName = domainName;
    this.sourceName = sourceName;
    
    // Read adapter-specific config
    this.config = loadYAML(
      `${configPath}/config/${domainName}/${sourceName}.yaml`
    );
    
    // Read/write to domain-specific data directory
    this.dataDir = `${configPath}/data/${domainName}/${sourceName}/`;
    this.rulesDir = `${configPath}/rules/${domainName}/${sourceName}/`;
  }
}
```

### Domain Engine Configuration

Domain engine receives injected path and loads domain-wide config:

```javascript
// In domain engine initialization
DomainEngine {
  constructor(configPath, domainName) {
    this.configPath = configPath;
    this.domainName = domainName;
    
    // Load domain config
    this.domainConfig = loadYAML(
      `${configPath}/config/${domainName}/domain.yaml`
    );
    
    // Initialize source adapters with injected path
    this.sources = [];
    for (const source of this.domainConfig.sources) {
      this.sources.push(
        new SourceAdapter(configPath, domainName, source.name)
      );
    }
    
    // Initialize rules engine with learned rules location
    this.rulesEngine = new RulesEngine(
      `${configPath}/rules/${domainName}/engine/`
    );
  }
}
```

### Key Guarantees

1. **Packs are read-only**: No data/config stored in `packs/`
2. **Configuration is parameterized**: Never hardcoded in code
3. **External is canonical**: `~/automation-monorepo-config/` is Single Source of Truth
4. **Domain isolation**: Each domain sees only its subtree
5. **No path discovery**: Domains don't resolve `~` or search for directories

### Extension: New Domains

To add a new domain:

1. Create new directory in `packs/new-domain/`
2. Follow same structure (sources/, engine/, ui/, jobs/, reports/)
3. Add domain entry to `~/automation-monorepo-config/config/framework.yaml`
4. Create `~/automation-monorepo-config/config/new-domain/domain.yaml`
5. Create `~/automation-monorepo-config/config/new-domain/{source}.yaml` for each source
6. Framework auto-discovers and injects config at startup

---

## Architecture Patterns

### 1. Source Adapter Pattern

**Purpose**: Fetch data from external systems and feed Domain Engine

```
Source Adapter (e.g., gmail-adapter)
  ├─ Read: Fetch data from Gmail API
  ├─ Transform: Extract transactions from emails
  ├─ Write-back: Send categorization confirmations back to Gmail labels
  └─ API: Exposes raw/processed data to Domain Engine

Jobs managing adapter:
  ├─ fetch-job: Runs daily, fetches new emails
  ├─ monitor-job: Watches for new data in real-time
  └─ extract-job: Processes fetched data
```

**Config location**: `~/automation-monorepo-config/config/expense-domain/gmail.yaml`  
**Output location**: `~/automation-monorepo-config/data/expense-domain/gmail/`  
**Learned rules**: `~/automation-monorepo-config/rules/expense-domain/gmail/`

### 2. Domain Engine Pattern

**Purpose**: Orchestrate sources, apply rules, expose API for UI

```
Domain Engine (e.g., expense-processor)
  ├─ Read: Processes data from all source adapters
  ├─ Rules: Applies learned rules from ~/automation-monorepo-config/rules/
  ├─ Write: Produces normalized domain data
  ├─ Write-back: Can update sources (e.g., wallet categorizations)
  └─ API: Exposes REST/GraphQL endpoints for UI

Processing flow:
  1. Sources feed raw data → domain engine
  2. Engine applies learned rules
  3. Engine produces canonical domain objects (Transactions, Accounts, Rules)
  4. Engine stores to ~/automation-monorepo-config/data/expense-domain/engine/
  5. UI reads from engine API
  6. UI writes updates back to engine API
  7. Engine can write-back to sources
```

**Config location**: `~/automation-monorepo-config/config/expense-domain/domain.yaml`  
**Output location**: `~/automation-monorepo-config/data/expense-domain/engine/`  
**API contract**: Defined in `packs/expense-domain/engine/manifest.yaml`

### 3. Domain UI Pattern

**Purpose**: Visualize domain data, accept user input, trigger source jobs

```
Domain UI (e.g., expense-dashboard)
  ├─ Read: Fetches data from Domain Engine API
  ├─ Read: Fetches source status/data from Source Adapter APIs
  ├─ Write: Updates domain data through Engine API
  ├─ Write: Triggers source jobs (upload CSV, sync wallet, etc.)
  ├─ Display: Visualizes domain state, source status, rules
  └─ Interact: Upload files, edit transactions, manage rules

UI endpoints:
  - GET /api/expenses (from Domain Engine)
  - GET /api/sources/gmail/status (from Gmail Adapter)
  - POST /api/expenses/{id} (update transaction)
  - POST /api/jobs/bank-csv-monitor/trigger (upload CSV)
  - GET /api/rules (learned rules)
  - POST /api/rules (create new rule)
```

**UI location**: `packs/expense-domain/ui/`  
**API client**: `packs/expense-domain/ui/api-client.js`  
**Manifest**: `packs/expense-domain/ui/manifest.yaml` (declares what UI needs)

### 4. Framework Aggregation UI Pattern

**Purpose**: Show all domains in one unified interface

```
Framework Aggregation UI
  ├─ Discovers available domains from framework config
  ├─ Loads each domain's UI as embedded component/iframe
  ├─ Aggregates domain data for dashboard view
  ├─ Provides navigation between domains
  ├─ Shows framework-level metrics (total jobs, rules, data sources)
  └─ Routes domain-specific interactions to domain UIs

Structure:
  framework-ui/
    ├─ index.html                  # Main aggregation page
    ├─ dashboard.jsx               # Framework dashboard
    ├─ domain-loader.js            # Dynamically loads domain UIs
    ├─ api-client.js               # Framework API client
    └─ routes/
        ├─ /dashboard              # Framework overview
        ├─ /domains/{domain}/       # Route to domain UI
        ├─ /jobs                    # Framework job scheduler view
        └─ /rules                   # Global rules view
```

**Location**: `packs/shared/framework-ui/` or separate `packs/framework/ui/`

---

## Data Flow

### Complete Request-Response Cycle

```
USER INTERACTION (Domain UI)
  ↓
[1] User uploads bank CSV to expense-domain
  ↓
[2] UI triggers: POST /api/jobs/bank-csv-monitor/trigger
  ↓
[3] Framework Job Scheduler receives trigger
  ↓
[4] Framework launches source job (bank-csv-monitor-job)
  ↓
[5] Source Adapter (bank-adapter) runs:
    - Reads CSV from upload directory
    - Parses transactions
    - Applies learned rules from ~/automation-monorepo-config/rules/expense-domain/bank/
    - Outputs to ~/automation-monorepo-config/data/expense-domain/bank/
  ↓
[6] Domain Engine job (process-transactions-job) runs:
    - Reads from all source outputs
    - Applies domain rules from ~/automation-monorepo-config/rules/expense-domain/engine/
    - Normalizes, categorizes, validates
    - Outputs to ~/automation-monorepo-config/data/expense-domain/engine/
    - Triggers AI learning job if new patterns detected
  ↓
[7] AI Learning job (learn-rules-job) runs:
    - Analyzes transaction patterns
    - Generates new categorization rules
    - Writes to ~/automation-monorepo-config/rules/expense-domain/engine/
    - Updates rules YAML for next run
  ↓
[8] Domain UI refreshes:
    - Fetches updated transactions from GET /api/expenses
    - Displays new transactions with learned categories
    - Shows new rules in rule editor
  ↓
[9] User sees results and can edit/approve
```

### Write-Back Flow

```
USER EDITS DOMAIN DATA (Domain UI)
  ↓
[1] User updates transaction: PATCH /api/expenses/{id}
  ↓
[2] Domain Engine API receives update
  ↓
[3] Engine determines which sources need write-back
    Example: Updated category → write label to Gmail
  ↓
[4] Engine calls source adapter write-back methods:
    gmail-adapter.writeBack({messageId, category, label})
  ↓
[5] Source Adapter writes to external system:
    Gmail API: Add/remove labels from message
  ↓
[6] Write-back confirmation stored in logs
  ↓
[7] UI reflects successful write-back to user
```

---

## Job Lifecycle

### Framework-Managed Jobs

**All jobs scheduled and executed by framework, not external cron/launchd**

```
Job Types:
  1. Source Jobs (per source adapter)
     - fetch-job: Periodic fetch (e.g., daily email check)
     - monitor-job: Continuous/event-driven (e.g., file upload detection)
     - extract-job: Process fetched data
     - write-back-job: Push updates back to source

  2. Domain Jobs (per domain engine)
     - process-transactions-job: Transform source data
     - learn-rules-job: AI discovers rules from patterns
     - reconciliation-job: Validate data integrity
     - report-generation-job: Create reports

  3. Framework Jobs
     - health-check-job: Monitor domain status
     - cleanup-job: Archive old data
     - aggregation-job: Compile framework-wide metrics

Job Definition (YAML):
  name: gmail-fetch-job
  schedule: "0 2 * * *"             # Daily at 2 AM (cron format)
  timeout: 3600                      # Seconds
  retries: 3
  backoff: exponential
  domain: expense-domain
  source: gmail
  handlers:
    onSuccess: notify-slack
    onFailure: notify-slack, alert-operator
```

**Job Scheduling**: Framework reads all job definitions from `packs/*/jobs/` and manages execution in-memory

---

## API Contracts

### Domain Engine API (for Domain UI)

```
GET /api/{domain}/expenses
  → Returns all transactions
  
GET /api/{domain}/expenses/{id}
  → Returns single transaction with full detail
  
PATCH /api/{domain}/expenses/{id}
  → Update transaction (category, notes, etc.)
  → Triggers write-back to sources
  
POST /api/{domain}/expenses
  → Create new transaction (manual entry)
  
DELETE /api/{domain}/expenses/{id}
  → Delete transaction
  
GET /api/{domain}/rules
  → Returns all applicable rules (learned + configured)
  
POST /api/{domain}/rules
  → Create new rule (framework validates for conflicts)
  
GET /api/{domain}/sources/{source}/status
  → Returns source adapter health and last fetch time
  
POST /api/{domain}/jobs/{jobName}/trigger
  → Manually trigger a job (e.g., fetch now, upload CSV)
  
GET /api/{domain}/jobs
  → Returns job execution history and status
```

### Source Adapter API (for Domain Engine)

```
read(filters)
  → Returns data from source (e.g., fetch emails from Gmail)
  
process(data, rules)
  → Transform source data using learned rules
  
writeBack(updates)
  → Write confirmations/categories back to source
  
getStatus()
  → Health and last-fetch-time
```

### Framework Aggregation API

```
GET /api/framework/domains
  → List all available domains with UIs
  
GET /api/framework/dashboard
  → Aggregated metrics (total transactions, rules, sources, jobs)
  
GET /api/framework/jobs
  → All jobs across all domains
  
POST /api/framework/rules/validate
  → Validate rule conflicts across domains
  
GET /api/framework/health
  → Framework and domain health status
```

---

## Configuration Inheritance

**Convention over Configuration principle**: Sensible defaults, minimal explicit config

```
Hierarchy (first match wins):
  1. Domain config: ~/automation-monorepo-config/config/{domain}/domain.yaml
  2. Source config: ~/automation-monorepo-config/config/{domain}/{source}.yaml
  3. Framework defaults: ~/automation-monorepo-config/config/framework.yaml
  4. Built-in defaults: packs/shared/defaults.yaml

Example:
  Fetch schedule for gmail:
    1. Check: config/expense-domain/gmail.yaml → schedule: "0 2 * * *"?
    2. If not, check: config/expense-domain/domain.yaml → sources.gmail.schedule?
    3. If not, check: config/framework.yaml → source-fetch-default-schedule?
    4. If not, use: built-in "0 2 * * *" (daily 2 AM)
```

---

## Rule Learning & Improvement

### AI-Driven Rule Discovery

```
Step 1: Detect Pattern
  - Transaction analyzer finds 10+ emails matching pattern: "invoice.*\$[0-9]+.from:vendor"
  - Pattern has >95% accuracy predicting "Office Supplies" category
  
Step 2: Generate Rule (YAML)
  name: invoice-office-supplies-rule
  type: categorization
  confidence: 0.98
  source: gmail
  pattern:
    subject: "invoice.*"
    body_contains: ["office", "supplies"]
    sender: "vendor@company.com"
  action: categorize_as: "Office Supplies"
  
Step 3: Store Rule
  Location: ~/automation-monorepo-config/rules/expense-domain/gmail/generated-rules.yaml
  Version: 2024-09-05T10:30:00Z
  Origin: ai-learning-job

Step 4: Apply in Future Runs
  - Next transaction matching pattern automatically categorized
  - No code change needed, only YAML updated
  - Framework applies rule across all jobs

Step 5: Handle Conflicts
  If new rule conflicts with existing rule:
    - Check: rule.confidence vs existing.confidence
    - Use: Higher confidence rule
    - Log: Both rules and reason for selection
    - Alert: User if confidence < threshold for manual review
```

---

## Convention Over Configuration Examples

```
Conventions (sensible defaults, no config needed):

1. Directory Structure
   packs/{domain}/{component}/
   → Automatically discovered, loaded, and wired

2. Job Scheduling
   Jobs run daily at 2 AM by default
   Config if different: schedule: "0 3 * * *"

3. API Endpoints
   Domain at: GET /api/{domain}/{resource}
   Source at: GET /api/{domain}/sources/{source}/status
   No config mapping needed

4. Write-back
   Default: Enabled (if source supports it)
   Config if disabled: write_back_enabled: false

5. Rule Learning
   Default: Enabled with >95% confidence threshold
   Config if different: ai_learning.confidence_threshold: 0.90

6. Data Retention
   Default: Keep all data
   Config if different: retention_days: 180
```

---

## Security & Isolation

- **Framework**: Manages all job execution, no external dependencies
- **Domains**: Isolated data directories, independent config
- **Sources**: Credentials in config files (~/automation-monorepo-config/config/), not in code
- **Write-back**: Explicit opt-in per source, logged for audit
- **Rules**: Version-controlled in YAML, no dynamic code execution
- **UI**: Domain UIs accessed through framework aggregation, no direct domain access

---

## Next Steps

1. **Create UI Architecture Specification** — Detail domain UI, aggregation UI, interactions
2. **Create Implementation Plan** — Decompose into tasks (setup, migration, testing, deployment)
3. **Start Migration** — Restructure current packs following this architecture
4. **Build Framework Components** — Job scheduler, rule engine, API layer
5. **Develop UIs** — Domain UIs, then framework aggregation UI
6. **Enable AI Learning** — Integrate AI rule generation
7. **Test End-to-End** — Validate entire data flow with real domains

