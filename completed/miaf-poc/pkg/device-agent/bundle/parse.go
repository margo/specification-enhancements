package bundle

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
)

// ExtractX509Authorities parses a SPIFFE Bundle Map JSON body (as served by
// MIS at /.well-known/spiffe/bundle.json) and returns the X.509 authorities
// for the named trust domain. JWT authorities are skipped.
//
// Input shape per SPIFFE §5.1.1:
//
//	{ "trust_domains": { "<td>": { "keys": [ { "use": "x509-svid", "x5c": [ "<b64der>" ] }, ... ] } } }
func ExtractX509Authorities(body []byte, trustDomain string) ([]*x509.Certificate, error) {
	var envelope struct {
		TrustDomains map[string]struct {
			Keys []struct {
				Use string   `json:"use"`
				X5C []string `json:"x5c"`
			} `json:"keys"`
		} `json:"trust_domains"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse bundle map: %w", err)
	}
	entry, ok := envelope.TrustDomains[trustDomain]
	if !ok {
		return nil, fmt.Errorf("bundle map has no entry for trust domain %q", trustDomain)
	}
	var out []*x509.Certificate
	for i, k := range entry.Keys {
		if k.Use != "x509-svid" {
			continue
		}
		if len(k.X5C) == 0 {
			return nil, fmt.Errorf("keys[%d]: x509-svid entry missing x5c", i)
		}
		der, err := base64.StdEncoding.DecodeString(k.X5C[0])
		if err != nil {
			return nil, fmt.Errorf("keys[%d]: decode x5c: %w", i, err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("keys[%d]: parse x509 authority: %w", i, err)
		}
		out = append(out, cert)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("bundle map has no x509-svid authorities for %q", trustDomain)
	}
	return out, nil
}

// JWTAuthority is a parsed jwt-svid key entry from the Bundle Map.
type JWTAuthority struct {
	KeyID     string
	PublicKey any
}

// ExtractJWTAuthorities parses a SPIFFE Bundle Map JSON body and returns the
// JWT-SVID authorities for the named trust domain.
func ExtractJWTAuthorities(body []byte, trustDomain string) ([]JWTAuthority, error) {
	var envelope struct {
		TrustDomains map[string]struct {
			Keys []struct {
				Use string `json:"use"`
				Kid string `json:"kid"`
				Kty string `json:"kty"`
				Crv string `json:"crv"`
				X   string `json:"x"`
				Y   string `json:"y"`
				N   string `json:"n"`
				E   string `json:"e"`
			} `json:"keys"`
		} `json:"trust_domains"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse bundle map: %w", err)
	}
	entry, ok := envelope.TrustDomains[trustDomain]
	if !ok {
		return nil, fmt.Errorf("bundle map has no entry for trust domain %q", trustDomain)
	}
	var out []JWTAuthority
	for i, k := range entry.Keys {
		if k.Use != "jwt-svid" {
			continue
		}
		pub, err := decodeJWTKey(k.Kty, k.Crv, k.X, k.Y, k.N, k.E)
		if err != nil {
			return nil, fmt.Errorf("keys[%d]: %w", i, err)
		}
		out = append(out, JWTAuthority{KeyID: k.Kid, PublicKey: pub})
	}
	return out, nil
}

func decodeJWTKey(kty, crv, x, y, n, e string) (any, error) {
	switch kty {
	case "EC":
		curveFunc, err := ecCurveByName(crv)
		if err != nil {
			return nil, err
		}
		xb, err := base64.RawURLEncoding.DecodeString(x)
		if err != nil {
			return nil, fmt.Errorf("decode x: %w", err)
		}
		yb, err := base64.RawURLEncoding.DecodeString(y)
		if err != nil {
			return nil, fmt.Errorf("decode y: %w", err)
		}
		return &ecdsa.PublicKey{
			Curve: curveFunc,
			X:     new(big.Int).SetBytes(xb),
			Y:     new(big.Int).SetBytes(yb),
		}, nil
	case "RSA":
		nb, err := base64.RawURLEncoding.DecodeString(n)
		if err != nil {
			return nil, fmt.Errorf("decode n: %w", err)
		}
		eb, err := base64.RawURLEncoding.DecodeString(e)
		if err != nil {
			return nil, fmt.Errorf("decode e: %w", err)
		}
		return &rsa.PublicKey{
			N: new(big.Int).SetBytes(nb),
			E: int(new(big.Int).SetBytes(eb).Int64()),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported kty %q", kty)
	}
}

func ecCurveByName(name string) (elliptic.Curve, error) {
	switch name {
	case "P-256":
		return elliptic.P256(), nil
	case "P-384":
		return elliptic.P384(), nil
	case "P-521":
		return elliptic.P521(), nil
	default:
		return nil, fmt.Errorf("unsupported EC curve %q", name)
	}
}
