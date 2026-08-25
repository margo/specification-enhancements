//go:build e2e

package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDeviceAgent_CheckRevocations drives:
//
//	docker exec mis mis generate-token    ==> single-use enrollment token
//	device-agent enroll enrollment-token  ==> fresh LDI, SVID written to svid.pem
//	docker exec mis mis revoke ...        ==> admin API revokes identity (loopback-only)
//	device-agent check-revocations        ==> exit code 10 (REVOKED)
//
// Uses enrollment tokens (not factory certs) so that each test run mints a
// fresh LDI.  Revoking that LDI does not leave factory-cert ESIs in a revoked
// state that would break the other E2E tests on the next run.
//
// The admin listener is intentionally loopback-only inside the container (not
// published).  Both admin calls run via `docker exec mis`.
func TestDeviceAgent_CheckRevocations(t *testing.T) {
	deviceAgent := filepath.Join("..", "..", "bin", "device-agent")
	if _, err := os.Stat(deviceAgent); err != nil {
		t.Fatalf("device-agent binary missing - run `make build` first: %v", err)
	}

	outDir := t.TempDir()
	svidCert := filepath.Join(outDir, "svid.pem")
	svidKey := filepath.Join(outDir, "svid.key")
	stateFile := filepath.Join(outDir, "state.json")

	// 1. Generate a single-use token and enroll the device.
	genOut, err := exec.Command("docker", "exec", "mis", "mis", "generate-token", "--ttl", "5m").CombinedOutput()
	if err != nil {
		t.Fatalf("generate-token: %v\n%s", err, genOut)
	}
	token := extractSecret(t, string(genOut))

	enrollOut, err := exec.Command(deviceAgent,
		"--mis-base-url", misBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", misTrustAnchor),
		"enroll", "enrollment-token",
		"--token", token,
		"--svid-cert", svidCert,
		"--svid-key", svidKey,
		"--state-file", stateFile,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("enroll failed: %v\n%s", err, enrollOut)
	}

	// Extract the device SPIFFE ID from the enroll output.
	spiffeID := extractSPIFFEID(t, string(enrollOut))

	// 2. Revoke via `mis revoke` inside the container.
	revokeOut, err := exec.Command("docker", "exec", "mis",
		"mis", "revoke", "--spiffe-id", spiffeID, "--reason", "e2e-test",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("mis revoke failed: %v\n%s", err, revokeOut)
	}

	// 3. check-revocations ==> exit code 10.
	checkCmd := exec.Command(deviceAgent,
		"--mis-base-url", misBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", misTrustAnchor),
		"check-revocations",
		"--svid-cert", svidCert,
	)
	checkOut, err := checkCmd.CombinedOutput()
	if err == nil {
		t.Fatalf("check-revocations exited 0 but SVID was revoked:\n%s", checkOut)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("unexpected error type: %v", err)
	}
	if code := exitErr.ExitCode(); code != 10 {
		t.Errorf("check-revocations exit code = %d, want 10:\n%s", code, checkOut)
	}
	if !strings.Contains(string(checkOut), "STATUS: REVOKED") {
		t.Errorf("expected 'STATUS: REVOKED' in output, got:\n%s", checkOut)
	}

	// 4. Sanity: a freshly enrolled device (different token) should be CLEAN.
	genOut2, err := exec.Command("docker", "exec", "mis", "mis", "generate-token", "--ttl", "5m").CombinedOutput()
	if err != nil {
		t.Fatalf("generate-token (second): %v\n%s", err, genOut2)
	}
	token2 := extractSecret(t, string(genOut2))

	secondCert := filepath.Join(outDir, "svid2.pem")
	secondKey := filepath.Join(outDir, "svid2.key")
	secondState := filepath.Join(outDir, "state2.json")
	if out, err := exec.Command(deviceAgent,
		"--mis-base-url", misBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", misTrustAnchor),
		"enroll", "enrollment-token",
		"--token", token2,
		"--svid-cert", secondCert,
		"--svid-key", secondKey,
		"--state-file", secondState,
	).CombinedOutput(); err != nil {
		t.Fatalf("second enroll failed: %v\n%s", err, out)
	}

	cleanOut, err := exec.Command(deviceAgent,
		"--mis-base-url", misBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", misTrustAnchor),
		"check-revocations",
		"--svid-cert", secondCert,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("second check-revocations failed: %v\n%s", err, cleanOut)
	}
	if !strings.Contains(string(cleanOut), "STATUS: CLEAN") {
		t.Errorf("expected 'STATUS: CLEAN' for fresh device, got:\n%s", cleanOut)
	}
}

func extractSPIFFEID(t *testing.T, enrollOutput string) string {
	t.Helper()
	for _, line := range strings.Split(enrollOutput, "\n") {
		if idx := strings.Index(line, "spiffe://"); idx >= 0 {
			rest := line[idx:]
			end := strings.IndexAny(rest, " \t\n")
			if end > 0 {
				return rest[:end]
			}
			return rest
		}
	}
	t.Fatalf("could not find SPIFFE ID in enroll output:\n%s", enrollOutput)
	return ""
}
