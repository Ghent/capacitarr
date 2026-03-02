package routes

import (
	"testing"
	"time"
)

func TestRateLimiterAllowsUnderLimit(t *testing.T) {
	rl := &loginRateLimiter{
		attempts: make(map[string][]time.Time),
		window:   1 * time.Minute,
		limit:    5,
	}

	for i := range 5 {
		if !rl.allow("192.168.1.1") {
			t.Errorf("Request %d should have been allowed (under limit)", i+1)
		}
	}
}

func TestRateLimiterBlocksOverLimit(t *testing.T) {
	rl := &loginRateLimiter{
		attempts: make(map[string][]time.Time),
		window:   1 * time.Minute,
		limit:    3,
	}

	// First 3 should succeed
	for i := range 3 {
		if !rl.allow("10.0.0.1") {
			t.Fatalf("Request %d should have been allowed", i+1)
		}
	}

	// 4th should be blocked
	if rl.allow("10.0.0.1") {
		t.Error("Request 4 should have been blocked (over limit)")
	}
}

func TestRateLimiterPerIPIsolation(t *testing.T) {
	rl := &loginRateLimiter{
		attempts: make(map[string][]time.Time),
		window:   1 * time.Minute,
		limit:    2,
	}

	// Exhaust limit for IP-A
	for range 2 {
		rl.allow("ip-a")
	}
	if rl.allow("ip-a") {
		t.Error("ip-a should be rate limited")
	}

	// IP-B should still be allowed
	if !rl.allow("ip-b") {
		t.Error("ip-b should NOT be rate limited (separate IP)")
	}
}

func TestRateLimiterWindowExpiry(t *testing.T) {
	rl := &loginRateLimiter{
		attempts: make(map[string][]time.Time),
		window:   50 * time.Millisecond,
		limit:    2,
	}

	// Exhaust limit
	rl.allow("10.0.0.1")
	rl.allow("10.0.0.1")

	if rl.allow("10.0.0.1") {
		t.Error("Should be blocked immediately after exhausting limit")
	}

	// Wait for window to expire
	time.Sleep(100 * time.Millisecond)

	// Should be allowed again after window expires
	if !rl.allow("10.0.0.1") {
		t.Error("Should be allowed after window expiry")
	}
}

func TestRateLimiterPrunesExpiredTimestamps(t *testing.T) {
	rl := &loginRateLimiter{
		attempts: make(map[string][]time.Time),
		window:   50 * time.Millisecond,
		limit:    3,
	}

	// Add some attempts
	rl.allow("10.0.0.1")
	rl.allow("10.0.0.1")

	// Wait for them to expire
	time.Sleep(100 * time.Millisecond)

	// New calls should succeed and prune old ones
	for i := range 3 {
		if !rl.allow("10.0.0.1") {
			t.Errorf("Request %d should be allowed after old timestamps expired", i+1)
		}
	}
}

func TestRateLimiterExactlyAtLimit(t *testing.T) {
	rl := &loginRateLimiter{
		attempts: make(map[string][]time.Time),
		window:   1 * time.Minute,
		limit:    1,
	}

	// First request should succeed
	if !rl.allow("10.0.0.1") {
		t.Error("First request should be allowed")
	}

	// Second request should fail (limit is 1)
	if rl.allow("10.0.0.1") {
		t.Error("Second request should be blocked with limit=1")
	}
}
