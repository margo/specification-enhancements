//go:build integration

package integration_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertmtls"
	"github.com/margo/miaf-poc/pkg/mis/ca"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/store"
	misadmin "github.com/margo/miaf-poc/pkg/mis/transport/admin"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

// TestIntegration_Revocation_EnrollAdminRevokeList drives:
//  1. enroll factory-cert-mtls ==> SVID issued
//  2. POST /admin/v1/revocations ==> LDI revoked, serial on list
//  3. GET /api/v1/revocations ==> list includes fingerprint + serial
//  4. conditional GET with If-None-Match ==> 304
//  5. renew attempt on revoked LDI ==> 404
func TestIntegration_Revocation_EnrollAdminRevokeList(t *testing.T) {
	const td = "margo.example.com"

	intCertPath, intKeyPath, _ := mintTestIntermediate(t, td)
	issuer, err := ca.NewIntermediateIssuer(ca.Config{TrustDomain: td, CertFile: intCertPath, KeyFile: intKeyPath})
	if err != nil {
		t.Fatalf("NewIntermediateIssuer: %v", err)
	}

	s := openStore(t)
	policy := identity.StaticPolicy{NewLDI: true, KeyRotation: true}
	svc := identity.NewService(s.Identities(), issuer, policy, identity.NoReplacements{}, s.IssuedSVIDs(), s.RevokedSVIDs(), td, 24*time.Hour)

	factoryRootPEMPath, deviceLeafCert := mintFactoryCA(t)
	factoryValidator, err := factorycertmtls.NewFromPEMFile(factoryRootPEMPath)
	if err != nil {
		t.Fatalf("factoryValidator: %v", err)
	}
	registry := bootstrap.NewRegistry()
	registry.Register(common.BootstrapMethodFactoryCertMTLS, factoryValidator)

	// Build muxes.
	enrollMux := http.NewServeMux()
	enrollMux.Handle("POST /api/v1/identities", mishttp.NewEnrollmentHandler(mishttp.EnrollmentConfig{
		Registry: registry,
		Service:  svc,
	}))

	revocationsHandler := mishttp.NewRevocationsHandler(mishttp.RevocationsConfig{
		Queue: s.RevokedSVIDs(),
	})
	publicMux := http.NewServeMux()
	publicMux.Handle("GET /api/v1/revocations", revocationsHandler)

	adminSrv := misadmin.NewServer(misadmin.Config{Revocations: svc})
	adminMux := adminSrv.Handler()

	renewMux := http.NewServeMux()
	renewMux.Handle("POST /api/v1/identities/{spiffeIdEncoded}/renewal", mishttp.NewRenewalHandler(mishttp.RenewalConfig{
		Service:     svc,
		RateLimiter: s.RenewalEvents(store.RenewalEventsConfig{MaxInWindow: 10, Window: time.Hour}),
	}))

	// ── Step 1: Enroll ──
	devKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, devKey)
	enrollBody, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI:      common.SVIDProfileURIX509V1,
		SVIDRequest:         common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(csrDER)},
		BootstrapCredential: common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS},
	})
	enrollReq := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(enrollBody))
	enrollReq.Header.Set("Content-Type", "application/json")
	factoryLeaf, _ := x509.ParseCertificate(deviceLeafCert.Certificate[0])
	enrollReq.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{factoryLeaf}}
	enrollRec := httptest.NewRecorder()
	enrollMux.ServeHTTP(enrollRec, enrollReq)
	if enrollRec.Code != 201 {
		t.Fatalf("enroll status %d: %s", enrollRec.Code, enrollRec.Body.String())
	}
	var enrollResp common.EnrollmentResponseDTO
	_ = json.Unmarshal(enrollRec.Body.Bytes(), &enrollResp)

	// Extract leaf cert details.
	chainPEM := strings.Join(enrollResp.SVID.CertificateChainPEM, "")
	block, _ := pem.Decode([]byte(chainPEM))
	if block == nil {
		t.Fatal("no PEM block in enrollment response")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	fp := sha256.Sum256(block.Bytes)
	fpHex := hex.EncodeToString(fp[:])
	serialHex := strings.ToUpper(leaf.SerialNumber.Text(16))
	spiffeID := leaf.URIs[0].String()

	// ── Step 2: Admin revoke ──
	revokeBody, _ := json.Marshal(common.RevokeSPIFFEIDRequest{SPIFFEID: spiffeID, Reason: "integration-test"})
	revokeReq := httptest.NewRequest(http.MethodPost, "/admin/v1/revocations", bytes.NewReader(revokeBody))
	revokeReq.Header.Set("Content-Type", "application/json")
	revokeRec := httptest.NewRecorder()
	adminMux.ServeHTTP(revokeRec, revokeReq)
	if revokeRec.Code != 200 {
		t.Fatalf("admin revoke status %d: %s", revokeRec.Code, revokeRec.Body.String())
	}
	var adminResp common.RevokeSPIFFEIDResponse
	_ = json.Unmarshal(revokeRec.Body.Bytes(), &adminResp)
	if len(adminResp.RevokedSerials) != 1 || adminResp.RevokedSerials[0] != serialHex {
		t.Errorf("revoked serials = %+v, want [%q]", adminResp.RevokedSerials, serialHex)
	}

	// ── Step 3: Fetch revocation list ──
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/revocations", nil)
	listRec := httptest.NewRecorder()
	publicMux.ServeHTTP(listRec, listReq)
	if listRec.Code != 200 {
		t.Fatalf("GET revocations status %d: %s", listRec.Code, listRec.Body.String())
	}
	var list common.RevocationListDTO
	_ = json.Unmarshal(listRec.Body.Bytes(), &list)
	if len(list.RevokedSVIDs) != 1 {
		t.Fatalf("list has %d entries, want 1: %+v", len(list.RevokedSVIDs), list)
	}
	got := list.RevokedSVIDs[0]
	if got.SerialNumber != serialHex {
		t.Errorf("serial = %q, want %q", got.SerialNumber, serialHex)
	}
	if got.CertFingerprintSHA256 != fpHex {
		t.Errorf("fingerprint = %q, want %q", got.CertFingerprintSHA256, fpHex)
	}
	if list.LastUpdated == "" {
		t.Error("last_updated is empty")
	}

	// ── Step 4: Conditional GET with If-None-Match ==> 304 ──
	etag := listRec.Header().Get("ETag")
	condReq := httptest.NewRequest(http.MethodGet, "/api/v1/revocations", nil)
	condReq.Header.Set("If-None-Match", etag)
	condRec := httptest.NewRecorder()
	publicMux.ServeHTTP(condRec, condReq)
	if condRec.Code != http.StatusNotModified {
		t.Errorf("conditional GET status = %d, want 304", condRec.Code)
	}

	// ── Step 5: Renew against revoked LDI ==> 404 ──
	renewCSR, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, devKey)
	renewBody, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(renewCSR)},
	})
	renewURL := "/api/v1/identities/" + common.EncodeSPIFFEID(spiffeID) + "/renewal"
	renewReq := httptest.NewRequest(http.MethodPost, renewURL, bytes.NewReader(renewBody))
	renewReq.Header.Set("Content-Type", "application/json")
	renewReq.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}}
	renewRec := httptest.NewRecorder()
	renewMux.ServeHTTP(renewRec, renewReq)
	if renewRec.Code != http.StatusNotFound {
		t.Errorf("renew after revoke status = %d, want 404", renewRec.Code)
	}
}
