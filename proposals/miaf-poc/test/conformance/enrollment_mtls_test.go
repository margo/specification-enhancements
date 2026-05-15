package conformance_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertmtls"
	errpkg "github.com/margo/miaf-poc/pkg/mis/errors"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/store"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

type goldenProblem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
}

func loadGolden(t *testing.T, relPath string) goldenProblem {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "conformance", "enrollment", relPath))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	var g goldenProblem
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return g
}

type fakeIssuer struct{}

func (fakeIssuer) IssueX509SVID(ctx context.Context, spiffeID string, csrDER []byte, ttl time.Duration) ([][]byte, time.Time, error) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	u, _ := url.Parse(spiffeID)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "fake-svid"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(ttl),
		URIs:         []*url.URL{u},
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	return [][]byte{der}, tmpl.NotAfter, nil
}

type harness struct {
	handler http.Handler
	root    *x509.Certificate
	rootKey *ecdsa.PrivateKey
}

func newHarness(t *testing.T, policy identity.StaticPolicy, tickets identity.ReplacementValidator) *harness {
	t.Helper()
	root, rootKey, pool := mkFactoryPKI(t)

	registry := bootstrap.NewRegistry()
	registry.Register(common.BootstrapMethodFactoryCertMTLS, factorycertmtls.New(pool))

	s, err := store.Open(filepath.Join(t.TempDir(), "id.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	repo := s.Identities()

	svc := identity.NewService(repo, fakeIssuer{}, policy, tickets, s.IssuedSVIDs(), noopRevocationQueue{}, "margo.example.com", time.Hour)

	h := mishttp.NewEnrollmentHandler(mishttp.EnrollmentConfig{
		Registry: registry, Service: svc, Logger: slog.Default(),
	})
	return &harness{handler: h, root: root, rootKey: rootKey}
}

func mkFactoryPKI(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey, *x509.CertPool) {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(10),
		Subject:               pkix.Name{CommonName: "Conf Root"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	cert, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return cert, k, pool
}

func mkLeaf(t *testing.T, root *x509.Certificate, rootKey *ecdsa.PrivateKey) *x509.Certificate {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(20),
		Subject:      pkix.Name{CommonName: "conf-dev"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, root, &k.PublicKey, rootKey)
	leaf, _ := x509.ParseCertificate(der)
	return leaf
}

func mkValidEnrollmentBody(t *testing.T) []byte {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:            pkix.Name{CommonName: "d"},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, k)
	req := common.EnrollmentRequestDTO{
		SVIDProfileURI:      common.SVIDProfileURIX509V1,
		SVIDRequest:         common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(der)},
		BootstrapCredential: common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS},
	}
	b, _ := json.Marshal(req)
	return b
}

func (h *harness) serve(t *testing.T, body []byte, leaf *x509.Certificate) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if leaf != nil {
		req.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf, h.root}}
	}
	rr := httptest.NewRecorder()
	h.handler.ServeHTTP(rr, req)
	return rr
}

func assertGolden(t *testing.T, rr *httptest.ResponseRecorder, g goldenProblem) {
	t.Helper()
	if rr.Code != g.Status {
		t.Errorf("status = %d, want %d (body: %s)", rr.Code, g.Status, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != errpkg.ContentType {
		t.Errorf("Content-Type = %q, want %q", ct, errpkg.ContentType)
	}
	var pd errpkg.ProblemDetails
	if err := json.Unmarshal(rr.Body.Bytes(), &pd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if pd.Type != g.Type {
		t.Errorf("type = %q, want %q", pd.Type, g.Type)
	}
	if pd.Title != g.Title {
		t.Errorf("title = %q, want %q", pd.Title, g.Title)
	}
	if pd.Status != g.Status {
		t.Errorf("body.status = %d, want %d", pd.Status, g.Status)
	}
}

func TestErrorMatrix_UnsupportedMethod(t *testing.T) {
	h := newHarness(t, identity.StaticPolicy{NewLDI: true, KeyRotation: true}, identity.NoReplacements{})
	leaf := mkLeaf(t, h.root, h.rootKey)

	req := common.EnrollmentRequestDTO{
		SVIDProfileURI:      common.SVIDProfileURIX509V1,
		SVIDRequest:         common.SVIDRequestX509DTO{CSR: "AAAA"},
		BootstrapCredential: common.BootstrapCredentialDTO{Method: "urn:margo:bootstrap:nope:v1"},
	}
	body, _ := json.Marshal(req)
	rr := h.serve(t, body, leaf)
	assertGolden(t, rr, loadGolden(t, "errors/unsupported-method.golden.json"))
}

func TestErrorMatrix_UnsupportedSVIDProfile(t *testing.T) {
	h := newHarness(t, identity.StaticPolicy{NewLDI: true, KeyRotation: true}, identity.NoReplacements{})
	leaf := mkLeaf(t, h.root, h.rootKey)

	req := common.EnrollmentRequestDTO{
		SVIDProfileURI:      common.SVIDProfileURIJWTV1,
		SVIDRequest:         common.SVIDRequestX509DTO{CSR: "AAAA"},
		BootstrapCredential: common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS},
	}
	body, _ := json.Marshal(req)
	rr := h.serve(t, body, leaf)
	assertGolden(t, rr, loadGolden(t, "errors/unsupported-svid-profile.golden.json"))
}

func TestErrorMatrix_MalformedCSR(t *testing.T) {
	h := newHarness(t, identity.StaticPolicy{NewLDI: true, KeyRotation: true}, identity.NoReplacements{})
	leaf := mkLeaf(t, h.root, h.rootKey)

	req := common.EnrollmentRequestDTO{
		SVIDProfileURI:      common.SVIDProfileURIX509V1,
		SVIDRequest:         common.SVIDRequestX509DTO{CSR: "!!!"},
		BootstrapCredential: common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS},
	}
	body, _ := json.Marshal(req)
	rr := h.serve(t, body, leaf)
	assertGolden(t, rr, loadGolden(t, "errors/malformed-csr.golden.json"))
}

func TestErrorMatrix_SpuriousReplacementAuthorization(t *testing.T) {
	h := newHarness(t, identity.StaticPolicy{NewLDI: true, KeyRotation: true}, identity.NoReplacements{})
	leaf := mkLeaf(t, h.root, h.rootKey)

	if rr := h.serve(t, mkValidEnrollmentBody(t), leaf); rr.Code != http.StatusCreated {
		t.Fatalf("bootstrap enroll status = %d (body: %s)", rr.Code, rr.Body.String())
	}

	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "d"}, SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, k)
	req := common.EnrollmentRequestDTO{
		SVIDProfileURI:      common.SVIDProfileURIX509V1,
		SVIDRequest:         common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(der)},
		BootstrapCredential: common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS},
		ReplacementAuthorization: &common.ReplacementAuthorizationDTO{
			Method: common.ReplacementAuthMethodOperatorTicket,
			Proof:  common.ReplacementAuthorizationProof{Ticket: "unused-ticket"},
		},
	}
	body, _ := json.Marshal(req)
	rr := h.serve(t, body, leaf)
	assertGolden(t, rr, loadGolden(t, "errors/spurious-replacement-authorization.golden.json"))
}

func TestErrorMatrix_UnsupportedReplacementAuthMethod(t *testing.T) {
	h := newHarness(t, identity.StaticPolicy{NewLDI: true, KeyRotation: true}, identity.NoReplacements{})
	leaf := mkLeaf(t, h.root, h.rootKey)

	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "d"}, SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, k)
	req := common.EnrollmentRequestDTO{
		SVIDProfileURI:           common.SVIDProfileURIX509V1,
		SVIDRequest:              common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(der)},
		BootstrapCredential:      common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS},
		ReplacementAuthorization: &common.ReplacementAuthorizationDTO{Method: "urn:margo:replacement-auth:mystery:v1"},
	}
	body, _ := json.Marshal(req)
	rr := h.serve(t, body, leaf)
	assertGolden(t, rr, loadGolden(t, "errors/unsupported-replacement-authorization-method.golden.json"))
}

func TestErrorMatrix_ReplacementNotAuthorized(t *testing.T) {
	h := newHarness(t, identity.StaticPolicy{NewLDI: true, KeyRotation: true}, identity.NoReplacements{})
	leaf := mkLeaf(t, h.root, h.rootKey)

	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "d"}, SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, k)
	req := common.EnrollmentRequestDTO{
		SVIDProfileURI:      common.SVIDProfileURIX509V1,
		SVIDRequest:         common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(der)},
		BootstrapCredential: common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS},
		ReplacementAuthorization: &common.ReplacementAuthorizationDTO{
			Method: common.ReplacementAuthMethodOperatorTicket,
			Proof:  common.ReplacementAuthorizationProof{Ticket: "not-a-real-ticket"},
		},
	}
	body, _ := json.Marshal(req)
	rr := h.serve(t, body, leaf)
	assertGolden(t, rr, loadGolden(t, "errors/replacement-not-authorized.golden.json"))
}

func TestErrorMatrix_KeyRotationNotPermitted(t *testing.T) {
	h := newHarness(t, identity.StaticPolicy{NewLDI: true, KeyRotation: false}, identity.NoReplacements{})
	leaf := mkLeaf(t, h.root, h.rootKey)

	if rr := h.serve(t, mkValidEnrollmentBody(t), leaf); rr.Code != http.StatusCreated {
		t.Fatalf("bootstrap enroll status = %d (body: %s)", rr.Code, rr.Body.String())
	}
	rr := h.serve(t, mkValidEnrollmentBody(t), leaf)
	assertGolden(t, rr, loadGolden(t, "errors/key-rotation-not-permitted.golden.json"))
}

func TestSuccessShape_201_InitialEnrollment(t *testing.T) {
	h := newHarness(t, identity.StaticPolicy{NewLDI: true, KeyRotation: true}, identity.NoReplacements{})
	leaf := mkLeaf(t, h.root, h.rootKey)

	rr := h.serve(t, mkValidEnrollmentBody(t), leaf)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
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

func TestSuccessShape_200_ReEnrollment(t *testing.T) {
	h := newHarness(t, identity.StaticPolicy{NewLDI: true, KeyRotation: true}, identity.NoReplacements{})
	leaf := mkLeaf(t, h.root, h.rootKey)

	if rr := h.serve(t, mkValidEnrollmentBody(t), leaf); rr.Code != http.StatusCreated {
		t.Fatalf("first status = %d", rr.Code)
	}
	rr := h.serve(t, mkValidEnrollmentBody(t), leaf)
	if rr.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
}
