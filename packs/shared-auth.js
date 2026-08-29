// Shared Authentication & Configuration
// Used by all viewers (wallet, gmail, etc.)

class SharedAuth {
  constructor() {
    this.pat = null;
    this.owner = null;
    this.repo = null;
    this.listeners = [];
  }

  // Subscribe to auth changes
  subscribe(callback) {
    this.listeners.push(callback);
  }

  notify() {
    this.listeners.forEach(cb => cb(this));
  }

  // Set credentials (PAT + repo)
  setCredentials(pat, owner, repo) {
    this.pat = pat;
    this.owner = owner;
    this.repo = repo;
    this.saveToStorage();
    this.notify();
  }

  // Get repo info
  getRepoInfo() {
    return { owner: this.owner, repo: this.repo };
  }

  // Get PAT
  getPAT() {
    return this.pat;
  }

  // Check if authenticated
  isAuthenticated() {
    return !!(this.pat && this.owner && this.repo);
  }

  // Clear credentials
  clearCredentials() {
    this.pat = null;
    this.owner = null;
    this.repo = null;
    deleteAuthCookie();
    localStorage.removeItem('shared_auth_config');
    this.notify();
  }

  // Save to storage
  saveToStorage() {
    if (this.pat) {
      setAuthCookie(this.pat);
    }
    if (this.owner && this.repo) {
      localStorage.setItem('shared_auth_config', JSON.stringify({
        owner: this.owner,
        repo: this.repo
      }));
    }
  }

  // Restore from storage
  restoreFromStorage() {
    const pat = getAuthCookie();
    const storedConfig = localStorage.getItem('shared_auth_config');

    if (pat) {
      this.pat = pat;
    }

    if (storedConfig) {
      try {
        const config = JSON.parse(storedConfig);
        this.owner = config.owner;
        this.repo = config.repo;
      } catch (e) {
        localStorage.removeItem('shared_auth_config');
      }
    }

    this.notify();
    return this.isAuthenticated();
  }
}

// Cookie management (shared)
function setAuthCookie(pat) {
  const expiryDate = new Date();
  expiryDate.setDate(expiryDate.getDate() + 30);
  document.cookie = `shared_github_pat=${encodeURIComponent(pat)}; path=/; expires=${expiryDate.toUTCString()}; SameSite=Strict${window.location.protocol === 'https:' ? '; Secure' : ''}`;
}

function getAuthCookie() {
  const name = 'shared_github_pat=';
  const cookies = document.cookie.split(';');
  for (let cookie of cookies) {
    cookie = cookie.trim();
    if (cookie.startsWith(name)) {
      try {
        return decodeURIComponent(cookie.substring(name.length));
      } catch (e) {
        return null;
      }
    }
  }
  return null;
}

function deleteAuthCookie() {
  document.cookie = 'shared_github_pat=; path=/; expires=Thu, 01 Jan 1970 00:00:00 UTC;';
}

// Global shared auth instance
const sharedAuth = new SharedAuth();

// Restore on page load
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', () => {
    sharedAuth.restoreFromStorage();
  });
} else {
  sharedAuth.restoreFromStorage();
}
