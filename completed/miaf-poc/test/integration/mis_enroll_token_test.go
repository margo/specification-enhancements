//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
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
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/enrollmenttoken"
	"github.com/margo/miaf-poc/pkg/mis/ca"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

func TestEnrollToken_Integration(t *testing.T) {
	const trustDomain = "margo.example.com"

	intCertPath, intKeyPath, rootCert := mintTestIntermediate(t, trustDomain)
	issuer, err := ca.NewIntermediateIssuer(ca.Config{
		TrustDomain: trustDomain, CertFile: intCertPath, KeyFile: intKeyPath,
	})
	if err != nil {
		t.Fatalf("NewIntermediateIssuer: %v", err)
	}

	s := openStore(t)
	repo := s.Identities()
	ts := s.EnrollmentTokens()

	policy := identity.StaticPolicy{NewLDI: true, KeyRotation: true}
	svc := identity.NewService(repo, issuer, policy, identity.NoReplacements{}, s.IssuedSVIDs(), s.RevokedSVIDs(), trustDomain, 24*time.Hour)

	registry := bootstrap.NewRegistry()
	registry.Register(common.BootstrapMethodEnrollmentToken, enrollmenttoken.New(enrollmenttoken.Config{Store: ts}))

	handler := mishttp.NewEnrollmentHandler(mishttp.EnrollmentConfig{Registry: registry, Service: svc})

	c, err := ts.Create(context.Background(), 5*time.Minute, time.Now())
	if err != nil {
		t.Fatalf("Create token: %v", err)
	}
	deviceKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate device key: %v", err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, deviceKey)
	if err != nil {
		t.Fatalf("create CSR: %v", err)
	}
	body, err := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(csrDER)},
		BootstrapCredential: common.BootstrapCredentialDTO{
			Method: common.BootstrapMethodEnrollmentToken,
			Proof:  &common.BootstrapProof{Token: c.Secret.Wire()},
		},
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d; body: %s", rr.Code, rr.Body.String())
	}
	var resp common.EnrollmentResponseDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	chain := resp.SVID.CertificateChainPEM
	if len(chain) != 2 {
		t.Fatalf("chain len = %d, want 2", len(chain))
	}
	leafBlock, _ := pem.Decode([]byte(chain[0]))
	if leafBlock == nil {
		t.Fatalf("chain[0] is not PEM")
	}
	leaf, err := x509.ParseCertificate(leafBlock.Bytes)
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	if len(leaf.URIs) != 1 || leaf.URIs[0].Host != trustDomain {
		t.Errorf("SPIFFE ID = %v", leaf.URIs)
	}

	// Chain verifies against the root returned by mintTestIntermediate.
	rootPool := x509.NewCertPool()
	rootPool.AddCert(rootCert)
	intPool := x509.NewCertPool()
	intBlock, _ := pem.Decode([]byte(chain[1]))
	if intBlock == nil {
		t.Fatalf("chain[1] is not PEM")
	}
	inter, err := x509.ParseCertificate(intBlock.Bytes)
	if err != nil {
		t.Fatalf("parse intermediate: %v", err)
	}
	intPool.AddCert(inter)
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: rootPool, Intermediates: intPool}); err != nil {
		t.Errorf("chain verify: %v", err)
	}

	// A second POST with a brand-new CSR (different public key) is rejected -
	// the token is consumed and the key mismatch disqualifies it as a replay.
	deviceKey2, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate device key2: %v", err)
	}
	csrDER2, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, deviceKey2)
	if err != nil {
		t.Fatalf("create CSR2: %v", err)
	}
	body2, err := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(csrDER2)},
		BootstrapCredential: common.BootstrapCredentialDTO{
			Method: common.BootstrapMethodEnrollmentToken,
			Proof:  &common.BootstrapProof{Token: c.Secret.Wire()},
		},
	})
	if err != nil {
		t.Fatalf("marshal body2: %v", err)
	}
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusUnauthorized {
		t.Fatalf("second status = %d, want 401", rr2.Code)
	}
}
