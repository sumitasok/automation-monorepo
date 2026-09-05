#!/usr/bin/env node
/**
 * Safe Wallet Deduplication with Backup & Source Tracking
 *
 * SAFETY FEATURES:
 * 1. Backup current state before any changes
 * 2. Track source code version for each record
 * 3. Generate revert instructions if needed
 * 4. Log all changes with timestamps
 */

const fs = require('fs');
const path = require('path');
const WalletDeduplicator = require('../packs/expense-domain/adapters/wallet-dedup');

console.log('');
console.log('═══════════════════════════════════════════════════════════════');
console.log('🛡️  SAFE WALLET DEDUPLICATION WITH BACKUP');
console.log('═══════════════════════════════════════════════════════════════');
console.log('');

// Simulate real wallet records (in production, fetched from API)
const walletRecords = [
  {
    id: "39629ad1-dfe9-47a8-bddd-aca5daf90318",
    merchant: "Zomato",
    amount: 868.76,
    date: "2026-09-05",
    category: "Food & Drinks",
    description: "Zomato | via Canara CC x6102 | gmail-sync gm:1a07051c0102f33e",
    labels: ["Chinju", "Ordering in outside food", "Dinner"],
    source_code_version: "unknown-manual-entry",
    created_by: "manual-web-entry",
    created_at: "2026-09-05T07:39:56.622Z"
  },
  {
    id: "e32dcffe-1d7f-4602-81de-970efc6a4258",
    merchant: "ZOMATO",
    amount: 985.42,
    date: "2026-09-05",
    category: "Unknown expense",
    description: "ZOMATO | via Canara CC X6102 | gm:1a070cb133b75004",
    labels: ["Chinju", "Ordering in outside food", "Dinner"],
    source_code_version: "unknown-manual-entry",
    created_by: "manual-web-entry",
    created_at: "2026-09-05T12:38:45.987Z"
  },
  {
    id: "e8156d87-ff75-4a7e-9fcb-660850e08f30",
    merchant: "Blinkit",
    amount: 607,
    date: "2026-09-05",
    category: "Unknown expense",
    description: "Blinkit | via Canara CC X6102 | gm:1a071131768ac83e",
    labels: ["Blinkit"],
    source_code_version: "unknown-manual-entry",
    created_by: "manual-web-entry",
    created_at: "2026-09-05T12:38:45.989Z"
  },
  {
    id: "e5b1d60e-be9a-438c-9705-fa16e24a1dfe",
    merchant: "Blinkit",
    amount: 607,
    date: "2026-09-05",
    category: "Food & Drinks",
    description: "Blinkit [gmail-csv 1a071131768ac83e]",
    labels: ["source:automation-monorepo"],
    source_code_version: "restructure-architecture-worktree",
    created_by: "framework-gmail-sync",
    created_at: "2026-09-05T10:35:25.708Z"
  },
  {
    id: "f3893584-a7a2-426f-9d43-27b4b36be499",
    merchant: "ZOMATO",
    amount: 985.42,
    date: "2026-09-05",
    category: "Food & Drinks",
    description: "ZOMATO [gmail-csv 1a070cb133b75004]",
    labels: ["source:automation-monorepo"],
    source_code_version: "restructure-architecture-worktree",
    created_by: "framework-gmail-sync",
    created_at: "2026-09-05T10:35:25.709Z"
  },
  {
    id: "67fc052c-c633-4965-95ff-eb25d93c330e",
    merchant: "Zomato",
    amount: 868.76,
    date: "2026-09-05",
    category: "Food & Drinks",
    description: "Zomato [gmail-csv 1a07051c0102f33e]",
    labels: ["source:automation-monorepo"],
    source_code_version: "restructure-architecture-worktree",
    created_by: "framework-gmail-sync",
    created_at: "2026-09-05T10:35:25.711Z"
  }
];

const backupDir = path.join(process.env.HOME, 'automation-monorepo-config', 'backups', 'wallet-dedup');
const timestamp = new Date().toISOString().replace(/[:.]/g, '-');

// Ensure backup directory exists
if (!fs.existsSync(backupDir)) {
  fs.mkdirSync(backupDir, { recursive: true });
}

console.log(`💾 Creating backup at: ${backupDir}`);
console.log('');

// 1. BACKUP BEFORE STATE
const beforeBackup = {
  timestamp: new Date().toISOString(),
  description: 'Wallet state before deduplication',
  total_records: walletRecords.length,
  records: walletRecords,
  analysis: {
    by_source: {},
    by_category: {}
  }
};

// Analyze current state
walletRecords.forEach(r => {
  // By source code version
  if (!beforeBackup.analysis.by_source[r.source_code_version]) {
    beforeBackup.analysis.by_source[r.source_code_version] = [];
  }
  beforeBackup.analysis.by_source[r.source_code_version].push({
    id: r.id,
    merchant: r.merchant,
    amount: r.amount,
    created_by: r.created_by
  });

  // By category
  if (!beforeBackup.analysis.by_category[r.category]) {
    beforeBackup.analysis.by_category[r.category] = 0;
  }
  beforeBackup.analysis.by_category[r.category]++;
});

const beforeFile = path.join(backupDir, `wallet-before-${timestamp}.json`);
fs.writeFileSync(beforeFile, JSON.stringify(beforeBackup, null, 2));

console.log('✅ BEFORE backup saved');
console.log('');

// 2. ANALYZE WITH SOURCE TRACKING
const dedup = new WalletDeduplicator(process.env.CONFIG_PATH || path.join(process.env.HOME, 'automation-monorepo-config'));
const duplicates = dedup.findDuplicates(walletRecords);
const { deduplicated, removed } = dedup.deduplicateRecords(walletRecords, 'best-of-both');

console.log('🔍 DEDUPLICATION ANALYSIS');
console.log('');

// 3. TRACE SOURCE OF EACH ACTION
console.log('📋 CHANGES TO BE MADE:');
console.log('');

const changeLog = {
  timestamp: new Date().toISOString(),
  total_changes: removed.length,
  deletions: [],
  updates: [],
  revert_instructions: []
};

removed.forEach((record, idx) => {
  console.log(`${idx + 1}. DELETE: ${record.id.substring(0, 8)}...`);
  console.log(`   Merchant: ${record.merchant} ₹${record.amount}`);
  console.log(`   Created by: ${record.created_by}`);
  console.log(`   Source version: ${record.source_code_version}`);
  console.log(`   Created at: ${record.created_at}`);
  console.log('');

  changeLog.deletions.push({
    id: record.id,
    merchant: record.merchant,
    amount: record.amount,
    created_by: record.created_by,
    source_version: record.source_code_version,
    created_at: record.created_at,
    reason: 'Duplicate without source:automation-monorepo label',
    revert_command: `curl -X POST https://api.wallet.example.com/records -d '${JSON.stringify(record)}'`
  });
});

deduplicated.forEach((record, idx) => {
  if (record._merged_attributes) {
    console.log(`${idx + 1}. UPDATE: ${record.id.substring(0, 8)}...`);
    console.log(`   Merchant: ${record.merchant}`);
    console.log(`   Changes: Description + Labels merged`);
    console.log(`   Merged with: ${record._merged_attributes.merged_with_id.substring(0, 8)}...`);
    console.log('');

    changeLog.updates.push({
      id: record.id,
      merchant: record.merchant,
      merged_with_id: record._merged_attributes.merged_with_id,
      new_description: record.description,
      new_labels: record.labels,
      revert_command: `curl -X PATCH https://api.wallet.example.com/records/${record.id} -d '{"description":"<original>", "labels":[]}'`
    });
  }
});

// 4. GENERATE REVERT INSTRUCTIONS
console.log('═══════════════════════════════════════════════════════════════');
console.log('⚠️  REVERT INSTRUCTIONS (if needed)');
console.log('═══════════════════════════════════════════════════════════════');
console.log('');

changeLog.revert_instructions = {
  note: 'If deduplication causes issues, use these commands to restore',
  step_1: 'Delete the updated records (if created)',
  step_2: 'Restore deleted records from backup JSON',
  backup_file: beforeFile,
  sql_restore: `DELETE FROM wallet_records WHERE id IN (${changeLog.deletions.map(d => `'${d.id}'`).join(', ')});`,
  restore_from_backup: `cat ${beforeFile} | jq '.records[] | select(.id | IN(${changeLog.deletions.map(d => `"${d.id}"`).join(',')}))' > restore.json`
};

console.log(`If issues occur, restore from backup:`);
console.log(`  Backup file: ${beforeFile}`);
console.log('');

// 5. SAVE CHANGELOG
const changeLogFile = path.join(backupDir, `wallet-changelog-${timestamp}.json`);
fs.writeFileSync(changeLogFile, JSON.stringify(changeLog, null, 2));

console.log('═══════════════════════════════════════════════════════════════');
console.log('✅ READY FOR EXECUTION');
console.log('═══════════════════════════════════════════════════════════════');
console.log('');
console.log('Safety Features:');
console.log(`  ✅ Before backup: ${beforeFile}`);
console.log(`  ✅ Change log: ${changeLogFile}`);
console.log(`  ✅ Source tracking: All records traced to origin`);
console.log(`  ✅ Revert instructions: Generated and saved`);
console.log('');
console.log('Summary:');
console.log(`  📊 Records before: ${beforeBackup.total_records}`);
console.log(`  📊 Records after: ${deduplicated.length}`);
console.log(`  🗑️  To delete: ${changeLog.deletions.length}`);
console.log(`  ✏️  To update: ${changeLog.updates.length}`);
console.log('');
console.log('By Source Code Version:');
Object.entries(beforeBackup.analysis.by_source).forEach(([version, records]) => {
  console.log(`  ${version}: ${records.length} records`);
});
console.log('');
console.log('To execute deduplication:');
console.log('  SKIP_CONFIRMATION=true node scripts/deduplicate-real-wallet.js');
console.log('');
console.log('To revert if needed:');
console.log(`  1. Read backup: cat ${beforeFile}`);
console.log(`  2. Restore records using commands in ${changeLogFile}`);
console.log('');
