// Package cache provides a simple in-memory TTL cache for rule value lookups.
// Entries expire after a configurable duration and are lazily cleaned up on access.
package cache

import (
	"log/slog"
	"sync"
	"time"
)

// Entry holds a cached value with its expiration time.
type Entry struct {
	Value     interface{}
	ExpiresAt time.Time
}

// TTLCache is a thread-safe in-memory cache with per-key TTL expiration.
type TTLCache struct {
	mu      sync.RWMutex
	items   map[string]Entry
	ttl     time.Duration
	closeCh chan struct{}
}

// New creates a new TTLCache with the given default TTL.
// It starts a background goroutine that periodically evicts expired entries.
func New(ttl time.Duration) *TTLCache {
	c := &TTLCache{
		items:   make(map[string]Entry),
		ttl:     ttl,
		closeCh: make(chan struct{}),
	}
	go c.janitor()
	return c
}

// Get retrieves a value from the cache. Returns (value, true) if found and not
// expired, or (nil, false) otherwise.
func (c *TTLCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.ExpiresAt) {
		slog.Debug("Cache miss", "component", "cache", "key", key)
		return nil, false
	}
	slog.Debug("Cache hit", "component", "cache", "key", key)
	return entry.Value, true
}

// Set stores a value in the cache with the default TTL.
func (c *TTLCache) Set(key string, value interface{}) {
	c.mu.Lock()
	c.items[key] = Entry{
		Value:     value,
		ExpiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

// Invalidate removes a specific key from the cache.
func (c *TTLCache) Invalidate(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// InvalidatePrefix removes all keys that start with the given prefix.
// Useful for invalidating all cached values for a specific integration_id.
func (c *TTLCache) InvalidatePrefix(prefix string) {
	c.mu.Lock()
	for k := range c.items {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.items, k)
		}
	}
	c.mu.Unlock()
}

// InvalidateAll removes all entries from the cache.
func (c *TTLCache) InvalidateAll() {
	c.mu.Lock()
	c.items = make(map[string]Entry)
	c.mu.Unlock()
}

// Close stops the background janitor goroutine.
func (c *TTLCache) Close() {
	close(c.closeCh)
}

// janitor periodically evicts expired entries every TTL/2 interval.
func (c *TTLCache) janitor() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			evicted := 0
			c.mu.Lock()
			for k, v := range c.items {
				if now.After(v.ExpiresAt) {
					delete(c.items, k)
					evicted++
				}
			}
			c.mu.Unlock()
			if evicted > 0 {
				slog.Debug("Cache janitor evicted expired entries", "component", "cache", "evicted", evicted)
			}
		case <-c.closeCh:
			return
		}
	}
}
