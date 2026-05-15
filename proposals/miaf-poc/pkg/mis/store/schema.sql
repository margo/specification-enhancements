-- MIS unified state.

-- Identity: LDI records and their ESI bindings (SUP §4, §5).
CREATE TABLE IF NOT EXISTS ldis (
    uuid         TEXT    PRIMARY KEY NOT NULL,
    spiffe_id    TEXT    NOT NULL UNIQUE,
    status       TEXT    NOT NULL CHECK (status IN ('active','revoked','retired')),
    created_at   TEXT    NOT NULL,
    updated_at   TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS esi_bindings (
    esi          BLOB    NOT NULL,
    ldi_uuid     TEXT    NOT NULL REFERENCES ldis(uuid) ON DELETE RESTRICT,
    method       TEXT    NOT NULL,
    status       TEXT    NOT NULL CHECK (status IN ('active','retired')),
    bound_at     TEXT    NOT NULL,
    retired_at   TEXT,
    PRIMARY KEY (esi, ldi_uuid)
);
CREATE UNIQUE INDEX IF NOT EXISTS esi_bindings_active_per_ldi
    ON esi_bindings (ldi_uuid) WHERE status = 'active';
CREATE UNIQUE INDEX IF NOT EXISTS esi_bindings_esi_unique
    ON esi_bindings (esi);

-- Enrollment tokens (SUP §Appendix A - Enrollment Token Method).
CREATE TABLE IF NOT EXISTS enrollment_tokens (
    token_id         TEXT    PRIMARY KEY NOT NULL,
    secret_hash      BLOB    NOT NULL UNIQUE,
    status           TEXT    NOT NULL CHECK (status IN ('active','consumed')),
    expires_at       TEXT    NOT NULL,
    created_at       TEXT    NOT NULL,
    consumed_at      TEXT
);

CREATE TABLE IF NOT EXISTS enrollment_token_outcomes (
    token_id               TEXT    PRIMARY KEY NOT NULL REFERENCES enrollment_tokens(token_id) ON DELETE CASCADE,
    status_code            INTEGER NOT NULL,
    certificate_chain_pem  TEXT    NOT NULL,
    svid_profile_uri       TEXT    NOT NULL,
    csr_pub_key_sha256     BLOB    NOT NULL,
    ldi_uuid               TEXT    NOT NULL,
    completed_at           TEXT    NOT NULL,
    retry_window_ends      TEXT    NOT NULL
);

-- JWT-assertion jti replay (SUP §Appendix A Signed-Assertion common reqs).
CREATE TABLE IF NOT EXISTS jwt_replay (
    jti          TEXT    PRIMARY KEY NOT NULL,
    expires_at   TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS jwt_replay_expires_at ON jwt_replay (expires_at);

-- Replacement tickets (SUP §4 Device replacement, urn:margo:replacement-auth:operator-ticket:v1).
-- Secrets are never stored; ticket_hash = SHA-256(ticket wire string).
CREATE TABLE IF NOT EXISTS replacement_tickets (
    ticket_hash   BLOB    PRIMARY KEY NOT NULL,
    ldi_uuid      TEXT    NOT NULL REFERENCES ldis(uuid) ON DELETE RESTRICT,
    status        TEXT    NOT NULL CHECK (status IN ('active','consumed')),
    expires_at    TEXT    NOT NULL,
    created_at    TEXT    NOT NULL,
    consumed_at   TEXT
);

-- Per-SPIFFE-ID renewal success log (SUP §"MIS Renewal Rate-Limiting").
-- The renewal handler inserts one row per successful issuance; the rate
-- limiter counts rows within a sliding window.
CREATE TABLE IF NOT EXISTS renewal_events (
    spiffe_id    TEXT    NOT NULL,
    recorded_at  TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS renewal_events_spiffe_time
    ON renewal_events (spiffe_id, recorded_at);

-- Issuance audit . One row per leaf SVID MIS has ever signed.
-- Serial is uppercase-hex of the X.509 serial (matches the wire format in
-- `/api/v1/revocations`). Fingerprint is lowercase-hex SHA-256 of the leaf DER.
CREATE TABLE IF NOT EXISTS issued_svids (
    serial_hex       TEXT    PRIMARY KEY NOT NULL,
    fingerprint_hex  TEXT    NOT NULL UNIQUE,
    spiffe_id        TEXT    NOT NULL,
    ldi_uuid         TEXT    NOT NULL REFERENCES ldis(uuid) ON DELETE RESTRICT,
    method           TEXT    NOT NULL,
    issued_at        TEXT    NOT NULL,
    not_before       TEXT    NOT NULL,
    not_after        TEXT    NOT NULL,
    revoked          INTEGER NOT NULL DEFAULT 0 CHECK (revoked IN (0,1)),
    revoked_at       TEXT,
    leaf_der         BLOB    NOT NULL DEFAULT (X'')
);
CREATE INDEX IF NOT EXISTS issued_svids_spiffe_id      ON issued_svids (spiffe_id);
CREATE INDEX IF NOT EXISTS issued_svids_ldi_uuid       ON issued_svids (ldi_uuid);
CREATE INDEX IF NOT EXISTS issued_svids_not_after      ON issued_svids (not_after);

-- Revocation list (SUP §"Revocation List Endpoint"). One row per revoked serial.
-- Deliberately independent of issued_svids so PruneExpired can drop audit rows
-- without destroying revocation facts; and so the admin endpoint can revoke a
-- serial imported from outside the issuance audit.
CREATE TABLE IF NOT EXISTS revoked_svids (
    serial_hex       TEXT    PRIMARY KEY NOT NULL,
    fingerprint_hex  TEXT    NOT NULL,
    revoked_at       TEXT    NOT NULL,
    reason           TEXT    NOT NULL DEFAULT '',
    not_after        TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS revoked_svids_not_after ON revoked_svids (not_after);

-- Issued JWT SVIDs audit. JWT SVIDs are not individually revocable
-- (SUP §"Revocation Model" point 1) but the audit answers "did MIS issue a
-- token of this jti?" Operators can use it for incident response.
CREATE TABLE IF NOT EXISTS issued_jwt_svids (
    jti          TEXT    PRIMARY KEY NOT NULL,
    spiffe_id    TEXT    NOT NULL,
    audiences    TEXT    NOT NULL,            -- JSON array
    issued_at    TEXT    NOT NULL,
    expires_at   TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS issued_jwt_svids_spiffe_id ON issued_jwt_svids (spiffe_id);
CREATE INDEX IF NOT EXISTS issued_jwt_svids_expires_at ON issued_jwt_svids (expires_at);
