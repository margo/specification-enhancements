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

	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertmtls"
	"github.com/margo/miaf-poc/pkg/mis/ca"
	errpkg "github.com/margo/miaf-poc/pkg/mis/errors"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/store"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"

	dagjwtsvid "github.com/margo/miaf-poc/pkg/device-agent/jwtsvid"
)

// TestJWTSVIDExchange_Integration is the end-to-end integration test covering
// both mTLS and client_assertion auth paths, replay rejection, rate limiting,
// and bearer rejection.
func TestJWTSVIDExchange_Integration(t *testing.T) {
	const (
		td        = "margo.example.com"
		issuerURL = "https://mis.example.test"
		audTarget = "https://dfm.example/"
	)

	// ---------- setup shared infrastructure ----------

	intCertPath, intKeyPath, _ := mintTestIntermediate(t, td)
	issuer, err := ca.NewIntermediateIssuer(ca.Config{
		TrustDomain: td,
		CertFile:    intCertPath,
		KeyFile:     intKeyPath,
	})
	if err != nil {
		t.Fatalf("NewIntermediateIssuer: %v", err)
	}

	st := openStore(t)
	jwtSigner := mintJWTSigner(t)

	policy := identity.StaticPolicy{NewLDI: true, KeyRotation: true}
	svc := identity.NewService(
		st.Identities(), issuer, policy, identity.NoReplacements{},
		st.IssuedSVIDs(), st.RevokedSVIDs(), td, 24*time.Hour,
	)

	factoryRootPEMPath, deviceLeafCert := mintFactoryCA(t)
	factoryValidator, err := factorycertmtls.NewFromPEMFile(factoryRootPEMPath)
	if err != nil {
		t.Fatalf("factorycertmtls.NewFromPEMFile: %v", err)
	}
	registry := bootstrap.NewRegistry()
	registry.Register(common.BootstrapMethodFactoryCertMTLS, factoryValidator)

	factoryLeaf, _ := x509.ParseCertificate(deviceLeafCert.Certificate[0])

	// rateLimiter with generous budget for the main test phases.
	rateLimiter := st.RenewalEvents(store.RenewalEventsConfig{MaxInWindow: 10, Window: time.Hour})

	enrollH := mishttp.NewEnrollmentHandler(mishttp.EnrollmentConfig{Registry: registry, Service: svc})
	renewH := mishttp.NewRenewalHandler(mishttp.RenewalConfig{Service: svc,
		RateLimiter: st.RenewalEvents(store.RenewalEventsConfig{MaxInWindow: 10, Window: time.Hour}),
	})

	// jwtSVIDH is built after we know the server URL; see below after server start.

	// We need to know the server URL to compute EndpointBase.
	// Use a channel trick: build the mux first, start the server, then add the
	// jwt-svid handler once we have the URL.
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/identities", enrollH)
	mux.Handle("POST /api/v1/identities/{spiffeIdEncoded}/renewals", renewH)

	// currentPeer holds the peer cert to inject for each request; nil means no
	// peer cert (client_assertion path).
	var currentPeer *x509.Certificate

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

	endpointBase := srv.URL

	jwtSVIDH := mishttp.NewJWTSVIDHandler(mishttp.JWTSVIDConfig{
		Service:      svc,
		Signer:       jwtSigner,
		IssuanceLog:  st.IssuedJWTSVIDs(),
		LeafLookup:   st.IssuedSVIDs(),
		ReplayStore:  st.JWTReplay(),
		RateLimiter:  rateLimiter,
		IssuerURL:    issuerURL,
		EndpointBase: endpointBase,
		MaxTTL:       time.Hour,
		DefaultTTL:   5 * time.Minute,
	})
	mux.Handle("POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid", jwtSVIDH)

	// ---------- Phase 1: Enroll ----------

	devKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	enrollCSR, _ := x509.CreateCertificateRequest(rand.Reader,
		&x509.CertificateRequest{SignatureAlgorithm: x509.ECDSAWithSHA256}, devKey)
	enrollBody, _ := json.Marshal(common.EnrollmentRequestDTO{
		SVIDProfileURI:      common.SVIDProfileURIX509V1,
		SVIDRequest:         common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(enrollCSR)},
		BootstrapCredential: common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS},
	})

	currentPeer = factoryLeaf
	enrollResp := doPost(t, srv.URL+"/api/v1/identities", enrollBody)
	if enrollResp.Code != http.StatusCreated {
		t.Fatalf("enroll: status %d body %s", enrollResp.Code, enrollResp.Body.String())
	}

	var enrollDTO common.EnrollmentResponseDTO
	_ = json.Unmarshal(enrollResp.Body.Bytes(), &enrollDTO)
	block, _ := pem.Decode([]byte(enrollDTO.SVID.CertificateChainPEM[0]))
	svidLeaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse issued SVID leaf: %v", err)
	}
	spiffeID := svidLeaf.URIs[0].String()
	encoded := common.EncodeSPIFFEID(spiffeID)

	// ---------- Phase 2: mTLS exchange ----------

	currentPeer = svidLeaf
	jwtPath := srv.URL + "/api/v1/identities/" + encoded + "/jwt-svid"
	jwtReqBody, _ := json.Marshal(common.JWTSVIDExchangeRequestDTO{
		Audience: []string{audTarget},
	})
	resp2 := doPost(t, jwtPath, jwtReqBody)
	if resp2.Code != http.StatusCreated {
		t.Fatalf("mTLS exchange: status %d body %s", resp2.Code, resp2.Body.String())
	}
	var jwtResp2 common.JWTSVIDExchangeResponseDTO
	if err := json.Unmarshal(resp2.Body.Bytes(), &jwtResp2); err != nil {
		t.Fatalf("mTLS exchange: decode response: %v", err)
	}
	if jwtResp2.JWT == "" {
		t.Fatal("mTLS exchange: JWT field is empty")
	}

	// Verify the JWT is well-formed with the expected sub and aud claims.
	verifyIssuedJWT(t, jwtResp2.JWT, spiffeID, audTarget, jwtSigner.JWTAuthority().PublicKey)

	// ---------- Phase 3: client_assertion exchange ----------

	currentPeer = nil // no peer cert - triggers client_assertion path
	assertionAud := endpointBase + "/api/v1/identities/" + encoded + "/jwt-svid"
	assertion, err := dagjwtsvid.BuildClientAssertion(devKey, spiffeID, assertionAud, time.Now())
	if err != nil {
		t.Fatalf("BuildClientAssertion: %v", err)
	}

	jwtReqBody3, _ := json.Marshal(common.JWTSVIDExchangeRequestDTO{
		Audience:        []string{audTarget},
		ClientAssertion: assertion,
	})
	resp3 := doPost(t, jwtPath, jwtReqBody3)
	if resp3.Code != http.StatusCreated {
		t.Fatalf("client_assertion exchange: status %d body %s", resp3.Code, resp3.Body.String())
	}
	var jwtResp3 common.JWTSVIDExchangeResponseDTO
	if err := json.Unmarshal(resp3.Body.Bytes(), &jwtResp3); err != nil {
		t.Fatalf("client_assertion exchange: decode response: %v", err)
	}
	if jwtResp3.JWT == "" {
		t.Fatal("client_assertion exchange: JWT field is empty")
	}

	// ---------- Phase 4: replay rejection ----------

	currentPeer = nil
	// Re-use the same assertion from Phase 3 - the jti is already in the replay store.
	resp4 := doPost(t, jwtPath, jwtReqBody3)
	if resp4.Code != http.StatusUnauthorized {
		t.Fatalf("replay: status %d want 401; body %s", resp4.Code, resp4.Body.String())
	}

	// ---------- Phase 5: rate-limit ----------

	t.Run("rate_limit", func(t *testing.T) {
		// Fresh store + svc so this sub-test is isolated.
		st2 := openStore(t)
		svc2 := identity.NewService(
			st2.Identities(), issuer, identity.StaticPolicy{NewLDI: true, KeyRotation: true},
			identity.NoReplacements{}, st2.IssuedSVIDs(), st2.RevokedSVIDs(), td, 24*time.Hour,
		)

		// Enroll a second device.
		devKey2, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		csr2, _ := x509.CreateCertificateRequest(rand.Reader,
			&x509.CertificateRequest{SignatureAlgorithm: x509.ECDSAWithSHA256}, devKey2)
		body2, _ := json.Marshal(common.EnrollmentRequestDTO{
			SVIDProfileURI:      common.SVIDProfileURIX509V1,
			SVIDRequest:         common.SVIDRequestX509DTO{CSR: base64.StdEncoding.EncodeToString(csr2)},
			BootstrapCredential: common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS},
		})

		// Tight limiter: budget = 1, so first exchange succeeds, second fails.
		tightLimiter := st2.RenewalEvents(store.RenewalEventsConfig{MaxInWindow: 1, Window: time.Hour})

		registry2 := bootstrap.NewRegistry()
		registry2.Register(common.BootstrapMethodFactoryCertMTLS, factoryValidator)
		enrollH2 := mishttp.NewEnrollmentHandler(mishttp.EnrollmentConfig{Registry: registry2, Service: svc2})

		var peer2 *x509.Certificate
		mux2 := http.NewServeMux()
		mux2.Handle("POST /api/v1/identities", enrollH2)

		srv2 := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if peer2 != nil {
				r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{peer2}}
			} else {
				r.TLS = &tls.ConnectionState{}
			}
			mux2.ServeHTTP(w, r)
		}))
		srv2.Start()
		t.Cleanup(srv2.Close)

		jwtSVIDH2 := mishttp.NewJWTSVIDHandler(mishttp.JWTSVIDConfig{
			Service:      svc2,
			Signer:       jwtSigner,
			IssuanceLog:  st2.IssuedJWTSVIDs(),
			LeafLookup:   st2.IssuedSVIDs(),
			ReplayStore:  st2.JWTReplay(),
			RateLimiter:  tightLimiter,
			IssuerURL:    issuerURL,
			EndpointBase: srv2.URL,
			MaxTTL:       time.Hour,
			DefaultTTL:   5 * time.Minute,
		})
		mux2.Handle("POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid", jwtSVIDH2)

		// Enroll.
		peer2 = factoryLeaf
		erec2 := doPost(t, srv2.URL+"/api/v1/identities", body2)
		if erec2.Code != http.StatusCreated {
			t.Fatalf("rl enroll: status %d body %s", erec2.Code, erec2.Body.String())
		}
		var eDTO2 common.EnrollmentResponseDTO
		_ = json.Unmarshal(erec2.Body.Bytes(), &eDTO2)
		blk2, _ := pem.Decode([]byte(eDTO2.SVID.CertificateChainPEM[0]))
		svid2, _ := x509.ParseCertificate(blk2.Bytes)
		spiffeID2 := svid2.URIs[0].String()
		enc2 := common.EncodeSPIFFEID(spiffeID2)
		jwtPath2 := srv2.URL + "/api/v1/identities/" + enc2 + "/jwt-svid"

		// First exchange: should succeed (consumes the 1-token budget).
		peer2 = svid2
		jwtBody2, _ := json.Marshal(common.JWTSVIDExchangeRequestDTO{
			Audience: []string{audTarget},
		})
		r1 := doPost(t, jwtPath2, jwtBody2)
		if r1.Code != http.StatusCreated {
			t.Fatalf("rl first exchange: status %d body %s", r1.Code, r1.Body.String())
		}

		// Second exchange: budget exhausted ==> 429.
		r2 := doPost(t, jwtPath2, jwtBody2)
		if r2.Code != http.StatusTooManyRequests {
			t.Fatalf("rl second exchange: status %d want 429; body %s", r2.Code, r2.Body.String())
		}
		if ra := r2.Header().Get("Retry-After"); ra == "" {
			t.Error("Retry-After header missing on 429")
		}
		var pd errpkg.ProblemDetails
		_ = json.Unmarshal(r2.Body.Bytes(), &pd)
		if pd.Type != "https://margo.org/docs/errors/too-many-requests" {
			t.Errorf("problem type = %q, want too-many-requests", pd.Type)
		}
	})

	// ---------- Phase 6: bearer rejection ----------

	currentPeer = svidLeaf
	req6, _ := http.NewRequest(http.MethodPost, jwtPath, bytes.NewReader(jwtReqBody))
	req6.Header.Set("Content-Type", "application/json")
	req6.Header.Set("Authorization", "Bearer faketoken")
	resp6raw, err6 := http.DefaultClient.Do(req6)
	if err6 != nil {
		t.Fatalf("bearer rejection: Do: %v", err6)
	}
	defer resp6raw.Body.Close()
	body6, _ := io.ReadAll(resp6raw.Body)
	if resp6raw.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bearer rejection: status %d want 401; body %s", resp6raw.StatusCode, body6)
	}
}

// doPost sends an HTTP POST to the given URL with a JSON body and returns the
// response recorder populated via httptest. Because the server is real (not a
// recorder), we use an actual HTTP client and wrap the result in a recorder for
// a uniform interface.
func doPost(t *testing.T, url string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader(body)) //nolint:noctx
	if err != nil {
		t.Fatalf("doPost %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)

	rec := httptest.NewRecorder()
	rec.Code = resp.StatusCode
	for k, vv := range resp.Header {
		for _, v := range vv {
			rec.Header().Set(k, v)
		}
	}
	rec.Body = bytes.NewBuffer(b)
	return rec
}

// verifyIssuedJWT parses the compact JWT and asserts sub and aud claims match,
// then verifies the signature using the supplied public key.
func verifyIssuedJWT(t *testing.T, compact, wantSub, wantAud string, pubKey any) {
	t.Helper()

	tok, err := josejwt.ParseSigned(compact, []jose.SignatureAlgorithm{
		jose.ES256, jose.RS256, jose.PS256,
	})
	if err != nil {
		t.Fatalf("verifyIssuedJWT: parse: %v", err)
	}

	var claims josejwt.Claims
	if err := tok.Claims(pubKey, &claims); err != nil {
		t.Fatalf("verifyIssuedJWT: verify signature: %v", err)
	}
	if claims.Subject != wantSub {
		t.Errorf("JWT sub = %q, want %q", claims.Subject, wantSub)
	}
	if len(claims.Audience) == 0 || claims.Audience[0] != wantAud {
		t.Errorf("JWT aud = %v, want [%q]", []string(claims.Audience), wantAud)
	}
}
