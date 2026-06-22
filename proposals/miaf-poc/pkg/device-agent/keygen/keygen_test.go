package keygen_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"testing"

	"github.com/margo/miaf-poc/pkg/device-agent/keygen"
)

func TestGenerateES256(t *testing.T) {
	key, err := keygen.Generate(keygen.ES256)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	k, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatalf("type = %T, want *ecdsa.PrivateKey", key)
	}
	if k.Curve != elliptic.P256() {
		t.Errorf("curve = %s, want P-256", k.Curve.Params().Name)
	}
}

func TestGeneratePS256(t *testing.T) {
	key, err := keygen.Generate(keygen.PS256)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	k, ok := key.(*rsa.PrivateKey)
	if !ok {
		t.Fatalf("type = %T, want *rsa.PrivateKey", key)
	}
	if k.N.BitLen() < 3072 {
		t.Errorf("modulus = %d, want >= 3072", k.N.BitLen())
	}
}

func TestGenerateUnknown(t *testing.T) {
	if _, err := keygen.Generate("HS256"); err == nil {
		t.Error("expected error for unsupported algorithm")
	}
}
