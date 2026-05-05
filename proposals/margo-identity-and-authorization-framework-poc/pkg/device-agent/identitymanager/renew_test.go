package identitymanager_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/device-agent/identitymanager"
	"github.com/margo/miaf-poc/pkg/device-agent/agentstate"
)

// mintLeafChain builds a minimal leaf cert signed by a fresh CA.
// Returns (chainPEM string, CA cert DER for trust anchor).
func mintLeafChain(t *testing.T) (chainPEM string, caDER []byte) {
	t.Helper()
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caDER, _ = x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	caCert, _ := x509.ParseCertificate(caDER)

	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	spiffeURI, _ := url.Parse("spiffe://td.example/margo/device/renewed")
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(99),
		Subject:      pkix.Name{CommonName: "device-leaf"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(24 * time.Hour),
		URIs:         []*url.URL{spiffeURI},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	leafDER, _ := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)

	chainPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}))
	return
}

func TestHandleRenew_HappyPath(t *testing.T) {
	t.Parallel()
	chainPEM, _ := mintLeafChain(t)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := common.EnrollmentResponseDTO{
			SVIDProfileURI: common.SVIDProfileURIX509V1,
			SVID: common.X509SVIDDTO{
				CertificateChainPEM: []string{chainPEM},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	certPath, keyPath, _, _ := mintDeviceCert(t)
	caPath := tlsServerCA(t, srv)
	stateFile := seedState(t, "spiffe://td.example/margo/device/test")

	mgr := identitymanager.NewManager(identitymanager.Config{
		Clock:          clockReal{},
		StateFile:      stateFile,
		SVIDCertFile:   certPath,
		SVIDKeyFile:    keyPath,
		MISBaseURL:     srv.URL,
		MISTrustAnchor: caPath,
		BundleMapURL:   srv.URL + "/bundle",
		RevocationsURL: srv.URL + "/revocations",
		Logger:         discardLogger(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	reply := make(chan identitymanager.RenewReply, 1)
	mgr.Submit(identitymanager.Cmd{
		Ctx:   ctx,
		Renew: &identitymanager.RenewCmd{Reply: reply},
	})
	r := <-reply
	if r.Err != nil {
		t.Fatalf("Renew: %v", r.Err)
	}
	if r.NewSerial == "" {
		t.Error("NewSerial should be set after renewal")
	}
	// Verify state persisted
	st, _ := agentstate.Load(stateFile)
	if st.Serial == "" {
		t.Error("state.Serial not updated")
	}
	if st.LastRenewalAt.IsZero() {
		t.Error("state.LastRenewalAt not set")
	}
	rawState, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("ReadFile(state): %v", err)
	}
	if strings.Contains(string(rawState), "private_key_pem") {
		t.Fatalf("state.json should not persist private_key_pem:\n%s", rawState)
	}
}

func TestHandleRenew_429ReturnsNoTransportError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "about:blank", "title": "Too Many Requests", "status": 429,
		})
	}))
	defer srv.Close()

	certPath, keyPath, _, _ := mintDeviceCert(t)
	caPath := tlsServerCA(t, srv)
	stateFile := seedState(t, "spiffe://td.example/margo/device/x")

	mgr := identitymanager.NewManager(identitymanager.Config{
		Clock:          clockReal{},
		StateFile:      stateFile,
		SVIDCertFile:   certPath,
		SVIDKeyFile:    keyPath,
		MISBaseURL:     srv.URL,
		MISTrustAnchor: caPath,
		Logger:         discardLogger(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	reply := make(chan identitymanager.RenewReply, 1)
	mgr.Submit(identitymanager.Cmd{
		Ctx:   ctx,
		Renew: &identitymanager.RenewCmd{Reply: reply},
	})
	r := <-reply
	if r.HTTPStatus != 429 {
		t.Errorf("HTTPStatus = %d, want 429", r.HTTPStatus)
	}
	// Error should be set (non-2xx) but NOT a transport error
	if r.Err == nil {
		t.Error("Err should be non-nil for 429")
	}
}

func TestHandleRenew_BearerAuthPath(t *testing.T) {
	t.Parallel()
	chainPEM, _ := mintLeafChain(t)

	var sawJWTExchange bool
	var sawBearerRenew bool
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/jwt-svid"):
			sawJWTExchange = true
			if len(r.TLS.PeerCertificates) != 0 {
				t.Errorf("jwt-svid exchange should not use mTLS in bearer renewal mode")
			}
			var req common.JWTSVIDExchangeRequestDTO
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode jwt exchange request: %v", err)
			}
			if req.ClientAssertion == "" {
				t.Error("client_assertion missing from jwt exchange request")
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(common.JWTSVIDExchangeResponseDTO{
				JWT:       "issued-jwt",
				ExpiresAt: time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339),
			})
		case strings.HasSuffix(r.URL.Path, "/renewals"):
			sawBearerRenew = true
			if got := r.Header.Get("Authorization"); got != "Bearer issued-jwt" {
				t.Errorf("Authorization = %q, want Bearer issued-jwt", got)
			}
			if len(r.TLS.PeerCertificates) != 0 {
				t.Errorf("renewal request should not use mTLS in bearer mode")
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(common.EnrollmentResponseDTO{
				SVIDProfileURI: common.SVIDProfileURIX509V1,
				SVID: common.X509SVIDDTO{
					CertificateChainPEM: []string{chainPEM},
				},
			})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	certPath, keyPath, _, _ := mintDeviceCert(t)
	caPath := tlsServerCA(t, srv)
	stateFile := seedState(t, "spiffe://td.example/margo/device/test")

	mgr := identitymanager.NewManager(identitymanager.Config{
		Clock:          clockReal{},
		StateFile:      stateFile,
		SVIDCertFile:   certPath,
		SVIDKeyFile:    keyPath,
		MISBaseURL:     srv.URL,
		MISTrustAnchor: caPath,
		RenewAuth:      "bearer",
		BundleMapURL:   srv.URL + "/bundle",
		RevocationsURL: srv.URL + "/revocations",
		Logger:         discardLogger(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	reply := make(chan identitymanager.RenewReply, 1)
	mgr.Submit(identitymanager.Cmd{
		Ctx:   ctx,
		Renew: &identitymanager.RenewCmd{Reply: reply},
	})
	r := <-reply
	if r.Err != nil {
		t.Fatalf("Renew: %v", r.Err)
	}
	if !sawJWTExchange {
		t.Error("expected bearer renewal flow to exchange a JWT first")
	}
	if !sawBearerRenew {
		t.Error("expected bearer renewal flow to call the renewal endpoint")
	}
}
