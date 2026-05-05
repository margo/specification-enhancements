//go:build integration

package integration_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/device-agent/jwtsvid"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertmtls"
	"github.com/margo/miaf-poc/pkg/mis/bundlemap"
	"github.com/margo/miaf-poc/pkg/mis/ca"
	errpkg "github.com/margo/miaf-poc/pkg/mis/errors"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/store"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

func TestRenewal_Integration(t *testing.T) {
	const td = "margo.example.com"

	intCertPath, intKeyPath, rootCert := mintTestIntermediate(t, td)
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

	enrollH := mishttp.NewEnrollmentHandler(mishttp.EnrollmentConfig{Registry: registry, Service: svc})
	rateLimiter := s.RenewalEvents(store.RenewalEventsConfig{MaxInWindow: 2, Window: time.Hour})
	renewH := mishttp.NewRenewalHandler(mishttp.RenewalConfig{Service: svc, RateLimiter: rateLimiter})

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/identities", enrollH)
	mux.Handle("POST /api/v1/identities/{spiffeIdEncoded}/renewals", renewH)

	factoryLeaf, _ := x509.ParseCertificate(deviceLeafCert.Certificate[0])

	// --- Phase 1: factory-mTLS enrollment yields an SVID we can renew ---
	devKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	enrollCSR, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{SignatureAlgorithm: x509.ECDSAWithSHA256}, devKey)
	enrollBody, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI:      common.SVIDProfileURIX509V1,
		SVIDRequest:         common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(enrollCSR)},
		BootstrapCredential: common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS},
	})
	enrollReq := httptest.NewRequest(http.MethodPost, "/api/v1/identities", bytes.NewReader(enrollBody))
	enrollReq.Header.Set("Content-Type", "application/json")
	enrollReq.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{factoryLeaf}}
	enrollRec := httptest.NewRecorder()
	mux.ServeHTTP(enrollRec, enrollReq)
	if enrollRec.Code != 201 {
		t.Fatalf("enroll status %d: %s", enrollRec.Code, enrollRec.Body.String())
	}
	var enrollResp common.EnrollmentResponseDTO
	_ = json.Unmarshal(enrollRec.Body.Bytes(), &enrollResp)
	block, _ := pem.Decode([]byte(enrollResp.SVID.CertificateChainPEM[0]))
	svidLeaf, _ := x509.ParseCertificate(block.Bytes)
	_ = rootCert // kept around in case the test is extended to chain-verify

	spiffeID := svidLeaf.URIs[0].String()

	// Helper that simulates a renewal request with the SVID as peer cert.
	postRenewal := func(t *testing.T, csrKey *ecdsa.PrivateKey) *httptest.ResponseRecorder {
		t.Helper()
		der, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{SignatureAlgorithm: x509.ECDSAWithSHA256}, csrKey)
		body, _ := json.Marshal(common.EnrollmentRequestDTO{
			SVIDProfileURI: common.SVIDProfileURIX509V1,
			SVIDRequest:    common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(der)},
		})
		path := "/api/v1/identities/" + common.EncodeSPIFFEID(spiffeID) + "/renewals"
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{svidLeaf}}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	// --- Phase 2: same-key renewal succeeds ---
	if rec := postRenewal(t, devKey); rec.Code != 200 {
		t.Fatalf("same-key renewal: status %d body %s", rec.Code, rec.Body.String())
	}

	// --- Phase 3: budget of 2 - first already consumed. Second succeeds. ---
	if rec := postRenewal(t, devKey); rec.Code != 200 {
		t.Fatalf("second same-key renewal: status %d", rec.Code)
	}

	// --- Phase 4: third call is rate-limited ---
	rec := postRenewal(t, devKey)
	if rec.Code != 429 {
		t.Fatalf("third renewal: status %d, want 429", rec.Code)
	}
	if ra := rec.Header().Get("Retry-After"); ra == "" {
		t.Error("Retry-After header missing on 429")
	}
	var pd errpkg.ProblemDetails
	_ = json.Unmarshal(rec.Body.Bytes(), &pd)
	if pd.Type != "https://margo.org/docs/errors/too-many-requests" {
		t.Errorf("type = %q", pd.Type)
	}

	// --- Phase 5: new-key renewal denied under tight policy ---
	tightSvc := identity.NewService(
		s.Identities(), issuer,
		identity.StaticPolicy{NewLDI: false, KeyRotation: false},
		identity.NoReplacements{}, s.IssuedSVIDs(), s.RevokedSVIDs(), td, 24*time.Hour,
	)
	tightLim := s.RenewalEvents(store.RenewalEventsConfig{MaxInWindow: 100, Window: time.Hour})
	tightH := mishttp.NewRenewalHandler(mishttp.RenewalConfig{Service: tightSvc, RateLimiter: tightLim})
	tightMux := http.NewServeMux()
	tightMux.Handle("POST /api/v1/identities/{spiffeIdEncoded}/renewals", tightH)

	rotKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	rotDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{SignatureAlgorithm: x509.ECDSAWithSHA256}, rotKey)
	rotBody, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(rotDER)},
	})
	rotReq := httptest.NewRequest(http.MethodPost,
		"/api/v1/identities/"+common.EncodeSPIFFEID(spiffeID)+"/renewals",
		bytes.NewReader(rotBody))
	rotReq.Header.Set("Content-Type", "application/json")
	rotReq.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{svidLeaf}}
	rotRec := httptest.NewRecorder()
	tightMux.ServeHTTP(rotRec, rotReq)
	if rotRec.Code != 409 {
		t.Fatalf("new-key renewal under tight policy: status %d body %s", rotRec.Code, rotRec.Body.String())
	}
}

func TestRenewal_BearerIntegration(t *testing.T) {
	const td = "margo.example.com"

	intCertPath, intKeyPath, rootCert := mintTestIntermediate(t, td)
	issuer, err := ca.NewIntermediateIssuer(ca.Config{TrustDomain: td, CertFile: intCertPath, KeyFile: intKeyPath})
	if err != nil {
		t.Fatalf("NewIntermediateIssuer: %v", err)
	}

	st := openStore(t)
	jwtSigner := mintJWTSigner(t)
	rootPath := t.TempDir() + "/root.pem"
	writePEM(t, rootPath, "CERTIFICATE", rootCert.Raw)
	trustAnchors, err := ca.NewTrustAnchors(ca.TrustAnchorsConfig{
		TrustDomain:    td,
		RootFile:       rootPath,
		JWTAuthorities: []bundlemap.JWTAuthority{jwtSigner.JWTAuthority()},
	})
	if err != nil {
		t.Fatalf("NewTrustAnchors: %v", err)
	}

	svc := identity.NewService(
		st.Identities(), issuer, identity.StaticPolicy{NewLDI: true, KeyRotation: true},
		identity.NoReplacements{}, st.IssuedSVIDs(), st.RevokedSVIDs(), td, 24*time.Hour,
	)

	factoryRootPEMPath, deviceLeafCert := mintFactoryCA(t)
	factoryValidator, err := factorycertmtls.NewFromPEMFile(factoryRootPEMPath)
	if err != nil {
		t.Fatalf("factorycertmtls.NewFromPEMFile: %v", err)
	}
	registry := bootstrap.NewRegistry()
	registry.Register(common.BootstrapMethodFactoryCertMTLS, factoryValidator)
	enrollH := mishttp.NewEnrollmentHandler(mishttp.EnrollmentConfig{Registry: registry, Service: svc})

	var currentPeer *x509.Certificate
	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if currentPeer != nil {
			r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{currentPeer}}
		} else {
			r.TLS = &tls.ConnectionState{}
		}
		mux.ServeHTTP(w, r)
	}))
	srv.Start()
	t.Cleanup(srv.Close)

	renewH := mishttp.NewRenewalHandler(mishttp.RenewalConfig{
		Service:         svc,
		RateLimiter:     st.RenewalEvents(store.RenewalEventsConfig{MaxInWindow: 10, Window: time.Hour}),
		BearerVerifier:  mishttp.NewLocalRenewalBearerVerifier(trustAnchors),
		EndpointBaseURL: srv.URL,
	})
	jwtSVIDH := mishttp.NewJWTSVIDHandler(mishttp.JWTSVIDConfig{
		Service:      svc,
		Signer:       jwtSigner,
		IssuanceLog:  st.IssuedJWTSVIDs(),
		LeafLookup:   st.IssuedSVIDs(),
		ReplayStore:  st.JWTReplay(),
		RateLimiter:  st.RenewalEvents(store.RenewalEventsConfig{MaxInWindow: 10, Window: time.Hour}),
		IssuerURL:    srv.URL + "/",
		EndpointBase: srv.URL,
		MaxTTL:       time.Hour,
		DefaultTTL:   5 * time.Minute,
	})
	mux.Handle("POST /api/v1/identities", enrollH)
	mux.Handle("POST /api/v1/identities/{spiffeIdEncoded}/renewals", renewH)
	mux.Handle("POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid", jwtSVIDH)

	factoryLeaf, _ := x509.ParseCertificate(deviceLeafCert.Certificate[0])

	devKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	enrollCSR, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{SignatureAlgorithm: x509.ECDSAWithSHA256}, devKey)
	enrollBody, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI:      common.SVIDProfileURIX509V1,
		SVIDRequest:         common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(enrollCSR)},
		BootstrapCredential: common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS},
	})
	currentPeer = factoryLeaf
	enrollResp := doPost(t, srv.URL+"/api/v1/identities", enrollBody)
	if enrollResp.Code != http.StatusCreated {
		t.Fatalf("enroll status %d: %s", enrollResp.Code, enrollResp.Body.String())
	}
	var enrollDTO common.EnrollmentResponseDTO
	_ = json.Unmarshal(enrollResp.Body.Bytes(), &enrollDTO)
	block, _ := pem.Decode([]byte(enrollDTO.SVID.CertificateChainPEM[0]))
	originalLeaf, _ := x509.ParseCertificate(block.Bytes)
	spiffeID := originalLeaf.URIs[0].String()
	encoded := common.EncodeSPIFFEID(spiffeID)

	currentPeer = nil
	assertion, err := jwtsvid.BuildClientAssertion(devKey, spiffeID, srv.URL+"/api/v1/identities/"+encoded+"/jwt-svid", time.Now())
	if err != nil {
		t.Fatalf("BuildClientAssertion: %v", err)
	}
	jwtBody, _ := json.Marshal(common.JWTSVIDExchangeRequestDTO{
		Audience:        []string{srv.URL + "/api/v1/identities/" + encoded + "/renewals"},
		ClientAssertion: assertion,
	})
	jwtResp := doPost(t, srv.URL+"/api/v1/identities/"+encoded+"/jwt-svid", jwtBody)
	if jwtResp.Code != http.StatusCreated {
		t.Fatalf("jwt-svid exchange: status %d body %s", jwtResp.Code, jwtResp.Body.String())
	}
	var jwtDTO common.JWTSVIDExchangeResponseDTO
	if err := json.Unmarshal(jwtResp.Body.Bytes(), &jwtDTO); err != nil {
		t.Fatalf("decode jwt-svid exchange: %v", err)
	}
	if jwtDTO.JWT == "" {
		t.Fatal("jwt-svid exchange returned empty JWT")
	}

	rotKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	rotCSR, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{SignatureAlgorithm: x509.ECDSAWithSHA256}, rotKey)
	rotBody, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(rotCSR)},
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/identities/"+encoded+"/renewals", bytes.NewReader(rotBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtDTO.JWT)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("bearer renewal request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bearer renewal status %d: %s", resp.StatusCode, body)
	}
	var renewed common.EnrollmentResponseDTO
	if err := json.Unmarshal(body, &renewed); err != nil {
		t.Fatalf("decode bearer renewal response: %v", err)
	}
	newBlock, _ := pem.Decode([]byte(renewed.SVID.CertificateChainPEM[0]))
	renewedLeaf, _ := x509.ParseCertificate(newBlock.Bytes)
	if renewedLeaf.URIs[0].String() != spiffeID {
		t.Fatalf("SPIFFE ID changed across bearer renewal: %q -> %q", spiffeID, renewedLeaf.URIs[0].String())
	}
	if renewedLeaf.SerialNumber.Cmp(originalLeaf.SerialNumber) == 0 {
		t.Fatal("bearer renewal returned the original serial; expected a fresh SVID")
	}
	wantPub, _ := x509.MarshalPKIXPublicKey(&rotKey.PublicKey)
	gotPub, _ := x509.MarshalPKIXPublicKey(renewedLeaf.PublicKey)
	if !bytes.Equal(gotPub, wantPub) {
		t.Fatal("bearer renewal leaf public key does not match the CSR key")
	}
}
