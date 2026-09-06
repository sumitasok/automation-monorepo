/* Wallet dashboard.
 *
 * Reads the JSON served by `wallet ui` (which in turn reads
 * ~/automation-monorepo-config/data/expense-domain/wallet) and renders it.
 * Aggregates are computed here rather than server-side so the tiles and charts
 * describe exactly the rows the active filters leave behind. No dependencies. */

'use strict';

const state = {
  records: [],
  files: [],
  issues: [],
  duplicates: { groups: [], files: [], issues: [] },
  sync: null,
  logs: [],
  sort: { key: 'recordDate', dir: -1 },
  filters: { q: '', from: '', to: '', account: '', category: '', label: '', type: '' },
  selectedLog: null,
};

const $ = (id) => document.getElementById(id);
const el = (tag, cls, text) => {
  const node = document.createElement(tag);
  if (cls) node.className = cls;
  if (text !== undefined) node.textContent = text;
  return node;
};

/* ---- formatting --------------------------------------------------------- */

const currencyOf = (records) => (records.find((r) => r.currency) || {}).currency || 'INR';

function money(value, currency, opts = {}) {
  const abs = opts.abs ? Math.abs(value) : value;
  try {
    return new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency: currency || 'INR',
      maximumFractionDigits: opts.compact ? 0 : 2,
      minimumFractionDigits: opts.compact ? 0 : 2,
      notation: opts.compact ? 'compact' : 'standard',
    }).format(abs);
  } catch {
    return `${currency || ''} ${abs.toFixed(2)}`.trim();
  }
}

const dayLabel = (iso) => {
  if (!iso) return '—';
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleDateString(undefined, { day: '2-digit', month: 'short', year: 'numeric' });
};

const stamp = (iso) => {
  if (!iso) return '—';
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
};

const bytes = (n) => (n < 1024 ? `${n} B` : n < 1024 * 1024 ? `${(n / 1024).toFixed(1)} KB` : `${(n / 1048576).toFixed(1)} MB`);

const titleCase = (s) => (s ? s.charAt(0).toUpperCase() + s.slice(1) : '');

/* ---- loading ------------------------------------------------------------ */

async function getJSON(path) {
  const res = await fetch(path, { headers: { Accept: 'application/json' } });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body.error || `${path} returned ${res.status}`);
  return body;
}

async function loadAll() {
  const errors = [];
  const settle = async (fn) => {
    try { return await fn(); } catch (err) { errors.push(err.message); return null; }
  };

  const [meta, records, duplicates, sync, logs] = await Promise.all([
    settle(() => getJSON('/api/meta')),
    settle(() => getJSON('/api/records')),
    settle(() => getJSON('/api/duplicates')),
    settle(() => getJSON('/api/sync')),
    settle(() => getJSON('/api/logs')),
  ]);

  if (meta) $('dataDir').textContent = meta.dataDir;
  if (records) {
    state.records = records.records || [];
    state.files = records.files || [];
    state.issues = records.issues || [];
  }
  if (duplicates) {
    state.duplicates = duplicates;
    state.files = state.files.concat(duplicates.files || []);
    state.issues = state.issues.concat(duplicates.issues || []);
  }
  state.sync = sync;
  state.logs = logs ? logs.logs || [] : [];

  const banner = $('loadError');
  if (errors.length) {
    banner.hidden = false;
    banner.innerHTML = '';
    banner.append(el('span', 'banner-icon', '✕'));
    const body = el('div', 'banner-body');
    body.append(el('strong', null, 'Some data could not be read. '), document.createTextNode(errors.join(' · ')));
    banner.append(body);
  } else {
    banner.hidden = true;
  }

  populateFilterOptions();
  render();
}

/* ---- filtering ---------------------------------------------------------- */

function visibleRecords() {
  const f = state.filters;
  const q = f.q.trim().toLowerCase();

  return state.records.filter((r) => {
    if (f.from && r.day && r.day < f.from) return false;
    if (f.to && r.day && r.day > f.to) return false;
    if (f.account && r.accountName !== f.account) return false;
    if (f.category && r.categoryName !== f.category) return false;
    if (f.label && !(r.labels || []).some((l) => l.name === f.label)) return false;
    if (f.type === 'expense' && r.amount >= 0) return false;
    if (f.type === 'income' && r.amount < 0) return false;
    if (q) {
      const haystack = [r.counterParty, r.note, r.accountName, r.categoryName, r.source, r.recordState]
        .concat((r.labels || []).map((l) => l.name))
        .join(' ')
        .toLowerCase();
      if (!haystack.includes(q)) return false;
    }
    return true;
  });
}

function sortRecords(records) {
  const { key, dir } = state.sort;
  return records.slice().sort((a, b) => {
    const av = a[key], bv = b[key];
    if (typeof av === 'number' && typeof bv === 'number') return (av - bv) * dir;
    return String(av ?? '').localeCompare(String(bv ?? '')) * dir;
  });
}

function populateFilterOptions() {
  const fill = (id, values, allLabel) => {
    const select = $(id);
    const current = select.value;
    select.innerHTML = '';
    select.append(new Option(allLabel, ''));
    values.forEach((v) => select.append(new Option(v, v)));
    if (values.includes(current)) select.value = current;
  };
  const uniq = (arr) => [...new Set(arr.filter(Boolean))].sort((a, b) => a.localeCompare(b));

  fill('accountInput', uniq(state.records.map((r) => r.accountName)), 'All accounts');
  fill('categoryInput', uniq(state.records.map((r) => r.categoryName)), 'All categories');
  fill('labelInput', uniq(state.records.flatMap((r) => (r.labels || []).map((l) => l.name))), 'All labels');
}

/* ---- aggregates --------------------------------------------------------- */

function summarise(records) {
  const out = {
    count: records.length,
    inflow: 0,
    outflow: 0,
    currency: currencyOf(records),
    accounts: new Set(),
    categories: new Set(),
    labels: new Set(),
    unknown: 0,
    transfers: 0,
    days: new Map(),
    byCategory: new Map(),
    byMerchant: new Map(),
    first: null,
    last: null,
  };

  for (const r of records) {
    if (r.amount >= 0) out.inflow += r.amount; else out.outflow += -r.amount;
    if (r.accountName) out.accounts.add(r.accountName);
    if (r.categoryName) out.categories.add(r.categoryName);
    (r.labels || []).forEach((l) => out.labels.add(l.name));
    if (isUnknownCategory(r)) out.unknown += 1;
    if (r.isTransfer) out.transfers += 1;

    if (r.day) {
      const bucket = out.days.get(r.day) || { day: r.day, in: 0, out: 0 };
      if (r.amount >= 0) bucket.in += r.amount; else bucket.out += -r.amount;
      out.days.set(r.day, bucket);
      if (!out.first || r.day < out.first) out.first = r.day;
      if (!out.last || r.day > out.last) out.last = r.day;
    }

    if (r.amount < 0) {
      const cat = r.categoryName || 'Uncategorized';
      const c = out.byCategory.get(cat) || { name: cat, total: 0, count: 0, color: r.categoryColor };
      c.total += -r.amount; c.count += 1;
      out.byCategory.set(cat, c);

      const merchant = r.counterParty || '(no merchant)';
      const m = out.byMerchant.get(merchant) || { name: merchant, total: 0, count: 0 };
      m.total += -r.amount; m.count += 1;
      out.byMerchant.set(merchant, m);
    }
  }

  out.net = out.inflow - out.outflow;
  return out;
}

const isUnknownCategory = (r) => {
  const name = (r.categoryName || '').toLowerCase();
  const group = (r.categoryGroup || '').toLowerCase();
  return !r.categoryName || name.startsWith('unknown') || name === 'uncategorized' || group === 'unknown';
};

/* ---- render: overview --------------------------------------------------- */

function renderTiles(sum) {
  const box = $('tiles');
  box.innerHTML = '';

  const tile = (label, value, note, cls) => {
    const t = el('div', 'tile');
    t.append(el('div', 'tile-label', label));
    t.append(el('div', `tile-value${cls ? ' ' + cls : ''}`, value));
    if (note) t.append(el('div', 'tile-note', note));
    box.append(t);
  };

  const span = sum.first && sum.last
    ? (sum.first === sum.last ? dayLabel(sum.first) : `${dayLabel(sum.first)} → ${dayLabel(sum.last)}`)
    : 'no dated records';

  tile('Records', String(sum.count), span);
  tile('Money out', money(sum.outflow, sum.currency), `${sum.byMerchant.size} merchants`, 'value-out');
  tile('Money in', money(sum.inflow, sum.currency), sum.inflow ? '' : 'none in range', 'value-in');
  tile('Net', money(sum.net, sum.currency), sum.net < 0 ? 'spent more than received' : 'received more than spent',
    sum.net < 0 ? 'value-out' : 'value-in');
  tile('Accounts', String(sum.accounts.size), `${sum.categories.size} categories · ${sum.labels.size} labels`);
}

function renderAttention(sum) {
  const box = $('attention');
  box.innerHTML = '';

  const banner = (kind, icon, strong, rest) => {
    const b = el('div', `banner banner-${kind}`);
    b.append(el('span', 'banner-icon', icon));
    const body = el('div', 'banner-body');
    body.append(el('strong', null, strong));
    if (rest) body.append(document.createTextNode(' ' + rest));
    b.append(body);
    box.append(b);
  };

  const failed = state.logs.find((l) => l.status === 'error');
  const newest = state.logs[0];
  if (newest && newest.status === 'error') {
    banner('critical', '✕', 'Latest sync run failed.', `${stamp(newest.modified)} — ${newest.headline || newest.name}`);
  } else if (failed) {
    banner('warning', '!', `${state.logs.filter((l) => l.status === 'error').length} of the last ${state.logs.length} runs failed.`,
      `Most recent failure: ${stamp(failed.modified)}.`);
  } else if (newest) {
    banner('good', '✓', 'Latest sync run completed.', stamp(newest.modified));
  }

  const pending = state.duplicates.groups.filter((g) => !g.decision).length;
  const decided = state.duplicates.groups.length - pending;
  if (state.duplicates.groups.length) {
    banner(pending ? 'warning' : 'neutral', pending ? '!' : '·',
      `${state.duplicates.groups.length} duplicate group${state.duplicates.groups.length === 1 ? '' : 's'} found.`,
      pending ? `${pending} still undecided, ${decided} resolved.` : 'All resolved.');
  }

  if (sum.unknown) {
    banner('warning', '!', `${sum.unknown} record${sum.unknown === 1 ? '' : 's'} without a real category.`,
      'These land in Wallet as "Unknown expense" or "Uncategorized".');
  }
}

/* Horizontal magnitude bars — one hue, ranked, direct value labels. */
function renderCategoryChart(sum) {
  const box = $('categoryChart');
  box.innerHTML = '';

  const rows = [...sum.byCategory.values()].sort((a, b) => b.total - a.total);
  if (!rows.length) {
    box.append(el('p', 'empty', 'No money-out records in this range.'));
    return;
  }

  const top = rows.slice(0, 8);
  const rest = rows.slice(8);
  if (rest.length) {
    top.push({ name: `Other (${rest.length})`, total: rest.reduce((a, r) => a + r.total, 0), count: rest.reduce((a, r) => a + r.count, 0) });
  }
  const max = Math.max(...top.map((r) => r.total));

  top.forEach((row) => {
    const line = el('div', 'hbar-row');
    line.append(el('div', 'hbar-name', row.name));

    const track = el('div', 'hbar-track');
    const fill = el('div', 'hbar-fill');
    fill.style.width = `${Math.max((row.total / max) * 100, 1)}%`;
    track.append(fill);
    line.append(track);

    line.append(el('div', 'hbar-value', money(row.total, sum.currency, { compact: row.total >= 100000 })));
    attachTooltip(line, () => ({
      title: row.name,
      rows: [`${money(row.total, sum.currency)} out`, `${row.count} record${row.count === 1 ? '' : 's'}`,
        `${((row.total / sum.outflow) * 100).toFixed(1)}% of money out`],
    }));
    box.append(line);
  });
}

/* Diverging bars around a zero baseline: in above, out below. Position and the
   legend carry the distinction as well as hue. */
function renderFlowChart(sum) {
  const box = $('flowChart');
  box.innerHTML = '';

  const days = [...sum.days.values()].sort((a, b) => a.day.localeCompare(b.day));
  if (!days.length) {
    box.append(el('p', 'empty', 'No dated records in this range.'));
    return;
  }

  const NS = 'http://www.w3.org/2000/svg';
  // The viewBox is sized to the container's real pixel width so the SVG scales
  // 1:1 — a stretched viewBox would smear the tick labels horizontally.
  const W = Math.max(360, Math.round(box.clientWidth) || 640);
  const H = 220, padL = 56, padR = 12, padT = 12, padB = 26;
  const innerW = W - padL - padR, innerH = H - padT - padB;

  const maxIn = Math.max(0, ...days.map((d) => d.in));
  const maxOut = Math.max(0, ...days.map((d) => d.out));
  const scaleMax = Math.max(maxIn, maxOut) || 1;
  // Zero sits proportionally, so a range with no income still uses the full height.
  const zeroY = padT + (maxIn / (maxIn + maxOut || 1)) * innerH;
  const upH = zeroY - padT, downH = padT + innerH - zeroY;

  const svg = document.createElementNS(NS, 'svg');
  svg.setAttribute('class', 'flow-svg');
  svg.setAttribute('viewBox', `0 0 ${W} ${H}`);
  svg.setAttribute('height', String(H));
  svg.setAttribute('role', 'img');
  svg.setAttribute('aria-label', `Daily money in and out across ${days.length} day${days.length === 1 ? '' : 's'}`);

  const add = (name, attrs, cls) => {
    const node = document.createElementNS(NS, name);
    for (const [k, v] of Object.entries(attrs)) node.setAttribute(k, v);
    if (cls) node.setAttribute('class', cls);
    svg.append(node);
    return node;
  };

  // Recessive reference lines at the two scale extremes.
  if (maxIn > 0) add('line', { x1: padL, x2: W - padR, y1: padT, y2: padT }, 'grid-line');
  if (maxOut > 0) add('line', { x1: padL, x2: W - padR, y1: padT + innerH, y2: padT + innerH }, 'grid-line');
  add('line', { x1: padL, x2: W - padR, y1: zeroY, y2: zeroY }, 'zero-line');

  const tick = (y, text, anchor = 'end', dy = 3) => {
    const t = add('text', { x: padL - 8, y: y + dy, 'text-anchor': anchor }, 'tick');
    t.textContent = text;
  };
  if (maxIn > 0) tick(padT, money(maxIn, sum.currency, { compact: true }));
  tick(zeroY, '0');
  if (maxOut > 0) tick(padT + innerH, money(maxOut, sum.currency, { compact: true }));

  const slot = innerW / days.length;
  const barW = Math.max(4, Math.min(48, slot - 8)); // 8px keeps a clear gap between adjacent days

  days.forEach((d, i) => {
    const cx = padL + slot * i + slot / 2;
    const x = cx - barW / 2;

    if (d.in > 0 && upH > 0) {
      const h = Math.max(2, (d.in / scaleMax) * upH);
      const bar = add('rect', { x, y: zeroY - h, width: barW, height: h, rx: 4 }, 'bar bar-in');
      attachTooltip(bar, () => ({ title: dayLabel(d.day), rows: [`${money(d.in, sum.currency)} in`] }));
    }
    if (d.out > 0 && downH > 0) {
      const h = Math.max(2, (d.out / scaleMax) * downH);
      const bar = add('rect', { x, y: zeroY, width: barW, height: h, rx: 4 }, 'bar bar-out');
      attachTooltip(bar, () => ({ title: dayLabel(d.day), rows: [`${money(d.out, sum.currency)} out`] }));
    }

    // Label the ends only, so a long range does not turn into a wall of dates.
    if (days.length <= 10 || i === 0 || i === days.length - 1) {
      const t = add('text', { x: cx, y: H - 8, 'text-anchor': 'middle' }, 'tick');
      t.textContent = d.day.slice(5);
    }
  });

  box.append(svg);
}

function renderMerchants(sum) {
  const box = $('merchantTable');
  box.innerHTML = '';

  const rows = [...sum.byMerchant.values()].sort((a, b) => b.total - a.total).slice(0, 10);
  if (!rows.length) {
    box.append(el('p', 'empty', 'No money-out records in this range.'));
    return;
  }
  box.append(miniTable(
    ['Merchant', 'Records', 'Total out', 'Average'],
    rows.map((m) => [m.name, String(m.count), money(m.total, sum.currency), money(m.total / m.count, sum.currency)]),
    [false, true, true, true],
  ));
}

function renderFiles() {
  const box = $('filesTable');
  box.innerHTML = '';
  if (!state.files.length) {
    box.append(el('p', 'empty', 'No record, dedup or decision files found in the data directory.'));
  } else {
    box.append(miniTable(
      ['File', 'Kind', 'Rows', 'Size', 'Modified'],
      state.files.map((f) => [f.name, f.kind, f.records ? String(f.records) : '—', bytes(f.size), stamp(f.modified)]),
      [false, false, true, true, false],
    ));
  }

  const issues = $('issuesBox');
  issues.innerHTML = '';
  if (state.issues.length) {
    const b = el('div', 'banner banner-warning');
    b.style.marginTop = '12px';
    b.append(el('span', 'banner-icon', '!'));
    const body = el('div', 'banner-body');
    body.append(el('strong', null, `${state.issues.length} line${state.issues.length === 1 ? '' : 's'} could not be parsed.`));
    state.issues.slice(0, 5).forEach((iss) => {
      const p = el('p', 'sub');
      p.textContent = `${iss.file}${iss.line ? `:${iss.line}` : ''} — ${iss.message}`;
      body.append(p);
    });
    b.append(body);
    issues.append(b);
  }
}

function miniTable(headers, rows, numeric) {
  const table = el('table', 'mini-table');
  const thead = el('thead');
  const hr = el('tr');
  headers.forEach((h, i) => {
    const th = el('th', numeric[i] ? 'num' : null, h);
    hr.append(th);
  });
  thead.append(hr);
  table.append(thead);

  const tbody = el('tbody');
  rows.forEach((cells) => {
    const tr = el('tr');
    cells.forEach((c, i) => tr.append(el('td', numeric[i] ? 'num' : null, c)));
    tbody.append(tr);
  });
  table.append(tbody);
  return table;
}

/* ---- render: records ---------------------------------------------------- */

function renderRecords(records) {
  const body = $('recordsBody');
  body.innerHTML = '';

  if (!records.length) {
    const tr = el('tr');
    const td = el('td', 'empty', state.records.length ? 'No records match these filters.' : 'No records found in the data directory.');
    td.colSpan = 7;
    tr.append(td);
    body.append(tr);
    return;
  }

  sortRecords(records).forEach((r) => {
    const tr = el('tr');
    tr.tabIndex = 0;

    tr.append(el('td', null, dayLabel(r.recordDate)));

    const merchant = el('td', 'merchant', r.counterParty || '—');
    if (r.note) merchant.title = r.note;
    tr.append(merchant);

    tr.append(el('td', `num ${r.amount < 0 ? 'amount-out' : 'amount-in'}`, money(r.amount, r.currency)));

    const cat = el('td');
    if (r.categoryName) {
      const chip = el('span', 'chip');
      if (r.categoryColor) {
        const dot = el('span', 'dot');
        dot.style.background = r.categoryColor;
        chip.append(dot);
      }
      chip.append(document.createTextNode(r.categoryName));
      cat.append(chip);
    } else {
      cat.textContent = '—';
    }
    tr.append(cat);

    tr.append(el('td', null, r.accountName || '—'));

    const labels = el('td');
    const chips = el('div', 'chips');
    (r.labels || []).forEach((l) => {
      const chip = el('span', 'chip');
      if (l.color) {
        const dot = el('span', 'dot');
        dot.style.background = l.color;
        chip.append(dot);
      }
      chip.append(document.createTextNode(l.name));
      chips.append(chip);
    });
    labels.append((r.labels || []).length ? chips : document.createTextNode('—'));
    tr.append(labels);

    tr.append(el('td', null, r.source || '—'));

    const open = () => openDrawer(r);
    tr.addEventListener('click', open);
    tr.addEventListener('keydown', (e) => { if (e.key === 'Enter') open(); });
    body.append(tr);
  });
}

function openDrawer(r) {
  $('drawerTitle').textContent = r.counterParty || 'Record';
  const body = $('drawerBody');
  body.innerHTML = '';

  const amount = el('div', `drawer-amount ${r.amount < 0 ? 'value-out' : 'value-in'}`, money(r.amount, r.currency));
  body.append(amount);
  body.append(el('p', 'sub', `${titleCase(r.recordType) || 'Record'} · ${dayLabel(r.recordDate)}`));

  const dl = el('dl', 'kv');
  const row = (key, value, mono) => {
    if (value === undefined || value === null || value === '') return;
    dl.append(el('dt', null, key));
    dl.append(el('dd', mono ? 'mono' : null, value));
  };

  row('Account', r.accountName || r.accountId);
  row('Category', [r.categoryName, r.categoryGroup && `(${r.categoryGroup})`].filter(Boolean).join(' '));
  row('State', titleCase(r.recordState));
  row('Source', r.source);
  row('Note', r.note);
  if (r.isTransfer) row('Transfer', r.transferId || 'yes', true);
  row('Record date', stamp(r.recordDate));
  row('Created', stamp(r.createdAt));
  row('Updated', stamp(r.updatedAt));
  row('Record id', r.id, true);
  row('Account id', r.accountId, true);
  row('Read from', r.sourceFile, true);
  body.append(dl);

  if ((r.labels || []).length) {
    body.append(el('h3', 'sub', 'Labels'));
    const chips = el('div', 'chips');
    chips.style.marginTop = '6px';
    r.labels.forEach((l) => {
      const chip = el('span', 'chip');
      if (l.color) {
        const dot = el('span', 'dot');
        dot.style.background = l.color;
        chip.append(dot);
      }
      chip.append(document.createTextNode(l.name + (l.archived ? ' (archived)' : '')));
      chips.append(chip);
    });
    body.append(chips);
  }

  if (r.raw) {
    const details = el('details');
    details.append(el('summary', null, 'Stored JSON'));
    const pre = el('pre');
    pre.textContent = JSON.stringify(r.raw, null, 2);
    details.append(pre);
    body.append(details);
  }

  $('drawer').hidden = false;
  $('drawerScrim').hidden = false;
  $('drawerClose').focus();
}

function closeDrawer() {
  $('drawer').hidden = true;
  $('drawerScrim').hidden = true;
}

/* ---- render: duplicates ------------------------------------------------- */

function renderDuplicates() {
  const summary = $('dupSummary');
  const box = $('dupGroups');
  summary.innerHTML = '';
  box.innerHTML = '';

  const groups = state.duplicates.groups || [];
  if (!groups.length) {
    const card = el('div', 'card');
    card.append(el('p', 'empty', 'No dedup scan found in the data directory. Run `wallet dedup scan` to produce one.'));
    box.append(card);
    return;
  }

  const pending = groups.filter((g) => !g.decision).length;
  const toDelete = groups.reduce((n, g) => n + (g.decision ? (g.decision.deleteRecordIds || []).length : 0), 0);

  const card = el('div', 'card');
  const tiles = el('div', 'tiles');
  const tile = (label, value, note) => {
    const t = el('div', 'tile');
    t.append(el('div', 'tile-label', label));
    t.append(el('div', 'tile-value', value));
    if (note) t.append(el('div', 'tile-note', note));
    tiles.append(t);
  };
  tile('Duplicate groups', String(groups.length), `${groups.reduce((n, g) => n + g.records.length, 0)} records involved`);
  tile('Decided', String(groups.length - pending), pending ? `${pending} awaiting a decision` : 'all resolved');
  tile('Marked for deletion', String(toDelete), toDelete ? 'run `wallet dedup execute` to apply' : '');
  card.append(tiles);
  summary.append(card);

  groups.forEach((g) => {
    const card = el('div', 'card dup-group');
    const head = el('div', 'dup-head');
    head.append(el('span', 'dup-key', g.duplicateKey));
    head.append(el('span', 'chip', `${g.matchType || 'match'} · ${Math.round((g.confidence ?? 0) * 100)}% confidence`));
    head.append(el('span', 'chip', g.decision ? 'decided' : 'undecided'));
    card.append(head);

    const keep = new Set(g.decision ? g.decision.keepRecordIds || [] : []);
    const drop = new Set(g.decision ? g.decision.deleteRecordIds || [] : []);

    const table = el('table', 'mini-table dup-table');
    const thead = el('thead');
    const hr = el('tr');
    ['', 'Record id', 'Merchant', 'Amount', 'Category', 'Created'].forEach((h, i) => hr.append(el('th', i === 3 ? 'num' : null, h)));
    thead.append(hr);
    table.append(thead);

    const tbody = el('tbody');
    g.records.forEach((r) => {
      const tr = el('tr', keep.has(r.id) ? 'tag-keep' : drop.has(r.id) ? 'tag-delete' : null);
      const verdict = keep.has(r.id) ? 'keep' : drop.has(r.id) ? 'delete' : r.isOriginal ? 'original' : '';
      tr.append(el('td', null, verdict));
      const id = el('td', null, r.id);
      id.style.fontFamily = 'var(--mono)';
      tr.append(id);
      tr.append(el('td', null, r.counterParty || '—'));
      tr.append(el('td', 'num', money(r.amount, currencyOf(state.records))));
      tr.append(el('td', null, r.category || '—'));
      tr.append(el('td', null, stamp(r.createdAt)));
      tbody.append(tr);
    });
    table.append(tbody);
    card.append(table);

    if (g.decision) {
      card.append(el('p', 'dup-note', `${g.decision.action} — ${g.decision.reason || 'no reason recorded'} (${g.decision.sourceFile})`));
    }
    card.append(el('p', 'dup-note', `Scanned from ${g.sourceFile}${g.scannedAt ? ` on ${stamp(g.scannedAt)}` : ''}`));
    box.append(card);
  });
}

/* ---- render: sync & logs ------------------------------------------------ */

function renderSync() {
  const box = $('syncState');
  const meta = $('syncFileMeta');
  box.innerHTML = '';

  if (!state.sync || state.sync.missing) {
    meta.textContent = '';
    box.append(el('p', 'empty', 'last-sync.json not found — the pack has not recorded a run yet.'));
    return;
  }
  meta.textContent = `${state.sync.file} · updated ${stamp(state.sync.modified)}`;

  const s = state.sync.state || {};

  // A failed or still-running state is the headline, not a table row.
  if (s.last_run_status && s.last_run_status !== 'success') {
    const kind = s.last_run_status === 'failed' ? 'critical' : 'warning';
    const b = el('div', `banner banner-${kind}`);
    b.style.marginBottom = '12px';
    b.append(el('span', 'banner-icon', s.last_run_status === 'failed' ? '✕' : '…'));
    const body = el('div', 'banner-body');
    body.append(el('strong', null, s.last_run_status === 'failed'
      ? 'The last run failed.' : 'A run is in progress (or was interrupted).'));
    if (s.last_run_error) body.append(el('p', 'sub', s.last_run_error));
    b.append(body);
    box.append(b);
  }

  const dl = el('dl', 'kv');
  const row = (k, v, mono) => {
    dl.append(el('dt', null, k));
    dl.append(el('dd', mono ? 'mono' : null, v));
  };

  row('Last run status', titleCase(s.last_run_status) || '—');
  row('Last email timestamp', s.last_email_timestamp ? stamp(s.last_email_timestamp) : '—');
  row('Last run start', s.last_run_start ? stamp(s.last_run_start) : 'never recorded');
  row('Last run end', s.last_run_end ? stamp(s.last_run_end) : 'never recorded');

  const sm = s.last_run_summary || {};
  const totals = [`fetched ${sm.total_fetched ?? 0}`, `created ${sm.created ?? 0}`,
    `skipped ${sm.skipped ?? 0}`, `failed ${sm.failed ?? 0}`];
  if (sm.unrouted !== undefined) totals.push(`unrouted ${sm.unrouted}`);
  row('Last run totals', totals.join(' · '));
  if (sm.failed) {
    row('Documents needing a format', `${sm.failed} email${sm.failed === 1 ? '' : 's'} matched no format, or matched one whose pattern did not extract`);
  }
  if (sm.unrouted) {
    row('Documents needing a route', `${sm.unrouted} extracted but had no account to post to`);
  }
  row('Drive files processed', String((s.processed_drive_files || []).length));
  row('Auto-created accounts', (s.auto_created_accounts || []).join(', ') || 'none');
  box.append(dl);

  const details = el('details');
  details.append(el('summary', null, 'Raw last-sync.json'));
  const pre = el('pre');
  pre.textContent = JSON.stringify(s, null, 2);
  pre.style.cssText = 'background:var(--surface-sunk);border-radius:6px;padding:12px;overflow-x:auto;font-family:var(--mono);font-size:11px;margin-top:8px';
  details.append(pre);
  box.append(details);
}

function renderLogs() {
  const list = $('logList');
  list.innerHTML = '';
  const failures = state.logs.filter((l) => l.status === 'error').length;
  $('logCount').textContent = state.logs.length
    ? `${state.logs.length} runs · ${failures} failed`
    : 'no logs';

  if (!state.logs.length) {
    const li = el('li', 'log-item');
    li.append(el('p', 'empty', 'No run logs in the data directory.'));
    list.append(li);
    return;
  }

  state.logs.forEach((log) => {
    const li = el('li', 'log-item');
    li.setAttribute('aria-current', String(state.selectedLog === log.name));
    li.append(el('span', `status-dot status-${log.status}`));

    const main = el('div');
    main.append(el('div', 'log-name', log.name));
    main.append(el('div', 'log-meta', `${stamp(log.modified)} · ${bytes(log.size)}${log.status === 'error' ? ' · failed' : ''}`));
    if (log.headline) main.append(el('div', 'log-meta', log.headline));
    li.append(main);

    li.addEventListener('click', () => selectLog(log.name));
    list.append(li);
  });

  if (!state.selectedLog) selectLog(state.logs[0].name);
}

async function selectLog(name) {
  state.selectedLog = name;
  $('logTitle').textContent = name;
  $('logBody').textContent = 'Loading…';
  [...$('logList').children].forEach((li) => {
    const label = li.querySelector('.log-name');
    li.setAttribute('aria-current', String(!!label && label.textContent === name));
  });

  try {
    const res = await getJSON(`/api/logs/${encodeURIComponent(name)}?tail=500`);
    const pre = $('logBody');
    pre.innerHTML = '';
    res.body.split('\n').forEach((line) => {
      const isErr = /Traceback|Error|ERROR|❌|FAILED/.test(line);
      pre.append(el('span', isErr ? 'err' : null, line + '\n'));
    });
  } catch (err) {
    $('logBody').textContent = `Could not read ${name}: ${err.message}`;
  }
}

/* ---- tooltip ------------------------------------------------------------ */

function attachTooltip(node, build) {
  const tip = $('tooltip');
  const show = (e) => {
    const { title, rows } = build();
    tip.innerHTML = '';
    tip.append(el('div', 'tt-title', title));
    rows.forEach((r) => tip.append(el('div', 'tt-row', r)));
    tip.hidden = false;
    move(e);
  };
  const move = (e) => {
    const pad = 14;
    const rect = tip.getBoundingClientRect();
    let x = e.clientX + pad, y = e.clientY + pad;
    if (x + rect.width > window.innerWidth - 8) x = e.clientX - rect.width - pad;
    if (y + rect.height > window.innerHeight - 8) y = e.clientY - rect.height - pad;
    tip.style.left = `${Math.max(8, x)}px`;
    tip.style.top = `${Math.max(8, y)}px`;
  };
  node.addEventListener('mouseenter', show);
  node.addEventListener('mousemove', move);
  node.addEventListener('mouseleave', () => { tip.hidden = true; });
}

/* ---- render orchestration ---------------------------------------------- */

function render() {
  const records = visibleRecords();
  const sum = summarise(records);

  $('tabCountRecords').textContent = state.records.length ? `(${records.length})` : '';
  $('tabCountDuplicates').textContent = state.duplicates.groups.length ? `(${state.duplicates.groups.length})` : '';
  $('filterSummary').textContent = records.length === state.records.length
    ? `${state.records.length} records`
    : `${records.length} of ${state.records.length} records`;

  renderTiles(sum);
  renderAttention(sum);
  renderCategoryChart(sum);
  renderFlowChart(sum);
  renderMerchants(sum);
  renderFiles();
  renderRecords(records);
  renderDuplicates();
  renderSync();
  renderLogs();
}

/* ---- wiring ------------------------------------------------------------- */

const TABS = ['overview', 'records', 'duplicates', 'runs'];

function showTab(name, updateHash = true) {
  if (!TABS.includes(name)) name = 'overview';
  if (updateHash && location.hash.slice(1) !== name) location.hash = name;
  document.querySelectorAll('.tab').forEach((t) => t.setAttribute('aria-selected', String(t.dataset.tab === name)));
  TABS.forEach((n) => { $(`panel-${n}`).hidden = n !== name; });
  // Filters drive the overview totals and the record list; the other two tabs
  // read different files entirely, so hiding the bar avoids implying otherwise.
  $('filters').hidden = name !== 'overview' && name !== 'records';
}

function bind() {
  document.querySelectorAll('.tab').forEach((tab) => tab.addEventListener('click', () => showTab(tab.dataset.tab)));
  window.addEventListener('hashchange', () => showTab(location.hash.slice(1), false));

  // The flow chart is drawn at the container's pixel width, so it has to be
  // redrawn when that width changes.
  let resizeTimer;
  window.addEventListener('resize', () => {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(() => renderFlowChart(summarise(visibleRecords())), 150);
  });

  const onFilter = (id, key) => $(id).addEventListener('input', (e) => {
    state.filters[key] = e.target.value;
    render();
  });
  onFilter('qInput', 'q');
  onFilter('fromInput', 'from');
  onFilter('toInput', 'to');
  onFilter('accountInput', 'account');
  onFilter('categoryInput', 'category');
  onFilter('labelInput', 'label');
  onFilter('typeInput', 'type');

  $('clearBtn').addEventListener('click', () => {
    state.filters = { q: '', from: '', to: '', account: '', category: '', label: '', type: '' };
    ['qInput', 'fromInput', 'toInput', 'accountInput', 'categoryInput', 'labelInput', 'typeInput']
      .forEach((id) => { $(id).value = ''; });
    render();
  });

  document.querySelectorAll('.sort').forEach((btn) => btn.addEventListener('click', () => {
    const key = btn.dataset.sort;
    state.sort = state.sort.key === key ? { key, dir: -state.sort.dir } : { key, dir: key === 'recordDate' ? -1 : 1 };
    document.querySelectorAll('.sort').forEach((b) => b.removeAttribute('data-active'));
    btn.dataset.active = state.sort.dir === 1 ? '↑' : '↓';
    render();
  }));

  $('refreshBtn').addEventListener('click', () => loadAll());
  $('drawerClose').addEventListener('click', closeDrawer);
  $('drawerScrim').addEventListener('click', closeDrawer);
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape') closeDrawer(); });

  $('themeBtn').addEventListener('click', () => {
    const now = document.documentElement.getAttribute('data-theme');
    const next = now === 'dark' ? 'light' : now === 'light' ? '' : 'dark';
    if (next) document.documentElement.setAttribute('data-theme', next);
    else document.documentElement.removeAttribute('data-theme');
    try { localStorage.setItem('wallet-ui-theme', next); } catch { /* private mode */ }
  });

  try {
    const saved = localStorage.getItem('wallet-ui-theme');
    if (saved) document.documentElement.setAttribute('data-theme', saved);
  } catch { /* private mode */ }
}

bind();
showTab(location.hash.slice(1) || 'overview', false);
loadAll();
