package scheduler_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/device-agent/clock"
	"github.com/margo/miaf-poc/pkg/device-agent/identitymanager"
	"github.com/margo/miaf-poc/pkg/device-agent/scheduler"
)

// fakeManager records the last submitted command kind.
type fakeManager struct {
	mu   sync.Mutex
	cmds []string
}

func (m *fakeManager) Submit(c identitymanager.Cmd) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case c.Renew != nil:
		m.cmds = append(m.cmds, "Renew")
		// Return a successful reply so the scheduler re-arms.
		go func() {
			c.Renew.Reply <- identitymanager.RenewReply{
				HTTPStatus: 200,
				NewSerial:  "99",
				NewExpiry:  time.Now().Add(60 * time.Minute),
			}
			close(c.Renew.Reply)
		}()
	case c.Refresh != nil:
		m.cmds = append(m.cmds, "Refresh")
		go func() {
			c.Refresh.Reply <- identitymanager.RefreshBundleReply{}
			close(c.Refresh.Reply)
		}()
	case c.Check != nil:
		m.cmds = append(m.cmds, "Check")
		go func() {
			c.Check.Reply <- identitymanager.CheckRevocationsReply{HTTPStatus: 200}
			close(c.Check.Reply)
		}()
	}
	return true
}

func (m *fakeManager) hasSubmitted(kind string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range m.cmds {
		if k == kind {
			return true
		}
	}
	return false
}

func TestScheduler_RenewalFiresAtThreshold(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	fc := clock.NewFake(start)
	mgr := &fakeManager{}

	leafNB := start
	leafNA := leafNB.Add(60 * time.Minute) // 60-minute leaf; threshold at 30m

	sch := scheduler.New(scheduler.Config{
		Clock:            fc,
		Manager:          mgr,
		BundleEvery:      time.Hour,
		RevocationsEvery: 5 * time.Minute,
		JWTEvictEvery:    30 * time.Second,
		RenewalRatio:     0.5,
		LeafNotBefore:    leafNB,
		LeafNotAfter:     leafNA,
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sch.Run(ctx)

	// Wait briefly for the goroutine to start up.
	time.Sleep(10 * time.Millisecond)

	// Advance to just before threshold - no renewal yet.
	fc.Advance(29 * time.Minute)
	time.Sleep(10 * time.Millisecond)
	if mgr.hasSubmitted("Renew") {
		t.Error("renewal should not fire before threshold")
	}

	// Cross the 30m threshold.
	fc.Advance(2 * time.Minute)
	// Wait for the renewal to be processed.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mgr.hasSubmitted("Renew") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !mgr.hasSubmitted("Renew") {
		t.Error("renewal should fire after threshold")
	}
}

func TestScheduler_RevocationsJobFires(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	fc := clock.NewFake(start)
	mgr := &fakeManager{}

	sch := scheduler.New(scheduler.Config{
		Clock:            fc,
		Manager:          mgr,
		BundleEvery:      time.Hour,
		RevocationsEvery: 5 * time.Minute,
		JWTEvictEvery:    30 * time.Second,
		RenewalRatio:     0.5,
		LeafNotBefore:    start,
		LeafNotAfter:     start.Add(2 * time.Hour),
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sch.Run(ctx)

	time.Sleep(10 * time.Millisecond)
	fc.Advance(6 * time.Minute)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mgr.hasSubmitted("Check") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !mgr.hasSubmitted("Check") {
		t.Error("revocations check should fire after 5 minutes")
	}
}
