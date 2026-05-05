package renew_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/device-agent/renew"
)

func TestRenew_HappyPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/renewals") {
			t.Errorf("path = %q; expected /renewals suffix", r.URL.Path)
		}
		var req common.EnrollmentRequestDTO
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.SVIDProfileURI != common.SVIDProfileURIX509V1 {
			t.Errorf("svid_profile_uri = %q", req.SVIDProfileURI)
		}
		if req.SVIDRequest.CSR == "" {
			t.Error("missing CSR")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(common.EnrollmentResponseDTO{
			SVIDProfileURI: common.SVIDProfileURIX509V1,
			SVID: common.X509SVIDDTO{
				CertificateChainPEM: []string{"-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"},
			},
		})
	}))
	defer ts.Close()

	devKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	res, err := renew.Renew(context.Background(), renew.Request{
		Client:        ts.Client(),
		MISBaseURL:    ts.URL,
		SPIFFEID:      "spiffe://margo.example.com/margo/device/abc",
		DevicePrivKey: devKey,
	})
	if err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if res.HTTPStatus != 200 {
		t.Errorf("status = %d, want 200", res.HTTPStatus)
	}
	if len(res.Response.SVID.CertificateChainPEM) == 0 {
		t.Error("empty chain")
	}
}

func TestRenew_PropagatesHTTPErrorBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(409)
		_, _ = io.WriteString(w, `{"type":"https://margo.org/docs/errors/key-rotation-not-permitted","title":"Key Rotation Not Permitted","status":409}`)
	}))
	defer ts.Close()

	devKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	res, err := renew.Renew(context.Background(), renew.Request{
		Client:        ts.Client(),
		MISBaseURL:    ts.URL,
		SPIFFEID:      "spiffe://margo.example.com/margo/device/abc",
		DevicePrivKey: devKey,
	})
	if err != nil {
		t.Fatalf("expected no transport error, got: %v", err)
	}
	if res.HTTPStatus != 409 {
		t.Errorf("HTTPStatus = %d, want 409", res.HTTPStatus)
	}
	if res.Problem.Type == "" {
		t.Error("Problem.Type should be populated on 409")
	}
}

func TestRenew_Non2xxIsReported(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(429)
		_, _ = io.WriteString(w, `{"type":"about:blank","title":"Too Many Requests","status":429}`)
	}))
	defer ts.Close()

	devKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	res, err := renew.Renew(context.Background(), renew.Request{
		Client:        ts.Client(),
		MISBaseURL:    ts.URL,
		SPIFFEID:      "spiffe://td/margo/device/x",
		DevicePrivKey: devKey,
	})
	if err != nil {
		t.Fatalf("Renew err: %v", err)
	}
	if res.HTTPStatus != 429 {
		t.Errorf("HTTPStatus = %d, want 429", res.HTTPStatus)
	}
	if res.RetryAfter != "30" {
		t.Errorf("RetryAfter = %q, want 30", res.RetryAfter)
	}
	if res.Problem.Status != 429 {
		t.Errorf("Problem.Status = %d, want 429", res.Problem.Status)
	}
}

func TestRenew_BearerAuthSetsAuthorizationHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer issued-jwt" {
			t.Errorf("Authorization = %q, want Bearer issued-jwt", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(common.EnrollmentResponseDTO{
			SVIDProfileURI: common.SVIDProfileURIX509V1,
			SVID: common.X509SVIDDTO{
				CertificateChainPEM: []string{"-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"},
			},
		})
	}))
	defer ts.Close()

	devKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	res, err := renew.Renew(context.Background(), renew.Request{
		Client:        ts.Client(),
		MISBaseURL:    ts.URL,
		SPIFFEID:      "spiffe://td/margo/device/x",
		DevicePrivKey: devKey,
		BearerToken:   "issued-jwt",
	})
	if err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if res.HTTPStatus != 200 {
		t.Errorf("status = %d, want 200", res.HTTPStatus)
	}
}
