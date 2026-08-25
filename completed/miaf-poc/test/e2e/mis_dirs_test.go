//go:build e2e

package e2e_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
)

// TestMIS_StartsWithDirFlags spawns a local MIS process using only --etc-dir
// and --data-dir (no individual file flags) and verifies it serves a valid
// discovery document. Requires `make dev-up` to have populated var/dev/mis-etc/.
func TestMIS_StartsWithDirFlags(t *testing.T) {
	misBin := filepath.Join("..", "..", "bin", "mis")
	if _, err := os.Stat(misBin); err != nil {
		t.Fatalf("binary missing — run `make build` first: %v", err)
	}

	etcDir := filepath.Join("..", "..", "var", "dev", "mis-etc")
	if _, err := os.Stat(filepath.Join(etcDir, "ca", "mis-intermediate.crt")); err != nil {
		t.Skip("var/dev/mis-etc not populated; run `make dev-up` first")
	}

	dataDir := t.TempDir()
	port := freeLocalPort(t)
	misURL := fmt.Sprintf("https://localhost:%d", port)

	cmd := exec.Command(misBin,
		"--etc-dir", etcDir,
		"--data-dir", dataDir,
		"--issuer-backend", "intermediate",
		"--bind-address", fmt.Sprintf("0.0.0.0:%d", port),
		"--advertise-url", misURL,
		"--admin-bind-address", "127.0.0.1:0",
	)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start mis: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_ = cmd.Wait()
	})

	client := dirsTestHTTPClient(t, filepath.Join(etcDir, "tls", "server.crt"))
	pollMISReady(t, client, misURL+"/margo/v1/discovery", 15*time.Second)

	resp, err := client.Get(misURL + "/margo/v1/discovery")
	if err != nil {
		t.Fatalf("GET discovery: %v", err)
	}
	defer resp.Body.Close()

	var doc common.DiscoveryResponseDTO
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		t.Fatalf("decode discovery: %v", err)
	}
	if doc.TrustDomain != "margo.example.com" {
		t.Errorf("trust_domain = %q, want margo.example.com", doc.TrustDomain)
	}
	if doc.MargoIdentityServiceBaseURI != misURL {
		t.Errorf("mis_base_uri = %q, want %q", doc.MargoIdentityServiceBaseURI, misURL)
	}
}

func freeLocalPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func dirsTestHTTPClient(t *testing.T, caFile string) *http.Client {
	t.Helper()
	pemBytes, err := os.ReadFile(caFile)
	if err != nil {
		t.Fatalf("read CA file %s: %v", caFile, err)
	}
	pool := x509.NewCertPool()
	for {
		var block *pem.Block
		block, pemBytes = pem.Decode(pemBytes)
		if block == nil {
			break
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatalf("parse cert in %s: %v", caFile, err)
		}
		pool.AddCert(cert)
	}
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool},
		},
	}
}

func pollMISReady(t *testing.T, client *http.Client, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("MIS did not become ready at %s within %v", url, timeout)
}
