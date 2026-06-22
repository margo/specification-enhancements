//go:build e2e

package e2e_test

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	interopServerAddr  = "localhost:8444"
	interopTrustDomain = "margo.example.com"
)

// TestSpiffeInterop enrolls a factory device against MIS-RA, then dials the
// SPIRE-attested workload over mTLS and asserts the two peer SPIFFE IDs.
//
// Prerequisites: `make up-interop` has started the stack and `make build` has
// produced ./bin/device-agent.
func TestSpiffeInterop(t *testing.T) {
	agent := filepath.Join("..", "..", "bin", "device-agent")
	if _, err := os.Stat(agent); err != nil {
		t.Fatalf("binary missing - run `make build` first: %v", err)
	}

	if c, err := net.DialTimeout("tcp", interopServerAddr, time.Second); err != nil {
		t.Skipf("interop stack not reachable at %s - run `make up-interop` or `make up-spire` first", interopServerAddr)
	} else {
		c.Close()
	}

	outDir := t.TempDir()
	svidCert := filepath.Join(outDir, "svid.pem")
	svidKey := filepath.Join(outDir, "svid.key")
	stateFile := filepath.Join(outDir, "state.json")

	enrollArgs := []string{
		"--mis-base-url", misBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", misTrustAnchor),
		"enroll", "factory-cert-mtls",
		"--factory-cert", filepath.Join("..", "..", factoryCert),
		"--factory-key", filepath.Join("..", "..", factoryKey),
		"--svid-cert", svidCert,
		"--svid-key", svidKey,
		"--state-file", stateFile,
	}
	if out, err := exec.Command(agent, enrollArgs...).CombinedOutput(); err != nil {
		t.Fatalf("enroll failed: %v\n%s", err, string(out))
	}

	interopArgs := []string{
		"--mis-base-url", misBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", misTrustAnchor),
		"interop-call",
		"--server", interopServerAddr,
		"--trust-domain", interopTrustDomain,
		"--svid-cert", svidCert,
		"--svid-key", svidKey,
	}
	out, err := exec.Command(agent, interopArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("interop-call failed: %v\n%s", err, string(out))
	}
	output := string(out)

	if !strings.Contains(output, "Device SPIFFE ID: spiffe://margo.example.com/margo/device/") {
		t.Errorf("device SPIFFE ID line missing or wrong prefix:\n%s", output)
	}
	wantWorkload := "spiffe://margo.example.com/margo/spire-workload/echo"
	if !strings.Contains(output, "Workload SPIFFE ID (from peer cert): "+wantWorkload) {
		t.Errorf("peer-cert workload ID wrong:\n%s", output)
	}
	if !strings.Contains(output, "Workload's self-reported SPIFFE ID: "+wantWorkload) {
		t.Errorf("self-reported workload ID wrong:\n%s", output)
	}
}
