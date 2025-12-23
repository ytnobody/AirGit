# Git Tag Management API Guide

This guide covers the new git tag management endpoints added to AirGit.

## Overview

AirGit now supports three new REST endpoints for managing git tags:

- **GET /api/tags** - List all tags
- **POST /api/tag/create** - Create a new tag
- **POST /api/tag/push** - Push tags to remote

## Endpoints

### 1. List Tags

**Request:**
```
GET /api/tags?repoPath=optional/path
```

**Query Parameters:**
- `repoPath` (optional): Relative path to the repository

**Response:**
```json
{
  "tags": ["v1.0.0", "v1.0.1", "v1.1.0"]
}
```

**Example:**
```bash
# List tags in current repository
curl http://localhost:8080/api/tags

# List tags in specific repository
curl "http://localhost:8080/api/tags?repoPath=projects/my-repo"
```

---

### 2. Create Tag

**Request:**
```
POST /api/tag/create
Content-Type: application/json
```

**Request Body:**
```json
{
  "tagName": "v1.0.0",
  "message": "Optional message for annotated tag"
}
```

**Parameters:**
- `tagName` (required): Name of the tag
- `message` (optional): Message for annotated tag. If omitted, creates lightweight tag.

**Response:**
```json
{
  "commit": "v1.0.0",
  "log": [
    "$ git tag -a v1.0.0 -m \"Optional message for annotated tag\"",
    "✓ Tag 'v1.0.0' created!"
  ]
}
```

**Examples:**

Create lightweight tag:
```bash
curl -X POST http://localhost:8080/api/tag/create \
  -H "Content-Type: application/json" \
  -d '{"tagName":"v1.0.0"}'
```

Create annotated tag:
```bash
curl -X POST http://localhost:8080/api/tag/create \
  -H "Content-Type: application/json" \
  -d '{"tagName":"v1.0.0","message":"Release version 1.0.0"}'
```

---

### 3. Push Tags

**Request:**
```
POST /api/tag/push?remote=origin&repoPath=optional/path
Content-Type: application/json
```

**Query Parameters:**
- `remote` (optional): Remote name to push to (default: `origin`)
- `repoPath` (optional): Relative path to the repository

**Request Body:**

Push single tag:
```json
{
  "tagName": "v1.0.0",
  "all": false
}
```

Push all tags:
```json
{
  "all": true
}
```

**Parameters:**
- `tagName` (required if `all` is false): Name of the tag to push
- `all` (optional): If true, push all tags. If false, push specific tag (default: false)

**Response:**
```json
{
  "log": [
    "$ git push origin v1.0.0",
    "✓ Push successful!"
  ]
}
```

**Examples:**

Push single tag:
```bash
curl -X POST http://localhost:8080/api/tag/push?remote=origin \
  -H "Content-Type: application/json" \
  -d '{"tagName":"v1.0.0"}'
```

Push all tags:
```bash
curl -X POST http://localhost:8080/api/tag/push?remote=origin \
  -H "Content-Type: application/json" \
  -d '{"all":true}'
```

Push to different remote:
```bash
curl -X POST http://localhost:8080/api/tag/push?remote=upstream \
  -H "Content-Type: application/json" \
  -d '{"tagName":"v2.0.0"}'
```

---

## Common Workflows

### Create and Push Release Tag

```bash
# 1. Create annotated tag
curl -X POST http://localhost:8080/api/tag/create \
  -H "Content-Type: application/json" \
  -d '{"tagName":"v1.0.0","message":"Release v1.0.0"}'

# 2. Push tag to remote
curl -X POST http://localhost:8080/api/tag/push?remote=origin \
  -H "Content-Type: application/json" \
  -d '{"tagName":"v1.0.0"}'
```

### Publish All Tags

```bash
# Push all tags at once
curl -X POST http://localhost:8080/api/tag/push?remote=origin \
  -H "Content-Type: application/json" \
  -d '{"all":true}'
```

### List and Manage Tags

```bash
# List all current tags
curl http://localhost:8080/api/tags

# Create a new tag
curl -X POST http://localhost:8080/api/tag/create \
  -H "Content-Type: application/json" \
  -d '{"tagName":"v1.1.0"}'

# Push the new tag
curl -X POST http://localhost:8080/api/tag/push?remote=origin \
  -H "Content-Type: application/json" \
  -d '{"tagName":"v1.1.0"}'
```

---

## Error Handling

All endpoints return appropriate HTTP status codes:

- **200 OK**: Operation successful
- **400 Bad Request**: Invalid request (missing required fields)
- **405 Method Not Allowed**: Wrong HTTP method used
- **500 Internal Server Error**: Git command failed

Error response example:
```json
{
  "error": "Failed to create tag: error message details",
  "log": ["$ git tag v1.0.0", "error output"]
}
```

---

## Notes

- Tags are created on the current commit in the repository
- Lightweight tags are faster and simpler
- Annotated tags store additional information (tagger, date, message)
- Use annotated tags for releases or important milestones
- The `message` field is optional; omitting it creates a lightweight tag
- All operations respect the configured repository base path for security
