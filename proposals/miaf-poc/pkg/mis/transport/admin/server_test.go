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
	"github.com/margo/miaf-poc/pkg/mis/store"
	"github.com/margo/miaf-poc/pkg/mis/transport/admin"
)

func newServer(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	h := admin.NewServer(admin.Config{
		EnrollmentTokens:   s.EnrollmentTokens(),
		ReplacementTickets: s.ReplacementTickets(),
		DefaultTokenTTL:    time.Hour,
	}).Handler()
	return h, s
}

func TestAdmin_CreateEnrollmentToken_Created(t *testing.T) {
	h, _ := newServer(t)
	body, _ := json.Marshal(common.CreateEnrollmentTokenRequest{TTL: "10m"})
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/enrollment-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var resp common.CreateEnrollmentTokenResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.TokenID == "" || resp.Secret == "" || resp.ExpiresAt == "" {
		t.Errorf("empty fields: %+v", resp)
	}
}

func TestAdmin_CreateEnrollmentToken_DefaultTTL(t *testing.T) {
	h, _ := newServer(t)
	body, _ := json.Marshal(common.CreateEnrollmentTokenRequest{}) // no TTL
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/enrollment-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
}

func TestAdmin_CreateEnrollmentToken_BadTTL(t *testing.T) {
	h, _ := newServer(t)
	body, _ := json.Marshal(common.CreateEnrollmentTokenRequest{TTL: "not-a-duration"})
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/enrollment-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestAdmin_CreateReplacementTicket_Created(t *testing.T) {
	h, s := newServer(t)
	ldi, err := s.Identities().CreateLDIAndBind(context.Background(),
		[]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), "urn:x:y", "margo.example.com")
	if err != nil {
		t.Fatalf("seed LDI: %v", err)
	}
	body, _ := json.Marshal(common.CreateReplacementTicketRequest{
		SPIFFEID: ldi.SPIFFEID, TTL: "1h",
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/replacement-tickets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var resp common.CreateReplacementTicketResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Ticket == "" || resp.SPIFFEID != ldi.SPIFFEID {
		t.Errorf("bad response: %+v", resp)
	}
}

func TestAdmin_CreateReplacementTicket_UnknownLDI(t *testing.T) {
	h, _ := newServer(t)
	body, _ := json.Marshal(common.CreateReplacementTicketRequest{
		SPIFFEID: "spiffe://margo.example.com/margo/device/00000000-0000-0000-0000-000000000000", TTL: "1h",
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/replacement-tickets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestAdmin_UnknownRoute(t *testing.T) {
	h, _ := newServer(t)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/admin/v1/nope", nil))
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}
