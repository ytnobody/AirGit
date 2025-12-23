# AirGit

A lightweight web-based GUI tool for pushing Git changes from mobile devices via SSH.

## Features

- 📱 Mobile-optimized interface with single-tap push
- 🔒 SSH public key authentication
- 🚀 Standalone Go binary (no external dependencies)
- 📴 PWA support (offline caching, home screen icon)
- 🎨 Dark mode UI optimized for mobile
- ⚡ Real-time branch and server info display

## Quick Start

### Prerequisites

- Go 1.21+
- SSH access to remote server with Git repository
- SSH public key configured on remote server

### Installation

1. Clone or download the repository
2. Build the binary:

```bash
go build -o airgit
```

3. Set up environment variables:

```bash
export AIRGIT_SSH_HOST=your-server.com
export AIRGIT_SSH_USER=git
export AIRGIT_SSH_KEY=$HOME/.ssh/id_rsa
export AIRGIT_REPO_PATH=/var/git/my-repo
```

4. Run:

```bash
./airgit
```

5. Open http://localhost:8080 in your browser

## Configuration

Configure via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `AIRGIT_SSH_HOST` | `localhost` | SSH server hostname |
| `AIRGIT_SSH_PORT` | `22` | SSH server port |
| `AIRGIT_SSH_USER` | `git` | SSH username |
| `AIRGIT_SSH_KEY` | `~/.ssh/id_rsa` | Path to SSH private key |
| `AIRGIT_REPO_PATH` | `/var/git/repo` | Absolute path to Git repository on remote server |
| `AIRGIT_LISTEN_ADDR` | `0.0.0.0` | Server listen address |
| `AIRGIT_LISTEN_PORT` | `8080` | Server listen port |

## API Endpoints

### GET /api/status
Returns current branch name and server info.

Response:
```json
{
  "branch": "main",
  "server": "git@your-server.com"
}
```

### POST /api/push
Executes: `git add .` → `git commit -m "Updated via AirGit"` → `git push origin [branch]`

Response:
```json
{
  "branch": "main",
  "log": ["$ git add .", "$ git commit...", "..."]
}
```

## How It Works

1. **Frontend** (HTML5 + Tailwind CSS): Simple one-button mobile UI served via embed
2. **Backend** (Go): 
   - Exposes HTTP API
   - Connects to remote server via SSH
   - Executes git commands via SSH session
   - Streams/returns logs to frontend

## Mobile Usage

1. Open AirGit in your phone's browser
2. See current branch at the top
3. Tap the large **PUSH** button
4. Watch the spinner while it commits and pushes
5. See "Success!" confirmation

## Add to Home Screen (iOS/Android)

1. Open AirGit in your browser
2. iOS: Tap Share → Add to Home Screen
3. Android: Tap ⋮ → Install app

The PWA manifest and service worker enable offline caching and home screen installation.

## Architecture

```
Mobile Browser
    ↓ HTTP
  AirGit Server (Go)
    ↓ SSH
  Remote Git Repository
```

## License

MIT
