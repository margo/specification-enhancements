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
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertjwt"
	"github.com/margo/miaf-poc/pkg/mis/ca"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
	"github.com/margo/miaf-poc/test/testsupport/bootstrapjwt"
)

func TestEnrollJWT_Integration(t *testing.T) {
	const trustDomain = "margo.example.com"
	const audience = "https://mis.example.com/api/v1/identities"

	intCertPath, intKeyPath, rootCert := mintTestIntermediate(t, trustDomain)
	issuer, err := ca.NewIntermediateIssuer(ca.Config{
		TrustDomain: trustDomain, CertFile: intCertPath, KeyFile: intKeyPath,
	})
	if err != nil {
		t.Fatalf("NewIntermediateIssuer: %v", err)
	}

	s := openStore(t)
	repo := s.Identities()

	policy := identity.StaticPolicy{NewLDI: true, KeyRotation: true}
	svc := identity.NewService(repo, issuer, policy, identity.NoReplacements{}, s.IssuedSVIDs(), s.RevokedSVIDs(), trustDomain, 24*time.Hour)

	factoryRootPEMPath, factoryLeaf, factoryKey, factoryRoot := mintFactoryLeafAndChain(t)
	pool := x509.NewCertPool()
	poolPEM, _ := readFile(factoryRootPEMPath)
	if !pool.AppendCertsFromPEM(poolPEM) {
		t.Fatalf("append factory root")
	}

	registry := bootstrap.NewRegistry()
	registry.Register(common.BootstrapMethodFactoryCertJWT, factorycertjwt.New(factorycertjwt.Config{
		Roots: pool, Audience: audience,
		Replay: s.JWTReplay(),
	}))

	handler := mishttp.NewEnrollmentHandler(mishttp.EnrollmentConfig{
		Registry: registry, Service: svc,
	})

	deviceKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, deviceKey)
	if err != nil {
		t.Fatalf("create CSR: %v", err)
	}

	a := bootstrapjwt.Mint(t, factoryLeaf, []*x509.Certificate{factoryLeaf, factoryRoot}, factoryKey, audience, bootstrapjwt.Options{})

	reqBody, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(csrDER)},
		BootstrapCredential: common.BootstrapCredentialDTO{
			Method: common.BootstrapMethodFactoryCertJWT,
			Proof:  &common.BootstrapProof{Assertion: a.Compact},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.Background())

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rr.Code, rr.Body.String())
	}

	var resp common.EnrollmentResponseDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	chain := resp.SVID.CertificateChainPEM
	if len(chain) != 2 {
		t.Fatalf("chain len = %d, want 2", len(chain))
	}
	block, _ := pem.Decode([]byte(chain[0]))
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	if len(leaf.URIs) != 1 || leaf.URIs[0].Host != trustDomain {
		t.Errorf("SPIFFE ID = %v", leaf.URIs)
	}

	roots := x509.NewCertPool()
	roots.AddCert(rootCert)
	intermediates := x509.NewCertPool()
	intBlock, _ := pem.Decode([]byte(chain[1]))
	inter, _ := x509.ParseCertificate(intBlock.Bytes)
	intermediates.AddCert(inter)
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates}); err != nil {
		t.Errorf("chain verify: %v", err)
	}
}
