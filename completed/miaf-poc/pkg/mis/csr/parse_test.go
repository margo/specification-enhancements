package csr_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"testing"

	"github.com/margo/miaf-poc/pkg/mis/csr"
)

func mustBuildCSR(t *testing.T, key any, sigAlg x509.SignatureAlgorithm) []byte {
	t.Helper()
	tmpl := &x509.CertificateRequest{
		Subject:            pkix.Name{CommonName: "test"},
		SignatureAlgorithm: sigAlg,
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	if err != nil {
		t.Fatalf("CreateCertificateRequest: %v", err)
	}
	return der
}

func TestParse_ECDSA_P256_OK(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der := mustBuildCSR(t, key, x509.ECDSAWithSHA256)
	enc := base64.StdEncoding.EncodeToString(der)

	got, err := csr.Parse(enc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.PublicKeyAlgorithm() != "ES256" {
		t.Errorf("algorithm = %q, want ES256", got.PublicKeyAlgorithm())
	}
	if len(got.DER()) == 0 {
		t.Error("DER bytes empty")
	}
}

func TestParse_RSA_PSS_3072_OK(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 3072)
	der := mustBuildCSR(t, key, x509.SHA256WithRSAPSS)
	enc := base64.StdEncoding.EncodeToString(der)

	got, err := csr.Parse(enc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.PublicKeyAlgorithm() != "PS256" {
		t.Errorf("algorithm = %q, want PS256", got.PublicKeyAlgorithm())
	}
}

func TestParse_RejectsPEMArmor(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der := mustBuildCSR(t, key, x509.ECDSAWithSHA256)
	armored := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})

	_, err := csr.Parse(base64.StdEncoding.EncodeToString(armored))
	if !errors.Is(err, csr.ErrPEMArmor) {
		t.Errorf("err = %v, want ErrPEMArmor", err)
	}
}

func TestParse_RejectsMalformedBase64(t *testing.T) {
	_, err := csr.Parse("!!!not-base64!!!")
	if !errors.Is(err, csr.ErrMalformed) {
		t.Errorf("err = %v, want ErrMalformed", err)
	}
}

func TestParse_RejectsRSA2048_TooShort(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	der := mustBuildCSR(t, key, x509.SHA256WithRSAPSS)
	_, err := csr.Parse(base64.StdEncoding.EncodeToString(der))
	if !errors.Is(err, csr.ErrUnsupportedKey) {
		t.Errorf("err = %v, want ErrUnsupportedKey", err)
	}
}

func TestParse_RejectsRSAPKCS1v15(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 3072)
	der := mustBuildCSR(t, key, x509.SHA256WithRSA)
	_, err := csr.Parse(base64.StdEncoding.EncodeToString(der))
	if !errors.Is(err, csr.ErrUnsupportedAlg) {
		t.Errorf("err = %v, want ErrUnsupportedAlg (RS256/PKCS#1 v1.5 forbidden)", err)
	}
}

func TestParse_RejectsEd25519(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	der := mustBuildCSR(t, key, x509.PureEd25519)
	_, err := csr.Parse(base64.StdEncoding.EncodeToString(der))
	if !errors.Is(err, csr.ErrUnsupportedKey) {
		t.Errorf("err = %v, want ErrUnsupportedKey", err)
	}
}

func TestParse_RejectsBadSignature(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der := mustBuildCSR(t, key, x509.ECDSAWithSHA256)
	// Corrupt the last byte of the signature.
	der[len(der)-1] ^= 0xff

	_, err := csr.Parse(base64.StdEncoding.EncodeToString(der))
	if !errors.Is(err, csr.ErrBadSignature) {
		t.Errorf("err = %v, want ErrBadSignature", err)
	}
}
