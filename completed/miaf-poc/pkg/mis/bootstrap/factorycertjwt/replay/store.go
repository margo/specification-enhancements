// Package replay provides a jti-based replay tracker for Bootstrap Assertion
// JWTs. Per SUP Appendix A "Signed-Assertion Method Common Requirements",
// any signed assertion MUST carry a unique jti and the MIS MUST reject
// replays of a previously seen jti. The PoC ships an in-memory single-process
// store; the interface is narrow so future backends (Redis, SQLite) can
// replace it without validator changes.
package replay

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrReplay is returned by Store.Witness when jti has already been seen and
// has not yet expired.
var ErrReplay = errors.New("replay: jti already witnessed")

// Store tracks seen jtis until their associated expiry.
type Store interface {
	// Witness records jti as seen until expiresAt. If the jti has already
	// been witnessed and has not yet expired, Witness returns ErrReplay and
	// does not mutate state. ctx is reserved for backends that make network
	// calls (Redis, SQLite over a connection pool).
	Witness(ctx context.Context, jti string, expiresAt time.Time) error
}

// Config configures MemoryStore.
type Config struct {
	// Clock defaults to time.Now when nil.
	Clock func() time.Time
	// SweepEvery is the interval between expired-entry sweeps. Zero disables
	// background sweeping (tests exercise purge-on-read).
	SweepEvery time.Duration
	// Context defaults to context.Background() when nil. Used to shutdown the
	// background sweep goroutine.
	Context context.Context
}

// MemoryStore is a single-process map-backed Store.
type MemoryStore struct {
	mu    sync.Mutex
	seen  map[string]time.Time // jti -> expiresAt
	clock func() time.Time
}

// NewMemoryStore returns a MemoryStore. If cfg.SweepEvery > 0, a background
// goroutine periodically purges expired entries; otherwise Witness purges
// lazily.
func NewMemoryStore(cfg Config) *MemoryStore {
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
	s := &MemoryStore{seen: map[string]time.Time{}, clock: cfg.Clock}
	if cfg.SweepEvery > 0 {
		go s.sweepLoop(cfg.Context, cfg.SweepEvery)
	}
	return s
}

// Witness implements Store.
func (s *MemoryStore) Witness(ctx context.Context, jti string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	if prev, ok := s.seen[jti]; ok {
		if prev.After(now) {
			return ErrReplay
		}
		// entry is stale - overwritable as new
	}
	s.seen[jti] = expiresAt
	return nil
}

func (s *MemoryStore) sweepLoop(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Sweep()
		}
	}
}

// Sweep drops entries whose expiresAt is not after s.clock().
func (s *MemoryStore) Sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	for jti, exp := range s.seen {
		if !exp.After(now) {
			delete(s.seen, jti)
		}
	}
}
