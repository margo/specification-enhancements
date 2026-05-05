package identitymanager_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/device-agent/identitymanager"
	"github.com/margo/miaf-poc/pkg/device-agent/agentstate"
)

func TestHandleRefreshBundle_HappyPath(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", "Mon, 27 Apr 2026 12:00:00 GMT")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"trustDomains":{}}`))
	}))
	defer srv.Close()

	certPath, keyPath, _, _ := mintDeviceCert(t)
	caPath := tlsServerCA(t, srv)
	stateFile := seedState(t, "spiffe://td.example/margo/device/test")

	mgr := identitymanager.NewManager(identitymanager.Config{
		Clock:          clockReal{},
		StateFile:      stateFile,
		SVIDCertFile:   certPath,
		SVIDKeyFile:    keyPath,
		MISBaseURL:     srv.URL,
		MISTrustAnchor: caPath,
		BundleMapURL:   srv.URL + "/bundle",
		RevocationsURL: srv.URL + "/revocations",
		Logger:         discardLogger(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	reply := make(chan identitymanager.RefreshBundleReply, 1)
	mgr.Submit(identitymanager.Cmd{
		Ctx:     ctx,
		Refresh: &identitymanager.RefreshBundleCmd{Reply: reply},
	})
	r := <-reply
	if r.Err != nil {
		t.Fatalf("RefreshBundle: %v", r.Err)
	}
	if r.BundleLastModif != "Mon, 27 Apr 2026 12:00:00 GMT" {
		t.Errorf("BundleLastModif = %q", r.BundleLastModif)
	}

	// Verify state was persisted
	st, _ := agentstate.Load(stateFile)
	if st.BundleLastModified != "Mon, 27 Apr 2026 12:00:00 GMT" {
		t.Errorf("state.BundleLastModified = %q", st.BundleLastModified)
	}
}
