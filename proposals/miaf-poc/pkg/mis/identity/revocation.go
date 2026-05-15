package identity

import (
	"context"
	"time"
)

// RevocationEntry is one row in the revocation list surfaced by
// GET /api/v1/revocations. NotAfter is retained for the PruneExpired job so
// the list drops naturally-expired serials; it is not exposed on the wire.
type RevocationEntry struct {
	SerialHex      string // matches issued_svids.serial_hex
	FingerprintHex string // lowercase hex SHA-256 of the leaf DER
	RevokedAt      time.Time
	Reason         string
	NotAfter       time.Time
}

// RevocationQueue is the write side of the revocation list.
type RevocationQueue interface {
	// Revoke is idempotent. It returns (true, nil) when a new row was inserted;
	// (false, nil) when the serial was already revoked. The first revocation
	// wins the timestamp and reason; subsequent calls do not overwrite.
	Revoke(ctx context.Context, serialHex, fingerprintHex, reason string, notAfter, now time.Time) (newly bool, err error)
	// List returns all non-expired revocations (NotAfter > now) plus the max
	// RevokedAt across returned rows. A zero-length slice yields a zero
	// time.Time for lastUpdated - the handler treats a zero time as "never
	// published" and formats Last-Modified accordingly.
	List(ctx context.Context, now time.Time) (entries []RevocationEntry, lastUpdated time.Time, err error)
	// PruneExpired removes rows where NotAfter <= now.
	// Always safe to call concurrently with Revoke / List
	// because the deletes target serials whose NotAfter has elapsed; any
	// subsequent Revoke for that serial will re-insert cleanly.
	PruneExpired(ctx context.Context, now time.Time) (removed int, err error)
}
