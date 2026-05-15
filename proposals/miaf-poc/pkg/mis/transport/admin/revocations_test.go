package admin_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/store"
	"github.com/margo/miaf-poc/pkg/mis/transport/admin"
)

// revokeSPIFFEIDFunc adapts a function into the admin.RevocationOperator port.
type revokeSPIFFEIDFunc func(ctx context.Context, spiffeID, reason string) ([]identity.RevocationEntry, []string, error)

func (f revokeSPIFFEIDFunc) RevokeSPIFFEID(ctx context.Context, spiffeID, reason string) ([]identity.RevocationEntry, []string, error) {
	return f(ctx, spiffeID, reason)
}

func TestAdmin_CreateRevocation_OK(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("store open: %v", err)
	}
	defer s.Close()

	srv := admin.NewServer(admin.Config{
		EnrollmentTokens:   s.EnrollmentTokens(),
		ReplacementTickets: s.ReplacementTickets(),
		Revocations: revokeSPIFFEIDFunc(func(_ context.Context, sid, reason string) ([]identity.RevocationEntry, []string, error) {
			if sid != "spiffe://td/margo/device/abc" {
				t.Errorf("spiffe_id = %q", sid)
			}
			if reason != "compromise" {
				t.Errorf("reason = %q", reason)
			}
			return []identity.RevocationEntry{{SerialHex: "AA"}}, []string{"BB"}, nil
		}),
		DefaultTokenTTL: time.Minute,
	})

	body, _ := json.Marshal(common.RevokeSPIFFEIDRequest{
		SPIFFEID: "spiffe://td/margo/device/abc",
		Reason:   "compromise",
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/revocations", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var resp common.RevokeSPIFFEIDResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.SPIFFEID != "spiffe://td/margo/device/abc" {
		t.Errorf("spiffe_id = %q", resp.SPIFFEID)
	}
	if len(resp.RevokedSerials) != 1 || resp.RevokedSerials[0] != "AA" {
		t.Errorf("revoked = %+v", resp.RevokedSerials)
	}
	if len(resp.AlreadyRevoked) != 1 || resp.AlreadyRevoked[0] != "BB" {
		t.Errorf("already = %+v", resp.AlreadyRevoked)
	}
}

func TestAdmin_CreateRevocation_UnknownSPIFFEID(t *testing.T) {
	s, _ := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	defer s.Close()

	srv := admin.NewServer(admin.Config{
		EnrollmentTokens:   s.EnrollmentTokens(),
		ReplacementTickets: s.ReplacementTickets(),
		Revocations: revokeSPIFFEIDFunc(func(_ context.Context, _, _ string) ([]identity.RevocationEntry, []string, error) {
			return nil, nil, identity.ErrNotFound
		}),
		DefaultTokenTTL: time.Minute,
	})

	body, _ := json.Marshal(common.RevokeSPIFFEIDRequest{SPIFFEID: "spiffe://td/margo/device/no-such"})
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/revocations", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestAdmin_CreateRevocation_BadBody(t *testing.T) {
	s, _ := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	defer s.Close()
	srv := admin.NewServer(admin.Config{
		EnrollmentTokens:   s.EnrollmentTokens(),
		ReplacementTickets: s.ReplacementTickets(),
		Revocations: revokeSPIFFEIDFunc(func(_ context.Context, _, _ string) ([]identity.RevocationEntry, []string, error) {
			t.Fatal("handler should not call RevokeSPIFFEID when spiffe_id is empty")
			return nil, nil, nil
		}),
		DefaultTokenTTL: time.Minute,
	})

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/revocations", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
