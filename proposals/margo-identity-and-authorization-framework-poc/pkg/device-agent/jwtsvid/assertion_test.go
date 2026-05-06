package jwtsvid_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/margo/miaf-poc/pkg/device-agent/jwtsvid"
)

func makeChain(t *testing.T, spiffeID string) ([]*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("root keygen: %v", err)
	}
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("root sign: %v", err)
	}
	root, _ := x509.ParseCertificate(rootDER)

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("leaf keygen: %v", err)
	}
	u, _ := url.Parse(spiffeID)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		URIs:         []*url.URL{u},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, root, &leafKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("leaf sign: %v", err)
	}
	leaf, _ := x509.ParseCertificate(leafDER)
	return []*x509.Certificate{leaf, root}, leafKey
}

func TestBuildClientAssertion_RoundTrip(t *testing.T) {
	spiffeID := "spiffe://margo.example.com/device/test-1"
	audience := "https://mis.example.com/api/v1/identities/c3BpZmZlOi8v/jwt-svid"
	now := time.Now().UTC()

	chain, key := makeChain(t, spiffeID)

	compact, err := jwtsvid.BuildClientAssertion(key, chain, spiffeID, audience, now)
	if err != nil {
		t.Fatalf("BuildClientAssertion: %v", err)
	}
	if compact == "" {
		t.Fatal("BuildClientAssertion returned empty string")
	}

	parsed, err := jwt.ParseSigned(compact, []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		t.Fatalf("ParseSigned: %v", err)
	}

	// x5c header is required by SUP §"JWT SVID Exchange Endpoint" Client
	// Assertion JWT and must contain the full chain.
	if len(parsed.Headers) != 1 {
		t.Fatalf("expected 1 JWS header, got %d", len(parsed.Headers))
	}
	got, err := parsed.Headers[0].Certificates(x509.VerifyOptions{
		Roots: rootPool(t, chain[len(chain)-1]),
	})
	if err != nil {
		t.Fatalf("decode/verify x5c: %v", err)
	}
	if len(got) == 0 || len(got[0]) != len(chain) {
		t.Fatalf("x5c chain len = %d, want %d", len(got[0]), len(chain))
	}
	for i := range chain {
		if got[0][i].SerialNumber.Cmp(chain[i].SerialNumber) != 0 {
			t.Errorf("x5c[%d] serial mismatch", i)
		}
	}
	// Sanity: x5c is base64-StdEncoded DER per RFC 7517 §4.7. go-jose stores
	// the parsed certs internally, so verify by inspecting the raw header
	// indirectly via Certificates() above (which round-trips through the
	// base64 codec).
	_ = base64.StdEncoding // reference base64 to keep the import alive

	var claims jwt.Claims
	var raw map[string]any
	if err := parsed.Claims(&key.PublicKey, &claims, &raw); err != nil {
		t.Fatalf("Claims: %v", err)
	}

	if claims.Issuer != spiffeID {
		t.Errorf("iss = %q, want %q", claims.Issuer, spiffeID)
	}
	if claims.Subject != spiffeID {
		t.Errorf("sub = %q, want %q", claims.Subject, spiffeID)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != audience {
		t.Errorf("aud = %v, want [%q]", []string(claims.Audience), audience)
	}
	jti, ok := raw["jti"].(string)
	if !ok || jti == "" {
		t.Error("jti missing or empty")
	}
	if claims.Expiry == nil || claims.IssuedAt == nil {
		t.Fatal("exp or iat missing")
	}
	if claims.Expiry.Time().Sub(claims.IssuedAt.Time()) > 5*time.Minute {
		t.Error("lifetime exceeds 5 minutes")
	}
}

func TestBuildClientAssertion_RejectsEmptyChain(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if _, err := jwtsvid.BuildClientAssertion(key, nil, "spiffe://x/y", "https://aud/", time.Now()); err == nil {
		t.Fatal("BuildClientAssertion(nil chain) succeeded; want error")
	}
}

func rootPool(t *testing.T, root *x509.Certificate) *x509.CertPool {
	t.Helper()
	p := x509.NewCertPool()
	p.AddCert(root)
	return p
}
