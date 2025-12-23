# AirGit Systemd Service Start Feature

## Overview

This document describes the service start functionality that allows users to manually start the AirGit systemd service from the frontend UI if it's not running.

## Features

### 1. Service Status Checking
- Check if AirGit is registered with systemd
- Check if the registered service is currently running
- Display status in the settings modal

### 2. Service Start
- Start the systemd service if it's registered but stopped
- Provide clear feedback on success or failure
- Prevent starting if already running
- Prevent starting if not registered

## API Endpoints

### GET /api/systemd/service-status

**Purpose:** Check the registration and running status of the AirGit service

**Request:**
```bash
curl http://localhost:8080/api/systemd/service-status
```

**Response (Running):**
```json
{
  "registered": true,
  "running": true
}
```

**Response (Stopped):**
```json
{
  "registered": true,
  "running": false
}
```

**Response (Not Registered):**
```json
{
  "registered": false,
  "running": false
}
```

**Status Code:** 200 OK (always)

---

### POST /api/systemd/service-start

**Purpose:** Start the AirGit systemd service

**Request:**
```bash
curl -X POST http://localhost:8080/api/systemd/service-start
```

**Success Response (HTTP 200):**
```json
{
  "success": true,
  "message": "Service started successfully"
}
```

**Error Response (Not Registered - HTTP 400):**
```json
{
  "success": false,
  "error": "Service is not registered with systemd"
}
```

**Error Response (Already Running - HTTP 409):**
```json
{
  "success": false,
  "error": "Service is already running"
}
```

**Error Response (Start Failed - HTTP 500):**
```json
{
  "success": false,
  "error": "Failed to start service: [error details]"
}
```

## Frontend UI

### Service Status Section

A new section appears in the Settings modal showing:

**Component: Service Status Badge**
- **Registered & Running:** Green badge with ✓ symbol
- **Registered but Stopped:** Yellow badge
- **Not Registered:** Gray badge
- **Error:** Red badge

**Component: Service Status Message**
- Provides context about the current state
- Suggests action if needed

**Component: Start Button**
- **Running:** Disabled (gray), text says "Already Running"
- **Stopped:** Enabled (blue), text says "Start Service"
- **Not Registered:** Disabled (gray), text says "Register First"
- **Error:** Shows error details

## User Workflows

### Workflow 1: Service Stopped, User Wants to Start

1. User clicks ⚙️ settings button
2. Settings modal opens
3. Service status loads via API
4. Service section shows:
   - Yellow "Stopped" badge
   - Message: "Service is registered but not running. Click below to start."
   - Blue "Start Service" button (enabled)
5. User clicks "Start Service"
6. Loading spinner appears
7. Button is disabled
8. System starts the service via `systemctl --user start airgit`
9. After success:
   - Badge turns green with ✓ symbol
   - Message: "✓ Service started successfully!"
   - Button becomes gray and disabled
   - Button text: "Already Running"
10. User closes modal

### Workflow 2: Service Already Running

1. User clicks ⚙️ settings button
2. Settings modal opens
3. Service status loads via API
4. Service section shows:
   - Green "✓ Running" badge
   - Message: "Service is running via systemd."
   - Gray "Already Running" button (disabled)
5. User sees service is running
6. User closes modal

### Workflow 3: Service Not Registered

1. User clicks ⚙️ settings button
2. Settings modal opens
3. Service status loads via API
4. Service section shows:
   - Gray "Not Registered" badge
   - Message: "Service must be registered first."
   - Gray "Register First" button (disabled)
5. User cannot start unregistered service
6. User must register first in the Systemd Registration section

### Workflow 4: Error Starting Service

1. User clicks "Start Service" button
2. Error occurs (network, permissions, systemd issues)
3. Badge turns red with "Error"
4. Message shows error details
5. Button remains enabled
6. User can click again to retry

## Implementation Details

### Backend: handleSystemdServiceStatus()

**Function Signature:**
```go
func handleSystemdServiceStatus(w http.ResponseWriter, r *http.Request)
```

**Flow:**
1. Check if service file exists (if not, registered = false)
2. If registered, execute: `systemctl --user is-active airgit`
3. If command succeeds, running = true
4. If command fails, running = false
5. Return JSON with both values

**Error Handling:**
- Safe error handling throughout
- Returns sensible defaults if error occurs
- No panics or crashes

---

### Backend: handleSystemdServiceStart()

**Function Signature:**
```go
func handleSystemdServiceStart(w http.ResponseWriter, r *http.Request)
```

**Flow:**
1. Check HTTP method (POST only)
2. Check if service is registered (400 if not)
3. Check if service is already running (409 if yes)
4. Execute: `systemctl --user start airgit`
5. Return success or error response

**Error Handling:**
- 400 Bad Request: Service not registered
- 409 Conflict: Service already running
- 500 Internal Server Error: Start failed
- All errors provide descriptive messages

---

### Frontend: loadServiceStatus()

**Purpose:** Load and display service status

**Flow:**
1. Fetch `/api/systemd/service-status`
2. Parse response
3. Update UI based on status:
   - Not registered: Disable button, show register message
   - Running: Disable button, show running message
   - Stopped: Enable button, show start message
4. Update badge color and text
5. Handle errors gracefully

---

### Frontend: Service Start Button Handler

**Purpose:** Handle user clicking start button

**Flow:**
1. Check if registered first (via loadServiceStatus)
2. Disable button and show loading spinner
3. POST to `/api/systemd/service-start`
4. Handle response:
   - Success (200): Update to running state
   - Conflict (409): Already running, update UI
   - Error (400/500): Show error, allow retry
5. Update badge, message, and button state

## Status States

### Not Registered
```
Badge:  Gray "Not Registered"
Message: "Service must be registered first."
Button: Gray, disabled, "Register First"
Action: User must register in systemd section
```

### Registered & Running
```
Badge:  Green "✓ Running"
Message: "Service is running via systemd."
Button: Gray, disabled, "Already Running"
Action: None needed
```

### Registered & Stopped
```
Badge:  Yellow "Stopped"
Message: "Service is registered but not running. Click below to start."
Button: Blue, enabled, "Start Service"
Action: User can click to start
```

### Starting (Loading)
```
Badge:  Yellow "Stopped" (unchanged)
Message: (unchanged)
Button: Gray, disabled, spinner visible
Action: None (button disabled)
```

### Start Failed
```
Badge:  Red "Error"
Message: "✗ Error: [error details]"
Button: Blue, enabled, "Start Service"
Action: User can retry
```

## Color Scheme

### Badge Colors
- **Running:** Emerald green with ✓ symbol
- **Stopped:** Yellow/amber
- **Not Registered:** Gray
- **Error:** Red

### Button Colors
- **Enabled:** Blue (blue-600, hover: blue-500)
- **Disabled:** Gray (gray-600, 50% opacity)

## Integration with Systemd Registration

The service start feature works in conjunction with the registration feature:

```
User Flow:
1. If not registered:
   - Register via "Register with Systemd" button
2. Then either:
   - Wait for auto-start on next login
   - Immediately start via "Start Service" button
```

## Use Cases

### Use Case 1: Manual Service Start
User registers AirGit but wants to use it immediately without rebooting:
1. Click ⚙️ settings
2. Register via "Register with Systemd"
3. Click "Start Service" to start immediately
4. Use AirGit right away

### Use Case 2: Service Stopped Unexpectedly
Service was running but crashed or was stopped:
1. Click ⚙️ settings
2. See service is stopped (yellow badge)
3. Click "Start Service"
4. Service restarts

### Use Case 3: Verify Service Status
User wants to check if service is running:
1. Click ⚙️ settings
2. Look at service status badge
3. See if green (running) or yellow (stopped)

## Error Handling

### Network Error
- **Display:** Red error badge with network error message
- **Button:** Remains enabled for retry
- **Recovery:** User can click again to retry

### Service Not Registered
- **Display:** Gray badge with "Register First" message
- **Button:** Disabled, text "Register First"
- **Recovery:** User must register service first

### Service Already Running
- **Display:** Green badge with running status
- **Button:** Disabled, text "Already Running"
- **Recovery:** No action needed

### Start Command Failed
- **Display:** Red error badge with error details
- **Button:** Remains enabled for retry
- **Recovery:** Check systemd logs, then retry

## Troubleshooting

### Service Won't Start
```bash
# Check if registered
systemctl --user list-unit-files | grep airgit

# Check service status
systemctl --user status airgit

# View error logs
journalctl --user-unit airgit -n 20
```

### Button Shows "Not Registered"
- Register service via "Register with Systemd" button first
- Refresh page if status doesn't update

### "Already Running" But Service Isn't Really Running
```bash
# Restart service via command line
systemctl --user restart airgit

# Or reload and refresh page
systemctl --user daemon-reload
# Then refresh browser
```

### Service Starts But Immediately Stops
- Check logs: `journalctl --user-unit airgit -n 50`
- May be a configuration issue
- Check if another AirGit instance is running on the same port

## Testing

### Manual Testing Checklist

- [ ] Service Status API returns correct values
- [ ] Not registered shows gray badge and disabled button
- [ ] Stopped service shows yellow badge and enabled button
- [ ] Running service shows green badge and disabled button
- [ ] Clicking start button on stopped service starts it
- [ ] After start, UI updates to show running state
- [ ] Clicking start on running service shows 409 error
- [ ] Network errors are handled gracefully
- [ ] Loading spinner appears during start
- [ ] Button is disabled while loading
- [ ] Error states allow retry

### API Testing

```bash
# Check status
curl http://localhost:8080/api/systemd/service-status

# Start service
curl -X POST http://localhost:8080/api/systemd/service-start

# Verify with systemctl
systemctl --user status airgit
```

## Future Enhancements

### Possible Additions

1. **Stop Service Button**
   - Stop running service from UI
   - Same UX pattern as start

2. **Restart Service Button**
   - Restart the service
   - Useful if service is stuck

3. **Service Auto-Refresh**
   - Periodically check service status
   - Update UI without user interaction

4. **Service Logs Display**
   - Show last N lines of service logs
   - Help with debugging

5. **Service Management**
   - Full service control from UI
   - Start, stop, restart, enable, disable

## Summary

The service start feature provides a convenient way for users to:
- Check if their AirGit service is running
- Manually start the service if needed
- Get immediate feedback on status changes
- Recover from service crashes

It complements the registration feature by allowing both automatic startup (on login) and manual startup (on demand) of the AirGit service.
