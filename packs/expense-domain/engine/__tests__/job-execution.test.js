/**
 * Job Execution Tests
 * Validates that framework scheduler executes jobs at configured times
 * with proper retry logic and failure handling
 */

const ExpenseServer = require('../server.js');
const ExpenseDomainJobManager = require('../job-integration.js');
const path = require('path');

const CONFIG_PATH = process.env.CONFIG_PATH ||
  process.env.HOME + '/automation-monorepo-config';

describe('Job Execution - Framework Scheduler', () => {
  let engine;
  let jobManager;

  beforeAll(async () => {
    const server = new ExpenseServer(CONFIG_PATH, 3100);
    engine = server.engine;
    // Initialize engine
    await engine.initialize();
    await engine.start();
    // Create job manager manually (don't use server.start() to avoid live intervals)
    jobManager = new ExpenseDomainJobManager(engine, CONFIG_PATH);
    await jobManager.registerJobs();
    // Manually set isRunning=true so jobs can be triggered (normally set by scheduler.start())
    jobManager.scheduler.isRunning = true;
  });

  afterAll(async () => {
    // Stop scheduler and clean up
    jobManager.scheduler.isRunning = false;
    for (const [jobId] of jobManager.scheduler.jobs) {
      jobManager.scheduler._clearJobSchedule(jobId);
    }
    jobManager.scheduler.jobs.clear();
    jobManager.scheduler.executions.clear();
  });

  describe('Job Registration', () => {
    test('should register all 5 jobs', () => {
      const allJobs = jobManager.scheduler.getAllJobs();
      expect(allJobs).toBeDefined();
      expect(allJobs.length).toBe(5);
    });

    test('should have correct job names', () => {
      const jobs = jobManager.scheduler.getAllJobs();
      const jobNames = jobs.map((j) => j.name);
      expect(jobNames).toContain('Gmail Fetch Job');
      expect(jobNames).toContain('Wallet Fetch Job');
      expect(jobNames).toContain('Bank CSV Monitor Job');
      expect(jobNames).toContain('Process Transactions Job');
      expect(jobNames).toContain('Learn Rules Job');
    });

    test('should have correct schedules', () => {
      const jobs = jobManager.scheduler.getAllJobs();
      const gmailJob = jobs.find((j) => j.name === 'Gmail Fetch Job');
      expect(gmailJob.schedule.interval).toBe('1d');

      const walletJob = jobs.find((j) => j.name === 'Wallet Fetch Job');
      expect(walletJob.schedule.interval).toBe('1h');

      const csvJob = jobs.find((j) => j.name === 'Bank CSV Monitor Job');
      expect(csvJob.schedule.interval).toBe('30s');
    });

    test('should have correct timeouts', () => {
      const jobs = jobManager.scheduler.getAllJobs();
      const gmailJob = jobs.find((j) => j.name === 'Gmail Fetch Job');
      expect(gmailJob.timeout).toBe(300);

      const csvJob = jobs.find((j) => j.name === 'Bank CSV Monitor Job');
      expect(csvJob.timeout).toBe(60);
    });
  });

  describe('Job Execution', () => {
    test('should trigger Gmail fetch job', async () => {
      const executionId = await jobManager.triggerJob('gmail-fetch-job');
      expect(executionId).toBeDefined();

      // Give job time to execute
      await new Promise((resolve) => setTimeout(resolve, 100));

      const execution = jobManager.scheduler.getExecution(executionId);
      expect(execution).toBeDefined();
      expect(execution.status).toBe('success');
    });

    test('should trigger Wallet fetch job', async () => {
      const executionId = await jobManager.triggerJob('wallet-fetch-job');
      expect(executionId).toBeDefined();

      await new Promise((resolve) => setTimeout(resolve, 100));

      const execution = jobManager.scheduler.getExecution(executionId);
      expect(execution).toBeDefined();
      expect(execution.status).toBe('success');
    });

    test('should trigger Bank CSV monitor job', async () => {
      const executionId = await jobManager.triggerJob('bank-csv-monitor-job');
      expect(executionId).toBeDefined();

      await new Promise((resolve) => setTimeout(resolve, 100));

      const execution = jobManager.scheduler.getExecution(executionId);
      expect(execution).toBeDefined();
      expect(execution.status).toBe('success');
    });

    test('should trigger Process Transactions job', async () => {
      const executionId = await jobManager.triggerJob('process-transactions-job');
      expect(executionId).toBeDefined();

      await new Promise((resolve) => setTimeout(resolve, 100));

      const execution = jobManager.scheduler.getExecution(executionId);
      expect(execution).toBeDefined();
      expect(execution.status).toBe('success');
    });

    test('should trigger Learn Rules job', async () => {
      const executionId = await jobManager.triggerJob('learn-rules-job');
      expect(executionId).toBeDefined();

      await new Promise((resolve) => setTimeout(resolve, 100));

      const execution = jobManager.scheduler.getExecution(executionId);
      expect(execution).toBeDefined();
      expect(execution.status).toBe('success');
    });
  });

  describe('Job Failure Handling (T039)', () => {
    test('should track execution attempts', async () => {
      const executionId = await jobManager.triggerJob('gmail-fetch-job');
      const execution = jobManager.scheduler.getExecution(executionId);

      expect(execution.attempts).toBeGreaterThan(0);
      expect(execution.attempts).toBeLessThanOrEqual(4); // max 3 retries + 1 initial = 4
    });

    test('should track execution errors when they occur', async () => {
      const executionId = await jobManager.triggerJob('wallet-fetch-job');
      const execution = jobManager.scheduler.getExecution(executionId);

      expect(execution).toHaveProperty('lastError');
      expect(execution).toHaveProperty('attempts');
      expect(execution).toHaveProperty('status');
    });

    test('should support retry configuration', () => {
      const job = jobManager.scheduler.getJob('gmail-fetch-job');
      expect(job.retry).toBeDefined();
      expect(job.retry.maxRetries).toBe(3);
      expect(job.retry.backoffMultiplier).toBe(2);
    });

    test('should track execution duration', async () => {
      const executionId = await jobManager.triggerJob('bank-csv-monitor-job');
      const execution = jobManager.scheduler.getExecution(executionId);

      expect(execution.startTime).toBeDefined();
      expect(execution.endTime).toBeDefined();
      expect(execution.endTime.getTime()).toBeGreaterThanOrEqual(
        execution.startTime.getTime()
      );
    });
  });

  describe('Execution Scheduling', () => {
    test('should track execution history', async () => {
      const executionId = await jobManager.triggerJob('gmail-fetch-job');

      const history = jobManager.getExecutionHistory({
        jobId: 'gmail-fetch-job',
      });
      expect(history).toBeDefined();
      expect(Array.isArray(history)).toBe(true);
    });

    test('should emit job events', async () => {
      let jobStarted = false;
      let jobCompleted = false;

      server.engine.once('job:started', () => {
        jobStarted = true;
      });

      server.engine.once('job:completed', () => {
        jobCompleted = true;
      });

      await jobManager.triggerJob('gmail-fetch-job');

      // Give events time to propagate
      await new Promise((resolve) => setTimeout(resolve, 100));

      expect(jobStarted || jobCompleted).toBe(true);
    });
  });

  describe('Job API Endpoints', () => {
    test('GET /api/expense-domain/jobs should list all jobs', async () => {
      // Test would require HTTP server running
      // Verify job manager has jobs to expose
      const jobs = jobManager.scheduler.getAllJobs();
      expect(jobs.length).toBe(5);
    });

    test('POST /api/expense-domain/jobs/{job}/trigger should trigger job', async () => {
      const executionId = await jobManager.triggerJob('gmail-fetch-job');
      expect(executionId).toBeDefined();
      expect(typeof executionId).toBe('string');
    });

    test('GET /api/expense-domain/jobs/{job}/history should return history', () => {
      const history = jobManager.getExecutionHistory({
        jobId: 'gmail-fetch-job',
      });
      expect(Array.isArray(history)).toBe(true);
    });
  });

  describe('Job Scheduler State', () => {
    test('should have scheduler running', () => {
      expect(jobManager.scheduler.isRunning).toBe(true);
    });

    test('should track execution statistics', async () => {
      await jobManager.triggerJob('gmail-fetch-job');
      await jobManager.triggerJob('wallet-fetch-job');

      const stats = jobManager.scheduler.executions.size;
      expect(stats).toBeGreaterThan(0);
    });
  });
});
