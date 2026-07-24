package enrollmenttoken_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/enrollmenttoken"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/enrollmenttoken/tokens"
	"github.com/margo/miaf-poc/pkg/mis/store"
)

func mkStore(t *testing.T) tokens.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s.EnrollmentTokens()
}

func mkValidator(t *testing.T, s tokens.Store, clock func() time.Time) *enrollmenttoken.Validator {
	t.Helper()
	return enrollmenttoken.New(enrollmenttoken.Config{
		Store:       s,
		RetryWindow: 5 * time.Minute,
		Clock:       clock,
	})
}

func mkCSR(t *testing.T, k *ecdsa.PrivateKey) string {
	t.Helper()
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, k)
	if err != nil {
		t.Fatalf("CreateCertificateRequest: %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

// === Validate happy path ===

func TestValidate_HappyPath_DerivesESIFromTokenID(t *testing.T) {
	s := mkStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	clock := func() time.Time { return now }
	v := mkValidator(t, s, clock)

	c, err := s.Create(context.Background(), 5*time.Minute, now)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identities", nil)
	outcome, err := v.Validate(context.Background(), req, common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodEnrollmentToken,
		Proof:  &common.BootstrapProof{Token: c.Secret.Wire()},
	}, "", "")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if outcome.Pending == nil {
		t.Fatal("outcome.Pending is nil, want PendingEnrollment")
	}
	vb := outcome.Pending.VB
	want := sha256.Sum256([]byte(c.TokenID))
	if !bytes.Equal(vb.ESI, want[:]) {
		t.Errorf("ESI mismatch\n got: %x\nwant: %x", vb.ESI, want[:])
	}
	if vb.Method != common.BootstrapMethodEnrollmentToken {
		t.Errorf("Method = %q", vb.Method)
	}
	ev, ok := vb.Evidence.(*enrollmenttoken.Evidence)
	if !ok || ev == nil {
		t.Fatalf("Evidence = %T, want *enrollmenttoken.Evidence", vb.Evidence)
	}
	if ev.TokenID != c.TokenID {
		t.Errorf("Evidence.TokenID = %q, want %q", ev.TokenID, c.TokenID)
	}
	if !bytes.Equal(ev.SecretHash, c.Secret.Hash()) {
		t.Errorf("Evidence.SecretHash mismatch")
	}
}

func TestValidate_MissingProof_ReturnsErrMissingToken(t *testing.T) {
	v := mkValidator(t, mkStore(t), time.Now)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	_, err := v.Validate(context.Background(), req, common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodEnrollmentToken,
	}, "", "")
	if !errors.Is(err, enrollmenttoken.ErrMissingToken) {
		t.Errorf("err = %v, want ErrMissingToken", err)
	}
}

func TestValidate_MalformedToken_ReturnsErrInvalidToken(t *testing.T) {
	v := mkValidator(t, mkStore(t), time.Now)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	_, err := v.Validate(context.Background(), req, common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodEnrollmentToken,
		Proof:  &common.BootstrapProof{Token: "not-a-margo-token"},
	}, "", "")
	if !errors.Is(err, enrollmenttoken.ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

func TestValidate_SecondCall_SameToken_ReturnsErrInvalidToken(t *testing.T) {
	s := mkStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	v := mkValidator(t, s, func() time.Time { return now })
	c, _ := s.Create(context.Background(), 5*time.Minute, now)
	cred := common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodEnrollmentToken,
		Proof:  &common.BootstrapProof{Token: c.Secret.Wire()},
	}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	if _, err := v.Validate(context.Background(), req, cred, "", ""); err != nil {
		t.Fatalf("first Validate: %v", err)
	}
	_, err := v.Validate(context.Background(), req, cred, "", "")
	if !errors.Is(err, enrollmenttoken.ErrInvalidToken) {
		t.Errorf("second Validate err = %v, want ErrInvalidToken", err)
	}
}

func TestValidate_UnknownToken_ReturnsErrInvalidToken(t *testing.T) {
	v := mkValidator(t, mkStore(t), time.Now)
	fresh, _ := tokens.NewSecret(rand.Reader)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	_, err := v.Validate(context.Background(), req, common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodEnrollmentToken,
		Proof:  &common.BootstrapProof{Token: fresh.Wire()},
	}, "", "")
	if !errors.Is(err, enrollmenttoken.ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

func TestValidate_ExpiredToken_ReturnsErrInvalidToken(t *testing.T) {
	s := mkStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	var clk time.Time = now
	v := mkValidator(t, s, func() time.Time { return clk })
	c, _ := s.Create(context.Background(), 10*time.Second, now)

	clk = now.Add(time.Minute) // past expiry

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	_, err := v.Validate(context.Background(), req, common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodEnrollmentToken,
		Proof:  &common.BootstrapProof{Token: c.Secret.Wire()},
	}, "", "")
	if !errors.Is(err, enrollmenttoken.ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

// === Commit (via PendingEnrollment.Commit) ===

func TestCommit_StoresOutcome(t *testing.T) {
	s := mkStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	v := mkValidator(t, s, func() time.Time { return now })
	c, _ := s.Create(context.Background(), 5*time.Minute, now)

	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrB64 := mkCSR(t, k)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	outcome, err := v.Validate(context.Background(), req, common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodEnrollmentToken,
		Proof:  &common.BootstrapProof{Token: c.Secret.Wire()},
	}, csrB64, common.SVIDProfileURIX509V1)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if outcome.Pending == nil {
		t.Fatal("outcome.Pending is nil")
	}

	resp := &common.EnrollmentResponseDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVID:           common.X509SVIDDTO{CertificateChainPEM: []string{"-----BEGIN CERTIFICATE-----\nAA==\n-----END CERTIFICATE-----\n"}},
	}
	if err := outcome.Pending.Commit(context.Background(), resp, 201, "LDI-UUID-1"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	_, out, err := s.FindConsumed(context.Background(), c.Secret.Hash(), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("FindConsumed: %v", err)
	}
	if out.StatusCode != 201 {
		t.Errorf("StatusCode = %d, want 201", out.StatusCode)
	}
	if out.LDIUUID != "LDI-UUID-1" {
		t.Errorf("LDIUUID = %q", out.LDIUUID)
	}
	if len(out.CertificateChainPEM) != 1 {
		t.Errorf("chain len = %d", len(out.CertificateChainPEM))
	}
	if len(out.CSRPubKeySHA256) != 32 {
		t.Errorf("CSRPubKeySHA256 len = %d, want 32", len(out.CSRPubKeySHA256))
	}
}

// === Replay detection (via second Validate call) ===

func TestCheckReplay_MatchWithinWindow_ReturnsReplay(t *testing.T) {
	s := mkStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	var clk time.Time = now
	v := mkValidator(t, s, func() time.Time { return clk })

	c, _ := s.Create(context.Background(), 5*time.Minute, now)

	cred := common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodEnrollmentToken,
		Proof:  &common.BootstrapProof{Token: c.Secret.Wire()},
	}
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrB64 := mkCSR(t, k)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	outcome, err := v.Validate(context.Background(), req, cred, csrB64, common.SVIDProfileURIX509V1)
	if err != nil {
		t.Fatalf("first Validate: %v", err)
	}
	resp := &common.EnrollmentResponseDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVID:           common.X509SVIDDTO{CertificateChainPEM: []string{"LEAF", "INT"}},
	}
	if err := outcome.Pending.Commit(context.Background(), resp, 201, "LDI-1"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	clk = now.Add(2 * time.Minute)

	outcome2, err := v.Validate(context.Background(), req, cred, csrB64, common.SVIDProfileURIX509V1)
	if err != nil {
		t.Fatalf("second Validate: %v", err)
	}
	if outcome2.Replay == nil {
		t.Fatal("outcome2.Replay is nil, want CachedResponse")
	}
	if outcome2.Replay.StatusCode != 201 {
		t.Errorf("StatusCode = %d", outcome2.Replay.StatusCode)
	}
	if len(outcome2.Replay.Response.SVID.CertificateChainPEM) != 2 || outcome2.Replay.Response.SVID.CertificateChainPEM[0] != "LEAF" {
		t.Errorf("chain = %v", outcome2.Replay.Response.SVID.CertificateChainPEM)
	}
}

func TestCheckReplay_AfterWindow_ReturnsErrInvalidToken(t *testing.T) {
	s := mkStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	var clk time.Time = now
	v := mkValidator(t, s, func() time.Time { return clk })

	c, _ := s.Create(context.Background(), 30*time.Minute, now)
	cred := common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodEnrollmentToken,
		Proof:  &common.BootstrapProof{Token: c.Secret.Wire()},
	}
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrB64 := mkCSR(t, k)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	outcome, _ := v.Validate(context.Background(), req, cred, csrB64, common.SVIDProfileURIX509V1)
	if err := outcome.Pending.Commit(context.Background(), &common.EnrollmentResponseDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
	}, 201, "LDI-2"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	clk = now.Add(10 * time.Minute)

	_, err := v.Validate(context.Background(), req, cred, csrB64, common.SVIDProfileURIX509V1)
	if !errors.Is(err, enrollmenttoken.ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

func TestCheckReplay_DifferentCSRPubKey_ReturnsErrInvalidToken(t *testing.T) {
	s := mkStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	v := mkValidator(t, s, func() time.Time { return now })

	c, _ := s.Create(context.Background(), 5*time.Minute, now)
	cred := common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodEnrollmentToken,
		Proof:  &common.BootstrapProof{Token: c.Secret.Wire()},
	}
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	kA, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrA := mkCSR(t, kA)
	outcome, _ := v.Validate(context.Background(), req, cred, csrA, common.SVIDProfileURIX509V1)
	if err := outcome.Pending.Commit(context.Background(), &common.EnrollmentResponseDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
	}, 201, "LDI-3"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	kB, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrB := mkCSR(t, kB)
	_, err := v.Validate(context.Background(), req, cred, csrB, common.SVIDProfileURIX509V1)
	if !errors.Is(err, enrollmenttoken.ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

func TestCheckReplay_DifferentSVIDProfile_ReturnsErrInvalidToken(t *testing.T) {
	s := mkStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	v := mkValidator(t, s, func() time.Time { return now })

	c, _ := s.Create(context.Background(), 5*time.Minute, now)
	cred := common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodEnrollmentToken,
		Proof:  &common.BootstrapProof{Token: c.Secret.Wire()},
	}
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrB64 := mkCSR(t, k)
	outcome, _ := v.Validate(context.Background(), req, cred, csrB64, common.SVIDProfileURIX509V1)
	if err := outcome.Pending.Commit(context.Background(), &common.EnrollmentResponseDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
	}, 201, "LDI-4"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	_, err := v.Validate(context.Background(), req, cred, csrB64, "https://margo.org/profiles/spiffe/jwt-svid/v1")
	if !errors.Is(err, enrollmenttoken.ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}
