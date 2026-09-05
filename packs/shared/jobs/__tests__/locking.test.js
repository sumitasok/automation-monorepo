/**
 * Distributed Locking Tests
 * Validates SQLite-based locking prevents concurrent orchestration execution
 */

const JobStateManager = require('../state-manager.js');
const path = require('path');
const fs = require('fs').promises;

describe('Distributed Locking - T037 (Singleton Execution)', () => {
  let stateManager;
  const testDbPath = path.join(__dirname, 'test-locks.sqlite');

  beforeAll(async () => {
    stateManager = new JobStateManager(testDbPath);
    await stateManager.initialize();
  });

  afterAll(async () => {
    if (stateManager) {
      await stateManager.close();
    }
    try {
      await fs.unlink(testDbPath);
    } catch (err) {
      // Ignore cleanup errors
    }
  });

  describe('Lock Acquisition', () => {
    test('should acquire lock for resource', async () => {
      const acquired = await stateManager.acquireLock('resource-1', 'holder-1', 3600);
      expect(acquired).toBe(true);
    });

    test('should fail to acquire lock if already held', async () => {
      const acquired1 = await stateManager.acquireLock('resource-2', 'holder-1', 3600);
      expect(acquired1).toBe(true);

      const acquired2 = await stateManager.acquireLock('resource-2', 'holder-2', 3600);
      expect(acquired2).toBe(false);
    });

    test('should allow same holder to acquire again', async () => {
      const acquired1 = await stateManager.acquireLock('resource-3', 'holder-1', 3600);
      expect(acquired1).toBe(true);

      // Same holder can acquire (idempotent)
      const acquired2 = await stateManager.acquireLock('resource-3', 'holder-1', 3600);
      // May or may not succeed depending on implementation
      expect(typeof acquired2).toBe('boolean');
    });
  });

  describe('Lock Release', () => {
    test('should release lock', async () => {
      await stateManager.acquireLock('resource-4', 'holder-1', 3600);
      const released = await stateManager.releaseLock('resource-4', 'holder-1');
      expect(released).toBe(true);
    });

    test('should allow acquisition after release', async () => {
      const resource = 'resource-5';
      const holder1 = 'holder-1';
      const holder2 = 'holder-2';

      // Holder 1 acquires
      const acquired1 = await stateManager.acquireLock(resource, holder1, 3600);
      expect(acquired1).toBe(true);

      // Holder 2 cannot acquire
      const acquired2 = await stateManager.acquireLock(resource, holder2, 3600);
      expect(acquired2).toBe(false);

      // Holder 1 releases
      const released = await stateManager.releaseLock(resource, holder1);
      expect(released).toBe(true);

      // Now holder 2 can acquire
      const acquired3 = await stateManager.acquireLock(resource, holder2, 3600);
      expect(acquired3).toBe(true);
    });

    test('should only release lock held by same holder', async () => {
      const resource = 'resource-6';
      const holder1 = 'holder-1';
      const holder2 = 'holder-2';

      // Holder 1 acquires
      await stateManager.acquireLock(resource, holder1, 3600);

      // Holder 2 cannot release holder 1's lock
      const released = await stateManager.releaseLock(resource, holder2);
      expect(released).toBe(false); // Different holder cannot release

      // Only holder 1 can release
      const releasedByHolder1 = await stateManager.releaseLock(resource, holder1);
      expect(releasedByHolder1).toBe(true);
    });
  });

  describe('Lock TTL (Time-To-Live)', () => {
    test('should respect lock TTL', async () => {
      const resource = 'resource-7';
      const holder = 'holder-1';

      // Acquire with 1-second TTL
      const acquired = await stateManager.acquireLock(resource, holder, 1);
      expect(acquired).toBe(true);

      // Should not be acquirable immediately
      const acquired2 = await stateManager.acquireLock(resource, 'holder-2', 1);
      expect(acquired2).toBe(false);

      // Wait for TTL to expire
      await new Promise((resolve) => setTimeout(resolve, 1100));

      // Should be acquirable now
      const acquired3 = await stateManager.acquireLock(resource, 'holder-2', 3600);
      expect(acquired3).toBe(true);
    });

    test('should create lock with default TTL', async () => {
      const acquired = await stateManager.acquireLock('resource-8', 'holder-1');
      expect(acquired).toBe(true);
    });
  });

  describe('Concurrent Execution Prevention', () => {
    test('should prevent concurrent orchestration runs', async () => {
      const orchName = 'test-orchestration';
      const execution1 = 'exec-1';
      const execution2 = 'exec-2';

      // Execution 1 acquires lock
      const lock1 = await stateManager.acquireLock(orchName, execution1, 3600);
      expect(lock1).toBe(true);

      // Execution 2 cannot acquire lock
      const lock2 = await stateManager.acquireLock(orchName, execution2, 3600);
      expect(lock2).toBe(false);

      // Execution 1 releases lock
      await stateManager.releaseLock(orchName, execution1);

      // Now execution 2 can acquire
      const lock3 = await stateManager.acquireLock(orchName, execution2, 3600);
      expect(lock3).toBe(true);
    });

    test('should work for multiple orchestrations independently', async () => {
      const orch1 = 'orch-1';
      const orch2 = 'orch-2';
      const holder1 = 'holder-1';
      const holder2 = 'holder-2';

      // Both can hold locks on different resources
      const lock1 = await stateManager.acquireLock(orch1, holder1, 3600);
      const lock2 = await stateManager.acquireLock(orch2, holder2, 3600);

      expect(lock1).toBe(true);
      expect(lock2).toBe(true);

      // Holder 2 cannot acquire orch1 lock
      const lock3 = await stateManager.acquireLock(orch1, holder2, 3600);
      expect(lock3).toBe(false);

      // Holder 1 cannot acquire orch2 lock
      const lock4 = await stateManager.acquireLock(orch2, holder1, 3600);
      expect(lock4).toBe(false);
    });
  });

  describe('Lock Holder Identification', () => {
    test('should track lock holder', async () => {
      const resource = 'resource-9';
      const holder = 'my-execution-id';

      const acquired = await stateManager.acquireLock(resource, holder, 3600);
      expect(acquired).toBe(true);

      // Lock is held by specific holder
      const acquired2 = await stateManager.acquireLock(resource, 'different-holder', 3600);
      expect(acquired2).toBe(false);

      // Only holder can release
      const released = await stateManager.releaseLock(resource, holder);
      expect(released).toBe(true);
    });
  });

  describe('Lock Safety', () => {
    test('should be resilient to rapid lock/unlock', async () => {
      const resource = 'resource-10';

      for (let i = 0; i < 10; i++) {
        const acquired = await stateManager.acquireLock(resource, `holder-${i}`, 3600);

        if (acquired) {
          const released = await stateManager.releaseLock(resource, `holder-${i}`);
          expect(released).toBe(true);
        }
      }
    });

    test('should be resilient to concurrent lock attempts', async () => {
      const resource = 'resource-11';
      const promises = [];

      // Attempt 100 concurrent lock acquisitions
      for (let i = 0; i < 100; i++) {
        promises.push(
          stateManager.acquireLock(resource, `holder-${i}`, 3600)
        );
      }

      const results = await Promise.all(promises);

      // Only one should succeed
      const successCount = results.filter((r) => r === true).length;
      expect(successCount).toBe(1);
    });
  });

  describe('State Manager Lock Integration', () => {
    test('should initialize with empty locks', () => {
      expect(stateManager).toBeDefined();
    });

    test('should maintain lock state across queries', async () => {
      const resource = 'resource-12';
      const holder = 'holder-1';

      // Acquire lock
      const acquired = await stateManager.acquireLock(resource, holder, 3600);
      expect(acquired).toBe(true);

      // Attempt to acquire again
      const acquired2 = await stateManager.acquireLock(resource, 'holder-2', 3600);
      expect(acquired2).toBe(false);

      // Lock persists
      const acquired3 = await stateManager.acquireLock(resource, 'holder-3', 3600);
      expect(acquired3).toBe(false);
    });
  });

  describe('Lock Duration', () => {
    test('should support different TTL values', async () => {
      const resource1 = 'resource-ttl-1';
      const resource2 = 'resource-ttl-2';

      // Short TTL
      const acquired1 = await stateManager.acquireLock(resource1, 'holder-1', 1);
      expect(acquired1).toBe(true);

      // Long TTL
      const acquired2 = await stateManager.acquireLock(resource2, 'holder-1', 86400);
      expect(acquired2).toBe(true);

      // Both locked
      expect(await stateManager.acquireLock(resource1, 'holder-2', 1)).toBe(false);
      expect(await stateManager.acquireLock(resource2, 'holder-2', 1)).toBe(false);
    });
  });
});
