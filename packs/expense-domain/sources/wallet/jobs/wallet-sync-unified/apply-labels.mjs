#!/usr/bin/env node
// apply-labels.mjs — One-shot script: fetch/create BudgetBakers labels, scan vault for tags,
// then patch wallet records with matching label IDs.
//
// Run from vault root:
//   WALLET_AUTH_HEADER="Bearer <token>" node _db/wallet-sync/apply-labels.mjs
//
// Or dry-run (no writes):
//   DRY_RUN=1 WALLET_AUTH_HEADER="Bearer <token>" node _db/wallet-sync/apply-labels.mjs

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const VAULT = path.resolve(__dirname, "../..");
const CACHE_FILE = path.join(__dirname, "labels-cache.json");

const BASE = "https://rest.budgetbakers.com/wallet/v1/api";
const AUTH = process.env.WALLET_AUTH_HEADER || "";
const DRY_RUN = !!process.env.DRY_RUN;

if (!AUTH) {
  console.error("ERROR: WALLET_AUTH_HEADER not set.");
  process.exit(1);
}

// ─── API helper ──────────────────────────────────────────────────────────────

async function api(method, path_, body, query) {
  let url = BASE + path_;
  if (query) {
    const parts = [];
    for (const [k, v] of Object.entries(query)) {
      if (v == null) continue;
      for (const item of Array.isArray(v) ? v : [v])
        parts.push(`${encodeURIComponent(k)}=${encodeURIComponent(item)}`);
    }
    if (parts.length) url += "?" + parts.join("&");
  }
  const res = await fetch(url, {
    method,
    headers: {
      Authorization: AUTH,
      Accept: "application/json",
      ...(body ? { "Content-Type": "application/json" } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  let json;
  try { json = JSON.parse(text); } catch { json = { raw: text }; }
  if (res.status >= 400) {
    console.error(`API error ${res.status} ${method} ${path_}:`, JSON.stringify(json).substring(0, 300));
  }
  return { status: res.status, body: json };
}

// ─── Labels ───────────────────────────────────────────────────────────────────

// Master list of labels we want in the wallet. Grouped for readability.
// Format: [tagSlug, displayName, hexColor]
const DESIRED_LABELS = [
  // Expense categories
  ["dining",          "Dining",           "#E8622A"],
  ["transport",       "Transport",        "#4A90D9"],
  ["shopping",        "Shopping",         "#9B59B6"],
  ["accommodation",   "Accommodation",    "#2ECC71"],
  ["entertainment",   "Entertainment",    "#F39C12"],
  ["electronics",     "Electronics",      "#1ABC9C"],
  ["groceries",       "Groceries",        "#27AE60"],
  ["clothing",        "Clothing",         "#E91E63"],
  ["subscriptions",   "Subscriptions",    "#8E44AD"],
  ["home-automation", "Home Automation",  "#16A085"],
  ["home-appliances", "Home Appliances",  "#2980B9"],
  ["utilities",       "Utilities",        "#7F8C8D"],
  ["maintenance",     "Maintenance",      "#95A5A6"],
  ["travel-gear",     "Travel Gear",      "#D35400"],
  ["tickets",         "Tickets",          "#C0392B"],
  ["taxi",            "Taxi",             "#F1C40F"],
  ["car-rental",      "Car Rental",       "#BDC3C7"],
  ["fuel",            "Fuel",             "#E74C3C"],
  ["souvenirs",       "Souvenirs",        "#A29BFE"],
  ["food",            "Food",             "#FD79A8"],
  ["snacks",          "Snacks",           "#FDCB6E"],
  ["cafe",            "Cafe",             "#6C5CE7"],
  ["atm",             "ATM",              "#636E72"],
  ["cash-withdrawal", "Cash Withdrawal",  "#2D3436"],
  ["forex",           "Forex",            "#00B894"],
  ["kitchen",         "Kitchen",          "#E17055"],
  ["books",           "Books",            "#74B9FF"],
  // Payment methods
  ["cash",            "Cash",             "#B2BEC3"],
  ["hdfc",            "HDFC",             "#0052CC"],
  ["schwab",          "Schwab",           "#008069"],
  ["canara-cc",       "Canara CC",        "#FF6B35"],
  ["upi",             "UPI",              "#6C5CE7"],
  ["thomas-cook",     "Thomas Cook Forex","#FDCB6E"],
  // Purpose
  ["japan-trip",      "Japan Trip 2026",  "#E84393"],
  ["pre-trip",        "Pre-Trip",         "#FF7675"],
  ["personal",        "Personal",         "#A0A0A0"],
  ["home",            "Home",             "#55EFC4"],
  // Tax / customs
  ["tax-free",        "Tax Free",         "#00CEC9"],
  ["customs-declarable","Customs Declarable","#D63031"],
  // Key merchants
  ["amazon-india",    "Amazon India",     "#FF9900"],
  ["disney",          "Disney",           "#00B4D8"],
  ["licious",         "Licious",          "#E63946"],
  ["starbucks",       "Starbucks",        "#00704A"],
  ["uniqlo",          "Uniqlo",           "#CC0000"],
  ["bic-camera",      "BicCamera",        "#003580"],
  // Locations (Japan trip)
  ["toyama",          "Toyama",           "#DFE6E9"],
  ["tokyo",           "Tokyo",            "#DFE6E9"],
  ["osaka",           "Osaka",            "#DFE6E9"],
  ["kyoto",           "Kyoto",            "#DFE6E9"],
  ["singapore",       "Singapore",        "#DFE6E9"],
  ["haneda",          "Haneda",           "#DFE6E9"],
];

async function fetchOrCreateLabels() {
  // Load cache
  let cache = {};
  if (fs.existsSync(CACHE_FILE)) {
    cache = JSON.parse(fs.readFileSync(CACHE_FILE, "utf8"));
    console.log(`Loaded ${Object.keys(cache).length} labels from cache.`);
  }

  // Fetch current labels from API
  console.log("Fetching labels from API…");
  const res = await api("GET", "/labels", null, { limit: 200 });
  if (res.status !== 200) {
    console.error("Failed to fetch labels. Using cache only.");
    return cache;
  }

  const apiLabels = Array.isArray(res.body) ? res.body : (res.body?.data ?? []);
  console.log(`API returned ${apiLabels.length} labels.`);

  // Merge into cache (by name, case-insensitive)
  for (const lbl of apiLabels) {
    const slug = lbl.name.toLowerCase().replace(/\s+/g, "-");
    cache[lbl.name] = lbl.id;
    cache[slug] = lbl.id; // also index by slug
  }

  // Create missing labels
  for (const [slug, displayName, color] of DESIRED_LABELS) {
    if (cache[slug]) continue; // already exists

    console.log(`Creating label: ${displayName} (${slug})…`);
    if (DRY_RUN) {
      cache[slug] = `dry-run-${slug}`;
      console.log(`  [DRY RUN] would create label "${displayName}"`);
      continue;
    }

    const created = await api("POST", "/labels", { name: displayName, color });
    if (created.status === 200 || created.status === 201) {
      const id = created.body?.id ?? created.body?.[0]?.id;
      if (id) {
        cache[slug] = id;
        cache[displayName] = id;
        console.log(`  ✓ Created: ${displayName} → ${id}`);
      }
    } else {
      console.warn(`  ✗ Failed to create "${displayName}": ${JSON.stringify(created.body).substring(0, 200)}`);
    }

    // Rate limit safety
    await new Promise(r => setTimeout(r, 200));
  }

  // Save cache
  fs.writeFileSync(CACHE_FILE, JSON.stringify(cache, null, 2));
  console.log(`\nLabel cache saved to ${CACHE_FILE} (${Object.keys(cache).length} entries)\n`);
  return cache;
}

// ─── Vault scanning ──────────────────────────────────────────────────────────

function walkMd(dir) {
  const results = [];
  if (!fs.existsSync(dir)) return results;
  for (const entry of fs.readdirSync(dir)) {
    const full = path.join(dir, entry);
    const stat = fs.statSync(full);
    if (stat.isDirectory()) results.push(...walkMd(full));
    else if (entry.endsWith(".md")) results.push(full);
  }
  return results;
}

function parseFrontmatter(content) {
  const m = content.match(/^---\n([\s\S]*?)\n---/);
  if (!m) return {};
  const fm = {};
  for (const line of m[1].split("\n")) {
    const kv = line.match(/^(\w[\w_-]*):\s*(.*)/);
    if (kv) fm[kv[1]] = kv[2].trim().replace(/^["']|["']$/g, "");
  }
  // tags as array
  const tagsM = m[1].match(/^tags:\s*\[([^\]]+)\]/m);
  if (tagsM) fm.tags = tagsM[1].split(",").map(t => t.trim().replace(/^["']|["']$/g, ""));
  else fm.tags = [];
  return fm;
}

// Build gm_id → { tags, vendor, date } map from vault files
function buildGmTagMap() {
  const gmMap = {}; // gm_id → { tags: [], vendor: '', date: '' }
  const wrMap = {}; // wallet_record_id → { tags: [] }

  const files = [
    ...walkMd(path.join(VAULT, "Expenses")),
    ...walkMd(path.join(VAULT, "Banking")),
  ];

  for (const fpath of files) {
    const content = fs.readFileSync(fpath, "utf8");
    const fm = parseFrontmatter(content);

    const tags = fm.tags ?? [];
    if (tags.length === 0) continue;

    // wallet_record_id mapping
    if (fm.wallet_record_id) {
      wrMap[fm.wallet_record_id] = { tags, vendor: fm.vendor ?? "", date: fm.date ?? "" };
    }

    // gm: IDs anywhere in the file
    const gmIds = [...content.matchAll(/gm:([0-9a-f]{14,})/g)].map(m => m[1]);
    for (const gm of new Set(gmIds)) {
      if (!gmMap[gm]) gmMap[gm] = { tags: [], vendor: fm.vendor ?? "", date: fm.date ?? "" };
      // Merge tags
      for (const t of tags) {
        if (!gmMap[gm].tags.includes(t)) gmMap[gm].tags.push(t);
      }
      if (!gmMap[gm].vendor && fm.vendor) gmMap[gm].vendor = fm.vendor;
    }
  }

  // Also parse monthly expense log table rows
  const expFiles = walkMd(path.join(VAULT, "Expenses")).filter(f => f.match(/\d{4}-\d{2} \w+\.md$/));
  for (const fpath of expFiles) {
    const content = fs.readFileSync(fpath, "utf8");
    // Table rows: | date | description | category | amount | account | notes |
    for (const row of content.matchAll(/\| (\d{4}-\d{2}-\d{2}) \| (.+?) \| (.+?) \| (.+?) \| (.+?) \| (.+?) \|/g)) {
      const [, date, desc, category, , , notes] = row;
      const gmIds = [...notes.matchAll(/gm:([0-9a-f]{14,})/g)].map(m => m[1]);
      if (!gmIds.length) continue;

      const tags = categoryToTags(category.trim(), desc.trim());

      for (const gm of gmIds) {
        if (!gmMap[gm]) gmMap[gm] = { tags: [], vendor: desc.trim(), date };
        for (const t of tags) {
          if (!gmMap[gm].tags.includes(t)) gmMap[gm].tags.push(t);
        }
      }
    }
  }

  return { gmMap, wrMap };
}

// Rule-based tag inference from category/description text
function categoryToTags(category, desc) {
  const c = category.toLowerCase();
  const d = desc.toLowerCase();
  const tags = [];

  if (c.includes("grocer") || c.includes("meat") || d.includes("licious") || d.includes("blinkit")) tags.push("groceries");
  if (c.includes("dining") || c.includes("food") || c.includes("eating") || c.includes("restaurant")) tags.push("dining");
  if (c.includes("ticket") || d.includes("disney") && c.includes("ticket")) tags.push("tickets");
  if (d.includes("disney")) { tags.push("disney"); tags.push("entertainment"); tags.push("japan-trip"); }
  if (c.includes("subscription") || d.includes("claude") || d.includes("app store") || d.includes("netflix")) tags.push("subscriptions");
  if (c.includes("utilities") || c.includes("energy") || d.includes("livpure")) { tags.push("utilities"); tags.push("maintenance"); }
  if (c.includes("maintenance") || d.includes("mygate")) tags.push("maintenance");
  if (c.includes("home automation") || d.includes("aziot") || d.includes("homemate")) tags.push("home-automation");
  if (d.includes("amazon")) tags.push("amazon-india");
  if (d.includes("canara")) tags.push("canara-cc");
  if (d.includes("hdfc")) tags.push("hdfc");
  if (d.includes("upi")) tags.push("upi");

  return [...new Set(tags)];
}

// Rule-based tagging from wallet record counterParty alone
function counterPartyToTags(cp) {
  if (!cp) return [];
  const c = cp.toLowerCase();
  const tags = [];
  if (c.includes("amazon") || c.includes("asspl")) tags.push("amazon-india", "shopping");
  if (c.includes("swiggy")) tags.push("dining", "food");
  if (c.includes("zomato")) tags.push("dining", "food");
  if (c.includes("blinkit")) tags.push("groceries");
  if (c.includes("licious")) tags.push("groceries", "licious");
  if (c.includes("starbucks")) tags.push("cafe", "starbucks");
  if (c.includes("uniqlo")) tags.push("clothing", "uniqlo");
  if (c.includes("disney") || c.includes("tokyodisney")) tags.push("disney", "entertainment", "japan-trip");
  if (c.includes("familymart") || c.includes("family mart")) tags.push("convenience-store", "snacks");
  if (c.includes("lawson")) tags.push("convenience-store");
  if (c.includes("7-eleven") || c.includes("7eleven")) tags.push("convenience-store");
  if (c.includes("toyota rent")) tags.push("car-rental", "transport");
  if (c.includes("klook")) tags.push("entertainment", "tickets");
  if (c.includes("ixigo")) tags.push("transport", "flights");
  if (c.includes("go taxi") || c.includes("ola") || c.includes("uber")) tags.push("taxi", "transport");
  if (c.includes("livpure")) tags.push("utilities", "maintenance");
  if (c.includes("mygate")) tags.push("maintenance");
  if (c.includes("netflix") || c.includes("spotify") || c.includes("apple") || c.includes("claude")) tags.push("subscriptions");
  if (c.includes("atm") || c.includes("cash")) tags.push("cash", "atm", "cash-withdrawal");
  return [...new Set(tags)];
}

// ─── Main ─────────────────────────────────────────────────────────────────────

async function fetchAllRecords() {
  console.log("Fetching wallet records (last 12 months)…");
  const dateFrom = new Date();
  dateFrom.setFullYear(dateFrom.getFullYear() - 1);
  const dateFromStr = dateFrom.toISOString().split("T")[0];

  let all = [];
  let offset = 0;
  while (true) {
    const res = await api("GET", "/records", null, {
      dateFrom: dateFromStr,
      limit: 200,
      offset,
    });
    if (res.status !== 200) { console.error("Error fetching records"); break; }
    const recs = Array.isArray(res.body) ? res.body : (res.body?.data ?? []);
    if (!recs.length) break;
    all.push(...recs);
    console.log(`  Fetched ${all.length} records so far…`);
    if (recs.length < 200) break;
    offset += 200;
    await new Promise(r => setTimeout(r, 300));
  }
  console.log(`Total records fetched: ${all.length}\n`);
  return all;
}

function tagsToLabelIds(tags, cache) {
  const ids = [];
  for (const tag of tags) {
    const id = cache[tag];
    if (id && !id.startsWith("dry-run")) ids.push(id);
  }
  return [...new Set(ids)];
}

async function main() {
  console.log(`\n=== apply-labels.mjs ${DRY_RUN ? "[DRY RUN]" : ""} ===\n`);

  // Step 1: fetch/create labels
  const cache = await fetchOrCreateLabels();

  // Step 2: build gm→tags map from vault
  console.log("Scanning vault for tags…");
  const { gmMap, wrMap } = buildGmTagMap();
  console.log(`  gm: IDs mapped: ${Object.keys(gmMap).length}`);
  console.log(`  wallet_record_ids mapped: ${Object.keys(wrMap).length}\n`);

  // Step 3: fetch all wallet records
  const records = await fetchAllRecords();

  // Step 4: build patch list
  const patches = [];
  let matchedByGm = 0, matchedByWrid = 0, matchedByRule = 0, skipped = 0;

  for (const rec of records) {
    const existingLabelIds = rec.labelIds ?? [];

    // Find gm: ID in record note
    const gmMatch = (rec.note ?? "").match(/gm:([0-9a-f]{14,})/);
    const gmId = gmMatch ? gmMatch[1] : null;

    let tags = [];
    let source = "none";

    if (gmId && gmMap[gmId]) {
      tags = gmMap[gmId].tags;
      source = "gm";
      matchedByGm++;
    } else if (rec.id && wrMap[rec.id]) {
      tags = wrMap[rec.id].tags;
      source = "wrid";
      matchedByWrid++;
    } else {
      // Rule-based fallback from counterParty
      tags = counterPartyToTags(rec.counterParty ?? "");
      if (tags.length) { source = "rule"; matchedByRule++; }
      else { skipped++; continue; }
    }

    const newLabelIds = tagsToLabelIds(tags, cache);
    if (!newLabelIds.length) { skipped++; continue; }

    // Merge with existing labels (don't remove existing ones)
    const merged = [...new Set([...existingLabelIds, ...newLabelIds])];
    if (merged.length === existingLabelIds.length &&
        merged.every(id => existingLabelIds.includes(id))) {
      skipped++;
      continue; // nothing new to add
    }

    patches.push({
      id: rec.id,
      labelIds: merged,
      _debug: { source, tags, gmId, counterParty: rec.counterParty }
    });
  }

  console.log(`\nPatch plan:`);
  console.log(`  Matched by gm: ID:       ${matchedByGm}`);
  console.log(`  Matched by wallet_record_id: ${matchedByWrid}`);
  console.log(`  Matched by rule (counterParty): ${matchedByRule}`);
  console.log(`  Skipped (no match / nothing new): ${skipped}`);
  console.log(`  Records to patch: ${patches.length}\n`);

  if (!patches.length) {
    console.log("Nothing to patch. Done.");
    return;
  }

  // Print what we're about to do
  for (const p of patches.slice(0, 20)) {
    const lnames = p.labelIds.map(id => {
      const entry = Object.entries(cache).find(([k, v]) => v === id);
      return entry ? entry[0] : id;
    });
    console.log(`  [${p._debug.source}] ${p.id} | tags: ${p._debug.tags.join(", ")} → labels: ${lnames.join(", ")}`);
  }
  if (patches.length > 20) console.log(`  … and ${patches.length - 20} more`);
  console.log();

  if (DRY_RUN) {
    console.log("[DRY RUN] No changes written.");
    return;
  }

  // Step 5: patch in batches of 10
  let patched = 0;
  for (let i = 0; i < patches.length; i += 10) {
    const batch = patches.slice(i, i + 10).map(p => ({
      id: p.id,
      labelIds: p.labelIds,
    }));
    console.log(`Patching batch ${Math.floor(i/10)+1}/${Math.ceil(patches.length/10)}…`);
    const res = await api("PATCH", "/records", batch);
    if (res.status >= 400) {
      console.error(`  Batch failed: ${JSON.stringify(res.body).substring(0, 200)}`);
    } else {
      patched += batch.length;
      console.log(`  ✓ Patched ${batch.length} records`);
    }
    await new Promise(r => setTimeout(r, 350)); // stay well inside 300 req/hr
  }

  console.log(`\n✓ Done. ${patched}/${patches.length} records patched.`);
}

main().catch(e => { console.error(e); process.exit(1); });
