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
	jwtcache "github.com/margo/miaf-poc/pkg/device-agent/jwtsvid/cache"
)

func TestHandleExchangeJWT_HappyPath(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := common.JWTSVIDExchangeResponseDTO{
			JWT:       "eyJ.fake.jwt",
			ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	certPath, keyPath, _, _ := mintDeviceCert(t)
	caPath := tlsServerCA(t, srv)
	stateFile := seedState(t, "spiffe://td.example/margo/device/test")
	jwtCache := jwtcache.New(5 * time.Minute)

	mgr := identitymanager.NewManager(identitymanager.Config{
		Clock:          clockReal{},
		StateFile:      stateFile,
		SVIDCertFile:   certPath,
		SVIDKeyFile:    keyPath,
		MISBaseURL:     srv.URL,
		MISTrustAnchor: caPath,
		JWTCache:       jwtCache,
		Logger:         discardLogger(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	reply := make(chan identitymanager.ExchangeJWTReply, 1)
	mgr.Submit(identitymanager.Cmd{
		Ctx: ctx,
		JWT: &identitymanager.ExchangeJWTCmd{
			Audiences: []string{"https://api.example.com"},
			TTL:       5 * time.Minute,
			Reply:     reply,
		},
	})
	r := <-reply
	if r.Err != nil {
		t.Fatalf("ExchangeJWT: %v", r.Err)
	}
	if r.JWT != "eyJ.fake.jwt" {
		t.Errorf("JWT = %q", r.JWT)
	}
	if r.FromCache {
		t.Error("first exchange should not come from cache")
	}
	if jwtCache.Len() != 1 {
		t.Errorf("cache size = %d, want 1", jwtCache.Len())
	}
}
