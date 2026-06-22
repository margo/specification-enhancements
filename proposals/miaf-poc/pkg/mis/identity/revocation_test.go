package identity_test

import (
	"context"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/identity"
)

// memRevocations is an in-memory RevocationQueue for tests.
type memRevocations struct {
	entries map[string]identity.RevocationEntry
}

func newMemRevocations() *memRevocations {
	return &memRevocations{entries: map[string]identity.RevocationEntry{}}
}

func (m *memRevocations) Revoke(_ context.Context, serialHex, fingerprintHex, reason string, notAfter, now time.Time) (bool, error) {
	if _, ok := m.entries[serialHex]; ok {
		return false, nil
	}
	m.entries[serialHex] = identity.RevocationEntry{
		SerialHex:      serialHex,
		FingerprintHex: fingerprintHex,
		RevokedAt:      now,
		Reason:         reason,
		NotAfter:       notAfter,
	}
	return true, nil
}

func (m *memRevocations) List(_ context.Context, now time.Time) ([]identity.RevocationEntry, time.Time, error) {
	var out []identity.RevocationEntry
	var latest time.Time
	for _, e := range m.entries {
		if e.NotAfter.After(now) {
			out = append(out, e)
			if e.RevokedAt.After(latest) {
				latest = e.RevokedAt
			}
		}
	}
	return out, latest, nil
}

func (m *memRevocations) PruneExpired(_ context.Context, now time.Time) (int, error) {
	var removed int
	for s, e := range m.entries {
		if !e.NotAfter.After(now) {
			delete(m.entries, s)
			removed++
		}
	}
	return removed, nil
}

func TestRevocationQueue_FakeInvariants(t *testing.T) {
	q := newMemRevocations()
	ctx := context.Background()
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	na := now.Add(24 * time.Hour)

	// First revocation.
	newly, err := q.Revoke(ctx, "AA", "ff", "test", na, now)
	if err != nil || !newly {
		t.Fatalf("first revoke: newly=%v err=%v", newly, err)
	}

	// Idempotent repeat.
	newly2, _ := q.Revoke(ctx, "AA", "ff", "test", na, now.Add(time.Hour))
	if newly2 {
		t.Error("repeated revoke returned newly=true")
	}

	entries, lastUpdated, _ := q.List(ctx, now)
	if len(entries) != 1 || entries[0].SerialHex != "AA" {
		t.Fatalf("List: %+v", entries)
	}
	if !lastUpdated.Equal(now) {
		t.Errorf("last updated = %v, want %v", lastUpdated, now)
	}

	// Prune: naturally-expired entry disappears.
	_, _ = q.Revoke(ctx, "BB", "ee", "expired", now.Add(-time.Second), now)
	n, _ := q.PruneExpired(ctx, now)
	if n != 1 {
		t.Errorf("prune removed %d, want 1", n)
	}
	entries2, _, _ := q.List(ctx, now)
	if len(entries2) != 1 || entries2[0].SerialHex != "AA" {
		t.Errorf("after prune: %+v", entries2)
	}
}
