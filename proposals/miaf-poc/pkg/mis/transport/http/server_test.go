package http_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/store"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

func TestServer_NotImplementedEndpoints_ReturnProblemDetails(t *testing.T) {
	stub501 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = fmt.Fprint(w, `{"type":"about:blank","title":"Not Implemented","status":501}`)
	})
	stub503 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(w, `{"type":"about:blank","title":"Service Unavailable","status":503}`)
	})
	stubAny := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	})
	srv := mishttp.NewServer(mishttp.Config{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		DiscoveryHandler: mishttp.NewDiscoveryHandler(&common.DiscoveryResponseDTO{
			TrustDomain:                 "example.test",
			TrustBundleURI:              "https://mis.example.test/.well-known/spiffe/bundle.json",
			MargoIdentityServiceBaseURI: "https://mis.example.test",
			SupportedBootstrapMethods:   []string{common.BootstrapMethodFactoryCertMTLS},
			SVIDProfilesSupported: []string{common.SVIDProfileURIX509V1},
		}),
		BundleMapHandler:   stubAny,
		EnrollmentHandler:  stub501,
		RenewalHandler:     stubAny,
		JWTSVIDHandler:     stub501,
		RevocationsHandler: stub503,
	})

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	cases := []struct {
		path       string
		method     string
		wantStatus int
	}{
		{"/api/v1/identities", http.MethodPost, http.StatusNotImplemented},
		{"/api/v1/identities/foo/jwt-svid", http.MethodPost, http.StatusNotImplemented},
		{"/api/v1/revocations", http.MethodGet, http.StatusServiceUnavailable},
	}
	for _, c := range cases {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			var resp *http.Response
			var err error
			switch c.method {
			case http.MethodPost:
				resp, err = http.Post(ts.URL+c.path, "application/json", strings.NewReader("{}"))
			case http.MethodGet:
				resp, err = http.Get(ts.URL + c.path)
			}
			if err != nil {
				t.Fatalf("%s: %v", c.method, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != c.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, c.wantStatus)
			}
			if got := resp.Header.Get("Content-Type"); got != "application/problem+json" {
				t.Errorf("Content-Type = %q, want application/problem+json", got)
			}
			if got := resp.Header.Get("X-Request-ID"); got == "" {
				t.Error("X-Request-ID header missing")
			}
		})
	}
}

func TestServer_RenewalRouteLive(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	// Minimal service + limiter - we never reach the service because the
	// request has no peer cert, but the handler constructor requires non-nil.
	svc := identity.NewService(
		s.Identities(),
		fakeServerIssuer{},
		identity.StaticPolicy{KeyRotation: true},
		identity.NoReplacements{},
		noopIssuanceLog{},
		noopRevocationQueue{},
		"margo.example.com",
		time.Hour,
	)
	lim := s.RenewalEvents(store.RenewalEventsConfig{MaxInWindow: 5, Window: time.Hour})

	stubAny := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	})
	srv := mishttp.NewServer(mishttp.Config{
		Logger:             slog.Default(),
		DiscoveryHandler:   mishttp.NewDiscoveryHandler(&common.DiscoveryResponseDTO{}),
		BundleMapHandler:   stubAny,
		EnrollmentHandler:  stubAny,
		RenewalHandler:     mishttp.NewRenewalHandler(mishttp.RenewalConfig{Service: svc, RateLimiter: lim}),
		JWTSVIDHandler:     stubAny,
		RevocationsHandler: stubAny,
	})

	// Well-formed path, no peer cert => the handler must return 401, not 501.
	enc := common.EncodeSPIFFEID("spiffe://margo.example.com/margo/device/x")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identities/"+enc+"/renewal", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Errorf("status = %d, want 401 (no peer cert)", rec.Code)
	}
}

// fakeServerIssuer is a no-op identity.Issuer used only to satisfy the
// service constructor; the test never reaches issuance.
type fakeServerIssuer struct{}

func (fakeServerIssuer) IssueX509SVID(context.Context, string, []byte, time.Duration) ([][]byte, time.Time, error) {
	return nil, time.Time{}, nil
}
