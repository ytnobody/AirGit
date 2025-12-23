# Git Remote Management API

This document describes the API endpoints for managing Git remotes (add, update, remove).

## Overview

The AirGit application now provides three new HTTP endpoints for managing remote repositories:
- `POST /api/remote/add` - Add a new remote
- `POST /api/remote/update` - Update an existing remote URL
- `POST /api/remote/remove` - Remove a remote

All endpoints support the optional `repoPath` query parameter to specify which repository to operate on.

## Endpoints

### 1. Add Remote
**Endpoint:** `POST /api/remote/add`

**Query Parameters:**
- `repoPath` (optional) - Path to the Git repository. If not provided, uses the configured default repository path.

**Request Body (JSON):**
```json
{
  "name": "origin",
  "url": "https://github.com/user/repo.git"
}
```

**Response (Success - 200):**
```json
{
  "success": true,
  "message": "Remote 'origin' added successfully"
}
```

**Response (Error - 400):**
```json
{
  "error": "Failed to add remote: error details"
}
```

**Example cURL:**
```bash
curl -X POST http://localhost:8080/api/remote/add \
  -H "Content-Type: application/json" \
  -d '{"name":"upstream","url":"https://github.com/user/upstream.git"}'
```

### 2. Update Remote
**Endpoint:** `POST /api/remote/update`

**Query Parameters:**
- `repoPath` (optional) - Path to the Git repository. If not provided, uses the configured default repository path.

**Request Body (JSON):**
```json
{
  "name": "origin",
  "url": "https://github.com/newuser/newrepo.git"
}
```

**Response (Success - 200):**
```json
{
  "success": true,
  "message": "Remote 'origin' updated successfully"
}
```

**Response (Error - 400):**
```json
{
  "error": "Failed to update remote: error details"
}
```

**Example cURL:**
```bash
curl -X POST http://localhost:8080/api/remote/update \
  -H "Content-Type: application/json" \
  -d '{"name":"origin","url":"https://github.com/newuser/newrepo.git"}'
```

### 3. Remove Remote
**Endpoint:** `POST /api/remote/remove`

**Query Parameters:**
- `repoPath` (optional) - Path to the Git repository. If not provided, uses the configured default repository path.

**Request Body (JSON):**
```json
{
  "name": "upstream"
}
```

**Response (Success - 200):**
```json
{
  "success": true,
  "message": "Remote 'upstream' removed successfully"
}
```

**Response (Error - 400):**
```json
{
  "error": "Failed to remove remote: error details"
}
```

**Example cURL:**
```bash
curl -X POST http://localhost:8080/api/remote/remove \
  -H "Content-Type: application/json" \
  -d '{"name":"upstream"}'
```

## Existing Endpoint

### List Remotes
**Endpoint:** `GET /api/remotes`

**Query Parameters:**
- `repoPath` (optional) - Path to the Git repository.

**Response:**
```json
{
  "remotes": [
    {
      "name": "origin",
      "url": "https://github.com/user/repo.git"
    },
    {
      "name": "upstream",
      "url": "https://github.com/upstream/repo.git"
    }
  ]
}
```

## Error Handling

All endpoints return appropriate HTTP status codes:
- `200 OK` - Request succeeded
- `400 Bad Request` - Invalid request or Git operation failed
- `405 Method Not Allowed` - Incorrect HTTP method used

Error responses include a JSON object with an `error` field describing what went wrong.

## Implementation Details

The handlers follow the same pattern as other AirGit endpoints:
1. Check HTTP method (POST required for add/update/remove)
2. Set JSON response header
3. Resolve and validate the repository path
4. Decode and validate the JSON request body
5. Execute the appropriate Git command
6. Return JSON response with success/error message

## Requirements

- Valid JSON request body
- Required fields must be present (name, url for add/update; name for remove)
- Target repository must exist (for add/update/remove)
- Remote name must be valid for the operation (existing for update/remove, non-existing for add)

## Git Commands Used

Internally, these endpoints execute the following Git commands:
- Add: `git remote add <name> <url>`
- Update: `git remote set-url <name> <url>`
- Remove: `git remote remove <name>`
