-- name: GetDeviceId :one
SELECT id FROM devices
WHERE id = ?;

-- name: GetDeploymentsByDeviceId :many
SELECT d.id, b.descriptor, d.descriptor_digest, d.device_id
FROM application_deployments d
JOIN deployment_blobs b ON b.digest = d.descriptor_digest
WHERE d.device_id = ?
ORDER BY d.id;

-- name: GetDeploymentByIdAndDigest :one
SELECT d.id, b.descriptor, d.descriptor_digest, d.device_id
FROM application_deployments d
JOIN deployment_blobs b ON b.digest = d.descriptor_digest
WHERE d.device_id = ? AND d.id = ? AND d.descriptor_digest = ?;

-- name: UpsertDeployment :exec
INSERT INTO application_deployments (
    id, descriptor_digest, device_id
) VALUES (
    ?, ?, ?
)
ON CONFLICT (device_id, id) 
DO UPDATE SET
    descriptor_digest = excluded.descriptor_digest;

-- name: DeleteDeployment :exec
DELETE FROM application_deployments
WHERE device_id = ? AND id = ?;

-- name: GetManifestByDeviceId :one
SELECT device_id, version, bundle_digest FROM application_deployment_manifests
WHERE device_id = ?;

-- name: UpsertManifest :exec
INSERT INTO application_deployment_manifests (
    version, bundle_digest, device_id
) VALUES (
    ?, ?, ?
)
ON CONFLICT (device_id) 
DO UPDATE SET
    version = excluded.version,
    bundle_digest = excluded.bundle_digest,
    device_id = excluded.device_id;

-- name: InsertDeploymentBlob :exec
INSERT INTO deployment_blobs (digest, descriptor)
VALUES (?, ?)
ON CONFLICT(digest) DO NOTHING;

-- name: GetDeploymentBlobByDigest :one
SELECT digest, descriptor, created_at
FROM deployment_blobs
WHERE digest = ?;

-- name: InsertBundleBlob :exec
INSERT INTO bundle_blobs (digest, archive)
VALUES (?, ?)
ON CONFLICT(digest) DO NOTHING;

-- name: GetBundleBlobByDigest :one
SELECT digest, archive, created_at
FROM bundle_blobs
WHERE digest = ?;