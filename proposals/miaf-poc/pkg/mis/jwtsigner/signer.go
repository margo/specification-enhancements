// Package jwtsigner owns MIS's JWT SVID signing key.
//
// MIS loads a single ECDSA P-256 (or RSA-PSS ≥ 3072) private key from disk at
// startup and keeps it for the life of the process. The signer is consumed by
// the JWT SVID Exchange handler (POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid)
// and its public key is published in the SPIFFE Bundle Map under
// `keys[].use="jwt-svid"`.
//
// SUP §"JWT SVID Profile" pins the verification material into the Trust Bundle.
// SUP §"Cryptographic Requirements" allows ES256 or PS256; RS256 is forbidden.
//
// Rotation is out of PoC scope. Production deployments should source the key
// from an HSM or KMS.
package jwtsigner

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/margo/miaf-poc/pkg/mis/bundlemap"
)

// Config parametrises New.
type Config struct {
	// KeyFile is a PEM file containing a PKCS#8-wrapped ECDSA P-256 or
	// RSA-PSS ≥ 3072 private key.
	KeyFile string
}

// Signer is MIS's JWT SVID issuance primitive.
type Signer struct {
	priv crypto.Signer
	alg  jose.SignatureAlgorithm
	pub  crypto.PublicKey
	kid  string
}

// New loads the signing key from disk and validates it against the SUP's
// cryptographic requirements.
func New(cfg Config) (*Signer, error) {
	if cfg.KeyFile == "" {
		return nil, errors.New("jwtsigner: KeyFile is required")
	}
	priv, err := loadKey(cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("jwtsigner: load key: %w", err)
	}
	alg, err := algorithmFor(priv)
	if err != nil {
		return nil, fmt.Errorf("jwtsigner: %w", err)
	}
	pub := priv.Public()
	kid, err := kidFromPublic(pub)
	if err != nil {
		return nil, fmt.Errorf("jwtsigner: kid: %w", err)
	}
	return &Signer{priv: priv, alg: alg, pub: pub, kid: kid}, nil
}

// Algorithm returns the JWS algorithm.
func (s *Signer) Algorithm() jose.SignatureAlgorithm { return s.alg }

// KeyID returns the JWK kid the public key is published under in the Bundle Map.
func (s *Signer) KeyID() string { return s.kid }

// JWTAuthority returns a Bundle-Map-ready JWT authority entry.
func (s *Signer) JWTAuthority() bundlemap.JWTAuthority {
	return bundlemap.JWTAuthority{KeyID: s.kid, PublicKey: s.pub}
}

// Sign builds a compact JWS over the given claims using kid as the JOSE
// `kid` header.
func (s *Signer) Sign(ctx context.Context, claims jwt.Claims) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: s.alg, Key: s.priv},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", s.kid),
	)
	if err != nil {
		return "", fmt.Errorf("jwtsigner: build jose signer: %w", err)
	}
	return jwt.Signed(signer).Claims(claims).Serialize()
}

func algorithmFor(priv crypto.Signer) (jose.SignatureAlgorithm, error) {
	switch k := priv.(type) {
	case *ecdsa.PrivateKey:
		if k.Curve != elliptic.P256() {
			return "", fmt.Errorf("ecdsa curve %q is not P-256", k.Curve.Params().Name)
		}
		return jose.ES256, nil
	case *rsa.PrivateKey:
		if k.N.BitLen() < 3072 {
			return "", fmt.Errorf("rsa key too small: %d bits (minimum 3072)", k.N.BitLen())
		}
		return jose.PS256, nil
	default:
		return "", fmt.Errorf("unsupported key type %T", priv)
	}
}

func kidFromPublic(pub crypto.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:]), nil
}

func loadKey(path string) (crypto.Signer, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("no PEM block in %s", path)
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		s, ok := k.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("PKCS#8 key in %s is not a crypto.Signer", path)
		}
		return s, nil
	}
	if k, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	return nil, fmt.Errorf("unsupported key format in %s", path)
}
