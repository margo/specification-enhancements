package identity

import (
	"context"
	"errors"
	"time"
)

// ErrReplacementNotAuthorized is returned when the presented replacement
// ticket is unknown, expired, or already used. Mapped to Appendix B
// "replacement-not-authorized" (403) by the enrollment handler.
var ErrReplacementNotAuthorized = errors.New("identity: replacement ticket not authorized")

// ReplacementValidator authorises rebinding a new ESI to an existing
// LDI.
type ReplacementValidator interface {
	// Validate consumes a ticket. On success it returns the target LDI
	// UUID. On failure it returns ErrReplacementNotAuthorized.
	Validate(ctx context.Context, ticket string, now time.Time) (string, error)
}

// ReplacementTicketStore is the superset interface that adds creation.
type ReplacementTicketStore interface {
	ReplacementValidator
	// Create mints a fresh single-use ticket authorising rebinding of
	// ldiUUID within ttl. The wire-format ticket is returned once
	// (never stored in plain).
	Create(ctx context.Context, ldiUUID string, ttl time.Duration, now time.Time) (ReplacementTicketCreated, error)
}

// ReplacementTicketCreated is the output of Create.
type ReplacementTicketCreated struct {
	Ticket    string // wire form - display to operator once
	LDIUUID   string
	ExpiresAt time.Time
}

// NoReplacements is a ReplacementValidator that rejects every ticket.
// Wire it into identity.NewService when no replacement flow is enabled.
type NoReplacements struct{}

func (NoReplacements) Validate(context.Context, string, time.Time) (string, error) {
	return "", ErrReplacementNotAuthorized
}
