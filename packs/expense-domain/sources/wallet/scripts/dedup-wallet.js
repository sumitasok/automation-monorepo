#!/usr/bin/env node
/**
 * One-Command Wallet Deduplication
 *
 * Usage:
 *   CONFIG_PATH=~/automation-monorepo-config node dedup-wallet.js
 *
 * This script:
 * 1. Reads config from CONFIG_PATH (loads WALLET_API_TOKEN from config/wallet/config.yaml)
 * 2. Fetches REAL wallet records from Wallet API
 * 3. Interactive approval loop (show each duplicate, ask y/n)
 * 4. Creates backup before changes
 * 5. Executes real API calls (DELETE + PATCH)
 * 6. Done!
 *
 * No environment variables needed - all from config!
 */

const https = require('https');
const path = require('path');
const fs = require('fs');
const readline = require('readline');

// Load config
const configPath = process.env.CONFIG_PATH || path.join(process.env.HOME, 'automation-monorepo-config');

if (!fs.existsSync(configPath)) {
  console.error(`❌ CONFIG_PATH not found: ${configPath}`);
  console.error('');
  console.error('Set it with:');
  console.error(`  CONFIG_PATH=~/automation-monorepo-config node dedup-wallet.js`);
  process.exit(1);
}

// Load wallet config
let walletToken, walletBaseUrl;

try {
  const yaml = require('js-yaml');
  const walletConfigFile = path.join(configPath, 'config', 'wallet', 'config.yaml');

  if (!fs.existsSync(walletConfigFile)) {
    console.error(`❌ Wallet config not found: ${walletConfigFile}`);
    process.exit(1);
  }

  const config = yaml.load(fs.readFileSync(walletConfigFile, 'utf8'));
  walletToken = config.env?.WALLET_API_TOKEN;
  walletBaseUrl = config.env?.WALLET_BASE_URL || 'https://rest.budgetbakers.com/wallet';

  if (!walletToken) {
    console.error(`❌ WALLET_API_TOKEN not found in ${walletConfigFile}`);
    process.exit(1);
  }
} catch (e) {
  console.error(`❌ Failed to load wallet config: ${e.message}`);
  process.exit(1);
}

// Load WalletDeduplicator
const WalletDeduplicator = require('../../../adapters/wallet-dedup');

console.log('');
console.log('═══════════════════════════════════════════════════════════════');
console.log('🎯 WALLET DEDUPLICATION - ONE COMMAND');
console.log('═══════════════════════════════════════════════════════════════');
console.log('');
console.log(`📁 Config: ${configPath}`);
console.log(`🔐 API: ${walletBaseUrl}`);
console.log('');

// API helpers
function fetchRecords() {
  return new Promise((resolve, reject) => {
    console.log('📥 Fetching records from Wallet API...');

    const url = new URL('/v1/api/records?limit=500', walletBaseUrl);
    const options = {
      hostname: url.hostname,
      port: url.port || 443,
      path: url.pathname + url.search,
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${walletToken}`,
        'Accept': 'application/json',
      },
    };

    const req = https.request(options, (res) => {
      let data = '';
      res.on('data', (chunk) => { data += chunk; });
      res.on('end', () => {
        if (res.statusCode === 200) {
          try {
            const records = JSON.parse(data);
            console.log(`✅ Fetched ${records.length} records`);
            resolve(records);
          } catch (e) {
            reject(new Error(`Failed to parse: ${e.message}`));
          }
        } else {
          reject(new Error(`API ${res.statusCode}: ${data}`));
        }
      });
    });

    req.on('error', reject);
    req.end();
  });
}

function deleteRecord(recordId) {
  return new Promise((resolve, reject) => {
    const url = new URL(`/v1/api/records/${recordId}`, walletBaseUrl);
    const options = {
      hostname: url.hostname,
      port: url.port || 443,
      path: url.pathname,
      method: 'DELETE',
      headers: {
        'Authorization': `Bearer ${walletToken}`,
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

function updateRecord(recordId, updateData) {
  return new Promise((resolve, reject) => {
    const url = new URL(`/v1/api/records/${recordId}`, walletBaseUrl);
    const payload = JSON.stringify(updateData);

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
`;
}

// Main
async function main() {
  try {
    // 1. Fetch real records
    const records = await fetchRecords();
    console.log('');

    if (records.length === 0) {
      console.log('✅ No records found');
      process.exit(0);
    }

    // 2. Find duplicates
    const dedup = new WalletDeduplicator(configPath);
    const duplicates = dedup.findDuplicates(records);

    console.log(`🔍 Found ${duplicates.length} duplicate pairs`);
    console.log('');

    if (duplicates.length === 0) {
      console.log('✅ No duplicates!');
      process.exit(0);
    }

    // 3. Interactive approval loop
    const approvals = [];
    let approvedCount = 0;

    for (let i = 0; i < duplicates.length; i++) {
      const pair = duplicates[i];
      const automation = pair.find(r => (r.labels || []).includes('source:automation-monorepo'));
      const manual = pair.find(r => !(r.labels || []).includes('source:automation-monorepo'));

      if (!automation || !manual) continue; // Skip if pattern doesn't match

      console.log('═══════════════════════════════════════════════════════════════');
      console.log(`📋 DUPLICATE ${i + 1}/${duplicates.length}`);
      console.log('═══════════════════════════════════════════════════════════════');

      console.log(formatRecord(manual, '❌ MANUAL (DELETE)'));
      console.log(formatRecord(automation, '✅ AUTOMATION (KEEP & UPDATE)'));

      console.log('📝 MERGE PREVIEW:');
      console.log(`   Description: "${manual.description}"`);
      const mergedLabels = [
        ...(automation.labels || []),
        ...(manual.labels || [])
      ].filter((l, i, a) => a.indexOf(l) === i);
      console.log(`   Labels: ${mergedLabels.join(', ')}`);
      console.log(`   Category: ${automation.categoryName || 'Uncategorized'}`);
      console.log('');

      const approved = await prompt(`✅ Approve? (y/n) `);
      console.log('');

      if (approved) {
        approvals.push({
          manualId: manual.id,
          manualRecord: manual,
          automationId: automation.id,
          automationRecord: automation,
          mergedLabels: mergedLabels,
          merchant: automation.merchant,
          amount: automation.amount
        });
        approvedCount++;
      }
    }

    // Summary
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('📊 SUMMARY');
    console.log('═══════════════════════════════════════════════════════════════');
    console.log(`✅ Approved: ${approvedCount}`);
    console.log(`❌ Rejected: ${duplicates.length - approvedCount}`);
    console.log('');

    if (approvedCount === 0) {
      console.log('⏭️  Exiting.');
      process.exit(0);
    }

    // 4. Create backup directory
    const backupDir = path.join(configPath, 'backups', 'wallet-dedup');
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-');

    if (!fs.existsSync(backupDir)) {
      fs.mkdirSync(backupDir, { recursive: true });
    }

    // Before-state backup
    const beforeBackup = {
      timestamp: new Date().toISOString(),
      total_records: records.length,
      records: records
    };

    const beforeFile = path.join(backupDir, `wallet-before-${timestamp}.json`);
    fs.writeFileSync(beforeFile, JSON.stringify(beforeBackup, null, 2));

    console.log('⚠️  Creating backup before execution...');
    console.log(`✅ Backup: ${beforeFile}`);
    console.log('');

    // Final confirmation
    const confirmed = await prompt('Execute on WALLET API? (y/n) ');
    console.log('');

    if (!confirmed) {
      console.log('❌ Cancelled.');
      console.log(`Backup saved: ${beforeFile}`);
      process.exit(0);
    }

    // 5. Execute API calls
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('⚡ EXECUTING ON WALLET API');
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('');

    let successCount = 0;
    let failureCount = 0;

    for (const approval of approvals) {
      try {
        console.log(`🗑️  Deleting: ${approval.merchant} ₹${approval.amount}`);
        await deleteRecord(approval.manualId);
        console.log(`   ✅ Deleted`);

        console.log(`📝 Updating: ${approval.merchant}`);
        await updateRecord(approval.automationId, {
          description: approval.manualRecord.description,
          labels: approval.mergedLabels
        });
        console.log(`   ✅ Updated`);
        console.log('');

        successCount++;
      } catch (error) {
        console.error(`   ❌ Error: ${error.message}`);
        failureCount++;
      }
    }

    // Change log
    const changeLog = {
      timestamp: new Date().toISOString(),
      executed: successCount,
      failed: failureCount,
      backup_file: beforeFile
    };

    const changeLogFile = path.join(backupDir, `wallet-changelog-${timestamp}.json`);
    fs.writeFileSync(changeLogFile, JSON.stringify(changeLog, null, 2));

    // Final summary
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('✅ COMPLETE');
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('');
    console.log(`✅ Executed: ${successCount}`);
    console.log(`❌ Failed: ${failureCount}`);
    console.log('');
    console.log('📋 Backups:');
    console.log(`  ${beforeFile}`);
    console.log(`  ${changeLogFile}`);
    console.log('');
    console.log('🔍 Check Wallet app for changes!');
    console.log('');

  } catch (error) {
    console.error('❌ Error:', error.message);
    process.exit(1);
  }
}

main();
