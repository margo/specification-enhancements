package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/margo/miaf-poc/pkg/device-agent/clock"
	"github.com/margo/miaf-poc/pkg/device-agent/identitymanager"
	jwtcache "github.com/margo/miaf-poc/pkg/device-agent/jwtsvid/cache"
)

// Submitter is the narrow interface Scheduler needs from the IdentityManager.
type Submitter interface {
	Submit(identitymanager.Cmd) bool
}

// Config holds all scheduler dependencies.
type Config struct {
	Clock            clock.Clock
	Manager          Submitter
	JWTCache         *jwtcache.Cache // shared with manager; scheduler evicts directly
	BundleEvery      time.Duration   // default 1h
	RevocationsEvery time.Duration   // default 5m
	JWTEvictEvery    time.Duration   // default 30s
	RenewalRatio     float64         // default 0.5

	// LeafNotBefore / LeafNotAfter from the loaded state on startup.
	LeafNotBefore time.Time
	LeafNotAfter  time.Time

	Logger *slog.Logger
}

// Scheduler drives four background jobs for the daemon.
type Scheduler struct {
	cfg Config
}

// New creates a Scheduler applying defaults for zero-value durations.
func New(cfg Config) *Scheduler {
	if cfg.BundleEvery == 0 {
		cfg.BundleEvery = time.Hour
	}
	if cfg.RevocationsEvery == 0 {
		cfg.RevocationsEvery = 5 * time.Minute
	}
	if cfg.JWTEvictEvery == 0 {
		cfg.JWTEvictEvery = 30 * time.Second
	}
	if cfg.RenewalRatio == 0 {
		cfg.RenewalRatio = 0.5
	}
	return &Scheduler{cfg: cfg}
}

// Run blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	now := s.cfg.Clock.Now()
	renewAt := RenewAt(s.cfg.LeafNotBefore, s.cfg.LeafNotAfter, s.cfg.RenewalRatio)
	renewDelay := renewAt.Sub(now)
	if renewDelay <= 0 {
		renewDelay = time.Millisecond // fire immediately if already past threshold
	}

	bundleT := s.cfg.Clock.NewTimer(s.cfg.BundleEvery)
	revT := s.cfg.Clock.NewTimer(s.cfg.RevocationsEvery)
	jwtT := s.cfg.Clock.NewTimer(s.cfg.JWTEvictEvery)
	renewT := s.cfg.Clock.NewTimer(renewDelay)

	for {
		select {
		case <-ctx.Done():
			return

		case <-renewT.C():
			s.cfg.Logger.Info("scheduler: renewal trigger")
			reply := make(chan identitymanager.RenewReply, 1)
			ok := s.cfg.Manager.Submit(identitymanager.Cmd{
				Ctx:   ctx,
				Renew: &identitymanager.RenewCmd{Reply: reply},
			})
			if !ok {
				return
			}
			r := <-reply
			if r.Err != nil {
				s.cfg.Logger.Warn("scheduler: renewal failed; backing off",
					"err", r.Err, "status", r.HTTPStatus)
				renewT = s.cfg.Clock.NewTimer(5 * time.Minute)
				continue
			}
			// Re-arm at 50% of the new leaf's remaining lifetime.
			next := RenewAt(s.cfg.Clock.Now(), r.NewExpiry, s.cfg.RenewalRatio)
			delay := next.Sub(s.cfg.Clock.Now())
			if delay <= 0 {
				delay = time.Millisecond
			}
			renewT = s.cfg.Clock.NewTimer(delay)

		case <-bundleT.C():
			reply := make(chan identitymanager.RefreshBundleReply, 1)
			s.cfg.Manager.Submit(identitymanager.Cmd{
				Ctx:     ctx,
				Refresh: &identitymanager.RefreshBundleCmd{Reply: reply},
			})
			<-reply
			bundleT = s.cfg.Clock.NewTimer(s.cfg.BundleEvery)

		case <-revT.C():
			reply := make(chan identitymanager.CheckRevocationsReply, 1)
			s.cfg.Manager.Submit(identitymanager.Cmd{
				Ctx:   ctx,
				Check: &identitymanager.CheckRevocationsCmd{Reply: reply},
			})
			r := <-reply
			if r.OwnSVIDListed {
				s.cfg.Logger.Warn("scheduler: own SVID revoked; stopping all timers")
				return
			}
			revT = s.cfg.Clock.NewTimer(s.cfg.RevocationsEvery)

		case <-jwtT.C():
			if s.cfg.JWTCache != nil {
				s.cfg.JWTCache.EvictExpired(s.cfg.Clock.Now())
			}
			jwtT = s.cfg.Clock.NewTimer(s.cfg.JWTEvictEvery)
		}
	}
}
