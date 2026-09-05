#!/usr/bin/env node
/**
 * Execute Approved Wallet Deduplication Against Real Wallet API
 *
 * This script:
 * 1. Fetches REAL wallet records from Budget Bakers Wallet API
 * 2. Finds duplicates
 * 3. For each duplicate: shows it, asks for approval
 * 4. Executes REAL deletions and updates against Wallet API
 * 5. Creates backup before any changes
 *
 * Requires: WALLET_API_TOKEN environment variable
 */

const https = require('https');
const path = require('path');
const fs = require('fs');
const readline = require('readline');
const WalletDeduplicator = require('../../../adapters/wallet-dedup');

const configPath = process.env.CONFIG_PATH || path.join(process.env.HOME, 'automation-monorepo-config');

// Get API credentials from environment or config
let walletToken = process.env.WALLET_API_TOKEN;
let walletBaseUrl = process.env.WALLET_BASE_URL || 'https://rest.budgetbakers.com/wallet';

// Try to load from config if not in env
if (!walletToken) {
  try {
    const yaml = require('js-yaml');
    const configFile = path.join(configPath, 'config', 'wallet', 'config.yaml');
    const config = yaml.load(fs.readFileSync(configFile, 'utf8'));
    walletToken = config.env?.WALLET_API_TOKEN;
    walletBaseUrl = config.env?.WALLET_BASE_URL || walletBaseUrl;
  } catch (e) {
    // Config file might not exist
  }
}

console.log('');
console.log('═══════════════════════════════════════════════════════════════');
console.log('🌐 REAL WALLET DEDUPLICATION - LIVE API EXECUTION');
console.log('═══════════════════════════════════════════════════════════════');
console.log('');

if (!walletToken) {
  console.error('❌ WALLET_API_TOKEN not found');
  console.error('');
  console.error('Set it with:');
  console.error('  export WALLET_API_TOKEN="your-premium-api-token"');
  process.exit(1);
}

console.log(`🔐 Wallet API: ${walletBaseUrl}`);
console.log('');

// Fetch records from API
function fetchRecords() {
  return new Promise((resolve, reject) => {
    console.log('📥 Fetching wallet records from Wallet API...');

    const url = new URL('/records', walletBaseUrl);
    const options = {
      hostname: url.hostname,
      port: url.port || 443,
      path: url.pathname + url.search,
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${walletToken}`,
        'Accept': 'application/json',
        'Content-Type': 'application/json',
      },
    };

    const req = https.request(options, (res) => {
      let data = '';

      res.on('data', (chunk) => {
        data += chunk;
      });

      res.on('end', () => {
        if (res.statusCode === 200) {
          try {
            const records = JSON.parse(data);
            console.log(`✅ Fetched ${records.length} records`);
            resolve(records);
          } catch (e) {
            reject(new Error(`Failed to parse response: ${e.message}`));
          }
        } else {
          reject(new Error(`API Error ${res.statusCode}: ${data}`));
        }
      });
    });

    req.on('error', reject);
    req.end();
  });
}

// Delete a record from Wallet API
function deleteRecord(recordId) {
  return new Promise((resolve, reject) => {
    const url = new URL(`/records/${recordId}`, walletBaseUrl);
    const options = {
      hostname: url.hostname,
      port: url.port || 443,
      path: url.pathname,
      method: 'DELETE',
      headers: {
        'Authorization': `Bearer ${walletToken}`,
        'Content-Type': 'application/json',
      },
    };

    const req = https.request(options, (res) => {
      let data = '';
      res.on('data', (chunk) => { data += chunk; });
      res.on('end', () => {
        if (res.statusCode === 200 || res.statusCode === 204) {
          resolve();
        } else {
          reject(new Error(`Delete failed ${res.statusCode}: ${data}`));
        }
      });
    });

    req.on('error', reject);
    req.end();
  });
}

// Update a record in Wallet API
function updateRecord(recordId, data) {
  return new Promise((resolve, reject) => {
    const url = new URL(`/records/${recordId}`, walletBaseUrl);
    const payload = JSON.stringify(data);

    const options = {
      hostname: url.hostname,
      port: url.port || 443,
      path: url.pathname,
      method: 'PATCH',
      headers: {
        'Authorization': `Bearer ${walletToken}`,
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(payload),
      },
    };

    const req = https.request(options, (res) => {
      let data = '';
      res.on('data', (chunk) => { data += chunk; });
      res.on('end', () => {
        if (res.statusCode === 200) {
          resolve();
        } else {
          reject(new Error(`Update failed ${res.statusCode}: ${data}`));
        }
      });
    });

    req.on('error', reject);
    req.write(payload);
    req.end();
  });
}

// Prompt for yes/no
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

// Format record for display
function formatRecord(r, label) {
  return `
  ${label}:
    ID:          ${r.id.substring(0, 8)}...
    Merchant:    ${r.merchant}
    Amount:      ₹${r.amount}
    Date:        ${r.recordDate}
    Category:    ${r.categoryName || 'Uncategorized'}
    Description: ${r.description}
    Labels:      ${(r.labels || []).join(', ') || '(none)'}
    Created:     ${r.createdAt}
`;
}

// Main execution
async function executeWithApprovals() {
  try {
    // Fetch real records
    const records = await fetchRecords();
    console.log('');

    if (records.length === 0) {
      console.log('✅ No records to deduplicate');
      process.exit(0);
    }

    // Initialize deduplicator
    const dedup = new WalletDeduplicator(configPath);

    // Find duplicates
    const duplicates = dedup.findDuplicates(records);
    console.log(`🔍 Found ${duplicates.length} duplicate pairs\n`);

    if (duplicates.length === 0) {
      console.log('✅ No duplicates found!');
      process.exit(0);
    }

    // Get merge plan
    const { deduplicated, removed } = dedup.deduplicateRecords(records, 'best-of-both');

    // Interactive approval loop
    const approvals = {};
    let approvedCount = 0;

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
      console.log(`   Description: "${manual.description}" ← FROM MANUAL`);
      const mergedLabels = [
        ...(automation.labels || []),
        ...(manual.labels || [])
      ].filter((l, i, a) => a.indexOf(l) === i);
      console.log(`   Labels: ${mergedLabels.join(', ')} ← COMBINED`);
      console.log(`   Category: ${automation.categoryName || 'Uncategorized'} ← FROM AUTOMATION`);
      console.log('');

      const approved = await prompt(`✅ Delete manual & merge into automation? (y/n) `);
      console.log('');

      if (approved) {
        approvals[manual.id] = {
          approved: true,
          manualId: manual.id,
          manualRecord: manual,
          automationId: automation.id,
          automationRecord: automation,
          mergedLabels: mergedLabels,
          merchant: automation.merchant,
          amount: automation.amount
        };
        approvedCount++;
      }
    }

    // Summary
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('📊 APPROVAL SUMMARY');
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('');
    console.log(`✅ Approved: ${approvedCount} deduplications`);
    console.log(`❌ Rejected: ${duplicates.length - approvedCount} deduplications`);
    console.log('');

    if (approvedCount === 0) {
      console.log('⏭️  No deduplications approved. Exiting.');
      process.exit(0);
    }

    // Create backup directory
    const backupDir = path.join(process.env.HOME, 'automation-monorepo-config', 'backups', 'wallet-dedup');
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-');

    if (!fs.existsSync(backupDir)) {
      fs.mkdirSync(backupDir, { recursive: true });
    }

    // Before-state backup
    const beforeBackup = {
      timestamp: new Date().toISOString(),
      description: 'Wallet state before real deduplication',
      total_records: records.length,
      records: records
    };

    const beforeFile = path.join(backupDir, `wallet-before-${timestamp}.json`);
    fs.writeFileSync(beforeFile, JSON.stringify(beforeBackup, null, 2));

    console.log('🔧 EXECUTION PLAN:');
    console.log('');

    const approvalsArray = Object.values(approvals).filter(a => a.approved);
    approvalsArray.forEach((approval, idx) => {
      console.log(`${idx + 1}. DELETE: ${approval.merchant} ₹${approval.amount}`);
      console.log(`   UPDATE: automation record with merged data`);
    });

    console.log('');
    console.log('⚠️  Creating backup before execution...');
    console.log(`✅ Backup: ${beforeFile}`);
    console.log('');

    // Final confirmation
    const confirmed = await prompt('Execute all approved changes on REAL wallet? (y/n) ');
    console.log('');

    if (!confirmed) {
      console.log('❌ Execution cancelled.');
      console.log(`Backup saved: ${beforeFile}`);
      process.exit(0);
    }

    // Execute on real API
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('⚡ EXECUTING ON REAL WALLET API');
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('');

    let successCount = 0;
    let failureCount = 0;

    for (const approval of approvalsArray) {
      try {
        // 1. Delete manual record
        console.log(`🗑️  Deleting: ${approval.merchant} ₹${approval.amount}`);
        await deleteRecord(approval.manualId);
        console.log(`   ✅ Deleted ${approval.manualId.substring(0, 8)}...`);

        // 2. Update automation record with merged data
        console.log(`📝 Updating: ${approval.merchant}`);
        await updateRecord(approval.automationId, {
          description: approval.manualRecord.description,
          labels: approval.mergedLabels
        });
        console.log(`   ✅ Updated ${approval.automationId.substring(0, 8)}...`);
        console.log('');

        successCount++;
      } catch (error) {
        console.error(`   ❌ Error: ${error.message}`);
        failureCount++;
      }
    }

    // Create change log
    const changeLog = {
      timestamp: new Date().toISOString(),
      total_changes: successCount,
      executed_on: walletBaseUrl,
      deletions: approvalsArray.map(a => ({
        id: a.manualId,
        merchant: a.merchant,
        amount: a.amount
      })),
      updates: approvalsArray.map(a => ({
        id: a.automationId,
        merchant: a.merchant,
        merged_with_id: a.manualId,
        new_description: a.manualRecord.description,
        new_labels: a.mergedLabels
      }))
    };

    const changeLogFile = path.join(backupDir, `wallet-changelog-${timestamp}.json`);
    fs.writeFileSync(changeLogFile, JSON.stringify(changeLog, null, 2));

    // Summary
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('✅ DEDUPLICATION COMPLETE');
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('');
    console.log(`✅ Success: ${successCount}`);
    console.log(`❌ Failed: ${failureCount}`);
    console.log('');
    console.log('📋 Backups:');
    console.log(`  Before: ${beforeFile}`);
    console.log(`  Log:    ${changeLogFile}`);
    console.log('');
    console.log('🔍 Check Wallet app to verify changes!');
    console.log('');

  } catch (error) {
    console.error('❌ Error:', error.message);
    process.exit(1);
  }
}

// Run
executeWithApprovals();
