package http_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	errpkg "github.com/margo/miaf-poc/pkg/mis/errors"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/store"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

// ---- shared fixture helpers ----

type renewHarness struct {
	srv     *httptest.Server
	limiter *store.RenewalEvents
	svc     *identity.Service
	ldi     identity.LDI
	peerKey *ecdsa.PrivateKey
	peer    *x509.Certificate
}

func newRenewHarness(t *testing.T, policy identity.StaticPolicy, limit store.RenewalEventsConfig) *renewHarness {
	return newRenewHarnessWithAuth(t, policy, limit, true, nil)
}

func newRenewHarnessNoPeer(t *testing.T, policy identity.StaticPolicy, limit store.RenewalEventsConfig, verifier mishttp.RenewalBearerVerifier) *renewHarness {
	return newRenewHarnessWithAuth(t, policy, limit, false, verifier)
}

func newRenewHarnessWithAuth(t *testing.T, policy identity.StaticPolicy, limit store.RenewalEventsConfig, installPeer bool, verifier mishttp.RenewalBearerVerifier) *renewHarness {
	t.Helper()
	td := "margo.example.com"
	s, err := store.Open(t.TempDir() + "/mis.sqlite3")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	repo := s.Identities()
	lim := s.RenewalEvents(limit)

	iss := fakeChainIssuer{}
	issuance := s.IssuedSVIDs()
	revocations := s.RevokedSVIDs()

	svc := identity.NewService(repo, iss, policy, identity.NoReplacements{}, issuance, revocations, td, time.Hour)

	ldi, err := repo.CreateLDIAndBind(context.Background(), mustHash("esi-x"), "urn:margo:bootstrap:factory-cert-mtls:v1", td)
	if err != nil {
		t.Fatalf("CreateLDIAndBind: %v", err)
	}

	peerKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	u, _ := url.Parse(ldi.SPIFFEID)
	peerTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(7),
		Subject:      pkix.Name{CommonName: "device"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		URIs:         []*url.URL{u},
	}
	peerDER, _ := x509.CreateCertificate(rand.Reader, peerTmpl, peerTmpl, &peerKey.PublicKey, peerKey)
	peer, _ := x509.ParseCertificate(peerDER)
	fingerprint := sha256.Sum256(peerDER)
	if err := issuance.Record(context.Background(), identity.IssuedSVIDRecord{
		SerialHex:      peer.SerialNumber.Text(16),
		FingerprintHex: hex.EncodeToString(fingerprint[:]),
		SPIFFEID:       ldi.SPIFFEID,
		LDIUUID:        ldi.UUID,
		Method:         "urn:margo:bootstrap:factory-cert-mtls:v1",
		IssuedAt:       time.Now().UTC(),
		NotBefore:      peer.NotBefore.UTC(),
		NotAfter:       peer.NotAfter.UTC(),
		LeafDER:        peerDER,
	}); err != nil {
		t.Fatalf("issuance.Record: %v", err)
	}

	h := mishttp.NewRenewalHandler(mishttp.RenewalConfig{
		Service:         svc,
		RateLimiter:     lim,
		BearerVerifier:  verifier,
		EndpointBaseURL: "https://mis.example.test",
	})

	// Wrap in a mux that installs the path-parameter value the way the real
	// mux does; use httptest so we can call it via a full HTTP round-trip.
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/identities/{spiffeIdEncoded}/renewal", h)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if installPeer {
			r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{peer}}
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	return &renewHarness{srv: srv, limiter: lim, svc: svc, ldi: ldi, peerKey: peerKey, peer: peer}
}

type fakeChainIssuer struct{}

func (fakeChainIssuer) IssueX509SVID(_ context.Context, spiffeID string, _ []byte, ttl time.Duration) ([][]byte, time.Time, error) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	u, _ := url.Parse(spiffeID)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(ttl),
		URIs:         []*url.URL{u},
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	return [][]byte{der}, tmpl.NotAfter, nil
}

func mustHash(s string) []byte {
	// 32 random-ish bytes (SHA-256-sized) - a literal "esi" stand-in.
	b := make([]byte, 32)
	copy(b, []byte(s))
	return b
}

func mkCSRForKey(t *testing.T, key *ecdsa.PrivateKey) string {
	t.Helper()
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, key)
	if err != nil {
		t.Fatalf("CreateCertificateRequest: %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func postRenewal(t *testing.T, h *renewHarness, spiffeID string, body []byte) (*http.Response, []byte) {
	return postRenewalWithHeaders(t, h, spiffeID, body, nil)
}

func postRenewalWithHeaders(t *testing.T, h *renewHarness, spiffeID string, body []byte, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	enc := common.EncodeSPIFFEID(spiffeID)
	u := h.srv.URL + "/api/v1/identities/" + enc + "/renewal"
	req, _ := http.NewRequest(http.MethodPost, u, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

// ---- tests ----

func TestRenewal_HappyPath_SameKey(t *testing.T) {
	h := newRenewHarness(t, identity.StaticPolicy{KeyRotation: false}, store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour})
	body, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: mkCSRForKey(t, h.peerKey)}, // same key as peer
	})
	resp, respBody := postRenewal(t, h, h.ldi.SPIFFEID, body)
	if resp.StatusCode != 200 {
		t.Fatalf("status %d body=%s", resp.StatusCode, respBody)
	}
	var out common.EnrollmentResponseDTO
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.SVID.CertificateChainPEM) == 0 {
		t.Fatal("empty chain")
	}
	// Sanity: response leaf parses and bears the expected SPIFFE ID.
	block, _ := pem.Decode([]byte(out.SVID.CertificateChainPEM[0]))
	leaf, _ := x509.ParseCertificate(block.Bytes)
	if len(leaf.URIs) == 0 || leaf.URIs[0].String() != h.ldi.SPIFFEID {
		t.Errorf("leaf URI SAN = %v, want %s", leaf.URIs, h.ldi.SPIFFEID)
	}
}

func TestRenewal_KeyRotationDenied(t *testing.T) {
	h := newRenewHarness(t, identity.StaticPolicy{KeyRotation: false}, store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour})
	rotKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	body, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: mkCSRForKey(t, rotKey)}, // different key
	})
	resp, respBody := postRenewal(t, h, h.ldi.SPIFFEID, body)
	if resp.StatusCode != 409 {
		t.Fatalf("status %d body=%s", resp.StatusCode, respBody)
	}
	var pd errpkg.ProblemDetails
	_ = json.Unmarshal(respBody, &pd)
	if pd.Type != "https://margo.org/docs/errors/key-rotation-not-permitted" {
		t.Errorf("type = %q", pd.Type)
	}
}

func TestRenewal_RateLimit_429WithRetryAfter(t *testing.T) {
	h := newRenewHarness(t, identity.StaticPolicy{KeyRotation: true}, store.RenewalEventsConfig{MaxInWindow: 1, Window: time.Hour})
	body, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: mkCSRForKey(t, h.peerKey)},
	})
	// First call succeeds (records event).
	if resp, _ := postRenewal(t, h, h.ldi.SPIFFEID, body); resp.StatusCode != 200 {
		t.Fatalf("warm-up status %d", resp.StatusCode)
	}
	// Second call is rate-limited.
	resp, respBody := postRenewal(t, h, h.ldi.SPIFFEID, body)
	if resp.StatusCode != 429 {
		t.Fatalf("status %d body=%s", resp.StatusCode, respBody)
	}
	if ra := resp.Header.Get("Retry-After"); ra == "" {
		t.Error("Retry-After header missing")
	}
}

func TestRenewal_SPIFFEIDMismatch(t *testing.T) {
	h := newRenewHarness(t, identity.StaticPolicy{KeyRotation: true}, store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour})
	// Ask for renewal of a different SPIFFE ID than the peer cert holds.
	body, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: mkCSRForKey(t, h.peerKey)},
	})
	other := "spiffe://margo.example.com/margo/device/other"
	resp, _ := postRenewal(t, h, other, body)
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

type stubRenewalBearerVerifier struct {
	spiffeID string
	err      error
}

func (s stubRenewalBearerVerifier) Verify(_ context.Context, _ string, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.spiffeID, nil
}

func TestRenewal_BearerAuth_HappyPath(t *testing.T) {
	verifier := &stubRenewalBearerVerifier{}
	h := newRenewHarnessNoPeer(t, identity.StaticPolicy{KeyRotation: false}, store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour}, verifier)
	verifier.spiffeID = h.ldi.SPIFFEID
	body, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: mkCSRForKey(t, h.peerKey)},
	})
	resp, respBody := postRenewalWithHeaders(t, h, h.ldi.SPIFFEID, body, map[string]string{"Authorization": "Bearer issued-jwt"})
	if resp.StatusCode != 200 {
		t.Fatalf("status %d body=%s", resp.StatusCode, respBody)
	}
}

func TestRenewal_BearerAuth_SubjectMismatch(t *testing.T) {
	h := newRenewHarnessNoPeer(t, identity.StaticPolicy{KeyRotation: true}, store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour}, stubRenewalBearerVerifier{spiffeID: "spiffe://margo.example.com/margo/device/other"})
	body, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: mkCSRForKey(t, h.peerKey)},
	})
	resp, _ := postRenewalWithHeaders(t, h, h.ldi.SPIFFEID, body, map[string]string{"Authorization": "Bearer issued-jwt"})
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestRenewal_BearerAuth_InvalidToken(t *testing.T) {
	h := newRenewHarnessNoPeer(t, identity.StaticPolicy{KeyRotation: true}, store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour}, stubRenewalBearerVerifier{err: context.DeadlineExceeded})
	body, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: mkCSRForKey(t, h.peerKey)},
	})
	resp, _ := postRenewalWithHeaders(t, h, h.ldi.SPIFFEID, body, map[string]string{"Authorization": "Bearer issued-jwt"})
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestRenewal_UnsupportedSVIDProfile(t *testing.T) {
	h := newRenewHarness(t, identity.StaticPolicy{KeyRotation: true}, store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour})
	body, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: "https://margo.org/profiles/spiffe/jwt-svid/v1",
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: mkCSRForKey(t, h.peerKey)},
	})
	resp, respBody := postRenewal(t, h, h.ldi.SPIFFEID, body)
	if resp.StatusCode != 422 {
		t.Fatalf("status %d body=%s", resp.StatusCode, respBody)
	}
	var pd errpkg.ProblemDetails
	_ = json.Unmarshal(respBody, &pd)
	if pd.Type != "https://margo.org/docs/errors/unsupported-svid-profile" {
		t.Errorf("type = %q", pd.Type)
	}
}

func TestRenewal_MalformedCSR(t *testing.T) {
	h := newRenewHarness(t, identity.StaticPolicy{KeyRotation: true}, store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour})
	body, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: "not-base64-nor-der"},
	})
	resp, respBody := postRenewal(t, h, h.ldi.SPIFFEID, body)
	if resp.StatusCode != 400 {
		t.Fatalf("status %d body=%s", resp.StatusCode, respBody)
	}
	var pd errpkg.ProblemDetails
	_ = json.Unmarshal(respBody, &pd)
	if pd.Type != "https://margo.org/docs/errors/invalid-svid-request" {
		t.Errorf("type = %q", pd.Type)
	}
}

func TestRenewal_NoPeerCert(t *testing.T) {
	td := "margo.example.com"
	s, _ := store.Open(t.TempDir() + "/mis.sqlite3")
	t.Cleanup(func() { s.Close() })

	svc := identity.NewService(s.Identities(), fakeChainIssuer{}, identity.StaticPolicy{KeyRotation: true}, identity.NoReplacements{}, noopIssuanceLog{}, noopRevocationQueue{}, td, time.Hour)
	h := mishttp.NewRenewalHandler(mishttp.RenewalConfig{
		Service:     svc,
		RateLimiter: s.RenewalEvents(store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour}),
	})

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/identities/{spiffeIdEncoded}/renewal", h)
	// Do NOT install a peer cert.
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body, _ := json.Marshal(common.EnrollmentRequestDTO{SVIDProfileURI: common.SVIDProfileURIX509V1})
	enc := common.EncodeSPIFFEID("spiffe://" + td + "/margo/device/x")
	resp, _ := http.Post(srv.URL+"/api/v1/identities/"+enc+"/renewal", "application/json", bytes.NewReader(body))
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}
