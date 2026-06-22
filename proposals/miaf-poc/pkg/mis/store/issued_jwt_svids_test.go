package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/store"
)

func TestIssuedJWTSVIDs_Record(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	rec := identity.IssuedJWTSVIDRecord{
		JTI:       "jti-1",
		SPIFFEID:  "spiffe://td/device/1",
		Audiences: []string{"https://aud/"},
		IssuedAt:  now,
		ExpiresAt: now.Add(5 * time.Minute),
	}
	if err := s.IssuedJWTSVIDs().Record(context.Background(), rec); err != nil {
		t.Fatalf("Record: %v", err)
	}
	// Idempotent: second insert with same jti should not error.
	if err := s.IssuedJWTSVIDs().Record(context.Background(), rec); err != nil {
		t.Fatalf("second Record (idempotent): %v", err)
	}
}
