# AirGit Frontend - Systemd Registration UI

## Overview

This document describes the user interface implementation for the systemd service registration feature. The frontend provides an easy-to-use settings panel where users can register AirGit to auto-start on login.

## UI Components

### 1. Settings Button

**Location:** Header (top-right corner)

**Design:**
- Small button with ⚙️ icon
- Consistent with existing button styling
- Positioned next to Branch, Remotes, and Repos buttons

**Functionality:**
- Click to open Settings modal
- Automatically loads systemd status when modal opens

```html
<button id="settings-btn" class="text-xs bg-gray-700 hover:bg-gray-600 px-2 py-1 rounded text-gray-300">⚙️</button>
```

### 2. Settings Modal

**Location:** Full-screen modal overlay

**Content:**
- Title: "Settings"
- Systemd Registration Section with:
  - Title: "Auto-Start with Systemd"
  - Description: "Register AirGit to auto-start on login"
  - Status Badge (shows registration state)
  - Status Message (dynamic information)
  - Register Button (enabled/disabled based on state)

**Styling:**
- Dark theme matching existing UI
- Rounded corners and proper spacing
- Responsive design for mobile

```html
<div id="settings-modal" class="hidden fixed inset-0 bg-black/50 backdrop-blur-sm z-50 ...">
    <div class="bg-gray-800 rounded-lg p-6 w-full max-w-sm border border-gray-700">
        <h2 class="text-lg font-bold text-emerald-400 mb-4">Settings</h2>
        
        <!-- Systemd Registration Section -->
        <div class="space-y-4 mb-6">
            <div class="border border-gray-700 rounded p-4 bg-gray-900/50">
                <!-- Content -->
            </div>
        </div>
        
        <button id="settings-modal-close-btn" ...>Close</button>
    </div>
</div>
```

## State Management

### Status Badge

Displays the current registration state with dynamic styling:

**Not Registered:**
```
Badge Text: "Not Registered"
Color: Gray (bg-gray-700, text-gray-300)
```

**Registered:**
```
Badge Text: "✓ Registered"
Color: Green (bg-emerald-900/50, text-emerald-300)
```

**Error:**
```
Badge Text: "Error"
Color: Red (bg-red-900/50, text-red-300)
```

### Status Message

Provides user-friendly information:

**Not Registered:**
```
"Click the button below to register AirGit for auto-start."
```

**Registered:**
```
"AirGit is registered with systemd and will auto-start on login."
```

**Already Registered (from duplicate attempt):**
```
"AirGit is already registered with systemd."
```

**Success (after registration):**
```
"✓ Successfully registered! AirGit will auto-start on your next login."
```

**Error:**
```
"✗ Error: [error message from server]"
```

### Register Button

**Not Registered State:**
- Text: "Register with Systemd"
- Enabled: ✓
- Color: Green (emerald-600, hover: emerald-500)
- Clickable: Yes

**Registered State:**
- Text: "Already Registered"
- Enabled: ✗
- Color: Gray (gray-600)
- Opacity: 50% (disabled appearance)
- Clickable: No

**Loading State (during registration):**
- Shows loading spinner next to text
- Button disabled
- Button text hidden

## JavaScript Functions

### loadSystemdStatus()

**Purpose:** Check current systemd registration status from server

**Flow:**
1. Send GET request to `/api/systemd/status`
2. Parse JSON response
3. Update UI based on registration status:
   - If registered: Show green badge, disable button
   - If not registered: Show gray badge, enable button
4. Update status message accordingly

**Error Handling:**
- Catch network errors
- Display error badge and message
- Log error to console

```javascript
async function loadSystemdStatus() {
    try {
        const response = await fetch('/api/systemd/status');
        const data = await response.json();

        if (data.registered) {
            // Update UI for registered state
        } else {
            // Update UI for not registered state
        }
    } catch (error) {
        // Handle error
    }
}
```

### systemdRegisterBtn.addEventListener('click', ...)

**Purpose:** Handle registration button click

**Flow:**
1. Disable button and show loading spinner
2. Send POST request to `/api/systemd/register`
3. Handle response:
   - Success (200): Show success message, disable button
   - Conflict (409): Show already registered message, disable button
   - Error (500): Show error message, enable button for retry
4. Hide loading spinner
5. Update UI state

**Error Handling:**
- Network errors: Catch and display error message
- HTTP errors: Parse error message from response
- Always reset button state for retry capability

```javascript
systemdRegisterBtn.addEventListener('click', async () => {
    // Show loading state
    systemdRegisterBtn.disabled = true;
    systemdBtnText.classList.add('hidden');
    systemdSpinner.classList.remove('hidden');

    try {
        const response = await fetch('/api/systemd/register', { method: 'POST' });
        const data = await response.json();

        if (response.ok && data.success) {
            // Success: Update UI and disable button
        } else if (response.status === 409) {
            // Already registered
        } else {
            // Error: Show message and enable for retry
        }
    } catch (error) {
        // Network error: Show message and enable for retry
    }
});
```

## UI Interactions

### Opening Settings Modal

**Trigger:** Click settings button (⚙️)

**Actions:**
1. Remove "hidden" class from settings modal
2. Call `loadSystemdStatus()` to check current state
3. Update UI elements based on response

**Visual Feedback:**
- Modal slides in with fade-in effect
- Background darkens with backdrop blur
- Modal appears centered on screen

### Closing Settings Modal

**Trigger:**
1. Click "Close" button
2. Click outside modal (backdrop)

**Actions:**
1. Add "hidden" class to settings modal
2. Modal fades out

### Registering with Systemd

**Trigger:** Click "Register with Systemd" button

**Before Click:**
- Button enabled
- Button text visible
- No spinner

**During Registration:**
- Button disabled (opacity reduced)
- Button text hidden
- Loading spinner visible
- Button click disabled

**After Success:**
- Status badge: "✓ Registered" (green)
- Status message: Success message
- Button text: "Already Registered"
- Button state: Disabled
- Button color: Gray

**After Error:**
- Status badge: "Error" (red)
- Status message: Error details
- Button state: Enabled for retry
- Button color: Green (ready to retry)

## Mobile Considerations

### Touch Targets
- All buttons meet 44px minimum touch target size
- Adequate spacing between interactive elements
- Proper padding for easy tapping

### Responsive Design
- Modal works on all screen sizes
- Proper safe area insets for notched devices
- Landscape and portrait orientation support

### Loading States
- Clear visual feedback during async operations
- Loading spinner prevents double-submission
- Disabled button prevents multiple requests

## Accessibility

### Semantic HTML
- Proper heading hierarchy (h2 for modal title)
- Label elements for form inputs (if added in future)
- Button elements with clear text

### Color Contrast
- All text meets WCAG AA contrast requirements
- Status colors support colorblind users
- Success/error states not color-only

### Keyboard Navigation
- Modal can be closed with Escape key (can be added)
- All buttons are keyboard accessible
- Focus states visible

## Code Structure

### HTML Elements

```html
<!-- Settings Button (in header) -->
<button id="settings-btn" ...>⚙️</button>

<!-- Settings Modal -->
<div id="settings-modal" ...>
    <!-- Status Badge -->
    <div id="systemd-status-badge" ...></div>
    
    <!-- Status Message -->
    <div id="systemd-status-message" ...></div>
    
    <!-- Register Button -->
    <button id="systemd-register-btn" ...>
        <span id="systemd-btn-text">Register with Systemd</span>
        <svg id="systemd-spinner" ...></svg>
    </button>
</div>
```

### JavaScript Variables

```javascript
const settingsBtn = document.getElementById('settings-btn');
const settingsModal = document.getElementById('settings-modal');
const settingsModalCloseBtn = document.getElementById('settings-modal-close-btn');
const systemdRegisterBtn = document.getElementById('systemd-register-btn');
const systemdBtnText = document.getElementById('systemd-btn-text');
const systemdSpinner = document.getElementById('systemd-spinner');
const systemdStatusBadge = document.getElementById('systemd-status-badge');
const systemdStatusMessage = document.getElementById('systemd-status-message');
```

### Event Listeners

1. `settingsBtn.addEventListener('click', ...)` - Open settings modal
2. `settingsModalCloseBtn.addEventListener('click', ...)` - Close settings modal
3. `settingsModal.addEventListener('click', ...)` - Close on backdrop click
4. `systemdRegisterBtn.addEventListener('click', ...)` - Handle registration

## User Workflow

### First Time User (Not Registered)

1. User clicks ⚙️ settings button
2. Settings modal opens
3. Modal shows "Not Registered" badge (gray)
4. Status message: "Click the button below to register AirGit for auto-start."
5. "Register with Systemd" button is enabled and green
6. User clicks button
7. Loading spinner appears
8. Button becomes disabled
9. After a moment, UI updates to show success:
   - Badge turns green with ✓ symbol
   - Status message: "✓ Successfully registered! AirGit will auto-start on your next login."
   - Button text changes to "Already Registered"
   - Button becomes disabled (gray)
10. User can close modal or try again (will show 409 conflict)

### Returning User (Already Registered)

1. User clicks ⚙️ settings button
2. Settings modal opens
3. Modal immediately shows "✓ Registered" badge (green)
4. Status message: "AirGit is registered with systemd and will auto-start on login."
5. "Already Registered" button is disabled (gray)
6. User closes modal

### Error Scenario

1. User clicks register button
2. Network error or server error occurs
3. Status badge turns red with "Error"
4. Status message shows error details
5. Button remains enabled for retry
6. User can click again to retry

## Styling

### Color Scheme

**Status Colors:**
- Green (Registered): `#059669` (emerald-600)
- Gray (Not Registered): `#374151` (gray-700)
- Red (Error): `#991b1b` (red-900)

**Component Colors:**
- Button Active: `#059669` (emerald-600)
- Button Hover: `#10b981` (emerald-500)
- Button Disabled: `#4b5563` (gray-600)
- Modal Background: `#1f2937` (gray-800)
- Modal Border: `#374151` (gray-700)

### Animations

**Loading Spinner:**
- Continuous rotation animation
- 1-second duration
- Linear easing

**Modal:**
- Fade-in effect with backdrop blur
- Smooth transitions

## Testing

### Manual Testing Checklist

- [ ] Settings button appears in header
- [ ] Clicking settings button opens modal
- [ ] Modal shows correct initial state
- [ ] Modal closes when clicking "Close" button
- [ ] Modal closes when clicking backdrop
- [ ] Status loads correctly
- [ ] "Not Registered" state shows correct UI
- [ ] "Registered" state shows correct UI
- [ ] Click register button triggers loading state
- [ ] Success response updates UI correctly
- [ ] 409 Conflict shows "Already Registered"
- [ ] Error response shows error message
- [ ] Button can be retried after error
- [ ] UI works on mobile portrait
- [ ] UI works on mobile landscape
- [ ] UI works on tablet
- [ ] UI works on desktop
- [ ] Modal backdrop blur effect works
- [ ] Loading spinner rotates smoothly

### Browser Compatibility

- Chrome/Chromium (latest)
- Firefox (latest)
- Safari (latest)
- Edge (latest)
- Mobile browsers (iOS Safari, Chrome Android)

## Future Enhancements

### Possible Additions

1. **Unregister Function**
   - Add button to unregister from systemd
   - Confirmation dialog before unregistering
   - Remove service file and disable

2. **Service Status**
   - Show if service is currently running
   - "Start" button to manually start service
   - "Stop" button to stop service
   - Real-time status updates

3. **Additional Settings**
   - Auto-update check option
   - Logging level selection
   - Performance preferences
   - Theme selection

4. **Status Polling**
   - Periodically check systemd status
   - Update UI if status changes externally
   - Notify user of changes

5. **Help/Documentation**
   - Inline help button
   - Link to SYSTEMD.md
   - Keyboard shortcuts guide

## Files Modified

- **static/index.html**
  - Added settings button to header (line 50)
  - Added settings modal (lines 210-233)
  - Added JavaScript event handlers and functions (lines 1017-1112)

## Code Statistics

- **HTML additions:** ~24 lines (modal structure)
- **JavaScript additions:** ~95 lines (event handlers and functions)
- **Total additions:** ~119 lines

## Summary

The frontend implementation provides a clean, user-friendly interface for systemd registration. Users can easily check if AirGit is registered and complete the registration process with a single click. The UI provides clear feedback at each step and handles errors gracefully.
