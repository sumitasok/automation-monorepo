/**
 * Job Manifest Schema
 * Defines the structure and validation for job manifests
 */

const Schema = {
  // Example job manifest structure with all valid fields
  exampleManifest: {
    // Job identity
    name: 'gmail-fetch-job',
    description: 'Fetch emails from Gmail daily',

    // Schedule configuration
    schedule: {
      type: 'interval', // 'interval' or 'cron' (cron planned)
      interval: '1d', // Format: {number}{s|m|h|d} = seconds, minutes, hours, days
    },

    // Execution configuration
    timeout: 300, // seconds before job times out
    retry: {
      maxRetries: 3,
      backoffMultiplier: 2, // exponential: 5s, 10s, 20s
    },

    // Job handlers (required)
    handlers: {
      // Called when job execution starts
      onStart: async ({ executionId, jobId, execution }) => {},

      // Called to execute the job (required)
      // Must return result object or throw error
      execute: async ({ executionId, jobId, execution }) => {
        return { itemsProcessed: 10 };
      },

      // Called when job succeeds
      onSuccess: async ({ executionId, jobId, execution, result }) => {},

      // Called when job fails (after all retries)
      onFailure: async ({ executionId, jobId, execution, error }) => {},

      // Called when job completes (success or failure)
      onComplete: async ({ executionId, jobId, execution }) => {},
    },

    // Enable/disable
    enabled: true,
  },

  // Validation rules
  validate: (manifest) => {
    const errors = [];

    // Required fields
    if (!manifest.name) errors.push('name is required');
    if (!manifest.handlers) errors.push('handlers object is required');
    if (!manifest.handlers.execute) {
      errors.push('handlers.execute function is required');
    }

    // Schedule validation
    if (!manifest.schedule) {
      errors.push('schedule is required');
    } else {
      if (!manifest.schedule.type) {
        errors.push('schedule.type is required');
      }
      if (
        manifest.schedule.type === 'interval' &&
        !manifest.schedule.interval
      ) {
        errors.push('schedule.interval is required for interval-based schedules');
      }

      // Validate interval format
      if (
        manifest.schedule.interval &&
        !Schema.isValidInterval(manifest.schedule.interval)
      ) {
        errors.push(
          `Invalid interval format: ${manifest.schedule.interval}. Expected: {number}{s|m|h|d}`
        );
      }
    }

    // Timeout validation
    if (manifest.timeout && typeof manifest.timeout !== 'number') {
      errors.push('timeout must be a number (seconds)');
    }
    if (manifest.timeout && manifest.timeout < 1) {
      errors.push('timeout must be at least 1 second');
    }

    // Retry validation
    if (manifest.retry) {
      if (
        manifest.retry.maxRetries &&
        typeof manifest.retry.maxRetries !== 'number'
      ) {
        errors.push('retry.maxRetries must be a number');
      }
      if (manifest.retry.maxRetries < 0) {
        errors.push('retry.maxRetries must be >= 0');
      }
      if (
        manifest.retry.backoffMultiplier &&
        typeof manifest.retry.backoffMultiplier !== 'number'
      ) {
        errors.push('retry.backoffMultiplier must be a number');
      }
      if (manifest.retry.backoffMultiplier < 1) {
        errors.push('retry.backoffMultiplier must be >= 1');
      }
    }

    return {
      valid: errors.length === 0,
      errors,
    };
  },

  // Helper: Check if interval is in valid format
  isValidInterval: (interval) => {
    return /^\d+[smhd]$/.test(interval);
  },

  // Helper: Parse interval to milliseconds
  parseInterval: (interval) => {
    const match = interval.match(/^(\d+)([smhd])$/);
    if (!match) return null;

    const [, value, unit] = match;
    const multipliers = {
      s: 1000,
      m: 60 * 1000,
      h: 60 * 60 * 1000,
      d: 24 * 60 * 60 * 1000,
    };

    return parseInt(value) * multipliers[unit];
  },

  // Sample manifests for reference

  // Adapter fetch job
  adapterFetchJobManifest: {
    name: 'gmail-fetch-job',
    description: 'Fetch new emails from Gmail',
    schedule: {
      type: 'interval',
      interval: '1h', // every hour
    },
    timeout: 300,
    retry: {
      maxRetries: 3,
      backoffMultiplier: 2,
    },
    enabled: true,
  },

  // Adapter monitor job
  adapterMonitorJobManifest: {
    name: 'bank-csv-monitor-job',
    description: 'Monitor for uploaded bank CSV files',
    schedule: {
      type: 'interval',
      interval: '30s', // every 30 seconds
    },
    timeout: 60,
    retry: {
      maxRetries: 2,
      backoffMultiplier: 2,
    },
    enabled: true,
  },

  // Domain engine processing job
  engineProcessingJobManifest: {
    name: 'process-transactions-job',
    description: 'Process fetched data through domain engine',
    schedule: {
      type: 'interval',
      interval: '5m', // every 5 minutes
    },
    timeout: 600,
    retry: {
      maxRetries: 3,
      backoffMultiplier: 2,
    },
    enabled: true,
  },

  // AI learning job
  aiLearningJobManifest: {
    name: 'learn-rules-job',
    description: 'Learn new rules from transaction patterns',
    schedule: {
      type: 'interval',
      interval: '1d', // daily
    },
    timeout: 1800, // 30 minutes
    retry: {
      maxRetries: 2,
      backoffMultiplier: 2,
    },
    enabled: true,
  },
};

module.exports = Schema;
