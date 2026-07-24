#!/usr/bin/env bash
# Generate a test factory PKI for the MIAF PoC.
#
# Layout:
#   test/testdata/pki/generated/factory/
#     root.crt, root.key          - factory root CA (self-signed)
#     device-01.crt, device-01.key - factory leaf (ECDSA P-256)
#     device-02.crt, device-02.key - factory leaf (RSA-PSS 3072, for alg coverage)
#     untrusted.crt, untrusted.key - leaf signed by a DIFFERENT root (negative test)
#     untrusted-root.crt           - the other root (NOT added to MIS ClientCAs)
#
# The factory root is ECDSA P-256 so both MIAF-permitted algorithms round-trip
# through the chain.  Per SUP Cryptographic Requirements, signatures must be
# ES256 or PS256; we produce one device of each kind.
set -euo pipefail

DIR="${1:-var/dev/mis-etc/factory}"
DAYS="${DAYS:-3650}"

mkdir -p "$DIR"

if [ -s "$DIR/root.crt" ]; then
    echo "Factory PKI already present at $DIR"
    exit 0
fi

# --- Root CA ---------------------------------------------------------------

openssl ecparam -name prime256v1 -genkey -noout -out "$DIR/root.key"
openssl req -x509 -new -key "$DIR/root.key" \
    -days "$DAYS" \
    -subj "/CN=Margo PoC Factory Root CA" \
    -addext "basicConstraints=critical,CA:true" \
    -addext "keyUsage=critical,keyCertSign,cRLSign" \
    -out "$DIR/root.crt"

# --- Device 01 (ECDSA P-256 / ES256) --------------------------------------

openssl ecparam -name prime256v1 -genkey -noout -out "$DIR/device-01.key"
openssl req -new -key "$DIR/device-01.key" \
    -subj "/CN=margo-poc-device-01/O=Margo PoC Factory" \
    -out "$DIR/device-01.csr"
openssl x509 -req -in "$DIR/device-01.csr" \
    -CA "$DIR/root.crt" -CAkey "$DIR/root.key" -CAcreateserial \
    -days "$DAYS" \
    -extfile <(printf "basicConstraints=CA:false\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=clientAuth\n") \
    -out "$DIR/device-01.crt"
rm -f "$DIR/device-01.csr"

# --- Device 02 (RSA-PSS 3072 / PS256) --------------------------------------

openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 -out "$DIR/device-02.key"
openssl req -new -key "$DIR/device-02.key" \
    -subj "/CN=margo-poc-device-02/O=Margo PoC Factory" \
    -out "$DIR/device-02.csr"
openssl x509 -req -in "$DIR/device-02.csr" \
    -CA "$DIR/root.crt" -CAkey "$DIR/root.key" -CAcreateserial \
    -days "$DAYS" \
    -extfile <(printf "basicConstraints=CA:false\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=clientAuth\n") \
    -out "$DIR/device-02.crt"
rm -f "$DIR/device-02.csr"

# --- Untrusted root + leaf (negative test material) -----------------------

openssl ecparam -name prime256v1 -genkey -noout -out "$DIR/untrusted-root.key"
openssl req -x509 -new -key "$DIR/untrusted-root.key" \
    -days "$DAYS" \
    -subj "/CN=Rogue Factory Root" \
    -addext "basicConstraints=critical,CA:true" \
    -addext "keyUsage=critical,keyCertSign,cRLSign" \
    -out "$DIR/untrusted-root.crt"

openssl ecparam -name prime256v1 -genkey -noout -out "$DIR/untrusted.key"
openssl req -new -key "$DIR/untrusted.key" \
    -subj "/CN=rogue-device/O=Rogue" \
    -out "$DIR/untrusted.csr"
openssl x509 -req -in "$DIR/untrusted.csr" \
    -CA "$DIR/untrusted-root.crt" -CAkey "$DIR/untrusted-root.key" -CAcreateserial \
    -days "$DAYS" \
    -extfile <(printf "basicConstraints=CA:false\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=clientAuth\n") \
    -out "$DIR/untrusted.crt"
rm -f "$DIR/untrusted.csr"

chmod 600 "$DIR"/*.key

echo "Factory PKI written to $DIR"
ls -l "$DIR"
