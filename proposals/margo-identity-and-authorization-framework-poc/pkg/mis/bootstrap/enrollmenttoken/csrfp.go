package enrollmenttoken

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
)

// csrPubKeyFingerprint returns SHA-256 over the DER-encoded SubjectPublicKeyInfo
// extracted from the base64-encoded DER CSR. Two CSRs minted from the same
// private key produce equal fingerprints regardless of Subject or SAN differences.
func csrPubKeyFingerprint(csrB64 string) ([]byte, error) {
	der, err := base64.StdEncoding.DecodeString(csrB64)
	if err != nil {
		return nil, fmt.Errorf("enrollmenttoken: csr base64 decode: %w", err)
	}
	csr, err := x509.ParseCertificateRequest(der)
	if err != nil {
		return nil, fmt.Errorf("enrollmenttoken: parse CSR: %w", err)
	}
	spkiDER, err := x509.MarshalPKIXPublicKey(csr.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("enrollmenttoken: marshal CSR public key: %w", err)
	}
	sum := sha256.Sum256(spkiDER)
	return sum[:], nil
}
