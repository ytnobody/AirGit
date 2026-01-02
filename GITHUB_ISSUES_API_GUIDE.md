# GitHub Issues API Guide

This guide describes how to use the GitHub Issues API endpoints in AirGit.

## Overview

AirGit provides two main endpoints for interacting with GitHub issues:
1. **List Issues**: Browse and display GitHub issues from a repository
2. **Create Issue**: Create new GitHub issues directly from AirGit

## Authentication

All GitHub-related endpoints require GitHub CLI authentication. Before using these endpoints:

```bash
# Authenticate with GitHub (requires copilot scope)
gh auth login -s copilot

# Verify authentication
gh auth status
```

## Endpoints

### List GitHub Issues

**Endpoint**: `GET /api/github/issues`

Lists all open issues from the GitHub repository.

#### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| repoPath | string | No | Optional repository path relative to base path |

#### Response

**Success (200)**:
```json
{
  "owner": "username",
  "repo": "repository-name",
  "remoteUrl": "https://github.com/username/repository-name.git",
  "issues": [
    {
      "number": 1,
      "title": "Issue title",
      "body": "Issue description",
      "author": "username",
      "assignees": ["username"]
    }
  ]
}
```

**Error (404)**:
```json
{
  "error": "No GitHub remote found. Make sure 'origin' remote is configured."
}
```

**Error (500)**:
```json
{
  "owner": "username",
  "repo": "repository-name",
  "remoteUrl": "https://github.com/username/repository-name.git",
  "issues": [],
  "error": "GitHub CLI error message"
}
```

#### Example

```bash
curl http://localhost:8080/api/github/issues
```

---

### Create GitHub Issue

**Endpoint**: `POST /api/github/issues/create`

Creates a new GitHub issue in the repository.

#### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| repoPath | string | No | Optional repository path relative to base path |

#### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| title | string | Yes | Issue title |
| body | string | No | Issue description/body (supports Markdown) |
| labels | array[string] | No | Array of label names to attach to the issue |

#### Response

**Success (200)**:
```json
{
  "success": true,
  "url": "https://github.com/username/repository-name/issues/42",
  "owner": "username",
  "repo": "repository-name",
  "message": "Issue created successfully: https://github.com/username/repository-name/issues/42"
}
```

**Error (400 - Missing Title)**:
```json
{
  "error": "Title is required"
}
```

**Error (400 - Invalid Body)**:
```json
{
  "error": "Invalid request body"
}
```

**Error (404 - No Remote)**:
```json
{
  "error": "No GitHub remote found. Make sure 'origin' remote is configured."
}
```

**Error (500 - Creation Failed)**:
```json
{
  "error": "GitHub CLI error message",
  "owner": "username",
  "repo": "repository-name"
}
```

#### Example: Create Simple Issue

```bash
curl -X POST http://localhost:8080/api/github/issues/create \
  -H "Content-Type: application/json" \
  -d '{
    "title": "New Feature Request"
  }'
```

#### Example: Create Issue with Body and Labels

```bash
curl -X POST http://localhost:8080/api/github/issues/create \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Fix: Login page not responsive",
    "body": "The login page does not display correctly on mobile devices.\n\n## Steps to reproduce\n1. Open login page on mobile\n2. Observe layout issues",
    "labels": ["bug", "mobile"]
  }'
```

#### Example: Create Issue with Specific Repository

```bash
curl -X POST "http://localhost:8080/api/github/issues/create?repoPath=my-repo" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Documentation update needed"
  }'
```

## Error Handling

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| "No GitHub remote found" | Repository doesn't have 'origin' remote | Configure GitHub remote: `git remote add origin <url>` |
| "Could not parse GitHub repository" | Invalid remote URL format | Ensure remote URL is valid GitHub URL |
| "Not authenticated" | GitHub CLI OAuth token invalid | Run `gh auth login -s copilot` |
| "404 Not Found" | Issue or repository doesn't exist | Verify repository exists and authentication is valid |

### Detailed Troubleshooting

1. **Authentication Fails**
   ```bash
   # Check authentication status
   gh auth status
   
   # Re-authenticate
   gh auth logout
   gh auth login -s copilot
   ```

2. **Remote Configuration**
   ```bash
   # List configured remotes
   git remote -v
   
   # Add/update GitHub remote
   git remote set-url origin https://github.com/username/repo.git
   ```

3. **Permission Issues**
   Ensure your GitHub token has the following scopes:
   - `repo` (full repository access)
   - `copilot` (for CLI usage)

## WebUI Integration

### List Issues

The web UI automatically detects the GitHub repository from the current git remote and displays issues. Navigate to the "Issues" section to view all open issues.

### Create Issue

To create a new issue from the web UI:
1. Click "New Issue" button
2. Fill in the issue title (required)
3. Add optional description and labels
4. Click "Create"

## Rate Limiting

GitHub API has rate limits:
- **Authenticated requests**: 5,000 requests per hour
- **Issue creation**: Subject to GitHub API rate limits

If you exceed rate limits, you'll receive HTTP 403 error. Wait for the rate limit window to reset.

## Security Notes

- All endpoints validate repository paths to prevent directory traversal
- GitHub CLI authentication is required for these endpoints
- Sensitive data (like tokens) should never be logged or exposed in responses
- Repository path is restricted to the configured base path

## See Also

- [GitHub CLI Documentation](https://cli.github.com/manual/)
- [GitHub REST API - Issues](https://docs.github.com/en/rest/issues)
- [AirGit README](./README.md)
