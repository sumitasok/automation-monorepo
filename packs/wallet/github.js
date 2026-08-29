class GitHubAPI {
  constructor() {
    this.owner = 'sumitasok';
    this.repo = 'automation-monorepo';
    this.path = 'data/wallet/records.jsonl';
  }

  // Set repository owner, name, and file path
  setRepository(owner, repo, path = 'data/wallet/records.jsonl') {
    this.owner = owner;
    this.repo = repo;
    this.path = path;
  }

  // Set PAT (never log or expose)
  setPAT(pat) {
    this.pat = pat;
  }

  // Get PAT
  getPAT() {
    return this.pat;
  }

  // Fetch records from GitHub
  async fetchRecords() {
    if (!this.pat) {
      throw new Error('GitHub PAT not set');
    }

    try {
      const url = `https://api.github.com/repos/${this.owner}/${this.repo}/contents/${this.path}`;
      
      const response = await fetch(url, {
        headers: {
          'Authorization': `Bearer ${this.pat}`,
          'Accept': 'application/vnd.github.v3.raw'
        }
      });

      if (!response.ok) {
        let errorMsg = `GitHub API error: ${response.status}`;
        
        if (response.status === 401) {
          errorMsg = 'Authentication failed. Check your GitHub PAT.';
        } else if (response.status === 403) {
          errorMsg = 'Access denied. Ensure your PAT has repo access.';
        } else if (response.status === 404) {
          errorMsg = 'Repository or records file not found.';
        } else if (response.status === 429) {
          errorMsg = 'Rate limited. Please try again later.';
        }
        
        throw new Error(errorMsg);
      }

      const content = await response.text();
      
      if (!content.trim()) {
        throw new Error('Records file is empty');
      }

      // Parse JSONL
      const records = parseJSONL(content);
      
      if (records.length === 0) {
        throw new Error('No valid records found in file');
      }

      return records;
    } catch (error) {
      if (error instanceof TypeError) {
        throw new Error('Network error. Check your internet connection.');
      }
      throw error;
    }
  }

  // Get repository info from .git/config if available locally
  async getRepoInfo() {
    try {
      const response = await fetch('.git/config');
      if (response.ok) {
        const text = await response.text();
        const match = text.match(/url = git@github\.com:([^/]+)\/([^.]+)\.git/);
        if (match) {
          this.owner = match[1];
          this.repo = match[2];
        }
      }
    } catch (e) {
      // Silently fail - use defaults
    }
  }
}

// Global GitHub API instance
const githubAPI = new GitHubAPI();

// Cookie management for PAT
function setAuthCookie(pat) {
  // Set secure httpOnly cookie with SameSite=Strict
  const expiryDate = new Date();
  expiryDate.setDate(expiryDate.getDate() + 30);  // 30 days
  
  document.cookie = `wallet_github_pat=${encodeURIComponent(pat)}; path=/; expires=${expiryDate.toUTCString()}; SameSite=Strict${window.location.protocol === 'https:' ? '; Secure' : ''}`;
  
  githubAPI.setPAT(pat);
}

function getAuthCookie() {
  const name = 'wallet_github_pat=';
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
  document.cookie = 'wallet_github_pat=; path=/; expires=Thu, 01 Jan 1970 00:00:00 UTC;';
  githubAPI.setPAT(null);
}

// Restore PAT from cookie on page load
function restoreAuth() {
  const pat = getAuthCookie();
  if (pat) {
    githubAPI.setPAT(pat);
    return true;
  }
  return false;
}
