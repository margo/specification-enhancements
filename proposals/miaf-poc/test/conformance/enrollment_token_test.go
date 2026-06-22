package conformance_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/enrollmenttoken"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/enrollmenttoken/tokens"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/store"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

type tokenHarness struct {
	handler http.Handler
	store   tokens.Store
}

func newTokenHarness(t *testing.T) *tokenHarness {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	ts := s.EnrollmentTokens()
	repo := s.Identities()

	reg := bootstrap.NewRegistry()
	reg.Register(common.BootstrapMethodEnrollmentToken, enrollmenttoken.New(enrollmenttoken.Config{Store: ts}))

	svc := identity.NewService(repo, fakeIssuer{},
		identity.StaticPolicy{NewLDI: true, KeyRotation: true},
		identity.NoReplacements{},
		noopIssuanceLog{}, noopRevocationQueue{},
		"margo.example.com", time.Hour)

	h := mishttp.NewEnrollmentHandler(mishttp.EnrollmentConfig{
		Registry: reg, Service: svc, Logger: slog.Default(),
	})
	return &tokenHarness{handler: h, store: ts}
}

func mkTokenEnrollmentBody(t *testing.T, token string) []byte {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, k)
	b, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(der)},
		BootstrapCredential: common.BootstrapCredentialDTO{
			Method: common.BootstrapMethodEnrollmentToken,
			Proof:  &common.BootstrapProof{Token: token},
		},
	})
	return b
}

func TestSuccessShape_EnrollmentToken_201_InitialEnrollment(t *testing.T) {
	h := newTokenHarness(t)
	c, _ := h.store.Create(context.Background(), 5*time.Minute, time.Now())
	body := mkTokenEnrollmentBody(t, c.Secret.Wire())

	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	h.handler.ServeHTTP(rr, r)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d; body: %s", rr.Code, rr.Body.String())
	}
	var resp common.EnrollmentResponseDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.SVIDProfileURI != common.SVIDProfileURIX509V1 {
		t.Errorf("profile = %q", resp.SVIDProfileURI)
	}
	if len(resp.SVID.CertificateChainPEM) == 0 {
		t.Error("chain empty")
	}
}

func TestSuccessShape_EnrollmentToken_201_IdempotentReplay(t *testing.T) {
	h := newTokenHarness(t)
	c, _ := h.store.Create(context.Background(), 5*time.Minute, time.Now())
	body := mkTokenEnrollmentBody(t, c.Secret.Wire())

	// First POST - mints.
	r1 := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body))
	r1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	h.handler.ServeHTTP(rr1, r1)
	if rr1.Code != http.StatusCreated {
		t.Fatalf("first status = %d (body: %s)", rr1.Code, rr1.Body.String())
	}
	// Second POST - replays (same bytes).
	r2 := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body))
	r2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	h.handler.ServeHTTP(rr2, r2)
	// SUP says "return the same successful enrollment outcome" - for a 201 origin we accept 201 on replay.
	if rr2.Code != http.StatusCreated {
		t.Fatalf("replay status = %d, want 201 (body: %s)", rr2.Code, rr2.Body.String())
	}
	if !bytes.Equal(rr1.Body.Bytes(), rr2.Body.Bytes()) {
		t.Errorf("replay body differs from original")
	}
}

func TestErrorMatrix_EnrollmentToken_UnknownToken(t *testing.T) {
	h := newTokenHarness(t)
	body := mkTokenEnrollmentBody(t, "margo-et-v1.AAAAAAAAAAAAAAAAAAAAAA")
	r := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.handler.ServeHTTP(rr, r)
	assertGolden(t, rr, loadGolden(t, "errors/invalid-enrollment-token.golden.json"))
}

func TestErrorMatrix_EnrollmentToken_ConsumedWithoutReplay(t *testing.T) {
	h := newTokenHarness(t)
	c, _ := h.store.Create(context.Background(), 5*time.Minute, time.Now())
	body1 := mkTokenEnrollmentBody(t, c.Secret.Wire())
	body2 := mkTokenEnrollmentBody(t, c.Secret.Wire()) // fresh CSR ==> different pub key

	r1 := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body1))
	r1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	h.handler.ServeHTTP(rr1, r1)
	if rr1.Code != http.StatusCreated {
		t.Fatalf("first status = %d (body: %s)", rr1.Code, rr1.Body.String())
	}
	r2 := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body2))
	r2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	h.handler.ServeHTTP(rr2, r2)
	assertGolden(t, rr2, loadGolden(t, "errors/invalid-enrollment-token.golden.json"))
}

func TestErrorMatrix_EnrollmentToken_MissingProof(t *testing.T) {
	h := newTokenHarness(t)
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, k)
	body, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(der)},
		BootstrapCredential: common.BootstrapCredentialDTO{
			Method: common.BootstrapMethodEnrollmentToken,
		},
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.handler.ServeHTTP(rr, r)
	assertGolden(t, rr, loadGolden(t, "errors/invalid-enrollment-token.golden.json"))
}
