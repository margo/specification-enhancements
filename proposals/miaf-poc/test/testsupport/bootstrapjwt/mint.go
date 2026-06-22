// Package bootstrapjwt mints Bootstrap Assertion JWTs for tests. It is the
// single source of test assertions consumed by factorycertjwt validator unit
// tests, transport/http integration tests, and test/conformance tests.
//
// Per SUP Appendix A "Factory Certificate Method (JWT Assertion)":
//
//   - Signature MUST be ES256 (P-256) or PS256 (RSA-PSS 3072), matching the
//     key type of the factory leaf.
//   - Header MUST include x5c with the complete chain, leaf at x5c[0].
//   - Payload MUST include iss, sub, aud, iat, exp, jti (and MAY include nbf).
//   - exp - iat MUST be <= 300 seconds.
//   - iss == sub == "urn:margo:device:sha256:<lowercase-hex-fingerprint>" of x5c[0].
package bootstrapjwt

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
)

// Assertion bundles the compact JWT together with the claims and jti used to
// build it, so tests can assert on what was actually signed.
type Assertion struct {
	Compact string
	Claims  Claims
}

// Claims is the payload tests control.  Zero values are filled with SUP-legal
// defaults by Mint.
type Claims struct {
	Issuer    string
	Subject   string
	Audience  string
	IssuedAt  time.Time
	NotBefore time.Time // zero => claim omitted
	Expiry    time.Time
	JTI       string
}

// Options lets tests override specific fields to exercise negative cases.
type Options struct {
	// Algorithm overrides the auto-detected signing algorithm. Must be one of
	// jose.ES256 / jose.PS256 / jose.RS256 (RS256 used only by the intentional
	// "forbidden alg" negative test).
	Algorithm jose.SignatureAlgorithm
	// SkipX5C omits the x5c header (negative test).
	SkipX5C bool
	// X5COverride replaces x5c entirely with the supplied DER chain.
	// Each element must be a valid DER-encoded certificate. To inject
	// a syntactically corrupt x5c value, build the compact token manually.
	X5COverride [][]byte
	// ClaimsOverride, when non-nil, replaces the default claims after they
	// are materialized. Test fills just the fields it wants to mutate.
	ClaimsOverride func(c *Claims)
}

// Mint builds a valid Bootstrap Assertion for (leaf, key). Tests pass zero
// Options for a fully-valid assertion; fill Options to produce broken ones.
func Mint(t *testing.T, leaf *x509.Certificate, chain []*x509.Certificate, key crypto.Signer, audience string, opts Options) Assertion {
	t.Helper()

	if opts.SkipX5C && opts.X5COverride != nil {
		t.Fatal("bootstrapjwt: SkipX5C and X5COverride are mutually exclusive")
	}

	now := time.Now().UTC()
	fp := sha256.Sum256(leaf.Raw)
	urn := "urn:margo:device:sha256:" + hex.EncodeToString(fp[:])
	claims := Claims{
		Issuer:   urn,
		Subject:  urn,
		Audience: audience,
		IssuedAt: now,
		Expiry:   now.Add(5 * time.Minute),
		JTI:      uuid.NewString(),
	}
	if opts.ClaimsOverride != nil {
		opts.ClaimsOverride(&claims)
	}

	alg := opts.Algorithm
	if alg == "" {
		alg = defaultAlg(t, key)
	}

	headerCerts := chain
	if opts.X5COverride != nil {
		headerCerts = parseDERChain(t, opts.X5COverride)
	}
	if opts.SkipX5C {
		headerCerts = nil
	}

	sopts := (&jose.SignerOptions{}).WithType("JWT")
	if headerCerts != nil {
		sopts = sopts.WithHeader(jose.HeaderKey("x5c"), encodeX5C(headerCerts))
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: alg, Key: key}, sopts)
	if err != nil {
		t.Fatalf("bootstrapjwt: NewSigner: %v", err)
	}

	builder := jwt.Signed(signer).Claims(jwt.Claims{
		Issuer:   claims.Issuer,
		Subject:  claims.Subject,
		Audience: jwt.Audience{claims.Audience},
		IssuedAt: jwt.NewNumericDate(claims.IssuedAt),
		Expiry:   jwt.NewNumericDate(claims.Expiry),
		ID:       claims.JTI,
	})
	if !claims.NotBefore.IsZero() {
		builder = builder.Claims(map[string]any{"nbf": jwt.NewNumericDate(claims.NotBefore)})
	}

	compact, err := builder.Serialize()
	if err != nil {
		t.Fatalf("bootstrapjwt: Serialize: %v", err)
	}
	return Assertion{Compact: compact, Claims: claims}
}

func defaultAlg(t *testing.T, key crypto.Signer) jose.SignatureAlgorithm {
	t.Helper()
	switch key.(type) {
	case *ecdsa.PrivateKey:
		return jose.ES256
	case *rsa.PrivateKey:
		return jose.PS256
	default:
		t.Fatalf("bootstrapjwt: unsupported key type %T", key)
		return ""
	}
}

// encodeX5C returns the x5c header value per RFC 7517 §4.7: an array of
// base64-encoded DER strings, leaf first.
func encodeX5C(chain []*x509.Certificate) []string {
	out := make([]string, 0, len(chain))
	for _, c := range chain {
		out = append(out, base64.StdEncoding.EncodeToString(c.Raw))
	}
	return out
}

func parseDERChain(t *testing.T, der [][]byte) []*x509.Certificate {
	t.Helper()
	out := make([]*x509.Certificate, 0, len(der))
	for _, b := range der {
		c, err := x509.ParseCertificate(b)
		if err != nil {
			t.Fatalf("bootstrapjwt: ParseCertificate: %v", err)
		}
		out = append(out, c)
	}
	return out
}
