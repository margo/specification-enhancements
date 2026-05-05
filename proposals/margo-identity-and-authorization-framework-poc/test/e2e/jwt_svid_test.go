//go:build e2e

package e2e_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/margo/miaf-poc/pkg/device-agent/bundle"
	"github.com/margo/miaf-poc/pkg/device-agent/discovery"
	"github.com/margo/miaf-poc/pkg/device-agent/httpclient"
	jwtverify "github.com/margo/miaf-poc/pkg/device-agent/jwtsvid/verify"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// TestDeviceAgent_JWTExchange enrolls a device, runs `jwt-exchange`, and
// verifies the returned JWT SVID against the Bundle Map.
func TestDeviceAgent_JWTExchange(t *testing.T) {
	agent := filepath.Join("..", "..", "bin", "device-agent")
	if _, err := os.Stat(agent); err != nil {
		t.Fatalf("binary missing - run `make build` first: %v", err)
	}

	outDir := t.TempDir()
	svidCert := filepath.Join(outDir, "svid.pem")
	svidKey := filepath.Join(outDir, "svid.key")
	stateFile := filepath.Join(outDir, "state.json")

	enroll := []string{
		"--mis-base-url", misBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", misTrustAnchor),
		"enroll", "factory-cert-mtls",
		"--factory-cert", filepath.Join("..", "..", factoryCert),
		"--factory-key", filepath.Join("..", "..", factoryKey),
		"--svid-cert", svidCert,
		"--svid-key", svidKey,
		"--state-file", stateFile,
	}
	if out, err := exec.Command(agent, enroll...).CombinedOutput(); err != nil {
		t.Fatalf("enroll failed: %v\n%s", err, out)
	}

	tokenFile := filepath.Join(outDir, "token.jwt")
	exchange := []string{
		"--mis-base-url", misBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", misTrustAnchor),
		"jwt-exchange",
		"--svid-cert", svidCert,
		"--svid-key", svidKey,
		"--state-file", stateFile,
		"--audience", "https://dfm.example/",
		"--out", tokenFile,
	}
	if out, err := exec.Command(agent, exchange...).CombinedOutput(); err != nil {
		t.Fatalf("jwt-exchange failed: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatalf("read %s: %v", tokenFile, err)
	}
	compact := strings.TrimSpace(string(raw))
	if compact == "" {
		t.Fatal("token file is empty")
	}

	// Verify against the Bundle Map.
	httpc, err := httpclient.New(filepath.Join("..", "..", misTrustAnchor))
	if err != nil {
		t.Fatalf("httpclient: %v", err)
	}
	doc, err := discovery.Fetch(context.Background(), httpc, misBaseURL)
	if err != nil {
		t.Fatalf("discovery: %v", err)
	}
	res, err := bundle.Fetch(context.Background(), httpc, doc.TrustBundleURI, "")
	if err != nil {
		t.Fatalf("bundle fetch: %v", err)
	}
	td, err := spiffeid.TrustDomainFromString(doc.TrustDomain)
	if err != nil {
		t.Fatalf("TrustDomainFromString: %v", err)
	}
	src, err := jwtverify.SourceFromBundleMap(res.Body, td)
	if err != nil {
		t.Fatalf("SourceFromBundleMap: %v", err)
	}
	svid, err := jwtverify.ParseAndValidate(context.Background(), src, []string{"https://dfm.example/"}, compact)
	if err != nil {
		t.Fatalf("ParseAndValidate: %v", err)
	}
	if !svid.ID.MemberOf(td) {
		t.Errorf("issued sub %s is not in trust domain %s", svid.ID, td)
	}
}
