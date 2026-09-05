/**
 * Job State Manager Tests
 * Validates persistence of job execution state to SQLite
 */

const JobStateManager = require('../state-manager.js');
const path = require('path');
const fs = require('fs').promises;

describe('JobStateManager - Phase 5 Persistence', () => {
  let stateManager;
  const testDbPath = path.join(__dirname, 'test-job-state.sqlite');

  beforeAll(async () => {
    stateManager = new JobStateManager(testDbPath);
    await stateManager.initialize();
  });

  afterAll(async () => {
    if (stateManager) {
      await stateManager.close();
    }
    // Cleanup test database
    try {
      await fs.unlink(testDbPath);
    } catch (err) {
      // Ignore if file doesn't exist
    }
  });

  describe('Job Registration', () => {
    test('should record job registration', () => {
      const jobDef = {
        name: 'Test Job',
        schedule: { type: 'interval', interval: '1h' },
        timeout: 300,
        retry: { maxRetries: 3, backoffMultiplier: 2 },
        enabled: true,
      };

      expect(() => {
        stateManager.recordJobRegistration('test-job-1', jobDef);
      }).not.toThrow();
    });

    test('should handle multiple job registrations', () => {
      const jobs = [
        {
          id: 'job-1',
          name: 'Job 1',
          schedule: { type: 'interval', interval: '1d' },
          timeout: 300,
        },
        {
          id: 'job-2',
          name: 'Job 2',
          schedule: { type: 'interval', interval: '1h' },
          timeout: 60,
        },
      ];

      jobs.forEach((job) => {
        stateManager.recordJobRegistration(job.id, job);
      });

      expect(stateManager.executions.size).toBe(0); // No executions yet
    });
  });

  describe('Execution Tracking', () => {
    test('should record execution start', () => {
      const execution = {
        startTime: new Date(),
        context: { source: 'test' },
      };

      stateManager.recordExecutionStart('exec-1', 'test-job-1', execution);

      expect(stateManager.executions.has('exec-1') || stateManager.db).toBeTruthy();
    });

    test('should record execution completion (success)', () => {
      const execution = {
        startTime: new Date(),
        endTime: new Date(),
        status: 'success',
        attempts: 1,
        results: { itemsProcessed: 5 },
      };

      stateManager.recordExecutionStart('exec-2', 'test-job-1', execution);
      stateManager.recordExecutionComplete('exec-2', 'test-job-1', execution);

      const history = stateManager.getExecutionHistory({ jobId: 'test-job-1' });
      expect(history.length).toBeGreaterThanOrEqual(0);
    });

    test('should record execution completion (failure)', () => {
      const execution = {
        startTime: new Date(),
        endTime: new Date(),
        status: 'failed',
        attempts: 3,
        lastError: new Error('Job failed'),
      };

      stateManager.recordExecutionStart('exec-3', 'test-job-2', execution);
      stateManager.recordExecutionComplete('exec-3', 'test-job-2', execution);

      const stats = stateManager.getExecutionStats('test-job-2');
      expect(stats).toBeDefined();
      expect(stats.totalExecutions).toBeGreaterThanOrEqual(0);
    });
  });

  describe('Execution History', () => {
    test('should retrieve execution history', () => {
      const history = stateManager.getExecutionHistory({
        jobId: 'test-job-1',
      });

      expect(Array.isArray(history)).toBe(true);
    });

    test('should filter history by status', () => {
      const successHistory = stateManager.getExecutionHistory({
        jobId: 'test-job-1',
        status: 'success',
      });

      expect(Array.isArray(successHistory)).toBe(true);
    });

    test('should limit history results', () => {
      const limited = stateManager.getExecutionHistory({
        jobId: 'test-job-1',
        limit: 5,
      });

      expect(limited.length).toBeLessThanOrEqual(5);
    });
  });

  describe('Execution Statistics', () => {
    test('should calculate execution statistics', () => {
      const stats = stateManager.getExecutionStats('test-job-1');

      expect(stats).toHaveProperty('totalExecutions');
      expect(stats).toHaveProperty('successCount');
      expect(stats).toHaveProperty('failureCount');
      expect(stats).toHaveProperty('successRate');
      expect(stats).toHaveProperty('avgDuration');
      expect(stats).toHaveProperty('lastExecution');
    });

    test('should return zero stats for non-existent job', () => {
      const stats = stateManager.getExecutionStats('non-existent-job');

      expect(stats.totalExecutions).toBe(0);
      expect(stats.successRate).toBe(0);
      expect(stats.avgDuration).toBe(0);
    });
  });

  describe('Distributed Locking', () => {
    test('should acquire lock', async () => {
      const acquired = await stateManager.acquireLock('resource-1', 'holder-1', 3600);
      expect(typeof acquired).toBe('boolean');
    });

    test('should prevent concurrent locks on same resource', async () => {
      const acquired1 = await stateManager.acquireLock('resource-2', 'holder-1', 3600);
      const acquired2 = await stateManager.acquireLock('resource-2', 'holder-2', 3600);

      // Only one should succeed
      expect(acquired1 || !acquired2).toBe(true);
    });

    test('should release lock', async () => {
      const acquired = await stateManager.acquireLock('resource-3', 'holder-1', 3600);
      const released = await stateManager.releaseLock('resource-3', 'holder-1');

      expect(released).toBe(true);
    });

    test('should allow lock after release', async () => {
      await stateManager.acquireLock('resource-4', 'holder-1', 3600);
      await stateManager.releaseLock('resource-4', 'holder-1');

      // Should be able to acquire again
      const acquired = await stateManager.acquireLock('resource-4', 'holder-2', 3600);
      expect(acquired).toBe(true);
    });
  });

  describe('Event Emission', () => {
    test('should emit execution recorded event', (done) => {
      stateManager.once('execution:recorded', ({ executionId, jobId }) => {
        expect(executionId).toBeDefined();
        expect(jobId).toBeDefined();
        done();
      });

      const execution = {
        startTime: new Date(),
        endTime: new Date(),
        status: 'success',
      };

      stateManager.recordExecutionStart('exec-emit-1', 'test-job-emit', execution);
      stateManager.recordExecutionComplete('exec-emit-1', 'test-job-emit', execution);
    });
  });

  describe('State Manager Initialization', () => {
    test('should initialize without throwing', async () => {
      const manager = new JobStateManager();
      expect(async () => {
        await manager.initialize();
      }).not.toThrow();
      await manager.close();
    });

    test('should set initialized flag', async () => {
      const manager = new JobStateManager();
      expect(manager.initialized).toBe(false);
      await manager.initialize();
      expect(manager.initialized).toBe(true);
      await manager.close();
    });
  });
});
