package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimiter provides per-IP rate limiting for authentication endpoints.
// It uses a simple token-bucket approach stored in memory. Expired entries
// are cleaned up periodically.
type RateLimiter struct {
	mu           sync.Mutex
	visitors     map[string]*visitor
	rate         int           // max attempts per window
	window       time.Duration // time window
	trustedNets  []*net.IPNet  // trusted proxy CIDRs
	stopCleanup  chan struct{} // signal to stop the cleanup goroutine
}

type visitor struct {
	count       int
	windowStart time.Time
}

// NewRateLimiter creates a rate limiter that allows `rate` requests per `window`
// per IP address. For example, NewRateLimiter(10, time.Minute) allows 10
// requests per minute per IP.
//
// trustedProxies is an optional list of CIDR strings (e.g., "127.0.0.1/32",
// "10.0.0.0/8") identifying reverse proxies whose X-Forwarded-For headers
// should be trusted. If empty, only RemoteAddr is used (safe default).
func NewRateLimiter(rate int, window time.Duration, trustedProxies ...string) *RateLimiter {
	var nets []*net.IPNet
	for _, cidr := range trustedProxies {
		// Allow bare IPs (e.g., "127.0.0.1") by appending /32 or /128.
		if !strings.Contains(cidr, "/") {
			if strings.Contains(cidr, ":") {
				cidr += "/128"
			} else {
				cidr += "/32"
			}
		}
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			continue // skip malformed entries
		}
		nets = append(nets, n)
	}

	rl := &RateLimiter{
		visitors:    make(map[string]*visitor),
		rate:        rate,
		window:      window,
		trustedNets: nets,
		stopCleanup: make(chan struct{}),
	}
	// Background cleanup of stale entries every 5 minutes.
	go rl.cleanup()
	return rl
}

// Stop terminates the background cleanup goroutine. Call on server shutdown.
func (rl *RateLimiter) Stop() {
	close(rl.stopCleanup)
}

// Limit wraps a handler and rejects requests that exceed the rate limit.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := rl.extractIP(r)

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
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-rl.stopCleanup:
			return
		case <-ticker.C:
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
}

// isTrustedProxy reports whether the given IP belongs to a trusted proxy CIDR.
func (rl *RateLimiter) isTrustedProxy(ipStr string) bool {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return false
	}
	for _, n := range rl.trustedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// extractIP returns the real client IP address. It only trusts
// X-Forwarded-For / X-Real-IP when the direct connection (RemoteAddr)
// comes from a configured trusted proxy. This prevents IP spoofing attacks
// where a client forges these headers to bypass rate limiting.
func (rl *RateLimiter) extractIP(r *http.Request) string {
	remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteIP = r.RemoteAddr
	}

	// Only trust proxy headers when the request comes from a known proxy.
	if len(rl.trustedNets) == 0 || !rl.isTrustedProxy(remoteIP) {
		return remoteIP
	}

	// Trust X-Forwarded-For: take the rightmost entry that is NOT a trusted proxy.
	// This is the last hop before the proxy chain, i.e., the real client.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			candidate := strings.TrimSpace(parts[i])
			if candidate != "" && !rl.isTrustedProxy(candidate) {
				return candidate
			}
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	return remoteIP
}
