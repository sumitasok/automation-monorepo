/**
 * Job State Manager
 * Persists job execution state to SQLite database
 * Provides query API for execution history and statistics
 */

const fs = require('fs').promises;
const path = require('path');
const EventEmitter = require('events');

class JobStateManager extends EventEmitter {
  constructor(dbPath = null) {
    super();
    this.dbPath = dbPath || path.join(process.env.HOME, 'automation-monorepo-config', 'data', 'job-state.sqlite');
    this.db = null;
    this.executions = new Map(); // In-memory fallback for Phase 5
    this.locks = new Map(); // In-memory locks: resource -> { holder, expiresAt }
    this.initialized = false;
  }

  /**
   * Initialize database connection and schema
   */
  async initialize() {
    try {
      // Ensure directory exists
      const dir = path.dirname(this.dbPath);
      await fs.mkdir(dir, { recursive: true });

      // Import sqlite3 (will be optional dependency)
      try {
        const sqlite3 = require('better-sqlite3');
        this.db = new sqlite3(this.dbPath);
        this.db.pragma('journal_mode = WAL');
        await this._initializeSchema();
        console.log(`✓ Job state database initialized: ${this.dbPath}`);
      } catch (err) {
        if (err.code === 'MODULE_NOT_FOUND') {
          console.warn('⚠️ sqlite3 not available, using in-memory execution tracking');
          console.warn('   Install: npm install better-sqlite3');
        } else {
          throw err;
        }
      }

      this.initialized = true;
    } catch (error) {
      console.error('Failed to initialize state manager:', error);
      throw error;
    }
  }

  /**
   * Initialize database schema
   */
  async _initializeSchema() {
    if (!this.db) return;

    const schema = `
      CREATE TABLE IF NOT EXISTS jobs (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        schedule TEXT,
        timeout INTEGER,
        retry_max_attempts INTEGER,
        retry_backoff_multiplier REAL,
        enabled BOOLEAN DEFAULT 1,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
      );

      CREATE TABLE IF NOT EXISTS executions (
        id TEXT PRIMARY KEY,
        job_id TEXT NOT NULL,
        status TEXT NOT NULL,
        started_at TIMESTAMP NOT NULL,
        ended_at TIMESTAMP,
        attempts INTEGER DEFAULT 1,
        last_error TEXT,
        result TEXT,
        context TEXT,
        FOREIGN KEY (job_id) REFERENCES jobs(id)
      );

      CREATE TABLE IF NOT EXISTS orchestrations (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        started_at TIMESTAMP NOT NULL,
        ended_at TIMESTAMP,
        status TEXT NOT NULL,
        total_steps INTEGER,
        completed_steps INTEGER DEFAULT 0
      );

      CREATE TABLE IF NOT EXISTS orchestration_steps (
        id TEXT PRIMARY KEY,
        orchestration_id TEXT NOT NULL,
        step_index INTEGER NOT NULL,
        job_id TEXT NOT NULL,
        status TEXT NOT NULL,
        started_at TIMESTAMP NOT NULL,
        ended_at TIMESTAMP,
        attempts INTEGER DEFAULT 1,
        result TEXT,
        FOREIGN KEY (orchestration_id) REFERENCES orchestrations(id),
        FOREIGN KEY (job_id) REFERENCES jobs(id)
      );

      CREATE TABLE IF NOT EXISTS locks (
        resource_id TEXT PRIMARY KEY,
        holder_id TEXT NOT NULL,
        acquired_at TIMESTAMP NOT NULL,
        expires_at TIMESTAMP NOT NULL
      );

      CREATE INDEX IF NOT EXISTS idx_executions_job_id ON executions(job_id);
      CREATE INDEX IF NOT EXISTS idx_executions_status ON executions(status);
      CREATE INDEX IF NOT EXISTS idx_executions_started_at ON executions(started_at);
      CREATE INDEX IF NOT EXISTS idx_orchestration_steps_orch_id ON orchestration_steps(orchestration_id);
      CREATE INDEX IF NOT EXISTS idx_locks_expires_at ON locks(expires_at);
    `;

    const statements = schema.split(';').filter((s) => s.trim());
    for (const stmt of statements) {
      this.db.exec(stmt);
    }
  }

  /**
   * Record job registration
   */
  recordJobRegistration(jobId, jobDefinition) {
    const record = {
      id: jobId,
      name: jobDefinition.name,
      schedule: JSON.stringify(jobDefinition.schedule),
      timeout: jobDefinition.timeout,
      retry_max_attempts: jobDefinition.retry?.maxRetries,
      retry_backoff_multiplier: jobDefinition.retry?.backoffMultiplier,
      enabled: jobDefinition.enabled !== false,
    };

    if (this.db) {
      try {
        const stmt = this.db.prepare(`
          INSERT OR REPLACE INTO jobs
          (id, name, schedule, timeout, retry_max_attempts, retry_backoff_multiplier, enabled, updated_at)
          VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
        `);
        stmt.run(
          record.id,
          record.name,
          record.schedule,
          record.timeout,
          record.retry_max_attempts,
          record.retry_backoff_multiplier,
          record.enabled ? 1 : 0
        );
      } catch (err) {
        console.error(`Failed to record job registration for ${jobId}:`, err.message);
      }
    }
  }

  /**
   * Record execution start
   */
  recordExecutionStart(executionId, jobId, execution) {
    const record = {
      id: executionId,
      job_id: jobId,
      status: 'running',
      started_at: execution.startTime instanceof Date ? execution.startTime.toISOString() : (execution.startTime || new Date().toISOString()),
      context: JSON.stringify(execution.context || {}),
    };

    if (this.db) {
      try {
        const stmt = this.db.prepare(`
          INSERT INTO executions
          (id, job_id, status, started_at, context, attempts)
          VALUES (?, ?, ?, ?, ?, 1)
        `);
        stmt.run(
          record.id,
          record.job_id,
          record.status,
          record.started_at,
          record.context
        );
      } catch (err) {
        console.error(`Failed to record execution start for ${executionId}:`, err.message);
      }
    } else {
      // In-memory fallback
      this.executions.set(executionId, record);
    }
  }

  /**
   * Record execution completion
   */
  recordExecutionComplete(executionId, jobId, execution) {
    const endTime = execution.endTime || new Date();
    const record = {
      id: executionId,
      status: execution.status || 'completed',
      ended_at: endTime instanceof Date ? endTime.toISOString() : endTime,
      attempts: execution.attempts || 1,
      last_error: execution.lastError?.message,
      result: execution.results ? JSON.stringify(execution.results) : null,
    };

    if (this.db) {
      try {
        const stmt = this.db.prepare(`
          UPDATE executions
          SET status = ?, ended_at = ?, attempts = ?, last_error = ?, result = ?
          WHERE id = ?
        `);
        stmt.run(
          record.status,
          record.ended_at,
          record.attempts,
          record.last_error,
          record.result,
          record.id
        );
      } catch (err) {
        console.error(`Failed to record execution completion for ${executionId}:`, err.message);
      }
    } else {
      // In-memory fallback
      const existing = this.executions.get(executionId) || {};
      this.executions.set(executionId, { ...existing, ...record });
    }

    this.emit('execution:recorded', { executionId, jobId, ...record });
  }

  /**
   * Get execution history
   */
  getExecutionHistory(filters = {}) {
    if (this.db) {
      try {
        let query = 'SELECT * FROM executions WHERE 1=1';
        const params = [];

        if (filters.jobId) {
          query += ' AND job_id = ?';
          params.push(filters.jobId);
        }

        if (filters.status) {
          query += ' AND status = ?';
          params.push(filters.status);
        }

        if (filters.startDate) {
          query += ' AND started_at >= ?';
          params.push(filters.startDate);
        }

        if (filters.endDate) {
          query += ' AND ended_at <= ?';
          params.push(filters.endDate);
        }

        query += ' ORDER BY started_at DESC';

        if (filters.limit) {
          query += ' LIMIT ?';
          params.push(filters.limit);
        }

        const stmt = this.db.prepare(query);
        return stmt.all(...params);
      } catch (err) {
        console.error('Failed to query execution history:', err.message);
        return [];
      }
    } else {
      // In-memory fallback
      let executions = Array.from(this.executions.values());

      if (filters.jobId) {
        executions = executions.filter((e) => e.job_id === filters.jobId);
      }

      if (filters.status) {
        executions = executions.filter((e) => e.status === filters.status);
      }

      return executions.sort((a, b) => b.started_at - a.started_at);
    }
  }

  /**
   * Get execution statistics for a job
   */
  getExecutionStats(jobId) {
    const executions = this.getExecutionHistory({ jobId });

    if (executions.length === 0) {
      return {
        totalExecutions: 0,
        successCount: 0,
        failureCount: 0,
        successRate: 0,
        avgDuration: 0,
        lastExecution: null,
      };
    }

    const successful = executions.filter((e) => e.status === 'success').length;
    const failed = executions.filter((e) => e.status === 'failed').length;

    const durations = executions
      .filter((e) => e.ended_at)
      .map((e) => new Date(e.ended_at) - new Date(e.started_at));

    const avgDuration = durations.length > 0 ? durations.reduce((a, b) => a + b, 0) / durations.length : 0;

    return {
      totalExecutions: executions.length,
      successCount: successful,
      failureCount: failed,
      successRate: (successful / executions.length) * 100,
      avgDuration,
      lastExecution: executions[0],
    };
  }

  /**
   * Acquire distributed lock
   */
  async acquireLock(resourceId, holderId, ttlSeconds = 3600) {
    if (!this.db) {
      // In-memory fallback: map-based lock with TTL
      const existingLock = this.locks.get(resourceId);
      const now = Date.now();

      // Check if lock exists and hasn't expired
      if (existingLock && existingLock.expiresAt > now) {
        // Lock is held
        return false;
      }

      // Lock is free or expired - acquire it
      const expiresAt = now + (ttlSeconds * 1000);
      this.locks.set(resourceId, { holder: holderId, expiresAt });
      return true;
    }

    try {
      const expiresAt = new Date(Date.now() + ttlSeconds * 1000);

      const stmt = this.db.prepare(`
        INSERT INTO locks (resource_id, holder_id, acquired_at, expires_at)
        SELECT ?, ?, CURRENT_TIMESTAMP, ?
        WHERE NOT EXISTS (
          SELECT 1 FROM locks
          WHERE resource_id = ? AND expires_at > CURRENT_TIMESTAMP
        )
      `);

      const result = stmt.run(resourceId, holderId, expiresAt, resourceId);
      return result.changes > 0;
    } catch (err) {
      console.error('Failed to acquire lock:', err.message);
      return false;
    }
  }

  /**
   * Release distributed lock
   */
  async releaseLock(resourceId, holderId) {
    if (!this.db) {
      // In-memory fallback: check holder matches and remove lock
      const lock = this.locks.get(resourceId);

      if (!lock) {
        // No lock exists
        return true; // Idempotent: releasing non-existent lock succeeds
      }

      if (lock.holder !== holderId) {
        // Lock is held by different holder - cannot release
        return false;
      }

      // Same holder - release the lock
      this.locks.delete(resourceId);
      return true;
    }

    try {
      const stmt = this.db.prepare(`
        DELETE FROM locks
        WHERE resource_id = ? AND holder_id = ?
      `);
      stmt.run(resourceId, holderId);
      return true;
    } catch (err) {
      console.error('Failed to release lock:', err.message);
      return false;
    }
  }

  /**
   * Close database connection
   */
  async close() {
    if (this.db) {
      this.db.close();
      this.db = null;
    }
  }
}

module.exports = JobStateManager;
