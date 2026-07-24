// Package factorycertmtls implements the enrollment-token bootstrap
// method defined in SUP Appendix A.
package enrollmenttoken

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/enrollmenttoken/tokens"
)

// MaxRetryWindow is the upper bound SUP Appendix A imposes on the
// idempotent-retry recognition window ("MUST NOT exceed 15 minutes").
const MaxRetryWindow = 15 * time.Minute

// DefaultRetryWindow is the PoC default, matching the SUP "SHOULD default
// to 5 minutes".
const DefaultRetryWindow = 5 * time.Minute

// Error sentinels.
var (
	ErrMissingToken = errors.New("enrollment-token: bootstrapCredential.proof.token is required")
	ErrInvalidToken = errors.New("enrollment-token: token unknown, expired, or already consumed")
)

// Config configures Validator.
type Config struct {
	Store       tokens.Store
	RetryWindow time.Duration
	Clock       func() time.Time
}

// Validator implements bootstrap.Validator for the enrollment-token method.
type Validator struct {
	cfg Config
}

// Evidence is attached to the VerifiedBootstrap so the handler can thread
// the token_id through to Commit without re-parsing the request.
type Evidence struct {
	TokenID    string
	SecretHash []byte
}

// New constructs a Validator. Panics if required config is missing.
func New(cfg Config) *Validator {
	if cfg.Store == nil {
		panic("enrollmenttoken: Config.Store is required")
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	if cfg.RetryWindow <= 0 {
		cfg.RetryWindow = DefaultRetryWindow
	}
	if cfg.RetryWindow > MaxRetryWindow {
		cfg.RetryWindow = MaxRetryWindow
	}
	return &Validator{cfg: cfg}
}

// Validate implements bootstrap.Validator. It first checks for a cached replay
// response; if none is found, it validates the credential and returns a
// PendingEnrollment whose Commit hook records the outcome for future replays.
func (v *Validator) Validate(ctx context.Context, r *http.Request, cred common.BootstrapCredentialDTO, csrB64, svidProfileURI string) (bootstrap.ValidationOutcome, error) {
	if cached, err := v.checkReplay(ctx, cred, csrB64, svidProfileURI); err != nil {
		return bootstrap.ValidationOutcome{}, err
	} else if cached != nil {
		return bootstrap.ValidationOutcome{Replay: cached}, nil
	}
	vb, err := v.validate(ctx, r, cred)
	if err != nil {
		return bootstrap.ValidationOutcome{}, err
	}
	return bootstrap.ValidationOutcome{
		Pending: &bootstrap.PendingEnrollment{
			VB: vb,
			Commit: func(ctx context.Context, resp *common.EnrollmentResponseDTO, statusCode int, ldiUUID string) error {
				return v.commit(ctx, vb, csrB64, resp, statusCode, ldiUUID)
			},
		},
	}, nil
}

// validate is the credential-verification phase: parses the token secret,
// consumes the one-time token in the store, and derives the ESI.
func (v *Validator) validate(ctx context.Context, r *http.Request, cred common.BootstrapCredentialDTO) (bootstrap.VerifiedBootstrap, error) {
	if cred.Proof == nil || strings.TrimSpace(cred.Proof.Token) == "" {
		return bootstrap.VerifiedBootstrap{}, ErrMissingToken
	}
	secret, err := tokens.ParseSecret(cred.Proof.Token)
	if err != nil {
		return bootstrap.VerifiedBootstrap{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	tokenID, err := v.cfg.Store.Consume(ctx, secret.Hash(), v.cfg.Clock())
	if err != nil {
		switch {
		case errors.Is(err, tokens.ErrNotFound),
			errors.Is(err, tokens.ErrExpired),
			errors.Is(err, tokens.ErrAlreadyUsed):
			return bootstrap.VerifiedBootstrap{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
		default:
			return bootstrap.VerifiedBootstrap{}, fmt.Errorf("enrollment-token: store: %w", err)
		}
	}
	esi := sha256.Sum256([]byte(tokenID))
	return bootstrap.VerifiedBootstrap{
		Method: cred.Method,
		ESI:    esi[:],
		Evidence: &Evidence{
			TokenID:    tokenID,
			SecretHash: secret.Hash(),
		},
	}, nil
}

// RetryWindow returns the effective idempotent-retry window.
func (v *Validator) RetryWindow() time.Duration { return v.cfg.RetryWindow }

// commit records the outcome for bounded idempotent-retry recognition.
func (v *Validator) commit(
	ctx context.Context,
	vb bootstrap.VerifiedBootstrap,
	csrB64 string,
	response *common.EnrollmentResponseDTO,
	statusCode int,
	ldiUUID string,
) error {
	ev, ok := vb.Evidence.(*Evidence)
	if !ok || ev == nil {
		return fmt.Errorf("enrollment-token: Commit got unexpected Evidence type %T", vb.Evidence)
	}
	if response == nil {
		return fmt.Errorf("enrollment-token: Commit response is nil")
	}
	fp, err := csrPubKeyFingerprint(csrB64)
	if err != nil {
		return fmt.Errorf("enrollment-token: Commit cannot fingerprint CSR: %w", err)
	}
	now := v.cfg.Clock()
	return v.cfg.Store.RecordOutcome(ctx, tokens.Outcome{
		TokenID:             ev.TokenID,
		StatusCode:          statusCode,
		CertificateChainPEM: append([]string(nil), response.SVID.CertificateChainPEM...),
		SVIDProfileURI:      response.SVIDProfileURI,
		CSRPubKeySHA256:     fp,
		LDIUUID:             ldiUUID,
		CompletedAt:         now,
		RetryWindowEnds:     now.Add(v.cfg.RetryWindow),
	})
}

// checkReplay recognises a bounded idempotent retry per SUP Appendix A.
func (v *Validator) checkReplay(
	ctx context.Context,
	cred common.BootstrapCredentialDTO,
	csrB64 string,
	svidProfileURI string,
) (*bootstrap.CachedResponse, error) {
	if cred.Proof == nil || strings.TrimSpace(cred.Proof.Token) == "" {
		return nil, nil
	}
	secret, err := tokens.ParseSecret(cred.Proof.Token)
	if err != nil {
		return nil, nil
	}
	now := v.cfg.Clock()
	_, outcome, err := v.cfg.Store.FindConsumed(ctx, secret.Hash(), now)
	switch {
	case err == nil:
		// consumed + outcome + within window - fall through to validation
	case errors.Is(err, tokens.ErrRetryWindowExpired):
		// Token was consumed and had an outcome, but the retry window has
		// now closed. Reject with ErrInvalidToken so the handler does not
		// re-issue a credential for a stale, single-use token.
		return nil, fmt.Errorf("%w: retry window closed", ErrInvalidToken)
	case errors.Is(err, tokens.ErrNotFound):
		// Token is unknown, still active (no outcome yet), or otherwise
		// not a replay candidate - let the normal Validate path handle it.
		return nil, nil
	default:
		return nil, fmt.Errorf("enrollment-token: store: %w", err)
	}
	if outcome.SVIDProfileURI != svidProfileURI {
		return nil, fmt.Errorf("%w: svid_profile_uri changed on retry", ErrInvalidToken)
	}
	fp, err := csrPubKeyFingerprint(csrB64)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot fingerprint CSR on replay check: %v", ErrInvalidToken, err)
	}
	if !bytesEqual(outcome.CSRPubKeySHA256, fp) {
		return nil, fmt.Errorf("%w: CSR public key changed on retry", ErrInvalidToken)
	}
	return &bootstrap.CachedResponse{
		StatusCode: outcome.StatusCode,
		Response: &common.EnrollmentResponseDTO{
			SVIDProfileURI: outcome.SVIDProfileURI,
			SVID:           common.X509SVIDDTO{CertificateChainPEM: append([]string(nil), outcome.CertificateChainPEM...)},
		},
	}, nil
}

func bytesEqual(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
