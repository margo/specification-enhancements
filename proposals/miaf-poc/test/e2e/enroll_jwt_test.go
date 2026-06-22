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

// TestDeviceAgent_EnrollFactoryCertJWT drives the device-agent binary through
// the application-layer Bootstrap Assertion flow against the live compose
// stack. MIS's factory trust pool is shared with factory-cert-mtls; the same
// device-01 ECDSA leaf is reused for the JWT method.
func TestDeviceAgent_EnrollFactoryCertJWT(t *testing.T) {
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
		"enroll", "factory-cert-jwt",
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
	// The second JWT assertion will have a fresh jti so replay-protection
	// does not block it.
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
