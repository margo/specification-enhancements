//go:build e2e

package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/device-agent/agentstate"
)

// requireDaemonE2E skips the test unless the MIAF_E2E_DAEMON env var is set.
// Set by `make e2e-daemon`, which also starts the daemon-test compose profile.
func requireDaemonE2E(t *testing.T) {
	t.Helper()
	if os.Getenv("MIAF_E2E_DAEMON") == "" {
		t.Skip("set MIAF_E2E_DAEMON=1 (or run `make e2e-daemon`) to run daemon e2e tests")
	}
}

// daemonSocketPath returns a short Unix socket path safe for macOS's 104-byte limit.
func daemonSocketPath() string {
	return fmt.Sprintf("/tmp/miafe2e%d.sock", os.Getpid())
}

// enrollOnce enrolls the device using factory-cert-mtls.
func enrollOnce(t *testing.T, svidCert, svidKey, stateFile string) {
	t.Helper()
	agent := filepath.Join("..", "..", "bin", "device-agent")
	out, err := exec.Command(agent,
		"--mis-base-url", misBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", misTrustAnchor),
		"enroll", "factory-cert-mtls",
		"--factory-cert", filepath.Join("..", "..", factoryCert),
		"--factory-key", filepath.Join("..", "..", factoryKey),
		"--svid-cert", svidCert,
		"--svid-key", svidKey,
		"--state-file", stateFile,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("enroll failed: %v\n%s", err, out)
	}
}

// startDaemon starts device-agent daemon in the background. Returns a stop function.
// The stop function sends SIGTERM and waits for exit. It is also registered as
// a t.Cleanup so the daemon is always stopped when the test ends.
func startDaemon(t *testing.T, stateFile, svidCert, svidKey, socketPath string) func() {
	t.Helper()
	agent := filepath.Join("..", "..", "bin", "device-agent")

	cmd := exec.Command(agent,
		"--mis-base-url", misBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", misTrustAnchor),
		"daemon",
		"--state-file", stateFile,
		"--svid-cert", svidCert,
		"--svid-key", svidKey,
		"--socket", socketPath,
		"--bundle-every", "30s",
		"--revocations-every", "10s",
		"--renewal-ratio", "0.5",
	)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start daemon: %v", err)
	}

	// Wait for socket to appear (signals daemon is ready).
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if _, err := os.Stat(socketPath); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("daemon socket did not appear at %s within 15s", socketPath)
	}

	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		_ = cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
		_ = os.Remove(socketPath)
	}
	t.Cleanup(stop)
	return stop
}

// readState loads the state file from disk.
func readState(t *testing.T, stateFile string) agentstate.State {
	t.Helper()
	st, err := agentstate.Load(stateFile)
	if err != nil {
		t.Fatalf("read state %s: %v", stateFile, err)
	}
	return st
}

// waitForSerialChange polls the state file until the Serial field differs from
// original, or times out.
func waitForSerialChange(t *testing.T, stateFile, original string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if st := readState(t, stateFile); st.Serial != "" && st.Serial != original {
			return
		}
		time.Sleep(5 * time.Second)
	}
	t.Fatalf("serial did not change from %q within %v", original, timeout)
}

// waitForRevokedFlag polls the state file until RevokedAt is non-zero, or times out.
func waitForRevokedFlag(t *testing.T, stateFile string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if st := readState(t, stateFile); !st.RevokedAt.IsZero() {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("RevokedAt flag not set within %v", timeout)
}

// revokeViaAdmin revokes a SPIFFE ID using the MIS admin CLI inside the container.
func revokeViaAdmin(t *testing.T, spiffeID string) {
	t.Helper()
	out, err := exec.Command("docker", "exec", "mis",
		"mis", "revoke", "--spiffe-id", spiffeID, "--reason", "e2e-daemon-test",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("mis revoke failed: %v\n%s", err, out)
	}
}

// runCtl runs `device-agent ctl <socketPath> <args...>` and returns combined output.
func runCtl(t *testing.T, socketPath string, args ...string) (string, error) {
	t.Helper()
	agent := filepath.Join("..", "..", "bin", "device-agent")
	ctlArgs := append([]string{
		"--mis-base-url", misBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", misTrustAnchor),
		"ctl", "--socket", socketPath,
	}, args...)
	out, err := exec.Command(agent, ctlArgs...).CombinedOutput()
	return string(out), err
}
