package discovery_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/margo/miaf-poc/pkg/device-agent/discovery"
)

func TestFetch_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/margo" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"trust_domain": "t",
			"trust_bundle_uri": "https://x/bundle",
			"margo_identity_service_base_uri": "https://x",
			"supported_bootstrap_methods": ["urn:margo:bootstrap:factory-cert-mtls:v1"],
			"svid_profiles_supported": [{"type":"x509-svid","versions":["u"]}],
			"recommended_svid_profile_uri": "u"
		}`))
	}))
	t.Cleanup(ts.Close)

	got, err := discovery.Fetch(context.Background(), ts.Client(), ts.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got.TrustDomain != "t" {
		t.Errorf("trust_domain = %q", got.TrustDomain)
	}
}

func TestFetch_Non200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"type":"about:blank","title":"Service Unavailable","status":503}`))
	}))
	t.Cleanup(ts.Close)

	_, err := discovery.Fetch(context.Background(), ts.Client(), ts.URL)
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("expected 503-bearing error, got %v", err)
	}
}
