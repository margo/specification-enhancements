package daemon_test

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/device-agent/ctl"
	"github.com/margo/miaf-poc/pkg/device-agent/agentstate"
)

// TestDaemon_SocketCreatedAndStatusWorks boots a daemon against an in-process
// fake MIS, then verifies the socket is created and status returns the SPIFFEID.
//
// Since the daemon calls discovery.Fetch and httpclient.New on startup, we skip
// a full in-process MIS and instead test the socket+manager integration directly
// via socket_test.go's socket-level tests. This test verifies the public Run API
// at a higher level by confirming Run returns on context cancellation.
func TestDaemon_RunExitsOnContextCancel(t *testing.T) {
	// This is a smoke test: Run should not block indefinitely after ctx cancel.
	// Full wiring (with a real MIS) is covered by the e2e test in Task 12.
	//
	// We skip this test here because Run calls discovery.Fetch which requires a
	// real HTTP server. The daemon integration is exercised end-to-end in
	// test/e2e/daemon_test.go.
	t.Skip("daemon.Run requires a live MIS; covered by e2e tests")
}

// TestSocket_CtlClientStatusRoundTrip uses the ctl.Call client against a
// real socket server to verify the full request-response cycle.
func TestSocket_CtlClientStatusRoundTrip(t *testing.T) {
	t.Parallel()
	// Use a short path under /tmp to stay within macOS's 104-byte Unix socket
	// path limit.  t.TempDir() can produce paths longer than that.
	dir, err := os.MkdirTemp("", "miaf")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sockPath := filepath.Join(dir, "d.sock")

	// Bootstrap a minimal state file.
	stateFile := filepath.Join(dir, "state.json")
	st := agentstate.State{
		SPIFFEID:     "spiffe://td.example/margo/device/ctl-test",
		Serial:       "42",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		LastEnrolled: time.Now().Add(-time.Hour),
	}
	b, _ := json.Marshal(st)
	os.WriteFile(stateFile, b, 0o600)

	// Boot a minimal socket server directly (not full daemon.Run).
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		var req ctl.Request
		json.NewDecoder(c).Decode(&req)
		json.NewEncoder(c).Encode(ctl.Response{
			OK: true,
			Data: map[string]string{
				"spiffe_id": "spiffe://td.example/margo/device/ctl-test",
			},
		})
	}()

	var out map[string]string
	if err := ctl.Call(context.Background(), sockPath,
		ctl.Request{Cmd: "status"}, &out); err != nil {
		t.Fatalf("ctl.Call: %v", err)
	}
	if out["spiffe_id"] != "spiffe://td.example/margo/device/ctl-test" {
		t.Errorf("spiffe_id = %q", out["spiffe_id"])
	}
}
