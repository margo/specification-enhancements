package bundle_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/device-agent/bundle"
	"github.com/margo/miaf-poc/pkg/mis/bundlemap"
)

func TestExtractX509Authorities_RoundTripsCertsForTrustDomain(t *testing.T) {
	cert := selfSigned(t, "fake-root")
	body := mustBundleMap(t, "margo.example.com", [][]byte{cert.Raw})

	got, err := bundle.ExtractX509Authorities(body, "margo.example.com")
	if err != nil {
		t.Fatalf("ExtractX509Authorities: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d authorities, want 1", len(got))
	}
	if got[0].SerialNumber.Cmp(cert.SerialNumber) != 0 {
		t.Errorf("round-tripped cert has wrong serial")
	}
}

func TestExtractX509Authorities_MissingTrustDomain(t *testing.T) {
	body := mustBundleMap(t, "other.example.com", nil)
	if _, err := bundle.ExtractX509Authorities(body, "margo.example.com"); err == nil {
		t.Fatalf("expected error for missing trust domain; got nil")
	}
}

func TestExtractX509Authorities_IgnoresJWTAuthorities(t *testing.T) {
	cert := selfSigned(t, "fake-root")
	raw := map[string]any{
		"trust_domains": map[string]any{
			"margo.example.com": map[string]any{
				"keys": []any{
					map[string]any{"use": "jwt-svid", "kty": "EC", "crv": "P-256", "x": "aa", "y": "bb"},
					map[string]any{"use": "x509-svid", "x5c": []string{base64.StdEncoding.EncodeToString(cert.Raw)}},
				},
			},
		},
	}
	body, _ := json.Marshal(raw)

	got, err := bundle.ExtractX509Authorities(body, "margo.example.com")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d authorities, want 1 (jwt-svid entry must be skipped)", len(got))
	}
}

func selfSigned(t *testing.T, cn string) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	tpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	return cert
}

func TestExtractJWTAuthorities_RoundTrips(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	const td = "margo.example.com"
	const kid = "test-kid-1"

	bundleJSON, err := bundlemap.Build(bundlemap.TrustAnchorSet{
		TrustDomain: td,
		JWTAuthorities: []bundlemap.JWTAuthority{
			{KeyID: kid, PublicKey: &key.PublicKey},
		},
	})
	if err != nil {
		t.Fatalf("bundlemap.Build: %v", err)
	}

	got, err := bundle.ExtractJWTAuthorities(bundleJSON, td)
	if err != nil {
		t.Fatalf("ExtractJWTAuthorities: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d authorities, want 1", len(got))
	}
	if got[0].KeyID != kid {
		t.Errorf("KeyID = %q, want %q", got[0].KeyID, kid)
	}
	if got[0].PublicKey == nil {
		t.Error("PublicKey is nil")
	}
}

func TestExtractJWTAuthorities_MissingTrustDomain(t *testing.T) {
	bundleJSON, err := bundlemap.Build(bundlemap.TrustAnchorSet{
		TrustDomain: "other.example.com",
	})
	if err != nil {
		t.Fatalf("bundlemap.Build: %v", err)
	}
	if _, err := bundle.ExtractJWTAuthorities(bundleJSON, "margo.example.com"); err == nil {
		t.Fatal("expected error for missing trust domain; got nil")
	}
}

func TestExtractJWTAuthorities_SkipsX509Entries(t *testing.T) {
	cert := selfSigned(t, "root")
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	const td = "margo.example.com"

	bundleJSON, err := bundlemap.Build(bundlemap.TrustAnchorSet{
		TrustDomain:     td,
		X509Authorities: []*x509.Certificate{cert},
		JWTAuthorities:  []bundlemap.JWTAuthority{{KeyID: "k1", PublicKey: &key.PublicKey}},
	})
	if err != nil {
		t.Fatalf("bundlemap.Build: %v", err)
	}

	got, err := bundle.ExtractJWTAuthorities(bundleJSON, td)
	if err != nil {
		t.Fatalf("ExtractJWTAuthorities: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d authorities, want 1 (x509-svid entry must be skipped)", len(got))
	}
	if got[0].KeyID != "k1" {
		t.Errorf("KeyID = %q, want k1", got[0].KeyID)
	}
}

func mustBundleMap(t *testing.T, td string, der [][]byte) []byte {
	t.Helper()
	keys := make([]any, 0, len(der))
	for _, d := range der {
		keys = append(keys, map[string]any{
			"use": "x509-svid",
			"x5c": []string{base64.StdEncoding.EncodeToString(d)},
		})
	}
	envelope := map[string]any{
		"trust_domains": map[string]any{
			td: map[string]any{"keys": keys},
		},
	}
	b, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
