package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/bootstrap/enrollmenttoken/tokens"
)

// EnrollmentTokens is a tokens.Store backed by the unified store.
type EnrollmentTokens struct {
	db *sql.DB
}

// EnrollmentTokens returns the tokens.Store adapter.
func (s *Store) EnrollmentTokens() tokens.Store { return &EnrollmentTokens{db: s.db} }

// Create implements tokens.Store.
func (e *EnrollmentTokens) Create(ctx context.Context, ttl time.Duration, now time.Time) (tokens.Created, error) {
	if ttl <= 0 {
		return tokens.Created{}, fmt.Errorf("sqlite: ttl must be positive, got %s", ttl)
	}
	secret, err := tokens.NewSecret(rand.Reader)
	if err != nil {
		return tokens.Created{}, err
	}
	tokenID := tokens.NewTokenID()
	expiresAt := now.Add(ttl).UTC()
	_, err = e.db.ExecContext(ctx,
		`INSERT INTO enrollment_tokens (token_id, secret_hash, status, expires_at, created_at)
		 VALUES (?, ?, 'active', ?, ?)`,
		tokenID, secret.Hash(),
		expiresAt.Format(time.RFC3339Nano), now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return tokens.Created{}, fmt.Errorf("insert token: %w", err)
	}
	return tokens.Created{TokenID: tokenID, Secret: secret, ExpiresAt: expiresAt}, nil
}

// Consume implements tokens.Store.
func (e *EnrollmentTokens) Consume(ctx context.Context, secretHash []byte, now time.Time) (string, error) {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback() //nolint:errcheck

	var tokenID, status, expiresAtS string
	err = tx.QueryRowContext(ctx,
		`SELECT token_id, status, expires_at FROM enrollment_tokens WHERE secret_hash = ?`,
		secretHash).Scan(&tokenID, &status, &expiresAtS)
	if errors.Is(err, sql.ErrNoRows) {
		return "", tokens.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, expiresAtS)
	if err != nil {
		return "", fmt.Errorf("parse expires_at: %w", err)
	}
	if !now.Before(expiresAt) {
		return "", tokens.ErrExpired
	}
	if status == tokens.StatusConsumed {
		return "", tokens.ErrAlreadyUsed
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE enrollment_tokens SET status='consumed', consumed_at=?
		 WHERE token_id = ? AND status='active'`,
		now.UTC().Format(time.RFC3339Nano), tokenID)
	if err != nil {
		return "", err
	}
	affected, _ := res.RowsAffected()
	if affected != 1 {
		return "", tokens.ErrAlreadyUsed
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return tokenID, nil
}

// FindConsumed implements tokens.Store.
func (e *EnrollmentTokens) FindConsumed(ctx context.Context, secretHash []byte, now time.Time) (string, tokens.Outcome, error) {
	const q = `
		SELECT t.token_id, o.status_code, o.certificate_chain_pem,
		       o.svid_profile_uri, o.csr_pub_key_sha256, o.ldi_uuid,
		       o.completed_at, o.retry_window_ends
		  FROM enrollment_tokens t
		  JOIN enrollment_token_outcomes o ON o.token_id = t.token_id
		 WHERE t.secret_hash = ?
	`
	var (
		tokenID, chainJSON, profileURI, completedAtS, retryEndsS, ldiUUID string
		statusCode                                                        int
		pubHash                                                           []byte
	)
	err := e.db.QueryRowContext(ctx, q, secretHash).Scan(
		&tokenID, &statusCode, &chainJSON, &profileURI, &pubHash, &ldiUUID, &completedAtS, &retryEndsS,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return "", tokens.Outcome{}, tokens.ErrNotFound
	}
	if err != nil {
		return "", tokens.Outcome{}, err
	}
	retryEnds, err := time.Parse(time.RFC3339Nano, retryEndsS)
	if err != nil {
		return "", tokens.Outcome{}, fmt.Errorf("parse retry_window_ends: %w", err)
	}
	if now.After(retryEnds) {
		// Multi-wrap so errors.Is(err, ErrNotFound) still holds for
		// callers that only check the broad sentinel, while the
		// validator can also check errors.Is(err, ErrRetryWindowExpired).
		return "", tokens.Outcome{}, fmt.Errorf("%w: %w", tokens.ErrNotFound, tokens.ErrRetryWindowExpired)
	}
	completedAt, err := time.Parse(time.RFC3339Nano, completedAtS)
	if err != nil {
		return "", tokens.Outcome{}, fmt.Errorf("parse completed_at: %w", err)
	}
	chain, err := decodeChain(chainJSON)
	if err != nil {
		return "", tokens.Outcome{}, err
	}
	return tokenID, tokens.Outcome{
		TokenID:             tokenID,
		StatusCode:          statusCode,
		CertificateChainPEM: chain,
		SVIDProfileURI:      profileURI,
		CSRPubKeySHA256:     pubHash,
		LDIUUID:             ldiUUID,
		CompletedAt:         completedAt,
		RetryWindowEnds:     retryEnds,
	}, nil
}

// RecordOutcome implements tokens.Store.
func (e *EnrollmentTokens) RecordOutcome(ctx context.Context, o tokens.Outcome) error {
	if o.TokenID == "" || o.SVIDProfileURI == "" || len(o.CSRPubKeySHA256) == 0 {
		return fmt.Errorf("sqlite: Outcome fields incomplete: %+v", o)
	}
	chainJSON, err := encodeChain(o.CertificateChainPEM)
	if err != nil {
		return err
	}
	_, err = e.db.ExecContext(ctx,
		`INSERT INTO enrollment_token_outcomes
		 (token_id, status_code, certificate_chain_pem, svid_profile_uri,
		  csr_pub_key_sha256, ldi_uuid, completed_at, retry_window_ends)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		o.TokenID, o.StatusCode, chainJSON, o.SVIDProfileURI,
		o.CSRPubKeySHA256, o.LDIUUID,
		o.CompletedAt.UTC().Format(time.RFC3339Nano),
		o.RetryWindowEnds.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("sqlite: outcome already recorded for token_id=%s: %w", o.TokenID, tokens.ErrOutcomeExists)
		}
		return err
	}
	return nil
}

func encodeChain(pems []string) (string, error) {
	b, err := json.Marshal(pems)
	return string(b), err
}

func decodeChain(s string) ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("decode chain: %w", err)
	}
	return out, nil
}
