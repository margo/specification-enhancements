package factorycertmtls_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertmtls"
)

func TestValidate_HappyPath_DerivesESIFromLeafFingerprint(t *testing.T) {
	root, rootKey := mustSelfSignedCA(t, "Test Factory Root")
	leaf, _ := mustIssueLeaf(t, root, rootKey, "test-device")

	pool := x509.NewCertPool()
	pool.AddCert(root)
	v := factorycertmtls.New(pool)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/identities", nil)
	r.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{leaf, root},
	}

	outcome, err := v.Validate(context.Background(), r, common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS}, "", "")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if outcome.Pending == nil {
		t.Fatal("outcome.Pending is nil")
	}
	vb := outcome.Pending.VB
	want := sha256.Sum256(leaf.Raw)
	if !bytes.Equal(vb.ESI, want[:]) {
		t.Errorf("ESI mismatch\ngot:  %x\nwant: %x", vb.ESI, want[:])
	}
	if vb.Method != common.BootstrapMethodFactoryCertMTLS {
		t.Errorf("Method = %q", vb.Method)
	}
	if vb.Evidence == nil {
		t.Error("Evidence must carry audit material (leaf cert)")
	}
}

func TestValidate_RejectsMissingTLS(t *testing.T) {
	v := factorycertmtls.New(x509.NewCertPool())
	r := httptest.NewRequest(http.MethodPost, "/api/v1/identities", nil)

	_, err := v.Validate(context.Background(), r, common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS}, "", "")
	if !errors.Is(err, factorycertmtls.ErrMissingMTLS) {
		t.Errorf("err = %v, want ErrMissingMTLS", err)
	}
}

func TestValidate_RejectsNoPeerCert(t *testing.T) {
	v := factorycertmtls.New(x509.NewCertPool())
	r := httptest.NewRequest(http.MethodPost, "/api/v1/identities", nil)
	r.TLS = &tls.ConnectionState{}

	_, err := v.Validate(context.Background(), r, common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS}, "", "")
	if !errors.Is(err, factorycertmtls.ErrMissingMTLS) {
		t.Errorf("err = %v, want ErrMissingMTLS", err)
	}
}

func TestValidate_RejectsUntrustedChain(t *testing.T) {
	trusted, _ := mustSelfSignedCA(t, "Trusted")
	rogue, rogueKey := mustSelfSignedCA(t, "Rogue")
	leaf, _ := mustIssueLeaf(t, rogue, rogueKey, "rogue-device")

	pool := x509.NewCertPool()
	pool.AddCert(trusted)
	v := factorycertmtls.New(pool)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/identities", nil)
	r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf, rogue}}

	_, err := v.Validate(context.Background(), r, common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertMTLS}, "", "")
	if !errors.Is(err, factorycertmtls.ErrUntrustedChain) {
		t.Errorf("err = %v, want ErrUntrustedChain", err)
	}
}

func TestValidate_RejectsProofField(t *testing.T) {
	root, rootKey := mustSelfSignedCA(t, "R")
	leaf, _ := mustIssueLeaf(t, root, rootKey, "d")
	pool := x509.NewCertPool()
	pool.AddCert(root)
	v := factorycertmtls.New(pool)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/identities", nil)
	r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}}

	// SUP Appendix A: "proof MUST be omitted (null or absent)"
	cred := common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodFactoryCertMTLS,
		Proof:  &common.BootstrapProof{Token: "should-not-be-here"},
	}
	_, err := v.Validate(context.Background(), r, cred, "", "")
	if !errors.Is(err, factorycertmtls.ErrUnexpectedProof) {
		t.Errorf("err = %v, want ErrUnexpectedProof", err)
	}
}

func TestNewFromPEMFile(t *testing.T) {
	root, _ := mustSelfSignedCA(t, "PEM Test Root")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: root.Raw})

	dir := t.TempDir()
	path := filepath.Join(dir, "factory-root.pem")
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := factorycertmtls.NewFromPEMFile(path)
	if err != nil {
		t.Fatalf("NewFromPEMFile: %v", err)
	}
}

// --- helpers --------------------------------------------------------------

func mustSelfSignedCA(t *testing.T, cn string) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse CA: %v", err)
	}
	return cert, key
}

func mustIssueLeaf(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, cn string) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	return cert, key
}
