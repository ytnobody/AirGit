#!/bin/bash
# Generate self-signed certificate for HTTPS support in HTTP environments
# This allows PWA installation even on non-HTTPS domains

set -e

CERT_DIR="${1:-.}"
CERT_FILE="$CERT_DIR/cert.pem"
KEY_FILE="$CERT_DIR/key.pem"

# Check if cert already exists
if [ -f "$CERT_FILE" ] && [ -f "$KEY_FILE" ]; then
    echo "✓ Certificate and key already exist:"
    echo "  - $CERT_FILE"
    echo "  - $KEY_FILE"
    exit 0
fi

# Generate self-signed certificate
echo "Generating self-signed certificate..."
openssl req -x509 -newkey rsa:4096 -nodes -out "$CERT_FILE" -keyout "$KEY_FILE" -days 365 \
    -subj "/C=JP/ST=Tokyo/L=Tokyo/O=AirGit/CN=localhost" 2>/dev/null

echo "✓ Certificate generated successfully:"
echo "  - Certificate: $CERT_FILE"
echo "  - Key: $KEY_FILE"
echo ""
echo "To use HTTPS with AirGit:"
echo "  export AIRGIT_TLS_CERT='$CERT_FILE'"
echo "  export AIRGIT_TLS_KEY='$KEY_FILE'"
echo "  airgit"
echo ""
echo "Or with command-line flags:"
echo "  airgit --tls-cert '$CERT_FILE' --tls-key '$KEY_FILE'"
