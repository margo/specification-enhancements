package factorycertmtls_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/device-agent/enroll/factorycertmtls"
	"github.com/margo/miaf-poc/pkg/device-agent/keygen"
)

// TestEnroll_SendsExpectedBody drives the client against an httptest server
// that inspects the request body shape. The PoC test does not exercise mTLS
// on the transport because httptest.NewTLSServer cannot request client
// certs without more plumbing than this layer owns. Full mTLS coverage lives
// in the e2e suite.
func TestEnroll_SendsExpectedBody(t *testing.T) {
	key, _ := keygen.Generate(keygen.ES256)

	gotBody := make(chan common.EnrollmentRequestDTO, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/identities" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		var req common.EnrollmentRequestDTO
		if err := json.Unmarshal(b, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		gotBody <- req

		resp := common.EnrollmentResponseDTO{
			SVIDProfileURI: common.SVIDProfileURIX509V1,
			SVID: common.X509SVIDDTO{
				CertificateChainPEM: []string{"-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n"},
			},
		}
		buf, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write(buf)
	}))
	t.Cleanup(ts.Close)

	out, err := factorycertmtls.Enroll(context.Background(), factorycertmtls.Request{
		Client:        ts.Client(),
		MISBaseURL:    ts.URL,
		DevicePrivKey: key,
	})
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if out.HTTPStatus != 201 {
		t.Errorf("HTTPStatus = %d", out.HTTPStatus)
	}
	if len(out.Response.SVID.CertificateChainPEM) != 1 {
		t.Errorf("chain len = %d", len(out.Response.SVID.CertificateChainPEM))
	}

	got := <-gotBody
	if got.SVIDProfileURI != common.SVIDProfileURIX509V1 {
		t.Errorf("profile = %q", got.SVIDProfileURI)
	}
	if got.BootstrapCredential.Method != common.BootstrapMethodFactoryCertMTLS {
		t.Errorf("method = %q", got.BootstrapCredential.Method)
	}
	if got.BootstrapCredential.Proof != nil {
		t.Errorf("proof must be omitted for factory-cert-mtls")
	}
	if got.SVIDRequest.CSR == "" {
		t.Error("csr missing")
	}
}

func TestEnroll_Non2xx_ReturnsInResult(t *testing.T) {
	key, _ := keygen.Generate(keygen.ES256)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"type":"about:blank","title":"Unauthorized","status":401}`))
	}))
	t.Cleanup(ts.Close)

	res, err := factorycertmtls.Enroll(context.Background(), factorycertmtls.Request{
		Client:        ts.Client(),
		MISBaseURL:    ts.URL,
		DevicePrivKey: key,
	})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("HTTPStatus = %d, want 401", res.HTTPStatus)
	}
	if res.Problem.Title != "Unauthorized" {
		t.Errorf("Problem.Title = %q", res.Problem.Title)
	}
	if res.RetryAfter != "60" {
		t.Errorf("RetryAfter = %q, want 60", res.RetryAfter)
	}
}

func TestEnroll_SendsReplacementAuthorizationWhenProvided(t *testing.T) {
	key, _ := keygen.Generate(keygen.ES256)

	gotBody := make(chan common.EnrollmentRequestDTO, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var req common.EnrollmentRequestDTO
		if err := json.Unmarshal(b, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		gotBody <- req

		resp := common.EnrollmentResponseDTO{
			SVIDProfileURI: common.SVIDProfileURIX509V1,
			SVID: common.X509SVIDDTO{
				CertificateChainPEM: []string{"-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n"},
			},
		}
		buf, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	}))
	t.Cleanup(ts.Close)

	_, err := factorycertmtls.Enroll(context.Background(), factorycertmtls.Request{
		Client:        ts.Client(),
		MISBaseURL:    ts.URL,
		DevicePrivKey: key,
		ReplacementAuthorization: &common.ReplacementAuthorizationDTO{
			Method: common.ReplacementAuthMethodOperatorTicket,
			Proof:  common.ReplacementAuthorizationProof{Ticket: "operator-ticket-123"},
		},
	})
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	got := <-gotBody
	if got.ReplacementAuthorization == nil {
		t.Fatal("ReplacementAuthorization = nil, want DTO")
	}
	if got.ReplacementAuthorization.Method != common.ReplacementAuthMethodOperatorTicket {
		t.Fatalf("Method = %q, want %q", got.ReplacementAuthorization.Method, common.ReplacementAuthMethodOperatorTicket)
	}
	if got.ReplacementAuthorization.Proof.Ticket != "operator-ticket-123" {
		t.Fatalf("Proof.Ticket = %q, want %q", got.ReplacementAuthorization.Proof.Ticket, "operator-ticket-123")
	}
	if got.BootstrapCredential.Proof != nil {
		t.Errorf("proof must be omitted for factory-cert-mtls")
	}
}
