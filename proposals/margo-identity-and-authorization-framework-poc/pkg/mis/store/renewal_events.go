package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/identity"
)

// RenewalEventsConfig configures the rate limiter.
type RenewalEventsConfig struct {
	MaxInWindow int           // e.g. 5
	Window      time.Duration // e.g. 24h
}

// RenewalEvents is an identity.RenewalRateLimiter backed by the unified
// store's *sql.DB.
type RenewalEvents struct {
	db  *sql.DB
	cfg RenewalEventsConfig
}

// RenewalEvents returns the adapter. Validates the config.
func (s *Store) RenewalEvents(cfg RenewalEventsConfig) *RenewalEvents {
	if cfg.MaxInWindow <= 0 {
		panic("store: RenewalEventsConfig.MaxInWindow must be > 0")
	}
	if cfg.Window <= 0 {
		panic("store: RenewalEventsConfig.Window must be > 0")
	}
	return &RenewalEvents{db: s.db, cfg: cfg}
}

// Interface check.
var _ identity.RenewalRateLimiter = (*RenewalEvents)(nil)

// Allow implements identity.RenewalRateLimiter.
func (r *RenewalEvents) Allow(ctx context.Context, spiffeID string, now time.Time) (bool, time.Duration, error) {
	cutoff := now.Add(-r.cfg.Window)
	// Fetch count + earliest event within the window in one pass so the
	// Retry-After calculation is consistent with the count that drove the
	// decision.
	var count int
	var earliest sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*), MIN(recorded_at) FROM renewal_events
		  WHERE spiffe_id = ? AND recorded_at > ?`,
		spiffeID, cutoff.Format(time.RFC3339Nano)).Scan(&count, &earliest)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, 0, fmt.Errorf("renewal_events: allow query: %w", err)
	}
	if count < r.cfg.MaxInWindow {
		return true, 0, nil
	}
	// count >= MaxInWindow - blocked. Compute Retry-After from the earliest
	// event in the current window.
	if !earliest.Valid {
		// Defensive: count said there were events but MIN returned NULL -
		// treat as 1-second backoff rather than panicking.
		return false, time.Second, nil
	}
	earliestT, perr := time.Parse(time.RFC3339Nano, earliest.String)
	if perr != nil {
		return false, time.Second, nil
	}
	retry := earliestT.Add(r.cfg.Window).Sub(now)
	if retry <= 0 {
		retry = time.Second
	}
	return false, retry, nil
}

// Record implements identity.RenewalRateLimiter.
// It inserts a success event and prunes rows outside the sliding window to
// keep the table bounded.
func (r *RenewalEvents) Record(ctx context.Context, spiffeID string, now time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO renewal_events (spiffe_id, recorded_at) VALUES (?, ?)`,
		spiffeID, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("renewal_events: insert: %w", err)
	}
	cutoff := now.Add(-r.cfg.Window).UTC().Format(time.RFC3339Nano)
	_, _ = r.db.ExecContext(ctx,
		`DELETE FROM renewal_events WHERE recorded_at <= ?`, cutoff)
	return nil
}
