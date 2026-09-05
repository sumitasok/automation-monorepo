#!/usr/bin/env node
/**
 * Deduplicate Real Wallet Records
 * Fetches from Budget Bakers Wallet API and removes duplicates
 */

const https = require('https');
const path = require('path');
const WalletDeduplicator = require('../packs/expense-domain/adapters/wallet-dedup');

console.log('');
console.log('═══════════════════════════════════════════════════════════════');
console.log('🗑️  DEDUPLICATING REAL WALLET RECORDS');
console.log('═══════════════════════════════════════════════════════════════');
console.log('');

// Get credentials from external config
const configPath = process.env.CONFIG_PATH || path.join(process.env.HOME, 'automation-monorepo-config');
const walletConfigPath = path.join(configPath, 'config', 'wallet', 'config.yaml');

let walletToken = process.env.WALLET_API_TOKEN;
let walletBaseUrl = process.env.WALLET_BASE_URL || 'https://rest.budgetbakers.com/wallet';
const walletLabel = 'source:automation-monorepo';

// Try to load from config file if not in env
if (!walletToken) {
  try {
    const yaml = require('js-yaml');
    const fs = require('fs');
    const config = yaml.load(fs.readFileSync(walletConfigPath, 'utf8'));
    walletToken = config.env?.WALLET_API_TOKEN;
    walletBaseUrl = config.env?.WALLET_BASE_URL || walletBaseUrl;
  } catch (e) {
    // Config file might not exist, that's ok - user will provide via env
  }
}

console.log(`🔐 Wallet API: ${walletBaseUrl}`);
console.log(`🏷️  Label: ${walletLabel}`);
console.log('');

if (!walletToken) {
  console.error('❌ WALLET_API_TOKEN not found in environment or config');
  process.exit(1);
}

/**
 * Fetch wallet records from Wallet API
 */
function fetchWalletRecords() {
  return new Promise((resolve, reject) => {
    console.log('📥 Fetching wallet records...');

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

/**
 * Delete a wallet record
 */
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
          reject(new Error(`Delete failed ${res.statusCode}`));
        }
      });
    });

    req.on('error', reject);
    req.end();
  });
}

/**
 * Update a wallet record
 */
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
          reject(new Error(`Update failed ${res.statusCode}`));
        }
      });
    });

    req.on('error', reject);
    req.write(payload);
    req.end();
  });
}

/**
 * Main execution
 */
async function main() {
  try {
    // Fetch real records
    const records = await fetchWalletRecords();
    console.log('');

    // Analyze for duplicates
    const dedup = new WalletDeduplicator(process.env.CONFIG_PATH || '/Users/sumitasok/automation-monorepo-config');
    const duplicates = dedup.findDuplicates(records);

    console.log(`🔍 Found ${duplicates.length} duplicate pairs`);
    console.log('');

    if (duplicates.length === 0) {
      console.log('✅ No duplicates found!');
      process.exit(0);
    }

    // Get merge instructions
    const { deduplicated, removed } = dedup.deduplicateRecords(records, 'best-of-both');

    console.log(`📊 Deduplication Plan:`);
    console.log(`   Records to delete: ${removed.length}`);
    console.log(`   Records to update: ${removed.length}`);
    console.log(`   Final records: ${deduplicated.length}`);
    console.log('');

    // Show what will happen
    console.log('📋 Records to be deleted (without source label):');
    removed.forEach(r => {
      console.log(`   ❌ ${r.id}: ${r.merchant} $${r.amount}`);
    });
    console.log('');

    console.log('📝 Records to be updated (merged with better data):');
    deduplicated.forEach(r => {
      if (r._merged_attributes) {
        console.log(`   ✏️  ${r.id}: ${r.merchant}`);
        console.log(`       Description: "${r.description}"`);
        console.log(`       Labels: ${r.labels.join(', ')}`);
      }
    });
    console.log('');

    // Ask for confirmation
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('⚠️  DESTRUCTIVE OPERATION');
    console.log('═══════════════════════════════════════════════════════════════');
    console.log('');
    console.log('This will PERMANENTLY DELETE records from your wallet.');
    console.log('');

    // For automation, check if environment says to proceed
    const SKIP_CONFIRMATION = process.env.SKIP_CONFIRMATION === 'true';

    if (SKIP_CONFIRMATION) {
      console.log('⏭️  SKIP_CONFIRMATION=true - Proceeding with deduplication...');
      console.log('');

      // Execute deletions
      console.log('🗑️  Deleting duplicate records...');
      for (const record of removed) {
        try {
          await deleteRecord(record.id);
          console.log(`   ✅ Deleted ${record.id}`);
        } catch (e) {
          console.error(`   ❌ Failed to delete ${record.id}: ${e.message}`);
        }
      }
      console.log('');

      // Execute updates
      console.log('📝 Updating kept records with merged data...');
      for (const record of deduplicated) {
        if (record._merged_attributes) {
          try {
            await updateRecord(record.id, {
              description: record.description,
              labels: record.labels,
            });
            console.log(`   ✅ Updated ${record.id}`);
          } catch (e) {
            console.error(`   ❌ Failed to update ${record.id}: ${e.message}`);
          }
        }
      }
      console.log('');

      console.log('═══════════════════════════════════════════════════════════════');
      console.log('✅ DEDUPLICATION COMPLETE');
      console.log('═══════════════════════════════════════════════════════════════');
      console.log('');
      console.log('Summary:');
      console.log(`  ✅ Deleted ${removed.length} duplicate records`);
      console.log(`  ✅ Updated ${deduplicated.filter(r => r._merged_attributes).length} records with merged data`);
      console.log(`  ✅ Final count: ${deduplicated.length} records`);
      console.log('');
    } else {
      console.log('To execute deduplication, run with:');
      console.log('  SKIP_CONFIRMATION=true node scripts/deduplicate-real-wallet.js');
      console.log('');
      console.log('Or set WALLET_API_TOKEN and WALLET_BASE_URL:');
      console.log('  export WALLET_API_TOKEN="your-token"');
      console.log('  node scripts/deduplicate-real-wallet.js');
      console.log('');
    }
  } catch (error) {
    console.error('❌ Error:', error.message);
    process.exit(1);
  }
}

main();
