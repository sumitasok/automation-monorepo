/**
 * Rules Engine
 * Applies learned and configured rules to domain data
 * Matches patterns and executes actions without code changes
 */

const EventEmitter = require('events');

class RulesEngine extends EventEmitter {
  constructor(rulesDir) {
    super();
    this.rulesDir = rulesDir;
    this.rules = []; // Loaded rules
    this.ruleCache = new Map(); // Cache for pattern matching
    this.appliedStats = {
      total: 0,
      byType: {},
      bySource: {},
      errors: 0,
    };
  }

  /**
   * Load rules
   * @param {Array} rules - Array of rule objects
   */
  loadRules(rules) {
    if (!Array.isArray(rules)) {
      throw new Error('Rules must be an array');
    }

    this.rules = rules.filter((rule) => rule.enabled !== false);
    this.ruleCache.clear();

    this.emit('rules:loaded', { count: this.rules.length });
    return this.rules;
  }

  /**
   * Apply rules to data
   * @param {Object|Array} data - Data to process
   * @param {Object} context - Processing context
   * @returns {Object|Array} processed data with rule applications
   */
  async applyRules(data, context = {}) {
    const isArray = Array.isArray(data);
    const items = isArray ? data : [data];

    this.emit('rules:applying', { itemCount: items.length });

    const results = [];

    for (const item of items) {
      try {
        const processed = await this._applyRulesToItem(item, context);
        results.push(processed);
      } catch (error) {
        this.appliedStats.errors++;
        this.emit('rule:application-error', {
          item,
          error,
          context,
        });
        throw error; // Re-throw or handle as configured
      }
    }

    this.emit('rules:applied', { itemsProcessed: results.length });
    return isArray ? results : results[0];
  }

  /**
   * Get applicable rules for an item
   * @param {Object} item - Item to check
   * @returns {Array} matching rules
   */
  getApplicableRules(item) {
    return this.rules.filter((rule) => {
      try {
        return this._matchesPattern(item, rule.pattern);
      } catch {
        return false;
      }
    });
  }

  /**
   * Get rule by ID
   */
  getRule(ruleId) {
    return this.rules.find((r) => r.id === ruleId);
  }

  /**
   * Get all rules of a specific type
   */
  getRulesByType(type) {
    return this.rules.filter((r) => r.type === type);
  }

  /**
   * Get statistics
   */
  getStats() {
    return this.appliedStats;
  }

  /**
   * Clear statistics
   */
  clearStats() {
    this.appliedStats = {
      total: 0,
      byType: {},
      bySource: {},
      errors: 0,
    };
  }

  // ============ Private Methods ============

  /**
   * Apply all matching rules to a single item
   */
  async _applyRulesToItem(item, context) {
    let processed = { ...item };

    for (const rule of this.rules) {
      try {
        if (this._matchesPattern(processed, rule.pattern)) {
          processed = await this._executeRule(processed, rule, context);

          this.appliedStats.total++;
          this.appliedStats.byType[rule.type] =
            (this.appliedStats.byType[rule.type] || 0) + 1;
          if (rule.source) {
            this.appliedStats.bySource[rule.source] =
              (this.appliedStats.bySource[rule.source] || 0) + 1;
          }

          this.emit('rule:applied', {
            ruleId: rule.id,
            ruleName: rule.name,
            item: processed,
          });
        }
      } catch (error) {
        this.emit('rule:application-failed', {
          rule,
          item,
          error,
        });

        // Continue with next rule if allowOnError is true
        if (!rule.allowOnError) {
          throw error;
        }
      }
    }

    return processed;
  }

  /**
   * Check if item matches rule pattern
   */
  _matchesPattern(item, pattern) {
    if (!pattern) return true;

    // Simple pattern matching implementations
    if (typeof pattern === 'string') {
      // Exact field match: "fieldName:value"
      return this._matchesSimplePattern(item, pattern);
    }

    if (typeof pattern === 'object') {
      // Object pattern matching
      return this._matchesObjectPattern(item, pattern);
    }

    if (typeof pattern === 'function') {
      // Custom function
      return pattern(item);
    }

    return true;
  }

  /**
   * Simple pattern matching
   */
  _matchesSimplePattern(item, pattern) {
    const [field, value] = pattern.split(':');
    if (!field || !value) return false;

    const itemValue = this._getNestedValue(item, field);
    return itemValue === value;
  }

  /**
   * Object pattern matching (AND logic)
   */
  _matchesObjectPattern(item, pattern) {
    for (const [field, value] of Object.entries(pattern)) {
      const itemValue = this._getNestedValue(item, field);

      if (typeof value === 'string' && value.includes('*')) {
        // Wildcard matching
        const regex = new RegExp(
          '^' + value.replace(/\*/g, '.*') + '$'
        );
        if (!regex.test(String(itemValue))) return false;
      } else if (typeof value === 'object' && value.$in) {
        // Array contains
        if (!value.$in.includes(itemValue)) return false;
      } else if (typeof value === 'object' && value.$regex) {
        // Regex matching
        const regex = new RegExp(value.$regex, value.$options);
        if (!regex.test(String(itemValue))) return false;
      } else {
        // Exact match
        if (itemValue !== value) return false;
      }
    }

    return true;
  }

  /**
   * Execute a rule action
   */
  async _executeRule(item, rule, context) {
    const action = rule.action;

    if (typeof action === 'function') {
      // Custom function
      return await action(item, context, rule);
    }

    if (typeof action === 'string') {
      // Predefined actions
      return this._executeStringAction(item, action, rule);
    }

    if (typeof action === 'object') {
      // Declarative action
      return this._executeObjectAction(item, action, rule);
    }

    return item;
  }

  /**
   * Execute string-based action (e.g., "set:field=value")
   */
  _executeStringAction(item, action, rule) {
    // Examples:
    // "set:category=uncategorized"
    // "set:status=processed"
    // "delete:internalField"

    if (action.startsWith('set:')) {
      const [field, value] = action.substring(4).split('=');
      return this._setNestedValue(item, field, value);
    }

    if (action.startsWith('delete:')) {
      const field = action.substring(7);
      return this._deleteNestedValue(item, field);
    }

    return item;
  }

  /**
   * Execute object-based action (declarative)
   */
  _executeObjectAction(item, action, rule) {
    // Examples:
    // { type: 'set', field: 'category', value: 'uncategorized' }
    // { type: 'delete', field: 'internalField' }
    // { type: 'transform', field: 'amount', fn: (v) => v * 1.1 }

    if (action.type === 'set') {
      return this._setNestedValue(item, action.field, action.value);
    }

    if (action.type === 'delete') {
      return this._deleteNestedValue(item, action.field);
    }

    if (action.type === 'transform' && typeof action.fn === 'function') {
      const value = this._getNestedValue(item, action.field);
      const transformed = action.fn(value);
      return this._setNestedValue(item, action.field, transformed);
    }

    return item;
  }

  /**
   * Get nested value from object (supports dot notation)
   */
  _getNestedValue(obj, path) {
    return path.split('.').reduce((current, part) => current?.[part], obj);
  }

  /**
   * Set nested value in object (supports dot notation)
   */
  _setNestedValue(obj, path, value) {
    const result = JSON.parse(JSON.stringify(obj));
    const parts = path.split('.');
    let current = result;

    for (let i = 0; i < parts.length - 1; i++) {
      const part = parts[i];
      if (!current[part]) {
        current[part] = {};
      }
      current = current[part];
    }

    current[parts[parts.length - 1]] = value;
    return result;
  }

  /**
   * Delete nested value from object
   */
  _deleteNestedValue(obj, path) {
    const result = JSON.parse(JSON.stringify(obj));
    const parts = path.split('.');
    let current = result;

    for (let i = 0; i < parts.length - 1; i++) {
      current = current[parts[i]];
      if (!current) return result;
    }

    delete current[parts[parts.length - 1]];
    return result;
  }
}

module.exports = RulesEngine;
