package jwtsvid

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
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
// audience MUST be the exact URL of the JWT SVID exchange endpoint
// ("<mis-base>/api/v1/identities/<spiffeIdEncoded>/jwt-svid").
//
// The JWS alg is selected from the key type: ES256 for ECDSA, PS256 for RSA.
func BuildClientAssertion(key crypto.Signer, spiffeID, audience string, now time.Time) (string, error) {
	alg, err := assertionAlg(key)
	if err != nil {
		return "", err
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: alg, Key: key},
		(&jose.SignerOptions{}).WithType("JWT"))
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
