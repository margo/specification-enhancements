#!/bin/sh
# One-shot: on every stack start, ensure the SPIRE Agent has a join token and
# trust bundle on the shared volume, and that the spire-workload entry is
# registered. All operations are idempotent so `docker compose up` can run
# repeatedly without duplicating state.
#
# SPIRE 1.10+ reserves the /spire/ path prefix for SPIRE's own agent IDs
# (spire/agent/join_token/<token>). Custom workload SPIFFE IDs must therefore
# use a different path prefix - we use /margo/spire-workload/ to keep the
# namespace disjoint from MIS's /margo/device/ path.
set -eu

BOOTSTRAP_DIR="/run/spire/agent/bootstrap"
TOKEN_FILE="$BOOTSTRAP_DIR/join-token"
BUNDLE_FILE="$BOOTSTRAP_DIR/bundle.pem"

TRUST_DOMAIN="margo.example.com"
WORKLOAD_SPIFFE_ID="spiffe://${TRUST_DOMAIN}/margo/spire-workload/echo"
# The spire-workload binary runs as uid 65532 (distroless "nonroot"); the unix
# WorkloadAttestor matches by the caller's effective UID from SO_PEERCRED.
WORKLOAD_SELECTOR="unix:uid:65532"

SPIRE_SERVER_CLI="/opt/spire/bin/spire-server"
SOCKET_FLAG="-socketPath=/tmp/spire-server/private/api.sock"

mkdir -p "$BOOTSTRAP_DIR"

# Bundle: refresh on every run (cheap; keeps agent aligned with server).
$SPIRE_SERVER_CLI bundle show $SOCKET_FLAG -format pem > "$BUNDLE_FILE"

# Join token: generate only if missing. Tokens are one-time-use; regenerating
# would invalidate an already-attested agent after a partial restart.
if [ ! -s "$TOKEN_FILE" ]; then
    echo "Generating join token..."
    TOKEN=$($SPIRE_SERVER_CLI token generate $SOCKET_FLAG \
        -ttl 3600 \
        -output json \
        | jq -r '.value')
    echo "$TOKEN" > "$TOKEN_FILE"
    chmod 0600 "$TOKEN_FILE"
    echo "Join token written to $TOKEN_FILE"
else
    TOKEN=$(cat "$TOKEN_FILE")
    echo "Join token already present at $TOKEN_FILE; reusing."
fi

# With the join_token NodeAttestor, SPIRE automatically assigns the agent the
# SPIFFE ID spiffe://<td>/spire/agent/join_token/<token-value>.
AGENT_SPIFFE_ID="spiffe://${TRUST_DOMAIN}/spire/agent/join_token/${TOKEN}"
echo "Agent SPIFFE ID: $AGENT_SPIFFE_ID"

# Workload entry: create only if the selector-parent-id-spiffe-id triple isn't
# already present.
existing="$($SPIRE_SERVER_CLI entry show $SOCKET_FLAG \
    -spiffeID "$WORKLOAD_SPIFFE_ID" \
    -parentID "$AGENT_SPIFFE_ID" \
    -selector "$WORKLOAD_SELECTOR" \
    2>/dev/null || true)"
if echo "$existing" | grep -q 'SPIFFE ID'; then
    echo "Workload entry already registered; skipping."
else
    echo "Registering workload entry ${WORKLOAD_SPIFFE_ID}..."
    $SPIRE_SERVER_CLI entry create $SOCKET_FLAG \
        -spiffeID "$WORKLOAD_SPIFFE_ID" \
        -parentID "$AGENT_SPIFFE_ID" \
        -selector "$WORKLOAD_SELECTOR" \
        -ttl 3600
fi

echo "Bootstrap complete:"
ls -l "$BOOTSTRAP_DIR"
$SPIRE_SERVER_CLI entry show $SOCKET_FLAG
