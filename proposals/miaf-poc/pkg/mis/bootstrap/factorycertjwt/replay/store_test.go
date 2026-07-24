package replay_test

import (
	"context"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertjwt/replay"
)

func TestStore_SeenBeforeExpiry_ReturnsErrReplay(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	clock := func() time.Time { return now }
	s := replay.NewMemoryStore(replay.Config{Clock: clock, SweepEvery: time.Hour})

	if err := s.Witness(context.Background(), "abc", now.Add(5*time.Minute)); err != nil {
		t.Fatalf("first witness: %v", err)
	}
	err := s.Witness(context.Background(), "abc", now.Add(5*time.Minute))
	if err != replay.ErrReplay {
		t.Errorf("second witness err = %v, want ErrReplay", err)
	}
}

func TestStore_ExpiredEntry_IsPurgedAndReAcceptedAsNew(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	var clock time.Time = now
	s := replay.NewMemoryStore(replay.Config{Clock: func() time.Time { return clock }, SweepEvery: time.Hour})

	if err := s.Witness(context.Background(), "abc", clock.Add(30*time.Second)); err != nil {
		t.Fatalf("first: %v", err)
	}
	clock = clock.Add(time.Minute) // jti expired
	if err := s.Witness(context.Background(), "abc", clock.Add(30*time.Second)); err != nil {
		t.Errorf("post-expiry re-use err = %v, want nil", err)
	}
}

func TestStore_Concurrent_IsSafe(t *testing.T) {
	s := replay.NewMemoryStore(replay.Config{SweepEvery: time.Hour})
	exp := time.Now().Add(time.Minute)
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() { errs <- s.Witness(context.Background(), "same-id", exp) }()
	}
	accepted := 0
	for i := 0; i < 10; i++ {
		if err := <-errs; err == nil {
			accepted++
		}
	}
	if accepted != 1 {
		t.Errorf("accepted = %d, want 1 (only first caller wins)", accepted)
	}
}
