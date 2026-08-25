package stepca_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/ra/stepca"
)

func TestClient_Sign_SendsOTTAndParsesChain(t *testing.T) {
	// Generate a CSR the client will send.
	devKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, devKey)

	// Generate a stand-in CA that the stubbed server will use to sign the leaf.
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTmpl := &x509.Certificate{SerialNumber: bigOne(), IsCA: true, BasicConstraintsValid: true}
	caDER, _ := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	caCert, _ := x509.ParseCertificate(caDER)

	var receivedOTT string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/1.0/sign" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "bad", 400)
			return
		}
		var body struct{ OTT, CSR string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		receivedOTT = body.OTT

		// Build a leaf cert that mirrors the CSR's key.
		leafTmpl := &x509.Certificate{
			SerialNumber: bigFromInt(2),
			NotBefore:    time.Now().Add(-time.Minute),
			NotAfter:     time.Now().Add(24 * time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
		}
		csrPEM, _ := pem.Decode([]byte(body.CSR))
		if csrPEM == nil {
			http.Error(w, "no csr PEM", 400)
			return
		}
		csr, _ := x509.ParseCertificateRequest(csrPEM.Bytes)
		leafDER, _ := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, csr.PublicKey, caKey)
		resp := map[string]any{
			"crt": toPEM("CERTIFICATE", leafDER),
			"ca":  toPEM("CERTIFICATE", caDER),
			"certChain": []string{
				toPEM("CERTIFICATE", leafDER),
				toPEM("CERTIFICATE", caDER),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cred := freshTestCredential(t)

	rootPool := x509.NewCertPool()
	rootPool.AddCert(srv.Certificate())
	c := mustNewClient(t, stepca.ClientConfig{
		BaseURL:     srv.URL,
		RootPool:    rootPool,
		Credential:  cred,
		HTTPTimeout: 5 * time.Second,
	})

	chain, notAfter, err := c.Sign(context.Background(), stepca.SignRequest{
		SpiffeID: "spiffe://margo.example.com/margo/device/abc",
		CSRDER:   csrDER,
		TTL:      10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if len(chain) != 2 {
		t.Fatalf("chain length = %d, want 2", len(chain))
	}
	if notAfter.Before(time.Now()) {
		t.Errorf("notAfter in the past: %v", notAfter)
	}

	// OTT sanity: 3-part JWT
	parts := strings.Split(receivedOTT, ".")
	if len(parts) != 3 {
		t.Errorf("OTT not a 3-segment JWT: %q", receivedOTT)
	} else {
		// Verify the security invariant: the top-level "sans" claim carries the
		// authoritative URI SAN, not anything from the CSR. step-ca's JWK
		// provisioner converts this via CreateSANs (auto-detecting the spiffe://
		// URI scheme) and the X.509 template reads .SANs. This is what makes
		// RA mode correct per SUP §5.
		payloadBytes, _ := base64.RawURLEncoding.DecodeString(parts[1])
		var payload map[string]any
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			t.Fatalf("OTT payload unmarshal: %v", err)
		}
		sans, ok := payload["sans"].([]any)
		if !ok || len(sans) == 0 {
			t.Fatal("OTT top-level sans missing or empty")
		}
		if sans[0] != "spiffe://margo.example.com/margo/device/abc" {
			t.Errorf("OTT sans[0] = %v, want SPIFFE ID", sans[0])
		}
	}
}

func TestClient_Sign_RejectsNon200(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad"}`, 422)
	}))
	defer srv.Close()

	rootPool := x509.NewCertPool()
	rootPool.AddCert(srv.Certificate())
	c := mustNewClient(t, stepca.ClientConfig{
		BaseURL:    srv.URL,
		RootPool:   rootPool,
		Credential: freshTestCredential(t),
	})
	devKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, devKey)
	_, _, err := c.Sign(context.Background(), stepca.SignRequest{
		SpiffeID: "spiffe://margo.example.com/margo/device/x",
		CSRDER:   csrDER,
		TTL:      time.Hour,
	})
	if err == nil {
		t.Fatal("expected error on 422")
	}
}

// freshTestCredential builds a *stepca.Credential in-process without touching
// provisioner.json, so client tests don't depend on credential.go.
func freshTestCredential(t *testing.T) *stepca.Credential {
	t.Helper()
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	return stepca.NewCredentialForTest("margo-mis", "test-kid", priv)
}

func toPEM(typ string, der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}))
}

func bigOne() *big.Int            { return big.NewInt(1) }
func bigFromInt(i int64) *big.Int { return big.NewInt(i) }

func mustNewClient(t *testing.T, cfg stepca.ClientConfig) *stepca.Client {
	t.Helper()
	c, err := stepca.NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}
