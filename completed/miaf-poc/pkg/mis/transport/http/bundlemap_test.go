package http_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/bundlemap"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

// fakeBundleSource implements mishttp.BundleSource for tests.
type fakeBundleSource struct {
	set       bundlemap.TrustAnchorSet
	refreshed time.Time
	returnErr error
	callCount int
}

func (f *fakeBundleSource) GetTrustAnchors(ctx context.Context) (bundlemap.TrustAnchorSet, time.Time, error) {
	f.callCount++
	if f.returnErr != nil {
		return bundlemap.TrustAnchorSet{}, time.Time{}, f.returnErr
	}
	return f.set, f.refreshed, nil
}

// newTestCert fabricates a self-signed EC P-256 cert.
func newTestCert(t *testing.T) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	return cert
}

func TestBundleMapHandler_Success(t *testing.T) {
	src := &fakeBundleSource{
		set: bundlemap.TrustAnchorSet{
			TrustDomain:     "margo.example.com",
			X509Authorities: []*x509.Certificate{newTestCert(t)},
		},
		refreshed: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}

	h := mishttp.NewBundleMapHandler(src, 60*time.Second)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/.well-known/spiffe/bundle.json")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if got := resp.Header.Get("Last-Modified"); got == "" {
		t.Error("Last-Modified header missing")
	}

	body, _ := io.ReadAll(resp.Body)

	// Assert top-level envelope has "trust_domains" key (SPIFFE §5.1.1).
	var env map[string]json.RawMessage
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	tdRaw, ok := env["trust_domains"]
	if !ok {
		t.Fatalf("envelope missing trust_domains key; got keys = %v", mapKeys(env))
	}

	// Assert trust_domains["margo.example.com"] exists.
	var tdMap map[string]json.RawMessage
	if err := json.Unmarshal(tdRaw, &tdMap); err != nil {
		t.Fatalf("unmarshal trust_domains: %v", err)
	}
	if _, ok := tdMap["margo.example.com"]; !ok {
		t.Errorf("trust_domains missing entry for margo.example.com; got keys = %v", mapKeys(tdMap))
	}
}

func TestBundleMapHandler_IfModifiedSince(t *testing.T) {
	src := &fakeBundleSource{
		set:       bundlemap.TrustAnchorSet{TrustDomain: "margo.example.com"},
		refreshed: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	h := mishttp.NewBundleMapHandler(src, 60*time.Second)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/.well-known/spiffe/bundle.json", nil)
	req.Header.Set("If-Modified-Since", src.refreshed.Format(http.TimeFormat))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotModified {
		t.Errorf("status = %d, want 304", resp.StatusCode)
	}
}

func TestBundleMapHandler_SourceError(t *testing.T) {
	src := &fakeBundleSource{returnErr: errors.New("boom")}
	h := mishttp.NewBundleMapHandler(src, 60*time.Second)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/.well-known/spiffe/bundle.json")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}
}

func TestBundleMapHandler_EmitsETag(t *testing.T) {
	// Use a set with an empty X509Authorities list - simplest valid body.
	src := &fakeBundleSource{
		set:       bundlemap.TrustAnchorSet{TrustDomain: "margo.example.com"},
		refreshed: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	h := mishttp.NewBundleMapHandler(src, 60*time.Second)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/.well-known/spiffe/bundle.json")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("ETag header missing")
	}
	if !strings.HasPrefix(etag, `W/"`) || !strings.HasSuffix(etag, `"`) {
		t.Errorf("ETag = %q, want weak-validator form W/\"...\"", etag)
	}
}

func TestBundleMapHandler_IfNoneMatch(t *testing.T) {
	src := &fakeBundleSource{
		set:       bundlemap.TrustAnchorSet{TrustDomain: "margo.example.com"},
		refreshed: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	h := mishttp.NewBundleMapHandler(src, 60*time.Second)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	// First request to learn the current ETag.
	resp1, err := http.Get(ts.URL + "/.well-known/spiffe/bundle.json")
	if err != nil {
		t.Fatalf("first get: %v", err)
	}
	etag := resp1.Header.Get("ETag")
	resp1.Body.Close()
	if etag == "" {
		t.Fatal("no ETag on first response")
	}

	// Second request with If-None-Match => expect 304.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/.well-known/spiffe/bundle.json", nil)
	req.Header.Set("If-None-Match", etag)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("second get: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotModified {
		t.Errorf("status = %d, want 304", resp2.StatusCode)
	}
	if got := resp2.Header.Get("ETag"); got != etag {
		t.Errorf("304 response ETag = %q, want %q", got, etag)
	}
}

func TestBundleMapHandler_IfNoneMatchWildcard(t *testing.T) {
	src := &fakeBundleSource{
		set:       bundlemap.TrustAnchorSet{TrustDomain: "margo.example.com"},
		refreshed: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	h := mishttp.NewBundleMapHandler(src, 60*time.Second)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/.well-known/spiffe/bundle.json", nil)
	req.Header.Set("If-None-Match", "*")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotModified {
		t.Errorf("status = %d, want 304", resp.StatusCode)
	}
}

func TestBundleMapHandler_CacheControlReflectsTTL(t *testing.T) {
	src := &fakeBundleSource{
		set:       bundlemap.TrustAnchorSet{TrustDomain: "margo.example.com"},
		refreshed: time.Now(),
	}
	h := mishttp.NewBundleMapHandler(src, 5*time.Minute)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/.well-known/spiffe/bundle.json")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if got, want := resp.Header.Get("Cache-Control"), "max-age=300"; got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
}

func mapKeys[K comparable, V any](m map[K]V) []K {
	out := make([]K, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
