# Permalink URLs - Usage Guide

This document explains how to use AirGit's permalink URL feature to create shareable links that automatically open a specific repository and branch.

## Overview

Permalink URLs allow you to encode the repository path and branch name directly in the URL. When someone opens the URL, AirGit automatically:

1. Switches to the specified repository (if provided)
2. Checks out the specified branch (if provided)
3. Displays the updated repository and branch information

## URL Format

```
http://my-airgit-server:8000?path=<repository-path>&branch=<branch-name>
```

## Query Parameters

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `path` | No | Absolute path to Git repository | `/var/git/my-repo` or `/home/user/projects/app` |
| `branch` | No | Branch name to checkout | `main`, `develop`, `feature/my-feature` |

## Examples

### Basic Examples

1. **Switch repository only:**
   ```
   http://localhost:8000?path=/var/git/my-repo
   ```
   - Opens AirGit and switches to `/var/git/my-repo`
   - Current branch stays the same

2. **Switch branch only:**
   ```
   http://localhost:8000?branch=develop
   ```
   - Opens AirGit with current repository
   - Checks out the `develop` branch

3. **Switch both repository and branch:**
   ```
   http://localhost:8000?path=/var/git/my-repo&branch=develop
   ```
   - Opens AirGit, switches to `/var/git/my-repo`
   - Checks out the `develop` branch

### Real-World Examples

**Development Setup:**
```
http://my-airgit-server.local:8000?path=/home/dev/projects/web-app&branch=develop
```

**Feature Branch:**
```
http://my-airgit-server.local:8000?path=/var/git/backend&branch=feature/user-auth
```

**Production Push:**
```
http://my-airgit-server.local:8000?path=/var/git/main-app&branch=main
```

## Use Cases

### 1. Team Coordination

Create standardized links for your team members:
- Each team can have a link to their repository and main branch
- Share the link via Slack, email, or documentation

```
# Marketing Team's Push Link
http://airgit.company.com?path=/var/git/marketing-site&branch=main

# Backend Team's Push Link
http://airgit.company.com?path=/var/git/api-server&branch=develop
```

### 2. Documentation

Include quick-push links in your project documentation:

```markdown
## Quick Deploy

Push changes to production:
[Push to Main](http://airgit.local:8000?path=/var/git/frontend&branch=main)

Push to staging:
[Push to Develop](http://airgit.local:8000?path=/var/git/frontend&branch=develop)
```

### 3. Mobile Bookmarks

Create home screen bookmarks on iOS/Android for quick access:

1. Open the permalink URL in your mobile browser
2. iOS: Tap Share → Add to Home Screen
3. Android: Tap ⋮ (menu) → Install App
4. The bookmark will open directly to the correct repository and branch

### 4. CI/CD Integration

Reference these URLs in CI/CD pipelines for manual push steps:

```yaml
# In your GitHub Actions workflow
- name: Push to repository
  run: |
    echo "Push using: http://airgit.local:8000?path=${{ env.REPO_PATH }}&branch=${{ env.BRANCH }}"
```

### 5. Quick Links in Chat/Email

Share in team communication:

```
🚀 Need to push? Click here:
http://airgit.local:8000?path=/var/git/my-app&branch=hotfix/urgent-bug
```

## Behavior Details

### Repository Switching
- If `path` parameter is provided, the server switches to that repository
- Path must be an absolute path to a valid Git repository
- Invalid paths will result in an error response

### Branch Checkout
- If `branch` parameter is provided and differs from the current branch, the server attempts to checkout that branch
- If checkout fails (e.g., branch doesn't exist), an error is returned but the path change persists
- If the requested branch equals the current branch, no checkout is performed (no-op)

### Error Handling
- If initialization fails, the UI displays an error message
- The server still responds with the current repository path and branch
- You can retry by modifying the URL or using the UI controls

## Security Considerations

⚠️ **Important:**

- **Path validation:** The `path` parameter should only specify repositories that are accessible by your AirGit server
- **Authorization:** Ensure your AirGit server is behind proper authentication/firewall
- **Branch names:** Only existing branches can be checked out; the system won't create branches via URL parameters
- **Sharing URLs:** Be cautious when sharing URLs that point to specific repositories - ensure you trust the recipient

## Troubleshooting

### "Failed to checkout branch"
- Verify the branch exists in the repository
- Check that the branch name is spelled correctly (case-sensitive)
- Ensure the repository is accessible at the specified path

### "Failed to switch repository"
- Verify the repository path is absolute and exists
- Check file permissions for the AirGit server process
- Ensure the directory is a valid Git repository (contains `.git`)

### Branch shows old name
- Refresh the page or click a different link
- The `/api/status` endpoint is called every 5 seconds to sync display
- Manual status update occurs when clicking UI elements

## API Endpoint

The permalink functionality uses the `/api/init` endpoint:

**Request:**
```
GET /api/init?path=/var/git/repo&branch=main
```

**Response (Success):**
```json
{
  "path": "/var/git/repo",
  "branch": "main"
}
```

**Response (Error):**
```json
{
  "error": "Failed to checkout branch 'main': branch not found",
  "path": "/var/git/repo",
  "branch": "develop"
}
```

## See Also

- [README.md](./README.md) - Main documentation
- [API Endpoints](./README.md#api-endpoints) - Complete API reference
