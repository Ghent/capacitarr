package routes

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

// ipRateLimiter is a simple in-memory sliding-window rate limiter keyed by
// client IP address. It tracks per-IP attempt timestamps and rejects requests
// that exceed the configured limit within the window.
//
// This is intentionally simple — it lives in process memory and resets on
// restart. For a self-hosted single-instance tool this is sufficient to
// prevent automated brute-force attacks without requiring Redis or external
// storage.
type ipRateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	window   time.Duration
	limit    int
	done     chan struct{}
}

// newIPRateLimiter creates a rate limiter that allows `limit` attempts per
// `window` duration from any single IP address.
//
// The cleanup goroutine runs for the lifetime of the process. Stop() is
// available but intentionally not called — rate limiters are created at
// startup and live until process exit. The goroutine cost is negligible
// (one timer tick per 5 minutes per limiter instance).
func newIPRateLimiter(limit int, window time.Duration) *ipRateLimiter {
	rl := &ipRateLimiter{
		attempts: make(map[string][]time.Time),
		window:   window,
		limit:    limit,
		done:     make(chan struct{}),
	}
	// Background goroutine to periodically evict stale entries and prevent
	// unbounded memory growth from IPs that stop sending requests.
	go rl.cleanup()
	return rl
}

// allow checks whether the given IP is within the rate limit. Returns true if
// the request should proceed, false if it should be rejected.
func (rl *ipRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Prune expired timestamps for this IP
	timestamps := rl.attempts[ip]
	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.attempts[ip] = valid
		return false
	}

	rl.attempts[ip] = append(valid, now)
	return true
}

// Stop terminates the background cleanup goroutine. Safe to call multiple times.
func (rl *ipRateLimiter) Stop() {
	select {
	case <-rl.done:
		// already closed
	default:
		close(rl.done)
	}
}

// cleanup runs every 5 minutes and removes entries for IPs that have no
// recent attempts, preventing unbounded memory growth.
func (rl *ipRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			cutoff := now.Add(-rl.window)
			for ip, timestamps := range rl.attempts {
				valid := timestamps[:0]
				for _, t := range timestamps {
					if t.After(cutoff) {
						valid = append(valid, t)
					}
				}
				if len(valid) == 0 {
					delete(rl.attempts, ip)
				} else {
					rl.attempts[ip] = valid
				}
			}
			rl.mu.Unlock()
		case <-rl.done:
			return
		}
	}
}

// IPRateLimit returns Echo middleware that rate-limits the wrapped handler.
// Requests exceeding the limit receive a 429 Too Many Requests response.
func IPRateLimit(rl *ipRateLimiter) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			if !rl.allow(ip) {
				slog.Warn("Rate limit exceeded", "component", "ratelimit", "ip", ip)
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "Too many login attempts. Please try again later.",
				})
			}
			return next(c)
		}
	}
}
