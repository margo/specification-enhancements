package identitymanager_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/device-agent/clock"
	"github.com/margo/miaf-poc/pkg/device-agent/identitymanager"
	"github.com/margo/miaf-poc/pkg/device-agent/agentstate"
)

// clockReal is a type alias so tests can use it without importing clock directly.
type clockReal = clock.Real

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// mintDeviceCert creates a self-signed SVID cert+key and writes them to temp files.
// Returns certPath, keyPath, the raw certificate DER bytes, and the private key.
func mintDeviceCert(t *testing.T) (certPath, keyPath string, certDER []byte, key *ecdsa.PrivateKey) {
	t.Helper()
	dir := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey: %v", err)
	}
	spiffeURI, _ := url.Parse("spiffe://td.example/margo/device/test")
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: "test-device"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		URIs:         []*url.URL{spiffeURI},
	}
	certDER, err = x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	certPath = filepath.Join(dir, "svid.pem")
	keyPath = filepath.Join(dir, "svid.key")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, _ := x509.MarshalPKCS8PrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	os.WriteFile(certPath, certPEM, 0o600)
	os.WriteFile(keyPath, keyPEM, 0o600)
	return
}

// tlsServerCA extracts the CA PEM from a TLS test server and writes it to a file.
func tlsServerCA(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	dir := t.TempDir()
	certBytes := srv.TLS.Certificates[0].Certificate[0]
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	path := filepath.Join(dir, "ca.pem")
	os.WriteFile(path, pemBytes, 0o600)
	return path
}

// seedState writes a minimal state.json to a temp dir and returns the path.
func seedState(t *testing.T, spiffeID string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st := agentstate.State{
		SPIFFEID:     spiffeID,
		Serial:       "1",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		LastEnrolled: time.Now().Add(-time.Hour),
	}
	b, _ := json.Marshal(st)
	os.WriteFile(path, b, 0o600)
	return path
}

// buildManager creates a Manager configured against the given test server URL and files.
func buildManager(t *testing.T, misBaseURL, trustAnchorFile, stateFile, certFile, keyFile string) *identitymanager.Manager {
	t.Helper()
	return identitymanager.NewManager(identitymanager.Config{
		Clock:          clockReal{},
		StateFile:      stateFile,
		SVIDCertFile:   certFile,
		SVIDKeyFile:    keyFile,
		MISBaseURL:     misBaseURL,
		MISTrustAnchor: trustAnchorFile,
		BundleMapURL:   misBaseURL + "/bundle",
		RevocationsURL: misBaseURL + "/api/v1/revocations",
		Logger:         discardLogger(),
	})
}
