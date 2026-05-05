#!/bin/sh
# Bootstrap SPIRE Server's UpstreamAuthority credentials on every stack start.
#
# Writes three files to the shared volume (/out):
#
#   root.crt               - Step CA's self-signed root (also used as
#                             spire-upstream.crt); MIS publishes this in the
#                             Bundle Map and SPIRE propagates it via
#                             upstream_bundle=true.
#   spire-upstream.crt     - Same as root.crt. SPIRE uses this cert as its
#                             UpstreamAuthority signing cert; SPIRE's internal
#                             CA is signed directly by the root, keeping the
#                             chain at root ==> spire-ca ==> leaf (2 certs, within
#                             the root's pathLen:1 constraint).
#   spire-upstream.key     - The Step CA root private key, decrypted from its
#                             JWK form. SPIRE signs its internal CA with this.
#
# Files are chowned to 0:0 (root) so the SPIRE Server container (run as root
# for volume-permissions compatibility) can read the private key.
set -eu

OUT="/out"

mkdir -p "$OUT"

if [ -s "$OUT/spire-upstream.crt" ] && [ -s "$OUT/spire-upstream.key" ] && [ -s "$OUT/root.crt" ]; then
    if step certificate verify "$OUT/spire-upstream.crt" --roots "$OUT/root.crt" >/dev/null 2>&1; then
        echo "SPIRE upstream already present and valid; skipping issuance."
        ls -l "$OUT"
        exit 0
    fi
    echo "Existing SPIRE upstream failed verify; re-issuing."
fi

echo "Exporting Step CA root as SPIRE upstream authority..."

cp /ca/certs/root_ca.crt "$OUT/root.crt"
cp /ca/certs/root_ca.crt "$OUT/spire-upstream.crt"

# Decrypt the Step CA root JWK private key to an unencrypted PKCS#8 PEM.
# SPIRE's UpstreamAuthority.disk plugin requires a plain PEM key file.
step crypto key format \
    --pem \
    --password-file /etc/step/password \
    --no-password --insecure \
    /ca/secrets/root_ca_key \
    > "$OUT/spire-upstream.key"

step certificate verify "$OUT/spire-upstream.crt" --roots "$OUT/root.crt"

chmod 0644 "$OUT/root.crt" "$OUT/spire-upstream.crt"
chmod 0600 "$OUT/spire-upstream.key"

echo "Wrote:"
ls -l "$OUT"
