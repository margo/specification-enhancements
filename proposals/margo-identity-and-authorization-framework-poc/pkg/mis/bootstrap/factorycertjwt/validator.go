// Package factorycertmtls implements the factory-cert-jwt bootstrap
// method defined in SUP Appendix A.
package factorycertjwt

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertjwt/replay"
)

// Error sentinels. All map to 401 about:blank in the enrollment handler
// (SUP Appendix B "Malformed JWT assertion or proof").
var (
	ErrMissingAssertion   = errors.New("factory-cert-jwt: bootstrapCredential.proof.assertion is required")
	ErrMalformedAssertion = errors.New("factory-cert-jwt: malformed assertion")
	ErrUnsupportedAlg     = errors.New("factory-cert-jwt: unsupported signing algorithm")
	ErrBadSignature       = errors.New("factory-cert-jwt: signature verification failed")
	ErrUntrustedChain     = errors.New("factory-cert-jwt: x5c chain does not verify against factory trust pool")
	ErrClaimMissing       = errors.New("factory-cert-jwt: required claim missing")
	ErrClaimInvalid       = errors.New("factory-cert-jwt: claim failed validation")
	ErrReplay             = errors.New("factory-cert-jwt: jti replayed")
)

// Config configures Validator.
type Config struct {
	// Roots is the factory trust pool. Required.
	Roots *x509.CertPool
	// Audience is the exact enrollment endpoint URL (aud claim). Required.
	// Typically "<advertise-url>/api/v1/identities".
	Audience string
	// Replay tracks jtis. Required.
	Replay replay.Store
	// ClockSkew is the allowed clock skew applied when evaluating exp and nbf.
	// Defaults to 30s.
	ClockSkew time.Duration
	// Clock defaults to time.Now.
	Clock func() time.Time
	// MaxAssertionLifetime enforces SUP "exp - iat <= 300s". Defaults to 5m.
	MaxAssertionLifetime time.Duration
}

// permittedAlgs lists the JWS algorithms the SUP allows for this method.
// RS256 is deliberately excluded (SUP Cryptographic Requirements).
var permittedAlgs = []jose.SignatureAlgorithm{jose.ES256, jose.PS256}

// Validator implements bootstrap.Validator for factory-cert-jwt.
type Validator struct {
	cfg Config
}

// New constructs a Validator. Panics if required config is missing.
func New(cfg Config) *Validator {
	if cfg.Roots == nil {
		panic("factorycertjwt: Config.Roots is required")
	}
	if cfg.Audience == "" {
		panic("factorycertjwt: Config.Audience is required")
	}
	if cfg.Replay == nil {
		panic("factorycertjwt: Config.Replay is required")
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	if cfg.ClockSkew == 0 {
		cfg.ClockSkew = 30 * time.Second
	}
	if cfg.MaxAssertionLifetime == 0 {
		cfg.MaxAssertionLifetime = 5 * time.Minute
	}
	return &Validator{cfg: cfg}
}

// Validate implements bootstrap.Validator.
func (v *Validator) Validate(ctx context.Context, _ *http.Request, cred common.BootstrapCredentialDTO, _, _ string) (bootstrap.ValidationOutcome, error) {
	if cred.Proof == nil || strings.TrimSpace(cred.Proof.Assertion) == "" {
		return bootstrap.ValidationOutcome{}, ErrMissingAssertion
	}

	// Parse the compact JWS with the permitted algs. jose v4 requires algs
	// up front so RS256 is rejected at parse time.
	parsed, err := jwt.ParseSigned(cred.Proof.Assertion, permittedAlgs)
	if err != nil {
		if strings.Contains(err.Error(), "unexpected signature algorithm") {
			return bootstrap.ValidationOutcome{},fmt.Errorf("%w: %v", ErrUnsupportedAlg, err)
		}
		return bootstrap.ValidationOutcome{},fmt.Errorf("%w: %v", ErrMalformedAssertion, err)
	}
	if len(parsed.Headers) != 1 {
		return bootstrap.ValidationOutcome{},fmt.Errorf("%w: expected exactly one JWS header, got %d", ErrMalformedAssertion, len(parsed.Headers))
	}

	hdr := parsed.Headers[0]

	// extractAndVerifyChain first decodes the x5c header (returning ErrMalformedAssertion
	// when absent or corrupt), then verifies the chain against our trust pool
	// (returning ErrUntrustedChain on failure). We separate these two error
	// classes so callers can distinguish "bad token" from "wrong factory CA".
	leaf, err := extractAndVerifyChain(hdr, v.cfg.Roots)
	if err != nil {
		return bootstrap.ValidationOutcome{},err
	}

	// The SUP requires alg to match the key type of the factory leaf.
	if err := algMatchesLeafKey(jose.SignatureAlgorithm(hdr.Algorithm), leaf); err != nil {
		return bootstrap.ValidationOutcome{},fmt.Errorf("%w: %v", ErrUnsupportedAlg, err)
	}

	// Claim validation per SUP Appendix A "Factory Certificate Method (JWT
	// Assertion)" and "Signed-Assertion Method Common Requirements".
	var std jwt.Claims
	var rawClaims map[string]any
	if err := parsed.Claims(leaf.PublicKey, &std, &rawClaims); err != nil {
		return bootstrap.ValidationOutcome{},fmt.Errorf("%w: %v", ErrBadSignature, err)
	}

	expectedIss := "urn:margo:device:sha256:" + hexFingerprint(leaf.Raw)
	if std.Issuer != expectedIss {
		return bootstrap.ValidationOutcome{},fmt.Errorf("%w: iss=%q, want %q", ErrClaimInvalid, std.Issuer, expectedIss)
	}
	if std.Subject != std.Issuer {
		return bootstrap.ValidationOutcome{},fmt.Errorf("%w: sub=%q, want %q (== iss)", ErrClaimInvalid, std.Subject, std.Issuer)
	}
	if len(std.Audience) != 1 || std.Audience[0] != v.cfg.Audience {
		return bootstrap.ValidationOutcome{},fmt.Errorf("%w: aud=%v, want [%q]", ErrClaimInvalid, []string(std.Audience), v.cfg.Audience)
	}
	if std.Expiry == nil || std.IssuedAt == nil {
		return bootstrap.ValidationOutcome{},fmt.Errorf("%w: iat and exp are required", ErrClaimMissing)
	}
	iat := std.IssuedAt.Time()
	exp := std.Expiry.Time()
	if exp.Sub(iat) > v.cfg.MaxAssertionLifetime {
		return bootstrap.ValidationOutcome{},fmt.Errorf("%w: exp - iat = %s, max %s", ErrClaimInvalid, exp.Sub(iat), v.cfg.MaxAssertionLifetime)
	}
	now := v.cfg.Clock()
	if now.After(exp.Add(v.cfg.ClockSkew)) {
		return bootstrap.ValidationOutcome{},fmt.Errorf("%w: assertion expired at %s (now %s)", ErrClaimInvalid, exp, now)
	}
	if std.NotBefore != nil {
		nbf := std.NotBefore.Time()
		if now.Add(v.cfg.ClockSkew).Before(nbf) {
			return bootstrap.ValidationOutcome{},fmt.Errorf("%w: assertion not valid before %s (now %s)", ErrClaimInvalid, nbf, now)
		}
	}
	// jti presence is required; replay enforcement per SUP "Signed-Assertion Method Common Requirements".
	jti, _ := rawClaims["jti"].(string)
	if strings.TrimSpace(jti) == "" {
		return bootstrap.ValidationOutcome{},fmt.Errorf("%w: jti claim required", ErrClaimMissing)
	}
	if err := v.cfg.Replay.Witness(ctx, jti, exp.Add(v.cfg.ClockSkew)); err != nil {
		if errors.Is(err, replay.ErrReplay) {
			return bootstrap.ValidationOutcome{},fmt.Errorf("%w: jti=%s", ErrReplay, jti)
		}
		return bootstrap.ValidationOutcome{},fmt.Errorf("replay store: %w", err)
	}

	esi := sha256.Sum256(leaf.Raw)
	return bootstrap.ValidationOutcome{
		Pending: &bootstrap.PendingEnrollment{
			VB: bootstrap.VerifiedBootstrap{
				Method:   cred.Method,
				ESI:      esi[:],
				Evidence: leaf,
			},
		},
	}, nil
}

// extractAndVerifyChain decodes the x5c header from h and verifies the chain
// against roots. It returns the leaf certificate (x5c[0]) on success.
//
// go-jose v4 stores x5c certificates in a private Header.certificates field,
// accessible only via Header.Certificates(opts). That method both decodes and
// verifies the chain in one call. jose.ErrMissingX5cHeader indicates the x5c
// header was absent entirely, which we map to ErrMalformedAssertion; any other
// error means the chain is present but fails trust verification => ErrUntrustedChain.
func extractAndVerifyChain(h jose.Header, roots *x509.CertPool) (*x509.Certificate, error) {
	chains, err := h.Certificates(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	if err != nil {
		if errors.Is(err, jose.ErrMissingX5cHeader) {
			return nil, fmt.Errorf("%w: x5c header missing", ErrMalformedAssertion)
		}
		// go-jose v4 wraps both "DER parse failure" and "chain not trusted" errors
		// identically; we map both to ErrUntrustedChain as they cannot be distinguished
		// without string-matching library internals.
		return nil, fmt.Errorf("%w: %v", ErrUntrustedChain, err)
	}
	// chains[0] is the verified chain starting from the leaf (x5c[0]).
	if len(chains) == 0 || len(chains[0]) == 0 {
		return nil, fmt.Errorf("%w: x5c chain empty after verification", ErrMalformedAssertion)
	}
	return chains[0][0], nil
}

// hexFingerprint returns the lowercase hex SHA-256 fingerprint of raw DER bytes.
// Used to construct the urn:margo:device:sha256:<fingerprint> identifier.
func hexFingerprint(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// algMatchesLeafKey enforces SUP: "alg MUST match the key type of the
// factory certificate (ES256 for ECDSA P-256 or PS256 for RSA-PSS 3072)".
func algMatchesLeafKey(alg jose.SignatureAlgorithm, leaf *x509.Certificate) error {
	switch leaf.PublicKey.(type) {
	case *ecdsa.PublicKey:
		if alg != jose.ES256 {
			return fmt.Errorf("leaf key is ECDSA but alg=%q (want ES256)", alg)
		}
	case *rsa.PublicKey:
		if alg != jose.PS256 {
			return fmt.Errorf("leaf key is RSA but alg=%q (want PS256)", alg)
		}
	default:
		return fmt.Errorf("unsupported leaf key type %T", leaf.PublicKey)
	}
	return nil
}
