#!/usr/bin/env node
/**
 * Manual Job Execution Test
 * Verifies framework scheduler executes jobs correctly
 */

const ExpenseServer = require('./packs/expense-domain/engine/server.js');
const ExpenseDomainJobManager = require('./packs/expense-domain/engine/job-integration.js');

const CONFIG_PATH = process.env.CONFIG_PATH || process.env.HOME + '/automation-monorepo-config';

async function testJobExecution() {
  console.log('🧪 Testing Job Execution Framework...\n');

  try {
    // Initialize engine and job manager
    const server = new ExpenseServer(CONFIG_PATH, 3100);
    const engine = server.engine;

    console.log('📦 Initializing engine...');
    await engine.initialize();
    await engine.start();

    console.log('📋 Creating job manager...');
    const jobManager = new ExpenseDomainJobManager(engine, CONFIG_PATH);
    await jobManager.registerJobs();
    jobManager.scheduler.isRunning = true;

    // Test 1: Job Registration
    console.log('\n✅ Test 1: Job Registration');
    const jobs = jobManager.scheduler.getAllJobs();
    console.log(`   Registered: ${jobs.length} jobs`);
    console.log(`   Jobs: ${jobs.map((j) => j.id).join(', ')}`);
    if (jobs.length !== 5) {
      throw new Error(`Expected 5 jobs, got ${jobs.length}`);
    }

    // Test 2: Trigger Jobs and Verify Execution
    console.log('\n✅ Test 2: Job Execution');
    const jobIds = [
      'gmail-fetch-job',
      'wallet-fetch-job',
      'bank-csv-monitor-job',
      'process-transactions-job',
      'learn-rules-job',
    ];

    for (const jobId of jobIds) {
      const executionId = await jobManager.triggerJob(jobId);
      console.log(`   Triggered ${jobId}: execution ${executionId}`);

      const execution = jobManager.scheduler.getExecution(executionId);
      if (!execution) {
        throw new Error(`Execution ${executionId} not found`);
      }

      if (execution.status !== 'success') {
        throw new Error(`${jobId} failed: ${execution.lastError || execution.status}`);
      }

      console.log(`   ✓ ${jobId} completed successfully`);
    }

    // Test 3: Verify Execution History
    console.log('\n✅ Test 3: Execution History');
    const history = jobManager.getExecutionHistory({ jobId: 'gmail-fetch-job' });
    console.log(`   Gmail Fetch Job executions: ${history.length}`);
    if (history.length === 0) {
      throw new Error('No execution history found');
    }

    // Test 4: Verify Retry Configuration
    console.log('\n✅ Test 4: Retry Configuration');
    const job = jobManager.scheduler.getJob('gmail-fetch-job');
    console.log(`   Max retries: ${job.retry.maxRetries}`);
    console.log(`   Backoff multiplier: ${job.retry.backoffMultiplier}`);
    if (job.retry.maxRetries !== 3) {
      throw new Error(`Expected 3 max retries, got ${job.retry.maxRetries}`);
    }

    // Test 5: Verify Schedule Configuration
    console.log('\n✅ Test 5: Schedule Configuration');
    const gmailJob = jobManager.scheduler.getJob('gmail-fetch-job');
    const walletJob = jobManager.scheduler.getJob('wallet-fetch-job');
    const csvJob = jobManager.scheduler.getJob('bank-csv-monitor-job');

    console.log(`   Gmail: ${gmailJob.schedule.interval} (${gmailJob.timeout}s timeout)`);
    console.log(`   Wallet: ${walletJob.schedule.interval} (${walletJob.timeout}s timeout)`);
    console.log(`   CSV: ${csvJob.schedule.interval} (${csvJob.timeout}s timeout)`);

    if (gmailJob.schedule.interval !== '1d') {
      throw new Error(`Expected 1d interval for gmail, got ${gmailJob.schedule.interval}`);
    }

    console.log('\n✨ All tests passed!\n');
    process.exit(0);
  } catch (error) {
    console.error('\n❌ Test failed:', error.message);
    console.error(error.stack);
    process.exit(1);
  }
}

testJobExecution();
