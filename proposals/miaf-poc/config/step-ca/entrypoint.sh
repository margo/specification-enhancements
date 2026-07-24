#!/bin/bash
# Custom step-ca entrypoint for the MIAF PoC.
#
# Replaces the stock image entrypoint to perform two extra init-time actions
# after `step ca init` runs:
#
#   1. Register a custom X.509 template on the `margo-mis` JWK provisioner that
#      reads URI SANs from the OTT top-level `sans` claim (the designed JWK
#      provisioner mechanism). step-ca converts `["spiffe://..."]` strings via
#      CreateSANs, which auto-detects the URI scheme and yields typed SANs.
#      The template uses `{{ toJson .SANs }}` which serialises those as the
#      `{type,value}` objects the Certificate template parser expects. This lets
#      MIS (acting as an RA) issue device SVIDs whose URI SAN is set
#      authoritatively by MIS and not by the device's CSR, per MIAF SUP §5.
#
#   2. Export the `margo-mis` provisioner block (public JWK + encrypted private
#      JWK) from ca.json into /cred-out/provisioner.json and copy the password
#      into /cred-out/password. These artefacts live on a dedicated volume
#      that is mounted into the MIS container ONLY in RA mode. MIS uses the
#      password to decrypt the private JWK at startup and then mints OTTs to
#      authenticate to /1.0/sign.
#
# Both actions are idempotent: if ca.json already names the custom template
# and provisioner.json already exists on the credential share, they're skipped.
#
# The script also duplicates the upstream entrypoint's responsibilities for
# running `step ca init` on first start and finally exec'ing step-ca.
#
# NOTE: docker-compose runs this container as root (user: "0") so that the
# credential export can write to and chown files on the step-ca-cred-share
# volume (which is owned by root at creation time).
set -eo pipefail

export STEPPATH="${STEPPATH:-$(step path)}"
CONFIGPATH="${CONFIGPATH:-${STEPPATH}/config/ca.json}"
PWDPATH="${PWDPATH:-${STEPPATH}/secrets/password}"
CRED_OUT="${CRED_OUT:-/cred-out}"

MIS_UID="${MIS_UID:-65532}"
MIS_GID="${MIS_GID:-65532}"

function init_if_needed() {
    if [ -f "${CONFIGPATH}" ]; then
        return 0
    fi

    : "${DOCKER_STEPCA_INIT_NAME:?required}"
    : "${DOCKER_STEPCA_INIT_DNS_NAMES:?required}"
    : "${DOCKER_STEPCA_INIT_PASSWORD_FILE:?required}"
    local PROVISIONER_NAME="${DOCKER_STEPCA_INIT_PROVISIONER_NAME:-admin}"
    local ADDRESS="${DOCKER_STEPCA_INIT_ADDRESS:-:9000}"

    cat < "${DOCKER_STEPCA_INIT_PASSWORD_FILE}" > "${STEPPATH}/password"
    cat < "${DOCKER_STEPCA_INIT_PASSWORD_FILE}" > "${STEPPATH}/provisioner_password"

    step ca init \
        --name "${DOCKER_STEPCA_INIT_NAME}" \
        --dns  "${DOCKER_STEPCA_INIT_DNS_NAMES}" \
        --address "${ADDRESS}" \
        --provisioner "${PROVISIONER_NAME}" \
        --password-file "${STEPPATH}/password" \
        --provisioner-password-file "${STEPPATH}/provisioner_password" \
        --deployment-type standalone \
        --no-db

    shred -u "${STEPPATH}/provisioner_password" 2>/dev/null || rm -f "${STEPPATH}/provisioner_password"
    mv "${STEPPATH}/password" "${PWDPATH}"
}

function apply_ra_template() {
    local PROVISIONER_NAME="${DOCKER_STEPCA_INIT_PROVISIONER_NAME:-admin}"
    if jq -e --arg name "${PROVISIONER_NAME}" \
        '.authority.provisioners[] | select(.name==$name) | .options.x509.template' \
        "${CONFIGPATH}" >/dev/null 2>&1; then
        echo "RA X.509 template already present on provisioner '${PROVISIONER_NAME}'; skipping."
        return 0
    fi

    cat > /tmp/mis-ra.tpl << 'ENDTPL'
{
    "subject": {},
    "sans": {{ toJson .SANs }},
    "keyUsage": ["digitalSignature"],
    "extKeyUsage": ["serverAuth", "clientAuth"]
}
ENDTPL

    cat > /tmp/mis-ra-policy.json << 'ENDPOL'
{
    "x509": {
        "allow": {
            "uris": ["margo.example.com"]
        }
    }
}
ENDPOL

    step ca provisioner update "${PROVISIONER_NAME}" \
        --x509-template /tmp/mis-ra.tpl \
        --x509-default-dur 24h \
        --x509-min-dur 1m \
        --x509-max-dur 43800h \
        --ca-config "${CONFIGPATH}"

    local tmp
    tmp="$(mktemp)"
    jq --arg name "${PROVISIONER_NAME}" --slurpfile pol /tmp/mis-ra-policy.json \
        '(.authority.provisioners[] | select(.name==$name) | .options.x509.policy) = $pol[0]' \
        "${CONFIGPATH}" > "$tmp"
    mv "$tmp" "${CONFIGPATH}"

    echo "RA X.509 template + URI policy applied to provisioner '${PROVISIONER_NAME}'."
}

function export_ra_credential() {
    local PROVISIONER_NAME="${DOCKER_STEPCA_INIT_PROVISIONER_NAME:-admin}"
    mkdir -p "${CRED_OUT}"

    if [ -s "${CRED_OUT}/provisioner.json" ] && [ -s "${CRED_OUT}/password" ] && [ -s "${CRED_OUT}/root.crt" ]; then
        echo "RA credential already exported to ${CRED_OUT}; skipping."
        return 0
    fi

    jq --arg name "${PROVISIONER_NAME}" \
        '.authority.provisioners[] | select(.name==$name)' \
        "${CONFIGPATH}" > "${CRED_OUT}/provisioner.json"

    cat < "${DOCKER_STEPCA_INIT_PASSWORD_FILE}" > "${CRED_OUT}/password"
    cp "${STEPPATH}/certs/root_ca.crt" "${CRED_OUT}/root.crt"

    chown -R "${MIS_UID}:${MIS_GID}" "${CRED_OUT}"
    chmod 0640 "${CRED_OUT}/provisioner.json" "${CRED_OUT}/password"
    chmod 0644 "${CRED_OUT}/root.crt"

    echo "Exported RA credential to ${CRED_OUT}:"
    ls -l "${CRED_OUT}"
}

init_if_needed
apply_ra_template
export_ra_credential

exec "$@"
