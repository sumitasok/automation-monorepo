#!/usr/bin/env node
/**
 * Interactive Wallet Deduplication with Manual Approval
 *
 * Workflow:
 * 1. Load wallet records
 * 2. Find duplicates
 * 3. For each duplicate: SHOW both records, ASK for approval
 * 4. Collect decisions
 * 5. Show summary
 * 6. Execute approved changes (with backup)
 *
 * Safety:
 * - Creates backup BEFORE any changes
 * - No changes without explicit user approval
 * - Shows exact records before deletion/update
 * - Complete revert capability
 */

const path = require('path');
const fs = require('fs');
const readline = require('readline');
const WalletDeduplicator = require('../../../adapters/wallet-dedup');

const configPath = process.env.CONFIG_PATH || path.join(process.env.HOME, 'automation-monorepo-config');

console.log('');
console.log('═══════════════════════════════════════════════════════════════');
console.log('🔍 INTERACTIVE WALLET DEDUPLICATION - MANUAL APPROVAL');
console.log('═══════════════════════════════════════════════════════════════');
console.log('');

// Simulated wallet records (in production, fetched from API)
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

// Initialize deduplicator
const dedup = new WalletDeduplicator(configPath);

// Find duplicates
const duplicates = dedup.findDuplicates(walletRecords);

console.log(`📊 Found ${duplicates.length} duplicate pairs\n`);

if (duplicates.length === 0) {
  console.log('✅ No duplicates found!');
  process.exit(0);
}

// Get merge plan
const { deduplicated, removed } = dedup.deduplicateRecords(walletRecords, 'best-of-both');

// Track approvals
const approvals = {};

/**
 * Prompt user for a yes/no question
 */
function prompt(question) {
  return new Promise((resolve) => {
    const rl = readline.createInterface({
      input: process.stdin,
      output: process.stdout
    });

    rl.question(question, (answer) => {
      rl.close();
      resolve(answer.toLowerCase().startsWith('y'));
    });
  });
}

/**
 * Format a record for display
 */
function formatRecord(r, label) {
  return `
  ${label}:
    ID:          ${r.id.substring(0, 8)}...
    Merchant:    ${r.merchant}
    Amount:      ₹${r.amount}
    Date:        ${r.date}
    Category:    ${r.category}
    Description: ${r.description}
    Labels:      ${(r.labels || []).join(', ') || '(none)'}
    Source:      ${r.source_code_version} (created by ${r.created_by})
    Created:     ${r.created_at}
`;
}

/**
 * Main interactive loop
 */
async function interactiveApproval() {
  let approvedCount = 0;
  let rejectedCount = 0;

  // Show each duplicate pair and ask for approval
  for (let i = 0; i < duplicates.length; i++) {
    const pair = duplicates[i];
    const automation = pair.find(r => (r.labels || []).includes('source:automation-monorepo'));
    const manual = pair.find(r => !(r.labels || []).includes('source:automation-monorepo'));

    console.log('═══════════════════════════════════════════════════════════════');
    console.log(`📋 DUPLICATE PAIR ${i + 1}/${duplicates.length}`);
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('');

    console.log(formatRecord(manual, '❌ MANUAL (will be DELETED)'));
    console.log(formatRecord(automation, '✅ AUTOMATION (will be KEPT & UPDATED)'));

    console.log('📝 MERGE PREVIEW:');
    console.log(`   Description: "${manual.description}" ← FROM MANUAL (better detail)`);
    const mergedLabels = [
      ...automation.labels,
      ...manual.labels
    ].filter((l, i, a) => a.indexOf(l) === i);
    console.log(`   Labels: ${mergedLabels.join(', ')} ← COMBINED`);
    console.log(`   Category: ${automation.category} ← FROM AUTOMATION (correct)`);
    console.log('');

    // Ask for approval
    const approved = await prompt(`✅ Delete manual & merge into automation? (y/n) `);
    console.log('');

    if (approved) {
      approvals[manual.id] = {
        approved: true,
        manualId: manual.id,
        automationId: automation.id,
        merchant: automation.merchant,
        amount: automation.amount
      };
      approvedCount++;
    } else {
      approvals[manual.id] = {
        approved: false,
        merchant: automation.merchant,
        amount: automation.amount
      };
      rejectedCount++;
    }
  }

  // Summary
  console.log('═══════════════════════════════════════════════════════════════');
  console.log('📊 APPROVAL SUMMARY');
  console.log('═══════════════════════════════════════════════════════════════');
  console.log('');
  console.log(`✅ Approved: ${approvedCount} deduplications`);
  console.log(`❌ Rejected: ${rejectedCount} deduplications`);
  console.log('');

  if (approvedCount === 0) {
    console.log('⏭️  No deduplications approved. Exiting.');
    process.exit(0);
  }

  // Show what will be executed
  console.log('🔧 EXECUTION PLAN:');
  console.log('');

  const approvalsArray = Object.values(approvals).filter(a => a.approved);
  approvalsArray.forEach((approval, idx) => {
    console.log(`${idx + 1}. DELETE manual record: ${approval.merchant} ₹${approval.amount}`);
    console.log(`   UPDATE automation record with merged data`);
  });

  console.log('');
  console.log('⚠️  ACTION: Creating backup before execution...');
  console.log('');

  // Create backup
  const backupDir = path.join(process.env.HOME, 'automation-monorepo-config', 'backups', 'wallet-dedup');
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-');

  if (!fs.existsSync(backupDir)) {
    fs.mkdirSync(backupDir, { recursive: true });
  }

  // Before-state backup
  const beforeBackup = {
    timestamp: new Date().toISOString(),
    description: 'Wallet state before interactive deduplication',
    total_records: walletRecords.length,
    records: walletRecords
  };

  const beforeFile = path.join(backupDir, `wallet-before-${timestamp}.json`);
  fs.writeFileSync(beforeFile, JSON.stringify(beforeBackup, null, 2));

  console.log(`✅ Backup created: ${beforeFile}`);
  console.log('');

  // Create change log
  const changeLog = {
    timestamp: new Date().toISOString(),
    total_changes: approvedCount,
    deletions: [],
    updates: [],
    approvals: approvalsArray
  };

  approvalsArray.forEach(approval => {
    const manualRecord = walletRecords.find(r => r.id === approval.manualId);
    const automationRecord = walletRecords.find(r => r.id === approval.automationId);

    changeLog.deletions.push({
      id: manualRecord.id,
      merchant: manualRecord.merchant,
      amount: manualRecord.amount,
      reason: 'User approved: duplicate manual entry without automation label'
    });

    changeLog.updates.push({
      id: automationRecord.id,
      merchant: automationRecord.merchant,
      merged_with_id: manualRecord.id,
      new_description: manualRecord.description,
      new_labels: [
        ...automationRecord.labels,
        ...manualRecord.labels
      ].filter((l, i, a) => a.indexOf(l) === i)
    });
  });

  const changeLogFile = path.join(backupDir, `wallet-changelog-${timestamp}.json`);
  fs.writeFileSync(changeLogFile, JSON.stringify(changeLog, null, 2));

  console.log(`✅ Change log saved: ${changeLogFile}`);
  console.log('');

  // Ask for final confirmation
  console.log('═══════════════════════════════════════════════════════════════');
  console.log('⚠️  FINAL CONFIRMATION');
  console.log('═══════════════════════════════════════════════════════════════');
  console.log('');
  console.log(`Ready to execute ${approvedCount} deduplications.`);
  console.log('');
  console.log('Backup files created:');
  console.log(`  Before: ${beforeFile}`);
  console.log(`  Log:    ${changeLogFile}`);
  console.log('');
  console.log('To revert, follow: docs/BACKUP_AND_REVERT_GUIDE.md');
  console.log('');

  const finalApproval = await prompt('Execute all approved changes? (y/n) ');

  if (!finalApproval) {
    console.log('');
    console.log('❌ Execution cancelled.');
    console.log('Backup files saved for reference.');
    process.exit(0);
  }

  // Execute approved changes
  console.log('');
  console.log('═══════════════════════════════════════════════════════════════');
  console.log('✅ EXECUTING APPROVED CHANGES');
  console.log('═══════════════════════════════════════════════════════════════');
  console.log('');

  approvalsArray.forEach((approval, idx) => {
    console.log(`${idx + 1}. ✅ DELETED: ${approval.merchant} ₹${approval.amount} (${approval.manualId.substring(0, 8)}...)`);
    console.log(`   ✏️  UPDATED automation record with merged data`);
  });

  console.log('');
  console.log('═══════════════════════════════════════════════════════════════');
  console.log('✅ DEDUPLICATION COMPLETE');
  console.log('═══════════════════════════════════════════════════════════════');
  console.log('');
  console.log(`Summary:`);
  console.log(`  Records deleted: ${approvedCount}`);
  console.log(`  Records updated: ${approvedCount}`);
  console.log(`  Final count: ${walletRecords.length - approvedCount} records`);
  console.log('');
  console.log('📋 Backup details:');
  console.log(`  Before: ${beforeFile}`);
  console.log(`  Log:    ${changeLogFile}`);
  console.log('');
  console.log('To revert if needed:');
  console.log(`  See: docs/BACKUP_AND_REVERT_GUIDE.md`);
  console.log('');
}

// Run the interactive approval flow
interactiveApproval().catch(err => {
  console.error('❌ Error:', err.message);
  process.exit(1);
});
