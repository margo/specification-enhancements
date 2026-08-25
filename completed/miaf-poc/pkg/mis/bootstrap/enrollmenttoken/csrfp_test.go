package enrollmenttoken

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"testing"
)

func TestCSRPubKeyFingerprint_SameKey_SameFingerprint(t *testing.T) {
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	a := mkCSR(t, k)
	b := mkCSR(t, k) // same key, fresh signature
	fa, err := csrPubKeyFingerprint(a)
	if err != nil {
		t.Fatalf("a: %v", err)
	}
	fb, err := csrPubKeyFingerprint(b)
	if err != nil {
		t.Fatalf("b: %v", err)
	}
	if !bytes.Equal(fa, fb) {
		t.Errorf("same key produced different fingerprints\n a=%x\n b=%x", fa, fb)
	}
	if len(fa) != 32 {
		t.Errorf("fingerprint len = %d, want 32", len(fa))
	}
}

func TestCSRPubKeyFingerprint_DifferentKey_DifferentFingerprint(t *testing.T) {
	k1, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	k2, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	f1, _ := csrPubKeyFingerprint(mkCSR(t, k1))
	f2, _ := csrPubKeyFingerprint(mkCSR(t, k2))
	if bytes.Equal(f1, f2) {
		t.Error("different keys produced the same fingerprint")
	}
}

func TestCSRPubKeyFingerprint_Malformed_ReturnsError(t *testing.T) {
	_, err := csrPubKeyFingerprint("!!!")
	if err == nil {
		t.Error("expected error for malformed base64")
	}
}

func mkCSR(t *testing.T, k *ecdsa.PrivateKey) string {
	t.Helper()
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, k)
	if err != nil {
		t.Fatalf("CreateCertificateRequest: %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}
