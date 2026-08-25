package jwtsvid

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
)

// BuildClientAssertion produces a Client Authentication Assertion JWT per
// SUP §"JWT SVID Exchange Endpoint" Client Assertion JWT (Normative
// Definition).
//
// chain MUST be the device's complete X.509 SVID chain. chain[0] is the leaf
// SVID and is embedded as x5c[0]; remaining entries are the intermediates
// required for path validation. The chain is encoded into the JWS x5c header
// per RFC 7517 §4.7.
//
// audience MUST be the exact URL of the JWT SVID exchange endpoint
// ("<mis-base>/api/v1/identities/<spiffeIdEncoded>/jwt-svid").
//
// The JWS alg is selected from the key type: ES256 for ECDSA, PS256 for RSA.
func BuildClientAssertion(key crypto.Signer, chain []*x509.Certificate, spiffeID, audience string, now time.Time) (string, error) {
	if key == nil {
		return "", errors.New("jwtsvid: signing key is required")
	}
	if len(chain) == 0 {
		return "", errors.New("jwtsvid: SVID chain is required (leaf + any intermediates)")
	}
	for i, c := range chain {
		if c == nil {
			return "", fmt.Errorf("jwtsvid: chain[%d] is nil", i)
		}
	}
	alg, err := assertionAlg(key)
	if err != nil {
		return "", err
	}
	x5c := make([]string, len(chain))
	for i, c := range chain {
		x5c[i] = base64.StdEncoding.EncodeToString(c.Raw)
	}
	sopts := (&jose.SignerOptions{}).
		WithType("JWT").
		WithHeader(jose.HeaderKey("x5c"), x5c)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: alg, Key: key}, sopts)
	if err != nil {
		return "", fmt.Errorf("build assertion signer: %w", err)
	}
	claims := jwt.Claims{
		Issuer:   spiffeID,
		Subject:  spiffeID,
		Audience: jwt.Audience{audience},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(5 * time.Minute)),
		ID:       uuid.NewString(),
	}
	return jwt.Signed(signer).Claims(claims).Serialize()
}

func assertionAlg(key crypto.Signer) (jose.SignatureAlgorithm, error) {
	switch key.(type) {
	case *ecdsa.PrivateKey:
		return jose.ES256, nil
	case *rsa.PrivateKey:
		return jose.PS256, nil
	default:
		return "", fmt.Errorf("unsupported assertion key type %T", key)
	}
}
