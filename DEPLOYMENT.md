# Deployment Guide

## Local Testing

### Prerequisites

- Go 1.21+
- SSH access to a test server
- Git repository on remote server

### Quick Start

```bash
# 1. Set environment variables
export AIRGIT_SSH_HOST=your-server.com
export AIRGIT_SSH_PORT=22
export AIRGIT_SSH_USER=git
export AIRGIT_SSH_KEY=$HOME/.ssh/id_rsa
export AIRGIT_REPO_PATH=/var/git/test-repo

# 2. Build and run
make build
./airgit

# 3. Test
curl http://localhost:8080/api/status
curl -X POST http://localhost:8080/api/push
```

## Docker Deployment

### Build Docker Image

```bash
docker build -t airgit:latest .
```

### Run in Docker

```bash
docker run -d \
  -p 8080:8080 \
  -e AIRGIT_SSH_HOST=your-server.com \
  -e AIRGIT_SSH_USER=git \
  -e AIRGIT_SSH_KEY=/root/.ssh/id_rsa \
  -e AIRGIT_REPO_PATH=/var/git/repo \
  -v ~/.ssh/id_rsa:/root/.ssh/id_rsa:ro \
  --name airgit \
  airgit:latest
```

## Kubernetes Deployment

### Create Secret for SSH Key

```bash
kubectl create secret generic airgit-ssh \
  --from-file=id_rsa=$HOME/.ssh/id_rsa \
  -n default
```

### Apply Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: airgit
spec:
  replicas: 1
  selector:
    matchLabels:
      app: airgit
  template:
    metadata:
      labels:
        app: airgit
    spec:
      containers:
      - name: airgit
        image: airgit:latest
        ports:
        - containerPort: 8080
        env:
        - name: AIRGIT_SSH_HOST
          value: "git.example.com"
        - name: AIRGIT_SSH_USER
          value: "git"
        - name: AIRGIT_SSH_KEY
          value: "/root/.ssh/id_rsa"
        - name: AIRGIT_REPO_PATH
          value: "/var/git/repo"
        volumeMounts:
        - name: ssh-key
          mountPath: /root/.ssh
          readOnly: true
      volumes:
      - name: ssh-key
        secret:
          secretName: airgit-ssh
          defaultMode: 0400
---
apiVersion: v1
kind: Service
metadata:
  name: airgit
spec:
  selector:
    app: airgit
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: LoadBalancer
```

## Systemd Service

### Create systemd unit file

```bash
sudo tee /etc/systemd/system/airgit.service << EOF
[Unit]
Description=AirGit - Mobile Git Push Tool
After=network.target

[Service]
Type=simple
User=airgit
WorkingDirectory=/opt/airgit
EnvironmentFile=/opt/airgit/.env
ExecStart=/opt/airgit/airgit
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
```

### Enable and start

```bash
sudo systemctl daemon-reload
sudo systemctl enable airgit
sudo systemctl start airgit
sudo systemctl status airgit
```

## Nginx Reverse Proxy

```nginx
server {
    listen 80;
    server_name airgit.example.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
        
        # Set appropriate headers for PWA
        add_header Cache-Control "public, max-age=3600";
        add_header Service-Worker-Allowed "/";
    }
}
```

Enable with Let's Encrypt:

```bash
sudo certbot --nginx -d airgit.example.com
```

## Security Considerations

1. **SSH Key Security**:
   - Use deploy keys (read-only) for production if possible
   - Restrict SSH key permissions (600)
   - Consider key rotation

2. **HTTPS**:
   - Always use HTTPS in production
   - Use Let's Encrypt or paid certificates

3. **Network Security**:
   - Use firewall rules to restrict access
   - Consider VPN/SSH tunneling for access
   - Use strong SSH authentication

4. **Host Key Verification**:
   - Current implementation disables host key verification
   - For production, enable and maintain `known_hosts`

5. **Rate Limiting**:
   - Consider adding rate limiting for push endpoint
   - Use reverse proxy (nginx) for this

## Monitoring

### Health Check

```bash
curl http://localhost:8080/api/status
```

### Logs

```bash
# If running with systemd
sudo journalctl -u airgit -f

# If running in Docker
docker logs -f airgit
```

### Metrics

Consider adding Prometheus metrics for:
- Push endpoint latency
- Success/failure counts
- SSH connection errors

## Troubleshooting

### SSH Connection Fails

```bash
# Test SSH manually
ssh -i ~/.ssh/id_rsa git@your-server.com "cd /var/git/repo && git status"

# Check SSH key permissions
ls -la ~/.ssh/id_rsa
chmod 600 ~/.ssh/id_rsa
```

### Git Commands Fail

```bash
# Check repository path exists
ssh -i ~/.ssh/id_rsa git@your-server.com "ls -la /var/git/repo"

# Test git command
ssh -i ~/.ssh/id_rsa git@your-server.com "cd /var/git/repo && git status"
```

### CORS Issues

If accessing from different domain:
- Add CORS headers to API responses
- Configure reverse proxy appropriately

### PWA Not Installing

- Ensure HTTPS is used
- Check manifest.json is valid
- Clear browser cache and service worker cache
