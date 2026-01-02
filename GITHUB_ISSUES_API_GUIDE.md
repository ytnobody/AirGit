# GitHub Issues Management API Guide

This guide covers the GitHub Issues management endpoints in AirGit.

## Overview

AirGit provides REST endpoints for working with GitHub issues:

- **GET /api/github/issues** - List all issues from the GitHub repository
- **POST /api/github/issues/create** - Create a new issue on GitHub

## Prerequisites

- Repository with a configured GitHub remote (`origin`)
- GitHub CLI (`gh`) installed and authenticated
- Valid GitHub OAuth token or personal access token

## Endpoints

### 1. List Issues

**Request:**
```
GET /api/github/issues?repoPath=optional/path
```

**Query Parameters:**
- `repoPath` (optional): Relative path to the repository

**Response:**
```json
{
  "owner": "username",
  "repo": "repository-name",
  "remoteUrl": "https://github.com/username/repository-name.git",
  "issues": [
    {
      "number": 1,
      "title": "First issue",
      "body": "Issue description",
      "author": "user1"
    },
    {
      "number": 2,
      "title": "Second issue",
      "body": "Another issue description",
      "author": "user2"
    }
  ]
}
```

**Example:**
```bash
# List issues in current repository
curl http://localhost:8080/api/github/issues

# List issues in specific repository
curl "http://localhost:8080/api/github/issues?repoPath=projects/my-repo"
```

---

### 2. Create Issue

**Request:**
```
POST /api/github/issues/create?repoPath=optional/path
Content-Type: application/json
```

**Query Parameters:**
- `repoPath` (optional): Relative path to the repository

**Request Body:**
```json
{
  "title": "Issue title",
  "body": "Detailed issue description",
  "labels": ["bug", "enhancement"]
}
```

**Parameters:**
- `title` (required): Title of the issue
- `body` (optional): Detailed description of the issue
- `labels` (optional): Array of labels to assign to the issue

**Response:**
```json
{
  "success": true,
  "url": "https://github.com/owner/repo/issues/42",
  "owner": "owner",
  "repo": "repo"
}
```

**Examples:**

Create simple issue:
```bash
curl -X POST http://localhost:8080/api/github/issues/create \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Fix login bug",
    "body": "Users are unable to login with OAuth"
  }'
```

Create issue with labels:
```bash
curl -X POST http://localhost:8080/api/github/issues/create \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Add dark mode support",
    "body": "Implement dark mode theme for the application",
    "labels": ["enhancement", "ui"]
  }'
```

Create issue in specific repository:
```bash
curl -X POST "http://localhost:8080/api/github/issues/create?repoPath=projects/my-repo" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Update dependencies",
    "body": "Update npm packages to latest versions"
  }'
```

---

## Common Workflows

### Create Issue with Full Details

```bash
curl -X POST http://localhost:8080/api/github/issues/create \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Refactor authentication module",
    "body": "## Background\nThe current auth module is becoming complex.\n\n## Proposed Changes\n1. Separate OAuth and session handling\n2. Add token refresh logic\n3. Improve error messages",
    "labels": ["refactoring", "authentication"]
  }'
```

### List and Create Issues

```bash
# 1. List existing issues
curl http://localhost:8080/api/github/issues

# 2. Create a new issue based on discussion
curl -X POST http://localhost:8080/api/github/issues/create \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Implement feature requested in issue #10",
    "body": "Following up on the discussion in #10, let'\''s implement this feature",
    "labels": ["feature"]
  }'
```

---

## Error Handling

All endpoints return appropriate HTTP status codes:

- **200 OK**: Operation successful
- **400 Bad Request**: Invalid request (missing required fields, invalid format)
- **404 Not Found**: GitHub remote not configured or not found
- **405 Method Not Allowed**: Wrong HTTP method used
- **500 Internal Server Error**: GitHub API error or gh CLI command failed

Error response example for missing title:
```json
{
  "error": "Title is required"
}
```

Error response for missing GitHub remote:
```json
{
  "error": "No GitHub remote found. Make sure 'origin' remote is configured."
}
```

Error response for GitHub API failure:
```json
{
  "error": "API rate limit exceeded",
  "message": "Failed to create issue: exit status 1"
}
```

---

## Authentication

### Using GitHub CLI Authentication

The issue creation feature uses the GitHub CLI (`gh`) for authentication. Ensure you're authenticated:

```bash
# Check authentication status
gh auth status

# Login if needed
gh auth login

# Login with specific scopes
gh auth login --scopes repo
```

### Environment Variables

The application respects standard GitHub environment variables:

- `GITHUB_TOKEN`: Personal access token (alternative to OAuth)
- `GH_TOKEN`: GitHub CLI token

For OAuth token (recommended):
```bash
# The oauth token is stored in ~/.config/gh/hosts.yml
# This is set up by running: gh auth login
```

---

## Notes

- Issues are created on the remote specified by `origin` remote
- The GitHub CLI (`gh`) must be installed and authenticated
- Repository must have `origin` remote pointing to GitHub
- Both HTTPS and SSH remote URLs are supported
- Label names must exist in the repository or GitHub will return an error
- Issue body supports Markdown formatting
- Requires write access to the repository to create issues

---

## Related Documentation

- [GitHub Issues API Reference](https://docs.github.com/en/rest/issues)
- [GitHub CLI Documentation](https://cli.github.com/manual/)
- [Markdown Formatting](https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github)
