# AirGit

![AirGit Logo](logo.png)

A lightweight web-based GUI for managing Git repositories directly from your browser. Push, pull, create branches, manage remotes, and more - all through an intuitive mobile-friendly interface.

## Features

- 📱 Mobile-optimized interface with single-tap operations
- 📚 Multiple repository management (local filesystem)
- 🌿 Full branch management (list, create, checkout, delete)
- 🏷️ Git tag management (list, create, push)
- 🔄 Git operations (push, pull, status)
- 🌐 Remote management (add, update, remove, select)
- 💾 Repository initialization and creation
- 📊 Ahead/behind commit tracking
- 🎨 UI optimized for mobile with bottom navigation bar
- 📴 PWA support (offline caching, home screen icon)
- 📋 Commit history viewing (past 20 commits)
- 🔧 Systemd service registration and management
- ⚙️ Settings menu for configuration
- 🚀 Standalone Go binary
- 🤖 **GitHub Issues integration** - Browse and display GitHub issues directly in UI
- 🧠 **AI Agent for issue resolution** - One-click automated issue fixing with PR generation

## Quick Start

### Prerequisites

- Go 1.21+
- Git installed
- Local Git repositories accessible on the file system

### Installation

1. Clone or download the repository
2. Build the binary:

```bash
go build -o airgit
```

3. Set up environment variables (optional) or use command-line flags:

```bash
export AIRGIT_LISTEN_ADDR=0.0.0.0
export AIRGIT_LISTEN_PORT=8080
export AIRGIT_REPO_PATH=$HOME
```

4. Run:

```bash
./airgit
```

5. Open http://localhost:8080 in your browser

### Using Release Binaries

Alternatively, you can download pre-built binaries directly from the [Releases](../../releases) page:

1. Go to the **Releases** page
2. Select the version and download the binary for your OS and architecture:
   - `airgit-linux-amd64` - Linux (x86_64)
   - `airgit-linux-arm64` - Linux (ARM64)
   - `airgit-darwin-amd64` - macOS (Intel)
   - `airgit-darwin-arm64` - macOS (Apple Silicon)
   - `airgit-windows-amd64.exe` - Windows (x86_64)

3. Make the binary executable (Linux/macOS):

```bash
chmod +x airgit-linux-amd64
```

4. Run the binary:

```bash
./airgit-linux-amd64
```

Or on Windows:
```cmd
airgit-windows-amd64.exe
```

### Displaying Help

To see all available options, use:

```bash
./airgit --help
# or
./airgit -h
```

To check the version:

```bash
./airgit --version
# or
./airgit -v
```

## Configuration

Configure via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `AIRGIT_REPO_PATH` | `$HOME` | Base path for repositories (default: user home directory) |
| `AIRGIT_LISTEN_ADDR` | `0.0.0.0` | Server listen address |
| `AIRGIT_LISTEN_PORT` | `8080` | Server listen port |

### Command-Line Flags

Alternatively, use command-line flags (which override environment variables):

| Flag | Description |
|------|-------------|
| `-h`, `--help` | Show help message |
| `-v`, `--version` | Show version information |
| `--repo-path <path>` | Base path for repositories (default: $HOME) |
| `--listen-addr <addr>` | Server listen address (default: 0.0.0.0) |
| `--listen-port <port>` | Server listen port (default: 8080) |
| `-p <port>` | Server listen port (shorthand) |

Example using flags:

```bash
./airgit --repo-path /var/git --listen-port 9000
```

## Multiple Repositories

AirGit supports managing multiple Git repositories on the same filesystem. All repositories must be within the configured `AIRGIT_REPO_PATH` base directory.

### Query Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `path` | Relative path to the Git repository | `projects/my-app` or `/absolute/path` |
| `branch` | Branch name to checkout | `feature/my-feature` |

### Examples

- **Load specific repository**: `http://localhost:8000?path=projects/my-repo`
- **Checkout branch**: `http://localhost:8000?branch=develop`
- **Both**: `http://localhost:8000?path=projects/repo&branch=feature/new`

When you open a URL with these parameters:
1. The server loads the specified repository (if `path` is provided)
2. The branch is automatically checked out (if `branch` is provided and different from current)
3. The UI displays the updated repository path and branch name

## Managing Remotes

### Web UI Remote Management

The AirGit frontend includes a complete UI for managing Git remotes:

1. Click the **"Remotes"** button in the header navigation bar
2. View all configured remotes for the current repository
3. **Add** a new remote with name and URL
4. **Edit** a remote's URL
5. **Remove** a remote from the repository

### REST API - Remote Management

#### Get All Remotes

```bash
GET /api/remotes
```

Response:
```json
{
  "remotes": [
    {
      "name": "origin",
      "url": "https://github.com/user/repo.git"
    },
    {
      "name": "upstream",
      "url": "https://github.com/org/repo.git"
    }
  ]
}
```

#### Add Remote

```bash
POST /api/remote/add
```

Request:
```json
{
  "name": "upstream",
  "url": "https://github.com/user/upstream.git"
}
```

#### Update Remote

```bash
POST /api/remote/update
```

Request:
```json
{
  "name": "origin",
  "url": "https://github.com/newowner/repo.git"
}
```

#### Remove Remote

```bash
POST /api/remote/remove
```

Request:
```json
{
  "name": "origin"
}
```

## API Endpoints

### GET /api/status
Returns current repository status including branch name and ahead/behind counts.

Response:
```json
{
  "branch": "main",
  "repoName": "my-repo",
  "ahead": 2,
  "behind": 1
}
```

### POST /api/push
Executes: `git add .` → `git commit -m "Updated via AirGit"` → `git push [remote]`

Query Parameters:
- `remote` (optional): Remote name to push to (default: `origin`)

Response:
```json
{
  "branch": "main",
  "log": ["$ git add .", "$ git commit...", "..."]
}
```

### POST /api/pull
Executes: `git pull [remote]`

Query Parameters:
- `remote` (optional): Remote name to pull from (default: `origin`)

Response:
```json
{
  "branch": "main",
  "log": ["$ git pull...", "..."]
}
```

### POST /api/checkout
Checkout a branch and return tracking information.

Request Body:
```json
{
  "branch": "feature/my-feature"
}
```

Response:
```json
{
  "branch": "feature/my-feature",
  "ahead": 3,
  "behind": 0,
  "log": ["Switched to branch: feature/my-feature"]
}
```

### GET /api/branches
List all branches in the repository.

Response:
```json
{
  "branches": ["main", "develop", "feature/new"],
  "current": "main"
}
```

### POST /api/branch/create
Create a new branch.

Request Body:
```json
{
  "branch": "feature/new-feature"
}
```

Response:
```json
{
  "branch": "feature/new-feature",
  "log": ["Branch created and checked out"]
}
```

### GET /api/repos
List all repositories in the configured base path.

Response:
```json
{
  "repos": [
    {
      "path": "/home/user/projects/repo1",
      "name": "repo1",
      "branch": "main",
      "isBare": false
    },
    {
      "path": "/home/user/projects/repo2",
      "name": "repo2",
      "branch": "develop",
      "isBare": false
    }
  ]
}
```

### POST /api/load-repo
Load a specific repository.

Request Body:
```json
{
  "path": "projects/my-repo"
}
```

Response:
```json
{
  "path": "projects/my-repo",
  "branch": "main",
  "log": ["Repository loaded"]
}
```

### POST /api/repo/create
Create a new repository.

Request Body:
```json
{
  "path": "projects/new-repo"
}
```

Response:
```json
{
  "path": "projects/new-repo",
  "log": ["Repository created"]
}
```

### POST /api/repo/init
Initialize a repository from an existing directory.

Request Body:
```json
{
  "path": "projects/existing-dir"
}
```

Response:
```json
{
  "path": "projects/existing-dir",
  "log": ["Repository initialized"]
}
```

### GET /api/remotes
Get all remotes in the current repository.

Response:
```json
{
  "remotes": [
    {
      "name": "origin",
      "url": "https://github.com/user/repo.git"
    }
  ]
}
```

### POST /api/remote/add
Add a new remote.

Request Body:
```json
{
  "name": "upstream",
  "url": "https://github.com/org/repo.git"
}
```

### POST /api/remote/update
Update a remote's URL.

Request Body:
```json
{
  "name": "origin",
  "url": "https://github.com/newowner/repo.git"
}
```

### POST /api/remote/remove
Remove a remote.

Request Body:
```json
{
  "name": "upstream"
}
```

### GET /api/tags
List all tags in the repository.

Query Parameters:
- `repoPath` (optional): Relative path to the repository

Response:
```json
{
  "tags": ["v1.0.0", "v1.0.1", "v1.1.0"]
}
```

### POST /api/tag/create
Create a new tag.

Request Body:
```json
{
  "tagName": "v1.0.0",
  "message": "Release version 1.0.0"
}
```

Response:
```json
{
  "commit": "v1.0.0",
  "log": ["$ git tag -a v1.0.0 -m \"Release version 1.0.0\"", "✓ Tag 'v1.0.0' created!"]
}
```

### POST /api/tag/push
Push tag(s) to a remote repository.

Query Parameters:
- `remote` (optional): Remote name to push to (default: `origin`)
- `repoPath` (optional): Relative path to the repository

Request Body:
```json
{
  "tagName": "v1.0.0",
  "all": false
}
```

Or to push all tags:
```json
{
  "all": true
}
```

Response:
```json
{
  "log": ["$ git push origin v1.0.0", "✓ Push successful!"]
}
```

### GET /api/commits
Get commit history for the current repository.

Query Parameters:
- `limit` (optional): Number of commits to return (default: `20`)
- `repoPath` (optional): Relative path to the repository

Response:
```json
{
  "commits": [
    {
      "hash": "abc123def456",
      "author": "John Doe <john@example.com>",
      "date": "2024-12-23 10:30:00",
      "message": "Fix: Update feature"
    },
    {
      "hash": "xyz789uvw123",
      "author": "Jane Smith <jane@example.com>",
      "date": "2024-12-22 15:45:00",
      "message": "Feature: Add new component"
    }
  ]
}
```

## Systemd Service Management

AirGit includes endpoints for registering and managing the application as a systemd user service.

### POST /api/systemd/register
Register AirGit as a systemd user service. This creates a service file at `~/.config/systemd/user/airgit.service` and enables it.

Response:
```json
{
  "success": true,
  "message": "Service registered and enabled"
}
```

### GET /api/systemd/status
Check if AirGit is registered as a systemd service.

Response:
```json
{
  "registered": true,
  "status": "enabled"
}
```

### POST /api/systemd/service-start
Start the AirGit systemd service.

Response:
```json
{
  "success": true,
  "message": "Service started"
}
```

### GET /api/systemd/service-status
Get the current status of the AirGit systemd service.

Response:
```json
{
  "active": true,
  "status": "running"
}
```

## How It Works

1. **Frontend** (HTML5 + Tailwind CSS): Mobile-first UI with bottom navigation bar and intuitive controls
2. **Backend** (Go):
   - Exposes comprehensive HTTP REST API (25+ endpoints)
   - Executes git commands locally
   - Manages multiple repositories in a base directory
   - Supports systemd service registration and management
   - Streams logs and results to frontend
   - Handles errors gracefully
3. **Systemd Integration**: Registers AirGit as a user service for continuous background operation

## Mobile Usage

1. Open AirGit in your phone's browser
2. Use the bottom navigation bar to navigate:
   - **Repos**: Browse and select repositories
   - **Branch**: Manage branches (list, create, checkout)
   - **Remotes**: Manage git remotes
   - **Log**: View commit history (past 20 commits)
   - **Settings**: Access settings and configuration
3. Tap **PUSH** or **PULL** buttons in the center to perform operations
4. Watch the spinner while operations execute
5. View operation logs for details

## Recommended Stack: Smartphone Vibe Coding Environment

For an optimal mobile development experience, we recommend the following stack:

### Architecture

**Tailscale + Vibe-Kanban + AirGit** provides a seamless smartphone-based development workflow:

```
Smartphone
    ↓ VPN (Tailscale)
    ↓
Development Server
    ├── Vibe-Kanban (Task Management & AI Assistant)
    └── AirGit (Git Operations)
```

### Components

1. **Tailscale** 🔐
   - Zero-config VPN connecting your smartphone securely to your development server
   - Works across different networks (home, office, mobile hotspot)
   - End-to-end encryption with no port forwarding needed
   - Install: [tailscale.com](https://tailscale.com)

2. **Vibe-Kanban** 📋
   - AI-powered task management and project planning
   - Kanban-style board for organizing development tasks
   - Integrated AI assistant for code suggestions and task breakdown
   - Lightweight web interface optimized for mobile

3. **AirGit** 💾
   - Browser-based Git operations (push, pull, branch management)
   - Mobile-first UI with bottom navigation
   - Direct repository access via REST API
   - PWA support for home screen installation

### Setup Steps

1. **Install Tailscale on both devices:**
   - Download from [tailscale.com](https://tailscale.com)
   - Authenticate with your preferred SSO provider
   - Note your server's Tailscale IP address

2. **Start Vibe-Kanban on your development server:**
   ```bash
   # On your server machine
   npm install
   npm run dev
   ```
   Access via: `http://<tailscale-ip>:3000`

3. **Start AirGit on your development server:**
   - Download the latest binary from [AirGit Releases](../../releases)
   - Choose the appropriate binary for your OS/architecture:
     - `airgit-linux-amd64` - Linux (x86_64)
     - `airgit-linux-arm64` - Linux (ARM64)
     - `airgit-darwin-amd64` - macOS (Intel)
     - `airgit-darwin-arm64` - macOS (Apple Silicon)
   - Make it executable and run:
   ```bash
   # On your server machine
   chmod +x airgit-linux-amd64
   ./airgit-linux-amd64 --listen-addr 0.0.0.0 --listen-port 8080
   ```
   Access via: `http://<tailscale-ip>:8080`

4. **Connect from your smartphone:**
   - Ensure Tailscale is running on your phone
   - Open browser and navigate to `http://<server-tailscale-ip>:3000` for Vibe-Kanban
   - Open browser and navigate to `http://<server-tailscale-ip>:8080` for AirGit
   - Install both as PWA apps (Add to Home Screen)

### Workflow Example

1. **Plan & Code**: Use Vibe-Kanban to break down tasks, generate code with AI, and review/commit changes
2. **Manage Git**: Use AirGit on your smartphone to:
   - Manage branches (create, switch, delete)
   - Push changes to remote repositories
   - Check repository status

### Benefits

- 🔒 **Secure**: Encrypted VPN connection via Tailscale (no public internet exposure)
- 📱 **Mobile-Native**: Both apps optimized for smartphone interaction
- 🔄 **Synchronized**: Real-time task and repository state across devices
- 🎯 **Focused**: Mobile UI reduces distractions from desktop complexity
- 💡 **AI-Assisted**: Leverage AI suggestions while on the move

### Notes

- All three components must be accessible on the same network (via Tailscale)
- Tailscale provides secure, encrypted connectivity without port forwarding
- Both Vibe-Kanban and AirGit support PWA installation for native-like mobile experience
- Recommended to run on a Linux/macOS development server with consistent uptime

## Add to Home Screen (iOS/Android)

1. Open AirGit in your browser
2. iOS: Tap Share → Add to Home Screen
3. Android: Tap ⋮ → Install app

The PWA manifest and service worker enable offline caching and home screen installation.

## GitHub AI Agent Integration

AirGit includes autonomous issue resolution capabilities. Users can trigger AI-powered agent processing directly from the AirGit UI.

### Quick Start - Trigger Agent

1. Open AirGit UI → **Issues** button in navigation bar
2. Agent will fetch GitHub issues from the current repository
3. Each issue shows an **🤖 Agent** button
4. Click to start processing that issue

### How It Works

1. **Issue Discovery**: AirGit detects GitHub remote and fetches open issues
2. **UI Display**: Issues listed with title, description, and Agent button
3. **Agent Trigger**: Click button to start processing (from AirGit UI)
4. **Processing**: Server-side execution:
   - Fetch latest from origin
   - Create feature branch (airgit/issue-{number})
   - Generate solution file
   - Commit and push changes
   - Create Pull Request via `gh cli`
5. **Review**: View and merge PR in GitHub

### Requirements

- GitHub repository with configured origin remote
- `gh` CLI installed and authenticated:
  ```bash
  gh auth login
  ```
- GITHUB_TOKEN environment variable (optional, for PR creation):
  ```bash
  export GITHUB_TOKEN=ghp_...
  ```

### Example Workflow

```
Click Agent Button
  └─ AirGit displays Issue details
  └─ Click 🤖 Agent
  └─ Server processes (git operations)
  └─ Feature branch created
  └─ Solution file generated
  └─ Commit pushed
  └─ PR created automatically
  └─ UI shows ✅ Done
```

### Architecture

```
AirGit UI (Browser)
    ↓ Click Agent Button
    ↓ POST /api/agent/process
AirGit Server (Go)
    ├─ Git operations (fetch, branch, commit, push)
    ├─ Solution generation
    └─ PR creation via gh CLI
    ↓
GitHub Repository
    ├─ Feature branch created
    ├─ Commits pushed
    └─ Pull Request opened
```

### Notes

- All processing happens on the server (not GitHub Actions)
- Works in isolated/firewall environments
- No external webhooks required
- Direct control from AirGit UI for better UX


## Architecture

```
Mobile Browser
    ↓ HTTP
AirGit Server (Go)
    ↓ Local File System & Systemd
Local Git Repositories & Systemd User Services
```

### Components

- **Frontend UI**: Bottom navigation bar with Repos, Branch, Remotes, Log, Settings buttons; central Push/Pull operation buttons
- **REST API**: 25+ endpoints for repository, git, and systemd operations
- **Git Executor**: Executes git commands locally on the filesystem
- **Repository Manager**: Handles multiple repositories within base directory
- **Systemd Integration**: Registers and manages AirGit as a systemd user service for background operation

## License

MIT

