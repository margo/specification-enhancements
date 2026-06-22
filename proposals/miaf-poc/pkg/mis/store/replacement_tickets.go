package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/identity"
)

// replacementTicketPrefix is the non-spec wire prefix used by the PoC
// to avoid operator confusion with enrollment-token strings.
const replacementTicketPrefix = "margo-rt-v1."

const replacementTicketSecretLen = 16

// ReplacementTickets is an identity.ReplacementTicketStore backed by
// the unified store.
type ReplacementTickets struct {
	db *sql.DB
}

// ReplacementTickets returns the adapter.
func (s *Store) ReplacementTickets() identity.ReplacementTicketStore {
	return &ReplacementTickets{db: s.db}
}

// Create implements identity.ReplacementTicketStore.
func (r *ReplacementTickets) Create(ctx context.Context, ldiUUID string, ttl time.Duration, now time.Time) (identity.ReplacementTicketCreated, error) {
	if ttl <= 0 {
		return identity.ReplacementTicketCreated{}, fmt.Errorf("replacement_tickets: ttl must be positive, got %s", ttl)
	}
	if ldiUUID == "" {
		return identity.ReplacementTicketCreated{}, fmt.Errorf("replacement_tickets: ldi_uuid required")
	}
	// FK check - reject early with a readable error.
	var dummy string
	err := r.db.QueryRowContext(ctx, `SELECT uuid FROM ldis WHERE uuid = ?`, ldiUUID).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.ReplacementTicketCreated{}, fmt.Errorf("replacement_tickets: no such ldi %q", ldiUUID)
	}
	if err != nil {
		return identity.ReplacementTicketCreated{}, err
	}

	buf := make([]byte, replacementTicketSecretLen)
	if _, err := rand.Read(buf); err != nil {
		return identity.ReplacementTicketCreated{}, fmt.Errorf("replacement_tickets: rand: %w", err)
	}
	wire := replacementTicketPrefix + base64.RawURLEncoding.EncodeToString(buf)
	hash := sha256.Sum256([]byte(wire))
	expiresAt := now.Add(ttl).UTC()

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO replacement_tickets (ticket_hash, ldi_uuid, status, expires_at, created_at)
		 VALUES (?, ?, 'active', ?, ?)`,
		hash[:], ldiUUID,
		expiresAt.Format(time.RFC3339Nano), now.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return identity.ReplacementTicketCreated{}, fmt.Errorf("replacement_tickets: insert: %w", err)
	}
	return identity.ReplacementTicketCreated{
		Ticket: wire, LDIUUID: ldiUUID, ExpiresAt: expiresAt,
	}, nil
}

// Validate implements identity.ReplacementValidator.
func (r *ReplacementTickets) Validate(ctx context.Context, ticket string, now time.Time) (string, error) {
	if !strings.HasPrefix(ticket, replacementTicketPrefix) {
		return "", identity.ErrReplacementNotAuthorized
	}
	hash := sha256.Sum256([]byte(ticket))

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback() //nolint:errcheck

	var ldiUUID, status, expiresAtS string
	err = tx.QueryRowContext(ctx,
		`SELECT ldi_uuid, status, expires_at FROM replacement_tickets WHERE ticket_hash = ?`,
		hash[:]).Scan(&ldiUUID, &status, &expiresAtS)
	if errors.Is(err, sql.ErrNoRows) {
		return "", identity.ErrReplacementNotAuthorized
	}
	if err != nil {
		return "", err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, expiresAtS)
	if err != nil {
		return "", fmt.Errorf("replacement_tickets: parse expires_at: %w", err)
	}
	if !now.Before(expiresAt) || status != "active" {
		return "", identity.ErrReplacementNotAuthorized
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE replacement_tickets SET status = 'consumed', consumed_at = ?
		 WHERE ticket_hash = ? AND status = 'active'`,
		now.UTC().Format(time.RFC3339Nano), hash[:])
	if err != nil {
		return "", err
	}
	affected, _ := res.RowsAffected()
	if affected != 1 {
		return "", identity.ErrReplacementNotAuthorized
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return ldiUUID, nil
}
