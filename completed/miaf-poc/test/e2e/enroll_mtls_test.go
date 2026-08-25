//go:build e2e

package e2e_test

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	misBaseURL     = "https://localhost:8443"
	misTrustAnchor = "var/dev/mis-etc/tls/server.crt"
	factoryCert    = "var/dev/mis-etc/factory/device-01.crt"
	factoryKey     = "var/dev/mis-etc/factory/device-01.key"
	factoryCert2   = "var/dev/mis-etc/factory/device-02.crt"
	factoryKey2    = "var/dev/mis-etc/factory/device-02.key"
)

// TestDeviceAgent_EnrollFactoryCertMTLS drives the compiled device-agent
// binary against the live compose stack.  The MIS must be configured with
// the same factory root (mounted at /etc/mis/factory/root.crt).
func TestDeviceAgent_EnrollFactoryCertMTLS(t *testing.T) {
	agent := filepath.Join("..", "..", "bin", "device-agent")
	if _, err := os.Stat(agent); err != nil {
		t.Fatalf("binary missing - run `make build` first: %v", err)
	}

	outDir := t.TempDir()
	svidCert := filepath.Join(outDir, "svid.pem")
	svidKey := filepath.Join(outDir, "svid.key")
	stateFile := filepath.Join(outDir, "state.json")

	args := []string{
		"--mis-base-url", misBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", misTrustAnchor),
		"enroll", "factory-cert-mtls",
		"--factory-cert", filepath.Join("..", "..", factoryCert),
		"--factory-key", filepath.Join("..", "..", factoryKey),
		"--svid-cert", svidCert,
		"--svid-key", svidKey,
		"--state-file", stateFile,
	}
	cmd := exec.Command(agent, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("device-agent enroll failed: %v\n%s", err, string(out))
	}

	chainPEM, err := os.ReadFile(svidCert)
	if err != nil {
		t.Fatalf("read %s: %v", svidCert, err)
	}
	block, _ := pem.Decode(chainPEM)
	if block == nil {
		t.Fatalf("leaf PEM not found in %s", svidCert)
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	if len(leaf.URIs) != 1 {
		t.Fatalf("URI SAN count = %d, want 1", len(leaf.URIs))
	}
	spiffeID := leaf.URIs[0].String()
	if !strings.HasPrefix(spiffeID, "spiffe://margo.example.com/margo/device/") {
		t.Errorf("SPIFFE ID = %q", spiffeID)
	}

	// Second enrollment with the same factory cert must succeed (200 re-enroll).
	cmd = exec.Command(agent, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("re-enrollment failed: %v\n%s", err, string(out))
	}

	if _, err := os.Stat(svidKey); err != nil {
		t.Fatalf("svid key missing: %v", err)
	}
	if _, err := os.Stat(stateFile); err != nil {
		t.Fatalf("state file missing: %v", err)
	}
}
