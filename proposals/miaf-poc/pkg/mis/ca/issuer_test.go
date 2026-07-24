package ca_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/ca"
)

// mintTestIntermediate generates a self-signed root and a sub-CA intermediate
// whose URI SAN is spiffe://<td>/. Returns PEM-encoded paths on disk.
func mintTestIntermediate(t *testing.T, td string) (intermediateCertPath, intermediateKeyPath string, rootCert *x509.Certificate) {
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
	uri := parseURI(t, "spiffe://"+td+"/")
	intTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "test intermediate"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(12 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		URIs:                  []*url.URL{uri},
		BasicConstraintsValid: true,
	}
	intDER, err := x509.CreateCertificate(rand.Reader, intTmpl, rootCert, &intKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("intermediate cert: %v", err)
	}

	intermediateCertPath = filepath.Join(dir, "int.crt")
	intermediateKeyPath = filepath.Join(dir, "int.key")
	writePEM(t, intermediateCertPath, "CERTIFICATE", intDER)
	keyDER, _ := x509.MarshalPKCS8PrivateKey(intKey)
	writePEM(t, intermediateKeyPath, "PRIVATE KEY", keyDER)
	return
}

func writePEM(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	buf := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		t.Fatal(err)
	}
}

func parseURI(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestIntermediateIssuer_IssueX509SVID_HappyPath(t *testing.T) {
	td := "margo.example.com"
	certPath, keyPath, rootCert := mintTestIntermediate(t, td)
	issuer, err := ca.NewIntermediateIssuer(ca.Config{
		TrustDomain: td,
		CertFile:    certPath,
		KeyFile:     keyPath,
	})
	if err != nil {
		t.Fatalf("NewIntermediateIssuer: %v", err)
	}

	deviceKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, deviceKey)

	chain, expires, err := issuer.IssueX509SVID(context.Background(),
		"spiffe://"+td+"/margo/device/abc", csrDER, time.Hour)
	if err != nil {
		t.Fatalf("IssueX509SVID: %v", err)
	}
	if len(chain) != 2 {
		t.Fatalf("chain length = %d, want 2 (leaf + intermediate)", len(chain))
	}

	leaf, err := x509.ParseCertificate(chain[0])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	if len(leaf.URIs) != 1 || leaf.URIs[0].String() != "spiffe://"+td+"/margo/device/abc" {
		t.Errorf("leaf URI SANs = %v, want one SPIFFE URI", leaf.URIs)
	}
	if leaf.IsCA {
		t.Error("leaf must not be a CA")
	}
	if leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		t.Error("leaf missing KeyUsageDigitalSignature")
	}
	if !leaf.PublicKey.(*ecdsa.PublicKey).Equal(&deviceKey.PublicKey) {
		t.Error("leaf public key does not match device CSR")
	}
	if !expires.After(time.Now()) || expires.Sub(time.Now()) > 2*time.Hour {
		t.Errorf("expires = %v, want ~1h from now", expires)
	}

	// Chain validates to root.
	roots := x509.NewCertPool()
	roots.AddCert(rootCert)
	intermediates := x509.NewCertPool()
	intermediate, _ := x509.ParseCertificate(chain[1])
	intermediates.AddCert(intermediate)
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates}); err != nil {
		t.Errorf("chain verify: %v", err)
	}
}

func TestIntermediateIssuer_IgnoresCSRSubjectAndSANs(t *testing.T) {
	td := "margo.example.com"
	certPath, keyPath, _ := mintTestIntermediate(t, td)
	issuer, _ := ca.NewIntermediateIssuer(ca.Config{
		TrustDomain: td, CertFile: certPath, KeyFile: keyPath,
	})

	deviceKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	// CSR has attacker-chosen Subject and SANs that MUST be ignored per SUP §5.
	bogus, _ := url.Parse("spiffe://attacker.example/rogue")
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:            pkix.Name{CommonName: "bogus"},
		DNSNames:           []string{"attacker.example"},
		URIs:               []*url.URL{bogus},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, deviceKey)

	chain, _, err := issuer.IssueX509SVID(context.Background(),
		"spiffe://"+td+"/margo/device/xyz", csrDER, time.Hour)
	if err != nil {
		t.Fatalf("IssueX509SVID: %v", err)
	}
	leaf, _ := x509.ParseCertificate(chain[0])
	if leaf.Subject.CommonName == "bogus" {
		t.Error("CSR Subject CN leaked into leaf (must be ignored)")
	}
	if len(leaf.DNSNames) != 0 {
		t.Errorf("leaf DNSNames = %v, want none (CSR DNS SAN must be ignored)", leaf.DNSNames)
	}
	if len(leaf.URIs) != 1 || leaf.URIs[0].String() != "spiffe://"+td+"/margo/device/xyz" {
		t.Errorf("leaf URIs = %v, want only the authoritative SPIFFE ID", leaf.URIs)
	}
}

func TestIntermediateIssuer_RejectsCSRWithUnsupportedKey(t *testing.T) {
	td := "margo.example.com"
	certPath, keyPath, _ := mintTestIntermediate(t, td)
	issuer, _ := ca.NewIntermediateIssuer(ca.Config{
		TrustDomain: td, CertFile: certPath, KeyFile: keyPath,
	})

	// P-384 is not in the MIAF algorithm table (§3 Cryptographic Requirements:
	// ECDSA MUST use P-256).
	k, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA384,
	}, k)

	_, _, err := issuer.IssueX509SVID(context.Background(),
		"spiffe://"+td+"/margo/device/abc", csrDER, time.Hour)
	if err == nil {
		t.Fatal("expected error for unsupported public key curve")
	}
}

func TestIntermediateIssuer_RejectsMismatchedTrustDomain(t *testing.T) {
	td := "margo.example.com"
	certPath, keyPath, _ := mintTestIntermediate(t, td)
	issuer, _ := ca.NewIntermediateIssuer(ca.Config{
		TrustDomain: td, CertFile: certPath, KeyFile: keyPath,
	})

	deviceKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, deviceKey)

	_, _, err := issuer.IssueX509SVID(context.Background(),
		"spiffe://other.example/margo/device/abc", csrDER, time.Hour)
	if err == nil {
		t.Fatal("expected error for SPIFFE ID outside the trust domain")
	}
}

// TestIntermediateIssuer_ClampsNotAfterToIntermediate asserts that a leaf
// issued with a TTL exceeding the intermediate's remaining lifetime has its
// NotAfter clamped to the intermediate's NotAfter. A leaf that outlived its
// signer could not be validated after the intermediate expired - a load-
// bearing security property.
func TestIntermediateIssuer_ClampsNotAfterToIntermediate(t *testing.T) {
	td := "margo.example.com"
	certPath, keyPath, _ := mintTestIntermediate(t, td)
	issuer, err := ca.NewIntermediateIssuer(ca.Config{
		TrustDomain: td, CertFile: certPath, KeyFile: keyPath,
	})
	if err != nil {
		t.Fatalf("NewIntermediateIssuer: %v", err)
	}

	// Look up the intermediate's NotAfter directly from the PEM so the
	// assertion does not re-derive it.
	pemBytes, _ := os.ReadFile(certPath)
	block, _ := pem.Decode(pemBytes)
	intermediate, _ := x509.ParseCertificate(block.Bytes)

	deviceKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, deviceKey)

	// Intermediate's lifetime is 12h; request 48h so the clamp must activate.
	chain, expires, err := issuer.IssueX509SVID(context.Background(),
		"spiffe://"+td+"/margo/device/clamp", csrDER, 48*time.Hour)
	if err != nil {
		t.Fatalf("IssueX509SVID: %v", err)
	}
	if !expires.Equal(intermediate.NotAfter) {
		t.Errorf("expires = %v, want intermediate.NotAfter = %v", expires, intermediate.NotAfter)
	}
	leaf, _ := x509.ParseCertificate(chain[0])
	if !leaf.NotAfter.Equal(intermediate.NotAfter) {
		t.Errorf("leaf.NotAfter = %v, want intermediate.NotAfter = %v", leaf.NotAfter, intermediate.NotAfter)
	}
}

// TestIntermediateIssuer_RejectsMultiCertFile asserts that loadCert refuses
// an intermediate cert file containing more than one CERTIFICATE block. The
// MIS intermediate is a single cert; stacking additional certs suggests the
// operator supplied a full chain where only the leaf intermediate belongs.
func TestIntermediateIssuer_RejectsMultiCertFile(t *testing.T) {
	td := "margo.example.com"
	certPath, keyPath, rootCert := mintTestIntermediate(t, td)

	// Append the root cert's PEM to the intermediate cert file.
	pemBytes, _ := os.ReadFile(certPath)
	rootPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootCert.Raw})
	if err := os.WriteFile(certPath, append(pemBytes, rootPEM...), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := ca.NewIntermediateIssuer(ca.Config{
		TrustDomain: td, CertFile: certPath, KeyFile: keyPath,
	})
	if err == nil {
		t.Fatal("expected error for cert file containing multiple certificates")
	}
}

// TestNewIntermediateIssuer_RejectsUnsupportedIntermediateKey asserts that the
// constructor refuses an intermediate whose own public key violates SUP §3
// "Cryptographic Requirements" (ECDSA MUST be P-256, RSA MUST be ≥3072).
// The intermediate is a trust-critical credential - its key type is checked
// at load time, not only for CSRs at issuance time.
func TestNewIntermediateIssuer_RejectsUnsupportedIntermediateKey(t *testing.T) {
	td := "margo.example.com"
	dir := t.TempDir()

	rootKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	rootDER, _ := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	rootCert, _ := x509.ParseCertificate(rootDER)

	// Intermediate uses P-384 - not SUP-compliant.
	intKey, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	uri := parseURI(t, "spiffe://"+td+"/")
	intTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(12 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		URIs:                  []*url.URL{uri},
		BasicConstraintsValid: true,
	}
	intDER, _ := x509.CreateCertificate(rand.Reader, intTmpl, rootCert, &intKey.PublicKey, rootKey)

	certPath := filepath.Join(dir, "int.crt")
	keyPath := filepath.Join(dir, "int.key")
	writePEM(t, certPath, "CERTIFICATE", intDER)
	keyDER, _ := x509.MarshalPKCS8PrivateKey(intKey)
	writePEM(t, keyPath, "PRIVATE KEY", keyDER)

	_, err := ca.NewIntermediateIssuer(ca.Config{
		TrustDomain: td, CertFile: certPath, KeyFile: keyPath,
	})
	if err == nil {
		t.Fatal("expected error for intermediate with P-384 key")
	}
}

// TestNewIntermediateIssuer_AcceptsPathlessURI asserts that an intermediate
// with a SPIFFE URI SAN that lacks the trailing slash (spiffe://<td>) loads
// successfully - SPIFFE X509-SVID §3.2 treats empty path and root-only path
// as the same authority-only form.
func TestNewIntermediateIssuer_AcceptsPathlessURI(t *testing.T) {
	td := "margo.example.com"
	dir := t.TempDir()

	rootKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	rootDER, _ := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	rootCert, _ := x509.ParseCertificate(rootDER)

	intKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	// No trailing slash - both forms should be accepted.
	uri := parseURI(t, "spiffe://"+td)
	intTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(12 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		URIs:                  []*url.URL{uri},
		BasicConstraintsValid: true,
	}
	intDER, _ := x509.CreateCertificate(rand.Reader, intTmpl, rootCert, &intKey.PublicKey, rootKey)

	certPath := filepath.Join(dir, "int.crt")
	keyPath := filepath.Join(dir, "int.key")
	writePEM(t, certPath, "CERTIFICATE", intDER)
	keyDER, _ := x509.MarshalPKCS8PrivateKey(intKey)
	writePEM(t, keyPath, "PRIVATE KEY", keyDER)

	if _, err := ca.NewIntermediateIssuer(ca.Config{
		TrustDomain: td, CertFile: certPath, KeyFile: keyPath,
	}); err != nil {
		t.Fatalf("NewIntermediateIssuer rejected path-less URI: %v", err)
	}
}
