package enrollmenttoken_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/margo/miaf-poc/pkg/common"
	enrollmenttoken "github.com/margo/miaf-poc/pkg/device-agent/enroll/enrollmenttoken"
)

func TestEnroll_SendsTokenInProof(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req common.EnrollmentRequestDTO
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("parse body: %v", err)
		}
		if req.BootstrapCredential.Method != common.BootstrapMethodEnrollmentToken {
			t.Errorf("method = %q", req.BootstrapCredential.Method)
		}
		if req.BootstrapCredential.Proof == nil || req.BootstrapCredential.Proof.Token != "my-token" {
			t.Errorf("proof.token = %+v", req.BootstrapCredential.Proof)
		}
		if req.SVIDProfileURI != common.SVIDProfileURIX509V1 {
			t.Errorf("svid_profile_uri = %q", req.SVIDProfileURI)
		}
		resp := common.EnrollmentResponseDTO{
			SVIDProfileURI: common.SVIDProfileURIX509V1,
			SVID:           common.X509SVIDDTO{CertificateChainPEM: []string{"-----BEGIN CERTIFICATE-----\nAA==\n-----END CERTIFICATE-----\n"}},
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	res, err := enrollmenttoken.Enroll(context.Background(), enrollmenttoken.Request{
		Client:        srv.Client(),
		MISBaseURL:    srv.URL,
		Token:         "my-token",
		DevicePrivKey: k,
	})
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if res.HTTPStatus != http.StatusCreated {
		t.Errorf("status = %d", res.HTTPStatus)
	}
	if len(res.Response.SVID.CertificateChainPEM) == 0 {
		t.Error("chain empty")
	}
}

func TestEnroll_Propagates4xxInResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"https://margo.org/docs/errors/invalid-enrollment-token","title":"Invalid Enrollment Token","status":401}`))
	}))
	defer srv.Close()

	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	res, err := enrollmenttoken.Enroll(context.Background(), enrollmenttoken.Request{
		Client:        srv.Client(),
		MISBaseURL:    srv.URL,
		Token:         "bad",
		DevicePrivKey: k,
	})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("HTTPStatus = %d, want 401", res.HTTPStatus)
	}
	if res.Problem.Title != "Invalid Enrollment Token" {
		t.Errorf("Problem.Title = %q", res.Problem.Title)
	}
}
