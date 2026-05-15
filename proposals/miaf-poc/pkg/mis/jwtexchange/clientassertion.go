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
)

// Error sentinels. All map to 401 about:blank in the HTTP handler.
var (
	ErrAssertionMissing = errors.New("jwtexchange: client_assertion missing")
	ErrAssertionInvalid = errors.New("jwtexchange: client_assertion invalid")
	ErrReplay           = errors.New("jwtexchange: client_assertion replay")
	ErrUntrustedChain   = errors.New("jwtexchange: x5c chain does not verify against the Trust Bundle")
	ErrUnknownSPIFFEID  = errors.New("jwtexchange: x5c[0] SPIFFE ID does not match path")
	ErrUnsupportedAlg   = errors.New("jwtexchange: unsupported alg")
)

// permittedAlgs lists the JWS algorithms the SUP allows for this method.
var permittedAlgs = []jose.SignatureAlgorithm{jose.ES256, jose.PS256}

// VerifyConfig parametrises VerifyClientAssertion.
type VerifyConfig struct {
	// Roots is the set of X.509 trust anchors (root CAs) for the local Trust
	// Domain, used to validate the x5c chain in the client_assertion. Required.
	Roots       *x509.CertPool
	Replay      replay.Store     // required
	Now         func() time.Time // defaults to time.Now
	ExpectedAud string           // required: exact endpoint URL
	MaxLifetime time.Duration    // SUP fixes at 5m; PoC keeps configurable for tests
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
//
// The assertion's JWS header MUST include x5c with the complete X.509 SVID
// chain (leaf first, then intermediates). The chain is validated against
// cfg.Roots; the JWS signature is verified against the public key of x5c[0];
// and x5c[0]'s URI SAN MUST equal pathSPIFFEID.
func VerifyClientAssertion(ctx context.Context, cfg VerifyConfig, pathSPIFFEID, compact string) (VerifiedClientAssertion, error) {
	if cfg.Roots == nil {
		return VerifiedClientAssertion{}, errors.New("jwtexchange: VerifyConfig.Roots is required")
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
	hdr := parsed.Headers[0]
	hdrAlg := jose.SignatureAlgorithm(hdr.Algorithm)

	// Extract and verify the x5c chain against the Trust Bundle root anchors.
	// go-jose v4's Header.Certificates() decodes x5c and runs x509.Verify in
	// one call, treating x5c[1:] as intermediates and x5c[0] as the leaf.
	leaf, err := extractAndVerifyChain(hdr, cfg.Roots)
	if err != nil {
		return VerifiedClientAssertion{}, err
	}

	// x5c[0]'s URI SAN MUST equal the path-conveyed SPIFFE ID.
	if len(leaf.URIs) == 0 || leaf.URIs[0].String() != pathSPIFFEID {
		var got string
		if len(leaf.URIs) > 0 {
			got = leaf.URIs[0].String()
		}
		return VerifiedClientAssertion{}, fmt.Errorf("%w: x5c[0] URI SAN = %q, want %q", ErrUnknownSPIFFEID, got, pathSPIFFEID)
	}

	if err := algMatchesLeafKey(hdrAlg, leaf); err != nil {
		return VerifiedClientAssertion{}, fmt.Errorf("%w: %v", ErrUnsupportedAlg, err)
	}

	// Verify signature against x5c[0]'s public key, and extract claims.
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

// extractAndVerifyChain decodes the x5c header from h and verifies the chain
// against roots. It returns x5c[0] on success.
//
// jose.ErrMissingX5cHeader indicates x5c was absent entirely, mapped to
// ErrAssertionInvalid. Any other error means the chain is present but fails
// trust validation, mapped to ErrUntrustedChain.
func extractAndVerifyChain(h jose.Header, roots *x509.CertPool) (*x509.Certificate, error) {
	chains, err := h.Certificates(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	if err != nil {
		if errors.Is(err, jose.ErrMissingX5cHeader) {
			return nil, fmt.Errorf("%w: x5c header missing", ErrAssertionInvalid)
		}
		return nil, fmt.Errorf("%w: %v", ErrUntrustedChain, err)
	}
	if len(chains) == 0 || len(chains[0]) == 0 {
		return nil, fmt.Errorf("%w: x5c chain empty after verification", ErrAssertionInvalid)
	}
	return chains[0][0], nil
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
