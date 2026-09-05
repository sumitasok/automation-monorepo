/**
 * Expense Domain Engine - Entry Point
 * Exports ExpenseEngine API and Server
 */

const ExpenseEngine = require('./api.js');
const ExpenseServer = require('./server.js');

module.exports = {
  ExpenseEngine,
  ExpenseServer,
};

// CLI: Start server if called directly
if (require.main === module) {
  const configPath = process.env.CONFIG_PATH ||
    process.env.HOME + '/automation-monorepo-config';
  const port = process.env.PORT || 3100;

  console.log('Starting Expense Domain Server...');
  console.log(`Config path: ${configPath}`);
  console.log(`Port: ${port}`);

  const server = new ExpenseServer(configPath, port);

  server.start()
    .then(() => {
      console.log(`✓ Expense Domain Server started on port ${port}`);
      console.log(`  API: http://localhost:${port}/api/expense-domain/`);
      console.log(`  Health: http://localhost:${port}/health`);
    })
    .catch((err) => {
      console.error('✗ Failed to start server:', err);
      process.exit(1);
    });

  // Graceful shutdown
  process.on('SIGTERM', async () => {
    console.log('Received SIGTERM, shutting down gracefully...');
    await server.stop();
    console.log('Server stopped');
    process.exit(0);
  });

  process.on('SIGINT', async () => {
    console.log('Received SIGINT, shutting down gracefully...');
    await server.stop();
    console.log('Server stopped');
    process.exit(0);
  });
}
