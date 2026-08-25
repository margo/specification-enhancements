package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/identity"
)

// RevokedSVIDs is an identity.RevocationQueue backed by the unified store.
type RevokedSVIDs struct {
	db *sql.DB
}

// RevokedSVIDs returns the adapter.
func (s *Store) RevokedSVIDs() *RevokedSVIDs { return &RevokedSVIDs{db: s.db} }

// Compile-time interface check.
var _ identity.RevocationQueue = (*RevokedSVIDs)(nil)

func (r *RevokedSVIDs) Revoke(ctx context.Context, serialHex, fingerprintHex, reason string, notAfter, now time.Time) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO revoked_svids (serial_hex, fingerprint_hex, revoked_at, reason, not_after)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(serial_hex) DO NOTHING
	`, serialHex, fingerprintHex,
		now.UTC().Format(time.RFC3339Nano),
		reason,
		notAfter.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return false, fmt.Errorf("revoked_svids: revoke: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func (r *RevokedSVIDs) List(ctx context.Context, now time.Time) ([]identity.RevocationEntry, time.Time, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT serial_hex, fingerprint_hex, revoked_at, reason, not_after
		  FROM revoked_svids
		 WHERE not_after > ?
		 ORDER BY revoked_at ASC, serial_hex ASC
	`, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("revoked_svids: list: %w", err)
	}
	defer rows.Close()

	var out []identity.RevocationEntry
	var latest time.Time
	for rows.Next() {
		var e identity.RevocationEntry
		var revokedAt, notAfter string
		if err := rows.Scan(&e.SerialHex, &e.FingerprintHex, &revokedAt, &e.Reason, &notAfter); err != nil {
			return nil, time.Time{}, err
		}
		e.RevokedAt = mustParseTime(revokedAt)
		e.NotAfter = mustParseTime(notAfter)
		if e.RevokedAt.After(latest) {
			latest = e.RevokedAt
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, time.Time{}, err
	}
	return out, latest, nil
}

func (r *RevokedSVIDs) PruneExpired(ctx context.Context, now time.Time) (int, error) {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM revoked_svids WHERE not_after <= ?
	`, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("revoked_svids: prune: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
