package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/alexedwards/scs/v2"

	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/models"
)

const (
	testClientID     = "test-client"
	testClientSecret = "test-secret"
	testRedirectURL  = "http://127.0.0.1/auth/oidc/callback"
)

// TestSanitizeReturnTo covers the open-redirect guard on the returnTo param.
func TestSanitizeReturnTo(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/foo", "/foo"},
		{"/foo/bar?q=1", "/foo/bar?q=1"},
		{"//evil.com", "/"},
		{"https://x", "/"},
		{"http://evil.com/foo", "/"},
		{"", "/"},
		{"/", "/"},
		{"foo", "/"},     // no leading slash
		{"\\/evil", "/"}, // backslash, not a same-site path
	}
	for _, c := range cases {
		if got := sanitizeReturnTo(c.in); got != c.want {
			t.Errorf("sanitizeReturnTo(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestRandString sanity-checks the entropy helper: distinct values, URL-safe.
func TestRandString(t *testing.T) {
	a, err := randString(24)
	if err != nil {
		t.Fatalf("randString: %v", err)
	}
	b, err := randString(24)
	if err != nil {
		t.Fatalf("randString: %v", err)
	}
	if a == "" || b == "" {
		t.Fatal("randString returned an empty string")
	}
	if a == b {
		t.Errorf("randString produced identical values %q", a)
	}
	if _, err := url.Parse("https://x/" + a); err != nil {
		t.Errorf("randString produced a non-URL-safe value %q: %v", a, err)
	}
}

// ---------------------------------------------------------------------------
// Stub OpenID Provider (IdP)
// ---------------------------------------------------------------------------

// tokenClaims is the set of id_token claims the stub's /token endpoint will
// mint on the next exchange. Tests set it before driving Callback.
type tokenClaims struct {
	sub           string
	email         string
	name          string
	nonce         string
	emailVerified bool
}

// stubIdP is a minimal, local, hermetic OpenID Provider: it serves discovery,
// a JWKS keyed to an in-memory RSA key, and a /token endpoint that returns a
// freshly signed RS256 id_token carrying whatever claims the test configured.
type stubIdP struct {
	server *httptest.Server
	priv   *rsa.PrivateKey
	keyID  string

	mu  sync.Mutex
	tok tokenClaims
}

func newStubIdP(t *testing.T) *stubIdP {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	s := &stubIdP{priv: priv, keyID: "test-key-1"}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", s.handleDiscovery)
	mux.HandleFunc("/jwks", s.handleJWKS)
	mux.HandleFunc("/token", s.handleToken)
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		// The relying party never follows the browser redirect in these tests;
		// Callback is driven directly. Fail loudly if something does hit it.
		http.Error(w, "authorize endpoint is not exercised in tests", http.StatusNotImplemented)
	})

	s.server = httptest.NewServer(mux)
	t.Cleanup(s.server.Close)
	return s
}

// setToken configures the claims for the next /token exchange.
func (s *stubIdP) setToken(tc tokenClaims) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tok = tc
}

func (s *stubIdP) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	base := s.server.URL
	doc := map[string]any{
		"issuer":                                base,
		"authorization_endpoint":                base + "/authorize",
		"token_endpoint":                        base + "/token",
		"jwks_uri":                              base + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(doc)
}

func (s *stubIdP) handleJWKS(w http.ResponseWriter, r *http.Request) {
	jwk := jose.JSONWebKey{
		Key:       &s.priv.PublicKey,
		KeyID:     s.keyID,
		Algorithm: "RS256",
		Use:       "sig",
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}})
}

func (s *stubIdP) handleToken(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	tc := s.tok
	s.mu.Unlock()

	now := time.Now()
	claims := map[string]any{
		"iss":            s.server.URL,
		"aud":            testClientID,
		"sub":            tc.sub,
		"exp":            now.Add(time.Hour).Unix(),
		"iat":            now.Unix(),
		"nonce":          tc.nonce,
		"email":          tc.email,
		"email_verified": tc.emailVerified,
		"name":           tc.name,
	}

	resp := map[string]any{
		"access_token": "stub-access-token",
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     s.signIDToken(w, claims),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// signIDToken serializes claims into a compact RS256 JWT signed by the stub's
// key. The kid header is set from the JSONWebKey KeyID so the relying party can
// match it against the JWKS entry.
func (s *stubIdP) signIDToken(w http.ResponseWriter, claims map[string]any) string {
	signer, err := jose.NewSigner(
		jose.SigningKey{
			Algorithm: jose.RS256,
			Key:       jose.JSONWebKey{Key: s.priv, KeyID: s.keyID},
		},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		http.Error(w, "sign: "+err.Error(), http.StatusInternalServerError)
		return ""
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		http.Error(w, "marshal: "+err.Error(), http.StatusInternalServerError)
		return ""
	}
	jws, err := signer.Sign(payload)
	if err != nil {
		http.Error(w, "sign payload: "+err.Error(), http.StatusInternalServerError)
		return ""
	}
	serialized, err := jws.CompactSerialize()
	if err != nil {
		http.Error(w, "serialize: "+err.Error(), http.StatusInternalServerError)
		return ""
	}
	return serialized
}

// ---------------------------------------------------------------------------
// Test harness helpers
// ---------------------------------------------------------------------------

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := database.RunMigrations(db); err != nil {
		_ = db.Close()
		t.Fatalf("run migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newTestSessions() *scs.SessionManager {
	sm := scs.New()
	sm.Lifetime = 30 * 24 * time.Hour
	return sm
}

// serve runs an http.HandlerFunc through the scs LoadAndSave middleware so the
// session context is populated exactly as in production. Any inbound cookies
// are attached; the returned recorder carries the (possibly renewed) cookie.
func serve(sm *scs.SessionManager, h http.HandlerFunc, target string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	sm.LoadAndSave(h).ServeHTTP(rr, req)
	return rr
}

// mergeCookies overlays response cookies onto the prior jar, keyed by name, so
// a renewed session cookie (RenewToken issues a new token) supersedes the old
// one instead of both being sent under the same name.
func mergeCookies(prior, resp []*http.Cookie) []*http.Cookie {
	byName := map[string]*http.Cookie{}
	for _, c := range prior {
		byName[c.Name] = c
	}
	for _, c := range resp {
		byName[c.Name] = c
	}
	out := make([]*http.Cookie, 0, len(byName))
	for _, c := range byName {
		out = append(out, c)
	}
	return out
}

// sessionUserID loads the session referenced by cookies and reports the stored
// userID (0 / false when no session was established).
func sessionUserID(sm *scs.SessionManager, cookies []*http.Cookie) (int64, bool) {
	var (
		id int64
		ok bool
	)
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sm.Exists(r.Context(), "userID") {
			id = sm.GetInt64(r.Context(), "userID")
			ok = true
		}
	})
	serve(sm, probe, "/probe", cookies)
	return id, ok
}

// startLogin drives Start and returns the session cookie plus the state and
// nonce it stashed (recovered from the authorization redirect URL).
func startLogin(t *testing.T, sm *scs.SessionManager, h *Handler) (cookies []*http.Cookie, state, nonce string) {
	t.Helper()
	rr := serve(sm, h.Start, "/auth/oidc/start", nil)
	if rr.Code != http.StatusFound {
		t.Fatalf("Start: status = %d, want 302", rr.Code)
	}
	loc, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Start: parse Location %q: %v", rr.Header().Get("Location"), err)
	}
	q := loc.Query()
	state = q.Get("state")
	nonce = q.Get("nonce")
	if state == "" || nonce == "" {
		t.Fatalf("Start: authorize URL missing state/nonce: %q", loc.String())
	}
	return rr.Result().Cookies(), state, nonce
}

func newHandler(t *testing.T, db *sql.DB, sm *scs.SessionManager, issuer string) *Handler {
	t.Helper()
	h, err := New(context.Background(), db, sm, issuer, testClientID, testClientSecret, testRedirectURL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h
}

// ---------------------------------------------------------------------------
// Callback tests
// ---------------------------------------------------------------------------

// TestCallback_StateMismatch: a callback whose state does not match the session
// state redirects to /login?error=state_mismatch and creates no session.
func TestCallback_StateMismatch(t *testing.T) {
	db := testDB(t)
	sm := newTestSessions()
	idp := newStubIdP(t)
	h := newHandler(t, db, sm, idp.server.URL)

	cookies, _, _ := startLogin(t, sm, h)

	rr := serve(sm, h.Callback, "/auth/oidc/callback?state=WRONG&code=abc", cookies)
	if rr.Code != http.StatusFound {
		t.Fatalf("Callback: status = %d, want 302", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/login?error=state_mismatch" {
		t.Errorf("Callback: Location = %q, want /login?error=state_mismatch", got)
	}

	// The token endpoint must never have been called and no session established.
	respCookies := mergeCookies(cookies, rr.Result().Cookies())
	if _, ok := sessionUserID(sm, respCookies); ok {
		t.Error("Callback established a session on state mismatch")
	}
}

// TestCallback_HappyPath drives the full Authorization Code flow against the
// stub IdP and asserts a session is established and the user resolved.
func TestCallback_HappyPath(t *testing.T) {
	db := testDB(t)
	sm := newTestSessions()
	idp := newStubIdP(t)
	h := newHandler(t, db, sm, idp.server.URL)

	cookies, state, nonce := startLogin(t, sm, h)

	const sub = "pocketid-sub-123"
	idp.setToken(tokenClaims{
		sub:           sub,
		email:         "athlete@example.com",
		name:          "Test Athlete",
		nonce:         nonce,
		emailVerified: true,
	})

	rr := serve(sm, h.Callback, "/auth/oidc/callback?state="+url.QueryEscape(state)+"&code=good-code", cookies)
	if rr.Code != http.StatusFound {
		t.Fatalf("Callback: status = %d, want 302; body=%q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Location"); got != "/" {
		t.Errorf("Callback: Location = %q, want / (default returnTo)", got)
	}

	// The upsert must have created/resolved the user keyed on sub.
	user, err := models.GetUserByPocketIDSub(db, sub)
	if err != nil {
		t.Fatalf("GetUserByPocketIDSub: %v", err)
	}

	// The session must carry that user's ID. RenewToken issues a fresh cookie.
	respCookies := mergeCookies(cookies, rr.Result().Cookies())
	id, ok := sessionUserID(sm, respCookies)
	if !ok {
		t.Fatal("Callback did not establish a session on success")
	}
	if id != user.ID {
		t.Errorf("session userID = %d, want %d", id, user.ID)
	}
}

// TestCallback_NonceMismatch: a valid, correctly signed id_token whose nonce
// does not match the session nonce is rejected — /login?error=nonce_mismatch,
// no session.
func TestCallback_NonceMismatch(t *testing.T) {
	db := testDB(t)
	sm := newTestSessions()
	idp := newStubIdP(t)
	h := newHandler(t, db, sm, idp.server.URL)

	cookies, state, _ := startLogin(t, sm, h)

	idp.setToken(tokenClaims{
		sub:           "pocketid-sub-nonce",
		email:         "athlete@example.com",
		nonce:         "not-the-session-nonce",
		emailVerified: true,
	})

	rr := serve(sm, h.Callback, "/auth/oidc/callback?state="+url.QueryEscape(state)+"&code=good-code", cookies)
	if rr.Code != http.StatusFound {
		t.Fatalf("Callback: status = %d, want 302", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/login?error=nonce_mismatch" {
		t.Errorf("Callback: Location = %q, want /login?error=nonce_mismatch", got)
	}

	respCookies := mergeCookies(cookies, rr.Result().Cookies())
	if _, ok := sessionUserID(sm, respCookies); ok {
		t.Error("Callback established a session on nonce mismatch")
	}
	// And no user should have been upserted.
	if _, err := models.GetUserByPocketIDSub(db, "pocketid-sub-nonce"); err == nil {
		t.Error("Callback upserted a user despite the nonce mismatch")
	}
}

// TestCallback_EmptySub: a correctly signed token with a matching nonce but an
// empty subject is refused at the request boundary — /login?error=empty_sub,
// no session, no upsert.
func TestCallback_EmptySub(t *testing.T) {
	db := testDB(t)
	sm := newTestSessions()
	idp := newStubIdP(t)
	h := newHandler(t, db, sm, idp.server.URL)

	cookies, state, nonce := startLogin(t, sm, h)

	idp.setToken(tokenClaims{
		sub:           "", // refused before any upsert
		email:         "athlete@example.com",
		nonce:         nonce,
		emailVerified: true,
	})

	rr := serve(sm, h.Callback, "/auth/oidc/callback?state="+url.QueryEscape(state)+"&code=good-code", cookies)
	if rr.Code != http.StatusFound {
		t.Fatalf("Callback: status = %d, want 302", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/login?error=empty_sub" {
		t.Errorf("Callback: Location = %q, want /login?error=empty_sub", got)
	}

	respCookies := mergeCookies(cookies, rr.Result().Cookies())
	if _, ok := sessionUserID(sm, respCookies); ok {
		t.Error("Callback established a session for an empty subject")
	}
}
