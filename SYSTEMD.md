# AirGit User-Mode Systemd Registration

## Overview

This document describes the user-mode systemd service registration feature for AirGit. This feature allows AirGit to be automatically started when the user logs in to their Linux system.

## Features

### 1. Automatic Service Registration
- Create `~/.config/systemd/user/airgit.service` automatically
- Detect and use the current executable path
- Enable auto-start on user login
- Disable registration if already registered

### 2. Status Check
- Query the current registration status
- Returns boolean indicating if service is registered

### 3. Error Handling
- Prevent duplicate registration
- Provide clear error messages
- Handle systemd command failures gracefully

## API Endpoints

### GET /api/systemd/status

Check if AirGit is registered as a systemd service.

**Request:**
```bash
curl http://localhost:8080/api/systemd/status
```

**Response (Registered):**
```json
{
  "registered": true
}
```

**Response (Not Registered):**
```json
{
  "registered": false
}
```

---

### POST /api/systemd/register

Register AirGit as a user-mode systemd service.

**Request:**
```bash
curl -X POST http://localhost:8080/api/systemd/register
```

**Success Response:**
```json
{
  "success": true,
  "message": "Service registered and enabled successfully",
  "path": "/home/user/.config/systemd/user/airgit.service"
}
```

**Error Response (Already Registered):**
```json
{
  "success": false,
  "error": "Service is already registered with systemd"
}
```

**HTTP Status Code:** 409 (Conflict)

---

**Error Response (Other Errors):**
```json
{
  "success": false,
  "error": "Failed to reload systemd daemon: <error message>"
}
```

**HTTP Status Code:** 500 (Internal Server Error)

---

## Service File Details

When registered, AirGit creates a systemd service file at:
```
~/.config/systemd/user/airgit.service
```

### Automatic Command-Line Arguments Preservation

The service file automatically captures and includes:

**1. Executable Path**
The absolute path to the current AirGit binary

**2. Command-Line Arguments**
Any arguments passed when registering (e.g., `--listen-port 8080`)

**3. Environment Variables**
Relevant environment variables are automatically included:
- All `AIRGIT_*` variables (AirGit configuration)
- All `SSH_*` variables (SSH configuration)
- All `GIT_*` variables (Git configuration)

### Example Service File

If you run: `./airgit --listen-port 9000` with `AIRGIT_SSH_KEY=/path/to/key`

The generated service file will be:
```ini
[Unit]
Description=AirGit - Lightweight web-based Git GUI
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/path/to/airgit --listen-port 9000
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
Environment="AIRGIT_SSH_KEY=/path/to/key"

[Install]
WantedBy=default.target
```

### Configuration Details

- **Type=simple**: AirGit runs as a simple daemon
- **ExecStart**: Includes full command with arguments
- **Environment**: Captures AIRGIT_, SSH_, and GIT_ variables
- **Restart=on-failure**: Service restarts if it fails
- **RestartSec=5**: Wait 5 seconds before restarting
- **StandardOutput/StandardError=journal**: Logs go to journald
- **WantedBy=default.target**: Starts when user logs in

### Supported Environment Variables

The service file automatically captures these prefixes:
- **AIRGIT_\*** - AirGit configuration variables
  - AIRGIT_SSH_HOST, AIRGIT_SSH_PORT, AIRGIT_SSH_USER, etc.
  - AIRGIT_REPO_PATH, AIRGIT_LISTEN_ADDR, AIRGIT_LISTEN_PORT
  
- **SSH_\*** - SSH configuration
  - SSH_AUTH_SOCK, SSH_AGENT_PID, etc.
  
- **GIT_\*** - Git configuration
  - GIT_AUTHOR_NAME, GIT_AUTHOR_EMAIL, etc.

## Managing the Service

Once registered, you can manage the service with standard systemctl commands:

### Check Service Status
```bash
systemctl --user status airgit
```

### Start the Service
```bash
systemctl --user start airgit
```

### Stop the Service
```bash
systemctl --user stop airgit
```

### Restart the Service
```bash
systemctl --user restart airgit
```

### Enable Auto-Start (Already done by registration)
```bash
systemctl --user enable airgit
```

### Disable Auto-Start
```bash
systemctl --user disable airgit
```

### View Logs
```bash
journalctl --user-unit airgit -f
```

### View Service File
```bash
systemctl --user cat airgit
```

## Requirements

- Linux system with systemd user units support
- User-mode systemd enabled (default on most modern Linux distributions)
- `systemctl` command available in PATH

## Implementation Details

### `handleSystemdStatus(w http.ResponseWriter, r *http.Request)`

**Functionality:**
- Checks if `~/.config/systemd/user/airgit.service` exists
- Returns registration status as JSON

**Flow:**
1. Call `isSystemdServiceRegistered()`
2. Encode result as JSON
3. Send HTTP 200 response

---

### `handleSystemdRegister(w http.ResponseWriter, r *http.Request)`

**Functionality:**
- Registers AirGit as a systemd service
- Creates service file
- Reloads systemd daemon
- Enables auto-start

**Flow:**
1. Check if already registered (return 409 if yes)
2. Get current executable path via `os.Executable()`
3. Get home directory via `os.UserHomeDir()`
4. Create `~/.config/systemd/user/` directory
5. Generate service file content
6. Write service file to disk
7. Execute `systemctl --user daemon-reload`
8. Execute `systemctl --user enable airgit.service`
9. Return success response with file path

**Error Handling:**
- Returns 409 Conflict if already registered
- Returns 500 with error message for any failures
- Provides detailed error messages

---

### `isSystemdServiceRegistered() bool`

**Functionality:**
- Helper function to check service registration status
- Used by both status and register endpoints

**Implementation:**
1. Get home directory
2. Construct path to `~/.config/systemd/user/airgit.service`
3. Check if file exists using `os.Stat()`
4. Return true if exists, false otherwise

---

## Example Workflow

### Step 1: Start AirGit
```bash
./airgit --listen-port 8080
```

### Step 2: Check Registration Status
```bash
curl http://localhost:8080/api/systemd/status
# Response: {"registered": false}
```

### Step 3: Register as Systemd Service
```bash
curl -X POST http://localhost:8080/api/systemd/register
# Response: {
#   "success": true,
#   "message": "Service registered and enabled successfully",
#   "path": "/home/user/.config/systemd/user/airgit.service"
# }
```

### Step 4: Verify Registration
```bash
systemctl --user status airgit
# Active: active (running) since ...
```

### Step 5: Reboot and Verify Auto-Start
```bash
reboot
# After reboot, AirGit should be running automatically
curl http://localhost:8080/api/systemd/status
```

## Security Considerations

1. **Executable Path**: The service file stores the absolute path to the AirGit executable. If you move the binary, you need to re-register.

2. **User Isolation**: The service runs as the current user only. It will not be available for other users on the system.

3. **Configuration**: Ensure that environment variables (`.env`) or configuration files are properly set up before registration.

4. **Logs**: Check logs with `journalctl --user-unit airgit` for debugging.

## Troubleshooting

### Service doesn't auto-start after login
```bash
# Check if service is enabled
systemctl --user is-enabled airgit
# Should output: enabled

# Check if user session is running
systemctl --user is-system-running
# Should output: running
```

### Service fails to start
```bash
# Check logs
journalctl --user-unit airgit -n 50

# Check service status
systemctl --user status airgit

# Manually start and check error
systemctl --user start airgit
```

### Permission Denied when accessing service file
```bash
# Check permissions
ls -la ~/.config/systemd/user/airgit.service

# Should be readable by current user
chmod 644 ~/.config/systemd/user/airgit.service
```

### Systemd changes not taking effect
```bash
# Reload systemd configuration
systemctl --user daemon-reload

# Then restart service
systemctl --user restart airgit
```

## Notes

- This feature only works on Linux systems with systemd and user-mode systemd support
- The executable path is determined at registration time. Moving the binary requires re-registration
- Environment variables should be set in `~/.config/systemd/user/airgit.service.d/override.conf` or `.env` file in the executable directory
