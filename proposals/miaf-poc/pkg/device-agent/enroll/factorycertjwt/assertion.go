// Package factorycertjwt drives the device-agent's factory-cert-JWT
// enrollment flow against MIS.
package factorycertjwt

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
)

// AssertionInput configures BuildAssertion. Lifetime defaults to 5 minutes
// (the SUP maximum) so MIS accepts the assertion under any operator clock
// within the allowed skew.
type AssertionInput struct {
	// FactoryCerts is the x5c chain to embed. FactoryCerts[0] MUST be the
	// factory leaf. Non-leaf entries are forwarded as-is.
	FactoryCerts []*x509.Certificate
	// FactoryKey signs the assertion. Must match FactoryCerts[0]'s public
	// key type (ECDSA P-256 or RSA-PSS 3072).
	FactoryKey crypto.Signer
	// Audience is the exact enrollment endpoint URL advertised by MIS.
	Audience string
	// Now overrides time.Now for tests.
	Now func() time.Time
	// Lifetime overrides the default 5-minute window.
	Lifetime time.Duration
}

// BuildAssertion returns a compact JWS suitable for
// bootstrapCredential.proof.assertion.
func BuildAssertion(in AssertionInput) (string, error) {
	if len(in.FactoryCerts) == 0 {
		return "", errors.New("factorycertjwt: FactoryCerts is required")
	}
	if in.FactoryKey == nil {
		return "", errors.New("factorycertjwt: FactoryKey is required")
	}
	if in.Audience == "" {
		return "", errors.New("factorycertjwt: Audience is required")
	}
	if in.Now == nil {
		in.Now = time.Now
	}
	if in.Lifetime == 0 {
		in.Lifetime = 5 * time.Minute
	}
	if in.Lifetime > 5*time.Minute {
		return "", fmt.Errorf("factorycertjwt: Lifetime %s exceeds maximum (5m)", in.Lifetime)
	}

	leaf := in.FactoryCerts[0]
	alg, err := algForKey(in.FactoryKey)
	if err != nil {
		return "", err
	}

	sopts := (&jose.SignerOptions{}).WithType("JWT")
	x5c := make([]string, 0, len(in.FactoryCerts))
	for _, c := range in.FactoryCerts {
		x5c = append(x5c, base64.StdEncoding.EncodeToString(c.Raw))
	}
	sopts = sopts.WithHeader(jose.HeaderKey("x5c"), x5c)

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: alg, Key: in.FactoryKey}, sopts)
	if err != nil {
		return "", fmt.Errorf("factorycertjwt: NewSigner: %w", err)
	}

	fp := sha256.Sum256(leaf.Raw)
	urn := "urn:margo:device:sha256:" + hex.EncodeToString(fp[:])
	now := in.Now().UTC()
	compact, err := jwt.Signed(signer).Claims(jwt.Claims{
		Issuer:   urn,
		Subject:  urn,
		Audience: jwt.Audience{in.Audience},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(in.Lifetime)),
		ID:       uuid.NewString(),
	}).Serialize()
	if err != nil {
		return "", fmt.Errorf("factorycertjwt: serialize: %w", err)
	}
	return compact, nil
}

func algForKey(k crypto.Signer) (jose.SignatureAlgorithm, error) {
	switch k.(type) {
	case *ecdsa.PrivateKey:
		return jose.ES256, nil
	case *rsa.PrivateKey:
		return jose.PS256, nil
	default:
		return "", fmt.Errorf("factorycertjwt: unsupported factory key type %T", k)
	}
}
