package mcpoauth

import (
	"sync"
	"time"
)

// In-memory transaction stores for the OAuth Authorization Server.
//
// The AS holds two short-lived, single-use artifacts entirely in memory (no
// DB): the authorize-state (the bridge across the PocketID PKCE hop) and the
// issued authorization code (the bridge from /oauth/callback to /oauth/token).
// Both live ~5 minutes and are consumed exactly once. Memory-only is correct
// here: RepLog is a single process, these artifacts are worthless after their
// short window, and persisting them would only add a cleanup burden. A process
// restart mid-handshake simply makes the client retry — acceptable.

const transactionTTL = 5 * time.Minute

// authorizeState bridges /oauth/authorize → PocketID → /oauth/callback. It
// carries the client's original PKCE/state alongside the AS↔PocketID PKCE
// verifier and nonce so the two PKCE legs never cross.
type authorizeState struct {
	claudeState         string
	claudeCodeChallenge string
	claudeClientID      string
	claudeRedirectURI   string
	pocketidVerifier    string
	nonce               string
	scope               string
	expiresAt           time.Time
}

// authzCode bridges /oauth/callback → /oauth/token. It binds the resolved user
// to the client/redirect/PKCE-challenge so the token endpoint can verify the
// presented code_verifier and mint a token for the right identity.
type authzCode struct {
	userID              int64
	clientID            string
	redirectURI         string
	claudeCodeChallenge string
	scope               string
	expiresAt           time.Time
}

type stateStore struct {
	mu sync.Mutex
	m  map[string]authorizeState
}

func newStateStore() *stateStore { return &stateStore{m: make(map[string]authorizeState)} }

// put records a state under key (the opaque state value sent to PocketID),
// sweeping expired entries first to bound memory.
func (s *stateStore) put(key string, v authorizeState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, e := range s.m {
		if now.After(e.expiresAt) {
			delete(s.m, k)
		}
	}
	s.m[key] = v
}

// take returns and removes the state for key (single-use). ok is false when
// the key is unknown or expired.
func (s *stateStore) take(key string) (authorizeState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[key]
	if !ok {
		return authorizeState{}, false
	}
	delete(s.m, key)
	if time.Now().After(v.expiresAt) {
		return authorizeState{}, false
	}
	return v, true
}

type codeStore struct {
	mu sync.Mutex
	m  map[string]authzCode
}

func newCodeStore() *codeStore { return &codeStore{m: make(map[string]authzCode)} }

func (s *codeStore) put(code string, v authzCode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, e := range s.m {
		if now.After(e.expiresAt) {
			delete(s.m, k)
		}
	}
	s.m[code] = v
}

func (s *codeStore) take(code string) (authzCode, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[code]
	if !ok {
		return authzCode{}, false
	}
	delete(s.m, code)
	if time.Now().After(v.expiresAt) {
		return authzCode{}, false
	}
	return v, true
}

// ipRateLimiter is a fixed-window per-IP counter for the DCR registration
// endpoint (RFC 7591 §5 advises rate-limiting open registration). It mirrors
// the reference AS's 10-per-hour cap. Lightweight and in-memory; a restart
// resets all windows, which is harmless for an abuse guard.
type ipRateLimiter struct {
	mu     sync.Mutex
	cap    int
	window time.Duration
	hits   map[string]*ipWindow
}

type ipWindow struct {
	count   int
	resetAt time.Time
}

func newIPRateLimiter(cap int, window time.Duration) *ipRateLimiter {
	return &ipRateLimiter{cap: cap, window: window, hits: make(map[string]*ipWindow)}
}

// allow records a hit for ip and reports whether it is within the cap.
func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	w, ok := l.hits[ip]
	if !ok || now.After(w.resetAt) {
		l.hits[ip] = &ipWindow{count: 1, resetAt: now.Add(l.window)}
		return true
	}
	if w.count >= l.cap {
		return false
	}
	w.count++
	return true
}
