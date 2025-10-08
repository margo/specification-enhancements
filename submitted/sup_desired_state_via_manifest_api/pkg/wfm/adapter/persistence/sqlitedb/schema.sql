CREATE TABLE IF NOT EXISTS devices (
    id TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS application_deployments (
    id TEXT NOT NULL,
    descriptor_digest TEXT NOT NULL,
    device_id TEXT NOT NULL,
    PRIMARY KEY (device_id, id),
    FOREIGN KEY (device_id)
        REFERENCES devices (id),
    FOREIGN KEY (descriptor_digest)
        REFERENCES deployment_blobs (digest)
);

CREATE TABLE IF NOT EXISTS application_deployment_manifests (
    device_id TEXT PRIMARY KEY,
    version INTEGER DEFAULT 1 NOT NULL,
    bundle_digest TEXT,
    FOREIGN KEY (device_id)
        REFERENCES devices (id),
    FOREIGN KEY (bundle_digest)
        REFERENCES bundle_blobs (digest)
);

CREATE TABLE IF NOT EXISTS deployment_blobs (
    digest TEXT PRIMARY KEY,
    descriptor BLOB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS bundle_blobs (
    digest TEXT PRIMARY KEY,
    archive BLOB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- Seed database
INSERT OR IGNORE INTO devices(id) VALUES ('c92cb339-c99c-4eca-9dd4-f8484dd16cfb')