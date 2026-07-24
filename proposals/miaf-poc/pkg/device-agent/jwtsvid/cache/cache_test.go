package cache

import (
	"testing"
	"time"
)

func TestCache_HitWithinThreshold(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	c := New(5 * time.Minute)

	c.Put([]string{"https://api.example.com"}, Entry{
		Token:     "JWT1",
		ExpiresAt: now.Add(30 * time.Minute),
	})
	got, ok := c.Get([]string{"https://api.example.com"}, now)
	if !ok || got.Token != "JWT1" {
		t.Fatalf("Get = (%v, %v), want hit on JWT1", got, ok)
	}
}

func TestCache_MissNearExpiry(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	c := New(5 * time.Minute)

	c.Put([]string{"a"}, Entry{Token: "JWT1", ExpiresAt: now.Add(4 * time.Minute)})
	if _, ok := c.Get([]string{"a"}, now); ok {
		t.Errorf("Get hit when expiry is within threshold")
	}
}

func TestCache_AudienceOrderInvariant(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	c := New(5 * time.Minute)

	c.Put([]string{"b", "a"}, Entry{Token: "JWT", ExpiresAt: now.Add(1 * time.Hour)})
	got, ok := c.Get([]string{"a", "b"}, now)
	if !ok || got.Token != "JWT" {
		t.Errorf("Get = (%v, %v); audience order should not affect cache hit", got, ok)
	}
}

func TestCache_EvictExpired(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	c := New(5 * time.Minute)
	c.Put([]string{"a"}, Entry{Token: "1", ExpiresAt: now.Add(2 * time.Minute)})
	c.Put([]string{"b"}, Entry{Token: "2", ExpiresAt: now.Add(2 * time.Hour)})
	c.EvictExpired(now.Add(10 * time.Minute))
	if _, ok := c.Get([]string{"a"}, now.Add(10*time.Minute)); ok {
		t.Errorf("expired entry not evicted")
	}
	if _, ok := c.Get([]string{"b"}, now.Add(10*time.Minute)); !ok {
		t.Errorf("non-expired entry should still be present")
	}
}
