package identity_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/margo/miaf-poc/pkg/mis/identity"
)

type stubJWTSigner struct {
	compact string
	err     error
}

func (s *stubJWTSigner) Sign(_ context.Context, _ jwt.Claims) (string, error) {
	return s.compact, s.err
}

type stubJWTAudit struct{ recorded []identity.IssuedJWTSVIDRecord }

func (s *stubJWTAudit) Record(_ context.Context, rec identity.IssuedJWTSVIDRecord) error {
	s.recorded = append(s.recorded, rec)
	return nil
}

func TestIssueJWTSVID_HappyPath(t *testing.T) {
	ldi := identity.LDI{UUID: "u1", SPIFFEID: "spiffe://td/margo/device/u1", Status: identity.LDIStatusActive}
	svc, _, _ := newTestService(t, &renewStubRepo{ldi: ldi}, &renewStubIssuer{},
		identity.StaticPolicy{KeyRotation: true}, identity.NoReplacements{})

	signer := &stubJWTSigner{compact: "eyJ.fake.sig"}
	audit := &stubJWTAudit{}
	res, err := svc.IssueJWTSVID(context.Background(), signer, audit,
		identity.JWTSVIDConfig{
			Issuer:     "https://mis.example/",
			DefaultTTL: 5 * time.Minute,
			MaxTTL:     time.Hour,
		},
		ldi.SPIFFEID, []string{"https://aud/"}, 0)
	if err != nil {
		t.Fatalf("IssueJWTSVID: %v", err)
	}
	if res.JWT != "eyJ.fake.sig" {
		t.Errorf("JWT = %q", res.JWT)
	}
	if len(audit.recorded) != 1 || audit.recorded[0].SPIFFEID != ldi.SPIFFEID {
		t.Errorf("audit not recorded")
	}
}

func TestIssueJWTSVID_UnknownSPIFFEID(t *testing.T) {
	svc, _, _ := newTestService(t, &renewStubRepo{err: identity.ErrNotFound}, &renewStubIssuer{},
		identity.StaticPolicy{KeyRotation: true}, identity.NoReplacements{})

	_, err := svc.IssueJWTSVID(context.Background(), &stubJWTSigner{}, &stubJWTAudit{},
		identity.JWTSVIDConfig{Issuer: "https://x/", DefaultTTL: time.Minute, MaxTTL: time.Hour},
		"spiffe://td/x", []string{"https://aud/"}, 0)
	if !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestIssueJWTSVID_TTLCap(t *testing.T) {
	ldi := identity.LDI{UUID: "u1", SPIFFEID: "spiffe://td/margo/device/u1", Status: identity.LDIStatusActive}
	svc, _, _ := newTestService(t, &renewStubRepo{ldi: ldi}, &renewStubIssuer{},
		identity.StaticPolicy{KeyRotation: true}, identity.NoReplacements{})
	identity.SetServiceNow(svc, func() time.Time {
		return time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	})

	audit := &stubJWTAudit{}
	res, _ := svc.IssueJWTSVID(context.Background(), &stubJWTSigner{compact: "x"}, audit,
		identity.JWTSVIDConfig{Issuer: "https://x/", DefaultTTL: time.Minute, MaxTTL: time.Hour},
		ldi.SPIFFEID, []string{"https://aud/"}, 24*time.Hour) // request a day, should cap to 1h
	want := time.Date(2026, 4, 27, 1, 0, 0, 0, time.UTC)
	if !res.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %s, want %s", res.ExpiresAt, want)
	}
}
