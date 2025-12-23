# Frontend Remote Management - User Guide

## Quick Start

### Opening Remotes Management

1. **Click the "Remotes" button** in the header navigation bar
   ```
   Header: [AirGit] [Branch] [Remotes] [Repos]
                             ↑ Click here
   ```

2. The **Remotes Management Modal** will open, showing all configured remotes

## Managing Remotes

### View All Remotes

When you click the "Remotes" button, you'll see a modal with all remotes:

```
┌────────────────────────────────────────────┐
│  Manage Remotes                        [×] │
├────────────────────────────────────────────┤
│                                            │
│  origin                                    │
│  https://github.com/user/repo.git         │
│                            [Edit] [Remove] │
│                                            │
│  upstream                                  │
│  https://github.com/upstream/repo.git     │
│                            [Edit] [Remove] │
│                                            │
├────────────────────────────────────────────┤
│        [+ Add Remote]                      │
│            [Close]                         │
└────────────────────────────────────────────┘
```

Each remote shows:
- 🏷️ **Remote name** (in green) - e.g., "origin", "upstream"
- 🔗 **Repository URL** (in gray) - full URL to the remote
- ✏️ **Edit button** (blue) - click to change the URL
- 🗑️ **Remove button** (red) - click to delete the remote

### Adding a New Remote

#### Step 1: Open Add Form
Click the **"+ Add Remote"** button in the modal

#### Step 2: Fill in Details

```
┌────────────────────────────────────────────┐
│  Add Remote                            [×] │
├────────────────────────────────────────────┤
│                                            │
│  Remote Name                               │
│  [upstream                             ]  │
│                                            │
│  Repository URL                            │
│  [https://github.com/user/repo.git    ]  │
│                                            │
├────────────────────────────────────────────┤
│      [Save]              [Cancel]          │
└────────────────────────────────────────────┘
```

Enter:
- **Remote Name** - A name for the remote (e.g., "upstream", "backup")
- **Repository URL** - The full URL to the repository

#### Step 3: Save
Click **"Save"** to add the remote

✅ **Success!** The modal will close and the new remote appears in the list

### Editing a Remote

#### Step 1: Find the Remote
In the Remotes Modal, find the remote you want to edit

#### Step 2: Click Edit
Click the blue **"Edit"** button next to the remote

#### Step 3: Update URL

```
┌────────────────────────────────────────────┐
│  Edit Remote                           [×] │
├────────────────────────────────────────────┤
│                                            │
│  Remote Name                               │
│  [upstream                             ]  │
│  (Cannot be changed)                       │
│                                            │
│  Repository URL                            │
│  [https://github.com/newowner/repo.git ]  │
│  (Update this)                             │
│                                            │
├────────────────────────────────────────────┤
│     [Update]             [Cancel]          │
└────────────────────────────────────────────┘
```

- 🔒 **Remote Name** - Disabled (cannot be changed)
- 🔗 **Repository URL** - Edit this to the new URL

#### Step 4: Update
Click **"Update"** to save changes

✅ **Success!** The remote URL is updated immediately

### Removing a Remote

#### Step 1: Find the Remote
In the Remotes Modal, find the remote you want to delete

#### Step 2: Click Remove
Click the red **"Remove"** button next to the remote

#### Step 3: Confirm Deletion

A confirmation dialog will appear:
```
Are you sure you want to remove the remote "upstream"?

[Cancel]  [OK]
```

⚠️ **Warning:** This action cannot be undone

#### Step 4: Confirm
Click **"OK"** to permanently remove the remote

✅ **Success!** The remote is deleted from the repository

## Common Tasks

### Adding GitHub Repository

1. Click "Remotes" button
2. Click "+ Add Remote"
3. Enter:
   - **Name:** origin
   - **URL:** https://github.com/username/repo.git
4. Click "Save"

### Switching Remote Source

1. Click "Remotes" button
2. Click "Edit" on the remote you want to change
3. Change the URL to the new repository
4. Click "Update"

### Managing Multiple Remotes

You can have multiple remotes pointing to different repositories:

```
origin   → https://github.com/me/repo.git (my fork)
upstream → https://github.com/original/repo.git (original)
backup   → https://github.com/backup/repo.git (backup)
```

This is useful for:
- 🍴 **Forks** - origin = your fork, upstream = original
- 💾 **Backups** - keep multiple copies
- 🤝 **Collaboration** - different team repositories

### Using Different Remote URLs

**SSH (Faster, Requires Setup):**
```
git@github.com:username/repo.git
```

**HTTPS (Password/Token Required):**
```
https://github.com/username/repo.git
```

**Git Protocol (Read-only):**
```
git://github.com/username/repo.git
```

## Tips & Tricks

### 🔄 Common Remote Names
- **origin** - Your main repository (usually on GitHub)
- **upstream** - Original repository (when forking)
- **backup** - Backup location
- **production** - Production server
- **staging** - Staging server

### ✅ Valid Remote Names
- Letters: `origin`, `upstream`, `main`
- Numbers: `backup1`, `server2`
- Hyphens: `production-old`, `my-backup`
- Underscores: `staging_old`, `my_repo`

### ❌ Invalid Remote Names
- Spaces: `my origin`
- Special chars: `my@origin`, `my$remote`
- Existing names: (already in use)

### 🔗 URL Tips
- **Always include .git** at the end for HTTPS:
  ✅ `https://github.com/user/repo.git`
  ❌ `https://github.com/user/repo`

- **SSH should not have .git:**
  ✅ `git@github.com:user/repo.git`
  ✅ `git@github.com:user/repo`

### ⚡ Quick Actions

**Push to a specific remote:**
Use the Push button with the remote pre-configured via PUSH or GIT_REMOTE env

**Pull from a specific remote:**
Set the upstream using edit function

## Error Messages

### "Missing required fields: name and url"
**Solution:** Fill in both Remote Name and Repository URL fields

### "Failed to add remote: remote already exists"
**Solution:** The remote name is already in use. Use a different name or edit the existing one.

### "Failed to update remote: error details"
**Solution:** Check the error message:
- Remote doesn't exist - remove and re-add it
- Invalid URL - verify the repository URL is correct
- Network issue - check your connection

### "Failed to remove remote: error details"
**Solution:**
- Remote doesn't exist - refresh the page
- Permission denied - check Git permissions
- Network issue - check your connection

## Troubleshooting

### Modal doesn't open
- 🔄 **Refresh the page** - F5 or Cmd+R
- 🗑️ **Clear cache** - Ctrl+Shift+Del
- 🔍 **Check console** - F12 → Console tab for errors

### Changes not appearing
- ⏳ **Wait a moment** - API may be processing
- 🔄 **Refresh** - Click Remotes button again
- 🌐 **Check network** - Look at Network tab (F12)

### Remote deleted accidentally
- ⚠️ **Cannot be undone** - Must re-add the remote
- ➕ **Re-add:** Click "+ Add Remote" and enter details again

### Slow performance
- 📊 **Too many remotes?** - Having 50+ remotes may slow things down
- 🌐 **Network issue** - Check your internet connection
- 💻 **Browser cache** - Clear cache and reload

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| **Esc** | Close modal or form |
| **Tab** | Move to next input |
| **Shift+Tab** | Move to previous input |
| **Enter** | Submit form |

## Mobile Usage

The remotes management works on mobile too!

### Mobile-Optimized Features
- ✋ **Large touch targets** - 44px minimum button size
- 📱 **Full-width modal** - Uses entire screen width
- 🔄 **Responsive text** - Readable on small screens
- ⌚ **Fast loading** - Optimized for mobile network

### Mobile Workflow
1. Tap "Remotes" button (bottom of header)
2. Scroll to see all remotes
3. Tap "Edit" or "Remove" on a remote
4. Fill in form (portrait or landscape)
5. Tap "Save" or "Update"

## Advanced Usage

### Importing from Existing Repository

If you already have remotes set up:

1. **In terminal:**
   ```bash
   cd /path/to/repo
   git remote -v
   ```

2. **In AirGit UI:**
   - Click "Remotes" button
   - You'll see all remotes listed
   - Can edit or remove as needed

### Syncing Multiple Remote Branches

After adding an upstream remote:

1. **Add upstream remote** using UI
2. **In terminal:**
   ```bash
   git fetch upstream
   git branch -u upstream/main main
   ```

### Setting Up for Pull Requests

1. **Fork** the repository on GitHub
2. **Add both remotes** in AirGit:
   - origin → your fork
   - upstream → original repository
3. **Create branch** from upstream
4. **Push to origin** for pull request

## Support

### Need Help?
- 📖 **Read the docs** - Check FRONTEND_REMOTES.md
- 🐛 **Report bugs** - Check GitHub issues
- 💬 **Ask for help** - GitHub discussions

### Keyboard Accessibility
- All buttons accessible via Tab key
- Enter key submits forms
- Escape closes modals
- Focus indicators visible

### Screen Reader Support
- Semantic HTML structure
- Labels on all inputs
- Descriptive button text
- Error messages announced

## Quick Reference

```bash
# What the UI does behind the scenes:

# Add Remote
git remote add <name> <url>

# Edit Remote
git remote set-url <name> <new-url>

# Remove Remote
git remote remove <name>

# View All Remotes (what we display)
git remote -v
```

---

**Ready to manage your remotes?** Click the "Remotes" button in AirGit to get started! 🚀
