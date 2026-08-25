package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/identity"
)

// IssuedJWTSVIDs is an identity.JWTSVIDIssuanceLog backed by the unified store.
type IssuedJWTSVIDs struct{ db *sql.DB }

// IssuedJWTSVIDs returns the adapter.
func (s *Store) IssuedJWTSVIDs() *IssuedJWTSVIDs { return &IssuedJWTSVIDs{db: s.db} }

// Compile-time check.
var _ identity.JWTSVIDIssuanceLog = (*IssuedJWTSVIDs)(nil)

// Record implements identity.JWTSVIDIssuanceLog.
func (l *IssuedJWTSVIDs) Record(ctx context.Context, rec identity.IssuedJWTSVIDRecord) error {
	audJSON, err := json.Marshal(rec.Audiences)
	if err != nil {
		return fmt.Errorf("issued_jwt_svids: marshal audiences: %w", err)
	}
	_, err = l.db.ExecContext(ctx, `
		INSERT INTO issued_jwt_svids (jti, spiffe_id, audiences, issued_at, expires_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(jti) DO NOTHING`,
		rec.JTI, rec.SPIFFEID, string(audJSON),
		rec.IssuedAt.UTC().Format(time.RFC3339Nano),
		rec.ExpiresAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("issued_jwt_svids: insert: %w", err)
	}
	return nil
}
