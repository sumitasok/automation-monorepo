/**
 * Wallet Deduplication & Source Identification
 * Identifies duplicate records and adds source tracking
 */

const fs = require('fs');
const path = require('path');

class WalletDeduplicator {
  constructor(configPath) {
    this.configPath = configPath;
    this.dataPath = path.join(configPath, 'data', 'expense-domain', 'wallet');
  }

  /**
   * Add source identification to wallet record
   */
  addSourceIdentification(record, source = 'automation-monorepo') {
    return {
      ...record,
      labels: [...(record.labels || []), `source:${source}`],
      description: `[source:${source}] ${record.description || ''}`.trim(),
      _source_added_at: new Date().toISOString(),
    };
  }

  /**
   * Find duplicate records by matching key fields
   * Returns array of duplicate groups
   */
  findDuplicates(records) {
    const groups = new Map();
    const duplicates = [];

    records.forEach((record) => {
      // Create signature for matching: amount + merchant + date
      const signature = this.createSignature(record);

      if (groups.has(signature)) {
        // Found a duplicate
        const group = groups.get(signature);
        group.push(record);
        if (group.length === 2) {
          // This is the second duplicate, add group to results
          duplicates.push(group);
        }
      } else {
        // New unique record
        groups.set(signature, [record]);
      }
    });

    return duplicates;
  }

  /**
   * Create signature for duplicate matching
   * Matches by amount, merchant, and date (±1 day tolerance)
   */
  createSignature(record) {
    const amount = Math.round(record.amount * 100); // Convert to cents
    const merchant = (record.merchant || '').toUpperCase();
    const date = this.normalizeDate(record.date);

    return `${amount}|${merchant}|${date}`;
  }

  /**
   * Normalize date for fuzzy matching (±1 day)
   */
  normalizeDate(dateStr) {
    const date = new Date(dateStr);
    // Use date without time, and floor to day
    return Math.floor(date.getTime() / (24 * 60 * 60 * 1000)).toString();
  }

  /**
   * Deduplicate records with intelligent merging
   * Rules:
   * - Base: Keep automation record (source:automation-monorepo)
   * - Merge: Best attributes from manual record
   *   * Take correct category from automation
   *   * Take better tags from manual record
   *   * Take better description from manual record
   * - Audit: Track what was merged
   */
  deduplicateRecords(recordsWithDuplicates, mergeStrategy = 'best-of-both') {
    const deduplicated = [];
    const removed = [];
    const merged = new Set(); // Track already merged records

    recordsWithDuplicates.forEach((record) => {
      if (merged.has(record.id)) return; // Skip already processed

      const hasSourceLabel = (record.labels || []).includes('source:automation-monorepo');

      // Find duplicate if this is automation record
      if (hasSourceLabel) {
        const duplicate = recordsWithDuplicates.find(r =>
          !merged.has(r.id) &&
          !(r.labels || []).includes('source:automation-monorepo') &&
          this.isSameDuplicate(r, record)
        );

        if (duplicate && mergeStrategy === 'best-of-both') {
          // Merge best attributes from both records
          const merged_record = this.mergeRecords(record, duplicate);
          deduplicated.push(merged_record);

          // Mark both as processed
          merged.add(record.id);
          merged.add(duplicate.id);

          // Track the removal
          removed.push({
            ...duplicate,
            _duplicate_reason: 'merged-into-automation-record',
            _merged_with: record.id,
            _merge_strategy: 'best-of-both',
          });
        } else {
          // No duplicate found, keep as is
          deduplicated.push(record);
          merged.add(record.id);
        }
      }
    });

    return { deduplicated, removed };
  }

  /**
   * Intelligently merge two duplicate records
   * Takes best attributes from both
   */
  mergeRecords(automationRecord, manualRecord) {
    return {
      // Keep automation record ID and base structure
      ...automationRecord,

      // Take better description (manual usually has more detail)
      description: this.isBetterDescription(automationRecord.description, manualRecord.description)
        ? manualRecord.description
        : automationRecord.description,

      // Merge tags/labels - combine both for maximum info
      labels: [
        ...(automationRecord.labels || []),
        ...(manualRecord.labels || []),
        // Remove duplicates while preserving order
      ].filter((label, index, self) => self.indexOf(label) === index),

      // Keep automation's category (it's AI-categorized and correct)
      category: automationRecord.category,

      // Merge notes/metadata
      _merged_attributes: {
        source_record_id: automationRecord.id,
        merged_with_id: manualRecord.id,
        merged_at: new Date().toISOString(),
        merged_strategy: 'best-of-both',
        kept_from_automation: {
          id: automationRecord.id,
          category: automationRecord.category,
          source_label: true,
        },
        merged_from_manual: {
          id: manualRecord.id,
          description: manualRecord.description !== automationRecord.description,
          tags: manualRecord.labels?.length > (automationRecord.labels?.length || 0),
        },
      },
    };
  }

  /**
   * Determine which description is better
   * Better = longer, more detailed
   */
  isBetterDescription(desc1, desc2) {
    if (!desc1 && desc2) return true;
    if (!desc2 && desc1) return false;
    if (!desc1 && !desc2) return false;

    // Prefer longer, more detailed descriptions
    return desc2.length > desc1.length;
  }

  /**
   * Check if two records are duplicates
   */
  isSameDuplicate(record1, record2) {
    return this.createSignature(record1) === this.createSignature(record2);
  }

  /**
   * Generate deduplication report
   */
  generateReport(allRecords) {
    const duplicates = this.findDuplicates(allRecords);
    const { deduplicated, removed } = this.deduplicateRecords(allRecords);

    return {
      summary: {
        total_records: allRecords.length,
        unique_records: deduplicated.length,
        duplicates_found: duplicates.length,
        records_to_remove: removed.length,
        dedup_rate: `${((removed.length / allRecords.length) * 100).toFixed(1)}%`,
      },
      duplicates: duplicates.map(group => ({
        count: group.length,
        amount: group[0].amount,
        merchant: group[0].merchant,
        date: group[0].date,
        with_source: group.some(r => (r.labels || []).includes('source:automation-monorepo')),
        without_source: group.some(r => !(r.labels || []).includes('source:automation-monorepo')),
        records: group.map(r => ({
          id: r.id,
          category: r.category,
          source_label: (r.labels || []).includes('source:automation-monorepo') ? '✓' : '✗',
        })),
      })),
      to_remove: removed.map(r => ({
        id: r.id,
        merchant: r.merchant,
        category: r.category,
        reason: r._duplicate_reason,
      })),
    };
  }

  /**
   * Apply source identification to all new records
   * Call this before syncing to wallet
   */
  enrichRecordsWithSource(records) {
    return records.map(record => this.addSourceIdentification(record));
  }

  /**
   * Save deduplication report
   */
  saveReport(report, filename = 'wallet-dedup-report.json') {
    const reportPath = path.join(this.dataPath, filename);

    // Ensure directory exists
    if (!fs.existsSync(this.dataPath)) {
      fs.mkdirSync(this.dataPath, { recursive: true });
    }

    fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
    return reportPath;
  }
}

module.exports = WalletDeduplicator;
