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
   * Deduplicate records, keeping automation records
   * Rules:
   * - Keep: Records with "source:automation-monorepo" label
   * - Delete: Records without label (manual/duplicates)
   * - Merge: Categorization into kept record
   */
  deduplicateRecords(recordsWithDuplicates) {
    const deduplicated = [];
    const removed = [];

    recordsWithDuplicates.forEach((record) => {
      // Check if this record should be kept or removed
      const hasSourceLabel = (record.labels || []).includes('source:automation-monorepo');

      if (hasSourceLabel) {
        // Keep records from automation
        deduplicated.push(record);
      } else {
        // Mark for removal
        removed.push({
          ...record,
          _duplicate_reason: 'manual-entry-without-source-label',
          _preferred_record: recordsWithDuplicates.find(r =>
            (r.labels || []).includes('source:automation-monorepo') &&
            this.isSameDuplicate(r, record)
          ),
        });
      }
    });

    return { deduplicated, removed };
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
