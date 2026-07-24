package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/identity"
)

// IssuedSVIDs is an identity.IssuanceLog backed by the unified store.
type IssuedSVIDs struct {
	db *sql.DB
}

// IssuedSVIDs returns the IssuanceLog adapter.
func (s *Store) IssuedSVIDs() *IssuedSVIDs { return &IssuedSVIDs{db: s.db} }

// Compile-time interface check.
var _ identity.IssuanceLog = (*IssuedSVIDs)(nil)

func (r *IssuedSVIDs) Record(ctx context.Context, rec identity.IssuedSVIDRecord) error {
	revoked := 0
	if rec.Revoked {
		revoked = 1
	}
	var revokedAt any
	if !rec.RevokedAt.IsZero() {
		revokedAt = rec.RevokedAt.UTC().Format(time.RFC3339Nano)
	}
	leafDER := rec.LeafDER
	if leafDER == nil {
		leafDER = []byte{}
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO issued_svids
		  (serial_hex, fingerprint_hex, spiffe_id, ldi_uuid, method, issued_at,
		   not_before, not_after, revoked, revoked_at, leaf_der)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(serial_hex) DO UPDATE SET
		  fingerprint_hex = excluded.fingerprint_hex,
		  spiffe_id       = excluded.spiffe_id,
		  ldi_uuid        = excluded.ldi_uuid,
		  method          = excluded.method,
		  issued_at       = excluded.issued_at,
		  not_before      = excluded.not_before,
		  not_after       = excluded.not_after,
		  revoked         = excluded.revoked,
		  revoked_at      = excluded.revoked_at,
		  leaf_der        = excluded.leaf_der
	`,
		rec.SerialHex, rec.FingerprintHex, rec.SPIFFEID, rec.LDIUUID, rec.Method,
		rec.IssuedAt.UTC().Format(time.RFC3339Nano),
		rec.NotBefore.UTC().Format(time.RFC3339Nano),
		rec.NotAfter.UTC().Format(time.RFC3339Nano),
		revoked, revokedAt, leafDER)
	if err != nil {
		return fmt.Errorf("issued_svids: record: %w", err)
	}
	return nil
}

func (r *IssuedSVIDs) ActiveBySPIFFEID(ctx context.Context, spiffeID string, now time.Time) ([]identity.IssuedSVIDRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT serial_hex, fingerprint_hex, spiffe_id, ldi_uuid, method,
		       issued_at, not_before, not_after, revoked, COALESCE(revoked_at, ''),
		       leaf_der
		  FROM issued_svids
		 WHERE spiffe_id = ? AND revoked = 0 AND not_after > ?
		 ORDER BY issued_at ASC
	`, spiffeID, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, fmt.Errorf("issued_svids: active query: %w", err)
	}
	defer rows.Close()

	var out []identity.IssuedSVIDRecord
	for rows.Next() {
		var rec identity.IssuedSVIDRecord
		var issuedAt, notBefore, notAfter, revokedAt string
		var revoked int
		if err := rows.Scan(&rec.SerialHex, &rec.FingerprintHex, &rec.SPIFFEID,
			&rec.LDIUUID, &rec.Method,
			&issuedAt, &notBefore, &notAfter, &revoked, &revokedAt,
			&rec.LeafDER); err != nil {
			return nil, err
		}
		rec.IssuedAt = mustParseTime(issuedAt)
		rec.NotBefore = mustParseTime(notBefore)
		rec.NotAfter = mustParseTime(notAfter)
		rec.Revoked = revoked == 1
		if revokedAt != "" {
			rec.RevokedAt = mustParseTime(revokedAt)
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *IssuedSVIDs) MarkRevoked(ctx context.Context, serialHex string, when time.Time) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE issued_svids SET revoked = 1, revoked_at = ?
		 WHERE serial_hex = ?`,
		when.UTC().Format(time.RFC3339Nano), serialHex)
	if err != nil {
		return fmt.Errorf("issued_svids: mark revoked: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return identity.ErrNotFound
	}
	return nil
}

// LeafByActiveSPIFFEID implements identity.IssuanceLog.
func (r *IssuedSVIDs) LeafByActiveSPIFFEID(ctx context.Context, spiffeID string, now time.Time) (identity.IssuedSVIDRecord, error) {
	const q = `
		SELECT serial_hex, fingerprint_hex, spiffe_id, ldi_uuid, method,
		       issued_at, not_before, not_after, revoked, IFNULL(revoked_at, ''),
		       leaf_der
		  FROM issued_svids
		 WHERE spiffe_id = ?
		   AND revoked = 0
		   AND not_after > ?
		 ORDER BY issued_at DESC
		 LIMIT 1`
	var rec identity.IssuedSVIDRecord
	var issuedAt, notBefore, notAfter, revokedAt string
	var revoked int
	err := r.db.QueryRowContext(ctx, q, spiffeID, now.UTC().Format(time.RFC3339Nano)).Scan(
		&rec.SerialHex, &rec.FingerprintHex, &rec.SPIFFEID, &rec.LDIUUID, &rec.Method,
		&issuedAt, &notBefore, &notAfter, &revoked, &revokedAt,
		&rec.LeafDER,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.IssuedSVIDRecord{}, identity.ErrNotFound
	}
	if err != nil {
		return identity.IssuedSVIDRecord{}, err
	}
	rec.IssuedAt = mustParseTime(issuedAt)
	rec.NotBefore = mustParseTime(notBefore)
	rec.NotAfter = mustParseTime(notAfter)
	rec.Revoked = revoked != 0
	if revokedAt != "" {
		rec.RevokedAt = mustParseTime(revokedAt)
	}
	return rec, nil
}
