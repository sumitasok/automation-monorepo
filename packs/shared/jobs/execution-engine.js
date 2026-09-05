/**
 * Job Execution Engine
 * Handles running, tracking, and monitoring individual job executions
 * Separate from scheduling logic for reusability
 */

const EventEmitter = require('events');

class ExecutionEngine extends EventEmitter {
  constructor(options = {}) {
    super();
    this.executions = new Map();
    this.executionId = 0;

    this.options = {
      defaultTimeout: options.defaultTimeout || 300, // seconds
      maxRetries: options.maxRetries || 3,
      backoffMultiplier: options.backoffMultiplier || 2,
      initialDelayMs: options.initialDelayMs || 5000,
      ...options,
    };
  }

  /**
   * Execute a job handler with all tracking
   * @param {Object} jobContext - { jobId, name, description, handlers, timeout, retry }
   * @param {Object} executionContext - { executionMetadata, userContext }
   * @returns {Promise} result of execution
   */
  async execute(jobContext, executionContext = {}) {
    const executionId = String(++this.executionId);

    const execution = {
      id: executionId,
      jobId: jobContext.jobId,
      jobName: jobContext.name,
      status: 'pending',
      startTime: new Date(),
      endTime: null,
      duration: 0,
      attempts: 0,
      maxAttempts: (jobContext.retry?.maxRetries || this.options.maxRetries) + 1,
      lastError: null,
      errors: [],
      results: null,
      metrics: {
        startMemory: process.memoryUsage().heapUsed,
        endMemory: null,
      },
      context: executionContext,
    };

    this.executions.set(executionId, execution);
    this._trackMetrics(execution, 'start');

    this.emit('execution:created', { executionId, jobId: jobContext.jobId });

    try {
      // Call onStart handler
      if (jobContext.handlers?.onStart) {
        this.emit('handler:starting', { executionId, handler: 'onStart' });
        await this._executeWithTimeout(
          () => jobContext.handlers.onStart({ executionId, ...jobContext }),
          jobContext.timeout || this.options.defaultTimeout
        );
        this.emit('handler:completed', { executionId, handler: 'onStart' });
      }

      // Execute with retry logic
      const result = await this._executeWithRetry(jobContext, execution);

      execution.status = 'success';
      execution.results = result;
      execution.endTime = new Date();
      execution.duration = execution.endTime - execution.startTime;

      // Call onSuccess handler
      if (jobContext.handlers?.onSuccess) {
        this.emit('handler:starting', { executionId, handler: 'onSuccess' });
        await this._executeWithTimeout(
          () =>
            jobContext.handlers.onSuccess({
              executionId,
              result,
              ...jobContext,
            }),
          jobContext.timeout || this.options.defaultTimeout
        );
        this.emit('handler:completed', { executionId, handler: 'onSuccess' });
      }

      this.emit('execution:succeeded', {
        executionId,
        jobId: jobContext.jobId,
        result,
        duration: execution.duration,
      });

      return result;
    } catch (error) {
      execution.status = 'failed';
      execution.lastError = {
        message: error.message,
        stack: error.stack,
        code: error.code,
      };
      execution.endTime = new Date();
      execution.duration = execution.endTime - execution.startTime;

      // Call onFailure handler
      if (jobContext.handlers?.onFailure) {
        this.emit('handler:starting', { executionId, handler: 'onFailure' });
        try {
          await this._executeWithTimeout(
            () =>
              jobContext.handlers.onFailure({
                executionId,
                error,
                attempts: execution.attempts,
                ...jobContext,
              }),
            jobContext.timeout || this.options.defaultTimeout
          );
        } catch (handlerError) {
          this.emit('handler:error', {
            executionId,
            handler: 'onFailure',
            error: handlerError,
          });
        }
        this.emit('handler:completed', { executionId, handler: 'onFailure' });
      }

      this.emit('execution:failed', {
        executionId,
        jobId: jobContext.jobId,
        error,
        attempts: execution.attempts,
        duration: execution.duration,
      });

      throw error;
    } finally {
      this._trackMetrics(execution, 'end');

      // Call onComplete handler
      if (jobContext.handlers?.onComplete) {
        this.emit('handler:starting', { executionId, handler: 'onComplete' });
        try {
          await this._executeWithTimeout(
            () =>
              jobContext.handlers.onComplete({
                executionId,
                status: execution.status,
                ...jobContext,
              }),
            jobContext.timeout || this.options.defaultTimeout
          );
        } catch (handlerError) {
          this.emit('handler:error', {
            executionId,
            handler: 'onComplete',
            error: handlerError,
          });
        }
        this.emit('handler:completed', { executionId, handler: 'onComplete' });
      }

      this.emit('execution:finished', {
        executionId,
        jobId: jobContext.jobId,
        status: execution.status,
      });
    }
  }

  /**
   * Execute handler with retry logic
   */
  async _executeWithRetry(jobContext, execution) {
    const maxAttempts = execution.maxAttempts;
    const retryConfig = jobContext.retry || {};

    let lastError;

    for (let attempt = 0; attempt < maxAttempts; attempt++) {
      execution.attempts = attempt + 1;

      try {
        this.emit('execution:attempt', {
          executionId: execution.id,
          attempt: attempt + 1,
          maxAttempts,
        });

        if (!jobContext.handlers?.execute) {
          throw new Error('Job handler must have execute function');
        }

        const result = await this._executeWithTimeout(
          () =>
            jobContext.handlers.execute({
              executionId: execution.id,
              attempt: attempt + 1,
              ...jobContext,
            }),
          jobContext.timeout || this.options.defaultTimeout
        );

        return result;
      } catch (error) {
        lastError = error;
        execution.errors.push({
          attempt: attempt + 1,
          message: error.message,
          stack: error.stack,
          timestamp: new Date(),
        });

        if (attempt < maxAttempts - 1) {
          const delay = this._calculateBackoffDelay(
            attempt,
            retryConfig.backoffMultiplier || this.options.backoffMultiplier
          );

          this.emit('execution:retry-scheduled', {
            executionId: execution.id,
            attempt: attempt + 1,
            nextRetryMs: delay,
            error: error.message,
          });

          await this._sleep(delay);
        }
      }
    }

    throw lastError;
  }

  /**
   * Execute function with timeout
   */
  async _executeWithTimeout(fn, timeoutSeconds) {
    return Promise.race([
      fn(),
      new Promise((_, reject) => {
        const handle = setTimeout(() => {
          reject(new Error(`Execution timeout after ${timeoutSeconds}s`));
        }, timeoutSeconds * 1000);

        // Clear timeout if execution completes
        Promise.resolve(fn()).finally(() => clearTimeout(handle));
      }),
    ]);
  }

  /**
   * Track execution metrics
   */
  _trackMetrics(execution, phase) {
    if (phase === 'start') {
      execution.metrics.startTime = Date.now();
      execution.metrics.startMemory = process.memoryUsage().heapUsed;
    } else if (phase === 'end') {
      execution.metrics.endTime = Date.now();
      execution.metrics.endMemory = process.memoryUsage().heapUsed;
      execution.metrics.duration = execution.metrics.endTime - execution.metrics.startTime;
      execution.metrics.memoryDelta =
        execution.metrics.endMemory - execution.metrics.startMemory;
    }
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

  /**
   * Get execution details
   */
  getExecution(executionId) {
    return this.executions.get(executionId);
  }

  /**
   * Get all executions (with optional filters)
   */
  getAllExecutions(filters = {}) {
    let executions = Array.from(this.executions.values());

    if (filters.jobId) {
      executions = executions.filter((e) => e.jobId === filters.jobId);
    }

    if (filters.status) {
      executions = executions.filter((e) => e.status === filters.status);
    }

    if (filters.minDuration) {
      executions = executions.filter((e) => e.duration >= filters.minDuration);
    }

    return executions;
  }

  /**
   * Clean up old executions
   */
  cleanup(olderThanMs = 24 * 60 * 60 * 1000) {
    const now = Date.now();
    const idsToDelete = [];

    for (const [id, execution] of this.executions) {
      if (execution.endTime && now - execution.endTime.getTime() > olderThanMs) {
        idsToDelete.push(id);
      }
    }

    idsToDelete.forEach((id) => this.executions.delete(id));
    return idsToDelete.length;
  }
}

module.exports = ExecutionEngine;
