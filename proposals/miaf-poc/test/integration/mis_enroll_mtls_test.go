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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertmtls"
	"github.com/margo/miaf-poc/pkg/mis/ca"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

func TestEnrollMTLS_Integration(t *testing.T) {
	const trustDomain = "margo.example.com"

	// Build intermediate CA issuer
	intCertPath, intKeyPath, rootCert := mintTestIntermediate(t, trustDomain)
	issuer, err := ca.NewIntermediateIssuer(ca.Config{
		TrustDomain: trustDomain,
		CertFile:    intCertPath,
		KeyFile:     intKeyPath,
	})
	if err != nil {
		t.Fatalf("NewIntermediateIssuer: %v", err)
	}

	// Open unified MIS store
	s := openStore(t)
	repo := s.Identities()

	policy := identity.StaticPolicy{NewLDI: true, KeyRotation: true}
	svc := identity.NewService(repo, issuer, policy, identity.NoReplacements{}, s.IssuedSVIDs(), s.RevokedSVIDs(), trustDomain, 24*time.Hour)

	// Build factory cert-mTLS bootstrap validator
	factoryRootPEMPath, deviceLeafCert := mintFactoryCA(t)
	factoryValidator, err := factorycertmtls.NewFromPEMFile(factoryRootPEMPath)
	if err != nil {
		t.Fatalf("factory validator: %v", err)
	}
	registry := bootstrap.NewRegistry()
	registry.Register(common.BootstrapMethodFactoryCertMTLS, factoryValidator)

	// Wire enrollment handler
	handler := mishttp.NewEnrollmentHandler(mishttp.EnrollmentConfig{
		Registry: registry,
		Service:  svc,
	})

	// Device CSR - must be ECDSA P-256 with ECDSAWithSHA256 (MIAF Cryptographic Requirements)
	deviceKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("device key: %v", err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, deviceKey)
	if err != nil {
		t.Fatalf("create CSR: %v", err)
	}
	// The csr field is standard base64-encoded DER (no PEM armor), per SUP §5 and csr.Parse.
	csrBase64 := base64.StdEncoding.EncodeToString(csrDER)

	// Build enrollment request using the correct DTO field names
	reqBody, err := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: csrBase64},
		BootstrapCredential: common.BootstrapCredentialDTO{
			Method: common.BootstrapMethodFactoryCertMTLS,
		},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	// Attach factory leaf cert as TLS client cert (simulating mTLS)
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

	// Parse response using the actual EnrollmentResponseDTO shape:
	//   { "svidProfileUri": "...", "svid": { "certificateChainPem": ["..."] } }
	var resp common.EnrollmentResponseDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	chain := resp.SVID.CertificateChainPEM
	if len(chain) != 2 {
		t.Fatalf("certificateChainPem length = %d, want 2 (leaf + intermediate)", len(chain))
	}

	// Decode leaf cert from chain[0]
	block, _ := pem.Decode([]byte(chain[0]))
	if block == nil {
		t.Fatalf("no PEM block in chain[0]")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}

	// Assert SPIFFE ID structure: exactly one URI SAN in the correct trust domain
	if len(leaf.URIs) != 1 {
		t.Fatalf("leaf URI SANs = %v, want exactly 1", leaf.URIs)
	}
	spiffeID := leaf.URIs[0].String()
	if leaf.URIs[0].Host != trustDomain {
		t.Errorf("leaf SPIFFE ID trust domain = %q, want %q", leaf.URIs[0].Host, trustDomain)
	}
	t.Logf("enrolled: %s", spiffeID)

	// Build chain for verification: root is trusted, intermediate from chain[1]
	roots := x509.NewCertPool()
	roots.AddCert(rootCert)
	intermediates := x509.NewCertPool()
	intBlock, _ := pem.Decode([]byte(chain[1]))
	if intBlock == nil {
		t.Fatalf("no PEM block in chain[1] (intermediate)")
	}
	intermediate, err := x509.ParseCertificate(intBlock.Bytes)
	if err != nil {
		t.Fatalf("parse intermediate from chain[1]: %v", err)
	}
	intermediates.AddCert(intermediate)

	if _, err := leaf.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates}); err != nil {
		t.Errorf("chain verify: %v", err)
	}
}
