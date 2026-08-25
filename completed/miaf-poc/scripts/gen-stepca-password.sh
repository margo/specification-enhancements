#!/usr/bin/env sh
# Generates the password used to encrypt Step CA's root key in the dev stack.
set -eu

OUT_DIR="${1:-config/step-ca}"
OUT_FILE="${OUT_DIR}/password"

mkdir -p "${OUT_DIR}"

if [ -s "${OUT_FILE}" ]; then
    echo "Step CA password already present at ${OUT_FILE}"
    exit 0
fi

openssl rand -base64 32 | tr -d '\n' > "${OUT_FILE}"
chmod 0600 "${OUT_FILE}"
echo "Wrote ${OUT_FILE}"
