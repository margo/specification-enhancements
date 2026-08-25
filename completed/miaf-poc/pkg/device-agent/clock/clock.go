// Package clock provides a Clock interface with a real (wall-clock) and a
// fake (advance-on-demand) implementation. Every time-dependent decision in
// the device-agent daemon (renewal threshold, scheduler tick, JWT cache
// eviction) reads time through Clock so unit tests can drive time forward.
package clock

import (
	"sync"
	"time"
)

// Clock abstracts time.Now and timer creation.
type Clock interface {
	Now() time.Time
	NewTimer(d time.Duration) Timer
}

// Timer abstracts time.Timer.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

// Real wraps the standard library time package.
type Real struct{}

func (Real) Now() time.Time { return time.Now() }
func (Real) NewTimer(d time.Duration) Timer {
	t := time.NewTimer(d)
	return realTimer{t}
}

type realTimer struct{ *time.Timer }

func (r realTimer) C() <-chan time.Time { return r.Timer.C }

// Fake is a deterministic clock for tests.
type Fake struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

// NewFake creates a Fake clock starting at start.
func NewFake(start time.Time) *Fake { return &Fake{now: start} }

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *Fake) NewTimer(d time.Duration) Timer {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := &fakeTimer{
		c:      make(chan time.Time, 1),
		fireAt: f.now.Add(d),
	}
	f.timers = append(f.timers, t)
	return t
}

// Advance moves the fake clock forward by d, firing any timers whose deadline
// is reached. Timers fire in deadline order on a buffered channel; tests pull
// from t.C() to observe the firing time.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	now := f.now
	pending := f.timers
	f.timers = pending[:0]
	for _, t := range pending {
		if !now.Before(t.fireAt) && !t.stopped {
			select {
			case t.c <- t.fireAt:
			default:
			}
			continue
		}
		f.timers = append(f.timers, t)
	}
	f.mu.Unlock()
}

type fakeTimer struct {
	c       chan time.Time
	fireAt  time.Time
	stopped bool
}

func (f *fakeTimer) C() <-chan time.Time { return f.c }
func (f *fakeTimer) Stop() bool {
	prev := f.stopped
	f.stopped = true
	return !prev
}
