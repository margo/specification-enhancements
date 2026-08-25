#!/usr/bin/env bash
# Generate a self-signed TLS certificate for the MIS HTTPS server.
# Idempotent: skips generation if the cert already exists so a running MIS
# container is not left with a stale trust anchor.  Delete the files manually
# to force regeneration.  Intended for local dev only.  Not an SVID.
set -euo pipefail

DIR="${1:-var/dev/mis-etc/tls}"
DAYS="${DAYS:-365}"
CN="${CN:-mis.margo.example.com}"

mkdir -p "$DIR"

if [ -s "$DIR/server.crt" ] && [ -s "$DIR/server.key" ]; then
    echo "MIS TLS cert already present at $DIR/server.crt"
    exit 0
fi

openssl req -x509 -newkey ec:<(openssl ecparam -name prime256v1) \
    -days "$DAYS" \
    -nodes \
    -keyout "$DIR/server.key" \
    -out "$DIR/server.crt" \
    -subj "/CN=$CN" \
    -addext "subjectAltName=DNS:$CN,DNS:localhost,IP:127.0.0.1"

chmod 600 "$DIR/server.key"

echo "Wrote:"
echo "  $DIR/server.crt"
echo "  $DIR/server.key"
