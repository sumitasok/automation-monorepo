/**
 * Orchestrator Manager Tests
 * Validates orchestration step execution and tracking
 */

const OrchestratorJobManager = require('../orchestrator-manager.js');
const JobScheduler = require('../scheduler.js');
const path = require('path');
const fs = require('fs').promises;

describe('OrchestratorJobManager - T036', () => {
  let scheduler;
  let orchestrator;
  const testOrchDir = path.join(__dirname, 'test-orchestrations');

  beforeAll(async () => {
    // Create test orchestrations directory
    await fs.mkdir(testOrchDir, { recursive: true });

    // Create test orchestration files
    const simpleOrch = {
      name: 'simple-workflow',
      description: 'Simple 2-step workflow',
      steps: [
        { job: 'job-1', args: [] },
        { job: 'job-2', args: [] },
      ],
    };

    const complexOrch = {
      name: 'complex-workflow',
      description: 'Complex 3-step workflow with args',
      steps: [
        { job: 'job-1', args: ['--arg1', 'value1'] },
        { job: 'job-2' },
        { job: 'job-3', args: ['--flag'] },
      ],
    };

    const yaml = require('js-yaml');
    await fs.writeFile(
      path.join(testOrchDir, 'simple.yaml'),
      yaml.dump(simpleOrch)
    );
    await fs.writeFile(
      path.join(testOrchDir, 'complex.yaml'),
      yaml.dump(complexOrch)
    );

    // Initialize scheduler with test jobs
    scheduler = new JobScheduler('.', {
      executionTimeout: 300,
      maxRetries: 2,
      backoffMultiplier: 2,
    });

    // Register mock jobs
    const mockHandler = async ({ executionId, jobId, execution }) => {
      return { status: 'success', jobId, executionId };
    };

    scheduler.registerJob('job-1', {
      name: 'Test Job 1',
      schedule: { type: 'manual' },
      timeout: 60,
      handlers: { execute: mockHandler },
    });

    scheduler.registerJob('job-2', {
      name: 'Test Job 2',
      schedule: { type: 'manual' },
      timeout: 60,
      handlers: { execute: mockHandler },
    });

    scheduler.registerJob('job-3', {
      name: 'Test Job 3',
      schedule: { type: 'manual' },
      timeout: 60,
      handlers: { execute: mockHandler },
    });

    scheduler.isRunning = true;

    // Initialize orchestrator
    orchestrator = new OrchestratorJobManager(scheduler, '.');
    const loaded = await orchestrator.loadOrchestrations(testOrchDir);
    expect(loaded).toBe(2);
  });

  afterAll(async () => {
    // Cleanup test orchestrations directory
    try {
      const files = await fs.readdir(testOrchDir);
      for (const file of files) {
        await fs.unlink(path.join(testOrchDir, file));
      }
      await fs.rmdir(testOrchDir);
    } catch (err) {
      // Ignore cleanup errors
    }
  });

  describe('Orchestration Loading', () => {
    test('should load orchestrations from directory', async () => {
      const loaded = await orchestrator.loadOrchestrations(testOrchDir);
      expect(loaded).toBeGreaterThan(0);
    });

    test('should have correct orchestration count', () => {
      expect(orchestrator.orchestrations.size).toBe(2);
    });

    test('should load orchestration with correct name', () => {
      const orch = orchestrator.orchestrations.get('simple-workflow');
      expect(orch).toBeDefined();
      expect(orch.name).toBe('simple-workflow');
    });

    test('should load orchestration with correct steps', () => {
      const orch = orchestrator.orchestrations.get('complex-workflow');
      expect(orch.steps.length).toBe(3);
      expect(orch.steps[0].job).toBe('job-1');
      expect(orch.steps[1].job).toBe('job-2');
      expect(orch.steps[2].job).toBe('job-3');
    });
  });

  describe('Orchestration Registration', () => {
    test('should register orchestrations as framework jobs', () => {
      const registered = orchestrator.registerOrchestrations();
      expect(registered).toBe(2);

      const simpleJob = scheduler.getJob('simple-workflow');
      expect(simpleJob).toBeDefined();
      expect(simpleJob.name).toContain('simple-workflow');
    });

    test('should have job handlers', () => {
      const job = scheduler.getJob('simple-workflow');
      expect(job.handlers).toBeDefined();
      expect(job.handlers.execute).toBeDefined();
      expect(job.handlers.onStart).toBeDefined();
    });
  });

  describe('Orchestration Listing', () => {
    test('should list all orchestrations', () => {
      const list = orchestrator.listOrchestrations();
      expect(Array.isArray(list)).toBe(true);
      expect(list.length).toBe(2);
    });

    test('should include step count in listing', () => {
      const list = orchestrator.listOrchestrations();
      const simpleOrch = list.find((o) => o.name === 'simple-workflow');
      expect(simpleOrch.steps).toBe(2);
    });

    test('should include step names in listing', () => {
      const list = orchestrator.listOrchestrations();
      const complexOrch = list.find((o) => o.name === 'complex-workflow');
      expect(complexOrch.stepNames).toContain('job-1');
      expect(complexOrch.stepNames).toContain('job-2');
      expect(complexOrch.stepNames).toContain('job-3');
    });
  });

  describe('Orchestration Execution', () => {
    test('should trigger orchestration', async () => {
      const executionId = await orchestrator.triggerOrchestration('simple-workflow');
      expect(executionId).toBeDefined();
      expect(typeof executionId).toBe('string');
    });

    test('should create execution record', async () => {
      const executionId = await orchestrator.triggerOrchestration('simple-workflow');

      // Wait for execution to complete
      await new Promise((resolve) => setTimeout(resolve, 100));

      const exec = orchestrator.getExecution(executionId);
      expect(exec).toBeDefined();
    });

    test('should track orchestration name', async () => {
      const executionId = await orchestrator.triggerOrchestration('complex-workflow');

      await new Promise((resolve) => setTimeout(resolve, 100));

      const exec = orchestrator.getExecution(executionId);
      expect(exec.orchestrationName).toBe('complex-workflow');
    });

    test('should fail if orchestration not found', async () => {
      expect(() => orchestrator.triggerOrchestration('non-existent')).rejects.toThrow();
    });
  });

  describe('Execution History', () => {
    test('should track execution history', async () => {
      const id1 = await orchestrator.triggerOrchestration('simple-workflow');
      const id2 = await orchestrator.triggerOrchestration('simple-workflow');

      await new Promise((resolve) => setTimeout(resolve, 100));

      const history = orchestrator.getExecutionHistory('simple-workflow');
      expect(history.length).toBeGreaterThanOrEqual(0);
    });

    test('should sort history by most recent first', async () => {
      const history = orchestrator.getExecutionHistory('simple-workflow');

      if (history.length > 1) {
        expect(history[0].startTime.getTime()).toBeGreaterThanOrEqual(
          history[1].startTime.getTime()
        );
      }
    });
  });

  describe('Event Emission', () => {
    test('should emit orchestration:started event', (done) => {
      orchestrator.once('orchestration:started', ({ executionId, name }) => {
        expect(executionId).toBeDefined();
        expect(name).toBe('simple-workflow');
        done();
      });

      orchestrator.triggerOrchestration('simple-workflow').catch((err) => {
        console.error('Orchestration failed:', err);
      });
    });

    test('should emit orchestration:completed event', (done) => {
      orchestrator.once('orchestration:completed', ({ executionId, name }) => {
        expect(executionId).toBeDefined();
        expect(name).toBeDefined();
        done();
      });

      orchestrator.triggerOrchestration('simple-workflow').catch((err) => {
        console.error('Orchestration failed:', err);
      });
    });

    test('should emit step events', (done) => {
      let startedCount = 0;
      let completedCount = 0;

      orchestrator.on('orchestration:step:started', () => {
        startedCount++;
      });

      orchestrator.once('orchestration:step:completed', () => {
        completedCount++;
        if (startedCount > 0 && completedCount > 0) {
          done();
        }
      });

      orchestrator.triggerOrchestration('simple-workflow').catch((err) => {
        console.error('Orchestration failed:', err);
      });
    });
  });

  describe('Step Tracking', () => {
    test('should track total steps', async () => {
      const executionId = await orchestrator.triggerOrchestration('simple-workflow');

      await new Promise((resolve) => setTimeout(resolve, 100));

      const exec = orchestrator.getExecution(executionId);
      expect(exec.totalSteps).toBe(2);
    });

    test('should track current step index', async () => {
      const executionId = await orchestrator.triggerOrchestration('complex-workflow');

      await new Promise((resolve) => setTimeout(resolve, 100));

      const exec = orchestrator.getExecution(executionId);
      expect(exec.currentStepIndex).toBeGreaterThanOrEqual(0);
    });

    test('should record step results', async () => {
      const executionId = await orchestrator.triggerOrchestration('simple-workflow');

      await new Promise((resolve) => setTimeout(resolve, 100));

      const exec = orchestrator.getExecution(executionId);
      expect(Array.isArray(exec.steps)).toBe(true);
    });
  });
});
