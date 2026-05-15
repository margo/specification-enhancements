// Package bootstrap defines the interface every MIAF bootstrap method plugs into.
//
// Each concrete method (factory-cert-mtls, factory-cert-jwt, enrollment-token,
// fdo) lives in its own sub-package and implements Validator. The enrollment
// handler looks up the right Validator by bootstrapCredential.method URN and
// delegates proof validation + ESI derivation.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"

	"github.com/margo/miaf-poc/pkg/common"
)

// ErrUnsupportedMethod is returned by Registry.Lookup when the requested
// method is not registered. Handlers map this to SUP Appendix B
// "unsupported-method" (422).
var ErrUnsupportedMethod = errors.New("bootstrap: unsupported method")

// VerifiedBootstrap is the output of a successful proof validation.
type VerifiedBootstrap struct {
	// Method is the URN that produced this result (echoed for audit).
	Method string
	// ESI is the method-derived Enrollment Subject Identifier, raw bytes.
	// factory-cert-mtls: SHA-256 of the DER leaf cert.
	// factory-cert-jwt:  SHA-256 of x5c[0].
	// enrollment-token:  SHA-256 of token_id.
	ESI []byte
	// Evidence holds method-specific audit material (e.g., the DER of the
	// validated factory leaf). Opaque to the handler.
	Evidence any
}

// CachedResponse holds a previously computed enrollment response for replay.
type CachedResponse struct {
	Response   *common.EnrollmentResponseDTO
	StatusCode int
}

// PendingEnrollment is the validated bootstrap state ready for Service.Enroll.
// Commit, if non-nil, is called by the enrollment handler after Service.Enroll
// succeeds to persist the outcome for bounded idempotent-retry recognition.
type PendingEnrollment struct {
	VB     VerifiedBootstrap
	Commit func(ctx context.Context, resp *common.EnrollmentResponseDTO, statusCode int, ldiUUID string) error
}

// ValidationOutcome is returned by Validator.Validate.
// Exactly one of Replay and Pending is non-nil on success.
type ValidationOutcome struct {
	// Replay is non-nil when a previous identical enrollment succeeded within
	// the retry window. The handler must return this response immediately
	// without calling Service.Enroll.
	Replay *CachedResponse
	// Pending is non-nil when the credential was validated and enrollment
	// should proceed. Its Commit field is called after Service.Enroll succeeds.
	Pending *PendingEnrollment
}

// Validator turns a method-specific proof into a ValidationOutcome.
// If a replay is detected, Replay is set and the handler returns early.
// Otherwise Pending carries the VerifiedBootstrap and an optional post-enroll
// Commit hook; both phases are always invoked uniformly, no type assertion needed.
type Validator interface {
	Validate(ctx context.Context, r *http.Request, cred common.BootstrapCredentialDTO, csrB64, svidProfileURI string) (ValidationOutcome, error)
}

// Registry holds the set of active Validators keyed by their method URN.
type Registry struct {
	mu  sync.RWMutex
	byM map[string]Validator
}

// NewRegistry constructs an empty Registry.
func NewRegistry() *Registry {
	return &Registry{byM: map[string]Validator{}}
}

// Register installs or replaces the validator for the given method URN.
// Calling Register for the same URN twice overrides the earlier binding -
// callers (cmd/mis) register exactly once at startup.
func (r *Registry) Register(method string, v Validator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byM[method] = v
}

// Lookup returns the validator for method or ErrUnsupportedMethod.
func (r *Registry) Lookup(method string) (Validator, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.byM[method]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedMethod, method)
	}
	return v, nil
}

// Methods returns the registered method URNs in sorted order, suitable for
// advertising in the MIAF discovery document.
func (r *Registry) Methods() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.byM))
	for m := range r.byM {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}
