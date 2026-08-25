package conformance_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertjwt"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/store"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
	"github.com/margo/miaf-poc/test/testsupport/bootstrapjwt"
)

const jwtAudience = "https://mis.example.com/api/v1/identities"

type jwtHarness struct {
	handler http.Handler
	root    *x509.Certificate
	rootKey *ecdsa.PrivateKey
	leaf    *x509.Certificate
	leafKey *ecdsa.PrivateKey
}

func newJWTHarness(t *testing.T, policy identity.StaticPolicy) *jwtHarness {
	t.Helper()
	root, rootKey, pool := mkFactoryPKI(t)
	leaf, leafKey := mkECLeafFor(t, root, rootKey)

	s, err := store.Open(filepath.Join(t.TempDir(), "id.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	repo := s.Identities()

	registry := bootstrap.NewRegistry()
	registry.Register(common.BootstrapMethodFactoryCertJWT, factorycertjwt.New(factorycertjwt.Config{
		Roots: pool, Audience: jwtAudience,
		Replay: s.JWTReplay(),
	}))

	svc := identity.NewService(repo, fakeIssuer{}, policy, identity.NoReplacements{}, noopIssuanceLog{}, noopRevocationQueue{}, "margo.example.com", time.Hour)
	h := mishttp.NewEnrollmentHandler(mishttp.EnrollmentConfig{
		Registry: registry, Service: svc, Logger: slog.Default(),
	})
	return &jwtHarness{handler: h, root: root, rootKey: rootKey, leaf: leaf, leafKey: leafKey}
}

func mkECLeafFor(t *testing.T, root *x509.Certificate, rootKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: "jwt-conf-dev"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, root, &k.PublicKey, rootKey)
	c, _ := x509.ParseCertificate(der)
	return c, k
}

func mkJWTEnrollmentBody(t *testing.T, h *jwtHarness, opts bootstrapjwt.Options) []byte {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:            pkix.Name{CommonName: "d"},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, k)
	a := bootstrapjwt.Mint(t, h.leaf, []*x509.Certificate{h.leaf, h.root}, h.leafKey, jwtAudience, opts)
	req := common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(der)},
		BootstrapCredential: common.BootstrapCredentialDTO{
			Method: common.BootstrapMethodFactoryCertJWT,
			Proof:  &common.BootstrapProof{Assertion: a.Compact},
		},
	}
	b, _ := json.Marshal(req)
	return b
}

func TestErrorMatrix_JWT_Malformed(t *testing.T) {
	h := newJWTHarness(t, identity.StaticPolicy{NewLDI: true, KeyRotation: true})
	body, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: "AAAA"},
		BootstrapCredential: common.BootstrapCredentialDTO{
			Method: common.BootstrapMethodFactoryCertJWT,
			Proof:  &common.BootstrapProof{Assertion: "not-a-compact-jws"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.handler.ServeHTTP(rr, req)
	assertGolden(t, rr, loadGolden(t, "errors/malformed-jwt-assertion.golden.json"))
}

func TestSuccessShape_JWT_201_InitialEnrollment(t *testing.T) {
	h := newJWTHarness(t, identity.StaticPolicy{NewLDI: true, KeyRotation: true})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(mkJWTEnrollmentBody(t, h, bootstrapjwt.Options{})))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.handler.ServeHTTP(rr, req)
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

func TestSuccessShape_JWT_200_ReEnrollment(t *testing.T) {
	h := newJWTHarness(t, identity.StaticPolicy{NewLDI: true, KeyRotation: true})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(mkJWTEnrollmentBody(t, h, bootstrapjwt.Options{})))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("first status = %d", rr.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(mkJWTEnrollmentBody(t, h, bootstrapjwt.Options{})))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200", rr.Code)
	}
}
