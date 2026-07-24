package identity_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/identity"
)

func TestErrRenewalRateLimited_Is(t *testing.T) {
	err := identity.ErrRenewalRateLimited
	if !errors.Is(err, identity.ErrRenewalRateLimited) {
		t.Error("sentinel does not match itself under errors.Is")
	}
	if errors.Is(err, identity.ErrKeyRotationNotPermitted) {
		t.Error("ErrRenewalRateLimited must not collide with ErrKeyRotationNotPermitted")
	}
}

type fakeLimiter struct {
	allow       bool
	retryAfter  time.Duration
	recordedFor string
}

func (l *fakeLimiter) Allow(_ context.Context, _ string, _ time.Time) (bool, time.Duration, error) {
	return l.allow, l.retryAfter, nil
}

func (l *fakeLimiter) Record(_ context.Context, id string, _ time.Time) error {
	l.recordedFor = id
	return nil
}

// Compile-time assertion that fakeLimiter implements the port.
var _ identity.RenewalRateLimiter = (*fakeLimiter)(nil)
