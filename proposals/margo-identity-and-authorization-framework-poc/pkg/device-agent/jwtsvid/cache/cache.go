// Package cache holds an in-memory near-expiry cache of JWT SVIDs keyed on
// the audience set. The IdentityManager consults it before issuing
// ExchangeJWT commands; a hit avoids a round-trip to MIS.
package cache

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// Entry is one cached JWT SVID.
type Entry struct {
	Token     string
	ExpiresAt time.Time
}

// Cache is safe for concurrent use; the IdentityManager goroutine is the only
// writer in the daemon, but the on-demand socket goroutine reads via Get.
type Cache struct {
	mu        sync.Mutex
	threshold time.Duration
	entries   map[string]Entry
}

// New constructs a Cache that misses when an entry's remaining lifetime is
// below threshold. PoC default: 5 minutes.
func New(threshold time.Duration) *Cache {
	return &Cache{threshold: threshold, entries: map[string]Entry{}}
}

// Put stores e under the given audience set.
func (c *Cache) Put(audiences []string, e Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key(audiences)] = e
}

// Get returns the cached entry if its remaining lifetime is at least the
// configured threshold relative to now.
func (c *Cache) Get(audiences []string, now time.Time) (Entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key(audiences)]
	if !ok {
		return Entry{}, false
	}
	if e.ExpiresAt.Sub(now) < c.threshold {
		return Entry{}, false
	}
	return e, true
}

// EvictExpired drops every entry that is now within threshold of expiry.
func (c *Cache) EvictExpired(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.entries {
		if e.ExpiresAt.Sub(now) < c.threshold {
			delete(c.entries, k)
		}
	}
}

// Len returns the number of entries currently in the cache.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// key encodes the audience set order-independently.
func key(audiences []string) string {
	cp := append([]string(nil), audiences...)
	sort.Strings(cp)
	return strings.Join(cp, "\x00")
}
