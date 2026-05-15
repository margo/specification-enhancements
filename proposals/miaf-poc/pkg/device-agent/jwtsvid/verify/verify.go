// Package verify validates an issued JWT SVID against a Trust Bundle.
package verify

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/spiffe/go-spiffe/v2/bundle/jwtbundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
)

// SourceFromBundleMap parses a Bundle Map JSON body and returns a
// jwtbundle.Bundle ready for jwtsvid.ParseAndValidate. Only the entry for
// trustDomain is populated.
func SourceFromBundleMap(body []byte, trustDomain spiffeid.TrustDomain) (*jwtbundle.Bundle, error) {
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
		return nil, fmt.Errorf("verify: parse bundle map: %w", err)
	}
	entry, ok := envelope.TrustDomains[trustDomain.String()]
	if !ok {
		return nil, fmt.Errorf("verify: bundle map has no entry for %q", trustDomain.String())
	}
	bundle := jwtbundle.New(trustDomain)
	for i, k := range entry.Keys {
		if k.Use != "jwt-svid" {
			continue
		}
		pub, err := decodeJWK(k.Kty, k.Crv, k.X, k.Y, k.N, k.E)
		if err != nil {
			return nil, fmt.Errorf("verify: keys[%d]: %w", i, err)
		}
		if err := bundle.AddJWTAuthority(k.Kid, pub); err != nil {
			return nil, fmt.Errorf("verify: keys[%d]: AddJWTAuthority: %w", i, err)
		}
	}
	return bundle, nil
}

// ParseAndValidate calls into go-spiffe v2 jwtsvid.ParseAndValidate. It
// returns the parsed JWT-SVID with all SPIFFE-required validations applied
// (signature against bundle, sub conforms to a SPIFFE ID in the trust
// domain, expiry, audience).
func ParseAndValidate(ctx context.Context, bundle *jwtbundle.Bundle, audience []string, compact string) (*jwtsvid.SVID, error) {
	svid, err := jwtsvid.ParseAndValidate(compact, bundle, audience)
	if err != nil {
		return nil, fmt.Errorf("verify: %w", err)
	}
	return svid, nil
}

func decodeJWK(kty, crv, x, y, n, e string) (any, error) {
	switch kty {
	case "EC":
		curve, err := ecCurve(crv)
		if err != nil {
			return nil, err
		}
		xb, err := base64.RawURLEncoding.DecodeString(x)
		if err != nil {
			return nil, err
		}
		yb, err := base64.RawURLEncoding.DecodeString(y)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{
			Curve: curve,
			X:     new(big.Int).SetBytes(xb),
			Y:     new(big.Int).SetBytes(yb),
		}, nil
	case "RSA":
		nb, err := base64.RawURLEncoding.DecodeString(n)
		if err != nil {
			return nil, err
		}
		eb, err := base64.RawURLEncoding.DecodeString(e)
		if err != nil {
			return nil, err
		}
		return &rsa.PublicKey{
			N: new(big.Int).SetBytes(nb),
			E: int(new(big.Int).SetBytes(eb).Int64()),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported kty %q", kty)
	}
}

func ecCurve(name string) (elliptic.Curve, error) {
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
