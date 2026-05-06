package jwtsvid_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/device-agent/jwtsvid"
)

func TestExchange_ModeMTLS(t *testing.T) {
	wantJWT := "eyJ.fake"
	wantExpires := "2026-04-27T00:00:00Z"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		var req common.JWTSVIDExchangeRequestDTO
		if err := json.Unmarshal(b, &req); err != nil {
			t.Errorf("unmarshal request: %v", err)
		}
		if req.ClientAssertion != "" {
			t.Error("ModeMTLS must not include client_assertion")
		}
		resp := common.JWTSVIDExchangeResponseDTO{JWT: wantJWT, ExpiresAt: wantExpires}
		buf, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(buf)
	}))
	t.Cleanup(ts.Close)

	result, err := jwtsvid.Exchange(context.Background(), jwtsvid.Request{
		Client:     ts.Client(),
		MISBaseURL: ts.URL,
		SPIFFEID:   "spiffe://margo.example.com/device/d1",
		Audiences:  []string{"https://dfm.example/"},
		Mode:       jwtsvid.ModeMTLS,
	})
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if result.HTTPStatus != http.StatusCreated {
		t.Errorf("status = %d, want 201", result.HTTPStatus)
	}
	if result.Response.JWT != wantJWT {
		t.Errorf("JWT = %q, want %q", result.Response.JWT, wantJWT)
	}
}

func TestExchange_ModeClientAssertion(t *testing.T) {
	wantJWT := "eyJ.fake"
	wantExpires := "2026-04-27T00:00:00Z"

	var gotBody common.JWTSVIDExchangeRequestDTO
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		resp := common.JWTSVIDExchangeResponseDTO{JWT: wantJWT, ExpiresAt: wantExpires}
		buf, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(buf)
	}))
	t.Cleanup(ts.Close)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	// Build a self-signed leaf so the Exchange path can construct the x5c
	// header. The handler in this test does not validate the chain.
	u, _ := url.Parse("spiffe://margo.example.com/device/d1")
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(99),
		Subject:      pkix.Name{},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		URIs:         []*url.URL{u},
	}
	leafDER, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	leaf, _ := x509.ParseCertificate(leafDER)

	result, err := jwtsvid.Exchange(context.Background(), jwtsvid.Request{
		Client:     ts.Client(),
		MISBaseURL: ts.URL,
		SPIFFEID:   "spiffe://margo.example.com/device/d1",
		Audiences:  []string{"https://dfm.example/"},
		Mode:       jwtsvid.ModeClientAssertion,
		SigningKey: key,
		SVIDChain:  []*x509.Certificate{leaf},
	})
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if result.HTTPStatus != http.StatusCreated {
		t.Errorf("status = %d, want 201", result.HTTPStatus)
	}
	if result.Response.JWT != wantJWT {
		t.Errorf("JWT = %q, want %q", result.Response.JWT, wantJWT)
	}
	if gotBody.ClientAssertion == "" {
		t.Error("ModeClientAssertion must include client_assertion in request body")
	}
	// Verify the assertion is a JWT (three dot-separated segments)
	parts := strings.Split(gotBody.ClientAssertion, ".")
	if len(parts) != 3 {
		t.Errorf("client_assertion has %d parts, want 3", len(parts))
	}
}
