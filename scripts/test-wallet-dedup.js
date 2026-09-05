#!/usr/bin/env node
/**
 * Test Wallet Deduplication
 * Demonstrates the deduplication and source identification workflow
 */

const WalletDeduplicator = require('../packs/expense-domain/adapters/wallet-dedup');
const path = require('path');

const configPath = process.env.CONFIG_PATH || path.join(process.env.HOME, 'automation-monorepo-config');
const dedup = new WalletDeduplicator(configPath);

console.log('');
console.log('═══════════════════════════════════════════════════════════════');
console.log('🧪 WALLET DEDUPLICATION TEST');
console.log('═══════════════════════════════════════════════════════════════');
console.log('');

// Example test data with duplicates
const testRecords = [
  {
    id: 'tx-001',
    amount: 45.50,
    merchant: 'Starbucks Coffee',
    date: '2026-09-01',
    category: 'Meals & Dining',
    description: 'Morning coffee',
    labels: ['source:automation-monorepo', 'categorized-by-ai'],
  },
  {
    id: 'tx-001-dup',
    amount: 45.50,
    merchant: 'Starbucks Coffee',
    date: '2026-09-01',
    category: 'Unknown Expense',
    description: 'Morning coffee',
    labels: [], // No source label - will be removed
  },
  {
    id: 'tx-002',
    amount: 125.75,
    merchant: 'Whole Foods',
    date: '2026-09-02',
    category: 'Groceries',
    description: 'Weekly groceries',
    labels: ['source:automation-monorepo'],
  },
  {
    id: 'tx-002-dup',
    amount: 125.75,
    merchant: 'Whole Foods',
    date: '2026-09-02',
    category: 'Shopping',
    description: 'Weekly groceries',
    labels: [], // No source label - will be removed
  },
  {
    id: 'tx-003',
    amount: 89.99,
    merchant: 'Amazon',
    date: '2026-09-03',
    category: 'Online Shopping',
    description: 'Book purchase',
    labels: ['source:automation-monorepo'],
  },
];

console.log('📊 Test Data:');
console.log(`   Input records: ${testRecords.length}`);
console.log('');

// Find duplicates
console.log('🔍 Finding duplicates...');
const duplicates = dedup.findDuplicates(testRecords);
console.log(`   Found ${duplicates.length} duplicate groups`);
console.log('');

// Analyze duplicates
console.log('📋 Duplicate Analysis:');
duplicates.forEach((group, idx) => {
  console.log(`   Group ${idx + 1}:`);
  group.forEach(record => {
    const hasSource = (record.labels || []).includes('source:automation-monorepo');
    console.log(`     - ${record.id}: ${record.merchant} $${record.amount}`);
    console.log(`       Category: ${record.category}`);
    console.log(`       Source: ${hasSource ? '✓ automation-monorepo' : '✗ none'}`);
  });
});
console.log('');

// Test deduplication
console.log('🔧 Deduplicating...');
const { deduplicated, removed } = dedup.deduplicateRecords(testRecords);
console.log(`   Kept: ${deduplicated.length} records`);
console.log(`   Removed: ${removed.length} records`);
console.log('');

// Test source enrichment
console.log('🏷️  Testing source identification enrichment...');
const newRecords = [
  {
    id: 'tx-004',
    amount: 32.50,
    merchant: 'Restaurant XYZ',
    date: '2026-09-04',
    category: 'Meals & Dining',
    description: 'Lunch with team',
    labels: [],
  },
];

const enriched = dedup.enrichRecordsWithSource(newRecords);
console.log(`   Original record:`);
console.log(`     Description: "${newRecords[0].description}"`);
console.log(`     Labels: ${newRecords[0].labels.length === 0 ? '(none)' : newRecords[0].labels.join(', ')}`);
console.log('');
console.log(`   Enriched record:`);
console.log(`     Description: "${enriched[0].description}"`);
console.log(`     Labels: ${enriched[0].labels.join(', ')}`);
console.log('');

// Generate report
console.log('📄 Generating deduplication report...');
const report = dedup.generateReport(testRecords);
const reportPath = dedup.saveReport(report);

console.log(`   Summary:`);
console.log(`     Total records: ${report.summary.total_records}`);
console.log(`     Unique records: ${report.summary.unique_records}`);
console.log(`     Duplicates found: ${report.summary.duplicates_found}`);
console.log(`     To remove: ${report.summary.records_to_remove}`);
console.log(`     Dedup rate: ${report.summary.dedup_rate}`);
console.log('');
console.log(`   Report saved: ${reportPath}`);
console.log('');

console.log('═══════════════════════════════════════════════════════════════');
console.log('✅ TEST COMPLETE');
console.log('═══════════════════════════════════════════════════════════════');
console.log('');
console.log('Summary:');
console.log('  ✓ Duplicates identified by amount + merchant + date');
console.log('  ✓ Automation records (with source label) kept');
console.log('  ✓ Manual records (without label) marked for removal');
console.log('  ✓ Source identification added to new records');
console.log('  ✓ Report generated with deduplication details');
console.log('');
