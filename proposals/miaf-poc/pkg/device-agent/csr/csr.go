// Package csr builds MIAF-conformant PKCS#10 enrollment requests.
//
// The MIS ignores the Subject DN and any SANs on the submitted CSR (SUP §5
// X.509 SVID profile validation), so the template carries only the public
// key and signature algorithm.  The returned string is the base64 standard
// encoding of the DER bytes - ready for insertion into svid_request.csr.
package csr

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
)

// Build returns the base64-encoded DER PKCS#10 for the given key.
func Build(key crypto.PrivateKey) (string, error) {
	alg, err := signatureAlgorithmFor(key)
	if err != nil {
		return "", err
	}
	tmpl := &x509.CertificateRequest{
		Subject:            pkix.Name{CommonName: "margo-device"},
		SignatureAlgorithm: alg,
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(der), nil
}

func signatureAlgorithmFor(key crypto.PrivateKey) (x509.SignatureAlgorithm, error) {
	switch key.(type) {
	case *ecdsa.PrivateKey:
		return x509.ECDSAWithSHA256, nil
	case *rsa.PrivateKey:
		return x509.SHA256WithRSAPSS, nil
	}
	return 0, errors.New("csr: unsupported key type")
}
