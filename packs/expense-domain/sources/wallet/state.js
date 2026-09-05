class AppState {
  constructor() {
    this.records = [];
    this.filteredRecords = [];
    this.isAuthenticated = false;
    this.isLoading = false;
    this.error = null;
    
    // Filter state
    this.searchQuery = '';
    this.dateRange = [null, null];
    this.amountRange = [null, null];
    
    // Sort state
    this.sortColumn = null;
    this.sortDirection = 'asc';
    
    // UI state
    this.selectedRecordId = null;
    
    // Listeners
    this.listeners = [];
  }

  // Subscribe to state changes
  subscribe(callback) {
    this.listeners.push(callback);
  }

  // Notify all listeners
  notify() {
    this.listeners.forEach(cb => cb(this));
  }

  // Load records
  setRecords(records) {
    this.records = records || [];
    this.applyFilters();
    this.notify();
  }

  // Update auth status
  setAuthenticated(value) {
    this.isAuthenticated = value;
    this.notify();
  }

  // Update loading state
  setLoading(value) {
    this.isLoading = value;
    this.notify();
  }

  // Set error message
  setError(error) {
    this.error = error;
    this.notify();
  }

  // Update filters and re-apply
  setSearchQuery(query) {
    this.searchQuery = query;
    this.applyFilters();
    this.notify();
  }

  setDateRange(start, end) {
    this.dateRange = [start, end];
    this.applyFilters();
    this.notify();
  }

  setAmountRange(min, max) {
    this.amountRange = [min, max];
    this.applyFilters();
    this.notify();
  }

  // Set sort column and direction
  setSort(column) {
    if (this.sortColumn === column) {
      this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      this.sortColumn = column;
      this.sortDirection = 'asc';
    }
    this.applyFilters();
    this.notify();
  }

  // Clear all filters
  clearFilters() {
    this.searchQuery = '';
    this.dateRange = [null, null];
    this.amountRange = [null, null];
    this.sortColumn = null;
    this.sortDirection = 'asc';
    this.applyFilters();
    this.notify();
  }

  // Select record for drill-down
  selectRecord(recordId) {
    this.selectedRecordId = recordId;
    this.notify();
  }

  // Apply all filters and sorting
  applyFilters() {
    let filtered = [...this.records];

    // Search filter
    if (this.searchQuery) {
      const q = this.searchQuery.toLowerCase();
      filtered = filtered.filter(r => 
        r.counterParty?.toLowerCase().includes(q) ||
        r.notes?.toLowerCase().includes(q)
      );
    }

    // Date range filter
    if (this.dateRange[0] || this.dateRange[1]) {
      filtered = filtered.filter(r => 
        isDateInRange(r.recordDate, this.dateRange[0], this.dateRange[1])
      );
    }

    // Amount range filter
    if (this.amountRange[0] !== null || this.amountRange[1] !== null) {
      filtered = filtered.filter(r => 
        isAmountInRange(r.amount?.value, this.amountRange[0], this.amountRange[1])
      );
    }

    // Sort
    if (this.sortColumn) {
      filtered.sort((a, b) => {
        let aVal, bVal;

        if (this.sortColumn === 'recordDate') {
          aVal = new Date(a.recordDate);
          bVal = new Date(b.recordDate);
        } else if (this.sortColumn === 'amount') {
          aVal = a.amount?.value || 0;
          bVal = b.amount?.value || 0;
        } else if (this.sortColumn === 'counterParty') {
          aVal = (a.counterParty || '').toLowerCase();
          bVal = (b.counterParty || '').toLowerCase();
        } else if (this.sortColumn === 'category') {
          aVal = (a.category?.name || '').toLowerCase();
          bVal = (b.category?.name || '').toLowerCase();
        }

        if (aVal < bVal) return this.sortDirection === 'asc' ? -1 : 1;
        if (aVal > bVal) return this.sortDirection === 'asc' ? 1 : -1;
        return 0;
      });
    }

    this.filteredRecords = filtered;
  }

  // Get record by ID
  getRecord(id) {
    return this.records.find(r => r.id === id);
  }
}

// Global state instance
const appState = new AppState();
