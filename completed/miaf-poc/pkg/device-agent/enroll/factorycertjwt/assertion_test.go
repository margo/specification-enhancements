package factorycertjwt_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/margo/miaf-poc/pkg/device-agent/enroll/factorycertjwt"
)

func TestBuildAssertion_PopulatesSUPClaims(t *testing.T) {
	leaf, key := mustLeaf(t)
	aud := "https://mis.example.com/api/v1/identities"

	a, err := factorycertjwt.BuildAssertion(factorycertjwt.AssertionInput{
		FactoryCerts: []*x509.Certificate{leaf},
		FactoryKey:   key,
		Audience:     aud,
	})
	if err != nil {
		t.Fatalf("BuildAssertion: %v", err)
	}

	parsed, err := jwt.ParseSigned(a, []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		t.Fatalf("ParseSigned: %v", err)
	}
	var claims jwt.Claims
	var raw map[string]any
	if err := parsed.Claims(&key.PublicKey, &claims, &raw); err != nil {
		t.Fatalf("Claims: %v", err)
	}

	fp := sha256.Sum256(leaf.Raw)
	want := "urn:margo:device:sha256:" + hex.EncodeToString(fp[:])
	if claims.Issuer != want {
		t.Errorf("iss = %q, want %q", claims.Issuer, want)
	}
	if claims.Subject != claims.Issuer {
		t.Errorf("sub = %q, want %q", claims.Subject, claims.Issuer)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != aud {
		t.Errorf("aud = %v, want [%q]", []string(claims.Audience), aud)
	}
	if claims.Expiry.Time().Sub(claims.IssuedAt.Time()) > 5*time.Minute {
		t.Error("lifetime exceeds 5 minutes")
	}
	if _, ok := raw["jti"].(string); !ok {
		t.Error("jti missing")
	}
}

func mustLeaf(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-dev"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	c, _ := x509.ParseCertificate(der)
	return c, k
}
