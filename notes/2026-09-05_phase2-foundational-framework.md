# Computation Notes: Phase 2 - Foundational Framework Infrastructure

**Date**: 2026-09-05  
**Feature**: 008-restructure-architecture  
**Phase**: 2 (Weeks 2-3)  
**Scope**: Core framework components (job scheduler, Domain Engine base, config/rules loaders, rules engine)

## Objective

Establish framework infrastructure that all domains depend on. Build job scheduler, Domain Engine base class, configuration/rules loaders, and rule application engine that enable autonomous operation without code changes.

## Approach

1. Design job lifecycle: register → schedule → execute → retry → complete
2. Create DomainEngine base class with CRUD and rules API
3. Implement configuration injection via ConfigLoader
4. Implement rule discovery and application via RulesEngine
5. Add metrics, event emission, and error handling throughout

## Inputs

- Phase 1 setup (external config structure)
- Architecture specification (domain engine pattern, source adapter pattern)
- Constitution Principle V (Configuration Over Code)
- Requirement: Framework handles job scheduling (no cron/launchd)
- Requirement: Rules applied without code changes

## Steps & Findings

### 1. Job Scheduler Design

Implemented `JobScheduler` with:
- Job registration (jobId, manifest with schedule/timeout/retry)
- Interval-based scheduling (interval: "1d", "1h", "30s")
- Execution tracking (attempts, duration, status)
- Retry logic (exponential backoff: 5s → 10s → 20s)
- Event emission (job:registered, execution:started, execution:completed, etc.)

**Finding**: Separating scheduler (when) from executor (how) allows independent testing and reuse.

### 2. Execution Engine

Implemented `ExecutionEngine` with:
- Job handler execution with timeout (Promise.race pattern)
- Retry logic with exponential backoff
- Handler lifecycle: onStart → execute → onSuccess/onFailure → onComplete
- Metrics tracking (duration, memory delta, attempt count)
- Error collection (all errors from all attempts)

**Finding**: Handler callbacks enable logging, notifications, and side effects without coupling to engine.

### 3. Job Manifest Schema

Designed job manifest structure:
```yaml
name: gmail-fetch-job
schedule:
  type: interval
  interval: 1d
timeout: 300
retry:
  maxRetries: 3
  backoffMultiplier: 2
handlers:
  execute: async () => { ... }
  onStart: async () => { ... }
  onSuccess: async () => { ... }
  onFailure: async () => { ... }
  onComplete: async () => { ... }
enabled: true
```

**Finding**: Manifest separates job metadata from implementation, enabling configuration-driven operation.

### 4. DomainEngine Base Class

Created base class with:
- CRUD operations: getData/createData/updateData/deleteData
- Rules management: getRules/createRule/updateRule/deleteRule
- Source operations: getSourceStatus/triggerSourceJob/writeBackToSource
- Processing: process() core business logic, learnRules() AI discovery
- Configuration injection: configPath in constructor ensures parameterized operation

**Finding**: Base class defines API contract that all domains must implement, enabling framework integration.

### 5. ConfigLoader

Implemented `ConfigLoader` with:
- Load framework config from framework.yaml
- Load domain config from domain.yaml
- Load source config from {domain}/{source}.yaml
- Load all sources for a domain
- Validation: check config exists, sources defined, domain in framework
- Caching: avoid repeated file I/O

**Finding**: YAML loading and caching allows configs to be treated as data, not code.

### 6. RulesLoader

Implemented `RulesLoader` with:
- Load domain rules from ~/automation-monorepo-config/rules/{domain}/
- Load source-specific rules from ~/automation-monorepo-config/rules/{domain}/{source}/
- Load engine-specific rules from ~/automation-monorepo-config/rules/{domain}/engine/
- Find rules by type (categorization, validation, dedup)
- Filter by confidence threshold
- Support both file-based and discovered rules

**Finding**: Hierarchical rules (engine, source, domain) enable scoped rule application.

### 7. RulesEngine

Implemented `RulesEngine` with:
- Pattern matching: simple (field:value), object (field operators), regex, functions
- Action execution: set/delete/transform on item fields
- Statistics tracking: total applied, by type, by source
- Rule application: load → match → apply in order

**Finding**: Declarative pattern/action pairs enable rule definition without code.

## Results

✅ **Job Lifecycle**: Complete (register → schedule → execute → retry → complete)  
✅ **Scheduler**: Interval-based scheduling with exponential backoff retries  
✅ **Executor**: Timeout, metrics, and error handling operational  
✅ **Domain Engine**: Base class with CRUD, rules, sources, processing APIs  
✅ **Config Loader**: YAML-based configuration with injection pattern  
✅ **Rules Loader**: Hierarchical rule discovery and loading  
✅ **Rules Engine**: Pattern matching and declarative action execution  
✅ **Event Emission**: Observable throughout for monitoring  

## Interpretation

The framework infrastructure establishes a clear separation of concerns:

**Scheduler** (JobScheduler):
- Knows WHEN to execute (schedule, interval)
- Doesn't know HOW (delegates to executor)
- Tracks which jobs and their history

**Executor** (ExecutionEngine):
- Knows HOW to execute (run, timeout, retry)
- Doesn't know WHEN (scheduler controls timing)
- Tracks metrics and errors

**Domain Engine** (DomainEngine):
- Knows WHAT to do (business logic)
- Doesn't know WHEN/HOW (framework handles)
- Implements CRUD and processing APIs

**Configuration** (ConfigLoader):
- Loads settings from ~/automation-monorepo-config/
- Injects to domains as parameters
- Never hardcoded in code

**Rules** (RulesEngine):
- Pattern matches and applies actions
- Executed by domains without modification
- Enables self-improvement (learn → apply → repeat)

## Caveats

1. **Interval-based only**: Cron scheduling planned but not implemented
2. **Simple patterns**: Advanced regex and complex conditions planned
3. **In-memory execution**: No distributed job queue or persistence
4. **No job dependencies**: Jobs run independently; chaining via orchestration layer
5. **Single-threaded**: All jobs execute sequentially (parallelization planned)

## Code Quality

- ✅ Event-driven (EventEmitter throughout)
- ✅ Error handling (all errors caught and tracked)
- ✅ Metrics (execution time, memory, attempts)
- ✅ Testability (callbacks allow mocking handlers)
- ✅ Extensibility (base classes for subclassing)

## Next Phases Enabled

- **Phase 3**: Domains inherit from DomainEngine and use this infrastructure
- **Phase 4**: Job scheduler integrated with source adapters
- **Phase 5**: Config loader used for all domain initialization
- **Phase 6**: Rules engine applies learned rules
- **Phase 7**: UIs trigger jobs and read rule engine output
- **Phase 8**: E2E tests validate scheduler and rules engine

## Dependencies

- Phase 1 (external config structure)
- Node.js EventEmitter (built-in)
- js-yaml (YAML parsing)
