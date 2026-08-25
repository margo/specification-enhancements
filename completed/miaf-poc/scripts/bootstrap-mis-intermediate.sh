#!/bin/sh
# Bootstrap MIS's intermediate CA on every stack start.
#
# Writes three files to the shared volume (/out):
#
#   root.crt               - Step CA's self-signed root; MIS publishes this as
#                             the sole X.509 trust anchor in the Bundle Map
#   mis-intermediate.crt   - MIS's sub-CA cert (URI SAN = spiffe://<td>/, per
#                             SPIFFE X509-SVID §3.2)
#   mis-intermediate.key   - private key matching mis-intermediate.crt
#
# All three files are chowned to uid/gid 65532 (distroless `nonroot`) so MIS
# can read them read-only when mounted at /etc/mis/ca.
set -eu

OUT="/out"
TRUST_DOMAIN="margo.example.com"
MIS_UID="65532"
MIS_GID="65532"

mkdir -p "$OUT"

# If the outputs already exist (compose restart without `-v`), keep them iff
# they still validate against the current root and have not expired. Step CA's
# keypair is stable across restarts, so re-issuance is usually unnecessary and
# would rotate the intermediate needlessly; but shipping an expired or
# root-mismatched artifact silently would be worse.
if [ -s "$OUT/mis-intermediate.crt" ] && [ -s "$OUT/mis-intermediate.key" ] && [ -s "$OUT/root.crt" ]; then
    if step certificate verify "$OUT/mis-intermediate.crt" --roots "$OUT/root.crt" >/dev/null 2>&1; then
        echo "Intermediate already present and valid; skipping issuance."
        ls -l "$OUT"
        exit 0
    fi
    echo "Existing intermediate failed verify; re-issuing."
fi

# Copy Step CA's root out as the Bundle Map trust anchor.
cp /ca/certs/root_ca.crt "$OUT/root.crt"

echo "Issuing MIS intermediate offline using Step CA's ROOT as signer..."

# The default --profile intermediate-ca template omits SANs entirely.  We
# supply a custom template that passes .SANs through so the SPIFFE URI SAN
# added via --san is included in the issued certificate.
cat > /tmp/mis-intermediate.tpl << 'ENDTPL'
{
    "subject": {{ toJson .Subject }},
    "sans": {{ toJson .SANs }},
    "keyUsage": ["certSign", "crlSign"],
    "basicConstraints": {
        "isCA": true,
        "maxPathLen": 0
    },
    "subjectKeyId": {{ toJson .SubjectKeyId }}
}
ENDTPL

step certificate create \
    "Margo MIS Intermediate" \
    "$OUT/mis-intermediate.crt" \
    "$OUT/mis-intermediate.key" \
    --template /tmp/mis-intermediate.tpl \
    --ca /ca/certs/root_ca.crt \
    --ca-key /ca/secrets/root_ca_key \
    --ca-password-file /etc/step/password \
    --san "spiffe://${TRUST_DOMAIN}/" \
    --not-after 720h \
    --kty EC --curve P-256 \
    --no-password --insecure \
    --force

# Sanity-check that the issued cert validates against the root we just copied
# out. Fails the bootstrap fast if Step CA's init placed unexpected
# constraints on the root (e.g. pathLen issues) instead of discovering at
# device enrollment time.
step certificate verify "$OUT/mis-intermediate.crt" --roots "$OUT/root.crt"

# `step certificate create` emits a PKCS#8 key by default under the --no-password
# path; the intermediate cert bundles only the leaf (no full chain). That is
# what `ca.NewIntermediateIssuer` expects - the root is published separately in
# the Bundle Map, and Go's x509.CreateCertificate call adds the intermediate to
# the chain returned to clients.

chown "${MIS_UID}:${MIS_GID}" "$OUT/root.crt" "$OUT/mis-intermediate.crt" "$OUT/mis-intermediate.key"
chmod 0644 "$OUT/root.crt" "$OUT/mis-intermediate.crt"
chmod 0600 "$OUT/mis-intermediate.key"

echo "Wrote:"
ls -l "$OUT"
