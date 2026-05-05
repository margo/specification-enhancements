package bundle_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/margo/miaf-poc/pkg/device-agent/bundle"
)

func TestFetch_200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Last-Modified", "Mon, 02 Jan 2026 03:04:05 GMT")
		_, _ = w.Write([]byte(`{"trust_domains":{"margo.example.com":{"keys":[]}}}`))
	}))
	t.Cleanup(ts.Close)

	res, err := bundle.Fetch(context.Background(), ts.Client(), ts.URL+"/.well-known/spiffe/bundle.json", "")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if res.NotModified {
		t.Error("unexpected NotModified")
	}
	if res.LastModified != "Mon, 02 Jan 2026 03:04:05 GMT" {
		t.Errorf("last modified = %q", res.LastModified)
	}
	if len(res.Body) == 0 {
		t.Error("empty body")
	}
}

func TestFetch_304(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-Modified-Since") == "" {
			t.Error("If-Modified-Since header missing")
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(ts.Close)

	res, err := bundle.Fetch(context.Background(), ts.Client(), ts.URL+"/x", "Mon, 02 Jan 2026 03:04:05 GMT")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !res.NotModified {
		t.Error("expected NotModified = true")
	}
}
