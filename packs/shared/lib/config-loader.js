/**
 * Configuration Loader
 * Loads and merges configuration from ~/automation-monorepo-config/config/
 * Provides injected config to domains and sources
 */

const fs = require('fs').promises;
const path = require('path');
const yaml = require('js-yaml'); // Requires: npm install js-yaml

class ConfigLoader {
  constructor(configPath) {
    if (!configPath) {
      throw new Error('configPath is required');
    }
    this.configPath = path.resolve(configPath);
    this.cache = new Map();
  }

  /**
   * Load framework-level configuration
   * @returns {Object} framework config
   */
  async loadFrameworkConfig() {
    const cacheKey = 'framework';
    if (this.cache.has(cacheKey)) {
      return this.cache.get(cacheKey);
    }

    const frameworkConfigPath = path.join(
      this.configPath,
      'config',
      'framework.yaml'
    );

    const config = await this._loadYAML(frameworkConfigPath);
    this.cache.set(cacheKey, config);
    return config;
  }

  /**
   * Load domain-level configuration
   * @param {string} domainName - Name of domain
   * @returns {Object} domain config
   */
  async loadDomainConfig(domainName) {
    const cacheKey = `domain:${domainName}`;
    if (this.cache.has(cacheKey)) {
      return this.cache.get(cacheKey);
    }

    const domainConfigPath = path.join(
      this.configPath,
      'config',
      domainName,
      'domain.yaml'
    );

    const config = await this._loadYAML(domainConfigPath);
    this.cache.set(cacheKey, config);
    return config;
  }

  /**
   * Load source adapter configuration
   * @param {string} domainName - Domain name
   * @param {string} sourceName - Source name
   * @returns {Object} source config
   */
  async loadSourceConfig(domainName, sourceName) {
    const cacheKey = `source:${domainName}:${sourceName}`;
    if (this.cache.has(cacheKey)) {
      return this.cache.get(cacheKey);
    }

    const sourceConfigPath = path.join(
      this.configPath,
      'config',
      domainName,
      `${sourceName}.yaml`
    );

    const config = await this._loadYAML(sourceConfigPath);
    this.cache.set(cacheKey, config);
    return config;
  }

  /**
   * Load all source configurations for a domain
   * @param {string} domainName - Domain name
   * @returns {Object} map of source-name -> config
   */
  async loadAllSourceConfigs(domainName) {
    const domainConfigDir = path.join(this.configPath, 'config', domainName);

    try {
      const files = await fs.readdir(domainConfigDir);
      const sourceConfigs = {};

      for (const file of files) {
        if (file.endsWith('.yaml') && file !== 'domain.yaml') {
          const sourceName = file.replace('.yaml', '');
          sourceConfigs[sourceName] = await this.loadSourceConfig(
            domainName,
            sourceName
          );
        }
      }

      return sourceConfigs;
    } catch (error) {
      if (error.code === 'ENOENT') {
        return {}; // Domain has no sources configured
      }
      throw error;
    }
  }

  /**
   * Load available domains from framework config
   * @returns {Array} domain definitions
   */
  async loadAvailableDomains() {
    const frameworkConfig = await this.loadFrameworkConfig();
    return frameworkConfig.domains || [];
  }

  /**
   * Validate configuration structure
   * @param {string} domainName - Domain to validate
   * @returns {Object} validation result
   */
  async validateDomainConfig(domainName) {
    const errors = [];
    const warnings = [];

    try {
      // Check domain config exists
      const domainConfig = await this.loadDomainConfig(domainName);
      if (!domainConfig) {
        errors.push(`Domain config missing for ${domainName}`);
      }

      // Check sources listed in domain config exist
      if (domainConfig?.sources) {
        const sourceConfigs = await this.loadAllSourceConfigs(domainName);
        for (const source of domainConfig.sources) {
          if (!sourceConfigs[source.name]) {
            warnings.push(
              `Source config missing for ${source.name} in ${domainName}`
            );
          }
        }
      }

      // Check framework config
      const frameworkConfig = await this.loadFrameworkConfig();
      if (!frameworkConfig) {
        errors.push('Framework config missing');
      }

      // Check domain is listed in framework
      const domains = frameworkConfig?.domains || [];
      const domainFound = domains.some((d) => d.name === domainName);
      if (!domainFound) {
        warnings.push(`${domainName} not listed in framework domains`);
      }
    } catch (error) {
      errors.push(`Config validation error: ${error.message}`);
    }

    return {
      valid: errors.length === 0,
      errors,
      warnings,
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
   * Get config path
   */
  getConfigPath() {
    return this.configPath;
  }

  /**
   * Get domain data directory
   */
  getDataDir(domainName) {
    return path.join(this.configPath, 'data', domainName);
  }

  /**
   * Get domain rules directory
   */
  getRulesDir(domainName) {
    return path.join(this.configPath, 'rules', domainName);
  }

  // ============ Private Methods ============

  /**
   * Load and parse YAML file
   */
  async _loadYAML(filePath) {
    try {
      const content = await fs.readFile(filePath, 'utf8');
      return yaml.load(content) || {};
    } catch (error) {
      if (error.code === 'ENOENT') {
        throw new Error(`Config file not found: ${filePath}`);
      }
      throw new Error(`Failed to parse config file ${filePath}: ${error.message}`);
    }
  }
}

module.exports = ConfigLoader;
