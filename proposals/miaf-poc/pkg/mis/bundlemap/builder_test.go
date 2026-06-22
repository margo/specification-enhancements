package bundlemap_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/bundlemap"
)

// newTestCert fabricates a self-signed P-256 certificate.
func newTestCert(t *testing.T) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	return cert
}

func TestBuild_MinimalBundle(t *testing.T) {
	trustDomain := "margo.example.com"
	cert := newTestCert(t)

	set := bundlemap.TrustAnchorSet{
		TrustDomain:     trustDomain,
		X509Authorities: []*x509.Certificate{cert},
	}

	got, err := bundlemap.Build(set)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Assert top-level envelope has "trust_domains" key (SPIFFE §5.1.1).
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(got, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	tdRaw, ok := envelope["trust_domains"]
	if !ok {
		t.Fatalf("envelope missing trust_domains key; got keys %v", keys(envelope))
	}

	// Assert trust_domains[trustDomain] exists.
	var tdMap map[string]json.RawMessage
	if err := json.Unmarshal(tdRaw, &tdMap); err != nil {
		t.Fatalf("unmarshal trust_domains: %v", err)
	}
	entryRaw, ok := tdMap[trustDomain]
	if !ok {
		t.Fatalf("trust_domains missing entry for %q; got keys %v", trustDomain, keys(tdMap))
	}

	// Assert the bundle entry has one JWK with the expected fields.
	var entry struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(entryRaw, &entry); err != nil {
		t.Fatalf("unmarshal entry: %v", err)
	}
	if len(entry.Keys) != 1 {
		t.Fatalf("keys len = %d, want 1", len(entry.Keys))
	}
	jwk := entry.Keys[0]
	if got := jwk["kty"]; got != "EC" {
		t.Errorf("kty = %v, want EC", got)
	}
	if got := jwk["use"]; got != "x509-svid" {
		t.Errorf("use = %v, want x509-svid", got)
	}
	if got := jwk["crv"]; got != "P-256" {
		t.Errorf("crv = %v, want P-256", got)
	}
	if got, _ := jwk["x"].(string); got == "" {
		t.Error("x coordinate missing or empty")
	}
	if got, _ := jwk["y"].(string); got == "" {
		t.Error("y coordinate missing or empty")
	}
	x5c, _ := jwk["x5c"].([]any)
	if len(x5c) == 0 {
		t.Fatal("x5c missing or empty")
	}
	if cert0, _ := x5c[0].(string); cert0 == "" {
		t.Error("x5c[0] is empty")
	}
}

func TestBuild_OmitsRefreshHint(t *testing.T) {
	cert := newTestCert(t)
	set := bundlemap.TrustAnchorSet{
		TrustDomain:     "margo.example.com",
		X509Authorities: []*x509.Certificate{cert},
		// SequenceNumber intentionally not set - TrustAnchorSet has no
		// RefreshHint field; verify the JSON output also omits it.
	}

	got, err := bundlemap.Build(set)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Unmarshal the bundle entry and verify spiffe_refresh_hint is absent.
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(got, &envelope); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var tdMap map[string]json.RawMessage
	if err := json.Unmarshal(envelope["trust_domains"], &tdMap); err != nil {
		t.Fatalf("unmarshal trust_domains: %v", err)
	}
	var entry map[string]json.RawMessage
	if err := json.Unmarshal(tdMap["margo.example.com"], &entry); err != nil {
		t.Fatalf("unmarshal entry: %v", err)
	}
	if _, present := entry["spiffe_refresh_hint"]; present {
		t.Error("spiffe_refresh_hint must be omitted from Bundle Map entries (SPIFFE §5.1.1)")
	}
}

func TestBuild_JWTAuthority(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	set := bundlemap.TrustAnchorSet{
		TrustDomain: "margo.example.com",
		JWTAuthorities: []bundlemap.JWTAuthority{
			{KeyID: "kid-1", PublicKey: &key.PublicKey},
		},
	}

	got, err := bundlemap.Build(set)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Drill down to the single JWK entry.
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(got, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	var tdMap map[string]json.RawMessage
	if err := json.Unmarshal(envelope["trust_domains"], &tdMap); err != nil {
		t.Fatalf("unmarshal trust_domains: %v", err)
	}
	var entry struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(tdMap["margo.example.com"], &entry); err != nil {
		t.Fatalf("unmarshal entry: %v", err)
	}
	if len(entry.Keys) != 1 {
		t.Fatalf("keys len = %d, want 1", len(entry.Keys))
	}
	jwk := entry.Keys[0]

	if got := jwk["use"]; got != "jwt-svid" {
		t.Errorf("use = %v, want jwt-svid", got)
	}
	if got := jwk["kid"]; got != "kid-1" {
		t.Errorf("kid = %v, want kid-1", got)
	}
	if got := jwk["kty"]; got != "EC" {
		t.Errorf("kty = %v, want EC", got)
	}
	if got := jwk["crv"]; got != "P-256" {
		t.Errorf("crv = %v, want P-256", got)
	}
	if _, present := jwk["x5c"]; present {
		t.Error("x5c must be absent from JWT authority JWKs")
	}
}

func keys[K comparable, V any](m map[K]V) []K {
	ks := make([]K, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
