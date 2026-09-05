/**
 * Orchestrator Job Manager
 * Wraps framework JobScheduler to execute multi-step orchestrations
 * Each orchestration step is tracked independently with retry logic
 */

const fs = require('fs').promises;
const path = require('path');
const yaml = require('js-yaml');
const EventEmitter = require('events');

class OrchestratorJobManager extends EventEmitter {
  constructor(jobScheduler, configPath, stateManager = null) {
    super();
    this.scheduler = jobScheduler;
    this.configPath = configPath;
    this.stateManager = stateManager; // Optional: JobStateManager for persistence
    this.orchestrations = new Map(); // name -> orchestration definition
    this.executions = new Map(); // execution-id -> execution record
    this.executionId = 0;
  }

  /**
   * Set state manager for persistence
   */
  setStateManager(stateManager) {
    this.stateManager = stateManager;
  }

  /**
   * Load orchestrations from YAML directory
   */
  async loadOrchestrations(orchestrationsDir) {
    try {
      const files = await fs.readdir(orchestrationsDir);
      const yamlFiles = files.filter((f) => f.endsWith('.yaml') || f.endsWith('.yml'));

      for (const file of yamlFiles) {
        const filePath = path.join(orchestrationsDir, file);
        const content = await fs.readFile(filePath, 'utf-8');
        const orch = yaml.load(content);

        if (!orch || !orch.steps) {
          console.warn(`Skipping invalid orchestration: ${file}`);
          continue;
        }

        const name = orch.name || file.replace(/\.(yaml|yml)$/, '');
        this.orchestrations.set(name, {
          id: name,
          name,
          description: orch.description || '',
          steps: orch.steps || [],
          filePath,
        });

        console.log(`✓ Loaded orchestration: ${name} (${orch.steps.length} steps)`);
      }

      return this.orchestrations.size;
    } catch (error) {
      console.error('Failed to load orchestrations:', error?.message || String(error) || 'Unknown error');
      return 0;
    }
  }

  /**
   * Register all loaded orchestrations as framework jobs
   */
  registerOrchestrations() {
    let registered = 0;

    for (const [name, orch] of this.orchestrations) {
      this.scheduler.registerJob(name, {
        name: `Orchestration: ${orch.name}`,
        description: orch.description,
        schedule: { type: 'manual' }, // Orchestrations triggered manually or by schedule
        timeout: this._calculateTimeout(orch.steps),
        retry: { maxRetries: 1, backoffMultiplier: 2 }, // Orchestrations have internal retry
        enabled: true,
        handlers: {
          onStart: this._onOrchestrationStart.bind(this, name),
          execute: this._executeOrchestration.bind(this, name),
          onSuccess: this._onOrchestrationSuccess.bind(this, name),
          onFailure: this._onOrchestrationFailure.bind(this, name),
          onComplete: this._onOrchestrationComplete.bind(this, name),
        },
      });

      registered++;
    }

    console.log(`✓ Registered ${registered} orchestrations as framework jobs`);
    return registered;
  }

  /**
   * Trigger an orchestration manually
   */
  async triggerOrchestration(orchestrationName, context = {}) {
    const orch = this.orchestrations.get(orchestrationName);
    if (!orch) {
      throw new Error(`Orchestration not found: ${orchestrationName}`);
    }

    return this.scheduler.triggerJob(orchestrationName, context);
  }

  /**
   * Get execution record for an orchestration
   */
  getExecution(executionId) {
    return this.executions.get(executionId);
  }

  /**
   * Get orchestration history
   */
  getExecutionHistory(orchestrationName) {
    return Array.from(this.executions.values())
      .filter((e) => e.orchestrationName === orchestrationName)
      .sort((a, b) => b.startTime - a.startTime);
  }

  /**
   * Get list of all orchestrations
   */
  listOrchestrations() {
    return Array.from(this.orchestrations.values()).map((orch) => ({
      name: orch.name,
      description: orch.description,
      steps: orch.steps.length,
      stepNames: orch.steps.map((s) => s.job),
    }));
  }

  /**
   * Get orchestration run history
   */
  async getOrchestrationHistory(orchestrationName, limit = 50) {
    if (!this.stateManager) {
      // In-memory fallback
      return this.getExecutionHistory(orchestrationName).slice(0, limit);
    }

    // Query database for orchestration history
    try {
      if (this.stateManager.db) {
        const stmt = this.stateManager.db.prepare(`
          SELECT * FROM orchestrations
          WHERE name = ?
          ORDER BY started_at DESC
          LIMIT ?
        `);
        return stmt.all(orchestrationName, limit);
      }
    } catch (err) {
      console.error('Failed to query orchestration history:', err.message);
    }

    return [];
  }

  /**
   * Get orchestration step details
   */
  async getOrchestrationSteps(orchestrationId) {
    if (!this.stateManager) {
      // In-memory fallback
      const exec = this.executions.get(orchestrationId);
      return exec?.steps || [];
    }

    // Query database for orchestration steps
    try {
      if (this.stateManager.db) {
        const stmt = this.stateManager.db.prepare(`
          SELECT * FROM orchestration_steps
          WHERE orchestration_id = ?
          ORDER BY step_index ASC
        `);
        return stmt.all(orchestrationId);
      }
    } catch (err) {
      console.error('Failed to query orchestration steps:', err.message);
    }

    return [];
  }

  // ============ Private Handlers ============

  async _onOrchestrationStart({ executionId, jobId }) {
    const orch = this.orchestrations.get(jobId);
    const totalSteps = orch?.steps.length || 0;

    const executionRecord = {
      id: executionId,
      orchestrationName: jobId,
      status: 'running',
      startTime: new Date(),
      steps: [],
      currentStepIndex: 0,
      totalSteps,
    };

    this.executions.set(executionId, executionRecord);

    // Persist orchestration start to database
    if (this.stateManager) {
      try {
        if (this.stateManager.db) {
          const stmt = this.stateManager.db.prepare(`
            INSERT INTO orchestrations (id, name, started_at, status, total_steps, completed_steps)
            VALUES (?, ?, ?, ?, ?, 0)
          `);
          stmt.run(executionId, jobId, new Date(), 'running', totalSteps);
        }
      } catch (err) {
        console.error(`Failed to record orchestration start for ${executionId}:`, err.message);
      }
    }

    this.emit('orchestration:started', { executionId, name: jobId });
  }

  async _executeOrchestration(orchestrationName, { executionId, jobId, execution }) {
    const orch = this.orchestrations.get(orchestrationName);
    if (!orch) {
      throw new Error(`Orchestration not found: ${orchestrationName}`);
    }

    let execRecord = this.executions.get(executionId);
    if (!execRecord) {
      execRecord = {
        id: executionId,
        orchestrationName,
        status: 'running',
        startTime: new Date(),
        endTime: null,
        steps: [],
        error: null,
        currentStepIndex: 0,
      };
      this.executions.set(executionId, execRecord);
    }

    try {
      const results = [];
      const stepRecords = [];

      // Execute each step sequentially
      for (let stepIndex = 0; stepIndex < orch.steps.length; stepIndex++) {
        const step = orch.steps[stepIndex];
        const stepStartTime = new Date();

        execRecord.currentStepIndex = stepIndex;

        this.emit('orchestration:step:started', {
          executionId,
          orchestrationName,
          stepIndex,
          jobId: step.job,
        });

        try {
          // Trigger the job for this step
          const stepExecutionId = await this.scheduler.triggerJob(step.job, {
            step: stepIndex,
            orchestration: orchestrationName,
            stepArgs: step.args || [],
          });

          const stepExecution = this.scheduler.getExecution(stepExecutionId);
          const stepEndTime = new Date();
          const stepRecord = {
            stepIndex,
            jobId: step.job,
            executionId: stepExecutionId,
            status: stepExecution?.status || 'completed',
            startTime: stepStartTime,
            endTime: stepEndTime,
            result: stepExecution?.results,
            error: stepExecution?.lastError,
          };

          stepRecords.push(stepRecord);
          results.push({
            step: stepIndex,
            job: step.job,
            status: stepRecord.status,
            result: stepRecord.result,
          });

          // Persist step execution to database
          if (this.stateManager) {
            try {
              if (this.stateManager.db) {
                const stepId = `${executionId}-step-${stepIndex}`;
                const stmt = this.stateManager.db.prepare(`
                  INSERT INTO orchestration_steps
                  (id, orchestration_id, step_index, job_id, status, started_at, ended_at, attempts, result)
                  VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)
                `);
                stmt.run(
                  stepId,
                  executionId,
                  stepIndex,
                  step.job,
                  stepRecord.status,
                  stepStartTime,
                  stepEndTime,
                  JSON.stringify(stepRecord.result || {})
                );
              }
            } catch (err) {
              console.error(`Failed to record orchestration step ${stepIndex}:`, err.message);
            }
          }

          this.emit('orchestration:step:completed', {
            executionId,
            orchestrationName,
            stepIndex,
            jobId: step.job,
            status: stepRecord.status,
          });

          if (stepRecord.status === 'failed') {
            throw new Error(`Step ${stepIndex} (${step.job}) failed: ${stepRecord.error?.message}`);
          }
        } catch (stepError) {
          const stepEndTime = new Date();
          const stepRecord = {
            stepIndex,
            jobId: step.job,
            status: 'failed',
            startTime: stepStartTime,
            endTime: stepEndTime,
            error: stepError,
          };

          stepRecords.push(stepRecord);

          // Persist failed step to database
          if (this.stateManager) {
            try {
              if (this.stateManager.db) {
                const stepId = `${executionId}-step-${stepIndex}`;
                const stmt = this.stateManager.db.prepare(`
                  INSERT INTO orchestration_steps
                  (id, orchestration_id, step_index, job_id, status, started_at, ended_at, attempts, result)
                  VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)
                `);
                stmt.run(
                  stepId,
                  executionId,
                  stepIndex,
                  step.job,
                  'failed',
                  stepStartTime,
                  stepEndTime,
                  JSON.stringify({ error: stepError.message })
                );
              }
            } catch (err) {
              console.error(`Failed to record orchestration step failure for ${stepIndex}:`, err.message);
            }
          }

          this.emit('orchestration:step:failed', {
            executionId,
            orchestrationName,
            stepIndex,
            jobId: step.job,
            error: stepError.message,
          });

          // Fail fast on step failure (no retry at orchestration level)
          throw stepError;
        }
      }

      execRecord.steps = stepRecords;
      execRecord.status = 'success';

      return {
        status: 'success',
        orchestration: orchestrationName,
        steps: results.length,
        results,
      };
    } catch (error) {
      execRecord.status = 'failed';
      execRecord.error = error?.message || String(error) || 'Unknown error';

      throw error;
    }
  }

  async _onOrchestrationSuccess({ executionId, jobId, execution, result }) {
    const execRecord = this.executions.get(executionId);
    const endTime = new Date();

    if (execRecord) {
      execRecord.endTime = endTime;
      execRecord.status = 'success';
    }

    // Persist orchestration success to database
    if (this.stateManager) {
      try {
        if (this.stateManager.db) {
          const stmt = this.stateManager.db.prepare(`
            UPDATE orchestrations
            SET status = ?, ended_at = ?, completed_steps = total_steps
            WHERE id = ?
          `);
          stmt.run('success', endTime, executionId);
        }
      } catch (err) {
        console.error(`Failed to record orchestration success for ${executionId}:`, err.message);
      }
    }

    this.emit('orchestration:succeeded', {
      executionId,
      name: jobId,
      result,
    });
  }

  async _onOrchestrationFailure({ executionId, jobId, execution, error }) {
    const execRecord = this.executions.get(executionId);
    const endTime = new Date();

    if (execRecord) {
      execRecord.endTime = endTime;
      execRecord.status = 'failed';
      execRecord.error = error?.message || String(error) || 'Unknown error';
    }

    // Persist orchestration failure to database
    if (this.stateManager) {
      try {
        if (this.stateManager.db) {
          const stmt = this.stateManager.db.prepare(`
            UPDATE orchestrations
            SET status = ?, ended_at = ?
            WHERE id = ?
          `);
          stmt.run('failed', endTime, executionId);
        }
      } catch (err) {
        console.error(`Failed to record orchestration failure for ${executionId}:`, err.message);
      }
    }

    this.emit('orchestration:failed', {
      executionId,
      name: jobId,
      error: error?.message || String(error) || 'Unknown error',
    });
  }

  async _onOrchestrationComplete({ executionId, jobId, execution }) {
    this.emit('orchestration:completed', {
      executionId,
      name: jobId,
    });
  }

  /**
   * Calculate total timeout for orchestration based on step timeouts
   */
  _calculateTimeout(steps) {
    // Default 30 minutes, but could be calculated from step timeouts
    return 1800;
  }
}

module.exports = OrchestratorJobManager;
