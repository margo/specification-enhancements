#!/usr/bin/env sh
# Generates the MIS JWT SVID signing keypair used by the dev compose stack.
# Idempotent: writes the file only if absent. Run once per repo clone.
set -eu

OUT_DIR="${1:-var/dev/mis-etc/jwtsvid}"
OUT_FILE="${OUT_DIR}/jwtsvid.key"

mkdir -p "${OUT_DIR}"

if [ -s "${OUT_FILE}" ]; then
    echo "MIS JWT signing key already present at ${OUT_FILE}"
    exit 0
fi

# ECDSA P-256 PKCS#8 PEM. SUP § "Cryptographic Requirements" admits ES256.
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -outform PEM -out "${OUT_FILE}"
chmod 0600 "${OUT_FILE}"
echo "Wrote ${OUT_FILE}"
