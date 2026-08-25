// Package bundlemap builds SPIFFE Bundle Map JSON responses from trust anchor
// sets expressed as standard library types (crypto/x509).
//
// The SPIFFE Bundle Map is defined in
// https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Trust_Domain_and_Bundle.md#5-spiffe-bundle-map
// as a JSON object with a top-level "trust_domains" key whose value is a map
// of trust-domain names to SPIFFE Trust Bundle documents (JWKS with additional
// SPIFFE fields).
package bundlemap

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
)

// TrustAnchorSet is the information needed to render a SPIFFE Bundle Map for
// a single trust domain: the trust domain name, the X.509 root authorities
// that validate device SVIDs, the optional JWT authorities, and bookkeeping
// (sequence number + refresh hint).
type TrustAnchorSet struct {
	TrustDomain     string
	X509Authorities []*x509.Certificate
	JWTAuthorities  []JWTAuthority
	SequenceNumber  int64
}

// JWTAuthority is a single JWT-SVID signing key in the bundle. KeyID maps to
// the JWK "kid" field; the public key is typically *ecdsa.PublicKey or
// *rsa.PublicKey. If no JWT-SVID support is configured the slice is empty.
type JWTAuthority struct {
	KeyID     string
	PublicKey any
}

// Build returns a SPIFFE Bundle Map (SPIFFE §5.1.1) serialising the supplied
// trust anchor set as the sole entry, keyed by its trust domain.
func Build(set TrustAnchorSet) ([]byte, error) {
	if set.TrustDomain == "" {
		return nil, errors.New("bundle: missing trust_domain")
	}
	entry, err := buildEntry(set)
	if err != nil {
		return nil, fmt.Errorf("build entry: %w", err)
	}
	envelope := map[string]any{
		"trust_domains": map[string]any{
			set.TrustDomain: entry,
		},
	}
	return json.Marshal(envelope)
}

func buildEntry(set TrustAnchorSet) (map[string]any, error) {
	keys := make([]map[string]any, 0, len(set.X509Authorities)+len(set.JWTAuthorities))

	for i, cert := range set.X509Authorities {
		if cert == nil {
			return nil, fmt.Errorf("x509_authorities[%d]: nil certificate", i)
		}
		jwk, err := publicKeyParams(cert.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("x509_authorities[%d]: %w", i, err)
		}
		jwk["use"] = "x509-svid"
		jwk["x5c"] = []string{base64.StdEncoding.EncodeToString(cert.Raw)}
		keys = append(keys, jwk)
	}

	for i, a := range set.JWTAuthorities {
		jwk, err := publicKeyParams(a.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("jwt_authorities[%d]: %w", i, err)
		}
		jwk["use"] = "jwt-svid"
		if a.KeyID != "" {
			jwk["kid"] = a.KeyID
		}
		keys = append(keys, jwk)
	}

	entry := map[string]any{"keys": keys}
	// spiffe_refresh_hint is intentionally omitted: SPIFFE §5.1.1 RECOMMENDS
	// producers of Bundle Maps omit this field from bundled entries.
	if set.SequenceNumber > 0 {
		entry["spiffe_sequence"] = set.SequenceNumber
	}
	return entry, nil
}

// publicKeyParams returns a partial JWK map with kty and key-specific
// parameters populated (SPIFFE §4.2.1, RFC 7517/7518). The caller must add
// the "use" field (and optionally "kid", "x5c").
func publicKeyParams(pub any) (map[string]any, error) {
	switch k := pub.(type) {
	case *ecdsa.PublicKey:
		crv, err := ecCurveName(k.Curve)
		if err != nil {
			return nil, err
		}
		size := (k.Curve.Params().BitSize + 7) / 8
		return map[string]any{
			"kty": "EC",
			"crv": crv,
			"x":   b64urlFixed(k.X, size),
			"y":   b64urlFixed(k.Y, size),
		}, nil
	case *rsa.PublicKey:
		return map[string]any{
			"kty": "RSA",
			"n":   b64url(k.N.Bytes()),
			"e":   b64url(big.NewInt(int64(k.E)).Bytes()),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported public key type %T", pub)
	}
}

func ecCurveName(c elliptic.Curve) (string, error) {
	switch c {
	case elliptic.P256():
		return "P-256", nil
	case elliptic.P384():
		return "P-384", nil
	case elliptic.P521():
		return "P-521", nil
	default:
		return "", fmt.Errorf("unsupported EC curve %q", c.Params().Name)
	}
}

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// b64urlFixed left-pads b to size bytes before base64url encoding. EC
// coordinates must be fixed-width per RFC 7518 §6.2.1.2.
func b64urlFixed(n *big.Int, size int) string {
	b := n.Bytes()
	if len(b) < size {
		pad := make([]byte, size)
		copy(pad[size-len(b):], b)
		b = pad
	}
	return b64url(b)
}
