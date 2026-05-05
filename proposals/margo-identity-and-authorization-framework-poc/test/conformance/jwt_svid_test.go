//go:build conformance

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
	"encoding/json"
	"encoding/pem"
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
	"github.com/margo/miaf-poc/pkg/mis/jwtsigner"
	"github.com/margo/miaf-poc/pkg/mis/store"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

// ---- stubs for JWT SVID conformance tests ----

// noopJWTSVIDLog is a no-op JWTSVIDIssuanceLog for conformance tests.
type noopJWTSVIDLog struct{}

func (noopJWTSVIDLog) Record(_ context.Context, _ identity.IssuedJWTSVIDRecord) error { return nil }

// noopLeafLookup is a no-op jwtexchange.LeafLookup (never used when mTLS auth
// succeeds, but required by JWTSVIDConfig).
type noopLeafLookup struct{}

func (noopLeafLookup) LeafByActiveSPIFFEID(_ context.Context, _ string, _ time.Time) (identity.IssuedSVIDRecord, error) {
	return identity.IssuedSVIDRecord{}, identity.ErrNotFound
}

// noopReplay is a no-op replay.Store (never used when mTLS auth succeeds).
type noopReplay struct{}

func (noopReplay) Witness(_ context.Context, _ string, _ time.Time) error { return nil }

// ---- golden fixture loader ----

func loadJWTSVIDGolden(t *testing.T, relPath string) goldenProblem {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "conformance", "jwt-svid", relPath))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	var g goldenProblem
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return g
}

// ---- harness ----

type jwtSVIDHarness struct {
	srv      *httptest.Server
	leaf     *x509.Certificate
	spiffeID string
	encoded  string
}

func writeECDSASigningKey(t *testing.T, dir string) string {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	path := filepath.Join(dir, "jwt-signing.key")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return path
}

func newJWTSVIDConformanceHarness(t *testing.T, lim store.RenewalEventsConfig) *jwtSVIDHarness {
	t.Helper()

	s, _ := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	t.Cleanup(func() { s.Close() })

	td := "margo.example.com"
	svc := identity.NewService(
		s.Identities(),
		fakeIssuer{},
		identity.StaticPolicy{KeyRotation: true},
		identity.NoReplacements{},
		noopIssuanceLog{},
		noopRevocationQueue{},
		td,
		time.Hour,
	)

	ldi, _ := s.Identities().CreateLDIAndBind(context.Background(), make([]byte, 32), "urn:margo:bootstrap:factory-cert-mtls:v1", td)

	// Build a peer cert whose URI SAN matches the LDI SPIFFE ID.
	peerKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	u, _ := url.Parse(ldi.SPIFFEID)
	peerTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(77),
		Subject:      pkix.Name{CommonName: "jwt-svid-conf-dev"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		URIs:         []*url.URL{u},
	}
	peerDER, _ := x509.CreateCertificate(rand.Reader, peerTmpl, peerTmpl, &peerKey.PublicKey, peerKey)
	leaf, _ := x509.ParseCertificate(peerDER)

	// Build a real jwtsigner from a temp key file.
	keyPath := writeECDSASigningKey(t, t.TempDir())
	signer, err := jwtsigner.New(jwtsigner.Config{KeyFile: keyPath})
	if err != nil {
		t.Fatalf("jwtsigner.New: %v", err)
	}

	const issuerURL = "https://mis.conformance.test"
	const endpointBase = "https://mis.conformance.test"

	h := mishttp.NewJWTSVIDHandler(mishttp.JWTSVIDConfig{
		Service:      svc,
		Signer:       signer,
		IssuanceLog:  noopJWTSVIDLog{},
		LeafLookup:   noopLeafLookup{},
		ReplayStore:  noopReplay{},
		RateLimiter:  s.RenewalEvents(lim),
		IssuerURL:    issuerURL,
		EndpointBase: endpointBase,
		MaxTTL:       time.Hour,
		DefaultTTL:   5 * time.Minute,
		Logger:       slog.Default(),
	})

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid", h)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	return &jwtSVIDHarness{
		srv:      srv,
		leaf:     leaf,
		spiffeID: ldi.SPIFFEID,
		encoded:  common.EncodeSPIFFEID(ldi.SPIFFEID),
	}
}

func (h *jwtSVIDHarness) url() string {
	return h.srv.URL + "/api/v1/identities/" + h.encoded + "/jwt-svid"
}

// ---- tests ----

func TestConformance_JWTSVID_201(t *testing.T) {
	h := newJWTSVIDConformanceHarness(t, store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour})

	body, _ := json.Marshal(common.JWTSVIDExchangeRequestDTO{
		Audience: []string{"https://dfm.example.com/"},
	})
	resp, err := http.Post(h.url(), "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var out common.JWTSVIDExchangeResponseDTO
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Golden fixture defines the shape: non-empty jwt and expires_at.
	// Exact values are non-deterministic (signed JWT, timestamp), so we
	// validate the shape rather than byte-comparing to the golden.
	if out.JWT == "" {
		t.Error("jwt field is empty")
	}
	if out.ExpiresAt == "" {
		t.Error("expires_at field is empty")
	}
	// Validate expires_at parses as RFC3339.
	if _, err := time.Parse(time.RFC3339, out.ExpiresAt); err != nil {
		t.Errorf("expires_at %q is not RFC3339: %v", out.ExpiresAt, err)
	}
}

func TestConformance_JWTSVID_Errors(t *testing.T) {
	cases := []struct {
		name     string
		lim      store.RenewalEventsConfig
		fixture  string
		wantCode int
		body     func(h *jwtSVIDHarness) []byte
		setup    func(h *jwtSVIDHarness)
	}{
		{
			name:     "invalid-audience",
			lim:      store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour},
			fixture:  "errors/invalid-audience.golden.json",
			wantCode: 400,
			body: func(_ *jwtSVIDHarness) []byte {
				// aud is missing/empty ==> invalid-audience
				b, _ := json.Marshal(common.JWTSVIDExchangeRequestDTO{
					Audience: []string{},
				})
				return b
			},
		},
		{
			name:     "invalid-svid-request",
			lim:      store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour},
			fixture:  "errors/invalid-svid-request.golden.json",
			wantCode: 400,
			body: func(_ *jwtSVIDHarness) []byte {
				// Malformed JSON ==> invalid-svid-request
				return []byte(`{not valid json}`)
			},
		},
		{
			name:     "too-many-requests",
			lim:      store.RenewalEventsConfig{MaxInWindow: 1, Window: time.Hour},
			fixture:  "errors/too-many-requests.golden.json",
			wantCode: 429,
			body: func(h *jwtSVIDHarness) []byte {
				// Burn the one-slot budget first.
				warm, _ := json.Marshal(common.JWTSVIDExchangeRequestDTO{
					Audience: []string{"https://dfm.example.com/"},
				})
				first, _ := http.Post(h.url(), "application/json", bytes.NewReader(warm))
				first.Body.Close()
				// Now the second request should be rate-limited.
				b, _ := json.Marshal(common.JWTSVIDExchangeRequestDTO{
					Audience: []string{"https://dfm.example.com/"},
				})
				return b
			},
		},
		{
			name:    "unsupported-svid-profile",
			fixture: "errors/unsupported-svid-profile.golden.json",
			body:    func(_ *jwtSVIDHarness) []byte { return nil },
			lim:     store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.name == "unsupported-svid-profile" {
				t.Skip("no svid_profile_uri field in jwt-svid request body; profile is implicit")
			}

			golden := loadJWTSVIDGolden(t, c.fixture)
			h := newJWTSVIDConformanceHarness(t, c.lim)
			body := c.body(h)

			resp, err := http.Post(h.url(), "application/json", bytes.NewReader(body))
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
			if err := json.NewDecoder(resp.Body).Decode(&pd); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if pd.Type != golden.Type || pd.Title != golden.Title || pd.Status != golden.Status {
				t.Errorf("problem details mismatch\n got: %+v\n want: %+v", pd, golden)
			}

			if c.name == "too-many-requests" && resp.Header.Get("Retry-After") == "" {
				t.Error("Retry-After header missing on 429")
			}
		})
	}
}
