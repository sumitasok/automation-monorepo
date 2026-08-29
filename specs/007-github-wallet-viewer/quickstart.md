# Quickstart: GitHub Wallet Records Viewer

## Prerequisites

- Records fetched to `~/data/wallet/records.jsonl` (see [wallet fetch documentation](../../RUNBOOK.md#wallet-fetch))
- Records repository URL configured in `.git/config`
- Browser with cookie support (httpOnly, secure flags)
- HTTPS connection (for secure cookie persistence)

## Validation Scenarios

### Scenario 1: Load Records from GitHub

**Goal**: Verify user can authenticate with GitHub PAT and load records

**Steps**:

1. Navigate to `packs/wallet/index.html` in browser
2. You should see an empty table with search/filter controls
3. Paste a valid GitHub read-only PAT in the auth box
4. Click "Load Records"

**Expected Outcome** (within 10 seconds):

- Table populated with transaction records
- 6000+ records displayed
- PAT stored in secure cookie (httpOnly flag, visible in Chrome DevTools → Application → Cookies)
- No errors in browser console
- Page remains interactive (sortable, searchable)

**Verification**:

- Open DevTools (F12)
- Check Network tab: Request to GitHub API includes `Authorization: Bearer <PAT>` header
- Check Application → Cookies: `wallet_github_pat` exists with httpOnly flag set
- No console errors or warnings related to authentication

---

### Scenario 2: Search and Filter Records

**Goal**: Verify filtering and sorting work correctly on loaded records

**Steps**:

1. With records loaded, type "Blinkit" in the search box
2. Verify table updates immediately
3. Click "Date" column header to sort ascending
4. Click again to sort descending

**Expected Outcome** (< 500ms per action):

- Search filters to only Blinkit transactions (counterparty match)
- Sorting toggles between ascending/descending instantly
- Page remains responsive with no lag
- Results update in real-time as user types

**Verification**:

- Filter results in <500ms (measure from Network tab timing)
- No pagination lag with 6000+ records
- Sorting handles negative amounts (expenses) correctly

---

### Scenario 3: View Record Details

**Goal**: Verify drill-down detail view works

**Steps**:

1. Click on any record row in the table
2. Detail modal/panel should open
3. Verify all fields are displayed (amount, date, category, labels, notes, account)
4. Close the detail view
5. Verify search/filter state is preserved

**Expected Outcome**:

- Detail view opens instantly (no network fetch)
- All transaction fields visible
- User can close and return to table with filters intact
- Detail view is read-only (no edit buttons)

---

### Scenario 4: Session Persistence

**Goal**: Verify PAT persists across page reload

**Steps**:

1. Load records and populate table
2. Refresh the page (F5)
3. Verify records are still displayed

**Expected Outcome**:

- Records remain visible without re-entering PAT
- Page loads within 10 seconds
- No network errors
- PAT cookie is still valid

**Verification**:

- Check DevTools → Application → Storage → sessionStorage for filter state
- Verify records.jsonl is cached (if using service worker or browser cache)

---

## Integration with Wallet Pack

The UI is deployed as a static artifact at:

```
packs/wallet/index.html
```

To access the UI:

1. Use GitHub Pages serving the wallet pack, or
2. Open `file:///Users/sumitasok/Claude/Projects/automation-monorepo/packs/wallet/index.html` locally
3. Ensure `.git/config` is accessible for repo URL resolution

## Troubleshooting

| Issue | Diagnosis | Solution |
|-------|-----------|----------|
| PAT not persisting | Cookie flags not set correctly | Check `secure` and `httpOnly` flags; ensure HTTPS in production |
| Slow filtering on 6000+ records | No virtual scrolling/pagination | Implement lazy loading or virtual scrolling for performance |
| Records not loading | GitHub API 404 or auth failure | Verify PAT is valid and has repo access; check records.jsonl exists at repo path |
| All records grouped as duplicates | Type assertion issue in dedup (fixed in past) | Refer to [wallet dedup documentation](../../RUNBOOK.md#wallet-dedup) |
