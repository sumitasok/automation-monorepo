# Implementation Tasks: GitHub Wallet Records Viewer

**Feature**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md) | **Branch**: `feature/github-wallet-viewer`

## Overview

Browser-based static UI for analyzing wallet records from GitHub. Tasks organized by user story (P1, P1, P2) with setup and polish phases. All tasks are independent and testable.

---

## Phase 1: Setup & Project Initialization

### Goal
Initialize project structure, configure HTML boilerplate, and set up development environment.

- [ ] T001 Create base HTML structure at `packs/wallet/index.html` with title, CSS/JS links, and root div
- [ ] T002 Create `packs/wallet/styles.css` with reset, layout grid, form styling, table styling
- [ ] T003 Create `packs/wallet/app.js` with app initialization and main event listeners
- [ ] T004 Create `packs/wallet/__tests__/` directory structure for Jest test files
- [ ] T005 Create `.gitignore` entries for test artifacts, coverage, and build outputs

---

## Phase 2: Foundational Infrastructure

### Goal
Build reusable utilities and state management foundation for all user stories.

- [ ] T006 [P] Create `packs/wallet/utils.js` with helper functions: parseJSONL, formatDate, formatAmount, debounce
- [ ] T007 [P] Create `packs/wallet/state.js` with AppState class (records, filters, sorting, UI state)
- [ ] T008 Create `packs/wallet/__tests__/utils.test.js` unit tests for parseJSONL, formatters
- [ ] T009 Create `packs/wallet/__tests__/state.test.js` unit tests for state derivations (filtering, sorting)

---

## Phase 3: User Story 1 — Load Records from GitHub (P1)

### Goal
Enable users to authenticate with GitHub PAT and fetch records from the repository.

### User Story Test Criteria
- Valid PAT → records fetched and displayed within 10 seconds
- Page refresh → PAT restored from cookie, records remain visible
- Invalid PAT → error message shown, prompt for new token

#### Setup
- [ ] T010 [P] Create `packs/wallet/github.js` with GitHubAPI client class (fetchRecords method)
- [ ] T011 [P] Implement secure cookie storage: setAuthCookie(pat), getAuthCookie(), deleteAuthCookie()
- [ ] T012 Create `packs/wallet/__tests__/github.test.js` with mock GitHub API responses

#### Implementation
- [ ] T013 [US1] Create `packs/wallet/ui.js` with AuthForm component (PAT input, submit button)
- [ ] T014 [US1] Implement PAT submission flow: validate input → fetch records → display table
- [ ] T015 [US1] Implement error handling: auth failures, network errors, invalid JSON
- [ ] T016 [US1] Implement page reload recovery: restore PAT from cookie, refetch if needed
- [ ] T017 [US1] Add loading indicator and loading state management during fetch
- [ ] T018 [US1] Implement rate limit retry logic (exponential backoff for 429 responses)

#### Validation
- [ ] T019 Manual test: Fetch records with valid PAT, verify table populates in <10s
- [ ] T020 Manual test: Refresh page, verify PAT restored and records visible without re-auth
- [ ] T021 Manual test: Invalid PAT, verify error message and retry prompt
- [ ] T022 Manual test: Network failure mid-fetch, verify error handling and retry option

---

## Phase 4: User Story 2 — View and Search Records (P1)

### Goal
Display records in an interactive table with search, filter, and sort capabilities.

### User Story Test Criteria
- Records displayed in sortable table with all fields
- Search by counterparty (case-insensitive contains) filters instantly
- Sort by column header (toggle asc/desc) works for date, amount, counterparty
- Filter by date range and amount range works correctly
- Filtering 6000+ records responds in <500ms

#### Implementation
- [ ] T023 [US2] Create `packs/wallet/table.js` with RecordsTable component (render table, handle clicks)
- [ ] T024 [US2] Implement column rendering: Date, Counterparty, Amount, Category, Account, State
- [ ] T025 [US2] Implement sort toggle: click column header → sort asc/desc → toggle
- [ ] T026 [US2] Create `packs/wallet/filters.js` with FilterControls component (search, date, amount inputs)
- [ ] T027 [US2] Implement search filter: debounced, case-insensitive, contains matching on counterParty
- [ ] T028 [US2] Implement date range filter: parse ISO dates, compare with record.recordDate
- [ ] T029 [US2] Implement amount range filter: handle negative amounts (expenses), min/max comparison
- [ ] T030 [US2] [P] Implement state derivations: getFilteredRecords, getSortedRecords (see contracts/ui-state.md)
- [ ] T031 [US2] Implement debounce on search input (300ms) to optimize filtering performance
- [ ] T032 [US2] Implement virtual scrolling OR pagination for 6000+ records (handle SC-002 <500ms requirement)

#### Validation
- [ ] T033 Manual test: Load records, verify all columns displayed with correct data
- [ ] T034 Manual test: Sort by Date, Amount, Counterparty — verify ascending/descending toggle
- [ ] T035 Manual test: Search "Blinkit", verify only matching records shown, <500ms response
- [ ] T036 Manual test: Filter by date range (e.g., Aug 1-15), verify only records in range shown
- [ ] T037 Manual test: Filter by amount range (e.g., -5000 to -1000), verify expense filtering works
- [ ] T038 Manual test: Combine multiple filters, verify intersecting results
- [ ] T039 Manual test: Empty search results, verify UI shows "No records found" message

---

## Phase 5: User Story 3 — View Record Details (P2)

### Goal
Allow users to drill down into individual transaction details.

### User Story Test Criteria
- Click record row → detail modal/panel opens with all fields
- Close detail view → return to table, filters preserved
- All transaction fields visible and correctly formatted

#### Implementation
- [ ] T040 [US3] Create `packs/wallet/detail.js` with DetailModal component (render all fields, close button)
- [ ] T041 [US3] Implement detail view layout: display all Transaction fields from record-schema.md
- [ ] T042 [US3] Format amounts: show currency, handle positive/negative with +/- prefix
- [ ] T043 [US3] Format dates: convert ISO to user-friendly format (e.g., "Aug 29, 2026 3:30 PM")
- [ ] T044 [US3] Render labels as tags with color badges
- [ ] T045 [US3] Implement modal close: X button, backdrop click, ESC key
- [ ] T046 [US3] Preserve filter state when opening/closing detail view (no filter reset)

#### Validation
- [ ] T047 Manual test: Click record row, detail modal opens showing all fields
- [ ] T048 Manual test: Verify dates formatted readably, amounts show currency and sign
- [ ] T049 Manual test: Close modal (X, backdrop, or ESC), verify filters preserved
- [ ] T050 Manual test: Navigate between records (open detail, close, click another row)

---

## Phase 6: Polish & Cross-Cutting Concerns

### Goal
Optimize performance, ensure accessibility, and handle edge cases.

#### Error Handling & Edge Cases
- [ ] T051 Implement "No records found" message when all filters result in empty set
- [ ] T052 Implement handling for records with missing fields (null/undefined category, notes, etc.)
- [ ] T053 Implement graceful fallback for malformed JSONL (skip invalid lines, log warning)
- [ ] T054 Implement network timeout handling (show "Connection timeout" error, retry option)

#### Performance Optimization
- [ ] T055 Measure and optimize initial load time (target <10s for 6000+ records)
- [ ] T056 Measure and optimize filter/sort performance (target <500ms)
- [ ] T057 Implement request caching for GitHub API (honor Cache-Control headers)
- [ ] T058 Remove console.debug and sensitive logging before release (ensure PAT never logged)

#### Accessibility & UX
- [ ] T059 Add aria-labels to form inputs (search, date range, amount range)
- [ ] T060 Implement keyboard navigation: Tab through table rows, Enter to open detail
- [ ] T061 Add visual focus states to all interactive elements (table headers, filter inputs, buttons)
- [ ] T062 Implement aria-live region for search results count (announce "X results found")
- [ ] T063 Test with screen reader (e.g., NVDA on Windows, VoiceOver on Mac)

#### Documentation & Testing
- [ ] T064 Write comprehensive README at `packs/wallet/README.md` (usage, setup, troubleshooting)
- [ ] T065 Add JSDoc comments to all public functions and components
- [ ] T066 Create integration test: full user flow (auth → load → search → drill-down)

#### Deployment
- [ ] T067 Verify static HTML works when served from `packs/wallet/index.html` on GitHub Pages
- [ ] T068 Test with HTTPS requirement (secure cookie flag for production)
- [ ] T069 Verify .git/config repo URL resolution or fallback to default

---

## Dependencies & Execution Order

### Story Independence
- **US1 (Auth & Load)** → Blocking prerequisite for US2 and US3 (must fetch records first)
- **US2 (Search/Filter)** → Can run in parallel with US3 after US1 complete
- **US3 (Details)** → Can run in parallel with US2 after US1 complete

### Parallel Opportunities
- **Setup Phase** (T001-T005): All tasks [P] can run in parallel
- **Foundational Phase** (T006-T007): Utilities tasks [P] can run in parallel; tests (T008-T009) must follow
- **US2 Implementation** (T023-T031): Table and filter implementations [P] can run in parallel
- **Polish Phase** (T051-T069): All tasks can run in parallel after main implementation complete

### Critical Path
1. T001-T005 (Setup)
2. T006-T009 (Utilities & State)
3. T010-T022 (US1 - Auth/Load) ← **Blocks everything else**
4. T023-T039 (US2 - Search/Filter) **[P]** T040-T050 (US3 - Details)
5. T051-T069 (Polish)

---

## Implementation Strategy

### MVP Scope (Phase 1-4)
Complete Setup, Foundational, US1, and US2 phases to achieve MVP:
- Users can authenticate and load records
- Users can search, filter, and sort records
- Measurable performance (10s load, 500ms filter)
- This satisfies core value proposition

### V1.1 Enhancement (Phase 5)
Add drill-down details (US3) for enhanced reconciliation workflow.

### V1.2+ Future (Polish + Beyond)
Implement polish phase optimizations, accessibility improvements, and potential future enhancements (OAuth, export, etc.).

---

## Testing Strategy

**Unit Tests**: Utilities, state derivations, formatters (T008, T009, T012)

**Integration Tests**: Full user flows (T066 - auth → load → search → detail)

**Manual Tests**: Browser testing at each phase completion (T019-T022, T033-T039, T047-T050)

**Performance Tests**: Load time, filter response, record count limits

**Accessibility Tests**: Keyboard navigation, screen reader, focus states

---

## Success Metrics

✅ All tasks completed and marked [X]
✅ All user stories independently testable and passing
✅ Performance targets met (SC-001 10s load, SC-002 500ms filter)
✅ No sensitive data (PAT) exposed in logs/HTML
✅ Static HTML deployable to GitHub Pages as packs/wallet/index.html
