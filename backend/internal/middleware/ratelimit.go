package middleware

import (
	"net/http"
	"sync"
	"time"
)

type rateLimitEntry struct {
	count    int
	windowAt time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateLimitEntry
	limit   int
	window  time.Duration
	cleanAt time.Time
}

// NewRateLimiter creates a limiter that allows up to limit requests per window per IP.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		entries: make(map[string]*rateLimitEntry),
		limit:   limit,
		window:  window,
		cleanAt: time.Now().Add(window * 10),
	}
}

// Allow counts one request for key and reports whether it is within the limit.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.record(key, time.Now()) <= rl.limit
}

// Exceeded reports whether key has already used up its quota in the current
// window, WITHOUT counting this call. Use it with Record to limit only some
// outcomes (e.g. failed attempts).
func (rl *RateLimiter) Exceeded(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	e, ok := rl.entries[key]
	return ok && time.Since(e.windowAt) <= rl.window && e.count >= rl.limit
}

// TryAcquire atomically reserves one unit of key's quota: it increments the
// counter under the lock and returns false (without incrementing) when the
// quota for the current window is already used up. Pair it with Release to
// refund the unit when the outcome should not count.
func (rl *RateLimiter) TryAcquire(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	if e, ok := rl.entries[key]; ok && now.Sub(e.windowAt) <= rl.window && e.count >= rl.limit {
		return false
	}
	rl.record(key, now)
	return true
}

// Release refunds one unit previously reserved with TryAcquire (no-op when
// the window already elapsed or nothing is reserved).
func (rl *RateLimiter) Release(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if e, ok := rl.entries[key]; ok && time.Since(e.windowAt) <= rl.window && e.count > 0 {
		e.count--
	}
}

// Record counts one event for key.
func (rl *RateLimiter) Record(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.record(key, time.Now())
}

// record increments key's counter (starting a new window when the previous
// one elapsed) and returns the new count. Caller holds rl.mu.
func (rl *RateLimiter) record(key string, now time.Time) int {
	// Periodic cleanup to avoid unbounded map growth
	if now.After(rl.cleanAt) {
		for k, e := range rl.entries {
			if now.Sub(e.windowAt) > rl.window {
				delete(rl.entries, k)
			}
		}
		rl.cleanAt = now.Add(rl.window * 10)
	}

	e, ok := rl.entries[key]
	if !ok || now.Sub(e.windowAt) > rl.window {
		rl.entries[key] = &rateLimitEntry{count: 1, windowAt: now}
		return 1
	}
	e.count++
	return e.count
}

// Limit returns middleware that rate-limits by client IP (see ClientIP).
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Key on the real client IP (honours TRUSTED_PROXIES), normalized with
		// LimiterIP (IPv6 by /64); fall back to the raw peer address when no
		// valid IP can be determined.
		ip := LimiterIP(ClientIP(r))
		if ip == "" {
			ip = r.RemoteAddr
		}
		if !rl.Allow(ip) {
			writeJSONError(w, http.StatusTooManyRequests, "Demasiadas solicitudes. Intenta más tarde.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// writeJSONError answers the 429 of the IP rate limiter in the legacy format.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
