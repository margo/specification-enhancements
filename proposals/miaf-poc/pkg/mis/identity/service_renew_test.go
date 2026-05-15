package identity_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"math/big"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/csr"
	"github.com/margo/miaf-poc/pkg/mis/identity"
)

// renewStubRepo is a minimal Repository stub for Renew tests.
// (service_test.go already defines memRepo and stubIssuer; use distinct names.)
type renewStubRepo struct {
	ldi identity.LDI
	err error
}

func (s *renewStubRepo) LookupByESI(context.Context, []byte) (identity.ESIBinding, identity.LDI, error) {
	return identity.ESIBinding{}, identity.LDI{}, identity.ErrNotFound
}
func (s *renewStubRepo) CreateLDIAndBind(context.Context, []byte, string, string) (identity.LDI, error) {
	return identity.LDI{}, nil
}
func (s *renewStubRepo) Rebind(context.Context, string, []byte, string) (identity.ESIBinding, error) {
	return identity.ESIBinding{}, nil
}
func (s *renewStubRepo) GetLDI(context.Context, string) (identity.LDI, error) {
	return identity.LDI{}, nil
}
func (s *renewStubRepo) ListBindings(context.Context, string) ([]identity.ESIBinding, error) {
	return nil, nil
}
func (s *renewStubRepo) GetLDIBySPIFFEID(_ context.Context, _ string) (identity.LDI, error) {
	return s.ldi, s.err
}
func (s *renewStubRepo) SetLDIStatus(context.Context, string, identity.LDIStatus) error { return nil }

// renewStubIssuer records whether IssueX509SVID was called.
type renewStubIssuer struct {
	called bool
}

func (s *renewStubIssuer) IssueX509SVID(_ context.Context, spiffeID string, _ []byte, ttl time.Duration) ([][]byte, time.Time, error) {
	s.called = true
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	now := time.Now()
	notAfter := now.Add(ttl)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: spiffeID},
		NotBefore:    now,
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, time.Time{}, err
	}
	return [][]byte{der}, notAfter, nil
}

// fakeIssuerSerial returns a fresh real X.509 DER leaf on each call, with a
// monotonically increasing serial so tests can distinguish prior vs new.
// now is used as the NotBefore; NotAfter = now + ttl.
type fakeIssuerSerial struct {
	mu     sync.Mutex
	serial int64
	now    func() time.Time
}

func (f *fakeIssuerSerial) IssueX509SVID(ctx context.Context, spiffeID string, csrDER []byte, ttl time.Duration) ([][]byte, time.Time, error) {
	f.mu.Lock()
	f.serial++
	serial := f.serial
	f.mu.Unlock()

	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	base := time.Now()
	if f.now != nil {
		base = f.now()
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: spiffeID},
		NotBefore:    base,
		NotAfter:     base.Add(ttl),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, time.Time{}, err
	}
	return [][]byte{der}, tmpl.NotAfter, nil
}

func mkCSR(t *testing.T) *csr.CSR {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	return mkCSRForKey(t, key)
}

func mkCSRForKey(t *testing.T, key *ecdsa.PrivateKey) *csr.CSR {
	t.Helper()
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, key)
	if err != nil {
		t.Fatalf("CreateCertificateRequest: %v", err)
	}
	parsed, err := csr.Parse(base64.StdEncoding.EncodeToString(der))
	if err != nil {
		t.Fatalf("csr.Parse: %v", err)
	}
	return parsed
}

func seedActiveLeaf(t *testing.T, log *memIssuanceLog, spiffeID string, key *ecdsa.PrivateKey, when time.Time) {
	t.Helper()
	u, err := url.Parse(spiffeID)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(7),
		Subject:      pkix.Name{CommonName: spiffeID},
		NotBefore:    when.Add(-time.Minute),
		NotAfter:     when.Add(time.Hour),
		URIs:         []*url.URL{u},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	if err := log.Record(context.Background(), identity.IssuedSVIDRecord{
		SerialHex:      strings.ToUpper(tmpl.SerialNumber.Text(16)),
		FingerprintHex: "fp",
		SPIFFEID:       spiffeID,
		LDIUUID:        "u1",
		Method:         "renewal",
		IssuedAt:       when.UTC(),
		NotBefore:      tmpl.NotBefore.UTC(),
		NotAfter:       tmpl.NotAfter.UTC(),
		LeafDER:        der,
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
}

func TestService_Renew_HappyPath(t *testing.T) {
	ldi := identity.LDI{UUID: "u1", SPIFFEID: "spiffe://td/margo/device/u1", Status: identity.LDIStatusActive}
	svc, log, _ := newTestService(t, &renewStubRepo{ldi: ldi}, &renewStubIssuer{}, identity.StaticPolicy{KeyRotation: true}, identity.NoReplacements{})
	currentKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	seedActiveLeaf(t, log, ldi.SPIFFEID, currentKey, time.Now().UTC())
	newKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	res, err := svc.Renew(context.Background(), ldi.SPIFFEID, mkCSRForKey(t, newKey))
	if err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if res.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", res.StatusCode)
	}
	if res.LDI.UUID != ldi.UUID {
		t.Errorf("LDI UUID mismatch")
	}
}

func TestService_Renew_SameKeyBypassesPolicy(t *testing.T) {
	// policy.KeyRotation=false; but renewal is same-key (keyChanged=false)
	// so the service MUST issue.
	ldi := identity.LDI{UUID: "u1", SPIFFEID: "spiffe://td/margo/device/u1", Status: identity.LDIStatusActive}
	svc, log, _ := newTestService(t, &renewStubRepo{ldi: ldi}, &renewStubIssuer{}, identity.StaticPolicy{KeyRotation: false}, identity.NoReplacements{})
	currentKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	seedActiveLeaf(t, log, ldi.SPIFFEID, currentKey, time.Now().UTC())
	if _, err := svc.Renew(context.Background(), ldi.SPIFFEID, mkCSRForKey(t, currentKey)); err != nil {
		t.Fatalf("same-key renewal must succeed regardless of policy: %v", err)
	}
}

func TestService_Renew_KeyRotationDeniedByPolicy(t *testing.T) {
	ldi := identity.LDI{UUID: "u1", SPIFFEID: "spiffe://td/margo/device/u1", Status: identity.LDIStatusActive}
	svc, log, _ := newTestService(t, &renewStubRepo{ldi: ldi}, &renewStubIssuer{}, identity.StaticPolicy{KeyRotation: false}, identity.NoReplacements{})
	currentKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	seedActiveLeaf(t, log, ldi.SPIFFEID, currentKey, time.Now().UTC())
	newKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	_, err := svc.Renew(context.Background(), ldi.SPIFFEID, mkCSRForKey(t, newKey))
	if !errors.Is(err, identity.ErrKeyRotationNotPermitted) {
		t.Errorf("err = %v, want ErrKeyRotationNotPermitted", err)
	}
}

func TestService_Renew_UnknownSPIFFEID(t *testing.T) {
	svc, _, _ := newTestService(t, &renewStubRepo{err: identity.ErrNotFound}, &renewStubIssuer{}, identity.StaticPolicy{KeyRotation: true}, identity.NoReplacements{})
	_, err := svc.Renew(context.Background(), "spiffe://td/margo/device/missing", mkCSR(t))
	if !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestService_Renew_NewKey_RevokesPrior(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	repo := newMemRepo()
	nowFn := func() time.Time { return now }
	issuer := &fakeIssuerSerial{now: nowFn}
	svc, log, rq := newTestService(t, repo, issuer, identity.StaticPolicy{KeyRotation: true, NewLDI: true}, identity.NoReplacements{})
	identity.SetServiceNow(svc, nowFn)

	// Initial enrollment seeds an issuance audit row.
	vb := bootstrap.VerifiedBootstrap{Method: "urn:margo:bootstrap:factory-cert-mtls:v1", ESI: bytes.Repeat([]byte{1}, 32)}
	parsed := buildValidCSR(t)
	r1, err := svc.Enroll(ctx, vb, parsed, nil)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	// Drive a new-key renewal. keyChanged=true triggers the prior-revocation path.
	parsed2 := buildValidCSR(t)
	r2, err := svc.Renew(ctx, r1.LDI.SPIFFEID, parsed2)
	if err != nil {
		t.Fatalf("Renew: %v", err)
	}

	// After renewal with a new key, only the new serial is active.
	active, _ := log.ActiveBySPIFFEID(ctx, r1.LDI.SPIFFEID, now)
	if len(active) != 1 {
		t.Fatalf("active after new-key renewal = %+v, want 1", active)
	}
	leafNew, _ := x509.ParseCertificate(r2.CertChainDER[0])
	wantSerial := strings.ToUpper(leafNew.SerialNumber.Text(16))
	if active[0].SerialHex != wantSerial {
		t.Errorf("active serial = %q, want %q", active[0].SerialHex, wantSerial)
	}

	// The prior serial is on the revocation list.
	entries, _, _ := rq.List(ctx, now)
	if len(entries) != 1 {
		t.Fatalf("revocation list = %+v, want 1 entry", entries)
	}
	leafOld, _ := x509.ParseCertificate(r1.CertChainDER[0])
	wantPriorSerial := strings.ToUpper(leafOld.SerialNumber.Text(16))
	if entries[0].SerialHex != wantPriorSerial {
		t.Errorf("revoked serial = %q, want %q", entries[0].SerialHex, wantPriorSerial)
	}
	if entries[0].Reason != "key-rotation" {
		t.Errorf("reason = %q, want key-rotation", entries[0].Reason)
	}
}
