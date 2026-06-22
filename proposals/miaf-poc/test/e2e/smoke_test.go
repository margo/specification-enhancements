//go:build e2e

package e2e_test

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/margo/miaf-poc/pkg/common"
)

const (
	defaultBaseURL     = "https://localhost:8443"
	defaultTrustAnchor = "var/dev/mis-etc/tls/server.crt"
)

func TestDeviceAgent_GetDiscovery(t *testing.T) {
	out := runAgent(t, "get-discovery")

	var doc common.DiscoveryResponseDTO
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("parse discovery: %v\n%s", err, out)
	}
	if doc.TrustDomain != "margo.example.com" {
		t.Errorf("trust_domain = %q, want margo.example.com", doc.TrustDomain)
	}
	if !strings.HasSuffix(doc.TrustBundleURI, "/.well-known/spiffe/bundle.json") {
		t.Errorf("trust_bundle_uri = %q", doc.TrustBundleURI)
	}
}

func TestDeviceAgent_GetBundle(t *testing.T) {
	out := runAgent(t, "get-bundle")
	// Body is printed after a "Last-Modified: ..." line - just assert the
	// trust-domain key is present.
	if !strings.Contains(out, "margo.example.com") {
		t.Errorf("bundle output did not reference margo.example.com:\n%s", out)
	}
}

func runAgent(t *testing.T, args ...string) string {
	t.Helper()
	agent := filepath.Join("..", "..", "bin", "device-agent")
	full := append([]string{
		"--mis-base-url", defaultBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", defaultTrustAnchor),
	}, args...)
	cmd := exec.Command(agent, full...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", agent, full, err, string(b))
	}
	return string(b)
}
