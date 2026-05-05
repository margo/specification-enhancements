package conformance_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

func loadRevocationGolden(t *testing.T, relPath string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "conformance", "revocation", relPath))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	return b
}

type staticRevocationQueue struct {
	entries []identity.RevocationEntry
	err     error
}

func (s *staticRevocationQueue) List(_ context.Context, _ time.Time) ([]identity.RevocationEntry, time.Time, error) {
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
func (s *staticRevocationQueue) Revoke(context.Context, string, string, string, time.Time, time.Time) (bool, error) {
	return false, errors.New("not used")
}
func (s *staticRevocationQueue) PruneExpired(context.Context, time.Time) (int, error) {
	return 0, errors.New("not used")
}

func TestConformance_Revocation_EmptyList(t *testing.T) {
	golden := loadRevocationGolden(t, "revocations.200-empty.golden.json")

	h := mishttp.NewRevocationsHandler(mishttp.RevocationsConfig{
		Queue: &staticRevocationQueue{},
		Now:   func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) },
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var got, want common.RevocationListDTO
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode got: %v", err)
	}
	if err := json.Unmarshal(golden, &want); err != nil {
		t.Fatalf("decode want: %v", err)
	}
	if got.LastUpdated != want.LastUpdated {
		t.Errorf("last_updated = %q, want %q", got.LastUpdated, want.LastUpdated)
	}
	if len(got.RevokedSVIDs) != 0 {
		t.Errorf("RevokedSVIDs = %+v, want []", got.RevokedSVIDs)
	}
}

func TestConformance_Revocation_OneEntry(t *testing.T) {
	golden := loadRevocationGolden(t, "revocations.200-one-entry.golden.json")

	h := mishttp.NewRevocationsHandler(mishttp.RevocationsConfig{
		Queue: &staticRevocationQueue{entries: []identity.RevocationEntry{{
			SerialHex:      "8F12A4C9D23E41B1",
			FingerprintHex: "2f4c3d9a7b1e0c6d8f2a1b9c0d7e6f5a4b3c2d1e0f9a8b7c6d5e4f3a2b1c0d9e",
			RevokedAt:      time.Date(2025, 10, 20, 9, 33, 45, 0, time.UTC),
			NotAfter:       time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC),
		}}},
		Now: func() time.Time { return time.Date(2025, 10, 25, 0, 0, 0, 0, time.UTC) },
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var got, want common.RevocationListDTO
	_ = json.NewDecoder(resp.Body).Decode(&got)
	_ = json.Unmarshal(golden, &want)

	if got.LastUpdated != want.LastUpdated {
		t.Errorf("last_updated: got %q, want %q", got.LastUpdated, want.LastUpdated)
	}
	if len(got.RevokedSVIDs) != 1 {
		t.Fatalf("got %d entries, want 1", len(got.RevokedSVIDs))
	}
	if got.RevokedSVIDs[0] != want.RevokedSVIDs[0] {
		t.Errorf("entry mismatch:\n got:  %+v\n want: %+v", got.RevokedSVIDs[0], want.RevokedSVIDs[0])
	}
}

func TestConformance_Revocation_500_Format(t *testing.T) {
	golden := loadRevocationGolden(t, "errors/revocation-format.golden.json")
	var want struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Status int    `json:"status"`
	}
	_ = json.Unmarshal(golden, &want)

	h := mishttp.NewRevocationsHandler(mishttp.RevocationsConfig{
		Queue: &staticRevocationQueue{err: errors.New("synthetic store failure")},
		Now:   time.Now,
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != want.Status {
		t.Errorf("status = %d, want %d", resp.StatusCode, want.Status)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("content-type = %q", ct)
	}
	var got struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Status int    `json:"status"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.Type != want.Type || got.Title != want.Title {
		t.Errorf("problem mismatch:\n got:  %+v\n want: %+v", got, want)
	}
}
