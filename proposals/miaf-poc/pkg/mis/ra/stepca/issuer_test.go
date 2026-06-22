package stepca_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/ra/stepca"
)

func startFakeStepCA(t *testing.T, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/1.0/sign" {
			http.NotFound(w, r)
			return
		}
		var body struct{ OTT, CSR string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		csrBlock, _ := pem.Decode([]byte(body.CSR))
		csr, _ := x509.ParseCertificateRequest(csrBlock.Bytes)
		leafTmpl := &x509.Certificate{
			SerialNumber: bigFromInt(42),
			NotBefore:    time.Now().Add(-time.Minute),
			NotAfter:     time.Now().Add(time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
			URIs: []*url.URL{mustParseURL(t, "spiffe://margo.example.com/margo/device/abc")},
		}
		leafDER, _ := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, csr.PublicKey, caKey)
		resp := map[string]any{
			"certChain": []string{toPEM("CERTIFICATE", leafDER), toPEM("CERTIFICATE", caCert.Raw)},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func mustParseURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestStepCAIssuer_HappyPath(t *testing.T) {
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTmpl := &x509.Certificate{SerialNumber: bigOne(), IsCA: true, BasicConstraintsValid: true}
	caDER, _ := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	caCert, _ := x509.ParseCertificate(caDER)

	srv := startFakeStepCA(t, caCert, caKey)
	defer srv.Close()

	rootPool := x509.NewCertPool()
	rootPool.AddCert(srv.Certificate())
	issuer, err := stepca.NewIssuer(stepca.IssuerConfig{
		TrustDomain: "margo.example.com",
		Client: mustNewClient(t, stepca.ClientConfig{
			BaseURL:    srv.URL,
			RootPool:   rootPool,
			Credential: freshTestCredential(t),
		}),
	})
	if err != nil {
		t.Fatal(err)
	}

	devKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, devKey)

	chain, notAfter, err := issuer.IssueX509SVID(context.Background(),
		"spiffe://margo.example.com/margo/device/abc", csrDER, time.Hour)
	if err != nil {
		t.Fatalf("IssueX509SVID: %v", err)
	}
	if len(chain) != 2 {
		t.Fatalf("chain length = %d, want 2", len(chain))
	}
	if notAfter.Before(time.Now()) {
		t.Errorf("notAfter in the past")
	}

	// Assert the issued leaf carries the authoritative URI SAN (per SUP §5).
	// The MIS-controlled OTT user.sans claim must drive the URI SAN, not the CSR.
	leaf, err := x509.ParseCertificate(chain[0])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	wantURI := "spiffe://margo.example.com/margo/device/abc"
	if len(leaf.URIs) != 1 || leaf.URIs[0].String() != wantURI {
		t.Errorf("leaf URIs = %v, want [%s]", leaf.URIs, wantURI)
	}
}

func TestStepCAIssuer_RejectsOutsideTrustDomain(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not have been called")
	}))
	defer srv.Close()
	rootPool := x509.NewCertPool()
	rootPool.AddCert(srv.Certificate())
	issuer, _ := stepca.NewIssuer(stepca.IssuerConfig{
		TrustDomain: "margo.example.com",
		Client:      mustNewClient(t, stepca.ClientConfig{BaseURL: srv.URL, RootPool: rootPool, Credential: freshTestCredential(t)}),
	})
	devKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, devKey)
	_, _, err := issuer.IssueX509SVID(context.Background(),
		"spiffe://other.example/margo/device/abc", csrDER, time.Hour)
	if err == nil {
		t.Fatal("expected error for SPIFFE ID outside trust domain")
	}
}

func TestStepCAIssuer_RejectsUnsupportedKey(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not have been called")
	}))
	defer srv.Close()
	rootPool := x509.NewCertPool()
	rootPool.AddCert(srv.Certificate())
	issuer, _ := stepca.NewIssuer(stepca.IssuerConfig{
		TrustDomain: "margo.example.com",
		Client:      mustNewClient(t, stepca.ClientConfig{BaseURL: srv.URL, RootPool: rootPool, Credential: freshTestCredential(t)}),
	})
	// P-384 is not in the supported set.
	badKey, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA384,
		Subject:            pkix.Name{CommonName: "bad"},
	}, badKey)
	_, _, err := issuer.IssueX509SVID(context.Background(),
		"spiffe://margo.example.com/margo/device/abc", csrDER, time.Hour)
	if err == nil {
		t.Fatal("expected error for P-384 key")
	}
}
