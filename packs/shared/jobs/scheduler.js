/**
 * Framework Job Scheduler
 * Manages execution of domain and source jobs with scheduling, retries, and tracking
 */

const EventEmitter = require('events');
const path = require('path');

class JobScheduler extends EventEmitter {
  constructor(configPath, options = {}) {
    super();
    this.configPath = configPath;
    this.jobs = new Map(); // job-id -> job definition
    this.executions = new Map(); // execution-id -> execution record
    this.executionId = 0;

    this.options = {
      executionTimeout: options.executionTimeout || 300, // 5 minutes
      maxRetries: options.maxRetries || 3,
      backoffMultiplier: options.backoffMultiplier || 2,
      initialDelayMs: options.initialDelayMs || 5000,
      ...options,
    };

    this.timers = new Map(); // job-id -> timeout/interval handles
    this.isRunning = false;
  }

  /**
   * Register a job with the scheduler
   * @param {string} jobId - Unique job identifier
   * @param {Object} manifest - Job manifest (schedule, timeout, retry, handlers)
   */
  registerJob(jobId, manifest) {
    if (this.jobs.has(jobId)) {
      throw new Error(`Job ${jobId} already registered`);
    }

    const job = {
      id: jobId,
      name: manifest.name,
      description: manifest.description || '',
      schedule: manifest.schedule, // cron or interval
      timeout: manifest.timeout || this.options.executionTimeout,
      retry: manifest.retry || {
        maxRetries: this.options.maxRetries,
        backoffMultiplier: this.options.backoffMultiplier,
      },
      handlers: manifest.handlers, // { onStart, onSuccess, onFailure, onComplete }
      enabled: manifest.enabled !== false,
      createdAt: new Date(),
    };

    this.jobs.set(jobId, job);
    this.emit('job:registered', { jobId, job });

    if (this.isRunning && job.enabled) {
      this._scheduleJob(jobId);
    }
  }

  /**
   * Unregister a job
   */
  unregisterJob(jobId) {
    if (!this.jobs.has(jobId)) {
      throw new Error(`Job ${jobId} not found`);
    }

    this._clearJobSchedule(jobId);
    this.jobs.delete(jobId);
    this.emit('job:unregistered', { jobId });
  }

  /**
   * Trigger a job immediately (user-initiated or automatic)
   * @param {string} jobId - Job to trigger
   * @param {Object} context - Context data for job execution
   * @returns {string} execution ID
   */
  async triggerJob(jobId, context = {}) {
    const job = this.jobs.get(jobId);
    if (!job) {
      throw new Error(`Job ${jobId} not found`);
    }

    if (!job.enabled) {
      throw new Error(`Job ${jobId} is disabled`);
    }

    return this._executeJob(jobId, context, { triggered: true });
  }

  /**
   * Start the scheduler
   * Activates all registered jobs according to their schedules
   */
  async start() {
    if (this.isRunning) {
      throw new Error('Scheduler is already running');
    }

    this.isRunning = true;
    this.emit('scheduler:started', { timestamp: new Date() });

    for (const [jobId, job] of this.jobs) {
      if (job.enabled) {
        this._scheduleJob(jobId);
      }
    }
  }

  /**
   * Stop the scheduler
   * Cancels all pending executions
   */
  async stop() {
    if (!this.isRunning) {
      throw new Error('Scheduler is not running');
    }

    this.isRunning = false;

    // Clear all timers
    for (const [jobId] of this.jobs) {
      this._clearJobSchedule(jobId);
    }

    this.emit('scheduler:stopped', { timestamp: new Date() });
  }

  /**
   * Get execution history
   * @param {Object} filters - Filter by jobId, status, date range
   * @returns {Array} execution records
   */
  getExecutionHistory(filters = {}) {
    let executions = Array.from(this.executions.values());

    if (filters.jobId) {
      executions = executions.filter((e) => e.jobId === filters.jobId);
    }

    if (filters.status) {
      executions = executions.filter((e) => e.status === filters.status);
    }

    if (filters.startDate) {
      executions = executions.filter((e) => e.startTime >= filters.startDate);
    }

    if (filters.endDate) {
      executions = executions.filter((e) => e.endTime <= filters.endDate);
    }

    return executions.sort((a, b) => b.startTime - a.startTime);
  }

  /**
   * Get job details
   */
  getJob(jobId) {
    return this.jobs.get(jobId);
  }

  /**
   * Get all registered jobs
   */
  getAllJobs() {
    return Array.from(this.jobs.values());
  }

  /**
   * Get execution details
   */
  getExecution(executionId) {
    return this.executions.get(executionId);
  }

  /**
   * Enable/disable a job
   */
  setJobEnabled(jobId, enabled) {
    const job = this.jobs.get(jobId);
    if (!job) {
      throw new Error(`Job ${jobId} not found`);
    }

    job.enabled = enabled;

    if (this.isRunning) {
      if (enabled) {
        this._scheduleJob(jobId);
      } else {
        this._clearJobSchedule(jobId);
      }
    }

    this.emit('job:enabled-changed', { jobId, enabled });
  }

  // ============ Private Methods ============

  /**
   * Schedule a job according to its schedule type
   */
  _scheduleJob(jobId) {
    const job = this.jobs.get(jobId);
    if (!job) return;

    const { schedule } = job;

    // TODO: Implement cron-based scheduling
    // For now, support simple interval-based scheduling
    if (schedule.type === 'interval') {
      const intervalMs = this._parseInterval(schedule.interval);
      const handle = setInterval(() => {
        this._executeJob(jobId, {}, { scheduled: true }).catch((err) => {
          this.emit('job:error', { jobId, error: err });
        });
      }, intervalMs);

      this.timers.set(jobId, handle);
    }

    this.emit('job:scheduled', { jobId, schedule });
  }

  /**
   * Clear scheduling for a job
   */
  _clearJobSchedule(jobId) {
    const handle = this.timers.get(jobId);
    if (handle) {
      clearInterval(handle);
      clearTimeout(handle);
      this.timers.delete(jobId);
    }
  }

  /**
   * Execute a job with retry logic
   */
  async _executeJob(jobId, context = {}, metadata = {}) {
    const job = this.jobs.get(jobId);
    if (!job) {
      throw new Error(`Job ${jobId} not found`);
    }

    const executionId = String(++this.executionId);
    const execution = {
      id: executionId,
      jobId,
      status: 'pending',
      startTime: new Date(),
      endTime: null,
      attempts: 0,
      lastError: null,
      results: null,
      context,
      metadata,
    };

    this.executions.set(executionId, execution);

    this.emit('execution:started', { executionId, jobId });

    try {
      // Call onStart handler if provided
      if (job.handlers?.onStart) {
        await job.handlers.onStart({ executionId, jobId, execution });
      }

      // Execute with retries
      let lastError;
      for (
        let attempt = 0;
        attempt <= job.retry.maxRetries;
        attempt++
      ) {
        execution.attempts = attempt + 1;

        try {
          const result = await this._runJobWithTimeout(jobId, execution, job);
          execution.status = 'success';
          execution.results = result;
          execution.endTime = new Date();

          if (job.handlers?.onSuccess) {
            await job.handlers.onSuccess({
              executionId,
              jobId,
              execution,
              result,
            });
          }

          this.emit('execution:completed', { executionId, jobId, status: 'success' });
          return executionId;
        } catch (err) {
          lastError = err;
          execution.lastError = {
            message: err.message,
            stack: err.stack,
            attempt,
          };

          if (attempt < job.retry.maxRetries) {
            const delay = this._calculateBackoffDelay(
              attempt,
              job.retry.backoffMultiplier
            );
            this.emit('execution:retry', {
              executionId,
              jobId,
              attempt,
              nextRetryIn: delay,
            });
            await this._sleep(delay);
          }
        }
      }

      // All retries failed
      execution.status = 'failed';
      execution.endTime = new Date();

      if (job.handlers?.onFailure) {
        await job.handlers.onFailure({
          executionId,
          jobId,
          execution,
          error: lastError,
        });
      }

      this.emit('execution:failed', { executionId, jobId, error: lastError });
      throw lastError;
    } finally {
      if (job.handlers?.onComplete) {
        await job.handlers.onComplete({
          executionId,
          jobId,
          execution,
        });
      }
    }
  }

  /**
   * Run job handler with timeout
   */
  async _runJobWithTimeout(jobId, execution, job) {
    const timeoutPromise = new Promise((_, reject) => {
      setTimeout(
        () => reject(new Error(`Job ${jobId} execution timeout after ${job.timeout}s`)),
        job.timeout * 1000
      );
    });

    if (!job.handlers?.execute) {
      throw new Error(`Job ${jobId} has no execute handler`);
    }

    return Promise.race([
      job.handlers.execute({ executionId: execution.id, jobId, execution }),
      timeoutPromise,
    ]);
  }

  /**
   * Parse interval string (e.g., "5s", "1m", "1h", "1d")
   */
  _parseInterval(interval) {
    const match = interval.match(/^(\d+)([smhd])$/);
    if (!match) throw new Error(`Invalid interval format: ${interval}`);

    const [, value, unit] = match;
    const multipliers = {
      s: 1000,
      m: 60 * 1000,
      h: 60 * 60 * 1000,
      d: 24 * 60 * 60 * 1000,
    };
    return parseInt(value) * multipliers[unit];
  }

  /**
   * Calculate exponential backoff delay
   */
  _calculateBackoffDelay(attempt, multiplier) {
    return this.options.initialDelayMs * Math.pow(multiplier, attempt);
  }

  /**
   * Sleep utility
   */
  _sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }
}

module.exports = JobScheduler;
