package daemon

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/device-agent/clock"
	"github.com/margo/miaf-poc/pkg/device-agent/identitymanager"
	"github.com/margo/miaf-poc/pkg/device-agent/agentstate"
)

// newTestManager creates a Manager loaded from a minimal state file.
// dir is used for temp files.
func newTestManager(t *testing.T) (*identitymanager.Manager, string) {
	t.Helper()
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")
	certFile := filepath.Join(dir, "svid.pem")
	keyFile := filepath.Join(dir, "svid.key")

	// Write a minimal state file.
	st := agentstate.State{
		SPIFFEID:     "spiffe://td.example/margo/device/test",
		Serial:       "1",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		LastEnrolled: time.Now().Add(-time.Hour),
	}
	b, _ := json.Marshal(st)
	os.WriteFile(stateFile, b, 0o600)
	// Write dummy cert+key (won't be dialed in socket tests).
	os.WriteFile(certFile, []byte("placeholder"), 0o600)
	os.WriteFile(keyFile, []byte("placeholder"), 0o600)

	mgr := identitymanager.NewManager(identitymanager.Config{
		Clock:          clock.Real{},
		StateFile:      stateFile,
		SVIDCertFile:   certFile,
		SVIDKeyFile:    keyFile,
		MISBaseURL:     "https://dummy.invalid",
		MISTrustAnchor: certFile, // unused in these tests
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return mgr, dir
}

func TestSocket_UnknownCommand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")
	mgr, _ := newTestManager(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	srv := newSocketServer(sockPath, mgr, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer srv.Close()

	// Check socket file permissions.
	info, err := os.Stat(sockPath)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("socket mode = %o, want 600", mode)
	}

	// Send unknown command.
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	json.NewEncoder(conn).Encode(map[string]string{"cmd": "bogus"})
	var resp map[string]interface{}
	json.NewDecoder(conn).Decode(&resp)
	if ok, _ := resp["ok"].(bool); ok {
		t.Errorf("expected ok=false for unknown cmd, got ok=true")
	}
}

func TestSocket_StatusCommand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")
	mgr, _ := newTestManager(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	srv := newSocketServer(sockPath, mgr, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer srv.Close()

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	json.NewEncoder(conn).Encode(map[string]string{"cmd": "status"})
	var resp map[string]interface{}
	json.NewDecoder(conn).Decode(&resp)
	if ok, _ := resp["ok"].(bool); !ok {
		t.Errorf("expected ok=true for status, got %v", resp)
	}
}
