//go:build e2e

package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDaemon_FullLifecycle exercises the device-agent daemon's full MIAF
// lifecycle: enroll ==> background renewal ==> JWT exchange ==> self-revocation ==> SIGTERM.
//
// Requires the `daemon-test` compose profile (MIS_SVID_TTL=2m) and runs
// device-agent daemon with --revocations-every=10s so revocation is detected
// quickly. Run via `make e2e-daemon`.
func TestDaemon_FullLifecycle(t *testing.T) {
	requireDaemonE2E(t)

	outDir := t.TempDir()
	svidCert := filepath.Join(outDir, "svid.pem")
	svidKey := filepath.Join(outDir, "svid.key")
	stateFile := filepath.Join(outDir, "state.json")
	socketPath := daemonSocketPath()

	// 1. Enroll once with factory cert mTLS.
	enrollOnce(t, svidCert, svidKey, stateFile)

	enrolled := readState(t, stateFile)
	originalSerial := enrolled.Serial
	originalSPIFFEID := enrolled.SPIFFEID

	// 2. Start daemon in background. MIS_SVID_TTL=2m, ratio=0.5 ==> renewal fires ~1m in.
	stop := startDaemon(t, stateFile, svidCert, svidKey, socketPath)

	// 3. Wait for renewal. Budget: 2m TTL / 2 ratio + 3m buffer = 4m.
	t.Log("waiting for renewal (up to 4m)...")
	waitForSerialChange(t, stateFile, originalSerial, 4*time.Minute)
	t.Logf("renewal observed: serial changed from %s", originalSerial)

	// 4. On-demand JWT exchange via ctl.
	out, err := runCtl(t, socketPath, "exchange-jwt", "--audience", "https://api.example.com")
	if err != nil {
		t.Fatalf("ctl exchange-jwt failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"JWT"`) {
		t.Fatalf("ctl exchange-jwt output did not contain JWT field:\n%s", out)
	}
	t.Log("JWT exchange succeeded")

	// 5. Revoke the daemon's current SVID via the MIS admin API.
	revokeViaAdmin(t, originalSPIFFEID)
	t.Logf("revoked SPIFFE ID %s via admin API", originalSPIFFEID)

	// 6. Wait for daemon to observe its own revocation (revocations-every=10s).
	// Budget is generous: a concurrent renewal attempt has a 30s HTTP timeout
	// and can delay the revocations check by that much.
	t.Log("waiting for self-revocation flag (up to 3m)...")
	waitForRevokedFlag(t, stateFile, 3*time.Minute)
	t.Log("RevokedAt flag set - daemon self-revoked")

	// 7. Send SIGTERM; verify state file persists RevokedAt across shutdown.
	stop()
	st := readState(t, stateFile)
	if st.RevokedAt.IsZero() {
		t.Errorf("RevokedAt should persist across SIGTERM; got zero")
	}
}
