package identitymanager_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/device-agent/identitymanager"
)

func TestHandleCheckRevocations_HappyPathClean(t *testing.T) {
	t.Parallel()
	list := common.RevocationListDTO{
		LastUpdated:  "2026-04-27T00:00:00Z",
		RevokedSVIDs: []common.RevokedSVIDEntry{},
	}
	raw, _ := json.Marshal(list)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `W/"abc"`)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(raw)
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
		RevocationsURL: srv.URL + "/api/v1/revocations",
		Logger:         discardLogger(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	reply := make(chan identitymanager.CheckRevocationsReply, 1)
	mgr.Submit(identitymanager.Cmd{
		Ctx:   ctx,
		Check: &identitymanager.CheckRevocationsCmd{Reply: reply},
	})
	r := <-reply
	if r.Err != nil {
		t.Fatalf("CheckRevocations: %v", r.Err)
	}
	if r.OwnSVIDListed {
		t.Error("OwnSVIDListed should be false for empty list")
	}
	if r.ETag != `W/"abc"` {
		t.Errorf("ETag = %q", r.ETag)
	}
}
