package tokens

import (
	"context"
	"time"
)

// Store is the persistence contract for enrollment tokens.
// All methods MUST be safe for concurrent use.
type Store interface {
	// Create inserts a fresh row. Returns the token_id assigned and the
	// wire-format secret to print to the operator.
	Create(ctx context.Context, ttl time.Duration, now time.Time) (Created, error)

	// Consume atomically claims an active token by secret hash.
	// Possible errors: ErrNotFound, ErrExpired, ErrAlreadyUsed.
	Consume(ctx context.Context, secretHash []byte, now time.Time) (tokenID string, err error)

	// FindConsumed returns the outcome cached for a previously consumed
	// token, iff it is within the retry window. ErrNotFound otherwise.
	FindConsumed(ctx context.Context, secretHash []byte, now time.Time) (tokenID string, outcome Outcome, err error)

	// RecordOutcome stores the enrollment outcome. Called exactly once
	// after a successful Service.Enroll.
	RecordOutcome(ctx context.Context, outcome Outcome) error
}

// Created is the output of Create - the token_id (for audit) plus the
// one-time-display secret string.
type Created struct {
	TokenID   string
	Secret    Secret
	ExpiresAt time.Time
}
