# Frontend Remote Management Implementation

## Overview

The AirGit frontend now includes a complete UI for managing Git remotes. Users can add, edit, and remove remotes directly from the web interface.

## User Interface

### Remotes Button
A new "Remotes" button has been added to the header navigation bar, between the "Branch" and "Repos" buttons. Clicking this button opens the Remotes Management modal.

### Remotes Management Modal

The modal displays all configured remotes for the current repository with the following information for each remote:
- **Remote Name** - The name of the remote (e.g., origin, upstream)
- **Repository URL** - The URL of the remote repository
- **Edit Button** - Allows editing the remote URL
- **Remove Button** - Allows removing the remote

Features:
- View all remotes at a glance
- Action buttons for each remote
- "+ Add Remote" button to add a new remote
- "Close" button to dismiss the modal

### Add/Edit Remote Modal

Opens when:
- Clicking "+ Add Remote" button
- Clicking "Edit" on an existing remote

The form contains:
- **Remote Name** - Input field for remote name (disabled when editing)
- **Repository URL** - Input field for the repository URL
- Error display area for validation/operation errors
- "Save" button to save changes
- "Cancel" button to dismiss the form

### Confirmation Dialogs

When removing a remote, a confirmation dialog appears asking:
```
Are you sure you want to remove the remote "name"?
```

This prevents accidental deletion of remotes.

## JavaScript Functions

### loadRemotes()
Fetches the list of remotes from the server and displays them.

**Parameters:** None (uses current repository from URL)

**Returns:** Promise

**API Endpoint:** GET `/api/remotes`

### displayRemotes(remotes)
Renders the list of remotes in the modal.

**Parameters:**
- `remotes` (Array) - Array of remote objects with `name` and `url` properties

**Returns:** void

### openAddRemoteForm()
Opens the remote form modal for adding a new remote.

**Parameters:** None

**Returns:** void

**Side Effects:**
- Clears input fields
- Sets form title to "Add Remote"
- Enables the remote name input
- Changes button text to "Add"

### editRemote(name, url)
Opens the remote form modal for editing an existing remote.

**Parameters:**
- `name` (String) - The name of the remote to edit
- `url` (String) - The current URL of the remote

**Returns:** void

**Side Effects:**
- Populates input fields with current values
- Sets form title to "Edit Remote"
- Disables the remote name input
- Changes button text to "Update"

### removeRemote(name)
Removes a remote after confirmation.

**Parameters:**
- `name` (String) - The name of the remote to remove

**Returns:** Promise

**Flow:**
1. Show confirmation dialog
2. If confirmed, send DELETE request to API
3. Refresh remotes list on success
4. Show error alert on failure

### saveRemote()
Validates and saves a remote (add or update).

**Parameters:** None

**Returns:** Promise

**Flow:**
1. Validate inputs (name and URL required)
2. Show loading state
3. Call appropriate API endpoint:
   - `/api/remote/add` for new remotes
   - `/api/remote/update` for editing
4. On success: Close modal and refresh list
5. On error: Display error message

## HTML Structure

### Main Elements

```html
<!-- Remotes Button (in header) -->
<button id="remotes-selector-btn">Remotes</button>

<!-- Remotes Management Modal -->
<div id="remotes-modal">
    <div id="remotes-list"><!-- Remote items rendered here --></div>
    <button id="add-remote-btn">+ Add Remote</button>
    <button id="remotes-modal-close-btn">Close</button>
</div>

<!-- Add/Edit Remote Form Modal -->
<div id="remote-form-modal">
    <input id="remote-name-input" placeholder="origin" />
    <input id="remote-url-input" placeholder="https://..." />
    <div id="remote-form-error"><!-- Error messages --></div>
    <button id="remote-form-submit">Save</button>
    <button id="remote-form-close-btn">Cancel</button>
</div>
```

### Remote Item Template

Each remote is displayed as:
```html
<div class="bg-gray-700 rounded px-4 py-3">
    <div class="font-semibold text-emerald-400">remoteName</div>
    <div class="text-xs text-gray-400">https://github.com/user/repo.git</div>
    <button onclick="editRemote(...)">Edit</button>
    <button onclick="removeRemote(...)">Remove</button>
</div>
```

## Event Listeners

| Element | Event | Handler | Action |
|---------|-------|---------|--------|
| Remotes Button | click | loadRemotes | Open modal and load remotes |
| Add Remote Button | click | openAddRemoteForm | Show add form |
| Edit Button (each remote) | click | editRemote | Show edit form |
| Remove Button (each remote) | click | removeRemote | Delete with confirmation |
| Form Submit Button | click | saveRemote | Save remote |
| Form Close Buttons | click | (inline) | Hide modal |
| Modal Background | click | (inline) | Close if clicked outside |

## Styling

The UI uses Tailwind CSS with the same color scheme as the rest of AirGit:

- **Background:** Gray-800 (`#1f2937`)
- **Text:** Gray-50 (`#f9fafb`)
- **Primary Color:** Emerald-400 (`#34d399`)
- **Secondary Color:** Blue-600 (`#2563eb`)
- **Danger Color:** Red-600 (`#dc2626`)

Modal features:
- Backdrop blur effect
- Smooth transitions
- Focus states for accessibility
- Responsive design for mobile

## Error Handling

The frontend handles the following error scenarios:

1. **Network Errors** - Displays "Connection failed: [error message]"
2. **API Errors** - Displays error message from server response
3. **Validation Errors** - Shows inline error in form
4. **Load Failures** - Displays "Failed to load remotes"

## Security

- HTML escaping for all user-provided content
- XSS prevention through `escapeHtml()` function
- Input validation before API calls
- Confirmation required for destructive actions

## Integration with Backend

The frontend communicates with the backend via these endpoints:

### GET /api/remotes
**Query Parameters:**
- `repoPath` (optional) - Current repository path

**Response Format:**
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
**Query Parameters:**
- `repoPath` (optional) - Current repository path

**Request Body:**
```json
{
  "name": "upstream",
  "url": "https://github.com/upstream/repo.git"
}
```

### POST /api/remote/update
**Query Parameters:**
- `repoPath` (optional) - Current repository path

**Request Body:**
```json
{
  "name": "origin",
  "url": "https://github.com/newowner/repo.git"
}
```

### POST /api/remote/remove
**Query Parameters:**
- `repoPath` (optional) - Current repository path

**Request Body:**
```json
{
  "name": "upstream"
}
```

## Browser Compatibility

The frontend uses:
- Modern CSS (Flexbox, Grid)
- ES6 JavaScript features (async/await, arrow functions)
- Fetch API

Supported browsers:
- Chrome 60+
- Firefox 55+
- Safari 12+
- Edge 79+

## Accessibility

Features:
- Semantic HTML elements
- Focus states on interactive elements
- Error messages associated with inputs
- Keyboard navigation support
- ARIA labels on modal dialogs

## Testing the Frontend

To test the remote management UI:

1. Start AirGit server:
   ```bash
   ./airgit -repo-path /path/to/repo
   ```

2. Open in browser:
   ```
   http://localhost:8080
   ```

3. Click the "Remotes" button in the header

4. Test operations:
   - View existing remotes
   - Click "Add Remote" and add a new remote
   - Click "Edit" on a remote and change its URL
   - Click "Remove" on a remote and confirm deletion

## Troubleshooting

### Remotes not loading
- Check browser console for errors (F12)
- Verify backend API endpoints are working
- Check repository path in URL

### Changes not persisting
- Verify backend is running
- Check network tab in browser dev tools
- Look for error messages in form

### Modal not opening
- Check browser console for JavaScript errors
- Verify all HTML elements are present
- Check CSS z-index values
