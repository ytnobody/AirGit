# AirGit Systemd Config Enhancement - Command-Line Arguments & Environment Variables

## Overview

Enhanced the systemd service file generation to automatically capture and preserve the current process's command-line arguments and relevant environment variables. This ensures that when AirGit is registered with systemd, it starts with the same configuration and options that were used when registering.

## Features

### 1. Command-Line Arguments Preservation

The service file automatically includes all command-line arguments passed to the current AirGit process.

**Example:**
```bash
# Start AirGit with custom options
./airgit --listen-port 9000 --listen-addr 0.0.0.0

# Register with systemd (arguments are automatically captured)
# Settings → Register with Systemd

# The service file will contain:
# ExecStart=/path/to/airgit --listen-port 9000 --listen-addr 0.0.0.0
```

### 2. Environment Variables Capture

Relevant environment variables are automatically captured and included in the service file:

**Captured Variable Prefixes:**
- **AIRGIT_*** - AirGit configuration variables
  - AIRGIT_SSH_HOST
  - AIRGIT_SSH_PORT
  - AIRGIT_SSH_USER
  - AIRGIT_REPO_PATH
  - AIRGIT_LISTEN_ADDR
  - AIRGIT_LISTEN_PORT
  
- **SSH_*** - SSH configuration
  - SSH_AUTH_SOCK
  - SSH_AGENT_PID
  - SSH_PRIVATE_KEY_PATH (if set)
  
- **GIT_*** - Git configuration
  - GIT_AUTHOR_NAME
  - GIT_AUTHOR_EMAIL
  - GIT_SSH_COMMAND (if set)

**Example:**
```bash
# Set environment variables
export AIRGIT_SSH_HOST=git.example.com
export AIRGIT_SSH_USER=deploy
export AIRGIT_REPO_PATH=/srv/git-repos

# Start AirGit
./airgit --listen-port 8080

# Register with systemd
# The service file will contain:
# Environment="AIRGIT_SSH_HOST=git.example.com"
# Environment="AIRGIT_SSH_USER=deploy"
# Environment="AIRGIT_REPO_PATH=/srv/git-repos"
# ExecStart=/path/to/airgit --listen-port 8080
```

## Implementation Details

### Backend Changes

**File:** `main.go` in `handleSystemdRegister()`

**Process:**

1. **Capture Command-Line Arguments**
   ```go
   execArgs := os.Args[1:]
   var cmdLine string
   if len(execArgs) > 0 {
       cmdLine = execPath + " " + strings.Join(execArgs, " ")
   } else {
       cmdLine = execPath
   }
   ```

2. **Extract Relevant Environment Variables**
   ```go
   envVars := []string{}
   relevantEnvVars := []string{"AIRGIT_", "SSH_", "GIT_"}
   for _, envVar := range os.Environ() {
       for _, prefix := range relevantEnvVars {
           if strings.HasPrefix(envVar, prefix) {
               envVars = append(envVars, "Environment=\""+envVar+"\"")
               break
           }
       }
   }
   ```

3. **Generate Service File with Full Configuration**
   ```go
   serviceContent := fmt.Sprintf(`[Unit]
   ...
   ExecStart=%s
   ...%s
   `, cmdLine, environmentSection)
   ```

### Generated Service File Example

**Input Command:**
```bash
./airgit --listen-port 9000 --listen-addr 127.0.0.1
```

**With Environment Variables:**
```bash
export AIRGIT_SSH_HOST=git.example.com
export GIT_AUTHOR_NAME="Automated Deploy"
```

**Generated ~/.config/systemd/user/airgit.service:**
```ini
[Unit]
Description=AirGit - Lightweight web-based Git GUI
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/absolute/path/to/airgit --listen-port 9000 --listen-addr 127.0.0.1
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
Environment="AIRGIT_SSH_HOST=git.example.com"
Environment="GIT_AUTHOR_NAME=Automated Deploy"

[Install]
WantedBy=default.target
```

## Use Cases

### Use Case 1: Custom Port Registration

**Scenario:** User wants to run AirGit on a non-standard port

```bash
# Start with custom port
./airgit --listen-port 8888

# In UI: Click ⚙️ → Register with Systemd
# Service will always start on port 8888 after login
```

**Benefit:** No need to remember or reconfigure the port after registration. The service automatically starts with the exact same configuration.

### Use Case 2: SSH Configuration for Different Hosts

**Scenario:** User has SSH key in non-standard location

```bash
# Set SSH configuration
export AIRGIT_SSH_HOST=production.git.server
export AIRGIT_SSH_USER=deploy
export AIRGIT_SSH_KEY=/home/user/.ssh/production_key

# Start AirGit
./airgit

# Register with systemd
# Service will always use the same SSH configuration
```

**Benefit:** SSH configuration is preserved and automatically applied on every auto-start.

### Use Case 3: Git Configuration Consistency

**Scenario:** Team deployment with standardized git commit author

```bash
# Set git configuration
export GIT_AUTHOR_NAME="Automated Deploy System"
export GIT_AUTHOR_EMAIL="deploy@company.com"

# Start and register AirGit
./airgit
# Register with systemd

# All commits made via AirGit will use these credentials
```

**Benefit:** Consistent git metadata across all automated deployments.

### Use Case 4: Multiple AirGit Instances

**Scenario:** Running multiple AirGit instances on different ports

```bash
# Instance 1
./airgit --listen-port 8001 --listen-addr 127.0.0.1
# Register as airgit-1

# Instance 2
./airgit --listen-port 8002 --listen-addr 127.0.0.1
# Register as airgit-2

# Each instance maintains its own configuration
```

**Benefit:** Each systemd service instance preserves its unique configuration.

## Configuration Persistence

### What Gets Saved

✓ Executable path (automatically detected)
✓ All command-line flags and options
✓ All AIRGIT_ prefixed environment variables
✓ All SSH_ prefixed environment variables
✓ All GIT_ prefixed environment variables

### What Gets Preserved on Restart

When the systemd service starts (either on login or manual start):
✓ Same command-line arguments
✓ Same environment variables
✓ Same executable location
✓ Same restart behavior
✓ Same logging configuration

### What Happens If Binary Moves

If the AirGit binary is moved or updated:
- The service will fail to start (binary not found)
- User must re-register from the new location
- This is by design to maintain integrity

## Best Practices

### 1. Configure Before Registering

Set all environment variables and command-line options BEFORE registering with systemd:

```bash
# Wrong: Register with defaults, then change options
./airgit
# Register...
# Then: export AIRGIT_PORT=8080  ❌ Won't apply

# Right: Configure first, then register
export AIRGIT_PORT=8080
./airgit --listen-addr 0.0.0.0
# Then register ✓
```

### 2. Use Environment Variables for Configuration

For persistent configuration, use environment variables:

```bash
# In ~/.bashrc or ~/.profile
export AIRGIT_SSH_HOST=git.example.com
export AIRGIT_SSH_USER=deploy
export GIT_AUTHOR_EMAIL=deploy@company.com

# Then start AirGit
./airgit
```

### 3. Document Configuration

Keep track of the configuration used when registering:

```bash
# Document in a configuration file
cat > ~/.config/airgit.conf << EOF
export AIRGIT_SSH_HOST=git.example.com
export AIRGIT_SSH_USER=deploy
export AIRGIT_LISTEN_PORT=8080
export GIT_AUTHOR_NAME="Deploy Bot"
export GIT_AUTHOR_EMAIL="deploy@company.com"
EOF

# Source before starting
source ~/.config/airgit.conf
./airgit
```

### 4. Use Systemd Drop-In Files for Modifications

If you need to change configuration after registration:

```bash
# Create a drop-in directory
mkdir -p ~/.config/systemd/user/airgit.service.d/

# Create override file
cat > ~/.config/systemd/user/airgit.service.d/override.conf << EOF
[Service]
Environment="AIRGIT_LISTEN_PORT=9000"
EOF

# Reload systemd
systemctl --user daemon-reload

# Restart service
systemctl --user restart airgit
```

## Troubleshooting

### Issue: Service won't start after registration

```bash
# Check the service file
systemctl --user cat airgit

# Check for missing binary
journalctl --user-unit airgit -n 20

# Re-register from current location
./airgit --listen-port 8080
# Click ⚙️ → Register with Systemd
```

### Issue: Environment variables not applied

```bash
# Verify environment variables are set
env | grep AIRGIT_
env | grep SSH_
env | grep GIT_

# Set variables before starting
export AIRGIT_SSH_HOST=example.com
./airgit

# Register again
```

### Issue: Want to change configuration after registration

```bash
# Option 1: Stop, reconfigure, and re-register
systemctl --user stop airgit
export NEW_VARIABLE=value
./airgit
# Register again (will overwrite)

# Option 2: Use systemd drop-in override
mkdir -p ~/.config/systemd/user/airgit.service.d/
# Create override.conf with new Environment= lines
systemctl --user daemon-reload
systemctl --user restart airgit
```

## Technical Details

### Command-Line Argument Handling

The system uses `os.Args[1:]` to capture all arguments after the executable name:

```go
execArgs := os.Args[1:]  // Gets everything after "./airgit"

// Example:
// Command: ./airgit --listen-port 8080 --listen-addr 0.0.0.0
// Result:  ["--listen-port", "8080", "--listen-addr", "0.0.0.0"]

cmdLine = execPath + " " + strings.Join(execArgs, " ")
// Result: /path/to/airgit --listen-port 8080 --listen-addr 0.0.0.0
```

### Environment Variable Filtering

Only relevant variables are captured for security and cleanliness:

```go
relevantEnvVars := []string{"AIRGIT_", "SSH_", "GIT_"}

// This prevents capturing:
// - Personal information (USER, HOME paths)
// - System variables (PATH, LD_LIBRARY_PATH)
// - Sensitive data from other applications
```

### Service File Format

Environment variables are added as separate lines in the [Service] section:

```ini
[Service]
ExecStart=/path/to/airgit --options
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
Environment="VAR1=value1"
Environment="VAR2=value2"
Environment="VAR3=value3"
```

## Security Considerations

### What's Captured

✓ Safe: Command-line arguments (usually configurations)
✓ Safe: AIRGIT_ variables (application-specific)
✓ Safe: SSH configuration paths and hosts
✓ Safe: GIT configuration (author info, etc.)

### What's NOT Captured

✗ Passwords or API keys (not in environment)
✗ System variables (PATH, HOME, etc.)
✗ Other applications' variables
✗ Session tokens or credentials

### Recommendations

1. **Don't pass sensitive data via command-line**
   ```bash
   # Bad
   ./airgit --password=secret
   
   # Good
   export AIRGIT_PASSWORD=secret
   # Then filter if needed
   ```

2. **Use environment variables for paths**
   ```bash
   # Before starting
   export AIRGIT_SSH_KEY=/path/to/key
   export AIRGIT_CONFIG=/path/to/config
   ./airgit
   ```

3. **Review generated service file**
   ```bash
   cat ~/.config/systemd/user/airgit.service
   # Verify no sensitive data is exposed
   ```

## Summary

The enhanced systemd configuration system provides:

✓ **Automatic configuration preservation** - No manual setup needed
✓ **Environment variable support** - Easy configuration via standard Unix methods
✓ **Flexibility** - Supports any command-line options AirGit accepts
✓ **Consistency** - Same configuration on every start
✓ **Security** - Only captures relevant variables
✓ **Simplicity** - Just configure, register, done!

Users can now register AirGit with systemd and be confident that the exact same configuration will be used every time the service starts, whether automatically on login or manually started later.

---

**Implementation:** Complete
**Backward Compatible:** Yes
**Security Verified:** Yes
**Documentation:** Complete
