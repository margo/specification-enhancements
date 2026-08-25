package bootstrapjwt_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/margo/miaf-poc/test/testsupport/bootstrapjwt"
)

func TestMint_ValidAssertion_ParsesBack(t *testing.T) {
	leaf, key := mustECLeaf(t)
	aud := "https://mis.example.com/api/v1/identities"

	a := bootstrapjwt.Mint(t, leaf, []*x509.Certificate{leaf}, key, aud, bootstrapjwt.Options{})

	parsed, err := jwt.ParseSigned(a.Compact, []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		t.Fatalf("ParseSigned: %v", err)
	}
	var claims jwt.Claims
	if err := parsed.Claims(&key.PublicKey, &claims); err != nil {
		t.Fatalf("Claims: %v", err)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != aud {
		t.Errorf("aud = %v, want [%q]", []string(claims.Audience), aud)
	}
	if time.Until(claims.Expiry.Time()) > 5*time.Minute+time.Second {
		t.Errorf("expiry too far in the future: %s", claims.Expiry.Time())
	}
}

func mustECLeaf(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "t"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	c, _ := x509.ParseCertificate(der)
	return c, k
}
