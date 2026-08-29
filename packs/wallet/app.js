class WalletApp {
  constructor() {
    this.authSection = document.getElementById('authSection');
    this.viewerSection = document.getElementById('viewerSection');
    this.patForm = document.getElementById('patForm');
    this.patInput = document.getElementById('patInput');
    this.authButton = document.getElementById('authButton');
    this.authError = document.getElementById('authError');
    this.authStatus = document.getElementById('authStatus');
    this.recordsTable = document.getElementById('recordsTable');

    this.init();
  }

  async init() {
    // Restore auth if available
    const hasAuth = restoreAuth();
    
    // Try to get repo info from .git/config
    await githubAPI.getRepoInfo();

    // Subscribe to state changes
    appState.subscribe(() => this.handleStateChange());

    // Attach event listeners
    this.patForm.addEventListener('submit', (e) => this.handleAuthSubmit(e));

    // Attach sorting listeners to table headers
    document.querySelectorAll('th a.sortable').forEach(link => {
      link.addEventListener('click', (e) => {
        e.preventDefault();
        const column = e.target.dataset.column;
        appState.setSort(column);
      });
    });

    // Show viewer if authenticated
    if (hasAuth) {
      this.showViewerSection();
    } else {
      this.showAuthSection();
    }
  }

  async handleAuthSubmit(e) {
    e.preventDefault();
    
    const pat = this.patInput.value.trim();
    if (!pat) {
      this.showAuthError('Please enter a GitHub PAT');
      return;
    }

    try {
      this.authError.style.display = 'none';
      this.authButton.disabled = true;
      this.authButton.textContent = 'Loading...';

      appState.setLoading(true);
      appState.setError(null);

      // Set PAT and fetch records
      githubAPI.setPAT(pat);
      const records = await githubAPI.fetchRecords();

      // Store PAT securely
      setAuthCookie(pat);

      // Update state
      appState.setRecords(records);
      appState.setAuthenticated(true);
      appState.setLoading(false);

      // Clear form
      this.patInput.value = '';
      this.patInput.type = 'password';

      // Show viewer
      this.showViewerSection();
    } catch (error) {
      appState.setError(error.message);
      this.showAuthError(error.message);
      appState.setLoading(false);
      this.authButton.disabled = false;
      this.authButton.textContent = 'Load Records';
    }
  }

  showAuthSection() {
    this.authSection.style.display = 'flex';
    this.viewerSection.style.display = 'none';
    this.authStatus.textContent = 'Not authenticated';
  }

  showViewerSection() {
    this.authSection.style.display = 'none';
    this.viewerSection.style.display = 'flex';
    this.authStatus.textContent = `${appState.records.length} records loaded`;
  }

  showAuthError(message) {
    this.authError.textContent = message;
    this.authError.style.display = 'block';
  }

  handleStateChange() {
    // Render table with filtered/sorted records
    tableRenderer.render(appState.filteredRecords);

    // Update auth button state
    if (appState.isLoading) {
      this.authButton.disabled = true;
      this.authButton.textContent = 'Loading...';
    } else if (appState.isAuthenticated) {
      this.authButton.disabled = false;
      this.authButton.textContent = 'Load Records';
    }
  }
}

// Initialize app when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
  new WalletApp();
});
