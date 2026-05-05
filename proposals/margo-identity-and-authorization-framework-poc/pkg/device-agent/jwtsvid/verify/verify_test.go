package verify_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/spiffe/go-spiffe/v2/spiffeid"

	"github.com/margo/miaf-poc/pkg/device-agent/jwtsvid/verify"
	"github.com/margo/miaf-poc/pkg/mis/bundlemap"
	"github.com/margo/miaf-poc/pkg/mis/jwtsigner"
)

func writeECDSAKey(t *testing.T) string {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	path := filepath.Join(t.TempDir(), "jwt.key")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return path
}

func TestSourceFromBundleMap_ParseAndValidate_HappyPath(t *testing.T) {
	keyPath := writeECDSAKey(t)
	signer, err := jwtsigner.New(jwtsigner.Config{KeyFile: keyPath})
	if err != nil {
		t.Fatalf("jwtsigner.New: %v", err)
	}

	trustDomainStr := "margo.example.com"
	spiffeID := "spiffe://" + trustDomainStr + "/device/test-1"
	audience := []string{"https://dfm.example/"}

	bundleJSON, err := bundlemap.Build(bundlemap.TrustAnchorSet{
		TrustDomain:    trustDomainStr,
		JWTAuthorities: []bundlemap.JWTAuthority{signer.JWTAuthority()},
	})
	if err != nil {
		t.Fatalf("bundlemap.Build: %v", err)
	}

	now := time.Now().UTC()
	compact, err := signer.Sign(context.Background(), jwt.Claims{
		Issuer:   "https://mis.example/",
		Subject:  spiffeID,
		Audience: jwt.Audience(audience),
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(5 * time.Minute)),
	})
	if err != nil {
		t.Fatalf("signer.Sign: %v", err)
	}

	td, err := spiffeid.TrustDomainFromString(trustDomainStr)
	if err != nil {
		t.Fatalf("TrustDomainFromString: %v", err)
	}

	bundle, err := verify.SourceFromBundleMap(bundleJSON, td)
	if err != nil {
		t.Fatalf("SourceFromBundleMap: %v", err)
	}

	svid, err := verify.ParseAndValidate(context.Background(), bundle, audience, compact)
	if err != nil {
		t.Fatalf("ParseAndValidate: %v", err)
	}
	if svid.ID.String() != spiffeID {
		t.Errorf("svid.ID = %q, want %q", svid.ID.String(), spiffeID)
	}
	if len(svid.Audience) != 1 || svid.Audience[0] != audience[0] {
		t.Errorf("svid.Audience = %v, want %v", svid.Audience, audience)
	}
}

func TestSourceFromBundleMap_ParseAndValidate_WrongAudienceFails(t *testing.T) {
	keyPath := writeECDSAKey(t)
	signer, err := jwtsigner.New(jwtsigner.Config{KeyFile: keyPath})
	if err != nil {
		t.Fatalf("jwtsigner.New: %v", err)
	}

	trustDomainStr := "margo.example.com"
	spiffeID := "spiffe://" + trustDomainStr + "/device/test-2"

	bundleJSON, err := bundlemap.Build(bundlemap.TrustAnchorSet{
		TrustDomain:    trustDomainStr,
		JWTAuthorities: []bundlemap.JWTAuthority{signer.JWTAuthority()},
	})
	if err != nil {
		t.Fatalf("bundlemap.Build: %v", err)
	}

	now := time.Now().UTC()
	compact, err := signer.Sign(context.Background(), jwt.Claims{
		Issuer:   "https://mis.example/",
		Subject:  spiffeID,
		Audience: jwt.Audience{"https://dfm.example/"},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(5 * time.Minute)),
	})
	if err != nil {
		t.Fatalf("signer.Sign: %v", err)
	}

	td, err := spiffeid.TrustDomainFromString(trustDomainStr)
	if err != nil {
		t.Fatalf("TrustDomainFromString: %v", err)
	}

	bundle, err := verify.SourceFromBundleMap(bundleJSON, td)
	if err != nil {
		t.Fatalf("SourceFromBundleMap: %v", err)
	}

	_, err = verify.ParseAndValidate(context.Background(), bundle, []string{"https://wrong-audience.example/"}, compact)
	if err == nil {
		t.Fatal("ParseAndValidate must fail for wrong audience; got nil error")
	}
}
