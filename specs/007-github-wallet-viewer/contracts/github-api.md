# Contract: GitHub API for Records Retrieval

## Overview

The UI authenticates with GitHub API using a Personal Access Token (PAT) to fetch transaction records stored in a GitHub repository. This contract defines the API interactions required.

## Authentication

**Method**: OAuth Bearer Token (HTTP Authorization header)

**Header Format**:
```
Authorization: Bearer {PAT}
```

**Token Requirements**:
- Scope: `repo` (full read/write access) or `public_repo` (public repos only)
- Permissions: Read-only access to repository files
- Storage**: Browser cookie (httpOnly, secure, SameSite=Strict)

## Endpoint: Fetch Records File

### Request

```
GET /repos/{owner}/{repo}/contents/data/wallet/records.jsonl
```

**Parameters**:

| Parameter | Type | Location | Required | Description |
|-----------|------|----------|----------|-------------|
| `owner` | string | URL path | Yes | Repository owner (username or org) |
| `repo` | string | URL path | Yes | Repository name |
| `ref` | string | Query | No | Branch/tag (defaults to default branch) |

**Example**:

```
GET /repos/sumitasok/automation-monorepo/contents/data/wallet/records.jsonl?ref=main
```

**Headers**:

```
Authorization: Bearer {PAT}
Accept: application/vnd.github.v3.raw
```

**Rate Limits**:

- Unauthenticated: 60 requests/hour
- Authenticated: 5000 requests/hour

## Response

### Success Response (200 OK)

**Content-Type**: `text/plain` (raw file content with `Accept: application/vnd.github.v3.raw`)

**Body**: Raw file content (JSONL format)

```
{"fetchedAt":"2026-08-29T15:30:45.123Z","recordCount":6329,"apiTotal":6329}
{"id":"txn_...", ...}
{"id":"txn_...", ...}
...
```

**Size Limits**:

- Max response size: ~5-10MB (typical 2MB for 6000+ records)
- Streaming recommended for large files

### Error Responses

| Status | Reason | Handling |
|--------|--------|----------|
| 401 Unauthorized | Invalid or expired PAT | Show auth error, prompt for new token |
| 403 Forbidden | PAT lacks permissions | Show permission error, advise user |
| 404 Not Found | Repository or file doesn't exist | Show clear error message |
| 429 Too Many Requests | Rate limit exceeded | Implement exponential backoff retry |
| 500 Server Error | GitHub API error | Show temporary error, retry after delay |

**Error Response Format**:

```json
{
  "message": "Not Found",
  "documentation_url": "https://docs.github.com/rest/..."
}
```

## Repository Configuration

### Getting Repository Info

The UI must determine the repository owner and name. Preferred sources:

1. **Local .git/config** (if UI runs locally or has access):
   ```ini
   [remote "origin"]
       url = git@github.com:sumitasok/automation-monorepo.git
   ```

2. **Fallback Default**:
   - Owner: `sumitasok`
   - Repo: `automation-monorepo`

3. **User Configuration**:
   - Allow user to override owner/repo in UI if needed

## Implementation Requirements

### Security

- PAT must NEVER appear in:
  - Console logs
  - HTML attributes
  - Network request body (always in Authorization header)
  - Stored in localStorage (httpOnly cookies only)
- Use `Authorization: Bearer` header (not query parameter)
- Reject non-HTTPS requests when deploying (secure cookie requires HTTPS)

### Performance

- Implement request caching (browser cache-control headers)
- Rate limit client-side requests (max 1 fetch per 10 seconds)
- Display loading indicator while fetching
- Show progress for large files (if streaming)

### Error Handling

- Implement exponential backoff for 429 responses (start 1s, max 32s)
- Provide user-friendly error messages for each error type
- Log detailed errors for debugging (not exposed to user)
- Retry transient failures (5xx) automatically

### Testing

**Mock API Responses**:

For local testing, mock these responses:

```javascript
// Mock successful response
const mockRecords = {
  "fetchedAt": "2026-08-29T15:30:45.123Z",
  "recordCount": 100,
  "apiTotal": 100,
  "records": [/* array of 100 transaction objects */]
}

// Mock auth error
{
  "message": "Bad credentials",
  "documentation_url": "https://docs.github.com/..."
}

// Mock rate limit error
{
  "message": "API rate limit exceeded",
  "documentation_url": "https://docs.github.com/..."
}
```

## Usage Example

```javascript
async function fetchRecords(owner, repo, pat) {
  try {
    const response = await fetch(
      `https://api.github.com/repos/${owner}/${repo}/contents/data/wallet/records.jsonl`,
      {
        headers: {
          'Authorization': `Bearer ${pat}`,
          'Accept': 'application/vnd.github.v3.raw'
        }
      }
    );
    
    if (!response.ok) {
      throw new Error(`GitHub API: ${response.status} ${response.statusText}`);
    }
    
    const content = await response.text();
    return parseJSONL(content);
  } catch (error) {
    console.error('Failed to fetch records:', error);
    throw error;
  }
}
```
