package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertjwt/replay"
)

// JWTReplayStore is a replay.Store backed by the unified store.
// Entries older than their expires_at are treated as absent and may be
// overwritten by a new Witness call.
type JWTReplayStore struct {
	db *sql.DB
}

// JWTReplay returns the replay.Store adapter.
func (s *Store) JWTReplay() replay.Store { return &JWTReplayStore{db: s.db} }

// Witness implements replay.Store.
func (r *JWTReplayStore) Witness(ctx context.Context, jti string, expiresAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	var prevExpS string
	err = tx.QueryRowContext(ctx,
		`SELECT expires_at FROM jwt_replay WHERE jti = ?`, jti).Scan(&prevExpS)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// unseen => insert.
	case err != nil:
		return fmt.Errorf("jwt_replay lookup: %w", err)
	default:
		prevExp, perr := time.Parse(time.RFC3339Nano, prevExpS)
		if perr != nil {
			return fmt.Errorf("jwt_replay parse prev expiry: %w", perr)
		}
		if prevExp.After(time.Now()) {
			return replay.ErrReplay
		}
		// stale - fall through to UPSERT.
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO jwt_replay (jti, expires_at) VALUES (?, ?)
		   ON CONFLICT(jti) DO UPDATE SET expires_at = excluded.expires_at`,
		jti, expiresAt.UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("jwt_replay upsert: %w", err)
	}
	return tx.Commit()
}
