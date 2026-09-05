/**
 * Domain Engine API Base Class
 * All domain engines inherit from this to expose consistent API contracts
 */

const EventEmitter = require('events');
const path = require('path');

class DomainEngine extends EventEmitter {
  /**
   * Initialize domain engine
   * @param {string} configPath - Path to ~/automation-monorepo-config/
   * @param {string} domainName - Name of domain (e.g., 'expense-domain')
   * @param {Object} options - Domain-specific options
   */
  constructor(configPath, domainName, options = {}) {
    super();
    this.configPath = configPath;
    this.domainName = domainName;
    this.options = options;

    // Data/config locations (injected)
    this.dataDir = path.join(configPath, 'data', domainName);
    this.configDir = path.join(configPath, 'config', domainName);
    this.rulesDir = path.join(configPath, 'rules', domainName);

    // Engine state
    this.state = {
      initialized: false,
      running: false,
      lastProcessed: null,
      dataVersion: 0,
    };

    this.sources = new Map(); // source-name -> source-adapter
    this.rulesEngine = null;
    this.domainConfig = null;
  }

  /**
   * Initialize the domain engine
   * Called once at startup
   */
  async initialize() {
    if (this.state.initialized) {
      throw new Error(`${this.domainName} already initialized`);
    }

    this.emit('engine:initializing', { domain: this.domainName });

    try {
      // Load domain configuration
      this.domainConfig = await this._loadDomainConfig();

      // Initialize source adapters
      await this._initializeSources();

      // Initialize rules engine
      await this._initializeRulesEngine();

      this.state.initialized = true;
      this.emit('engine:initialized', { domain: this.domainName });
    } catch (error) {
      this.emit('engine:initialization-failed', {
        domain: this.domainName,
        error,
      });
      throw error;
    }
  }

  /**
   * Start the domain engine
   */
  async start() {
    if (!this.state.initialized) {
      await this.initialize();
    }

    if (this.state.running) {
      throw new Error(`${this.domainName} is already running`);
    }

    this.state.running = true;
    this.emit('engine:started', { domain: this.domainName });
  }

  /**
   * Stop the domain engine
   */
  async stop() {
    this.state.running = false;
    this.emit('engine:stopped', { domain: this.domainName });
  }

  // ============ Data Operations ============

  /**
   * GET: Retrieve domain data
   * @param {string} resource - Resource type (e.g., 'expenses')
   * @param {string} id - Optional: specific resource ID
   * @returns {Object|Array} domain data
   */
  async getData(resource, id = null) {
    this._assertRunning();
    this.emit('api:get-data', { domain: this.domainName, resource, id });

    // Implementation: Load from ~/automation-monorepo-config/data/{domain}/engine/
    // Expected: File-based or database storage
    const data = await this._loadData(resource, id);
    return data;
  }

  /**
   * PATCH: Update existing domain data
   * @param {string} resource - Resource type
   * @param {string} id - Resource ID
   * @param {Object} updates - Fields to update
   * @returns {Object} updated resource
   */
  async updateData(resource, id, updates) {
    this._assertRunning();
    this.emit('api:update-data', {
      domain: this.domainName,
      resource,
      id,
      updates,
    });

    const updated = await this._persistData(resource, id, updates);

    // Trigger potential write-back to sources
    await this._maybeWriteBack(resource, id, updated);

    this.state.dataVersion++;
    return updated;
  }

  /**
   * POST: Create new domain data
   * @param {string} resource - Resource type
   * @param {Object} data - Data to create
   * @returns {Object} created resource with ID
   */
  async createData(resource, data) {
    this._assertRunning();
    this.emit('api:create-data', { domain: this.domainName, resource, data });

    const created = await this._persistData(resource, null, data);
    this.state.dataVersion++;
    return created;
  }

  /**
   * DELETE: Remove domain data
   * @param {string} resource - Resource type
   * @param {string} id - Resource ID
   * @returns {boolean} success
   */
  async deleteData(resource, id) {
    this._assertRunning();
    this.emit('api:delete-data', { domain: this.domainName, resource, id });

    await this._deleteData(resource, id);
    this.state.dataVersion++;
    return true;
  }

  // ============ Rules Operations ============

  /**
   * GET: Retrieve rules for this domain
   * @returns {Array} rules with metadata
   */
  async getRules() {
    this._assertRunning();
    this.emit('api:get-rules', { domain: this.domainName });

    return this.rulesEngine ? await this.rulesEngine.loadRules() : [];
  }

  /**
   * POST: Create a new rule
   * @param {Object} rule - Rule definition
   * @returns {Object} created rule with ID
   */
  async createRule(rule) {
    this._assertRunning();
    this.emit('api:create-rule', { domain: this.domainName, rule });

    return this.rulesEngine ? await this.rulesEngine.createRule(rule) : null;
  }

  /**
   * PATCH: Update an existing rule
   * @param {string} ruleId - Rule ID
   * @param {Object} updates - Fields to update
   * @returns {Object} updated rule
   */
  async updateRule(ruleId, updates) {
    this._assertRunning();
    this.emit('api:update-rule', {
      domain: this.domainName,
      ruleId,
      updates,
    });

    return this.rulesEngine
      ? await this.rulesEngine.updateRule(ruleId, updates)
      : null;
  }

  /**
   * DELETE: Remove a rule
   * @param {string} ruleId - Rule ID
   * @returns {boolean} success
   */
  async deleteRule(ruleId) {
    this._assertRunning();
    this.emit('api:delete-rule', { domain: this.domainName, ruleId });

    return this.rulesEngine
      ? await this.rulesEngine.deleteRule(ruleId)
      : false;
  }

  // ============ Source Operations ============

  /**
   * GET: Retrieve source status
   * @param {string} sourceName - Source name
   * @returns {Object} status information
   */
  async getSourceStatus(sourceName) {
    this._assertRunning();
    const source = this.sources.get(sourceName);
    if (!source) {
      throw new Error(`Source ${sourceName} not found`);
    }

    return source.getStatus ? await source.getStatus() : { status: 'unknown' };
  }

  /**
   * POST: Trigger a source job
   * @param {string} sourceName - Source name
   * @param {string} jobName - Job name (e.g., 'fetch', 'monitor')
   * @param {Object} context - Job context/parameters
   * @returns {string} execution ID
   */
  async triggerSourceJob(sourceName, jobName, context = {}) {
    this._assertRunning();
    const source = this.sources.get(sourceName);
    if (!source) {
      throw new Error(`Source ${sourceName} not found`);
    }

    this.emit('api:trigger-job', {
      domain: this.domainName,
      source: sourceName,
      job: jobName,
    });

    return source.triggerJob
      ? await source.triggerJob(jobName, context)
      : null;
  }

  // ============ Processing ============

  /**
   * Process: Core business logic
   * Override in subclass for domain-specific processing
   */
  async process() {
    this._assertRunning();
    this.emit('engine:processing-start', { domain: this.domainName });

    try {
      // 1. Read from sources
      const sourceData = await this._readFromSources();

      // 2. Apply rules
      const processedData = await this._applyRules(sourceData);

      // 3. Validate
      const validated = await this._validate(processedData);

      // 4. Persist
      await this._persist(validated);

      // 5. Check for learning opportunities
      await this._checkForLearning(validated);

      this.state.lastProcessed = new Date();
      this.emit('engine:processing-complete', {
        domain: this.domainName,
        itemsProcessed: validated.length,
      });

      return validated;
    } catch (error) {
      this.emit('engine:processing-failed', {
        domain: this.domainName,
        error,
      });
      throw error;
    }
  }

  // ============ Protected Methods (Override in Subclass) ============

  async _loadDomainConfig() {
    // TODO: Implement loading domain config from YAML
    return {};
  }

  async _initializeSources() {
    // TODO: Initialize source adapters from config
  }

  async _initializeRulesEngine() {
    // TODO: Initialize rules engine
  }

  async _loadData(resource, id) {
    // TODO: Load data from storage
    return id ? {} : [];
  }

  async _persistData(resource, id, data) {
    // TODO: Persist data to storage
    return data;
  }

  async _deleteData(resource, id) {
    // TODO: Delete data from storage
  }

  async _maybeWriteBack(resource, id, data) {
    // TODO: Write updates back to sources if configured
  }

  async _readFromSources() {
    // TODO: Read data from all sources
    return [];
  }

  async _applyRules(data) {
    // TODO: Apply learned rules to data
    return data;
  }

  async _validate(data) {
    // TODO: Validate processed data
    return data;
  }

  async _persist(data) {
    // TODO: Persist processed data
  }

  async _checkForLearning(data) {
    // TODO: Check if new patterns should be learned
  }

  // ============ Private Helpers ============

  _assertRunning() {
    if (!this.state.running) {
      throw new Error(`${this.domainName} is not running`);
    }
  }
}

module.exports = DomainEngine;
