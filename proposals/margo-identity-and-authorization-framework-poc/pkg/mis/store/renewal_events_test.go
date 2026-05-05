package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/store"
)

func openRenewalLimiter(t *testing.T, maxInWindow int, window time.Duration) (*store.Store, func(context.Context, string, time.Time) (bool, time.Duration, error), func(context.Context, string, time.Time) error) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	lim := s.RenewalEvents(store.RenewalEventsConfig{
		MaxInWindow: maxInWindow,
		Window:      window,
	})
	return s, lim.Allow, lim.Record
}

func TestRenewalEvents_AllowUnderLimit(t *testing.T) {
	_, allow, record := openRenewalLimiter(t, 5, 24*time.Hour)
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		ok, _, err := allow(ctx, "spiffe://margo.example.com/margo/device/abc", now.Add(time.Duration(i)*time.Minute))
		if err != nil {
			t.Fatalf("allow: %v", err)
		}
		if !ok {
			t.Fatalf("iteration %d: expected allowed", i)
		}
		if err := record(ctx, "spiffe://margo.example.com/margo/device/abc", now.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("record: %v", err)
		}
	}
}

func TestRenewalEvents_BlocksAtLimit(t *testing.T) {
	_, allow, record := openRenewalLimiter(t, 2, time.Hour)
	ctx := context.Background()
	start := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)

	_ = record(ctx, "spiffe://td/x", start)
	_ = record(ctx, "spiffe://td/x", start.Add(10*time.Minute))

	ok, retry, err := allow(ctx, "spiffe://td/x", start.Add(20*time.Minute))
	if err != nil {
		t.Fatalf("allow: %v", err)
	}
	if ok {
		t.Fatal("expected rate limit to block the 3rd attempt")
	}
	// Earliest event in window is `start`; retry-after = start + 1h - (start+20m) = 40m.
	wantRetry := 40 * time.Minute
	if retry != wantRetry {
		t.Errorf("retryAfter = %s, want %s", retry, wantRetry)
	}
}

func TestRenewalEvents_WindowExpiryFreesSlot(t *testing.T) {
	_, allow, record := openRenewalLimiter(t, 1, time.Hour)
	ctx := context.Background()
	start := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)

	_ = record(ctx, "spiffe://td/x", start)

	// Just inside the window - blocked.
	ok, _, _ := allow(ctx, "spiffe://td/x", start.Add(59*time.Minute))
	if ok {
		t.Error("expected blocked at 59m")
	}
	// Just outside the window - allowed.
	ok, _, _ = allow(ctx, "spiffe://td/x", start.Add(61*time.Minute))
	if !ok {
		t.Error("expected allowed at 61m")
	}
}

func TestRenewalEvents_PerSPIFFEID(t *testing.T) {
	_, allow, record := openRenewalLimiter(t, 1, time.Hour)
	ctx := context.Background()
	start := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)

	_ = record(ctx, "spiffe://td/a", start)

	ok, _, _ := allow(ctx, "spiffe://td/a", start.Add(time.Minute))
	if ok {
		t.Error("/a should be rate-limited")
	}
	ok, _, _ = allow(ctx, "spiffe://td/b", start.Add(time.Minute))
	if !ok {
		t.Error("/b must not share /a's budget")
	}
}
