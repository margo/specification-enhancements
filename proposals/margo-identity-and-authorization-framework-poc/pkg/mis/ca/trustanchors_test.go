package ca_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/bundlemap"
	"github.com/margo/miaf-poc/pkg/mis/ca"
)

func TestTrustAnchors_LoadSingleRoot(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "root.crt")

	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(42),
		Subject:               pkix.Name{CommonName: "test root"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(rootPath, pemBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	anchors, err := ca.NewTrustAnchors(ca.TrustAnchorsConfig{
		TrustDomain: "margo.example.com",
		RootFile:    rootPath,
	})
	if err != nil {
		t.Fatalf("NewTrustAnchors: %v", err)
	}

	set, refreshed, err := anchors.GetTrustAnchors(context.Background())
	if err != nil {
		t.Fatalf("GetTrustAnchors: %v", err)
	}
	if set.TrustDomain != "margo.example.com" {
		t.Errorf("TrustDomain = %q", set.TrustDomain)
	}
	if len(set.X509Authorities) != 1 {
		t.Fatalf("X509Authorities = %d, want 1", len(set.X509Authorities))
	}
	if set.X509Authorities[0].SerialNumber.Cmp(big.NewInt(42)) != 0 {
		t.Error("loaded cert does not match")
	}
	if refreshed.IsZero() {
		t.Error("refreshed time is zero")
	}
}

func TestTrustAnchors_MultipleRootsInOneFile(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "roots.crt")

	var pemBuf []byte
	for i := int64(1); i <= 2; i++ {
		key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		tmpl := &x509.Certificate{
			SerialNumber:          big.NewInt(i),
			NotBefore:             time.Now().Add(-time.Minute),
			NotAfter:              time.Now().Add(24 * time.Hour),
			IsCA:                  true,
			KeyUsage:              x509.KeyUsageCertSign,
			BasicConstraintsValid: true,
		}
		der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
		pemBuf = append(pemBuf, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	}
	if err := os.WriteFile(rootPath, pemBuf, 0o644); err != nil {
		t.Fatal(err)
	}

	anchors, err := ca.NewTrustAnchors(ca.TrustAnchorsConfig{TrustDomain: "x", RootFile: rootPath})
	if err != nil {
		t.Fatalf("NewTrustAnchors: %v", err)
	}
	set, _, _ := anchors.GetTrustAnchors(context.Background())
	if len(set.X509Authorities) != 2 {
		t.Errorf("X509Authorities = %d, want 2 (root rotation scenario)", len(set.X509Authorities))
	}
}

func TestTrustAnchors_MissingFile(t *testing.T) {
	_, err := ca.NewTrustAnchors(ca.TrustAnchorsConfig{
		TrustDomain: "x",
		RootFile:    "/no/such/file",
	})
	if err == nil {
		t.Fatal("expected error for missing root file")
	}
}

func TestTrustAnchors_PublishesJWTAuthority(t *testing.T) {
	// Build a self-signed root cert so NewTrustAnchors's PEM read succeeds.
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("rootKey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	rootPath := filepath.Join(t.TempDir(), "root.pem")
	if err := os.WriteFile(rootPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write root.pem: %v", err)
	}

	// Independent ECDSA key for the JWT authority entry.
	jwtKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	a, err := ca.NewTrustAnchors(ca.TrustAnchorsConfig{
		TrustDomain:    "td",
		RootFile:       rootPath,
		JWTAuthorities: []bundlemap.JWTAuthority{{KeyID: "kid-1", PublicKey: &jwtKey.PublicKey}},
	})
	if err != nil {
		t.Fatalf("NewTrustAnchors: %v", err)
	}
	set, _, _ := a.GetTrustAnchors(context.Background())
	if len(set.JWTAuthorities) != 1 || set.JWTAuthorities[0].KeyID != "kid-1" {
		t.Errorf("JWTAuthorities = %+v, want one entry with kid=kid-1", set.JWTAuthorities)
	}
}
