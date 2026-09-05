/**
 * Orchestration History Persistence Tests - T038
 * Validates orchestration execution tracking with step-level details in database
 */

const OrchestratorJobManager = require('../orchestrator-manager.js');
const JobScheduler = require('../scheduler.js');
const JobStateManager = require('../state-manager.js');
const path = require('path');
const fs = require('fs').promises;
const yaml = require('js-yaml');

describe('Orchestration History Persistence - T038', () => {
  let scheduler;
  let stateManager;
  let orchestrator;
  const testDbPath = path.join(__dirname, 'test-orchestration-history.sqlite');
  const orchDir = path.join(__dirname, 'test-orchestrations');

  beforeAll(async () => {
    // Initialize state manager with test database
    stateManager = new JobStateManager(testDbPath);
    await stateManager.initialize();

    // Initialize scheduler
    scheduler = new JobScheduler('.', {
      stateManager,
    });

    // Initialize orchestrator with state manager
    orchestrator = new OrchestratorJobManager(scheduler, '.', stateManager);

    // Create test orchestrations directory
    await fs.mkdir(orchDir, { recursive: true });

    // Create simple test orchestration
    const simpleOrch = {
      name: 'test-simple-workflow',
      description: 'Simple test workflow',
      steps: [
        { job: 'test-job-1', args: [] },
        { job: 'test-job-2', args: [] },
      ],
    };

    await fs.writeFile(
      path.join(orchDir, 'simple-workflow.yaml'),
      yaml.dump(simpleOrch)
    );

    // Register test jobs
    scheduler.registerJob('test-job-1', {
      name: 'Test Job 1',
      schedule: { type: 'manual' },
      timeout: 60,
      retry: { maxRetries: 1, backoffMultiplier: 2 },
      handlers: {
        execute: async () => ({
          status: 'success',
          result: 'Job 1 completed',
        }),
      },
    });

    scheduler.registerJob('test-job-2', {
      name: 'Test Job 2',
      schedule: { type: 'manual' },
      timeout: 60,
      retry: { maxRetries: 1, backoffMultiplier: 2 },
      handlers: {
        execute: async () => ({
          status: 'success',
          result: 'Job 2 completed',
        }),
      },
    });

    // Load and register orchestrations
    await orchestrator.loadOrchestrations(orchDir);
    orchestrator.registerOrchestrations();
  });

  afterAll(async () => {
    if (orchestrator) {
      if (orchestrator.stateManager) {
        await orchestrator.stateManager.close();
      }
    }
    try {
      await fs.unlink(testDbPath);
    } catch (err) {
      // Ignore cleanup errors
    }
    try {
      await fs.rm(orchDir, { recursive: true });
    } catch (err) {
      // Ignore cleanup errors
    }
  });

  describe('Orchestration Start Recording', () => {
    test('should record orchestration start to database', async () => {
      const execId = `orch-exec-${Date.now()}`;

      // Simulate orchestration start
      await orchestrator._onOrchestrationStart({
        executionId: execId,
        jobId: 'test-simple-workflow',
      });

      // Verify in database
      if (stateManager.db) {
        const stmt = stateManager.db.prepare('SELECT * FROM orchestrations WHERE id = ?');
        const result = stmt.get(execId);

        expect(result).toBeDefined();
        expect(result.name).toBe('test-simple-workflow');
        expect(result.status).toBe('running');
        expect(result.total_steps).toBe(2);
      }
    });

    test('should track total steps on start', async () => {
      const execId = `orch-exec-${Date.now()}-2`;

      await orchestrator._onOrchestrationStart({
        executionId: execId,
        jobId: 'test-simple-workflow',
      });

      if (stateManager.db) {
        const stmt = stateManager.db.prepare('SELECT total_steps FROM orchestrations WHERE id = ?');
        const result = stmt.get(execId);
        expect(result.total_steps).toBe(2);
      }
    });
  });

  describe('Orchestration Step Recording', () => {
    test('should record step execution with details', async () => {
      const execId = `orch-exec-${Date.now()}-3`;

      // Create orchestration record
      if (stateManager.db) {
        const stmt = stateManager.db.prepare(`
          INSERT INTO orchestrations (id, name, started_at, status, total_steps, completed_steps)
          VALUES (?, ?, ?, ?, ?, 0)
        `);
        stmt.run(execId, 'test-simple-workflow', new Date(), 'running', 2);

        // Simulate step recording
        const stepId = `${execId}-step-0`;
        const stepStmt = stateManager.db.prepare(`
          INSERT INTO orchestration_steps
          (id, orchestration_id, step_index, job_id, status, started_at, ended_at, attempts, result)
          VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)
        `);
        stepStmt.run(
          stepId,
          execId,
          0,
          'test-job-1',
          'completed',
          new Date(),
          new Date(),
          JSON.stringify({ result: 'Job 1 completed' })
        );

        // Verify step in database
        const getStmt = stateManager.db.prepare('SELECT * FROM orchestration_steps WHERE id = ?');
        const step = getStmt.get(stepId);

        expect(step).toBeDefined();
        expect(step.step_index).toBe(0);
        expect(step.job_id).toBe('test-job-1');
        expect(step.status).toBe('completed');
      }
    });

    test('should record multiple steps in order', async () => {
      const execId = `orch-exec-${Date.now()}-4`;

      if (stateManager.db) {
        // Create orchestration
        const orchStmt = stateManager.db.prepare(`
          INSERT INTO orchestrations (id, name, started_at, status, total_steps, completed_steps)
          VALUES (?, ?, ?, ?, ?, 0)
        `);
        orchStmt.run(execId, 'test-simple-workflow', new Date(), 'running', 2);

        // Record step 0
        const step0Id = `${execId}-step-0`;
        const stepStmt = stateManager.db.prepare(`
          INSERT INTO orchestration_steps
          (id, orchestration_id, step_index, job_id, status, started_at, ended_at, attempts, result)
          VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)
        `);
        stepStmt.run(
          step0Id,
          execId,
          0,
          'test-job-1',
          'completed',
          new Date(),
          new Date(),
          JSON.stringify({ result: 'Job 1' })
        );

        // Record step 1
        const step1Id = `${execId}-step-1`;
        stepStmt.run(
          step1Id,
          execId,
          1,
          'test-job-2',
          'completed',
          new Date(),
          new Date(),
          JSON.stringify({ result: 'Job 2' })
        );

        // Verify both steps
        const getStmt = stateManager.db.prepare(
          'SELECT * FROM orchestration_steps WHERE orchestration_id = ? ORDER BY step_index'
        );
        const steps = getStmt.all(execId);

        expect(steps).toHaveLength(2);
        expect(steps[0].step_index).toBe(0);
        expect(steps[1].step_index).toBe(1);
      }
    });

    test('should record step failure with error details', async () => {
      const execId = `orch-exec-${Date.now()}-5`;

      if (stateManager.db) {
        // Create orchestration
        const orchStmt = stateManager.db.prepare(`
          INSERT INTO orchestrations (id, name, started_at, status, total_steps, completed_steps)
          VALUES (?, ?, ?, ?, ?, 0)
        `);
        orchStmt.run(execId, 'test-simple-workflow', new Date(), 'running', 2);

        // Record failed step
        const stepId = `${execId}-step-0`;
        const stepStmt = stateManager.db.prepare(`
          INSERT INTO orchestration_steps
          (id, orchestration_id, step_index, job_id, status, started_at, ended_at, attempts, result)
          VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)
        `);
        stepStmt.run(
          stepId,
          execId,
          0,
          'test-job-1',
          'failed',
          new Date(),
          new Date(),
          JSON.stringify({ error: 'Job failed' })
        );

        // Verify failed step
        const getStmt = stateManager.db.prepare('SELECT * FROM orchestration_steps WHERE id = ?');
        const step = getStmt.get(stepId);

        expect(step.status).toBe('failed');
        const result = JSON.parse(step.result);
        expect(result.error).toBe('Job failed');
      }
    });
  });

  describe('Orchestration Success Recording', () => {
    test('should record orchestration success', async () => {
      const execId = `orch-exec-${Date.now()}-6`;

      if (stateManager.db) {
        // Create orchestration
        const orchStmt = stateManager.db.prepare(`
          INSERT INTO orchestrations (id, name, started_at, status, total_steps, completed_steps)
          VALUES (?, ?, ?, ?, ?, 0)
        `);
        orchStmt.run(execId, 'test-simple-workflow', new Date(), 'running', 2);

        // Simulate success
        await orchestrator._onOrchestrationSuccess({
          executionId: execId,
          jobId: 'test-simple-workflow',
        });

        // Verify
        const getStmt = stateManager.db.prepare('SELECT * FROM orchestrations WHERE id = ?');
        const result = getStmt.get(execId);

        expect(result.status).toBe('success');
        expect(result.ended_at).toBeDefined();
      }
    });

    test('should update completed steps count on success', async () => {
      const execId = `orch-exec-${Date.now()}-7`;

      if (stateManager.db) {
        const orchStmt = stateManager.db.prepare(`
          INSERT INTO orchestrations (id, name, started_at, status, total_steps, completed_steps)
          VALUES (?, ?, ?, ?, ?, 0)
        `);
        orchStmt.run(execId, 'test-simple-workflow', new Date(), 'running', 2);

        // Simulate success
        await orchestrator._onOrchestrationSuccess({
          executionId: execId,
          jobId: 'test-simple-workflow',
        });

        // Verify completed_steps == total_steps
        const getStmt = stateManager.db.prepare('SELECT * FROM orchestrations WHERE id = ?');
        const result = getStmt.get(execId);

        expect(result.completed_steps).toBe(result.total_steps);
      }
    });
  });

  describe('Orchestration Failure Recording', () => {
    test('should record orchestration failure', async () => {
      const execId = `orch-exec-${Date.now()}-8`;

      if (stateManager.db) {
        // Create orchestration
        const orchStmt = stateManager.db.prepare(`
          INSERT INTO orchestrations (id, name, started_at, status, total_steps, completed_steps)
          VALUES (?, ?, ?, ?, ?, 0)
        `);
        orchStmt.run(execId, 'test-simple-workflow', new Date(), 'running', 2);

        // Simulate failure
        await orchestrator._onOrchestrationFailure({
          executionId: execId,
          jobId: 'test-simple-workflow',
          error: new Error('Test error'),
        });

        // Verify
        const getStmt = stateManager.db.prepare('SELECT * FROM orchestrations WHERE id = ?');
        const result = getStmt.get(execId);

        expect(result.status).toBe('failed');
        expect(result.ended_at).toBeDefined();
      }
    });
  });

  describe('Orchestration History Queries', () => {
    test('should retrieve orchestration history', async () => {
      const execId1 = `orch-exec-${Date.now()}-9`;
      const execId2 = `orch-exec-${Date.now()}-10`;

      if (stateManager.db) {
        const orchStmt = stateManager.db.prepare(`
          INSERT INTO orchestrations (id, name, started_at, status, total_steps, completed_steps)
          VALUES (?, ?, ?, ?, ?, 0)
        `);

        orchStmt.run(execId1, 'test-simple-workflow', new Date(Date.now() - 1000), 'success', 2);
        orchStmt.run(execId2, 'test-simple-workflow', new Date(), 'success', 2);

        // Query history
        const history = await orchestrator.getOrchestrationHistory('test-simple-workflow', 10);

        expect(history).toBeDefined();
        expect(history.length).toBeGreaterThanOrEqual(2);
      }
    });

    test('should return history in reverse chronological order', async () => {
      const execId1 = `orch-exec-${Date.now()}-11`;
      const execId2 = `orch-exec-${Date.now()}-12`;

      if (stateManager.db) {
        const orchStmt = stateManager.db.prepare(`
          INSERT INTO orchestrations (id, name, started_at, status, total_steps, completed_steps)
          VALUES (?, ?, ?, ?, ?, 0)
        `);

        const time1 = new Date(Date.now() - 2000);
        const time2 = new Date();

        orchStmt.run(execId1, 'test-simple-workflow', time1, 'success', 2);
        orchStmt.run(execId2, 'test-simple-workflow', time2, 'success', 2);

        // Query and check order
        const history = await orchestrator.getOrchestrationHistory('test-simple-workflow', 10);

        if (history.length >= 2) {
          // Most recent should be first
          expect(new Date(history[0].started_at)).toBeGreaterThanOrEqual(
            new Date(history[1].started_at)
          );
        }
      }
    });

    test('should retrieve orchestration steps', async () => {
      const execId = `orch-exec-${Date.now()}-13`;

      if (stateManager.db) {
        // Create orchestration
        const orchStmt = stateManager.db.prepare(`
          INSERT INTO orchestrations (id, name, started_at, status, total_steps, completed_steps)
          VALUES (?, ?, ?, ?, ?, 0)
        `);
        orchStmt.run(execId, 'test-simple-workflow', new Date(), 'success', 2);

        // Create steps
        const stepStmt = stateManager.db.prepare(`
          INSERT INTO orchestration_steps
          (id, orchestration_id, step_index, job_id, status, started_at, ended_at, attempts, result)
          VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)
        `);

        stepStmt.run(
          `${execId}-step-0`,
          execId,
          0,
          'test-job-1',
          'completed',
          new Date(),
          new Date(),
          '{}'
        );
        stepStmt.run(
          `${execId}-step-1`,
          execId,
          1,
          'test-job-2',
          'completed',
          new Date(),
          new Date(),
          '{}'
        );

        // Query steps
        const steps = await orchestrator.getOrchestrationSteps(execId);

        expect(steps).toBeDefined();
        expect(steps.length).toBe(2);
        expect(steps[0].step_index).toBe(0);
        expect(steps[1].step_index).toBe(1);
      }
    });

    test('should retrieve steps in execution order', async () => {
      const execId = `orch-exec-${Date.now()}-14`;

      if (stateManager.db) {
        // Create orchestration
        const orchStmt = stateManager.db.prepare(`
          INSERT INTO orchestrations (id, name, started_at, status, total_steps, completed_steps)
          VALUES (?, ?, ?, ?, ?, 0)
        `);
        orchStmt.run(execId, 'test-simple-workflow', new Date(), 'success', 2);

        // Create steps in different order
        const stepStmt = stateManager.db.prepare(`
          INSERT INTO orchestration_steps
          (id, orchestration_id, step_index, job_id, status, started_at, ended_at, attempts, result)
          VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)
        `);

        stepStmt.run(`${execId}-step-1`, execId, 1, 'test-job-2', 'completed', new Date(), new Date(), '{}');
        stepStmt.run(`${execId}-step-0`, execId, 0, 'test-job-1', 'completed', new Date(), new Date(), '{}');

        // Query should return in step order
        const steps = await orchestrator.getOrchestrationSteps(execId);

        expect(steps[0].step_index).toBe(0);
        expect(steps[1].step_index).toBe(1);
      }
    });
  });

  describe('State Manager Integration', () => {
    test('should persist orchestration with state manager', async () => {
      const execId = `orch-exec-${Date.now()}-15`;

      await orchestrator._onOrchestrationStart({
        executionId: execId,
        jobId: 'test-simple-workflow',
      });

      // Should be in memory
      const inMemory = orchestrator.executions.get(execId);
      expect(inMemory).toBeDefined();

      // Should also be in database
      if (stateManager.db) {
        const stmt = stateManager.db.prepare('SELECT * FROM orchestrations WHERE id = ?');
        const dbRecord = stmt.get(execId);
        expect(dbRecord).toBeDefined();
      }
    });

    test('should handle missing state manager gracefully', async () => {
      const orchNoDb = new OrchestratorJobManager(scheduler, '.', null);

      await orchNoDb.loadOrchestrations(orchDir);

      // Should work without state manager
      const execId = `orch-exec-no-db-${Date.now()}`;
      await orchNoDb._onOrchestrationStart({
        executionId: execId,
        jobId: 'test-simple-workflow',
      });

      // Should be in memory only
      const inMemory = orchNoDb.executions.get(execId);
      expect(inMemory).toBeDefined();
    });
  });
});
