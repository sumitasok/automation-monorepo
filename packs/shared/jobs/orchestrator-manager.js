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
  constructor(jobScheduler, configPath) {
    super();
    this.scheduler = jobScheduler;
    this.configPath = configPath;
    this.orchestrations = new Map(); // name -> orchestration definition
    this.executions = new Map(); // execution-id -> execution record
    this.executionId = 0;
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
      console.error('Failed to load orchestrations:', error.message);
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

  // ============ Private Handlers ============

  async _onOrchestrationStart({ executionId, jobId }) {
    const executionRecord = {
      id: executionId,
      orchestrationName: jobId,
      status: 'running',
      startTime: new Date(),
      steps: [],
      currentStepIndex: 0,
      totalSteps: this.orchestrations.get(jobId)?.steps.length || 0,
    };

    this.executions.set(executionId, executionRecord);
    this.emit('orchestration:started', { executionId, name: jobId });
  }

  async _executeOrchestration(orchestrationName, { executionId, jobId, execution }) {
    const orch = this.orchestrations.get(orchestrationName);
    if (!orch) {
      throw new Error(`Orchestration not found: ${orchestrationName}`);
    }

    const execRecord = this.executions.get(executionId);

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
          const stepRecord = {
            stepIndex,
            jobId: step.job,
            executionId: stepExecutionId,
            status: stepExecution?.status || 'completed',
            startTime: stepStartTime,
            endTime: new Date(),
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
          const stepRecord = {
            stepIndex,
            jobId: step.job,
            status: 'failed',
            startTime: stepStartTime,
            endTime: new Date(),
            error: stepError,
          };

          stepRecords.push(stepRecord);

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
      execRecord.error = error.message;

      throw error;
    }
  }

  async _onOrchestrationSuccess({ executionId, jobId, execution, result }) {
    const execRecord = this.executions.get(executionId);
    if (execRecord) {
      execRecord.endTime = new Date();
      execRecord.status = 'success';
    }

    this.emit('orchestration:succeeded', {
      executionId,
      name: jobId,
      result,
    });
  }

  async _onOrchestrationFailure({ executionId, jobId, execution, error }) {
    const execRecord = this.executions.get(executionId);
    if (execRecord) {
      execRecord.endTime = new Date();
      execRecord.status = 'failed';
      execRecord.error = error.message;
    }

    this.emit('orchestration:failed', {
      executionId,
      name: jobId,
      error: error.message,
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
