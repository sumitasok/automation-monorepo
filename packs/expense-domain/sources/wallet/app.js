class WalletApp {
  constructor() {
    this.authSection = document.getElementById('authSection');
    this.viewerSection = document.getElementById('viewerSection');
    this.patForm = document.getElementById('patForm');
    this.ownerInput = document.getElementById('ownerInput');
    this.repoInput = document.getElementById('repoInput');
    this.pathInput = document.getElementById('pathInput');
    this.patInput = document.getElementById('patInput');
    this.authButton = document.getElementById('authButton');
    this.authError = document.getElementById('authError');
    this.authStatus = document.getElementById('authStatus');
    this.recordsTable = document.getElementById('recordsTable');

    this.init();
  }

  async init() {
    // Use shared auth from parent (packs/index.html)
    const hasSharedAuth = sharedAuth && sharedAuth.isAuthenticated();

    if (hasSharedAuth) {
      // Use credentials from shared auth
      const { owner, repo } = sharedAuth.getRepoInfo();
      githubAPI.setRepository(owner, repo, 'data/wallet/records.jsonl');
      githubAPI.setPAT(sharedAuth.getPAT());
      this.setSharedAuthUI(owner, repo);
    } else {
      // Fallback to local auth if shared auth not available
      const hasAuth = restoreAuth();
      if (hasAuth) {
        this.restoreRepoConfig();
      } else {
        this.setDefaultRepoConfig();
      }
    }

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

  setSharedAuthUI(owner, repo) {
    // When using shared auth, hide the auth form and show viewer
    this.patForm.style.display = 'none';
    this.authStatus.textContent = `Using shared credentials (${owner}/${repo})`;
    // Auto-load data
    this.loadRecordsWithSharedAuth();
  }

  async loadRecordsWithSharedAuth() {
    try {
      this.authButton.disabled = true;
      this.authButton.textContent = 'Loading...';
      appState.setLoading(true);
      appState.setError(null);

      const records = await githubAPI.fetchRecords();
      appState.setRecords(records);
      appState.setAuthenticated(true);
      appState.setLoading(false);
      this.showViewerSection();
    } catch (error) {
      appState.setError(error.message);
      this.showAuthError(error.message);
      appState.setLoading(false);
      this.authButton.disabled = false;
      this.authButton.textContent = 'Load Records';
    }
  }

  restoreRepoConfig() {
    const stored = localStorage.getItem('wallet_repo_config');
    if (stored) {
      try {
        const config = JSON.parse(stored);
        this.ownerInput.value = config.owner || 'sumitasok';
        this.repoInput.value = config.repo || 'automation-monorepo-data';
        this.pathInput.value = config.path || 'wallet/records.jsonl';
        githubAPI.setRepository(config.owner, config.repo, config.path);
      } catch (e) {
        // Silently fail, use defaults
        this.setDefaultRepoConfig();
      }
    } else {
      this.setDefaultRepoConfig();
    }
  }

  setDefaultRepoConfig() {
    this.ownerInput.value = 'sumitasok';
    this.repoInput.value = 'automation-monorepo-data';
    this.pathInput.value = 'wallet/records.jsonl';
    githubAPI.setRepository('sumitasok', 'automation-monorepo-data', 'wallet/records.jsonl');
  }

  saveRepoConfig(owner, repo, path) {
    localStorage.setItem('wallet_repo_config', JSON.stringify({ owner, repo, path }));
  }

  async handleAuthSubmit(e) {
    e.preventDefault();

    const owner = this.ownerInput.value.trim();
    const repo = this.repoInput.value.trim();
    const path = this.pathInput.value.trim();
    const pat = this.patInput.value.trim();

    console.log('Form submitted:', { owner, repo, path, pat: pat ? '***' : '' });

    if (!owner || !repo || !path || !pat) {
      this.showAuthError('Please fill in all fields (Owner, Repository, Path, PAT)');
      return;
    }

    try {
      this.authError.style.display = 'none';
      this.authButton.disabled = true;
      this.authButton.textContent = 'Loading...';

      appState.setLoading(true);
      appState.setError(null);

      // Configure and set PAT
      console.log('Configuring API with:', { owner, repo, path });
      githubAPI.setRepository(owner, repo, path);
      githubAPI.setPAT(pat);

      // Fetch records
      const records = await githubAPI.fetchRecords();

      // Store config and PAT securely
      this.saveRepoConfig(owner, repo, path);
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
