/**
 * Expense Domain Engine API
 * REST API for expense tracking domain
 * Inherits from DomainEngine base class
 */

const DomainEngine = require('../../../shared/lib/domain-api.js');
const ConfigLoader = require('../../../shared/lib/config-loader.js');
const RulesLoader = require('../../../shared/lib/rules-loader.js');
const RulesEngine = require('../../../shared/lib/rules-engine.js');
const fs = require('fs').promises;
const path = require('path');

class ExpenseEngine extends DomainEngine {
  constructor(configPath, options = {}) {
    super(configPath, 'expense-domain', options);

    // Initialize loaders
    this.configLoader = new ConfigLoader(configPath);
    this.rulesLoader = new RulesLoader(configPath);
    this.rulesEngine = new RulesEngine(
      path.join(configPath, 'rules', 'expense-domain')
    );

    // Data store (in-memory for now, should use database)
    this.expenses = new Map(); // id -> expense
    this.expenseId = 0;
  }

  /**
   * Initialize the expense engine
   */
  async initialize() {
    if (this.state.initialized) {
      return;
    }

    this.emit('expense-engine:initializing');

    try {
      // Load domain configuration
      this.domainConfig = await this.configLoader.loadDomainConfig('expense-domain');
      this.emit('expense-engine:config-loaded', { config: this.domainConfig });

      // Load all source configurations
      this.sourceConfigs = await this.configLoader.loadAllSourceConfigs(
        'expense-domain'
      );
      this.emit('expense-engine:sources-loaded', {
        sourceCount: Object.keys(this.sourceConfigs).length,
      });

      // Load rules
      const rules = await this.rulesLoader.loadDomainRules('expense-domain');
      this.rulesEngine.loadRules(rules);
      this.emit('expense-engine:rules-loaded', { ruleCount: rules.length });

      // Load existing expenses from data directory
      await this._loadExpenses();

      this.state.initialized = true;
      this.emit('expense-engine:initialized');
    } catch (error) {
      this.emit('expense-engine:initialization-failed', { error });
      throw error;
    }
  }

  /**
   * GET /api/expense-domain/expenses
   * Retrieve all expenses with optional filtering
   */
  async getExpenses(filters = {}) {
    this._assertRunning();

    let expenses = Array.from(this.expenses.values());

    // Apply filters
    if (filters.startDate) {
      expenses = expenses.filter((e) => new Date(e.date) >= new Date(filters.startDate));
    }
    if (filters.endDate) {
      expenses = expenses.filter((e) => new Date(e.date) <= new Date(filters.endDate));
    }
    if (filters.category) {
      expenses = expenses.filter((e) => e.category === filters.category);
    }
    if (filters.source) {
      expenses = expenses.filter((e) => e.source === filters.source);
    }
    if (filters.minAmount) {
      expenses = expenses.filter((e) => e.amount >= parseFloat(filters.minAmount));
    }
    if (filters.maxAmount) {
      expenses = expenses.filter((e) => e.amount <= parseFloat(filters.maxAmount));
    }

    return expenses.sort((a, b) => new Date(b.date) - new Date(a.date));
  }

  /**
   * GET /api/expense-domain/expenses/{id}
   * Retrieve a specific expense
   */
  async getExpense(id) {
    this._assertRunning();

    const expense = this.expenses.get(id);
    if (!expense) {
      throw new Error(`Expense ${id} not found`);
    }

    return expense;
  }

  /**
   * POST /api/expense-domain/expenses
   * Create a new expense
   */
  async createExpense(expenseData) {
    this._assertRunning();

    // Validate required fields
    if (!expenseData.amount) {
      throw new Error('amount is required');
    }
    if (!expenseData.date) {
      throw new Error('date is required');
    }

    const expense = {
      id: String(++this.expenseId),
      ...expenseData,
      createdAt: new Date(),
      updatedAt: new Date(),
      rules_applied: [],
    };

    // Apply rules
    const processed = await this.rulesEngine.applyRules(expense);

    this.expenses.set(processed.id, processed);
    this.state.dataVersion++;

    this.emit('expense:created', { id: processed.id, expense: processed });

    // Persist to data directory
    await this._persistExpense(processed);

    return processed;
  }

  /**
   * PATCH /api/expense-domain/expenses/{id}
   * Update an existing expense
   */
  async updateExpense(id, updates) {
    this._assertRunning();

    const expense = this.expenses.get(id);
    if (!expense) {
      throw new Error(`Expense ${id} not found`);
    }

    const updated = {
      ...expense,
      ...updates,
      id, // Ensure ID doesn't change
      updatedAt: new Date(),
    };

    // Re-apply rules to updated expense
    const processed = await this.rulesEngine.applyRules(updated);

    this.expenses.set(id, processed);
    this.state.dataVersion++;

    this.emit('expense:updated', { id, expense: processed });

    // Persist to data directory
    await this._persistExpense(processed);

    // Attempt write-back to sources
    await this._maybeWriteBack('expenses', id, processed);

    return processed;
  }

  /**
   * DELETE /api/expense-domain/expenses/{id}
   * Delete an expense
   */
  async deleteExpense(id) {
    this._assertRunning();

    if (!this.expenses.has(id)) {
      throw new Error(`Expense ${id} not found`);
    }

    this.expenses.delete(id);
    this.state.dataVersion++;

    this.emit('expense:deleted', { id });

    return true;
  }

  /**
   * GET /api/expense-domain/rules
   * Get all rules for this domain
   */
  async getRules(filters = {}) {
    this._assertRunning();

    let rules = this.rulesEngine.rules;

    if (filters.type) {
      rules = rules.filter((r) => r.type === filters.type);
    }

    if (filters.source) {
      rules = rules.filter((r) => r.source === filters.source);
    }

    if (filters.confidence) {
      rules = rules.filter((r) => (r.confidence || 1) >= parseFloat(filters.confidence));
    }

    return rules;
  }

  /**
   * POST /api/expense-domain/rules
   * Create a new rule
   */
  async createRule(ruleData) {
    this._assertRunning();

    // Validate rule
    const validation = this.rulesEngine._validateRule ?
      this.rulesEngine._validateRule(ruleData) :
      { valid: true };

    if (!validation.valid) {
      throw new Error(`Invalid rule: ${validation.errors.join(', ')}`);
    }

    const rule = {
      id: `rule-${Date.now()}`,
      ...ruleData,
      createdAt: new Date(),
      origin: 'configured',
      enabled: true,
    };

    // Add to rules engine
    this.rulesEngine.rules.push(rule);

    this.emit('rule:created', { id: rule.id, rule });

    // Persist rule to data directory
    await this._persistRule(rule);

    return rule;
  }

  /**
   * PATCH /api/expense-domain/rules/{id}
   * Update a rule
   */
  async updateRule(ruleId, updates) {
    this._assertRunning();

    const ruleIndex = this.rulesEngine.rules.findIndex((r) => r.id === ruleId);
    if (ruleIndex === -1) {
      throw new Error(`Rule ${ruleId} not found`);
    }

    const rule = this.rulesEngine.rules[ruleIndex];
    const updated = {
      ...rule,
      ...updates,
      id: ruleId,
      updatedAt: new Date(),
    };

    this.rulesEngine.rules[ruleIndex] = updated;

    this.emit('rule:updated', { id: ruleId, rule: updated });

    // Persist rule to data directory
    await this._persistRule(updated);

    return updated;
  }

  /**
   * DELETE /api/expense-domain/rules/{id}
   * Delete a rule
   */
  async deleteRule(ruleId) {
    this._assertRunning();

    const index = this.rulesEngine.rules.findIndex((r) => r.id === ruleId);
    if (index === -1) {
      throw new Error(`Rule ${ruleId} not found`);
    }

    this.rulesEngine.rules.splice(index, 1);

    this.emit('rule:deleted', { id: ruleId });

    return true;
  }

  /**
   * GET /api/expense-domain/sources/{source}/status
   * Get source status
   */
  async getSourceStatus(sourceName) {
    this._assertRunning();

    const config = this.sourceConfigs[sourceName];
    if (!config) {
      throw new Error(`Source ${sourceName} not found`);
    }

    // Load last sync info if available
    const statusFile = path.join(
      this.dataDir,
      sourceName,
      '.status.json'
    );

    let status = {
      name: sourceName,
      enabled: config.enabled !== false,
      lastFetch: null,
      nextScheduled: null,
      status: 'idle',
      itemsProcessed: 0,
    };

    try {
      const content = await fs.readFile(statusFile, 'utf8');
      const data = JSON.parse(content);
      status = { ...status, ...data };
    } catch {
      // Status file doesn't exist yet
    }

    return status;
  }

  /**
   * POST /api/expense-domain/sources/{source}/write-back
   * Write updates back to a source
   */
  async writeBackToSource(sourceName, data) {
    this._assertRunning();

    const config = this.sourceConfigs[sourceName];
    if (!config) {
      throw new Error(`Source ${sourceName} not found`);
    }

    if (config.write_back?.enabled !== true) {
      throw new Error(`Write-back not enabled for source ${sourceName}`);
    }

    // This would call the source adapter's write-back capability
    this.emit('source:write-back-requested', { source: sourceName, data });

    return { status: 'queued', source: sourceName };
  }

  /**
   * Process domain data (called by process-transactions-job)
   */
  async process() {
    this._assertRunning();

    this.emit('expense-engine:processing-start');

    try {
      const expenses = Array.from(this.expenses.values());

      // Apply rules to all expenses
      const processed = await this.rulesEngine.applyRules(expenses);

      // Validate
      const validated = processed.filter((e) => {
        return e.amount && e.date && (e.source || e.category);
      });

      // Update in-memory store
      for (const exp of validated) {
        this.expenses.set(exp.id, exp);
      }

      this.state.lastProcessed = new Date();

      this.emit('expense-engine:processing-complete', {
        itemsProcessed: validated.length,
      });

      return validated;
    } catch (error) {
      this.emit('expense-engine:processing-failed', { error });
      throw error;
    }
  }

  /**
   * Learn rules from data patterns (called by learn-rules-job)
   */
  async learnRules() {
    this._assertRunning();

    this.emit('expense-engine:learning-start');

    try {
      const expenses = Array.from(this.expenses.values());

      // Analyze patterns (simplified AI learning)
      const patterns = this._analyzePatterns(expenses);

      const newRules = [];

      // Generate rules from high-confidence patterns
      for (const [category, merchants] of Object.entries(patterns)) {
        for (const [merchant, confidence] of Object.entries(merchants)) {
          if (confidence > 0.95) {
            const rule = {
              id: `learned-${Date.now()}-${Math.random()}`,
              name: `Auto-categorize ${merchant}`,
              type: 'categorization',
              pattern: { merchant: merchant },
              action: { type: 'set', field: 'category', value: category },
              confidence,
              origin: 'ai-learned',
              enabled: true,
              createdAt: new Date(),
            };

            newRules.push(rule);
            this.rulesEngine.rules.push(rule);
            await this._persistRule(rule);
          }
        }
      }

      this.emit('expense-engine:learning-complete', {
        rulesLearned: newRules.length,
      });

      return newRules;
    } catch (error) {
      this.emit('expense-engine:learning-failed', { error });
      throw error;
    }
  }

  // ============ Private Methods ============

  async _loadExpenses() {
    // Load expenses from data directory
    const dataFile = path.join(this.dataDir, 'engine', 'expenses.json');

    try {
      const content = await fs.readFile(dataFile, 'utf8');
      const expenses = JSON.parse(content);

      for (const exp of expenses) {
        this.expenses.set(exp.id, exp);
        if (parseInt(exp.id) > this.expenseId) {
          this.expenseId = parseInt(exp.id);
        }
      }

      this.emit('expense-engine:data-loaded', {
        expenseCount: this.expenses.size,
      });
    } catch (error) {
      if (error.code !== 'ENOENT') {
        this.emit('expense-engine:data-load-failed', { error });
      }
    }
  }

  async _persistExpense(expense) {
    // Persist expense to data directory
    const dataDir = path.join(this.dataDir, 'engine');
    const dataFile = path.join(dataDir, 'expenses.json');

    try {
      await fs.mkdir(dataDir, { recursive: true });

      const expenses = Array.from(this.expenses.values());
      await fs.writeFile(dataFile, JSON.stringify(expenses, null, 2));
    } catch (error) {
      this.emit('expense-engine:persist-failed', { error });
      throw error;
    }
  }

  async _persistRule(rule) {
    // Persist rule to rules directory
    const rulesDir = path.join(this.rulesDir, 'engine');

    try {
      await fs.mkdir(rulesDir, { recursive: true });

      const ruleFile = path.join(rulesDir, `${rule.id}.yaml`);
      const yaml = require('js-yaml');
      await fs.writeFile(ruleFile, yaml.dump(rule));
    } catch (error) {
      this.emit('expense-engine:rule-persist-failed', { error });
    }
  }

  async _maybeWriteBack(resource, id, data) {
    // TODO: Implement write-back to sources
  }

  _analyzePatterns(expenses) {
    // Simple pattern analysis: category -> merchant -> frequency
    const patterns = {};

    for (const exp of expenses) {
      if (!exp.category || !exp.merchant) continue;

      if (!patterns[exp.category]) {
        patterns[exp.category] = {};
      }

      if (!patterns[exp.category][exp.merchant]) {
        patterns[exp.category][exp.merchant] = 0;
      }

      patterns[exp.category][exp.merchant]++;
    }

    // Convert counts to confidence scores
    const totalExpenses = expenses.length;
    const confidencePatterns = {};

    for (const [category, merchants] of Object.entries(patterns)) {
      confidencePatterns[category] = {};

      for (const [merchant, count] of Object.entries(merchants)) {
        // Confidence based on frequency
        const confidence = Math.min(1.0, count / (totalExpenses * 0.1));
        confidencePatterns[category][merchant] = confidence;
      }
    }

    return confidencePatterns;
  }
}

module.exports = ExpenseEngine;
