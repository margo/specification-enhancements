package adminclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/margo/miaf-poc/cmd/mis/internal/adminclient"
	"github.com/margo/miaf-poc/pkg/common"
)

func TestClient_CreateEnrollmentToken(t *testing.T) {
	var got common.CreateEnrollmentTokenRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/admin/v1/enrollment-tokens" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(common.CreateEnrollmentTokenResponse{
			TokenID: "tid", Secret: "margo-et-v1.AAAA", ExpiresAt: "2030-01-01T00:00:00Z",
		})
	}))
	t.Cleanup(srv.Close)

	c := adminclient.New(srv.URL)
	resp, err := c.CreateEnrollmentToken(context.Background(), 90*time.Second)
	if err != nil {
		t.Fatalf("CreateEnrollmentToken: %v", err)
	}
	if resp.TokenID != "tid" {
		t.Errorf("TokenID = %q", resp.TokenID)
	}
	if got.TTL != "1m30s" {
		t.Errorf("server saw ttl = %q, want 1m30s", got.TTL)
	}
}

func TestClient_CreateReplacementTicket(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(common.CreateReplacementTicketResponse{
			Ticket: "margo-rt-v1.BBBB", SPIFFEID: "spiffe://margo.example.com/margo/device/ldi-1", ExpiresAt: "2030-01-01T00:00:00Z",
		})
	}))
	t.Cleanup(srv.Close)

	c := adminclient.New(srv.URL)
	resp, err := c.CreateReplacementTicket(context.Background(), "spiffe://margo.example.com/margo/device/ldi-1", time.Hour)
	if err != nil {
		t.Fatalf("CreateReplacementTicket: %v", err)
	}
	if resp.Ticket != "margo-rt-v1.BBBB" {
		t.Errorf("Ticket = %q", resp.Ticket)
	}
}

func TestClient_NonCreatedResponseIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid ttl"})
	}))
	t.Cleanup(srv.Close)
	c := adminclient.New(srv.URL)
	if _, err := c.CreateEnrollmentToken(context.Background(), time.Second); err == nil {
		t.Fatal("expected error on non-201 response")
	}
}
