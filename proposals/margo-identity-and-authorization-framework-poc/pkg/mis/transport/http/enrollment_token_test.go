package http_test

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
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/enrollmenttoken"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/enrollmenttoken/tokens"
	errpkg "github.com/margo/miaf-poc/pkg/mis/errors"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/store"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

type stubIssuer struct{}

func (stubIssuer) IssueX509SVID(ctx context.Context, spiffeID string, csrDER []byte, ttl time.Duration) ([][]byte, time.Time, error) {
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	u, _ := url.Parse(spiffeID)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(ttl),
		URIs:         []*url.URL{u},
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	return [][]byte{der}, tmpl.NotAfter, nil
}

func newTokenHandler(t *testing.T) (http.Handler, *enrollmenttoken.Validator, tokens.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ts := st.EnrollmentTokens()

	v := enrollmenttoken.New(enrollmenttoken.Config{Store: ts})
	reg := bootstrap.NewRegistry()
	reg.Register(common.BootstrapMethodEnrollmentToken, v)

	repo := st.Identities()

	svc := identity.NewService(repo, stubIssuer{},
		identity.StaticPolicy{NewLDI: true, KeyRotation: true},
		identity.NoReplacements{},
		noopIssuanceLog{}, noopRevocationQueue{},
		"margo.example.com", time.Hour)
	h := mishttp.NewEnrollmentHandler(mishttp.EnrollmentConfig{
		Registry: reg, Service: svc, Logger: slog.Default(),
	})
	return h, v, ts
}

func mkCSRBody(t *testing.T, token string) []byte {
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

func TestHandler_EnrollmentToken_HappyPath_201(t *testing.T) {
	h, _, ts := newTokenHandler(t)
	c, _ := ts.Create(context.Background(), 5*time.Minute, time.Now())
	r := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(mkCSRBody(t, c.Secret.Wire())))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandler_EnrollmentToken_SecondSubmission_SameKey_ReplaysOriginal(t *testing.T) {
	h, _, ts := newTokenHandler(t)
	c, _ := ts.Create(context.Background(), 5*time.Minute, time.Now())

	// Mint a single CSR + body; submit the SAME bytes twice.
	body := mkCSRBody(t, c.Secret.Wire())
	r1 := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body))
	r1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	h.ServeHTTP(rr1, r1)
	if rr1.Code != http.StatusCreated {
		t.Fatalf("first status = %d; body: %s", rr1.Code, rr1.Body.String())
	}
	orig := rr1.Body.Bytes()

	r2 := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body))
	r2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, r2)
	if rr2.Code != http.StatusCreated {
		t.Fatalf("replay status = %d, want 201; body: %s", rr2.Code, rr2.Body.String())
	}
	if !bytes.Equal(orig, rr2.Body.Bytes()) {
		t.Errorf("replay body differs from original\norig: %s\nrepl: %s", orig, rr2.Body.String())
	}
}

func TestHandler_EnrollmentToken_SecondSubmission_DifferentKey_401(t *testing.T) {
	h, _, ts := newTokenHandler(t)
	c, _ := ts.Create(context.Background(), 5*time.Minute, time.Now())

	r1 := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(mkCSRBody(t, c.Secret.Wire())))
	r1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	h.ServeHTTP(rr1, r1)
	if rr1.Code != http.StatusCreated {
		t.Fatalf("first status = %d", rr1.Code)
	}
	// Fresh CSR => different public key.
	r2 := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(mkCSRBody(t, c.Secret.Wire())))
	r2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, r2)
	if rr2.Code != http.StatusUnauthorized {
		t.Fatalf("retry-with-new-key status = %d, want 401; body: %s", rr2.Code, rr2.Body.String())
	}
	var pd errpkg.ProblemDetails
	_ = json.Unmarshal(rr2.Body.Bytes(), &pd)
	if pd.Type != errpkg.InvalidEnrollmentToken.TypeURI {
		t.Errorf("type = %q, want %q", pd.Type, errpkg.InvalidEnrollmentToken.TypeURI)
	}
}

func TestHandler_EnrollmentToken_UnknownToken_401(t *testing.T) {
	h, _, _ := newTokenHandler(t)
	body := mkCSRBody(t, "margo-et-v1.AAAAAAAAAAAAAAAAAAAAAA") // valid wire, unknown secret
	r := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	var pd errpkg.ProblemDetails
	_ = json.Unmarshal(rr.Body.Bytes(), &pd)
	if pd.Type != errpkg.InvalidEnrollmentToken.TypeURI {
		t.Errorf("type = %q, want %q", pd.Type, errpkg.InvalidEnrollmentToken.TypeURI)
	}
}
