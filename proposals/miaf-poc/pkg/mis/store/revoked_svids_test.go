package store_test

import (
	"context"
	"testing"
	"time"
)

func TestRevokedSVIDs_RevokeAndList(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	later := now.Add(time.Hour)
	na := now.Add(24 * time.Hour)

	q := s.RevokedSVIDs()

	newly, err := q.Revoke(ctx, "AA", "ff", "test", na, now)
	if err != nil || !newly {
		t.Fatalf("first revoke: newly=%v err=%v", newly, err)
	}

	// Second revocation of the same serial is a no-op; must not move revoked_at.
	newly2, err := q.Revoke(ctx, "AA", "ff", "second-reason", na, later)
	if err != nil || newly2 {
		t.Fatalf("repeat revoke: newly=%v err=%v", newly2, err)
	}

	entries, lastUpdated, err := q.List(ctx, now)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[0].SerialHex != "AA" || entries[0].FingerprintHex != "ff" {
		t.Errorf("entry = %+v", entries[0])
	}
	if entries[0].Reason != "test" {
		t.Errorf("first-revocation reason should win, got %q", entries[0].Reason)
	}
	if !lastUpdated.Equal(now) {
		t.Errorf("lastUpdated = %v, want %v", lastUpdated, now)
	}

	// Prune: entry whose not_after has elapsed disappears.
	_, _ = q.Revoke(ctx, "BB", "ee", "expired", now.Add(-time.Second), now)
	n, err := q.PruneExpired(ctx, now)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if n != 1 {
		t.Errorf("pruned = %d, want 1", n)
	}
	entries2, _, _ := q.List(ctx, now)
	if len(entries2) != 1 || entries2[0].SerialHex != "AA" {
		t.Errorf("after prune: %+v", entries2)
	}
}

func TestRevokedSVIDs_EmptyList(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	entries, lastUpdated, err := s.RevokedSVIDs().List(ctx, now)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("entries = %+v, want empty", entries)
	}
	if !lastUpdated.IsZero() {
		t.Errorf("lastUpdated = %v, want zero time", lastUpdated)
	}
}
