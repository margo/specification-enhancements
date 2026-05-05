// Package jwtexchange implements the JWT SVID Exchange Endpoint's validation
// and minting flows.
package jwtexchange

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertjwt/replay"
	"github.com/margo/miaf-poc/pkg/mis/identity"
)

// Error sentinels. All map to 401 about:blank in the HTTP handler.
var (
	ErrAssertionMissing = errors.New("jwtexchange: client_assertion missing")
	ErrAssertionInvalid = errors.New("jwtexchange: client_assertion invalid")
	ErrReplay           = errors.New("jwtexchange: client_assertion replay")
	ErrUnknownSPIFFEID  = errors.New("jwtexchange: no active issued SVID for SPIFFE ID")
	ErrUnsupportedAlg   = errors.New("jwtexchange: unsupported alg")
)

// permittedAlgs lists the JWS algorithms the SUP allows for this method.
var permittedAlgs = []jose.SignatureAlgorithm{jose.ES256, jose.PS256}

// LeafLookup is the narrow slice of identity.IssuanceLog the verifier needs.
type LeafLookup interface {
	LeafByActiveSPIFFEID(ctx context.Context, spiffeID string, now time.Time) (identity.IssuedSVIDRecord, error)
}

// VerifyConfig parametrises VerifyClientAssertion.
type VerifyConfig struct {
	LeafLookup  LeafLookup       // required
	Replay      replay.Store     // required
	Now         func() time.Time // defaults to time.Now
	ExpectedAud string           // required: exact endpoint URL
	MaxLifetime time.Duration    // SUP fixes at 5m; PoC keeps configurable for  tests
	ClockSkew   time.Duration    // applied to exp/nbf
}

// VerifiedClientAssertion is the success result.
type VerifiedClientAssertion struct {
	SPIFFEID string
	JTI      string
	IssuedAt time.Time
	Expiry   time.Time
}

// VerifyClientAssertion implements SUP §"JWT SVID Exchange Endpoint" Client
// Assertion verification. It requires the path-conveyed SPIFFE ID up front so
// the iss/sub equality is checked against the same value that authorises the
// surrounding request.
func VerifyClientAssertion(ctx context.Context, cfg VerifyConfig, pathSPIFFEID, compact string) (VerifiedClientAssertion, error) {
	if cfg.LeafLookup == nil {
		return VerifiedClientAssertion{}, errors.New("jwtexchange: VerifyConfig.LeafLookup is required")
	}
	if cfg.Replay == nil {
		return VerifiedClientAssertion{}, errors.New("jwtexchange: VerifyConfig.Replay is required")
	}
	if cfg.ExpectedAud == "" {
		return VerifiedClientAssertion{}, errors.New("jwtexchange: VerifyConfig.ExpectedAud is required")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.MaxLifetime == 0 {
		cfg.MaxLifetime = 5 * time.Minute
	}
	if strings.TrimSpace(compact) == "" {
		return VerifiedClientAssertion{}, ErrAssertionMissing
	}

	parsed, err := jwt.ParseSigned(compact, permittedAlgs)
	if err != nil {
		if strings.Contains(err.Error(), "unexpected signature algorithm") {
			return VerifiedClientAssertion{}, fmt.Errorf("%w: %v", ErrUnsupportedAlg, err)
		}
		return VerifiedClientAssertion{}, fmt.Errorf("%w: parse: %v", ErrAssertionInvalid, err)
	}
	if len(parsed.Headers) != 1 {
		return VerifiedClientAssertion{}, fmt.Errorf("%w: expected one JWS header, got %d", ErrAssertionInvalid, len(parsed.Headers))
	}
	hdrAlg := jose.SignatureAlgorithm(parsed.Headers[0].Algorithm)

	// Look up the active leaf for the path SPIFFE ID.
	rec, err := cfg.LeafLookup.LeafByActiveSPIFFEID(ctx, pathSPIFFEID, cfg.Now())
	if err != nil {
		if errors.Is(err, identity.ErrNotFound) {
			return VerifiedClientAssertion{}, ErrUnknownSPIFFEID
		}
		return VerifiedClientAssertion{}, fmt.Errorf("jwtexchange: leaf lookup: %w", err)
	}
	leaf, perr := x509.ParseCertificate(rec.LeafDER)
	if perr != nil {
		return VerifiedClientAssertion{}, fmt.Errorf("jwtexchange: parse stored leaf: %w", perr)
	}
	if err := algMatchesLeafKey(hdrAlg, leaf); err != nil {
		return VerifiedClientAssertion{}, fmt.Errorf("%w: %v", ErrUnsupportedAlg, err)
	}

	// Verify signature and extract claims.
	var std jwt.Claims
	if err := parsed.Claims(leaf.PublicKey, &std); err != nil {
		return VerifiedClientAssertion{}, fmt.Errorf("%w: signature: %v", ErrAssertionInvalid, err)
	}

	if std.Issuer != pathSPIFFEID || std.Subject != pathSPIFFEID {
		return VerifiedClientAssertion{}, fmt.Errorf("%w: iss=%q sub=%q want both %q", ErrAssertionInvalid, std.Issuer, std.Subject, pathSPIFFEID)
	}
	if len(std.Audience) != 1 || std.Audience[0] != cfg.ExpectedAud {
		return VerifiedClientAssertion{}, fmt.Errorf("%w: aud=%v want [%q]", ErrAssertionInvalid, []string(std.Audience), cfg.ExpectedAud)
	}
	if std.IssuedAt == nil || std.Expiry == nil {
		return VerifiedClientAssertion{}, fmt.Errorf("%w: iat and exp required", ErrAssertionInvalid)
	}
	iat := std.IssuedAt.Time()
	exp := std.Expiry.Time()
	if exp.Sub(iat) > cfg.MaxLifetime {
		return VerifiedClientAssertion{}, fmt.Errorf("%w: exp - iat = %s > %s", ErrAssertionInvalid, exp.Sub(iat), cfg.MaxLifetime)
	}
	now := cfg.Now()
	if now.After(exp.Add(cfg.ClockSkew)) {
		return VerifiedClientAssertion{}, fmt.Errorf("%w: assertion expired at %s (now %s)", ErrAssertionInvalid, exp, now)
	}
	if std.NotBefore != nil {
		nbf := std.NotBefore.Time()
		if now.Add(cfg.ClockSkew).Before(nbf) {
			return VerifiedClientAssertion{}, fmt.Errorf("%w: assertion not valid before %s (now %s)", ErrAssertionInvalid, nbf, now)
		}
	}
	if std.ID == "" {
		return VerifiedClientAssertion{}, fmt.Errorf("%w: jti claim required", ErrAssertionInvalid)
	}
	if err := cfg.Replay.Witness(ctx, std.ID, exp.Add(cfg.ClockSkew)); err != nil {
		if errors.Is(err, replay.ErrReplay) {
			return VerifiedClientAssertion{}, fmt.Errorf("%w: jti=%s", ErrReplay, std.ID)
		}
		return VerifiedClientAssertion{}, fmt.Errorf("jwtexchange: replay store: %w", err)
	}
	return VerifiedClientAssertion{
		SPIFFEID: pathSPIFFEID,
		JTI:      std.ID,
		IssuedAt: iat,
		Expiry:   exp,
	}, nil
}

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
