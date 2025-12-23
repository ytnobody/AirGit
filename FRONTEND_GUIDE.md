# AirGit Frontend - Systemd Registration UI Guide

## Quick Start

The systemd registration feature is now integrated into AirGit's settings interface. Users can access it via the ⚙️ settings button in the header.

## UI Layout

### Header

```
┌────────────────────────────────────────┐
│ AirGit (green)    Branch Remotes Repos ⚙️ │
│ Branch: main                            │
└────────────────────────────────────────┘
```

The ⚙️ button is positioned in the top-right corner next to other action buttons.

## Settings Modal

When users click the ⚙️ button, the Settings modal opens:

```
┌─────────────────────────────────────────┐
│           Settings                      │
├─────────────────────────────────────────┤
│                                         │
│  ┌─────────────────────────────────┐   │
│  │ Auto-Start with Systemd    [✓]  │   │
│  │ Register AirGit to auto-start    │   │
│  │ on login                         │   │
│  │                                 │   │
│  │ AirGit is registered with       │   │
│  │ systemd and will auto-start     │   │
│  │ on login.                       │   │
│  │                                 │   │
│  │ [  Already Registered (gray)  ] │   │
│  └─────────────────────────────────┘   │
│                                         │
│          [    Close    ]                │
└─────────────────────────────────────────┘
```

## Status States

### 1. Not Registered (Initial State)

```
Status Badge:    ✗ Not Registered (gray)
Status Message:  Click the button below to register AirGit for auto-start.
Button State:    Register with Systemd (green, enabled)
Button Hover:    Slightly lighter green
```

Visual Example:
```
┌──────────────────────────────────────┐
│ Auto-Start with Systemd   [Not Reg]  │
│ Register AirGit to auto-start        │
│                                      │
│ Click the button below to register   │
│ AirGit for auto-start.               │
│                                      │
│ [ Register with Systemd ] (green)    │
└──────────────────────────────────────┘
```

### 2. Loading State

When user clicks the register button:

```
Status Badge:    (unchanged)
Status Message:  (unchanged)
Button State:    Disabled with spinner
Button Text:     Hidden
Spinner:         Rotating animation
```

Visual Example:
```
┌──────────────────────────────────────┐
│ Auto-Start with Systemd   [Not Reg]  │
│ Register AirGit to auto-start        │
│                                      │
│ Click the button below to register   │
│ AirGit for auto-start.               │
│                                      │
│ [        ⟳ (loading)        ] (gray) │
└──────────────────────────────────────┘
```

### 3. Successfully Registered

After successful registration:

```
Status Badge:    ✓ Registered (green)
Status Message:  ✓ Successfully registered! AirGit will auto-start on your next login.
Button State:    Already Registered (gray, disabled)
Button Opacity:  50% (disabled appearance)
```

Visual Example:
```
┌──────────────────────────────────────┐
│ Auto-Start with Systemd   [✓ Reg]    │
│ Register AirGit to auto-start        │
│                                      │
│ ✓ Successfully registered! AirGit    │
│ will auto-start on your next login.  │
│                                      │
│ [ Already Registered ] (gray, 50%)   │
└──────────────────────────────────────┘
```

### 4. Already Registered (Return Visit)

When user opens settings and is already registered:

```
Status Badge:    ✓ Registered (green)
Status Message:  AirGit is registered with systemd and will auto-start on login.
Button State:    Already Registered (gray, disabled)
Button Opacity:  50% (disabled appearance)
```

Visual Example:
```
┌──────────────────────────────────────┐
│ Auto-Start with Systemd   [✓ Reg]    │
│ Register AirGit to auto-start        │
│                                      │
│ AirGit is registered with systemd   │
│ and will auto-start on login.        │
│                                      │
│ [ Already Registered ] (gray, 50%)   │
└──────────────────────────────────────┘
```

### 5. Error State

If registration fails:

```
Status Badge:    ✗ Error (red)
Status Message:  ✗ Error: [error details]
Button State:    Register with Systemd (green, enabled for retry)
Button Opacity:  100% (enabled)
```

Visual Example:
```
┌──────────────────────────────────────┐
│ Auto-Start with Systemd   [Error]    │
│ Register AirGit to auto-start        │
│                                      │
│ ✗ Error: Failed to reload systemd   │
│ daemon: [error details]              │
│                                      │
│ [ Register with Systemd ] (green)    │
└──────────────────────────────────────┘
```

## User Workflows

### Workflow 1: New User Registration

```
1. User opens AirGit app
   ↓
2. User sees ⚙️ button in header
   ↓
3. User clicks ⚙️ button
   ↓
4. Settings modal opens
   ↓
5. User sees "Not Registered" badge and button
   ↓
6. User clicks "Register with Systemd"
   ↓
7. Loading spinner appears
   ↓
8. System registers service and reloads systemd
   ↓
9. Modal updates with success message
   ↓
10. Button is now disabled (already registered)
    ↓
11. User closes modal and continues using AirGit
    ↓
12. On next login, AirGit starts automatically
```

### Workflow 2: Returning Registered User

```
1. User opens AirGit app
   ↓
2. User clicks ⚙️ button
   ↓
3. Settings modal opens and loads status
   ↓
4. System detects service is already registered
   ↓
5. Modal immediately shows green "✓ Registered" badge
   ↓
6. "Already Registered" button is disabled
   ↓
7. Status message confirms auto-start is enabled
   ↓
8. User closes modal
```

### Workflow 3: Error Scenario

```
1. User clicks register button
   ↓
2. Network error occurs (no internet, server down, etc.)
   ↓
3. Modal shows red error badge
   ↓
4. Error message displays: "Network error: [details]"
   ↓
5. Register button remains enabled
   ↓
6. User can click button again to retry
   ↓
7. If successful, updates to success state
   ↓
8. If fails again, shows error state again
```

## Color Scheme

### Status Badges

| State | Color | Text | Background |
|-------|-------|------|------------|
| Not Registered | Gray | Not Registered | `bg-gray-700` |
| Registered | Green | ✓ Registered | `bg-emerald-900/50` |
| Error | Red | Error | `bg-red-900/50` |

### Buttons

| State | Color | Hover | Disabled |
|-------|-------|-------|----------|
| Enabled | `emerald-600` | `emerald-500` | `gray-600` (50% opacity) |
| Loading | `emerald-600` | (no hover) | disabled |
| Disabled | `gray-600` | (no hover) | disabled (50% opacity) |

## Responsive Design

The settings modal is fully responsive and works on:

### Mobile Phone (Portrait)
```
┌──────────────────┐
│ Settings         │
│ ┌──────────────┐ │
│ │ Auto-Start   │ │
│ │ [  ✓ Reg ]   │ │
│ │ Registered   │ │
│ │              │ │
│ │[Already Reg] │ │
│ └──────────────┘ │
│ [   Close    ]   │
└──────────────────┘
```

### Mobile Phone (Landscape)
```
┌──────────────────────────────────────┐
│ Settings                             │
│ ┌────────────────────────────────┐   │
│ │ Auto-Start    [✓ Registered]   │   │
│ │ Registered with systemd         │   │
│ │ [    Already Registered    ]    │   │
│ └────────────────────────────────┘   │
│ [         Close         ]             │
└──────────────────────────────────────┘
```

### Tablet
```
┌──────────────────────────────────────────┐
│              Settings                    │
│ ┌──────────────────────────────────────┐ │
│ │ Auto-Start with Systemd  [✓ Reg]    │ │
│ │ Register AirGit to auto-start on     │ │
│ │ login                                │ │
│ │                                      │ │
│ │ AirGit is registered with systemd   │ │
│ │ and will auto-start on login.       │ │
│ │                                      │ │
│ │ [  Already Registered (disabled) ]  │ │
│ └──────────────────────────────────────┘ │
│                                          │
│ [         Close          ]                │
└──────────────────────────────────────────┘
```

## Interactive Elements

### Settings Button (⚙️)
- **Hover Effect**: Background changes from `gray-700` to `gray-600`
- **Click Action**: Opens settings modal
- **Feedback**: Immediate visual response

### Settings Modal
- **Backdrop**: Semi-transparent black with blur effect
- **Modal**: Centered card with rounded corners
- **Close Behavior**: Click Close button or click backdrop
- **Animation**: Smooth fade-in/out

### Register Button
- **Not Registered State**:
  - Colors: Green (`emerald-600` / hover: `emerald-500`)
  - Click: Initiates registration

- **Loading State**:
  - Disabled: Yes
  - Shows: Loading spinner
  - Hides: Button text

- **Registered State**:
  - Colors: Gray (`gray-600`)
  - Opacity: 50%
  - Click: Disabled (no action)

- **Error State**:
  - Colors: Green (enabled for retry)
  - Click: Initiates registration again

## Accessibility Features

### Keyboard Navigation
- Settings button accessible via Tab key
- Modal close button accessible via Tab key
- Register button accessible via Tab key

### Visual Indicators
- Status badges use both color and text
- Button states are visually distinct
- Loading spinner provides feedback
- Error messages are clear and visible

### Color Independence
- Not using color alone to convey status
- Text labels accompany all color coding
- High contrast maintained for readability

## Browser Support

The frontend is compatible with:
- ✓ Chrome/Chromium (90+)
- ✓ Firefox (88+)
- ✓ Safari (14+)
- ✓ Edge (90+)
- ✓ Mobile browsers (iOS Safari, Chrome Android)

Uses modern CSS features:
- Flexbox
- CSS Grid
- CSS animations
- Backdrop filter (blur)
- Tailwind CSS

## Mobile Considerations

### Touch Targets
- Settings button: 28px × 22px (text) + padding
- Register button: full width (responsive)
- Close button: full width (responsive)
- All touch targets ≥ 44px minimum recommended height

### Viewport Handling
- Works on all screen sizes (320px to 2560px)
- Safe area insets for notched devices
- Responsive padding and spacing
- Portrait and landscape orientations

### Performance
- Minimal JavaScript (event listeners only)
- No heavy animations (only spinner)
- Efficient DOM updates
- No unnecessary re-renders

## Screenshot Descriptions

### Initial View (Not Registered)
The settings modal shows a gray badge with "Not Registered" and the register button is bright green and enabled. The status message explains that clicking the button will register AirGit.

### Success State
After successful registration, the badge turns green with a checkmark, the status message congratulates the user, and the button becomes disabled gray with text "Already Registered".

### Error State
If an error occurs, the badge turns red with "Error", and the status message shows the error details. The button remains green and enabled so users can retry.

## Code Examples

### Opening the Settings Modal
```javascript
settingsBtn.addEventListener('click', () => {
    settingsModal.classList.remove('hidden');
    loadSystemdStatus();
});
```

### Updating Status Display
```javascript
if (data.registered) {
    systemdStatusBadge.textContent = '✓ Registered';
    systemdStatusBadge.className = 'px-2 py-1 rounded text-xs font-medium bg-emerald-900/50 text-emerald-300';
    systemdStatusMessage.textContent = 'AirGit is registered with systemd and will auto-start on login.';
    systemdRegisterBtn.disabled = true;
}
```

### Handling Registration Click
```javascript
systemdRegisterBtn.addEventListener('click', async () => {
    // Disable button and show spinner
    systemdRegisterBtn.disabled = true;
    systemdBtnText.classList.add('hidden');
    systemdSpinner.classList.remove('hidden');

    try {
        const response = await fetch('/api/systemd/register', { method: 'POST' });
        const data = await response.json();
        
        // Handle response and update UI
    } catch (error) {
        // Handle error
    }
});
```

## Best Practices

### For Users
1. Click settings button early in first use
2. Ensure systemd is available on your system
3. Check status after registration
4. Restart if auto-start doesn't work

### For Developers
1. Always check registration status before showing register button
2. Provide clear feedback for all operations
3. Allow retries on error
4. Keep UI consistent with application theme
5. Handle network errors gracefully

## Troubleshooting

### Settings Modal Won't Open
- Check if JavaScript is enabled
- Verify ⚙️ button is clickable
- Check browser console for errors

### Register Button Not Working
- Ensure server is running on localhost:8080
- Check network connectivity
- Verify API endpoints are working
- Check browser console for error messages

### Status Not Updating
- Try clicking settings button again
- Check server logs for API errors
- Verify `/api/systemd/status` endpoint is working

### Button Stays Loading
- Page may have lost connection
- Refresh page and try again
- Check server logs for issues

## Support

For issues or questions:
1. Check SYSTEMD.md for technical details
2. Check FRONTEND_SYSTEMD.md for component documentation
3. Review browser console for error messages
4. Check server logs for API issues
5. Refer to troubleshooting section above
