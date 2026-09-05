class FilterControls {
  constructor() {
    this.searchInput = document.getElementById('searchInput');
    this.dateStartInput = document.getElementById('dateStartInput');
    this.dateEndInput = document.getElementById('dateEndInput');
    this.amountMinInput = document.getElementById('amountMinInput');
    this.amountMaxInput = document.getElementById('amountMaxInput');
    this.clearBtn = document.getElementById('clearFiltersBtn');

    // Debounced search handler
    this.handleSearch = debounce((query) => {
      appState.setSearchQuery(query);
    }, 300);

    this.attachListeners();
  }

  attachListeners() {
    // Search
    this.searchInput.addEventListener('input', (e) => {
      this.handleSearch(e.target.value);
    });

    // Date range
    this.dateStartInput.addEventListener('change', () => {
      appState.setDateRange(
        this.dateStartInput.value || null,
        this.dateEndInput.value || null
      );
    });

    this.dateEndInput.addEventListener('change', () => {
      appState.setDateRange(
        this.dateStartInput.value || null,
        this.dateEndInput.value || null
      );
    });

    // Amount range
    this.amountMinInput.addEventListener('change', () => {
      const min = this.amountMinInput.value ? parseFloat(this.amountMinInput.value) : null;
      const max = this.amountMaxInput.value ? parseFloat(this.amountMaxInput.value) : null;
      appState.setAmountRange(min, max);
    });

    this.amountMaxInput.addEventListener('change', () => {
      const min = this.amountMinInput.value ? parseFloat(this.amountMinInput.value) : null;
      const max = this.amountMaxInput.value ? parseFloat(this.amountMaxInput.value) : null;
      appState.setAmountRange(min, max);
    });

    // Clear filters
    this.clearBtn.addEventListener('click', () => {
      this.clearAllFilters();
    });
  }

  clearAllFilters() {
    this.searchInput.value = '';
    this.dateStartInput.value = '';
    this.dateEndInput.value = '';
    this.amountMinInput.value = '';
    this.amountMaxInput.value = '';
    appState.clearFilters();
  }
}

const filterControls = new FilterControls();
