# Troubleshooting Guide

## Build Issues

### "go: command not found"
Install Go 1.21+:
- macOS: `brew install go`
- Linux: `apt install golang-go` or download from golang.org
- Windows: Download installer from golang.org

### "module golang.org/x/crypto not found"
Run:
```bash
go mod tidy
go build -o airgit
```

## Runtime Issues

### "SSH connection refused" / "Connection timeout"

```bash
# 1. Check SSH credentials
ssh -i $AIRGIT_SSH_KEY $AIRGIT_SSH_USER@$AIRGIT_SSH_HOST "echo OK"

# 2. Verify host and port
echo "Host: $AIRGIT_SSH_HOST Port: $AIRGIT_SSH_PORT"

# 3. Check if server is accessible
ping $AIRGIT_SSH_HOST
nslookup $AIRGIT_SSH_HOST

# 4. Try with verbose SSH
ssh -v -i $AIRGIT_SSH_KEY -p $AIRGIT_SSH_PORT $AIRGIT_SSH_USER@$AIRGIT_SSH_HOST
```

### "Permission denied (publickey)"

```bash
# 1. Check key permissions (should be 600)
ls -la ~/.ssh/id_rsa
chmod 600 ~/.ssh/id_rsa

# 2. Check key exists
test -f $AIRGIT_SSH_KEY && echo "Key found" || echo "Key not found"

# 3. Check if key is valid
ssh-keygen -l -f ~/.ssh/id_rsa

# 4. Verify pub key on remote
ssh $AIRGIT_SSH_USER@$AIRGIT_SSH_HOST "cat ~/.ssh/authorized_keys"
```

### "cd: /var/git/repo: No such file or directory"

```bash
# Verify repo path on remote server
ssh -i $AIRGIT_SSH_KEY $AIRGIT_SSH_USER@$AIRGIT_SSH_HOST \
  "ls -la /var/git/repo"

# Verify it's a git repo
ssh -i $AIRGIT_SSH_KEY $AIRGIT_SSH_USER@$AIRGIT_SSH_HOST \
  "cd /var/git/repo && git status"

# Check permissions
ssh -i $AIRGIT_SSH_KEY $AIRGIT_SSH_USER@$AIRGIT_SSH_HOST \
  "ls -la /var/git/"
```

## Git Operation Failures

### "git push: nothing to commit"
This is expected! The frontend handles this gracefully.
- Run `git status` to check what changed
- If nothing changed, push is skipped

### "git push: diverged branch"
The remote branch has commits you don't have:

```bash
# Pull first (manually or via different tool)
ssh -i $AIRGIT_SSH_KEY $AIRGIT_SSH_USER@$AIRGIT_SSH_HOST \
  "cd /var/git/repo && git pull origin main"

# Then try push again
```

### "git push: permission denied"
The SSH key doesn't have write permissions:

```bash
# Check which key is authorized
ssh -i $AIRGIT_SSH_KEY $AIRGIT_SSH_USER@$AIRGIT_SSH_HOST \
  "ssh-add -l"

# Verify key has push access
ssh -i $AIRGIT_SSH_KEY $AIRGIT_SSH_USER@$AIRGIT_SSH_HOST \
  "cd /var/git/repo && git push --dry-run origin main"
```

## Frontend Issues

### "Cannot GET /"
The HTML file isn't being served. Check:

```bash
# 1. Server is running
curl http://localhost:8080

# 2. Check logs
tail -f logs/airgit.log  # if logging to file

# 3. Verify port
netstat -tlnp | grep :8080
```

### "Status shows Error"

1. Check browser console (F12):
   - Are there network errors?
   - What's the actual error response?

2. Test API directly:
```bash
curl http://localhost:8080/api/status
curl -X POST http://localhost:8080/api/push
```

### "PWA not installing"

1. Must be HTTPS (except localhost)
2. Service worker must be at root (`/service-worker.js`)
3. Manifest must be valid JSON
4. Test manifest:
```bash
curl http://localhost:8080/manifest.json | jq .
```

### "Mobile button not responsive"

1. Check browser zoom (should be 100%)
2. Try different browser (Chrome, Safari, Firefox)
3. Check network tab for failed requests
4. Test on different phone brand

## Configuration Issues

### "AIRGIT_REPO_PATH not working"
Must be absolute path on remote server:

```bash
# ✅ Correct
export AIRGIT_REPO_PATH=/var/git/myrepo
export AIRGIT_REPO_PATH=/home/user/projects/myrepo

# ❌ Wrong
export AIRGIT_REPO_PATH=~/myrepo           # Don't use ~
export AIRGIT_REPO_PATH=./myrepo           # Don't use relative
export AIRGIT_REPO_PATH=$HOME/myrepo       # $HOME won't expand over SSH
```

### "Port already in use"

```bash
# Find what's using the port
lsof -i :8080
netstat -tlnp | grep :8080

# Use different port
export AIRGIT_LISTEN_PORT=9000
./airgit
```

### ".env file not being read"

Environment variables must be set before running:

```bash
# Option 1: Source .env (manual)
export $(cat .env | grep -v '^#')
./airgit

# Option 2: Inline
AIRGIT_SSH_HOST=server.com ./airgit

# Option 3: Systemd
# Use EnvironmentFile= in service file

# Option 4: Docker
# Use -e flags or --env-file
```

## Logging & Debugging

### Enable verbose logging

Add to main.go and rebuild:
```go
log.SetFlags(log.Ldate | log.Ltime | log.Llongfile)
log.SetOutput(os.Stdout)
```

### Test git command directly

```bash
# Run the exact command AirGit runs
ssh -i $AIRGIT_SSH_KEY -p $AIRGIT_SSH_PORT \
  $AIRGIT_SSH_USER@$AIRGIT_SSH_HOST \
  "cd $AIRGIT_REPO_PATH && git add . && git commit -m 'test' && git push"
```

### Check SSH key format

```bash
# Should be OpenSSH format
file ~/.ssh/id_rsa

# Convert if needed (from PuTTY format)
ssh-keygen -p -N "" -m pem -f ~/.ssh/id_rsa
```

## Performance Issues

### Slow push operations

1. Check network latency:
```bash
ping -c 4 $AIRGIT_SSH_HOST
```

2. Check SSH connection speed:
```bash
time ssh -i $AIRGIT_SSH_KEY $AIRGIT_SSH_USER@$AIRGIT_SSH_HOST "echo OK"
```

3. Check repository size:
```bash
ssh -i $AIRGIT_SSH_KEY $AIRGIT_SSH_USER@$AIRGIT_SSH_HOST \
  "du -sh /var/git/repo"
```

4. Check remote server load:
```bash
ssh -i $AIRGIT_SSH_KEY $AIRGIT_SSH_USER@$AIRGIT_SSH_HOST \
  "uptime && df -h"
```

## Getting Help

### Provide debug info

When asking for help, include:

```bash
# 1. AirGit version/info
./airgit -v  # if implemented

# 2. Go version
go version

# 3. Environment
env | grep AIRGIT

# 4. Test SSH
ssh -i $AIRGIT_SSH_KEY -p $AIRGIT_SSH_PORT \
  $AIRGIT_SSH_USER@$AIRGIT_SSH_HOST \
  "cd $AIRGIT_REPO_PATH && git status"

# 5. API test
curl -v http://localhost:8080/api/status
curl -v -X POST http://localhost:8080/api/push
```

## Common Solutions

| Problem | Solution |
|---------|----------|
| SSH key not found | Check AIRGIT_SSH_KEY path, verify with `test -f` |
| Connection timeout | Verify host/port, check firewall, test with ssh command |
| Permission denied | Fix SSH key permissions to 600, check authorized_keys |
| Port in use | Change AIRGIT_LISTEN_PORT or kill conflicting process |
| Nothing to commit | Normal - run `git status` to see actual changes |
| Push fails | Check remote branch exists, pull if diverged |
| PWA won't install | Must be HTTPS, clear cache, check manifest.json |
| Slow response | Check network latency, server load, repo size |
