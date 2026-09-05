# Computation Notes: Phase 4 - Framework Job Scheduling

**Date**: 2026-09-05  
**Feature**: 008-restructure-architecture  
**Phase**: 4 (Weeks 6-7)  
**Scope**: Integrate framework JobScheduler with expense-domain, implement job management API, validate all 5 jobs execute successfully with retry logic and execution tracking

## Objective

Replace external cron/launchd job scheduling with self-contained framework JobScheduler that:
- Manages job registration, scheduling, and execution
- Implements retry logic with exponential backoff
- Tracks execution history and status
- Exposes REST API for manual job triggering and history retrieval
- Supports configuration-driven job definitions (YAML)

## Approach

1. Create ExpenseDomainJobManager wrapping framework JobScheduler
2. Register all 5 domain jobs with proper schedules (daily, hourly, 30s, 5min intervals)
3. Implement job handler lifecycle (onStart → execute → onSuccess/onFailure → onComplete)
4. Add 3 REST API endpoints for job management
5. Create comprehensive job execution tests
6. Validate all jobs execute successfully with proper error handling

## Inputs

- Phase 1 (external config structure)
- Phase 2 (DomainEngine base, ConfigLoader, RulesLoader)
- Phase 3 (ExpenseEngine API implementation, REST server)
- Phase 3.5 (job definitions in YAML)
- Framework JobScheduler class (scheduler.js, execution-engine.js, manifest-schema.js)
- Requirement: All 5 jobs must be executable through framework
- Requirement: Job execution must support manual triggering and history tracking
- Requirement: Framework must handle retries and timeout

## Steps & Findings

### 1. Job Manager Integration (T031-T032)

Created `ExpenseDomainJobManager` class that:
- Wraps `JobScheduler` from framework
- Exposes `registerJobs()` to register all 5 domain jobs
- Implements `triggerJob(jobId)` for manual execution
- Provides `getExecutionHistory(filters)` for history queries
- Manages scheduler lifecycle (start, stop)

**Finding**: Domain job managers can be implemented consistently across all domains by wrapping JobScheduler, enabling code reuse pattern.

### 2. Job Registration (T033-T034)

Registered 5 domain jobs with proper configuration:
- **Gmail Fetch Job** (ID: gmail-fetch-job)
  - Schedule: 1d (daily)
  - Timeout: 300s (5 minutes)
  - Retry: max 3 attempts, 2x backoff
- **Wallet Fetch Job** (ID: wallet-fetch-job)
  - Schedule: 1h (hourly)
  - Timeout: 300s
  - Retry: max 3 attempts, 2x backoff
- **Bank CSV Monitor Job** (ID: bank-csv-monitor-job)
  - Schedule: 30s (every 30 seconds)
  - Timeout: 60s (1 minute)
  - Retry: max 2 attempts, 2x backoff
- **Process Transactions Job** (ID: process-transactions-job)
  - Schedule: 5m (every 5 minutes)
  - Timeout: 600s (10 minutes)
  - Retry: max 3 attempts, 2x backoff
- **Learn Rules Job** (ID: learn-rules-job)
  - Schedule: 1d (daily)
  - Timeout: 1800s (30 minutes)
  - Retry: max 2 attempts, 2x backoff

**Finding**: Configuration-driven job registration enables adding/modifying jobs without code changes (Principle V).

### 3. Job Executor Implementation (T035)

Implemented 5 job executor methods with handler lifecycle:

```
Execution Flow:
  onStart() → Log job started, emit 'job:started' event
  execute() → Run job logic, return result
  onSuccess() → Log success, emit 'job:succeeded' event
  onFailure() → Log error, emit 'job:failed' event
  onComplete() → Log completion, emit 'job:completed' event
```

Each executor simulates its operation:
- **_executeGmailFetch**: Simulates fetching 1 email
- **_executeWalletFetch**: Simulates fetching 1 transaction
- **_executeBankCsvMonitor**: Simulates checking for CSV files
- **_executeProcessTransactions**: Calls engine.process(), returns processed count
- **_executeLearnRules**: Calls engine.learnRules(), returns learned rules

**Finding**: Handler lifecycle provides observability hooks for monitoring, logging, and event-driven integrations.

### 4. API Integration (T036)

Added 3 REST API endpoints to expense-domain/engine/server.js:
- `GET /api/expense-domain/jobs` → List all registered jobs with metadata
- `POST /api/expense-domain/jobs/{jobId}/trigger` → Manually trigger a job, return executionId
- `GET /api/expense-domain/jobs/{jobId}/history` → Get execution history for a job

**Finding**: Exposing job management via REST API enables UI and external systems to manage jobs without direct framework access (API-first design, Principle VI).

### 5. Framework Enhancement (T037)

Enhanced framework JobScheduler._parseInterval to support day intervals:
- Before: `_parseInterval(/^(\d+)([smh])$/)`
- After: `_parseInterval(/^(\d+)([smhd])$/)`
- Added multiplier: `d: 24 * 60 * 60 * 1000`

**Finding**: Extending framework to support additional interval formats is straightforward and doesn't break existing functionality.

### 6. Path Resolution Fix (T038)

Fixed relative path issues in:
- `packs/expense-domain/engine/api.js`
- `packs/expense-domain/engine/job-integration.js`

Changed from: `require('../../../shared/lib/...')`  
Changed to: `require('../../shared/lib/...')`

**Finding**: Relative paths depend on file location in hierarchy; packs/ sibling structure requires 2 levels up, not 3.

### 7. Job Execution Testing (T038-T039)

Created comprehensive test coverage:
- **job-execution.test.js**: Jest test suite with 20 test cases
- **test-job-execution.js**: Manual integration test script

Test categories:
- Job Registration: Verify all 5 jobs registered with correct names, schedules, timeouts
- Job Execution: Trigger each job, verify successful completion
- Job Failure Handling: Track attempts, errors, retry configuration
- Execution Scheduling: Track execution history, emit events
- Job API Endpoints: Test REST endpoints for jobs listing and triggering
- Job Scheduler State: Verify scheduler running state and execution tracking

**Finding**: Manual integration test script avoids Jest hanging issues with setInterval and provides clearer output for debugging.

### 8. Integration Validation

Manual test results (test-job-execution.js):
```
✅ Test 1: Job Registration (5/5 jobs registered)
✅ Test 2: Job Execution (5/5 jobs executed successfully)
✅ Test 3: Execution History (tracked correctly)
✅ Test 4: Retry Configuration (max retries: 3, multiplier: 2)
✅ Test 5: Schedule Configuration (1d, 1h, 30s intervals parsed)
```

All 5 jobs executed successfully with proper:
- Execution tracking (ID, status, duration)
- Event emission (job:started, job:succeeded, job:completed)
- Handler lifecycle (onStart → execute → onSuccess → onComplete)
- Retry configuration (maxRetries: 3, backoffMultiplier: 2)

## Results

✅ **Job Manager**: ExpenseDomainJobManager fully functional, wrapping JobScheduler  
✅ **Job Registration**: All 5 domain jobs registered with proper configuration  
✅ **Job Execution**: All 5 jobs execute successfully with handler lifecycle  
✅ **API Integration**: 3 job management endpoints added to REST server  
✅ **Framework Enhancement**: _parseInterval supports 'd' for day intervals  
✅ **Path Resolution**: Fixed relative paths in api.js and job-integration.js  
✅ **Testing**: Manual integration test validates all 5 jobs and retry configuration  

## Interpretation

Framework-managed job scheduling replaces external dependencies with self-contained execution:

**Architecture Pattern**:
- Domain declares jobs in YAML (manifest.yaml)
- Domain manager (ExpenseDomainJobManager) registers jobs with framework
- Framework JobScheduler manages execution, retries, and tracking
- REST API exposes job operations to UI and external systems
- Events enable monitoring and observability

**Configuration-Driven Operation**:
- Job schedules specified in job definitions (1d, 1h, 30s)
- Timeout and retry configuration in job registration
- No hardcoded schedule times (all via configuration)
- New jobs added by registering with scheduler (no framework code changes)

**Observability & Control**:
- Manual job triggering via REST API (`POST /jobs/{id}/trigger`)
- Execution history tracking (`GET /jobs/{id}/history`)
- Handler lifecycle events (job:started, job:succeeded, job:failed, job:completed)
- Metrics tracking (attempts, duration, status)

**Local-First Design**:
- No external cron/launchd dependencies
- All scheduling managed by framework
- Job state in memory (production: persistence layer)
- Self-contained, reproducible execution

## Caveats

1. **Simulated Executors**: Job executors are simulated (hardcoded results). Production will call:
   - Gmail API via Gmail adapter
   - Wallet API via Wallet adapter
   - File system via CSV monitor
   - Engine methods (process, learnRules) for real processing

2. **No Job Persistence**: Job execution state not persisted across restarts. Phase 5+ will add:
   - Job queue with persistent storage (PostgreSQL/DynamoDB)
   - In-flight job recovery on startup
   - Execution archive for auditing

3. **No Concurrent Execution Control**: Multiple instances of same job can execute concurrently. Phase 5 will add:
   - Distributed locking (Redis, DynamoDB)
   - Job concurrency limits
   - Rate limiting per job

4. **Framework Dependency**: Job scheduling fully depends on framework JobScheduler. Phase 5 will validate:
   - Scheduler behavior under load
   - Timeout handling edge cases
   - Error propagation from executors

## Code Quality

- ✅ Consistent with DomainEngine API contract
- ✅ Handler lifecycle enables observability
- ✅ Configuration injection (no hardcoded paths)
- ✅ Event-driven design (job lifecycle events)
- ✅ Retry logic with exponential backoff
- ✅ Comprehensive error tracking
- ✅ History tracking and query filtering

## Integration Points

- **Phase 1**: External config structure used ✅
- **Phase 2**: DomainEngine base class inherited, RulesEngine used ✅
- **Phase 3**: ExpenseEngine API enhanced with job management ✅
- **Phase 4**: Framework job scheduling integrated ✅
- **Phase 5**: Config consolidation will add job persistence
- **Phase 6**: Rule learning jobs will be executed by scheduler
- **Phase 7**: Domain UIs will call job management endpoints
- **Phase 8**: E2E tests will validate job workflows

## Dependencies

- Framework JobScheduler class (packs/shared/jobs/scheduler.js)
- Framework ExecutionEngine (packs/shared/jobs/execution-engine.js)
- ExpenseEngine (packs/expense-domain/engine/api.js)
- Node.js: EventEmitter, fs, path, http modules
- js-yaml for YAML parsing (configuration)

## Next Steps

**Phase 5 (Config Consolidation)**:
- Persist job execution state (database)
- Add job queue with durability
- Implement distributed locking for singleton jobs
- Add job execution metrics and monitoring

**Phase 6 (Rule Learning)**:
- Execute learn-rules job daily
- Store learned rules in ~/automation-monorepo-config/rules/
- Apply learned rules during processing

**Phase 7 (Domain UIs)**:
- Domain UI calls job management endpoints
- UI shows job execution history and status
- UI enables manual job triggering

**Phase 8 (E2E Testing)**:
- End-to-end test job workflows
- Validate job data flow through domain
- Test write-back to sources from job results
