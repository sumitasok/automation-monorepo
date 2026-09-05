/**
 * Rules Loader
 * Loads learned and configured rules from ~/automation-monorepo-config/rules/
 * Provides rules to domain engines for application
 */

const fs = require('fs').promises;
const path = require('path');
const yaml = require('js-yaml');

class RulesLoader {
  constructor(configPath) {
    if (!configPath) {
      throw new Error('configPath is required');
    }
    this.configPath = path.resolve(configPath);
    this.cache = new Map();
  }

  /**
   * Load all rules for a domain
   * @param {string} domainName - Domain name
   * @returns {Array} rules array
   */
  async loadDomainRules(domainName) {
    const cacheKey = `domain:${domainName}`;
    if (this.cache.has(cacheKey)) {
      return this.cache.get(cacheKey);
    }

    const rules = [];

    // Load engine rules
    const engineRules = await this._loadRulesFromDir(
      path.join(this.configPath, 'rules', domainName, 'engine')
    );
    rules.push(...engineRules);

    // Load source-specific rules
    const rulesDir = path.join(this.configPath, 'rules', domainName);
    const sourceDirs = await this._getSubdirectories(rulesDir);

    for (const sourceDir of sourceDirs) {
      if (sourceDir !== 'engine') {
        const sourceRules = await this._loadRulesFromDir(
          path.join(rulesDir, sourceDir)
        );
        rules.push(...sourceRules.map((r) => ({ ...r, source: sourceDir })));
      }
    }

    this.cache.set(cacheKey, rules);
    return rules;
  }

  /**
   * Load rules for a specific source within a domain
   * @param {string} domainName - Domain name
   * @param {string} sourceName - Source name
   * @returns {Array} source-specific rules
   */
  async loadSourceRules(domainName, sourceName) {
    const cacheKey = `source:${domainName}:${sourceName}`;
    if (this.cache.has(cacheKey)) {
      return this.cache.get(cacheKey);
    }

    const rulesDir = path.join(
      this.configPath,
      'rules',
      domainName,
      sourceName
    );
    const rules = await this._loadRulesFromDir(rulesDir);

    this.cache.set(cacheKey, rules);
    return rules;
  }

  /**
   * Load engine-specific rules
   * @param {string} domainName - Domain name
   * @returns {Array} engine rules
   */
  async loadEngineRules(domainName) {
    const cacheKey = `engine:${domainName}`;
    if (this.cache.has(cacheKey)) {
      return this.cache.get(cacheKey);
    }

    const engineDir = path.join(
      this.configPath,
      'rules',
      domainName,
      'engine'
    );
    const rules = await this._loadRulesFromDir(engineDir);

    this.cache.set(cacheKey, rules);
    return rules;
  }

  /**
   * Get enabled rules (filtering out disabled ones)
   * @param {string} domainName - Domain name
   * @returns {Array} enabled rules only
   */
  async getEnabledRules(domainName) {
    const allRules = await this.loadDomainRules(domainName);
    return allRules.filter((rule) => rule.enabled !== false);
  }

  /**
   * Get rules by type
   * @param {string} domainName - Domain name
   * @param {string} type - Rule type (categorization, validation, dedup, etc.)
   * @returns {Array} rules of specified type
   */
  async getRulesByType(domainName, type) {
    const allRules = await this.loadDomainRules(domainName);
    return allRules.filter((rule) => rule.type === type);
  }

  /**
   * Get high-confidence rules (AI-learned)
   * @param {string} domainName - Domain name
   * @param {number} minConfidence - Minimum confidence threshold (0-1)
   * @returns {Array} high-confidence rules
   */
  async getHighConfidenceRules(domainName, minConfidence = 0.95) {
    const allRules = await this.loadDomainRules(domainName);
    return allRules.filter(
      (rule) =>
        (rule.origin === 'ai-learned' && rule.confidence >= minConfidence) ||
        rule.origin !== 'ai-learned' // Always include manual rules
    );
  }

  /**
   * Find rules matching a pattern
   * @param {string} domainName - Domain name
   * @param {string} searchTerm - Search term (rule name, pattern, action)
   * @returns {Array} matching rules
   */
  async findRules(domainName, searchTerm) {
    const allRules = await this.loadDomainRules(domainName);
    const term = searchTerm.toLowerCase();

    return allRules.filter(
      (rule) =>
        rule.name?.toLowerCase().includes(term) ||
        rule.description?.toLowerCase().includes(term) ||
        rule.pattern?.toString().toLowerCase().includes(term)
    );
  }

  /**
   * Validate rule structure
   */
  validateRule(rule) {
    const errors = [];

    if (!rule.name) errors.push('Rule name is required');
    if (!rule.type) errors.push('Rule type is required');
    if (!rule.pattern) errors.push('Rule pattern is required');
    if (!rule.action) errors.push('Rule action is required');

    if (rule.confidence && (rule.confidence < 0 || rule.confidence > 1)) {
      errors.push('Confidence must be between 0 and 1');
    }

    return {
      valid: errors.length === 0,
      errors,
    };
  }

  /**
   * Clear cache
   */
  clearCache() {
    this.cache.clear();
  }

  /**
   * Clear specific cache entry
   */
  clearCacheEntry(cacheKey) {
    this.cache.delete(cacheKey);
  }

  /**
   * Get rules directory for domain
   */
  getRulesDir(domainName) {
    return path.join(this.configPath, 'rules', domainName);
  }

  // ============ Private Methods ============

  /**
   * Load all rule YAML files from a directory
   */
  async _loadRulesFromDir(dirPath) {
    const rules = [];

    try {
      const files = await fs.readdir(dirPath);

      for (const file of files) {
        if (file.endsWith('.yaml') || file.endsWith('.yml')) {
          const filePath = path.join(dirPath, file);
          const content = await fs.readFile(filePath, 'utf8');
          const parsed = yaml.load(content);

          // Handle both single rule and array of rules in file
          if (Array.isArray(parsed)) {
            rules.push(...parsed);
          } else if (parsed) {
            rules.push(parsed);
          }
        }
      }
    } catch (error) {
      if (error.code !== 'ENOENT') {
        // Directory doesn't exist is OK, other errors should throw
        throw new Error(
          `Failed to load rules from ${dirPath}: ${error.message}`
        );
      }
    }

    return rules;
  }

  /**
   * Get subdirectories of a path
   */
  async _getSubdirectories(dirPath) {
    try {
      const entries = await fs.readdir(dirPath, { withFileTypes: true });
      return entries
        .filter((entry) => entry.isDirectory())
        .map((entry) => entry.name);
    } catch (error) {
      if (error.code === 'ENOENT') {
        return [];
      }
      throw error;
    }
  }
}

module.exports = RulesLoader;
