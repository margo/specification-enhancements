// Package keygen produces MIAF-conformant private keys for enrollment and
// renewal.  Only ES256 (ECDSA P-256) and PS256 (RSA-PSS 3072) are supported,
// matching SUP Cryptographic Requirements.
package keygen

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
)

// Algorithm selects a key type for Generate.
type Algorithm string

const (
	ES256 Algorithm = "ES256"
	PS256 Algorithm = "PS256"
)

// Generate returns a fresh private key of the requested type.
func Generate(alg Algorithm) (crypto.PrivateKey, error) {
	switch alg {
	case ES256:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case PS256:
		return rsa.GenerateKey(rand.Reader, 3072)
	default:
		return nil, fmt.Errorf("keygen: unsupported algorithm %q", alg)
	}
}
