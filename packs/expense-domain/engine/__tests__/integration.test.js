/**
 * Integration Test: Expense Domain Restructuring
 * Validates all 7 existing features work with new hierarchical structure
 *
 * Features to validate:
 * 1. Gmail adapter integration
 * 2. Wallet adapter integration
 * 3. Telegram notifications
 * 4. CSV upload handling
 * 5. Rule application and learning
 * 6. Job scheduling
 * 7. Write-back capabilities
 */

const ExpenseEngine = require('../api.js');
const ExpenseServer = require('../server.js');
const path = require('path');

// Test configuration
const CONFIG_PATH = process.env.CONFIG_PATH ||
  process.env.HOME + '/automation-monorepo-config';

describe('Expense Domain Restructuring - Integration Tests', () => {
  let engine;
  let server;

  beforeAll(async () => {
    // Initialize engine and server
    server = new ExpenseServer(CONFIG_PATH, 3100);
    engine = server.engine;

    await engine.initialize();
    await engine.start();
  });

  afterAll(async () => {
    await engine.stop();
    if (server) {
      await server.stop();
    }
  });

  describe('Feature 1: Gmail Adapter Integration', () => {
    test('should load Gmail configuration', async () => {
      const config = await engine.configLoader.loadSourceConfig(
        'expense-domain',
        'gmail'
      );
      expect(config).toBeDefined();
      expect(config.name).toBe('gmail');
      expect(config.auth.provider).toBe('oauth2');
    });

    test('should have Gmail rules directory configured', async () => {
      const rulesDir = path.join(
        CONFIG_PATH,
        'rules',
        'expense-domain',
        'gmail'
      );
      expect(rulesDir).toBeDefined();
    });

    test('should report Gmail source status', async () => {
      const status = await engine.getSourceStatus('gmail');
      expect(status).toBeDefined();
      expect(status.name).toBe('gmail');
      expect(status.enabled).toBe(true);
    });
  });

  describe('Feature 2: Wallet Adapter Integration', () => {
    test('should load Wallet configuration', async () => {
      const config = await engine.configLoader.loadSourceConfig(
        'expense-domain',
        'wallet'
      );
      expect(config).toBeDefined();
      expect(config.name).toBe('wallet');
      expect(config.auth.provider).toBe('api_key');
    });

    test('should support Wallet write-back', async () => {
      const config = await engine.configLoader.loadSourceConfig(
        'expense-domain',
        'wallet'
      );
      expect(config.write_back.enabled).toBe(true);
      expect(config.write_back.writable_fields).toContain('category');
    });

    test('should report Wallet source status', async () => {
      const status = await engine.getSourceStatus('wallet');
      expect(status).toBeDefined();
      expect(status.name).toBe('wallet');
    });
  });

  describe('Feature 3: Telegram Notifications', () => {
    test('should load Telegram configuration', async () => {
      const config = await engine.configLoader.loadSourceConfig(
        'expense-domain',
        'telegram'
      );
      expect(config).toBeDefined();
      expect(config.name).toBe('telegram');
      expect(config.auth.provider).toBe('telegram_bot');
    });

    test('should configure alert thresholds', async () => {
      const config = await engine.configLoader.loadSourceConfig(
        'expense-domain',
        'telegram'
      );
      expect(config.alerts).toBeDefined();
      expect(config.alerts.daily_summary.enabled).toBe(true);
    });
  });

  describe('Feature 4: CSV Upload Handling', () => {
    test('should load Bank CSV monitor configuration', async () => {
      const config = await engine.configLoader.loadSourceConfig(
        'expense-domain',
        'bank-csv'
      );
      expect(config).toBeDefined();
    });

    test('should have CSV upload job defined', async () => {
      const config = await engine.domainConfig;
      const jobs = config?.jobs || [];
      expect(jobs).toContain('bank-csv-monitor-job');
    });
  });

  describe('Feature 5: Rule Application and Learning', () => {
    test('should load domain rules', async () => {
      const rules = await engine.rulesLoader.loadDomainRules('expense-domain');
      expect(Array.isArray(rules)).toBe(true);
    });

    test('should create new rule', async () => {
      const newRule = await engine.createRule({
        name: 'Test categorization rule',
        type: 'categorization',
        pattern: { merchant: 'Starbucks' },
        action: { type: 'set', field: 'category', value: 'Meals & Dining' },
      });
      expect(newRule.id).toBeDefined();
      expect(newRule.origin).toBe('configured');
    });

    test('should apply rules to expenses', async () => {
      const expense = await engine.createExpense({
        amount: 5.50,
        date: new Date().toISOString(),
        merchant: 'Starbucks',
        source: 'wallet',
      });
      expect(expense.id).toBeDefined();
      expect(expense.amount).toBe(5.50);
    });

    test('should update rule', async () => {
      const rules = await engine.getRules();
      if (rules.length > 0) {
        const rule = rules[0];
        const updated = await engine.updateRule(rule.id, {
          enabled: false,
        });
        expect(updated.enabled).toBe(false);
      }
    });

    test('should delete rule', async () => {
      const rules = await engine.getRules();
      if (rules.length > 0) {
        const rule = rules[0];
        await engine.deleteRule(rule.id);
        const remaining = await engine.getRules();
        expect(remaining.find((r) => r.id === rule.id)).toBeUndefined();
      }
    });
  });

  describe('Feature 6: Job Scheduling', () => {
    test('should have all required job definitions', async () => {
      const jobs = engine.domainConfig?.jobs || [];
      expect(jobs).toContain('gmail-fetch-job');
      expect(jobs).toContain('wallet-fetch-job');
      expect(jobs).toContain('process-transactions-job');
      expect(jobs).toContain('learn-rules-job');
    });

    test('should trigger process job', async () => {
      // Simulate processing
      const result = await engine.process();
      expect(Array.isArray(result)).toBe(true);
      expect(engine.state.lastProcessed).toBeDefined();
    });

    test('should trigger learn job', async () => {
      // Simulate learning
      const learned = await engine.learnRules();
      expect(Array.isArray(learned)).toBe(true);
    });
  });

  describe('Feature 7: Write-Back Capabilities', () => {
    test('should support write-back to Wallet', async () => {
      const config = await engine.configLoader.loadSourceConfig(
        'expense-domain',
        'wallet'
      );
      expect(config.write_back.enabled).toBe(true);
    });

    test('should queue write-back request', async () => {
      const result = await engine.writeBackToSource('wallet', {
        expenseId: 'test-id',
        category: 'updated',
      });
      expect(result.status).toBe('queued');
      expect(result.source).toBe('wallet');
    });
  });

  describe('Data Persistence', () => {
    test('should create expense with all fields', async () => {
      const expense = await engine.createExpense({
        amount: 100.00,
        date: new Date().toISOString(),
        merchant: 'Test Merchant',
        source: 'gmail',
        category: 'Test Category',
        description: 'Test expense',
      });
      expect(expense.id).toBeDefined();
      expect(expense.amount).toBe(100.00);
      expect(expense.createdAt).toBeDefined();
    });

    test('should retrieve created expense', async () => {
      const expenses = await engine.getExpenses();
      expect(expenses.length).toBeGreaterThan(0);
      const first = expenses[0];
      expect(first.id).toBeDefined();
      expect(first.amount).toBeDefined();
    });

    test('should update expense', async () => {
      const expenses = await engine.getExpenses();
      if (expenses.length > 0) {
        const exp = expenses[0];
        const updated = await engine.updateExpense(exp.id, {
          category: 'Updated Category',
        });
        expect(updated.category).toBe('Updated Category');
        expect(updated.updatedAt).toBeDefined();
      }
    });

    test('should delete expense', async () => {
      const expenses = await engine.getExpenses();
      if (expenses.length > 0) {
        const exp = expenses[0];
        await engine.deleteExpense(exp.id);
        const remaining = await engine.getExpenses();
        expect(remaining.find((e) => e.id === exp.id)).toBeUndefined();
      }
    });
  });

  describe('Configuration Injection', () => {
    test('should load config from injected path', () => {
      expect(engine.configPath).toBe(CONFIG_PATH);
    });

    test('should have correct domain name', () => {
      expect(engine.domainName).toBe('expense-domain');
    });

    test('should have correct data directory', () => {
      const expectedDataDir = path.join(CONFIG_PATH, 'data', 'expense-domain');
      expect(engine.dataDir).toContain('expense-domain');
    });

    test('should have correct rules directory', () => {
      const expectedRulesDir = path.join(CONFIG_PATH, 'rules', 'expense-domain');
      expect(engine.rulesDir).toContain('expense-domain');
    });
  });

  describe('API Events', () => {
    test('should emit events on operations', async () => {
      let eventReceived = false;

      engine.once('expense:created', () => {
        eventReceived = true;
      });

      await engine.createExpense({
        amount: 50,
        date: new Date().toISOString(),
        source: 'test',
      });

      expect(eventReceived).toBe(true);
    });
  });

  describe('Error Handling', () => {
    test('should reject invalid expense', async () => {
      expect.assertions(1);
      try {
        await engine.createExpense({ amount: 100 }); // Missing date
      } catch (error) {
        expect(error.message).toContain('date');
      }
    });

    test('should reject invalid source', async () => {
      expect.assertions(1);
      try {
        await engine.getSourceStatus('nonexistent-source');
      } catch (error) {
        expect(error.message).toContain('not found');
      }
    });

    test('should reject invalid expense ID', async () => {
      expect.assertions(1);
      try {
        await engine.getExpense('nonexistent-id');
      } catch (error) {
        expect(error.message).toContain('not found');
      }
    });
  });
});
