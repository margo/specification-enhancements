package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
)

// IssuedJWTSVIDRecord is one row in the JWT SVID issuance audit.
type IssuedJWTSVIDRecord struct {
	JTI       string
	SPIFFEID  string
	Audiences []string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// JWTSVIDIssuanceLog persists IssuedJWTSVIDRecord rows.
type JWTSVIDIssuanceLog interface {
	Record(ctx context.Context, rec IssuedJWTSVIDRecord) error
}

// JWTSigner is the narrow signer port the service consumes.
type JWTSigner interface {
	Sign(ctx context.Context, claims jwt.Claims) (string, error)
}

// JWTSVIDResult is what the handler needs to render a 201 Created response.
type JWTSVIDResult struct {
	JWT       string
	ExpiresAt time.Time
}

// JWTSVIDConfig captures the per-call inputs.
type JWTSVIDConfig struct {
	Issuer     string        // iss claim - typically MIS_ADVERTISE_URL
	DefaultTTL time.Duration // applied when the request omits ttl
	MaxTTL     time.Duration // upper bound regardless of request
}

// IssueJWTSVID resolves the LDI for spiffeID, caps the TTL, signs, and audits.
//
// Audience validation is the caller's responsibility (jwtexchange.ValidateAudience).
// Authentication is the caller's responsibility (mTLS peer-cert match or
// jwtexchange.VerifyClientAssertion).
func (s *Service) IssueJWTSVID(
	ctx context.Context,
	signer JWTSigner,
	auditLog JWTSVIDIssuanceLog,
	cfg JWTSVIDConfig,
	spiffeID string,
	requestedAudiences []string,
	requestedTTL time.Duration,
) (JWTSVIDResult, error) {
	if signer == nil {
		return JWTSVIDResult{}, errors.New("identity: IssueJWTSVID requires a signer")
	}
	if auditLog == nil {
		return JWTSVIDResult{}, errors.New("identity: IssueJWTSVID requires an audit log")
	}
	if cfg.Issuer == "" {
		return JWTSVIDResult{}, errors.New("identity: JWTSVIDConfig.Issuer is required")
	}
	if cfg.MaxTTL <= 0 {
		return JWTSVIDResult{}, errors.New("identity: JWTSVIDConfig.MaxTTL must be > 0")
	}
	if cfg.DefaultTTL <= 0 || cfg.DefaultTTL > cfg.MaxTTL {
		cfg.DefaultTTL = cfg.MaxTTL
	}

	ldi, err := s.repo.GetLDIBySPIFFEID(ctx, spiffeID)
	if err != nil {
		return JWTSVIDResult{}, err
	}
	if ldi.Status != LDIStatusActive {
		return JWTSVIDResult{}, ErrNotFound
	}

	ttl := requestedTTL
	if ttl <= 0 {
		ttl = cfg.DefaultTTL
	}
	if ttl > cfg.MaxTTL {
		ttl = cfg.MaxTTL
	}

	now := s.now().UTC()
	exp := now.Add(ttl)
	jti := uuid.NewString()

	claims := jwt.Claims{
		Issuer:   cfg.Issuer,
		Subject:  spiffeID,
		Audience: jwt.Audience(requestedAudiences),
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(exp),
		ID:       jti,
	}
	compact, err := signer.Sign(ctx, claims)
	if err != nil {
		return JWTSVIDResult{}, fmt.Errorf("identity: sign jwt-svid: %w", err)
	}
	if err := auditLog.Record(ctx, IssuedJWTSVIDRecord{
		JTI:       jti,
		SPIFFEID:  spiffeID,
		Audiences: requestedAudiences,
		IssuedAt:  now,
		ExpiresAt: exp,
	}); err != nil {
		// Auditing is best-effort: log and proceed (matches existing
		// recordIssuance pattern).
		_ = err
	}
	return JWTSVIDResult{JWT: compact, ExpiresAt: exp}, nil
}
