package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

// staticQueue implements identity.RevocationQueue for handler tests.
type staticQueue struct {
	entries []identity.RevocationEntry
	now     time.Time
	err     error
}

func (s *staticQueue) List(_ context.Context, _ time.Time) ([]identity.RevocationEntry, time.Time, error) {
	if s.err != nil {
		return nil, time.Time{}, s.err
	}
	var latest time.Time
	for _, e := range s.entries {
		if e.RevokedAt.After(latest) {
			latest = e.RevokedAt
		}
	}
	return s.entries, latest, nil
}
func (s *staticQueue) Revoke(_ context.Context, _, _, _ string, _, _ time.Time) (bool, error) {
	return false, errors.New("not used in handler tests")
}
func (s *staticQueue) PruneExpired(_ context.Context, _ time.Time) (int, error) {
	return 0, errors.New("not used in handler tests")
}

func TestRevocations_200_OK(t *testing.T) {
	fixed := time.Date(2025, 10, 25, 14, 12, 31, 0, time.UTC)
	q := &staticQueue{entries: []identity.RevocationEntry{
		{SerialHex: "8F12A4C9D23E41B1", FingerprintHex: "2f4c3d9a7b1e0c6d8f2a1b9c0d7e6f5a4b3c2d1e0f9a8b7c6d5e4f3a2b1c0d9e",
			RevokedAt: time.Date(2025, 10, 20, 9, 33, 45, 0, time.UTC), NotAfter: fixed.Add(48 * time.Hour)},
	}}
	h := mishttp.NewRevocationsHandler(mishttp.RevocationsConfig{
		Queue: q,
		Now:   func() time.Time { return fixed },
	})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/revocations", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if etag := w.Header().Get("ETag"); etag == "" {
		t.Errorf("ETag missing")
	}
	if lm := w.Header().Get("Last-Modified"); lm == "" {
		t.Errorf("Last-Modified missing")
	}

	var dto common.RevocationListDTO
	if err := json.NewDecoder(w.Body).Decode(&dto); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dto.LastUpdated != "2025-10-20T09:33:45Z" {
		t.Errorf("last_updated = %q", dto.LastUpdated)
	}
	if len(dto.RevokedSVIDs) != 1 || dto.RevokedSVIDs[0].SerialNumber != "8F12A4C9D23E41B1" {
		t.Errorf("dto = %+v", dto)
	}
}

func TestRevocations_200_EmptyList(t *testing.T) {
	q := &staticQueue{}
	h := mishttp.NewRevocationsHandler(mishttp.RevocationsConfig{Queue: q, Now: time.Now})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/revocations", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var dto common.RevocationListDTO
	_ = json.NewDecoder(w.Body).Decode(&dto)
	if dto.LastUpdated != "" {
		t.Errorf("last_updated = %q, want empty", dto.LastUpdated)
	}
	if dto.RevokedSVIDs == nil {
		t.Errorf("RevokedSVIDs is nil; want empty array for JSON []")
	}
}

func TestRevocations_304_IfNoneMatch(t *testing.T) {
	q := &staticQueue{entries: []identity.RevocationEntry{{
		SerialHex: "AA", FingerprintHex: "ff",
		RevokedAt: time.Date(2025, 10, 20, 9, 33, 45, 0, time.UTC),
		NotAfter:  time.Date(2025, 11, 20, 0, 0, 0, 0, time.UTC),
	}}}
	h := mishttp.NewRevocationsHandler(mishttp.RevocationsConfig{Queue: q, Now: time.Now})
	// First GET to discover the ETag.
	r1 := httptest.NewRequest(http.MethodGet, "/api/v1/revocations", nil)
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, r1)
	etag := w1.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("first response had no ETag")
	}

	// Second GET with If-None-Match.
	r2 := httptest.NewRequest(http.MethodGet, "/api/v1/revocations", nil)
	r2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, r2)
	if w2.Code != 304 {
		t.Fatalf("status = %d, want 304", w2.Code)
	}
	if w2.Body.Len() != 0 {
		t.Errorf("304 body = %q, want empty", w2.Body.String())
	}
	if w2.Header().Get("ETag") != etag {
		t.Errorf("ETag mismatched on 304")
	}
}

func TestRevocations_500_RevocationFormat(t *testing.T) {
	q := &staticQueue{err: errors.New("store boom")}
	h := mishttp.NewRevocationsHandler(mishttp.RevocationsConfig{Queue: q, Now: time.Now})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/revocations", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 500 {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var pd struct {
		Type  string `json:"type"`
		Title string `json:"title"`
	}
	_ = json.NewDecoder(w.Body).Decode(&pd)
	if pd.Type != "https://margo.org/docs/errors/revocation-format" {
		t.Errorf("type = %q", pd.Type)
	}
	if pd.Title != "Revocation List Parsing Error" {
		t.Errorf("title = %q", pd.Title)
	}
}
