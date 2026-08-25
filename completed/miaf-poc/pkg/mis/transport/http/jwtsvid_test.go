package http_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertjwt/replay"
	"github.com/margo/miaf-poc/pkg/mis/bundlemap"
	errpkg "github.com/margo/miaf-poc/pkg/mis/errors"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/store"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

// ---- stubs ----

// stubJWTSignerT implements identity.JWTSigner for handler tests.
type stubJWTSignerT struct {
	compact string
	err     error
}

func (s *stubJWTSignerT) Sign(_ context.Context, _ jwt.Claims) (string, error) {
	return s.compact, s.err
}

// stubJWTAuditT implements identity.JWTSVIDIssuanceLog.
type stubJWTAuditT struct{}

func (stubJWTAuditT) Record(_ context.Context, _ identity.IssuedJWTSVIDRecord) error { return nil }

// stubBundleSourceT implements mishttp.BundleSource. It returns a snapshot
// containing a single X.509 root authority used to validate x5c chains in
// client_assertion JWTs.
type stubBundleSourceT struct {
	root *x509.Certificate
}

func (s *stubBundleSourceT) GetTrustAnchors(_ context.Context) (bundlemap.TrustAnchorSet, time.Time, error) {
	return bundlemap.TrustAnchorSet{
		TrustDomain:     "margo.example.com",
		X509Authorities: []*x509.Certificate{s.root},
	}, time.Now(), nil
}

// testCA mints a self-signed root and signs leaf certificates.
type testCA struct {
	rootKey  *ecdsa.PrivateKey
	rootCert *x509.Certificate
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("ca sign: %v", err)
	}
	root, _ := x509.ParseCertificate(der)
	return &testCA{rootKey: key, rootCert: root}
}

func (c *testCA) signLeaf(t *testing.T, spiffeID string, pub crypto.PublicKey, serial int64) *x509.Certificate {
	t.Helper()
	u, _ := url.Parse(spiffeID)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		URIs:         []*url.URL{u},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.rootCert, pub, c.rootKey)
	if err != nil {
		t.Fatalf("leaf sign: %v", err)
	}
	leaf, _ := x509.ParseCertificate(der)
	return leaf
}

// stubReplayT implements replay.Store.
type stubReplayT struct{}

func (stubReplayT) Witness(_ context.Context, _ string, _ time.Time) error { return nil }

// stubRateLimiterT implements identity.RenewalRateLimiter.
type stubRateLimiterT struct {
	allowed    bool
	retryAfter time.Duration
	allowErr   error
	recordErr  error
}

func (s *stubRateLimiterT) Allow(_ context.Context, _ string, _ time.Time) (bool, time.Duration, error) {
	return s.allowed, s.retryAfter, s.allowErr
}
func (s *stubRateLimiterT) Record(_ context.Context, _ string, _ time.Time) error {
	return s.recordErr
}

// ---- harness ----

const (
	jwtHarnessSpiffeID  = "spiffe://margo.example.com/margo/device/jwt-test-01"
	jwtHarnessIssuerURL = "https://mis.example.test"
	jwtHarnessBase      = "https://mis.example.test"
)

type jwtHarness struct {
	srv      *httptest.Server
	svc      *identity.Service
	ldi      identity.LDI
	peerKey  *ecdsa.PrivateKey
	peer     *x509.Certificate
	ca       *testCA
	limiter  *stubRateLimiterT
	signer   *stubJWTSignerT
	bundle   *stubBundleSourceT
	replayS  replay.Store
	encoded  string
	spiffeID string
}

func newJWTHarness(t *testing.T) *jwtHarness {
	t.Helper()
	td := "margo.example.com"

	s, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	repo := s.Identities()
	ldi, err := repo.CreateLDIAndBind(context.Background(), mustHash("jwt-esi-01"),
		"urn:margo:bootstrap:factory-cert-mtls:v1", td)
	if err != nil {
		t.Fatalf("CreateLDIAndBind: %v", err)
	}

	svc := identity.NewService(
		repo,
		fakeChainIssuer{},
		identity.StaticPolicy{KeyRotation: true},
		identity.NoReplacements{},
		noopIssuanceLog{},
		noopRevocationQueue{},
		td,
		time.Hour,
	)

	// Build a CA-signed peer cert whose URI SAN matches the LDI SPIFFE ID.
	ca := newTestCA(t)
	peerKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	peer := ca.signLeaf(t, ldi.SPIFFEID, &peerKey.PublicKey, 99)

	signer := &stubJWTSignerT{compact: "eyJ.fake.svid"}
	limiter := &stubRateLimiterT{allowed: true}
	bundle := &stubBundleSourceT{root: ca.rootCert}
	replayStore := stubReplayT{}

	encoded := common.EncodeSPIFFEID(ldi.SPIFFEID)

	cfg := mishttp.JWTSVIDConfig{
		Service:      svc,
		Signer:       signer,
		IssuanceLog:  stubJWTAuditT{},
		BundleSource: bundle,
		ReplayStore:  replayStore,
		RateLimiter:  limiter,
		IssuerURL:    jwtHarnessIssuerURL,
		EndpointBase: jwtHarnessBase,
		MaxTTL:       time.Hour,
		DefaultTTL:   5 * time.Minute,
	}
	h := mishttp.NewJWTSVIDHandler(cfg)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid", h)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Inject peer cert so mTLS tests work without a real TLS listener.
		r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{peer}}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	return &jwtHarness{
		srv:      srv,
		svc:      svc,
		ldi:      ldi,
		peerKey:  peerKey,
		peer:     peer,
		ca:       ca,
		limiter:  limiter,
		signer:   signer,
		bundle:   bundle,
		replayS:  replayStore,
		encoded:  encoded,
		spiffeID: ldi.SPIFFEID,
	}
}

// newJWTHarnessNoPeer builds a harness that does NOT inject a peer cert,
// so handlers reach the client_assertion branch.
func newJWTHarnessNoPeer(t *testing.T) *jwtHarness {
	t.Helper()
	td := "margo.example.com"

	s, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	repo := s.Identities()
	ldi, err := repo.CreateLDIAndBind(context.Background(), mustHash("jwt-esi-nopeer"),
		"urn:margo:bootstrap:factory-cert-mtls:v1", td)
	if err != nil {
		t.Fatalf("CreateLDIAndBind: %v", err)
	}

	svc := identity.NewService(
		repo,
		fakeChainIssuer{},
		identity.StaticPolicy{KeyRotation: true},
		identity.NoReplacements{},
		noopIssuanceLog{},
		noopRevocationQueue{},
		td,
		time.Hour,
	)

	ca := newTestCA(t)
	peerKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	peer := ca.signLeaf(t, ldi.SPIFFEID, &peerKey.PublicKey, 100)

	signer := &stubJWTSignerT{compact: "eyJ.fake.svid"}
	limiter := &stubRateLimiterT{allowed: true}
	bundle := &stubBundleSourceT{root: ca.rootCert}

	replayStore := replay.NewMemoryStore(replay.Config{})

	encoded := common.EncodeSPIFFEID(ldi.SPIFFEID)

	cfg := mishttp.JWTSVIDConfig{
		Service:      svc,
		Signer:       signer,
		IssuanceLog:  stubJWTAuditT{},
		BundleSource: bundle,
		ReplayStore:  replayStore,
		RateLimiter:  limiter,
		IssuerURL:    jwtHarnessIssuerURL,
		EndpointBase: jwtHarnessBase,
		MaxTTL:       time.Hour,
		DefaultTTL:   5 * time.Minute,
	}
	h := mishttp.NewJWTSVIDHandler(cfg)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid", h)

	// No peer cert injection.
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &jwtHarness{
		srv:      srv,
		svc:      svc,
		ldi:      ldi,
		peerKey:  peerKey,
		peer:     peer,
		ca:       ca,
		limiter:  limiter,
		signer:   signer,
		bundle:   bundle,
		replayS:  replayStore,
		encoded:  encoded,
		spiffeID: ldi.SPIFFEID,
	}
}

// postJWT sends a POST to the jwt-svid endpoint and returns the response + body.
func postJWT(t *testing.T, h *jwtHarness, encoded string, body []byte, extraHeaders map[string]string) (*http.Response, []byte) {
	t.Helper()
	u := h.srv.URL + "/api/v1/identities/" + encoded + "/jwt-svid"
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range extraHeaders {
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

// mkClientAssertion builds a signed client_assertion JWT for tests, embedding
// the SVID chain in the JWS x5c header per the SUP X.509 SVID Profile.
func mkClientAssertion(t *testing.T, key *ecdsa.PrivateKey, chain []*x509.Certificate, spiffeID, audURL string) string {
	t.Helper()
	x5c := make([]string, len(chain))
	for i, c := range chain {
		x5c[i] = base64.StdEncoding.EncodeToString(c.Raw)
	}
	sopts := (&jose.SignerOptions{}).
		WithType("JWT").
		WithHeader(jose.HeaderKey("x5c"), x5c)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: key}, sopts)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	out, err := jwt.Signed(signer).Claims(jwt.Claims{
		Issuer:   spiffeID,
		Subject:  spiffeID,
		Audience: jwt.Audience{audURL},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(2 * time.Minute)),
		ID:       uuid.NewString(),
	}).Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	return out
}

// ---- tests ----

func TestJWTSVID_HappyPath_MTLSAuth(t *testing.T) {
	h := newJWTHarness(t)
	body, _ := json.Marshal(common.JWTSVIDExchangeRequestDTO{
		Audience: []string{"https://workload.example.test"},
	})
	resp, respBody := postJWT(t, h, h.encoded, body, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, respBody)
	}
	var out common.JWTSVIDExchangeResponseDTO
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.JWT == "" {
		t.Error("JWT is empty")
	}
	if out.ExpiresAt == "" {
		t.Error("expires_at is empty")
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestJWTSVID_HappyPath_ClientAssertion(t *testing.T) {
	h := newJWTHarnessNoPeer(t)
	encoded := common.EncodeSPIFFEID(h.spiffeID)
	audURL := jwtHarnessBase + "/api/v1/identities/" + encoded + "/jwt-svid"
	chain := []*x509.Certificate{h.peer, h.ca.rootCert}
	assertion := mkClientAssertion(t, h.peerKey, chain, h.spiffeID, audURL)

	body, _ := json.Marshal(common.JWTSVIDExchangeRequestDTO{
		Audience:        []string{"https://workload.example.test"},
		ClientAssertion: assertion,
	})
	resp, respBody := postJWT(t, h, encoded, body, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, respBody)
	}
	var out common.JWTSVIDExchangeResponseDTO
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.JWT == "" {
		t.Error("JWT is empty")
	}
}

func TestJWTSVID_InvalidAudience_EmptyArray(t *testing.T) {
	h := newJWTHarness(t)
	body, _ := json.Marshal(common.JWTSVIDExchangeRequestDTO{
		Audience: []string{}, // empty - invalid
	})
	resp, respBody := postJWT(t, h, h.encoded, body, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, respBody)
	}
	var pd errpkg.ProblemDetails
	_ = json.Unmarshal(respBody, &pd)
	if pd.Type != "https://margo.org/docs/errors/invalid-audience" {
		t.Errorf("problem type = %q", pd.Type)
	}
}

func TestJWTSVID_InvalidRequest_NegativeTTL(t *testing.T) {
	h := newJWTHarness(t)
	// Encode TTL as -1 in raw JSON since the DTO uses omitempty on ttl.
	body := []byte(`{"aud":["https://workload.example.test"],"ttl":-1}`)
	resp, respBody := postJWT(t, h, h.encoded, body, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, respBody)
	}
	var pd errpkg.ProblemDetails
	_ = json.Unmarshal(respBody, &pd)
	if pd.Type != "https://margo.org/docs/errors/invalid-svid-request" {
		t.Errorf("problem type = %q", pd.Type)
	}
}

func TestJWTSVID_InvalidRequest_MalformedJSON(t *testing.T) {
	h := newJWTHarness(t)
	resp, respBody := postJWT(t, h, h.encoded, []byte(`{not-json`), nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, respBody)
	}
	var pd errpkg.ProblemDetails
	_ = json.Unmarshal(respBody, &pd)
	if pd.Type != "https://margo.org/docs/errors/invalid-svid-request" {
		t.Errorf("problem type = %q", pd.Type)
	}
}

func TestJWTSVID_Unauthorized_NoAuth(t *testing.T) {
	// Use the no-peer harness so no TLS peer cert is injected.
	h := newJWTHarnessNoPeer(t)
	body, _ := json.Marshal(common.JWTSVIDExchangeRequestDTO{
		Audience: []string{"https://workload.example.test"},
		// ClientAssertion deliberately absent.
	})
	resp, _ := postJWT(t, h, h.encoded, body, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestJWTSVID_Unauthorized_BearerAuth(t *testing.T) {
	h := newJWTHarness(t)
	body, _ := json.Marshal(common.JWTSVIDExchangeRequestDTO{
		Audience: []string{"https://workload.example.test"},
	})
	resp, _ := postJWT(t, h, h.encoded, body, map[string]string{
		"Authorization": "Bearer pretend-jwt",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestJWTSVID_Unauthorized_PeerCertURIMismatch(t *testing.T) {
	h := newJWTHarness(t)
	// Request a different SPIFFE ID than what the injected peer cert holds.
	otherSpiffeID := "spiffe://margo.example.com/margo/device/other-device"
	otherEncoded := common.EncodeSPIFFEID(otherSpiffeID)
	body, _ := json.Marshal(common.JWTSVIDExchangeRequestDTO{
		Audience: []string{"https://workload.example.test"},
	})
	resp, _ := postJWT(t, h, otherEncoded, body, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestJWTSVID_RateLimit_429WithRetryAfter(t *testing.T) {
	h := newJWTHarness(t)
	// Override limiter to deny.
	h.limiter.allowed = false
	h.limiter.retryAfter = 30 * time.Second

	body, _ := json.Marshal(common.JWTSVIDExchangeRequestDTO{
		Audience: []string{"https://workload.example.test"},
	})

	// We need a fresh harness-level handler wired with our updated limiter.
	// The easiest approach: rebuild a handler pointing at the same service,
	// using a fresh mux/server.
	td := "margo.example.com"
	s2, _ := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	t.Cleanup(func() { _ = s2.Close() })
	ldi2, _ := s2.Identities().CreateLDIAndBind(context.Background(), mustHash("rl-esi"), "urn:margo:bootstrap:factory-cert-mtls:v1", td)
	svc2 := identity.NewService(s2.Identities(), fakeChainIssuer{}, identity.StaticPolicy{KeyRotation: true}, identity.NoReplacements{}, noopIssuanceLog{}, noopRevocationQueue{}, td, time.Hour)

	ca2 := newTestCA(t)
	peerKey2, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	peer2 := ca2.signLeaf(t, ldi2.SPIFFEID, &peerKey2.PublicKey, 55)

	rateLimiter := &stubRateLimiterT{allowed: false, retryAfter: 30 * time.Second}
	cfg2 := mishttp.JWTSVIDConfig{
		Service:      svc2,
		Signer:       &stubJWTSignerT{compact: "eyJ.x"},
		IssuanceLog:  stubJWTAuditT{},
		BundleSource: &stubBundleSourceT{root: ca2.rootCert},
		ReplayStore:  stubReplayT{},
		RateLimiter:  rateLimiter,
		IssuerURL:    jwtHarnessIssuerURL,
		EndpointBase: jwtHarnessBase,
		MaxTTL:       time.Hour,
	}
	h2 := mishttp.NewJWTSVIDHandler(cfg2)

	mux2 := http.NewServeMux()
	mux2.Handle("POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid", h2)
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{peer2}}
		mux2.ServeHTTP(w, r)
	}))
	t.Cleanup(srv2.Close)

	enc2 := common.EncodeSPIFFEID(ldi2.SPIFFEID)
	req2, _ := http.NewRequest(http.MethodPost, srv2.URL+"/api/v1/identities/"+enc2+"/jwt-svid", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err2 := http.DefaultClient.Do(req2)
	if err2 != nil {
		t.Fatalf("Do: %v", err2)
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)

	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, body = %s", resp2.StatusCode, body2)
	}
	if ra := resp2.Header.Get("Retry-After"); ra == "" {
		t.Error("Retry-After header missing")
	}
	var pd errpkg.ProblemDetails
	_ = json.Unmarshal(body2, &pd)
	if pd.Type != "https://margo.org/docs/errors/too-many-requests" {
		t.Errorf("problem type = %q", pd.Type)
	}
}

func TestJWTSVID_NotFound_UnknownSPIFFEID(t *testing.T) {
	// Use a SPIFFE ID that is not in the store; we need a peer cert that matches
	// that unknown ID so mTLS auth passes and we reach the issuance step.
	unknownID := "spiffe://margo.example.com/margo/device/unknown-99"
	caU := newTestCA(t)
	unknownKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	unknownCert := caU.signLeaf(t, unknownID, &unknownKey.PublicKey, 77)

	// Build a second server that injects the unknown peer cert.
	td := "margo.example.com"
	s2, _ := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	t.Cleanup(func() { _ = s2.Close() })
	svc2 := identity.NewService(s2.Identities(), fakeChainIssuer{}, identity.StaticPolicy{KeyRotation: true}, identity.NoReplacements{}, noopIssuanceLog{}, noopRevocationQueue{}, td, time.Hour)

	cfg2 := mishttp.JWTSVIDConfig{
		Service:      svc2,
		Signer:       &stubJWTSignerT{compact: "eyJ.x"},
		IssuanceLog:  stubJWTAuditT{},
		BundleSource: &stubBundleSourceT{root: caU.rootCert},
		ReplayStore:  stubReplayT{},
		RateLimiter:  &stubRateLimiterT{allowed: true},
		IssuerURL:    jwtHarnessIssuerURL,
		EndpointBase: jwtHarnessBase,
		MaxTTL:       time.Hour,
	}
	h2 := mishttp.NewJWTSVIDHandler(cfg2)

	mux2 := http.NewServeMux()
	mux2.Handle("POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid", h2)
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{unknownCert}}
		mux2.ServeHTTP(w, r)
	}))
	t.Cleanup(srv2.Close)

	body, _ := json.Marshal(common.JWTSVIDExchangeRequestDTO{
		Audience: []string{"https://workload.example.test"},
	})
	enc := common.EncodeSPIFFEID(unknownID)
	req, _ := http.NewRequest(http.MethodPost, srv2.URL+"/api/v1/identities/"+enc+"/jwt-svid", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, respBody)
	}
}
