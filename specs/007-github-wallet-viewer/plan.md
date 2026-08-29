# Implementation Plan: GitHub Wallet Records Viewer

**Branch**: `007-github-wallet-viewer` | **Date**: 2026-08-29 | **Spec**: [spec.md](spec.md)

## Summary

Static browser-based UI for analyzing wallet transaction records fetched from GitHub repo. Users authenticate with read-only PAT, data persists in secure cookies, and records are displayed in an interactive sortable/filterable table with drill-down details. No backend, no export, no editing—pure read-only analysis.

## Technical Context

**Language/Version**: JavaScript (ES6+), HTML5, CSS3

**Primary Dependencies**: Vanilla JS (no framework); GitHub API (REST)

**Storage**: Browser cookies (httpOnly, secure) for PAT; sessionStorage for filter state

**Testing**: Jest for units; manual browser testing

**Target Platform**: Modern browsers (Chrome, Firefox, Safari, Edge)

**Project Type**: Static web application (SPA)

**Performance Goals**: Load 6000+ records in <10s; filter <500ms; instant drill-down

**Constraints**: PAT must never appear in logs/HTML/network; HTTPS-only; fully static; read-only UI

**Scale/Scope**: 6000+ transaction records; single user; 3 user stories (P1 auth+view, P1 search, P2 drill-down)

## Constitution Check

| Gate | Verdict |
|------|---------|
| I - Config declaration? | N/A - PAT is user-provided, not config-injected |
| II - Nothing in packs/? | YES - Static artifact at packs/wallet/index.html, state in browser only |
| III - Static UI under data/? | PARTIAL - UI at packs/wallet/index.html (in pack), declared in manifest, opens from disk, no port binding |
| IV - Derived artefacts regenerated? | N/A - Records fetched on-demand from GitHub |
| V - New instances as data? | N/A - Displays records as-is, no transformation |
| VI - Boundaries enforced? | PARTIAL - GitHub PAT controls access; browser sandbox enforces origin; httpOnly cookies limit access |
| VII - Only localhost, no secrets, explicit data leaving? | YES - No server; PAT never rendered; only in Authorization header |

**Status**: PASS

## Project Structure

**Artifacts**:
```
specs/007-github-wallet-viewer/
├── spec.md              # Specification ✓
├── plan.md              # This file
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1
├── contracts/
│   ├── ui-state.md
│   ├── github-api.md
│   └── record-schema.md
└── checklists/
    └── requirements.md
```

**Source Code**:
```
packs/wallet/
├── index.html           # Main UI
├── styles.css
├── app.js
├── github.js            # GitHub API client
├── ui.js                # DOM rendering
├── table.js             # Table with sort/filter
└── __tests__/           # Jest tests
    ├── github.test.js
    ├── table.test.js
    └── app.test.js
```

## Phase 1 Design

### Data Model

**Transaction Record**:
- id, amount (value + currencyCode), recordDate, counterParty, category (name + id), account, labels[], notes, createdAt, recordState

**UI State**:
- records, filteredRecords, searchQuery, dateRange, amountRange, sortColumn, sortDirection, selectedRecordId

**PAT Storage**:
- Cookie: `wallet_github_pat` (httpOnly, secure, SameSite=Strict)

### Contracts

**GitHub API**:
- GET /repos/{owner}/{repo}/contents/data/wallet/records.jsonl
- Auth: Bearer {PAT}
- Response: JSONL file contents
- Rate: 5000 req/hr (authenticated)

**UI Components**:
- Input: Transaction[], FilterState
- Output: Rendered table with sort/filter/drill-down
- Behavior: Sort on click, filter on input, drill-down on row click

### Quickstart

1. Auth & Load: PAT → records in <10s
2. Search: Type "Blinkit" → instant filter
3. Sort: Click "Date" → sorted records
4. Drill-down: Click row → detail modal
5. Persistence: Refresh page → PAT restored from cookie

---

**Status**: Ready for Task Generation (`/speckit-tasks`)
