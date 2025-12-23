# Quick Start Guide

## 5-Minute Setup

### Option 1: Using Interactive Setup Script

```bash
# 1. Run setup script
./setup.sh

# Follow the prompts to enter:
# - SSH host and port
# - SSH user and key path
# - Remote repository absolute path
# - Server listen address/port

# 2. Build
make build

# 3. Run
./airgit

# 4. Open browser
# http://localhost:8080
```

### Option 2: Manual Configuration with Environment Variables

```bash
# 1. Set environment variables (in your shell or .env file)
export AIRGIT_SSH_HOST=your-server.com
export AIRGIT_SSH_USER=git
export AIRGIT_SSH_KEY=$HOME/.ssh/id_rsa
export AIRGIT_REPO_PATH=/var/git/my-repo
export AIRGIT_LISTEN_PORT=8080

# 2. Build the binary
make build

# 3. Run
./airgit

# 4. Open in browser or mobile
# http://localhost:8080
```

### Option 3: Using Command-Line Flags

```bash
# 1. Build the binary
make build

# 2. Run with command-line flags
./airgit --ssh-host your-server.com --ssh-user git --repo-path /var/git/my-repo

# 3. Open in browser or mobile
# http://localhost:8080
```

## Help and Version

Show all available command-line options:

```bash
./airgit --help
# or
./airgit -h
```

Check the version:

```bash
./airgit --version
# or
./airgit -v
```

## Using With Different SSH Keys

```bash
# Use a specific SSH key (e.g., deploy key)
export AIRGIT_SSH_KEY=/path/to/deploy-key
./airgit
```

Or with flags:

```bash
./airgit --ssh-key /path/to/deploy-key
```

## Testing the API

### Check Status

```bash
curl http://localhost:8080/api/status
# Response:
# {
#   "branch": "main",
#   "server": "git@your-server.com"
# }
```

### Push Changes

```bash
curl -X POST http://localhost:8080/api/push
# Response:
# {
#   "branch": "main",
#   "log": [
#     "$ git add .",
#     "$ git commit...",
#     "..."
#   ]
# }
```

## On Mobile

1. **Open AirGit in browser**:
   - From computer: http://YOUR_IP:8080
   - Add to home screen for PWA experience

2. **See status**:
   - Branch name shown at top
   - Server info displayed
   - Green PUSH button in center

3. **Push changes**:
   - Tap the PUSH button
   - Watch the spinner while it works
   - See "Success!" when done

## Troubleshooting

### "Connection failed"

Check your SSH configuration:
```bash
# Test SSH manually
ssh -i $AIRGIT_SSH_KEY $AIRGIT_SSH_USER@$AIRGIT_SSH_HOST \
  "cd $AIRGIT_REPO_PATH && git status"
```

### "git push failed"

Check the logs in the browser - common issues:
- No commits to push
- Diverged branches (need pull first)
- No write permissions to remote
- SSH key doesn't have permission

### Port already in use

```bash
# Use a different port
export AIRGIT_LISTEN_PORT=9000
./airgit
```

### SSH key permission denied

```bash
# Fix SSH key permissions
chmod 600 ~/.ssh/id_rsa
chmod 700 ~/.ssh
```

## Production Deployment

See [DEPLOYMENT.md](DEPLOYMENT.md) for:
- Docker setup
- Kubernetes deployment
- Nginx reverse proxy
- Systemd service
- Security best practices
