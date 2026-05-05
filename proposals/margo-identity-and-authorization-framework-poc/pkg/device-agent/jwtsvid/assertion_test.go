package jwtsvid_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/margo/miaf-poc/pkg/device-agent/jwtsvid"
)

func TestBuildClientAssertion_RoundTrip(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}

	spiffeID := "spiffe://margo.example.com/device/test-1"
	audience := "https://mis.example.com/api/v1/identities/c3BpZmZlOi8v/jwt-svid"
	now := time.Now().UTC()

	compact, err := jwtsvid.BuildClientAssertion(key, spiffeID, audience, now)
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
