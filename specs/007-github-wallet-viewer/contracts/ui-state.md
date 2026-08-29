# Contract: UI State & Component Interface

## Overview

The UI manages application state including records, filter state, and authentication. This contract defines the state shape and component responsibilities.

## State Shape

### Root App State

```typescript
interface AppState {
  // Authentication
  isAuthenticated: boolean
  patCookie: string | null  // Read from httpOnly cookie
  
  // Records
  records: Transaction[]  // All fetched records
  filteredRecords: Transaction[]  // Filtered subset
  
  // UI Filters
  searchQuery: string  // Counterparty search
  dateRange: [string, string] | null  // [startISO, endISO]
  amountRange: [number, number] | null  // [min, max]
  
  // Sorting
  sortColumn: 'recordDate' | 'amount' | 'counterParty' | 'category' | null
  sortDirection: 'asc' | 'desc'
  
  // UI State
  selectedRecordId: string | null  // Drill-down selected
  isLoading: boolean  // During fetch
  error: string | null  // Error message
}
```

## Component Interfaces

### 1. AuthForm Component

**Responsibility**: Capture and store GitHub PAT

**Input Props**:
```typescript
{
  onSubmit: (pat: string) => Promise<void>
  isLoading: boolean
  error: string | null
}
```

**Output Events**:
- `onSubmit(pat)`: User submitted PAT

**Behavior**:
- Input field for PAT (masked)
- Submit button with loading indicator
- Error display area
- No PAT displayed anywhere (logs or HTML)

---

### 2. RecordsTable Component

**Responsibility**: Display records in sortable, filterable table

**Input Props**:
```typescript
{
  records: Transaction[]
  sortColumn: string | null
  sortDirection: 'asc' | 'desc'
  onSort: (column: string) => void
  onRowClick: (recordId: string) => void
  isLoading: boolean
}
```

**Output Events**:
- `onSort(column)`: User clicked column header
- `onRowClick(recordId)`: User clicked a row

**Columns Displayed**:
1. Date (recordDate)
2. Counterparty (counterParty)
3. Amount (amount.value)
4. Category (category.name)
5. Account (account.name)
6. State (recordState)

**Behavior**:
- Click column header to sort (toggle asc/desc)
- Click row to drill down into details
- Handle virtual scrolling for 6000+ records
- Display loading state during fetch

---

### 3. FilterControls Component

**Responsibility**: Provide search and filter inputs

**Input Props**:
```typescript
{
  searchQuery: string
  dateRange: [string, string] | null
  amountRange: [number, number] | null
  onSearchChange: (query: string) => void
  onDateRangeChange: (range: [string, string] | null) => void
  onAmountRangeChange: (range: [number, number] | null) => void
  resultCount: number
}
```

**Output Events**:
- `onSearchChange(query)`: Search text changed
- `onDateRangeChange([start, end])`: Date range changed
- `onAmountRangeChange([min, max])`: Amount range changed

**Filters**:
1. **Search Box**: Text search on counterParty (case-insensitive, contains)
2. **Date Range**: Start and end date inputs (ISO format)
3. **Amount Range**: Min and max amount inputs (can be negative)
4. **Clear Filters**: Button to reset all filters

**Behavior**:
- Real-time filtering as user types
- Debounce search (300ms) to avoid lag
- Display result count after filtering
- Handle edge cases (negative amounts, empty results)

---

### 4. DetailModal Component

**Responsibility**: Display full transaction details

**Input Props**:
```typescript
{
  record: Transaction | null
  isOpen: boolean
  onClose: () => void
}
```

**Output Events**:
- `onClose()`: User closed modal

**Displayed Fields**:
- All fields from Transaction (see record-schema.md)
- Formatted dates and amounts
- Labels as tags
- Notes as text block
- Read-only display (no edit buttons)

**Behavior**:
- Modal overlays table
- Close button (X) and backdrop click
- Preserve table scroll position when closed
- Handle long text fields (word wrap, scroll if needed)

---

## State Derivations

### Filtering Logic

```javascript
function getFilteredRecords(records, { searchQuery, dateRange, amountRange }) {
  return records.filter(r => {
    // Search filter
    if (searchQuery && !r.counterParty.toLowerCase().includes(searchQuery.toLowerCase())) {
      return false
    }
    
    // Date range filter
    if (dateRange) {
      const [start, end] = dateRange
      const recordDate = new Date(r.recordDate)
      if (recordDate < new Date(start) || recordDate > new Date(end)) {
        return false
      }
    }
    
    // Amount range filter
    if (amountRange) {
      const [min, max] = amountRange
      const amount = r.amount.value
      if (amount < min || amount > max) {
        return false
      }
    }
    
    return true
  })
}
```

### Sorting Logic

```javascript
function getSortedRecords(records, sortColumn, sortDirection) {
  if (!sortColumn) return records
  
  const sorted = [...records].sort((a, b) => {
    let aVal, bVal
    
    switch (sortColumn) {
      case 'recordDate':
        aVal = new Date(a.recordDate)
        bVal = new Date(b.recordDate)
        break
      case 'amount':
        aVal = a.amount.value
        bVal = b.amount.value
        break
      case 'counterParty':
        aVal = a.counterParty.toLowerCase()
        bVal = b.counterParty.toLowerCase()
        break
      // ... other columns
    }
    
    if (aVal < bVal) return sortDirection === 'asc' ? -1 : 1
    if (aVal > bVal) return sortDirection === 'asc' ? 1 : -1
    return 0
  })
  
  return sorted
}
```

## Performance Targets

| Operation | Target | Tolerance |
|-----------|--------|-----------|
| Load 6000+ records | < 10s | Measure from PAT submission to table display |
| Filter/search | < 500ms | Per keystroke, debounced |
| Sort | < 200ms | Per column click |
| Drill-down detail | Instant | No network fetch |
| Page refresh | < 10s | PAT restored from cookie |

## Accessibility

- Table rows keyboard navigable (Tab, Enter for detail)
- Search input has aria-label and autocomplete
- Buttons have proper focus states
- Error messages announced via aria-live
- Modal dialog properly scoped
