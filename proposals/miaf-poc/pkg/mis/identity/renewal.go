package identity

import (
	"context"
	"errors"
	"time"
)

// ErrRenewalRateLimited signals the caller exceeded the SUP §"MIS Renewal
// Rate-Limiting and Backoff Policy" frequency budget. Handlers map this to
// 429 too-many-requests with a Retry-After header.
var ErrRenewalRateLimited = errors.New("identity: renewal rate limit exceeded")

// RenewalRateLimiter tracks successful renewals per SPIFFE ID and decides
// whether a new renewal is allowed under the configured window.
//
// Allow reports whether a renewal for the given SPIFFE ID is allowed right
// now. When not allowed, retryAfter is the delta-duration until the earliest
// recorded event in the current window ages out (always > 0).
//
// Record appends a success event. Callers invoke it ONLY after the issuer
// has produced a new SVID; failed attempts MUST NOT consume budget.
type RenewalRateLimiter interface {
	Allow(ctx context.Context, spiffeID string, now time.Time) (allowed bool, retryAfter time.Duration, err error)
	Record(ctx context.Context, spiffeID string, now time.Time) error
}
