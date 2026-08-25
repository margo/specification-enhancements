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

// TestDeviceAgent_EnrollEnrollmentToken drives the device-agent binary
// through the enrollment-token flow against the live compose stack. The
// token is generated via the running MIS's admin subcommand to exercise
// the operator ==> MIS ==> device hand-off end-to-end.
func TestDeviceAgent_EnrollEnrollmentToken(t *testing.T) {
	agent := filepath.Join("..", "..", "bin", "device-agent")
	if _, err := os.Stat(agent); err != nil {
		t.Fatalf("binary missing - run `make build` first: %v", err)
	}

	// Generate a token inside the running MIS container and capture the secret.
	// Use "docker exec" (container name) rather than "docker compose exec" (service
	// name) so this works with both the intermediate and interop compose profiles -
	// both name their MIS container "mis" via container_name.
	gen := exec.Command("docker", "exec", "mis", "mis", "generate-token", "--ttl", "5m")
	rawOut, err := gen.CombinedOutput()
	if err != nil {
		t.Fatalf("generate-token: %v\n%s", err, string(rawOut))
	}
	token := extractSecret(t, string(rawOut))

	outDir := t.TempDir()
	svidCert := filepath.Join(outDir, "svid.pem")
	svidKey := filepath.Join(outDir, "svid.key")
	stateFile := filepath.Join(outDir, "state.json")

	args := []string{
		"--mis-base-url", misBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", misTrustAnchor),
		"enroll", "enrollment-token",
		"--token", token,
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
}

// extractSecret searches all output lines of `mis generate-token` for the
// `secret:` key and returns the trimmed value.
func extractSecret(t *testing.T, blob string) string {
	t.Helper()
	for _, line := range strings.Split(blob, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "secret:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				t.Fatalf("malformed line %q", line)
			}
			return strings.TrimSpace(parts[1])
		}
	}
	t.Fatalf("no secret line in output:\n%s", blob)
	return ""
}
