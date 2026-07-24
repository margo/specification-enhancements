//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertmtls"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/ra/stepca"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

func TestEnrollMTLS_RAMode(t *testing.T) {
	const trustDomain = "margo.example.com"

	// 1. Local CA that the fake Step CA uses to sign leaves.
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		IsCA:                  true,
		BasicConstraintsValid: true,
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, _ := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	caCert, _ := x509.ParseCertificate(caDER)

	// 2. Fake Step CA honouring user.sans URI claim.
	fakeCA := startFakeStepCA(t, caCert, caKey)
	defer fakeCA.Close()

	// 3. Build a StepCAIssuer pointing at the fake.
	rootPool := x509.NewCertPool()
	rootPool.AddCert(fakeCA.Certificate())
	provKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	raClient, err := stepca.NewClient(stepca.ClientConfig{
		BaseURL:    fakeCA.URL,
		RootPool:   rootPool,
		Credential: stepca.NewCredentialForTest("margo-mis", "test-kid", provKey),
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	issuer, err := stepca.NewIssuer(stepca.IssuerConfig{
		TrustDomain: trustDomain,
		Client:      raClient,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 4. Wire in-process MIS with the RA issuer.
	s := openStore(t)
	repo := s.Identities()

	policy := identity.StaticPolicy{NewLDI: true, KeyRotation: true}
	svc := identity.NewService(repo, issuer, policy, identity.NoReplacements{}, s.IssuedSVIDs(), s.RevokedSVIDs(), trustDomain, 24*time.Hour)

	factoryRootPEMPath, deviceLeafCert := mintFactoryCA(t)
	factoryValidator, err := factorycertmtls.NewFromPEMFile(factoryRootPEMPath)
	if err != nil {
		t.Fatalf("factory validator: %v", err)
	}
	registry := bootstrap.NewRegistry()
	registry.Register(common.BootstrapMethodFactoryCertMTLS, factoryValidator)

	handler := mishttp.NewEnrollmentHandler(mishttp.EnrollmentConfig{
		Registry: registry,
		Service:  svc,
	})

	// 5. Generate device CSR.
	deviceKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, deviceKey)
	csrBase64 := base64.StdEncoding.EncodeToString(csrDER)

	reqBody, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: csrBase64},
		BootstrapCredential: common.BootstrapCredentialDTO{
			Method: common.BootstrapMethodFactoryCertMTLS,
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.Background())
	req.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{func() *x509.Certificate {
			c, _ := x509.ParseCertificate(deviceLeafCert.Certificate[0])
			return c
		}()},
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rr.Code, rr.Body.String())
	}

	var resp common.EnrollmentResponseDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	chain := resp.SVID.CertificateChainPEM
	if len(chain) != 2 {
		t.Fatalf("certificate_chain_pem length = %d, want 2", len(chain))
	}

	// Parse leaf and verify URI SAN was set from OTT user.sans (not CSR).
	block, _ := pem.Decode([]byte(chain[0]))
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}

	if len(leaf.URIs) != 1 {
		t.Fatalf("leaf URI SANs = %v, want exactly 1", leaf.URIs)
	}
	if leaf.URIs[0].Host != trustDomain {
		t.Errorf("leaf SPIFFE trust domain = %q, want %q", leaf.URIs[0].Host, trustDomain)
	}
	t.Logf("RA mode enrolled: %s (issuer: %s)", leaf.URIs[0], leaf.Issuer.CommonName)

	// Verify chain: leaf must verify against the fake CA root.
	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: roots}); err != nil {
		t.Errorf("chain verify: %v", err)
	}
}
