package revocations_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/device-agent/revocations"
)

func TestClient_Fetch_200(t *testing.T) {
	list := common.RevocationListDTO{
		LastUpdated: "2026-05-01T00:00:00Z",
		RevokedSVIDs: []common.RevokedSVIDEntry{{
			CertFingerprintSHA256: "aa",
			SerialNumber:          "BB",
			RevokedAt:             "2026-05-01T00:00:00Z",
		}},
	}
	raw, _ := json.Marshal(&list)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `W/"abc"`)
		w.Header().Set("Last-Modified", "Fri, 01 May 2026 00:00:00 GMT")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	res, err := revocations.Fetch(context.Background(), http.DefaultClient, srv.URL+"/api/v1/revocations", "", "")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if res.NotModified {
		t.Error("NotModified unexpectedly true")
	}
	if res.List.RevokedSVIDs[0].CertFingerprintSHA256 != "aa" {
		t.Errorf("parsed list = %+v", res.List)
	}
	if res.ETag != `W/"abc"` {
		t.Errorf("etag = %q", res.ETag)
	}
	if res.LastModified != "Fri, 01 May 2026 00:00:00 GMT" {
		t.Errorf("last-modified = %q", res.LastModified)
	}
	if !res.ContainsFingerprint("aa") {
		t.Error("ContainsFingerprint(aa) = false; want true")
	}
	if res.ContainsFingerprint("cc") {
		t.Error("ContainsFingerprint(cc) = true; want false")
	}
}

func TestClient_Fetch_304(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != `W/"abc"` {
			t.Errorf("expected If-None-Match passthrough, got %q", r.Header.Get("If-None-Match"))
		}
		w.Header().Set("ETag", `W/"abc"`)
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	res, err := revocations.Fetch(context.Background(), http.DefaultClient, srv.URL+"/api/v1/revocations", `W/"abc"`, "")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !res.NotModified {
		t.Error("NotModified = false")
	}
}

func TestClient_Fetch_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"type":"https://margo.org/docs/errors/revocation-format"}`))
	}))
	defer srv.Close()

	res, err := revocations.Fetch(context.Background(), http.DefaultClient, srv.URL+"/api/v1/revocations", "", "")
	if err != nil {
		t.Fatalf("expected no transport error, got: %v", err)
	}
	if res.HTTPStatus != 500 {
		t.Errorf("HTTPStatus = %d, want 500", res.HTTPStatus)
	}
}

// Unused: keeps bytes import in scope if test body grows.
var _ = bytes.NewReader
var _ = time.Now
