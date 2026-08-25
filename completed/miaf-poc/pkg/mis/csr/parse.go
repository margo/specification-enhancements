// Package csr parses a device enrollment CSR per SUP §5 "X.509 SVID profile"
// validation rules and enforces MIAF Cryptographic Requirements.
//
// Only ECDSA on P-256 (ES256) and RSA-PSS with >= 3072-bit modulus (PS256) are
// accepted; see SUP §3 "Cryptographic Requirements".
package csr

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
)

// Error sentinels. Callers convert these to SUP Problem Details in the
// handler layer.
var (
	ErrPEMArmor       = errors.New("csr: PEM armor present; expected base64-encoded DER")
	ErrMalformed      = errors.New("csr: malformed base64 or ASN.1")
	ErrUnsupportedKey = errors.New("csr: unsupported public key type (require P-256 ECDSA or RSA-PSS >= 3072 bits)")
	ErrUnsupportedAlg = errors.New("csr: unsupported signature algorithm (require ES256 or PS256)")
	ErrBadSignature   = errors.New("csr: self-signature verification failed")
)

// CSR is the parsed, validated request.
type CSR struct {
	der       []byte
	inner     *x509.CertificateRequest
	algorithm string // "ES256" or "PS256"
}

// DER returns the original DER bytes of the request.
func (c *CSR) DER() []byte { return c.der }

// Inner returns the parsed *x509.CertificateRequest.
func (c *CSR) Inner() *x509.CertificateRequest { return c.inner }

// PublicKeyAlgorithm returns "ES256" or "PS256".
func (c *CSR) PublicKeyAlgorithm() string { return c.algorithm }

// Parse decodes the standard base64-encoded DER CSR from the enrollment body,
// rejects PEM armor and malformed inputs, parses the PKCS#10 structure,
// verifies the self-signature, and enforces MIAF Cryptographic Requirements
// on the public key and signature algorithm.
func Parse(encoded string) (*CSR, error) {
	if len(encoded) == 0 {
		return nil, fmt.Errorf("%w: empty input", ErrMalformed)
	}
	// PEM armor is explicitly rejected by the SUP.
	if hasPEMArmor(encoded) {
		return nil, ErrPEMArmor
	}
	der, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	// After base64-decode, still check for PEM-in-base64 (the client base64-
	// encoded an already-armored blob instead of raw DER).
	if hasPEMArmor(string(der)) {
		return nil, ErrPEMArmor
	}
	req, err := x509.ParseCertificateRequest(der)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	if err := req.CheckSignature(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadSignature, err)
	}

	alg, err := enforceMIAFCrypto(req)
	if err != nil {
		return nil, err
	}
	return &CSR{der: der, inner: req, algorithm: alg}, nil
}

// enforceMIAFCrypto checks the request's public key and signature algorithm
// against SUP §3 Cryptographic Requirements.
func enforceMIAFCrypto(req *x509.CertificateRequest) (string, error) {
	switch pub := req.PublicKey.(type) {
	case *ecdsa.PublicKey:
		if pub.Curve != elliptic.P256() {
			return "", fmt.Errorf("%w: ECDSA curve %q is not P-256", ErrUnsupportedKey, pub.Curve.Params().Name)
		}
		if req.SignatureAlgorithm != x509.ECDSAWithSHA256 {
			return "", fmt.Errorf("%w: ECDSA signature must be ES256, got %s", ErrUnsupportedAlg, req.SignatureAlgorithm)
		}
		return "ES256", nil
	case *rsa.PublicKey:
		if pub.N.BitLen() < 3072 {
			return "", fmt.Errorf("%w: RSA modulus %d < 3072", ErrUnsupportedKey, pub.N.BitLen())
		}
		if req.SignatureAlgorithm != x509.SHA256WithRSAPSS {
			return "", fmt.Errorf("%w: RSA signature must be PS256 (RSA-PSS + SHA-256), got %s", ErrUnsupportedAlg, req.SignatureAlgorithm)
		}
		return "PS256", nil
	default:
		return "", fmt.Errorf("%w: %T", ErrUnsupportedKey, req.PublicKey)
	}
}

// hasPEMArmor returns true if the input contains a "-----BEGIN" marker.
func hasPEMArmor(s string) bool {
	return len(s) >= 10 && containsASCII(s, "-----BEGIN")
}

func containsASCII(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
