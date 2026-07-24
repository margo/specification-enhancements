package jwtsigner_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/margo/miaf-poc/pkg/mis/jwtsigner"
)

func writeECDSAKey(t *testing.T, dir, name string) string {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func writeRSAKey(t *testing.T, dir, name string, bits int) string {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestNewSigner_LoadsAndSigns(t *testing.T) {
	keyPath := writeECDSAKey(t, t.TempDir(), "jwt.key")
	signer, err := jwtsigner.New(jwtsigner.Config{KeyFile: keyPath})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if signer.Algorithm() != jose.ES256 {
		t.Errorf("alg = %s, want ES256", signer.Algorithm())
	}
	if signer.KeyID() == "" {
		t.Error("kid empty")
	}

	compact, err := signer.Sign(context.Background(), jwt.Claims{
		Subject:  "spiffe://td/margo/device/u1",
		Audience: jwt.Audience{"https://dfm.example/"},
		Issuer:   "https://mis.example/",
		IssuedAt: jwt.NewNumericDate(time.Now()),
		Expiry:   jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if compact == "" {
		t.Error("Sign returned empty compact JWT")
	}
}

func TestNewSigner_RejectsRSAUnder3072(t *testing.T) {
	keyPath := writeRSAKey(t, t.TempDir(), "weak.key", 2048)
	_, err := jwtsigner.New(jwtsigner.Config{KeyFile: keyPath})
	if err == nil {
		t.Fatal("expected error for 2048-bit RSA key, got nil")
	}
	if !strings.Contains(err.Error(), "rsa key too small") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewSigner_AcceptsRSA3072(t *testing.T) {
	if testing.Short() {
		t.Skip("RSA 3072 keygen is slow; -short skips")
	}
	keyPath := writeRSAKey(t, t.TempDir(), "strong.key", 3072)
	signer, err := jwtsigner.New(jwtsigner.Config{KeyFile: keyPath})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if signer.Algorithm() != jose.PS256 {
		t.Errorf("alg = %s, want PS256", signer.Algorithm())
	}
}

func TestSigner_JWTAuthority(t *testing.T) {
	keyPath := writeECDSAKey(t, t.TempDir(), "jwt.key")
	signer, _ := jwtsigner.New(jwtsigner.Config{KeyFile: keyPath})
	a := signer.JWTAuthority()
	if a.KeyID == "" {
		t.Error("JWTAuthority.KeyID empty")
	}
	if a.PublicKey == nil {
		t.Error("JWTAuthority.PublicKey nil")
	}
}
