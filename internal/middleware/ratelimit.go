package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter provides per-IP rate limiting for authentication endpoints.
// It uses a simple token-bucket approach stored in memory. Expired entries
// are cleaned up periodically.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     int           // max attempts per window
	window   time.Duration // time window
}

type visitor struct {
	count    int
	windowStart time.Time
}

// NewRateLimiter creates a rate limiter that allows `rate` requests per `window`
// per IP address. For example, NewRateLimiter(10, time.Minute) allows 10
// requests per minute per IP.
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		window:   window,
	}
	// Background cleanup of stale entries every 5 minutes.
	go rl.cleanup()
	return rl
}

// Limit wraps a handler and rejects requests that exceed the rate limit.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)

		if !rl.allow(ip) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too many requests — please try again later", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// allow checks whether the given IP is within the rate limit and records the attempt.
func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, exists := rl.visitors[ip]

	if !exists || now.Sub(v.windowStart) > rl.window {
		// New window.
		rl.visitors[ip] = &visitor{count: 1, windowStart: now}
		return true
	}

	v.count++
	return v.count <= rl.rate
}

// cleanup removes expired visitor entries periodically.
func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		rl.mu.Lock()
		now := time.Now()
		for ip, v := range rl.visitors {
			if now.Sub(v.windowStart) > rl.window*2 {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// extractIP returns the client's IP address, checking X-Forwarded-For and
// X-Real-IP headers for proxied requests before falling back to RemoteAddr.
func extractIP(r *http.Request) string {
	// Trust X-Forwarded-For (first entry) for reverse proxy setups.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can be comma-separated; take the first (client) IP.
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
