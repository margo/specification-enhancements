package factorycertjwt_test

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
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/device-agent/enroll/factorycertjwt"
	"github.com/margo/miaf-poc/pkg/device-agent/keygen"
)

func TestEnroll_SendsAssertionInProof(t *testing.T) {
	factoryCert, factoryKey := buildFactoryCert(t)
	deviceKey, _ := keygen.Generate(keygen.ES256)

	var got common.EnrollmentRequestDTO
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/identities" {
			t.Errorf("path = %q", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
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

	out, err := factorycertjwt.Enroll(context.Background(), factorycertjwt.Request{
		Client:        ts.Client(),
		MISBaseURL:    ts.URL,
		FactoryCerts:  []*x509.Certificate{factoryCert},
		FactoryKey:    factoryKey,
		DevicePrivKey: deviceKey,
	})
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if out.HTTPStatus != http.StatusCreated {
		t.Errorf("status = %d", out.HTTPStatus)
	}
	if got.BootstrapCredential.Method != common.BootstrapMethodFactoryCertJWT {
		t.Errorf("method = %q", got.BootstrapCredential.Method)
	}
	if got.BootstrapCredential.Proof == nil || got.BootstrapCredential.Proof.Assertion == "" {
		t.Error("proof.assertion missing")
	}
	if got.SVIDRequest.CSR == "" {
		t.Error("csr missing")
	}
}

func buildFactoryCert(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "factory-dev"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	c, _ := x509.ParseCertificate(der)
	return c, k
}
