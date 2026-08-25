package scheduler

import (
	"testing"
	"time"
)

func TestRenewAt_HalfLifetime(t *testing.T) {
	t.Parallel()
	nb := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	na := nb.Add(24 * time.Hour)
	got := RenewAt(nb, na, 0.5)
	want := nb.Add(12 * time.Hour)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRenewAt_BadRatioFallsBackTo50Percent(t *testing.T) {
	t.Parallel()
	nb := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	na := nb.Add(24 * time.Hour)
	if got := RenewAt(nb, na, -1); !got.Equal(nb.Add(12*time.Hour)) {
		t.Errorf("ratio=-1: got %v, want %v", got, nb.Add(12*time.Hour))
	}
	if got := RenewAt(nb, na, 1.5); !got.Equal(nb.Add(12*time.Hour)) {
		t.Errorf("ratio=1.5: got %v, want %v", got, nb.Add(12*time.Hour))
	}
}
