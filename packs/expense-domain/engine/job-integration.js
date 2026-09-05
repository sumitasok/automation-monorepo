/**
 * Job Scheduler Integration for Expense Domain
 * Wires framework JobScheduler to ExpenseEngine
 * Manages all job lifecycle and execution
 */

const JobScheduler = require('../../shared/jobs/scheduler.js');
const ManifestSchema = require('../../shared/jobs/manifest-schema.js');
const JobStateManager = require('../../shared/jobs/state-manager.js');

class ExpenseDomainJobManager {
  constructor(engine, configPath) {
    this.engine = engine;
    this.configPath = configPath;

    // Phase 5: Initialize state manager for persistence
    this.stateManager = new JobStateManager();

    this.scheduler = new JobScheduler(configPath, {
      executionTimeout: 300,
      maxRetries: 3,
      backoffMultiplier: 2,
      initialDelayMs: 5000,
      stateManager: this.stateManager, // Phase 5: Enable persistence
    });

    this.jobDefinitions = [];
  }

  /**
   * Initialize state manager and scheduler
   */
  async initialize() {
    try {
      await this.stateManager.initialize();
      console.log('✓ Job state manager initialized');
    } catch (error) {
      console.warn('⚠️ Failed to initialize state manager:', error.message);
      // Continue without persistence (Phase 5 fallback)
    }
  }

  /**
   * Register all expense domain jobs with scheduler
   */
  async registerJobs() {
    // Job 1: Gmail Fetch (daily at 2 AM)
    this.scheduler.registerJob('gmail-fetch-job', {
      name: 'Gmail Fetch Job',
      description: 'Fetch new emails from Gmail daily',
      schedule: { type: 'interval', interval: '1d' },
      timeout: 300,
      retry: { maxRetries: 3, backoffMultiplier: 2 },
      enabled: true,
      handlers: {
        onStart: this._onJobStart.bind(this),
        execute: this._executeGmailFetch.bind(this),
        onSuccess: this._onJobSuccess.bind(this),
        onFailure: this._onJobFailure.bind(this),
        onComplete: this._onJobComplete.bind(this),
      },
    });

    // Job 2: Wallet Fetch (hourly)
    this.scheduler.registerJob('wallet-fetch-job', {
      name: 'Wallet Fetch Job',
      description: 'Fetch transactions from Wallet hourly',
      schedule: { type: 'interval', interval: '1h' },
      timeout: 300,
      retry: { maxRetries: 3, backoffMultiplier: 2 },
      enabled: true,
      handlers: {
        onStart: this._onJobStart.bind(this),
        execute: this._executeWalletFetch.bind(this),
        onSuccess: this._onJobSuccess.bind(this),
        onFailure: this._onJobFailure.bind(this),
        onComplete: this._onJobComplete.bind(this),
      },
    });

    // Job 3: Bank CSV Monitor (every 30 seconds)
    this.scheduler.registerJob('bank-csv-monitor-job', {
      name: 'Bank CSV Monitor Job',
      description: 'Monitor for uploaded bank CSV files',
      schedule: { type: 'interval', interval: '30s' },
      timeout: 60,
      retry: { maxRetries: 2, backoffMultiplier: 2 },
      enabled: true,
      handlers: {
        onStart: this._onJobStart.bind(this),
        execute: this._executeBankCsvMonitor.bind(this),
        onSuccess: this._onJobSuccess.bind(this),
        onFailure: this._onJobFailure.bind(this),
        onComplete: this._onJobComplete.bind(this),
      },
    });

    // Job 4: Process Transactions (every 5 minutes)
    this.scheduler.registerJob('process-transactions-job', {
      name: 'Process Transactions Job',
      description: 'Process fetched data through domain engine',
      schedule: { type: 'interval', interval: '5m' },
      timeout: 600,
      retry: { maxRetries: 3, backoffMultiplier: 2 },
      enabled: true,
      handlers: {
        onStart: this._onJobStart.bind(this),
        execute: this._executeProcessTransactions.bind(this),
        onSuccess: this._onJobSuccess.bind(this),
        onFailure: this._onJobFailure.bind(this),
        onComplete: this._onJobComplete.bind(this),
      },
    });

    // Job 5: Learn Rules (daily)
    this.scheduler.registerJob('learn-rules-job', {
      name: 'Learn Rules Job',
      description: 'Learn new rules from transaction patterns',
      schedule: { type: 'interval', interval: '1d' },
      timeout: 1800,
      retry: { maxRetries: 2, backoffMultiplier: 2 },
      enabled: true,
      handlers: {
        onStart: this._onJobStart.bind(this),
        execute: this._executeLearnRules.bind(this),
        onSuccess: this._onJobSuccess.bind(this),
        onFailure: this._onJobFailure.bind(this),
        onComplete: this._onJobComplete.bind(this),
      },
    });

    console.log('✓ All 5 expense domain jobs registered');
  }

  /**
   * Start the scheduler (begins all scheduled jobs)
   */
  async start() {
    // Phase 5: Initialize state manager first
    await this.initialize();
    await this.registerJobs();
    await this.scheduler.start();
    console.log('✓ Job scheduler started');
  }

  /**
   * Stop the scheduler (cancels all pending jobs)
   */
  async stop() {
    await this.scheduler.stop();
    // Phase 5: Close state manager connection
    if (this.stateManager) {
      await this.stateManager.close();
    }
    console.log('✓ Job scheduler stopped');
  }

  /**
   * Trigger a job immediately (manual trigger)
   */
  async triggerJob(jobId, context = {}) {
    return this.scheduler.triggerJob(jobId, context);
  }

  /**
   * Get job execution history
   */
  getExecutionHistory(filters = {}) {
    return this.scheduler.getExecutionHistory(filters);
  }

  /**
   * Get job details
   */
  getJob(jobId) {
    return this.scheduler.getJob(jobId);
  }

  // ============ Job Handler Implementations ============

  async _onJobStart({ executionId, jobId, execution }) {
    console.log(`[${jobId}] Starting execution ${executionId}`);
    this.engine.emit('job:started', { jobId, executionId });
  }

  async _onJobSuccess({ executionId, jobId, execution, result }) {
    console.log(`[${jobId}] Execution ${executionId} succeeded`);
    this.engine.emit('job:succeeded', { jobId, executionId, result });
  }

  async _onJobFailure({ executionId, jobId, execution, error }) {
    console.error(`[${jobId}] Execution ${executionId} failed: ${error.message}`);
    this.engine.emit('job:failed', { jobId, executionId, error });
  }

  async _onJobComplete({ executionId, jobId, execution }) {
    console.log(`[${jobId}] Execution ${executionId} complete`);
    this.engine.emit('job:completed', { jobId, executionId });
  }

  // ============ Job Executors ============

  async _executeGmailFetch({ executionId, jobId, execution }) {
    console.log(`[${jobId}] Fetching emails from Gmail...`);

    try {
      // Simulate Gmail fetch
      const emails = [
        {
          id: 'email-1',
          subject: 'Receipt: Starbucks',
          amount: 5.50,
          date: new Date().toISOString(),
          source: 'gmail',
        },
      ];

      console.log(`[${jobId}] Fetched ${emails.length} emails`);

      // Would call actual Gmail API here
      // const emails = await gmailAdapter.fetch();

      return {
        status: 'success',
        itemsProcessed: emails.length,
        newItems: emails,
      };
    } catch (error) {
      throw new Error(`Gmail fetch failed: ${error.message}`);
    }
  }

  async _executeWalletFetch({ executionId, jobId, execution }) {
    console.log(`[${jobId}] Fetching transactions from Wallet...`);

    try {
      // Simulate Wallet fetch
      const transactions = [
        {
          id: 'wallet-1',
          merchant: 'Target',
          amount: 45.00,
          date: new Date().toISOString(),
          source: 'wallet',
        },
      ];

      console.log(`[${jobId}] Fetched ${transactions.length} transactions`);

      // Would call actual Wallet API here
      // const transactions = await walletAdapter.fetch();

      return {
        status: 'success',
        itemsProcessed: transactions.length,
        newItems: transactions,
      };
    } catch (error) {
      throw new Error(`Wallet fetch failed: ${error.message}`);
    }
  }

  async _executeBankCsvMonitor({ executionId, jobId, execution }) {
    console.log(`[${jobId}] Monitoring for bank CSV uploads...`);

    try {
      // Check for CSV files in upload directory
      const csvFiles = [];

      // Would check actual upload directory here
      // const csvFiles = await checkUploadDirectory();

      if (csvFiles.length > 0) {
        console.log(`[${jobId}] Found ${csvFiles.length} CSV files to process`);

        return {
          status: 'success',
          itemsProcessed: csvFiles.length,
          files: csvFiles,
        };
      } else {
        console.log(`[${jobId}] No CSV files found`);

        return {
          status: 'success',
          itemsProcessed: 0,
          files: [],
        };
      }
    } catch (error) {
      throw new Error(`Bank CSV monitor failed: ${error.message}`);
    }
  }

  async _executeProcessTransactions({ executionId, jobId, execution }) {
    console.log(`[${jobId}] Processing transactions through domain engine...`);

    try {
      const result = await this.engine.process();

      console.log(`[${jobId}] Processed ${result.length} transactions`);

      return {
        status: 'success',
        itemsProcessed: result.length,
        processed: result,
      };
    } catch (error) {
      throw new Error(`Transaction processing failed: ${error.message}`);
    }
  }

  async _executeLearnRules({ executionId, jobId, execution }) {
    console.log(`[${jobId}] Learning rules from transaction patterns...`);

    try {
      const learned = await this.engine.learnRules();

      console.log(`[${jobId}] Learned ${learned.length} new rules`);

      return {
        status: 'success',
        rulesLearned: learned.length,
        rules: learned,
      };
    } catch (error) {
      throw new Error(`Rule learning failed: ${error.message}`);
    }
  }
}

module.exports = ExpenseDomainJobManager;
