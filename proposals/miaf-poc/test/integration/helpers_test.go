//go:build integration

package integration_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/margo/miaf-poc/pkg/mis/jwtsigner"
	"github.com/margo/miaf-poc/pkg/mis/store"
)

// mintTestIntermediate generates a self-signed root + sub-CA intermediate
// whose URI SAN is spiffe://<td>/. Returns PEM cert/key paths on disk and
// the root certificate for chain verification.
func mintTestIntermediate(t *testing.T, td string) (certPath, keyPath string, rootCert *x509.Certificate) {
	t.Helper()
	dir := t.TempDir()

	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("root key: %v", err)
	}
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test root"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("root cert: %v", err)
	}
	rootCert, _ = x509.ParseCertificate(rootDER)

	intKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("intermediate key: %v", err)
	}
	intURI, _ := url.Parse("spiffe://" + td + "/")
	intTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "test intermediate"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(12 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		URIs:                  []*url.URL{intURI},
		BasicConstraintsValid: true,
	}
	intDER, err := x509.CreateCertificate(rand.Reader, intTmpl, rootCert, &intKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("intermediate cert: %v", err)
	}

	certPath = filepath.Join(dir, "int.crt")
	keyPath = filepath.Join(dir, "int.key")
	writePEM(t, certPath, "CERTIFICATE", intDER)
	keyDER, _ := x509.MarshalPKCS8PrivateKey(intKey)
	writePEM(t, keyPath, "PRIVATE KEY", keyDER)
	return
}

func writePEM(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	buf := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		t.Fatal(err)
	}
}

// mintFactoryCA creates a self-signed factory CA and a leaf cert signed by it.
// Returns the factory CA PEM path and the device leaf cert+key as a tls.Certificate.
func mintFactoryCA(t *testing.T) (factoryRootPEMPath string, leafCert tls.Certificate) {
	t.Helper()
	dir := t.TempDir()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("factory CA key: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(10),
		Subject:               pkix.Name{CommonName: "test factory CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("factory CA cert: %v", err)
	}
	factoryRoot, _ := x509.ParseCertificate(caDER)

	factoryRootPEMPath = filepath.Join(dir, "factory-root.crt")
	writePEM(t, factoryRootPEMPath, "CERTIFICATE", caDER)

	// Leaf cert signed by factory CA
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("leaf key: %v", err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(11),
		Subject:      pkix.Name{CommonName: "test-device-01"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(12 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, factoryRoot, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("leaf cert: %v", err)
	}

	leafCertPath := filepath.Join(dir, "device.crt")
	leafKeyPath := filepath.Join(dir, "device.key")
	writePEM(t, leafCertPath, "CERTIFICATE", leafDER)
	leafKeyDER, _ := x509.MarshalPKCS8PrivateKey(leafKey)
	writePEM(t, leafKeyPath, "PRIVATE KEY", leafKeyDER)

	leafCert, err = tls.LoadX509KeyPair(leafCertPath, leafKeyPath)
	if err != nil {
		t.Fatalf("load leaf keypair: %v", err)
	}
	return
}

// mintFactoryLeafAndChain creates a self-signed factory root CA and a leaf cert
// signed by it. Returns the root PEM path on disk, the parsed leaf certificate,
// the leaf private key, and the parsed root certificate.
// Used by the factory-cert-jwt integration test.
func mintFactoryLeafAndChain(t *testing.T) (rootPEMPath string, leaf *x509.Certificate, leafKey *ecdsa.PrivateKey, root *x509.Certificate) {
	t.Helper()
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("factory root key: %v", err)
	}
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(100),
		Subject:               pkix.Name{CommonName: "Integ Factory Root"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("factory root cert: %v", err)
	}
	root, err = x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatalf("parse factory root: %v", err)
	}

	leafKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("factory leaf key: %v", err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(101),
		Subject:      pkix.Name{CommonName: "Integ Factory Device"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, root, &leafKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("factory leaf cert: %v", err)
	}
	leaf, err = x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatalf("parse factory leaf: %v", err)
	}

	dir := t.TempDir()
	rootPEMPath = filepath.Join(dir, "factory-root.pem")
	if err := os.WriteFile(rootPEMPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: root.Raw}), 0o600); err != nil {
		t.Fatalf("write root pem: %v", err)
	}
	return
}

func readFile(path string) ([]byte, error) { return os.ReadFile(path) }

// mintJWTSigner writes an ECDSA P-256 PKCS#8 PEM into t.TempDir() and returns
// a loaded *jwtsigner.Signer.
func mintJWTSigner(t *testing.T) *jwtsigner.Signer {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.MarshalPKCS8PrivateKey(k)
	path := filepath.Join(t.TempDir(), "jwt.key")
	_ = os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600)
	s, err := jwtsigner.New(jwtsigner.Config{KeyFile: path})
	if err != nil {
		t.Fatalf("jwtsigner.New: %v", err)
	}
	return s
}

// openStore opens a fresh unified MIS store under t.TempDir().
func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// parseChainPEM decodes each PEM string in the certificate_chain_pem array
// from an enrollment / renewal response into parsed certificates, leaf-first.
func parseChainPEM(chainPEM []string) ([]*x509.Certificate, error) {
	out := make([]*x509.Certificate, 0, len(chainPEM))
	for i, p := range chainPEM {
		block, _ := pem.Decode([]byte(p))
		if block == nil {
			return nil, fmt.Errorf("chain[%d]: no PEM block", i)
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("chain[%d]: %w", i, err)
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty certificate chain")
	}
	return out, nil
}

// startFakeStepCA returns an httptest.TLSServer that responds to POST
// /1.0/sign by issuing a leaf cert signed under the supplied CA keypair.
// It honours the OTT user.sans claim: the returned leaf's URI SAN matches
// what MIS sent, confirming the URI-SAN-override contract.
func startFakeStepCA(t *testing.T, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/1.0/sign" {
			http.NotFound(w, r)
			return
		}
		var body struct{ OTT, CSR string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		tok, err := jwt.ParseSigned(body.OTT, []jose.SignatureAlgorithm{jose.ES256, jose.RS256})
		if err != nil {
			http.Error(w, "bad OTT", 400)
			return
		}
		// Read the top-level "sans" claim (the JWK provisioner's designed
		// mechanism; step-ca converts spiffe:// strings to URI SANs via CreateSANs).
		var rawClaims struct {
			SANs []string `json:"sans"`
		}
		_ = tok.UnsafeClaimsWithoutVerification(&rawClaims)

		uris := []*url.URL{}
		for _, s := range rawClaims.SANs {
			if strings.HasPrefix(s, "spiffe://") || strings.HasPrefix(s, "uri:") {
				u, _ := url.Parse(s)
				uris = append(uris, u)
			}
		}

		csrBlock, _ := pem.Decode([]byte(body.CSR))
		csr, _ := x509.ParseCertificateRequest(csrBlock.Bytes)
		leafTmpl := &x509.Certificate{
			SerialNumber: big.NewInt(time.Now().UnixNano()),
			NotBefore:    time.Now().Add(-time.Minute),
			NotAfter:     time.Now().Add(24 * time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
			URIs:         uris,
		}
		leafDER, _ := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, csr.PublicKey, caKey)
		resp := map[string]any{
			"certChain": []string{
				string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})),
				string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}
