# PWA Installation Guide for AirGit

## Quick Start

AirGit is a Progressive Web App (PWA) that can be installed on mobile devices and desktops for quick access.

### Standard Installation (Recommended)

#### Option 1: HTTP + Browser Menu (Any Environment)

1. Open AirGit in your browser
2. Open the Settings menu (gear icon)
3. Scroll to "Install App" section
4. Follow the browser-specific instructions shown

**Browser Instructions:**
- **Android Chrome/Firefox**: Menu (⋮) → "Add to Home screen" or "Install app"
- **iOS Safari**: Share (↗️) → "Add to Home Screen"
- **Desktop Chrome/Edge/Firefox**: Address bar install icon or menu → "Install AirGit"

### Advanced Setup: HTTPS (For Full PWA Features)

If you want the app to run in true standalone mode without browser UI, you need HTTPS. You can enable HTTPS with a self-signed certificate:

#### Step 1: Generate Certificate

```bash
cd /path/to/AirGit
./generate-cert.sh .
```

This creates:
- `cert.pem` - Certificate file
- `key.pem` - Private key file

#### Step 2: Start AirGit with HTTPS

```bash
# Using environment variables
export AIRGIT_TLS_CERT=$(pwd)/cert.pem
export AIRGIT_TLS_KEY=$(pwd)/key.pem
airgit

# Or using command-line flags
airgit --tls-cert ./cert.pem --tls-key ./key.pem
```

#### Step 3: Access via HTTPS

```
https://your-machine-ip:8080
```

**Note:** Browsers will show a security warning for self-signed certificates. Click "Advanced" → "Proceed" to continue.

#### Step 4: Install the App

- The browser will show an "Install" button in the address bar
- Click it to install the app
- The app will now run in standalone mode without browser UI

## How It Works

### HTTP Mode (Default)
- ✅ Works on any network (LAN, remote)
- ✅ Full functionality for Git operations
- ✅ Can add to home screen via browser menu
- ⚠️ Runs within browser (shows browser UI)
- ⚠️ Service Worker may have limitations

### HTTPS Mode (With Certificate)
- ✅ Full PWA standalone mode
- ✅ App runs without browser UI
- ✅ Better offline support
- ✅ Installation prompt in address bar
- ⚠️ Requires certificate setup
- ⚠️ Self-signed certificates show browser warning

## Troubleshooting

### "Install" button not showing in address bar
- Ensure you're using HTTPS (not HTTP)
- Or use the Settings menu to add via browser menu
- Clear browser cache and reload

### App still opens in browser
- Use HTTPS mode instead of HTTP
- Ensure the certificate is properly configured
- Uninstall and reinstall the app

### Certificate warnings
- Self-signed certificates will show a browser warning
- This is normal and safe - click "Proceed" to continue
- For production, use a CA-signed certificate instead

### Service Worker not updating
- Clear Service Worker cache: DevTools → Application → Service Workers → Unregister
- Or open DevTools → Application → Storage → Clear all

## Environment Variables

```bash
AIRGIT_TLS_CERT    # Path to certificate file (enables HTTPS)
AIRGIT_TLS_KEY     # Path to key file (enables HTTPS)
AIRGIT_REPO_PATH   # Git repository path (default: $HOME)
AIRGIT_LISTEN_ADDR # Listen address (default: 0.0.0.0)
AIRGIT_LISTEN_PORT # Listen port (default: 8080)
```

## Browser Compatibility

| Browser | HTTP | HTTPS | Install Method |
|---------|------|-------|-----------------|
| Chrome (Android) | ✅ | ✅ | Menu or address bar |
| Firefox (Android) | ✅ | ✅ | Menu |
| Safari (iOS) | ✅ | ✅ | Share button |
| Chrome (Desktop) | ✅ | ✅ | Address bar or menu |
| Edge (Desktop) | ✅ | ✅ | Menu |
| Firefox (Desktop) | ✅ | ✅ | Menu (if available) |

## Creating a Production Certificate

For production use, get a proper SSL certificate from a Certificate Authority:

```bash
# Let's Encrypt (free)
# https://letsencrypt.org/

# Using certbot
sudo certbot certonly --standalone -d your-domain.com

# Then start AirGit with the certificate
export AIRGIT_TLS_CERT=/etc/letsencrypt/live/your-domain.com/fullchain.pem
export AIRGIT_TLS_KEY=/etc/letsencrypt/live/your-domain.com/privkey.pem
airgit
```

## Development Notes

- Service Worker is configured to cache static assets
- API calls always fetch from network for fresh data
- Offline mode provides basic fallback to home screen
- PWA metadata in `manifest.json` defines app behavior
