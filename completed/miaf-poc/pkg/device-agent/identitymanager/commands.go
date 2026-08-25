package identitymanager

import (
	"context"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
)

// Cmd is the discriminated union the manager goroutine consumes.
// Exactly one of the command fields is non-nil. Each command carries its own
// response channel; the manager closes the channel after writing.
type Cmd struct {
	Ctx     context.Context
	Renew   *RenewCmd
	Refresh *RefreshBundleCmd
	Check   *CheckRevocationsCmd
	JWT     *ExchangeJWTCmd
	Status  *StatusCmd
	Stop    *StopCmd
}

// RenewCmd requests SVID renewal.
type RenewCmd struct {
	Reply chan RenewReply
}

// RenewReply carries the renewal outcome.
type RenewReply struct {
	HTTPStatus int
	NewSerial  string
	NewExpiry  time.Time
	Problem    common.ProblemDetails
	Err        error
}

// RefreshBundleCmd requests a bundle map refresh.
type RefreshBundleCmd struct {
	Reply chan RefreshBundleReply
}

// RefreshBundleReply carries the bundle refresh outcome.
type RefreshBundleReply struct {
	HTTPStatus      int
	NotModified     bool
	BundleLastModif string
	Err             error
}

// CheckRevocationsCmd requests a revocation list check.
type CheckRevocationsCmd struct {
	Reply chan CheckRevocationsReply
}

// CheckRevocationsReply carries the revocations check outcome.
type CheckRevocationsReply struct {
	HTTPStatus    int
	NotModified   bool
	OwnSVIDListed bool
	ETag          string
	LastModified  string
	Err           error
}

// ExchangeJWTCmd requests a JWT SVID exchange.
type ExchangeJWTCmd struct {
	Audiences []string
	TTL       time.Duration
	Reply     chan ExchangeJWTReply
}

// ExchangeJWTReply carries the JWT exchange outcome.
type ExchangeJWTReply struct {
	HTTPStatus int
	JWT        string
	ExpiresAt  time.Time
	FromCache  bool
	Problem    common.ProblemDetails
	Err        error
}

// StatusCmd requests a status snapshot.
type StatusCmd struct {
	Reply chan StatusReply
}

// StatusReply is what `device-agent ctl status` reads.
type StatusReply struct {
	SPIFFEID                string
	Serial                  string
	ExpiresAt               time.Time
	LastRenewalAt           time.Time
	LastBundleRefreshAt     time.Time
	LastRevocationsCheckAt  time.Time
	RevokedAt               time.Time
	BundleLastModified      string
	RevocationsETag         string
	RevocationsLastModified string
	JWTCacheSize            int
}

// StopCmd requests graceful shutdown.
type StopCmd struct {
	Reply chan struct{} // closed when shutdown drains
}
