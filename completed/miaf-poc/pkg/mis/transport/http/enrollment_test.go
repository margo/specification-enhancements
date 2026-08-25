package http_test

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
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertjwt"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertjwt/replay"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertmtls"
	"github.com/margo/miaf-poc/pkg/mis/csr"
	errpkg "github.com/margo/miaf-poc/pkg/mis/errors"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

type fakeIssuer struct{}

func (fakeIssuer) IssueX509SVID(_ context.Context, spiffeID string, _ []byte, ttl time.Duration) ([][]byte, time.Time, error) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	notAfter := time.Now().Add(ttl)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: spiffeID},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     notAfter,
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	return [][]byte{der}, notAfter, nil
}

func makeFactoryPKI(t *testing.T) (root *x509.Certificate, rootKey *ecdsa.PrivateKey, pool *x509.CertPool) {
	t.Helper()
	rootKey, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Handler Test Root"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &rootKey.PublicKey, rootKey)
	root, _ = x509.ParseCertificate(der)

	pool = x509.NewCertPool()
	pool.AddCert(root)
	return
}

func issueFactoryLeaf(t *testing.T, root *x509.Certificate, rootKey *ecdsa.PrivateKey) *x509.Certificate {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "factory-device"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, root, &key.PublicKey, rootKey)
	leaf, _ := x509.ParseCertificate(der)
	return leaf
}

func buildEnrollmentBody(t *testing.T) []byte {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:            pkix.Name{CommonName: "d"},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, key)
	req := common.EnrollmentRequestDTO{
		SVIDProfileURI:      common.SVIDProfileURIX509V1,
		SVIDRequest:         common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(der)},
		BootstrapCredential: common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS},
	}
	body, _ := json.Marshal(req)
	return body
}

// memRepoForHandler is a minimal Repository stub for the handler tests.
type memRepoForHandler struct {
	created  map[string]identity.LDI
	bindings map[string]identity.ESIBinding
}

func newMemRepoForHandler() *memRepoForHandler {
	return &memRepoForHandler{
		created:  map[string]identity.LDI{},
		bindings: map[string]identity.ESIBinding{},
	}
}

func (r *memRepoForHandler) CreateLDIAndBind(ctx context.Context, esi []byte, method, trustDomain string) (identity.LDI, error) {
	l := identity.LDI{UUID: "00000000-0000-4000-8000-000000000001", SPIFFEID: "spiffe://" + trustDomain + "/margo/device/00000000-0000-4000-8000-000000000001", Status: identity.LDIStatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	r.created[l.UUID] = l
	r.bindings[string(esi)] = identity.ESIBinding{
		ESI:     append([]byte(nil), esi...),
		LDIUUID: l.UUID,
		Method:  method,
		Status:  identity.ESIBindingActive,
		BoundAt: time.Now(),
	}
	return l, nil
}
func (r *memRepoForHandler) Rebind(context.Context, string, []byte, string) (identity.ESIBinding, error) {
	return identity.ESIBinding{}, identity.ErrNotFound
}
func (r *memRepoForHandler) GetLDI(ctx context.Context, u string) (identity.LDI, error) {
	return r.created[u], nil
}
func (r *memRepoForHandler) ListBindings(context.Context, string) ([]identity.ESIBinding, error) {
	return nil, nil
}

func (r *memRepoForHandler) GetLDIBySPIFFEID(ctx context.Context, spiffeID string) (identity.LDI, error) {
	for _, l := range r.created {
		if l.SPIFFEID == spiffeID {
			return l, nil
		}
	}
	return identity.LDI{}, identity.ErrNotFound
}

func (r *memRepoForHandler) LookupByESI(ctx context.Context, esi []byte) (identity.ESIBinding, identity.LDI, error) {
	binding, ok := r.bindings[string(esi)]
	if !ok {
		return identity.ESIBinding{}, identity.LDI{}, identity.ErrNotFound
	}
	ldi, ok := r.created[binding.LDIUUID]
	if !ok {
		return identity.ESIBinding{}, identity.LDI{}, identity.ErrNotFound
	}
	return binding, ldi, nil
}

func (r *memRepoForHandler) SetLDIStatus(_ context.Context, _ string, _ identity.LDIStatus) error {
	return nil
}

func newHarness(t *testing.T) (http.Handler, *x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	root, rootKey, pool := makeFactoryPKI(t)

	registry := bootstrap.NewRegistry()
	registry.Register(common.BootstrapMethodFactoryCertMTLS, factorycertmtls.New(pool))

	repo := newMemRepoForHandler()
	issuer := fakeIssuer{}
	svc := identity.NewService(repo, issuer, identity.DefaultPolicy(), identity.NoReplacements{}, noopIssuanceLog{}, noopRevocationQueue{}, "margo.example.com", 90*24*time.Hour)

	h := mishttp.NewEnrollmentHandler(mishttp.EnrollmentConfig{
		Registry: registry,
		Service:  svc,
		Logger:   slog.Default(),
	})
	return h, root, rootKey
}

func TestEnroll_HappyPath_201AndCertChainPEM(t *testing.T) {
	h, root, rootKey := newHarness(t)
	leaf := issueFactoryLeaf(t, root, rootKey)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(buildEnrollmentBody(t)))
	req.Header.Set("Content-Type", "application/json")
	req.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf, root}}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var resp common.EnrollmentResponseDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.SVIDProfileURI != common.SVIDProfileURIX509V1 {
		t.Errorf("svid_profile_uri = %q", resp.SVIDProfileURI)
	}
	if len(resp.SVID.CertificateChainPEM) == 0 {
		t.Fatal("certificate_chain_pem empty")
	}
	block, _ := pem.Decode([]byte(resp.SVID.CertificateChainPEM[0]))
	if block == nil || block.Type != "CERTIFICATE" {
		t.Errorf("leaf PEM malformed: %q", resp.SVID.CertificateChainPEM[0])
	}
}

func TestEnroll_WrongMethod_Returns422_UnsupportedMethod(t *testing.T) {
	h, root, rootKey := newHarness(t)
	leaf := issueFactoryLeaf(t, root, rootKey)

	req := common.EnrollmentRequestDTO{
		SVIDProfileURI:      common.SVIDProfileURIX509V1,
		SVIDRequest:         common.SVIDRequestX509DTO{CSR: "AAAA"},
		BootstrapCredential: common.BootstrapCredentialDTO{Method: "urn:margo:bootstrap:does-not-exist:v1"},
	}
	body, _ := json.Marshal(req)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf, root}}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	assertProblem(t, rr, 422, "https://margo.org/docs/errors/unsupported-method")
}

func TestEnroll_WrongProfile_Returns422_UnsupportedSVIDProfile(t *testing.T) {
	h, root, rootKey := newHarness(t)
	leaf := issueFactoryLeaf(t, root, rootKey)

	req := common.EnrollmentRequestDTO{
		SVIDProfileURI:      common.SVIDProfileURIJWTV1,
		SVIDRequest:         common.SVIDRequestX509DTO{CSR: "AAAA"},
		BootstrapCredential: common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS},
	}
	body, _ := json.Marshal(req)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf, root}}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	assertProblem(t, rr, 422, "https://margo.org/docs/errors/unsupported-svid-profile")
}

func TestEnroll_MalformedCSR_Returns400(t *testing.T) {
	h, root, rootKey := newHarness(t)
	leaf := issueFactoryLeaf(t, root, rootKey)

	req := common.EnrollmentRequestDTO{
		SVIDProfileURI:      common.SVIDProfileURIX509V1,
		SVIDRequest:         common.SVIDRequestX509DTO{CSR: "!!!!"},
		BootstrapCredential: common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS},
	}
	body, _ := json.Marshal(req)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf, root}}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	assertProblem(t, rr, 400, "https://margo.org/docs/errors/invalid-svid-request")
}

func TestEnroll_NoMTLS_Returns401(t *testing.T) {
	h, _, _ := newHarness(t)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(buildEnrollmentBody(t)))
	r.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	assertProblem(t, rr, 401, "https://margo.org/docs/errors/invalid-bootstrap-proof")
}

func TestEnrollmentHandler_JWTMalformed_Returns401InvalidBootstrapProof(t *testing.T) {
	// Build registry with factorycertjwt registered. Empty roots pool means
	// every JWT will fail (ErrUntrustedChain or ErrMalformedAssertion) - which
	// is exactly what we want to exercise the error-mapping path.
	registry := bootstrap.NewRegistry()
	registry.Register(common.BootstrapMethodFactoryCertJWT, factorycertjwt.New(factorycertjwt.Config{
		Roots:    x509.NewCertPool(),
		Audience: "https://mis.example.com/api/v1/identities",
		Replay:   replay.NewMemoryStore(replay.Config{}),
	}))

	repo := newMemRepoForHandler()
	issuer := fakeIssuer{}
	svc := identity.NewService(repo, issuer, identity.DefaultPolicy(), identity.NoReplacements{}, noopIssuanceLog{}, noopRevocationQueue{}, "margo.example.com", 90*24*time.Hour)

	h := mishttp.NewEnrollmentHandler(mishttp.EnrollmentConfig{
		Registry: registry,
		Service:  svc,
		Logger:   slog.Default(),
	})

	// Build an enrollment request using the JWT bootstrap method with a
	// deliberately malformed assertion ("not-a-jwt").
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:            pkix.Name{CommonName: "d"},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, key)
	reqBody := common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(csrDER)},
		BootstrapCredential: common.BootstrapCredentialDTO{
			Method: common.BootstrapMethodFactoryCertJWT,
			Proof:  &common.BootstrapProof{Assertion: "not-a-jwt"},
		},
	}
	body, _ := json.Marshal(reqBody)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)

	assertProblem(t, rr, http.StatusUnauthorized, "https://margo.org/docs/errors/invalid-bootstrap-proof")
}

func TestEnroll_SpuriousReplacementAuthorization_Returns400SpecificType(t *testing.T) {
	h, root, rootKey := newHarness(t)
	leaf := issueFactoryLeaf(t, root, rootKey)

	first := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(buildEnrollmentBody(t)))
	first.Header.Set("Content-Type", "application/json")
	first.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf, root}}
	firstRR := httptest.NewRecorder()
	h.ServeHTTP(firstRR, first)
	if firstRR.Code != http.StatusCreated {
		t.Fatalf("first enrollment status = %d, want 201 (body: %s)", firstRR.Code, firstRR.Body.String())
	}

	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:            pkix.Name{CommonName: "d"},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, key)
	reqBody := common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(csrDER)},
		BootstrapCredential: common.BootstrapCredentialDTO{
			Method: common.BootstrapMethodFactoryCertMTLS,
		},
		ReplacementAuthorization: &common.ReplacementAuthorizationDTO{
			Method: common.ReplacementAuthMethodOperatorTicket,
			Proof:  common.ReplacementAuthorizationProof{Ticket: "unused-ticket"},
		},
	}
	body, _ := json.Marshal(reqBody)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf, root}}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	assertProblem(t, rr, http.StatusBadRequest, "https://margo.org/docs/errors/spurious-replacement-authorization")
}

func assertProblem(t *testing.T, rr *httptest.ResponseRecorder, status int, typeURI string) {
	t.Helper()
	if rr.Code != status {
		t.Errorf("status = %d, want %d (body: %s)", rr.Code, status, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != errpkg.ContentType {
		t.Errorf("Content-Type = %q, want %q", ct, errpkg.ContentType)
	}
	var pd errpkg.ProblemDetails
	if err := json.Unmarshal(rr.Body.Bytes(), &pd); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, rr.Body.String())
	}
	if pd.Type != typeURI {
		t.Errorf("type = %q, want %q", pd.Type, typeURI)
	}
	if pd.Status != status {
		t.Errorf("body status = %d, want %d", pd.Status, status)
	}
}

var (
	_ = csr.ErrMalformed
	_ = io.Discard
)
