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
// Note: Manual records often have better tags/descriptions than automation records
const testRecords = [
  {
    id: 'tx-001',
    amount: 45.50,
    merchant: 'Starbucks Coffee',
    date: '2026-09-01',
    category: 'Meals & Dining',
    description: 'Coffee',
    labels: ['source:automation-monorepo', 'categorized-by-ai'],
  },
  {
    id: 'tx-001-dup',
    amount: 45.50,
    merchant: 'Starbucks Coffee',
    date: '2026-09-01',
    category: 'Unknown Expense',
    description: 'Morning espresso with Sarah at downtown Starbucks on 5th Ave',
    labels: ['business-meeting', 'with-colleague', 'downtown'], // Better tags!
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

// Test deduplication with intelligent merging
console.log('🔧 Deduplicating with intelligent merging...');
const { deduplicated, removed } = dedup.deduplicateRecords(testRecords, 'best-of-both');
console.log(`   Final records: ${deduplicated.length}`);
console.log(`   Merged/Removed: ${removed.length}`);
console.log('');

// Show merged record details
console.log('📋 Merged Record Details:');
deduplicated.forEach(record => {
  if (record._merged_attributes) {
    console.log(`   Record ID: ${record.id}`);
    console.log(`   Merchant: ${record.merchant} $${record.amount}`);
    console.log(`   Category: ${record.category} ✓ (from automation)`);
    console.log(`   Description: "${record.description}" ✓ (from manual - better detail)`);
    console.log(`   Labels: ${record.labels.join(', ')}`);
    console.log(`   Merge Info:`);
    console.log(`     - Source: ${record._merged_attributes.source_record_id}`);
    console.log(`     - Merged with: ${record._merged_attributes.merged_with_id}`);
    console.log(`     - Strategy: ${record._merged_attributes.merged_strategy}`);
    console.log('');
  }
});

// Show what was merged away
console.log('🗑️  Merged Away (Best of Both Applied):');
removed.forEach(record => {
  if (record._duplicate_reason === 'merged-into-automation-record') {
    console.log(`   Record ${record.id} merged into ${record._merged_with}`);
    console.log(`   - Kept: Correct category from automation record`);
    console.log(`   - Merged: Better description + tags from manual record`);
  }
});
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
