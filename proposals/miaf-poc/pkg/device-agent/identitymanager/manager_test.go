package identitymanager_test

import (
	"context"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/device-agent/identitymanager"
)

func TestManager_StatusReturnsSeedState(t *testing.T) {
	t.Parallel()
	certPath, keyPath, _, _ := mintDeviceCert(t)
	stateFile := seedState(t, "spiffe://td.example/margo/device/test")

	// For status, we don't need a real MIS - use a dummy URL (not called).
	mgr := identitymanager.NewManager(identitymanager.Config{
		Clock:          clockReal{},
		StateFile:      stateFile,
		SVIDCertFile:   certPath,
		SVIDKeyFile:    keyPath,
		MISBaseURL:     "https://dummy.invalid",
		MISTrustAnchor: certPath, // unused for status
		Logger:         discardLogger(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Run(ctx)
	time.Sleep(10 * time.Millisecond) // let goroutine start

	reply := make(chan identitymanager.StatusReply, 1)
	mgr.Submit(identitymanager.Cmd{
		Ctx:    ctx,
		Status: &identitymanager.StatusCmd{Reply: reply},
	})
	r := <-reply
	if r.SPIFFEID != "spiffe://td.example/margo/device/test" {
		t.Errorf("SPIFFEID = %q", r.SPIFFEID)
	}
}

func TestManager_StopDrainsGracefully(t *testing.T) {
	t.Parallel()
	certPath, keyPath, _, _ := mintDeviceCert(t)
	stateFile := seedState(t, "spiffe://td.example/margo/device/x")

	mgr := identitymanager.NewManager(identitymanager.Config{
		Clock:          clockReal{},
		StateFile:      stateFile,
		SVIDCertFile:   certPath,
		SVIDKeyFile:    keyPath,
		MISBaseURL:     "https://dummy.invalid",
		MISTrustAnchor: certPath,
		Logger:         discardLogger(),
	})

	ctx := context.Background()
	done := mgr.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	stopReply := make(chan struct{})
	mgr.Submit(identitymanager.Cmd{
		Ctx:  ctx,
		Stop: &identitymanager.StopCmd{Reply: stopReply},
	})
	select {
	case <-stopReply:
	case <-time.After(2 * time.Second):
		t.Fatal("StopCmd did not drain within 2s")
	}
	<-done
}
