# AirGit Permalink Examples

Quick reference guide for creating and using permalink URLs in AirGit.

## Basic URL Structure

```
http://[server]:[port]?path=[repo-path]&branch=[branch-name]
```

## Quick Examples

### 1. Simple Repository Link
**Scenario:** Switch to a specific repository

```
http://localhost:8000?path=/var/git/my-app
```

**What happens:**
- Opens AirGit
- Switches to `/var/git/my-app` repository
- Keeps current branch

---

### 2. Simple Branch Link
**Scenario:** Switch branches in current repository

```
http://localhost:8000?branch=develop
```

**What happens:**
- Opens AirGit
- Checks out `develop` branch
- Keeps current repository

---

### 3. Full Permalink
**Scenario:** Switch both repository and branch

```
http://localhost:8000?path=/var/git/api&branch=feature/user-auth
```

**What happens:**
- Opens AirGit
- Switches to `/var/git/api` repository
- Checks out `feature/user-auth` branch
- Ready to push immediately

---

## Real-World Examples

### Development Team Setup

**Team Repository Links:**

```
# Web Frontend
http://airgit.local:8000?path=/var/git/frontend&branch=develop

# Backend API
http://airgit.local:8000?path=/var/git/api&branch=develop

# Mobile App
http://airgit.local:8000?path=/var/git/mobile&branch=develop
```

**Save these in your bookmark manager for quick access!**

---

### Release Management

**Release Branch Links:**

```
# Production release
http://airgit.company.com?path=/var/git/main-app&branch=main

# Staging release
http://airgit.company.com?path=/var/git/main-app&branch=staging

# Hotfix release
http://airgit.company.com?path=/var/git/main-app&branch=hotfix/critical-bug
```

---

### Feature Branch Workflow

**Feature branches for team members:**

```
# Feature: User authentication
http://airgit.local:8000?path=/var/git/api&branch=feature/oauth2-integration

# Feature: Payment processing
http://airgit.local:8000?path=/var/git/api&branch=feature/stripe-integration

# Fix: Database migration
http://airgit.local:8000?path=/var/git/api&branch=bugfix/migration-issue
```

---

## Use Cases by Role

### For Project Managers
**Share a single link for team to push updates:**
```
http://airgit.company.com?path=/var/git/web-app&branch=develop
```

### For Developers
**Create links for different features you work on:**
```
http://localhost:8000?path=/home/user/projects/app&branch=feature/new-ui
http://localhost:8000?path=/home/user/projects/app&branch=feature/refactor
```

### For DevOps/SRE
**Release management links:**
```
http://deploy.company.com?path=/var/repo/production&branch=main
http://deploy.company.com?path=/var/repo/staging&branch=staging
```

---

## Sharing via Different Channels

### Slack Message
```
Push to develop branch:
http://airgit.local:8000?path=/var/git/app&branch=develop
```

### Email Link
```html
<a href="http://airgit.company.com?path=/var/git/web&branch=main">
  Push to Production
</a>
```

### Documentation
```markdown
## Deployment

1. Make your changes
2. [Push to staging](http://airgit.local:8000?path=/var/git/app&branch=staging)
3. [Push to production](http://airgit.local:8000?path=/var/git/app&branch=main)
```

---

## Mobile Bookmarks

### Save to Home Screen (iOS)

1. Open URL: `http://airgit.local:8000?path=/var/git/app&branch=develop`
2. Tap Share button
3. Tap "Add to Home Screen"
4. Name it: "Push App to Develop"
5. Tap "Add"

### Save to Home Screen (Android)

1. Open URL: `http://airgit.local:8000?path=/var/git/app&branch=develop`
2. Tap ⋮ (three dots menu)
3. Tap "Install app"
4. Confirm installation

**Result:** One-tap access to your development workflow!

---

## Copy-Paste Ready Examples

### Example 1: Frontend Development
```
http://airgit.local:8000?path=/home/user/projects/frontend&branch=develop
```

### Example 2: Backend Development  
```
http://airgit.local:8000?path=/home/user/projects/backend&branch=develop
```

### Example 3: Production Deployment
```
http://airgit.company.com?path=/var/repo/production&branch=main
```

### Example 4: Hotfix
```
http://airgit.company.com?path=/var/repo/api&branch=hotfix/critical-issue
```

### Example 5: Feature Branch
```
http://airgit.local:8000?path=/var/git/app&branch=feature/new-login
```

---

## Testing Your URLs

1. **Test without parameters:**
   ```
   http://localhost:8000
   ```
   - Should work normally

2. **Test with path only:**
   ```
   http://localhost:8000?path=/var/git/test-repo
   ```
   - Should switch repository

3. **Test with branch only:**
   ```
   http://localhost:8000?branch=main
   ```
   - Should switch branch

4. **Test with both:**
   ```
   http://localhost:8000?path=/var/git/test-repo&branch=develop
   ```
   - Should switch both and work correctly

---

## See Also

- [README.md](./README.md) - Main documentation
- [PERMALINK_USAGE.md](./PERMALINK_USAGE.md) - Comprehensive usage guide
