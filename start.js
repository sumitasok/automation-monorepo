#!/usr/bin/env node
/**
 * Framework Startup Script
 * Starts the Expense Domain Server with framework job scheduling
 */

const path = require('path');
const ExpenseServer = require('./packs/expense-domain/engine/server.js');

const configPath = process.env.CONFIG_PATH || path.join(process.env.HOME, 'automation-monorepo-config');
const port = process.env.PORT || 3100;

console.log('');
console.log('═══════════════════════════════════════════════════════════════');
console.log('🚀 AUTOMATION FRAMEWORK STARTUP');
console.log('═══════════════════════════════════════════════════════════════');
console.log('');
console.log(`📁 Config Path: ${configPath}`);
console.log(`🔌 Port: ${port}`);
console.log('');

const server = new ExpenseServer(configPath, port);

server.start()
  .then(() => {
    console.log('');
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('✅ FRAMEWORK READY');
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('');
    console.log('📋 API Endpoints:');
    console.log(`   Health: http://localhost:${port}/health`);
    console.log(`   Orchestrations: http://localhost:${port}/api/orchestrations`);
    console.log(`   Job Stats: http://localhost:${port}/api/jobs/{jobId}/stats`);
    console.log('');
    console.log('🔄 Scheduled Jobs:');
    console.log('   • gmail-fetch-job (daily)');
    console.log('   • wallet-fetch-job (hourly)');
    console.log('   • bank-csv-monitor-job (30s)');
    console.log('   • process-transactions-job (5m)');
    console.log('   • learn-rules-job (daily)');
    console.log('   • wallet-sync-orchestration (4h)');
    console.log('');
    console.log('Press Ctrl+C to stop');
    console.log('');
  })
  .catch((err) => {
    console.error('');
    console.error('❌ STARTUP FAILED');
    console.error('═══════════════════════════════════════════════════════════════');
    console.error(`Error: ${err.message}`);
    console.error('');
    if (err.stack) {
      console.error('Stack:', err.stack);
    }
    process.exit(1);
  });

// Graceful shutdown
process.on('SIGINT', async () => {
  console.log('');
  console.log('⏹️  Shutting down gracefully...');
  try {
    await server.stop();
    console.log('✅ Framework stopped');
    process.exit(0);
  } catch (err) {
    console.error('❌ Error during shutdown:', err.message);
    process.exit(1);
  }
});

process.on('SIGTERM', async () => {
  console.log('');
  console.log('⏹️  Received SIGTERM, shutting down...');
  try {
    await server.stop();
    console.log('✅ Framework stopped');
    process.exit(0);
  } catch (err) {
    console.error('❌ Error during shutdown:', err.message);
    process.exit(1);
  }
});

// Keep process alive indefinitely
setInterval(() => {}, 1000 * 60 * 60); // Check every hour
