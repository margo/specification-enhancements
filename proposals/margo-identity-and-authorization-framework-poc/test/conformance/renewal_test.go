package conformance_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
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
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/store"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

func loadRenewalGolden(t *testing.T, relPath string) goldenProblem {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "conformance", "renewal", relPath))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	var g goldenProblem
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return g
}

type renewalHarness struct {
	srv  *httptest.Server
	leaf *x509.Certificate
	key  *ecdsa.PrivateKey
	spid string
}

func newRenewalConformanceHarness(t *testing.T, policy identity.StaticPolicy, lim store.RenewalEventsConfig) *renewalHarness {
	t.Helper()
	s, _ := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	t.Cleanup(func() { s.Close() })

	td := "margo.example.com"
	issuance := s.IssuedSVIDs()
	revocations := s.RevokedSVIDs()
	svc := identity.NewService(s.Identities(), fakeIssuer{}, policy, identity.NoReplacements{}, issuance, revocations, td, time.Hour)
	ldi, _ := s.Identities().CreateLDIAndBind(context.Background(), make([]byte, 32), "urn:margo:bootstrap:factory-cert-mtls:v1", td)

	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	u, _ := url.Parse(ldi.SPIFFEID)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), URIs: []*url.URL{u}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour)}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	leaf, _ := x509.ParseCertificate(der)
	fingerprint := sha256.Sum256(der)
	if err := issuance.Record(context.Background(), identity.IssuedSVIDRecord{
		SerialHex:      leaf.SerialNumber.Text(16),
		FingerprintHex: hex.EncodeToString(fingerprint[:]),
		SPIFFEID:       ldi.SPIFFEID,
		LDIUUID:        ldi.UUID,
		Method:         "urn:margo:bootstrap:factory-cert-mtls:v1",
		IssuedAt:       time.Now().UTC(),
		NotBefore:      leaf.NotBefore.UTC(),
		NotAfter:       leaf.NotAfter.UTC(),
		LeafDER:        der,
	}); err != nil {
		t.Fatalf("issuance.Record: %v", err)
	}

	h := mishttp.NewRenewalHandler(mishttp.RenewalConfig{
		Service:     svc,
		RateLimiter: s.RenewalEvents(lim),
		Logger:      slog.Default(),
	})
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/identities/{spiffeIdEncoded}/renewals", h)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return &renewalHarness{srv: srv, leaf: leaf, key: key, spid: ldi.SPIFFEID}
}

func renewalCSR(t *testing.T, key *ecdsa.PrivateKey) string {
	t.Helper()
	der, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{SignatureAlgorithm: x509.ECDSAWithSHA256}, key)
	return base64.StdEncoding.EncodeToString(der)
}

func TestConformance_Renewal_200(t *testing.T) {
	h := newRenewalConformanceHarness(t, identity.StaticPolicy{KeyRotation: true}, store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour})
	body, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: renewalCSR(t, h.key)},
	})
	u := h.srv.URL + "/api/v1/identities/" + common.EncodeSPIFFEID(h.spid) + "/renewals"
	resp, err := http.Post(u, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST renewals: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var out common.EnrollmentResponseDTO
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out.SVIDProfileURI != common.SVIDProfileURIX509V1 {
		t.Errorf("profile uri = %q", out.SVIDProfileURI)
	}
	if len(out.SVID.CertificateChainPEM) == 0 {
		t.Error("empty chain")
	}
}

func TestConformance_Renewal_Errors(t *testing.T) {
	cases := []struct {
		name     string
		policy   identity.StaticPolicy
		lim      store.RenewalEventsConfig
		fixture  string
		mutate   func(*testing.T, *renewalHarness) (statusWant int, url string, body []byte)
		wantCode int
	}{
		{
			name:     "key-rotation-not-permitted",
			policy:   identity.StaticPolicy{KeyRotation: false},
			lim:      store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour},
			fixture:  "errors/key-rotation-not-permitted.golden.json",
			wantCode: 409,
			mutate: func(t *testing.T, h *renewalHarness) (int, string, []byte) {
				rotKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				b, _ := json.Marshal(common.EnrollmentRequestDTO{
					SVIDProfileURI: common.SVIDProfileURIX509V1,
					SVIDRequest:    common.SVIDRequestX509DTO{CSR: renewalCSR(t, rotKey)},
				})
				return 409, h.srv.URL + "/api/v1/identities/" + common.EncodeSPIFFEID(h.spid) + "/renewals", b
			},
		},
		{
			name:     "unsupported-svid-profile",
			policy:   identity.StaticPolicy{KeyRotation: true},
			lim:      store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour},
			fixture:  "errors/unsupported-svid-profile.golden.json",
			wantCode: 422,
			mutate: func(t *testing.T, h *renewalHarness) (int, string, []byte) {
				b, _ := json.Marshal(common.EnrollmentRequestDTO{
					SVIDProfileURI: "https://margo.org/profiles/spiffe/jwt-svid/v1",
					SVIDRequest:    common.SVIDRequestX509DTO{CSR: renewalCSR(t, h.key)},
				})
				return 422, h.srv.URL + "/api/v1/identities/" + common.EncodeSPIFFEID(h.spid) + "/renewals", b
			},
		},
		{
			name:     "invalid-svid-request",
			policy:   identity.StaticPolicy{KeyRotation: true},
			lim:      store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour},
			fixture:  "errors/invalid-svid-request.golden.json",
			wantCode: 400,
			mutate: func(t *testing.T, h *renewalHarness) (int, string, []byte) {
				b, _ := json.Marshal(common.EnrollmentRequestDTO{
					SVIDProfileURI: common.SVIDProfileURIX509V1,
					SVIDRequest:    common.SVIDRequestX509DTO{CSR: "not-a-csr"},
				})
				return 400, h.srv.URL + "/api/v1/identities/" + common.EncodeSPIFFEID(h.spid) + "/renewals", b
			},
		},
		{
			name:     "too-many-requests",
			policy:   identity.StaticPolicy{KeyRotation: true},
			lim:      store.RenewalEventsConfig{MaxInWindow: 0 + 1, Window: time.Hour}, // 1 allowed
			fixture:  "errors/too-many-requests.golden.json",
			wantCode: 429,
			mutate: func(t *testing.T, h *renewalHarness) (int, string, []byte) {
				// First burn the one-slot budget.
				u := h.srv.URL + "/api/v1/identities/" + common.EncodeSPIFFEID(h.spid) + "/renewals"
				b, _ := json.Marshal(common.EnrollmentRequestDTO{
					SVIDProfileURI: common.SVIDProfileURIX509V1,
					SVIDRequest:    common.SVIDRequestX509DTO{CSR: renewalCSR(t, h.key)},
				})
				warm, _ := http.Post(u, "application/json", bytes.NewReader(b))
				warm.Body.Close()
				return 429, u, b
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			golden := loadRenewalGolden(t, c.fixture)
			h := newRenewalConformanceHarness(t, c.policy, c.lim)
			_, url, body := c.mutate(t, h)
			resp, err := http.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("post: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != c.wantCode {
				t.Fatalf("status = %d, want %d", resp.StatusCode, c.wantCode)
			}
			if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
				t.Errorf("Content-Type = %q, want application/problem+json", ct)
			}
			var pd struct {
				Type   string `json:"type"`
				Title  string `json:"title"`
				Status int    `json:"status"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&pd)
			if pd.Type != golden.Type || pd.Title != golden.Title || pd.Status != golden.Status {
				t.Errorf("problem details mismatch\n got: %+v\n want: %+v", pd, golden)
			}
			if c.name == "too-many-requests" && resp.Header.Get("Retry-After") == "" {
				t.Error("Retry-After header missing on 429")
			}
		})
	}
}
