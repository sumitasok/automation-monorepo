# Phase 5: Config Consolidation & Framework Integration

**Duration**: Weeks 8-9 (10 tasks)  
**Goal**: Migrate LaunchD schedules to Phase 4 Framework Scheduler, add job persistence, implement unified job management

## Overview

Phase 5 bridges the gap between LaunchD-based orchestration and framework-managed scheduling by:
1. Persisting job execution state to SQLite
2. Tracking orchestration runs with full history
3. Implementing distributed locking (singleton jobs)
4. Integrating framework scheduler with existing orchestrations
5. Adding Telegram notifications to framework

## Tasks

### T031-T035: Job Persistence Layer

**T031**: Create SQLite schema for job execution state
- Jobs table (id, name, schedule, enabled, created_at, updated_at)
- Executions table (id, job_id, status, started_at, ended_at, attempts, result)
- Orchestrations table (id, name, steps, started_at, ended_at, status)

**T032**: Implement JobStateManager class
- Persist execution records to SQLite
- Query execution history with filters
- Track execution statistics (success rate, avg duration)

**T033**: Add state persistence to JobScheduler
- Hook into execution lifecycle (start, success, failure, complete)
- Atomically write state to database
- Handle concurrent write conflicts

**T034**: Create migration system for job state database
- Initial schema creation
- Version tracking
- Rollback support

**T035**: Implement execution history API endpoints
- GET /api/jobs/{id}/executions (paginated)
- GET /api/jobs/{id}/stats (success rate, avg duration)
- GET /api/orchestrations (list runs)

### T036-T040: Orchestration Integration

**T036**: Create OrchestratorJobManager
- Wrap framework JobScheduler
- Register all orchestrations as schedulable jobs
- Map orchestration steps to job execution

**T037**: Implement distributed locking
- Redis or SQLite-based locking
- Prevent concurrent orchestration runs
- Lock timeout with auto-release

**T038**: Add orchestration history tracking
- Track step execution within orchestrations
- Store results per step
- Enable step-level retry

**T039**: Implement orchestration execution API
- POST /api/orchestrations/{name}/run (manual trigger)
- GET /api/orchestrations/{name}/runs (history)
- PUT /api/orchestrations/{name}/pause (pause execution)

**T040**: Migrate LaunchD jobs to framework
- Remove launchd configuration
- Create equivalent framework jobs
- Verify schedule equivalence

## Implementation Details

### Job Persistence Schema

```sql
CREATE TABLE jobs (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  schedule TEXT,
  timeout INTEGER,
  retry_max_attempts INTEGER,
  retry_backoff_multiplier REAL,
  enabled BOOLEAN,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);

CREATE TABLE executions (
  id TEXT PRIMARY KEY,
  job_id TEXT REFERENCES jobs(id),
  status TEXT,
  started_at TIMESTAMP,
  ended_at TIMESTAMP,
  attempts INTEGER,
  last_error TEXT,
  result TEXT,
  context TEXT
);

CREATE TABLE orchestrations (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  started_at TIMESTAMP,
  ended_at TIMESTAMP,
  status TEXT,
  total_steps INTEGER,
  completed_steps INTEGER
);

CREATE TABLE orchestration_steps (
  id TEXT PRIMARY KEY,
  orchestration_id TEXT REFERENCES orchestrations(id),
  step_index INTEGER,
  job_id TEXT REFERENCES jobs(id),
  status TEXT,
  started_at TIMESTAMP,
  ended_at TIMESTAMP,
  attempts INTEGER,
  result TEXT
);
```

### Integration Points

```
Phase 5 Architecture:
┌──────────────────────────────────────────┐
│ Framework JobScheduler (Phase 4)         │
│ • Register jobs/orchestrations           │
│ • Manage execution lifecycle             │
│ • Emit events (start, success, fail)     │
└──────────────┬───────────────────────────┘
               │
               ↓
┌──────────────────────────────────────────┐
│ JobStateManager (NEW - Phase 5)          │
│ • Persist executions to SQLite           │
│ • Track history & statistics             │
│ • Provide query API                      │
└──────────────┬───────────────────────────┘
               │
               ↓
┌──────────────────────────────────────────┐
│ REST API Layer (Phase 4+5)               │
│ • /api/jobs/{id}/executions              │
│ • /api/orchestrations/{name}/run         │
│ • /api/jobs/{id}/stats                   │
└──────────────────────────────────────────┘
```

### Distributed Locking Strategy

**SQLite-based (Simple)**:
```
CREATE TABLE locks (
  resource_id TEXT PRIMARY KEY,
  holder_id TEXT,
  acquired_at TIMESTAMP,
  expires_at TIMESTAMP
);

-- Acquire lock with timeout
INSERT INTO locks VALUES (?, ?, NOW(), NOW() + 3600)
WHERE resource_id NOT IN (SELECT resource_id FROM locks WHERE expires_at > NOW())
```

**Benefits**: No external dependencies, works in Phase 5  
**Limitations**: Single-machine only (fine for Phase 5, Redis for Phase 6+)

## Dependencies

- SQLite3 (Node.js `sqlite3` or `better-sqlite3` package)
- Phase 4 framework components (JobScheduler, ExecutionEngine)
- Orchestration YAML files from earlier phases

## Risk Mitigations

1. **Data Loss**: Keep existing orchestrations.sqlite untouched until migration complete
2. **Locking Deadlocks**: Implement timeout with automatic release
3. **API Compatibility**: Maintain backward compatibility with Phase 4 endpoints

## Success Criteria

- [ ] All job executions persisted to SQLite
- [ ] Orchestration runs tracked with full history
- [ ] Distributed locking prevents concurrent runs
- [ ] LaunchD jobs successfully migrated to framework
- [ ] All Phase 4 tests pass with persistence layer
- [ ] New API endpoints tested and documented
- [ ] Zero data loss during migration

## Timeline

| Phase | Tasks | Duration |
|-------|-------|----------|
| Setup | T031-T034 | 2-3 days |
| Integration | T036-T038 | 2-3 days |
| Migration | T039-T040 | 1-2 days |
| Testing | E2E tests | 1 day |

**Total**: ~5-6 days for Phase 5
