// Package oidc implements RepLog's PocketID OIDC relying-party login for the
// webui (ADR 019 Phase 1 — HOF-012).
//
// RepLog federates webui authentication to PocketID via the Authorization Code
// flow with PKCE. This package owns the two browser-facing endpoints — Start
// (build PKCE + state + nonce, 302 to PocketID) and Callback (exchange the
// code, verify the ID token against PocketID's JWKS, resolve the user, and
// establish the existing scs session) — and nothing else. It does NOT mint
// tokens, run a consent UI, or touch the MCP bearer path; those are the AS
// (Phase 2) and the native MCP server (Phase 3).
//
// Identity resolution is delegated to models.UpsertUserFromOIDC, which keys on
// the PocketID `sub`, falls back to a verified-email bind for the one-time
// cutover, and JIT-creates otherwise. An empty `sub` is refused here at the
// request boundary (mirrors the bearer middleware's empty-claim 401) before any
// upsert runs.
package oidc

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/alexedwards/scs/v2"

	"github.com/carpenike/replog/internal/models"
)

// Session keys for the in-flight authorization transaction. These live in the
// scs session (server-side, sqlite-backed) between Start and Callback; the
// same session cookie round-trips because the PocketID hop is a top-level GET
// redirect (SameSite=Lax sends the cookie on return).
const (
	sessKeyState    = "oidc_state"
	sessKeyVerifier = "oidc_verifier"
	sessKeyNonce    = "oidc_nonce"
	sessKeyReturnTo = "oidc_return_to"
)

// Handler holds the configured OIDC relying-party state. Construct via New.
type Handler struct {
	db       *sql.DB
	sessions *scs.SessionManager
	verifier *gooidc.IDTokenVerifier
	oauth    *oauth2.Config
}

// BuildConfig discovers the PocketID provider and returns an ID-token verifier
// plus an oauth2.Config. It is the single shared constructor for both OIDC
// consumers so the webui relying party (Phase 1) and the MCP OAuth
// Authorization Server's PocketID hop (Phase 2/3) cannot drift on issuer,
// client credentials, or scopes — each caller supplies only its own
// redirectURL and the scopes it needs.
//
// Discovery performs a network call to the issuer's well-known document, so
// BuildConfig should be called once at startup per consumer.
func BuildConfig(ctx context.Context, issuer, clientID, clientSecret, redirectURL string, scopes []string) (*gooidc.IDTokenVerifier, *oauth2.Config, error) {
	if issuer == "" || clientID == "" || clientSecret == "" || redirectURL == "" {
		return nil, nil, errors.New("oidc: issuer, client id, client secret, and redirect url are all required")
	}

	provider, err := gooidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, nil, fmt.Errorf("oidc: discover provider at %q: %w", issuer, err)
	}

	return provider.Verifier(&gooidc.Config{ClientID: clientID}),
		&oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       scopes,
		}, nil
}

// New discovers the PocketID provider and builds the relying-party handler.
// issuer is the PocketID issuer URL (REPLOG_OIDC_ISSUER); clientID/clientSecret
// are the statically pre-registered confidential client (no DCR in this phase);
// redirectURL is this app's callback (REPLOG_OIDC_REDIRECT_URL), which must
// exactly match the redirect registered in PocketID.
//
// Discovery performs a network call to the issuer's well-known document, so
// New should be called once at startup. A failure here is fatal to OIDC login
// but the caller decides whether that's fatal to the process.
func New(ctx context.Context, db *sql.DB, sessions *scs.SessionManager, issuer, clientID, clientSecret, redirectURL string) (*Handler, error) {
	verifier, oauthCfg, err := BuildConfig(ctx, issuer, clientID, clientSecret, redirectURL,
		[]string{gooidc.ScopeOpenID, "profile", "email"})
	if err != nil {
		return nil, err
	}

	return &Handler{
		db:       db,
		sessions: sessions,
		verifier: verifier,
		oauth:    oauthCfg,
	}, nil
}

// Start begins the login: it generates an opaque state, a PKCE verifier, and a
// nonce, stashes them in the session, and 302s the browser to PocketID's
// authorization endpoint.
//
//	GET /auth/oidc/start
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	state, err := randString(24)
	if err != nil {
		log.Printf("oidc: start: generate state: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	nonce, err := randString(24)
	if err != nil {
		log.Printf("oidc: start: generate nonce: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	verifier := oauth2.GenerateVerifier()

	h.sessions.Put(r.Context(), sessKeyState, state)
	h.sessions.Put(r.Context(), sessKeyNonce, nonce)
	h.sessions.Put(r.Context(), sessKeyVerifier, verifier)

	// Preserve an optional in-app return target across the hop. Only relative
	// paths are honored on callback to avoid an open-redirect.
	if rt := r.URL.Query().Get("returnTo"); rt != "" {
		h.sessions.Put(r.Context(), sessKeyReturnTo, rt)
	}

	authURL := h.oauth.AuthCodeURL(
		state,
		gooidc.Nonce(nonce),
		oauth2.S256ChallengeOption(verifier),
	)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback completes the login: it validates state, exchanges the code (with
// the PKCE verifier), verifies the ID token and nonce, refuses an empty `sub`,
// resolves the user via models.UpsertUserFromOIDC, and establishes the scs
// session exactly as the password/magic-link paths do.
//
//	GET /auth/oidc/callback
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Surface a provider-side error without leaking it into a redirect target.
	if e := r.URL.Query().Get("error"); e != "" {
		log.Printf("oidc: callback: provider returned error=%q", e)
		h.fail(w, r, "provider_error")
		return
	}

	wantState := h.sessions.GetString(ctx, sessKeyState)
	verifier := h.sessions.GetString(ctx, sessKeyVerifier)
	wantNonce := h.sessions.GetString(ctx, sessKeyNonce)
	returnTo := sanitizeReturnTo(h.sessions.GetString(ctx, sessKeyReturnTo))

	// Single-use: clear the transaction regardless of outcome.
	h.sessions.Remove(ctx, sessKeyState)
	h.sessions.Remove(ctx, sessKeyVerifier)
	h.sessions.Remove(ctx, sessKeyNonce)
	h.sessions.Remove(ctx, sessKeyReturnTo)

	if wantState == "" || r.URL.Query().Get("state") != wantState {
		log.Printf("oidc: callback: state mismatch")
		h.fail(w, r, "state_mismatch")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		h.fail(w, r, "missing_code")
		return
	}

	oauth2Token, err := h.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		log.Printf("oidc: callback: code exchange failed: %v", err)
		h.fail(w, r, "exchange_failed")
		return
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		log.Printf("oidc: callback: no id_token in token response")
		h.fail(w, r, "no_id_token")
		return
	}

	idToken, err := h.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		log.Printf("oidc: callback: id token verification failed: %v", err)
		h.fail(w, r, "verify_failed")
		return
	}
	if idToken.Nonce != wantNonce {
		log.Printf("oidc: callback: nonce mismatch")
		h.fail(w, r, "nonce_mismatch")
		return
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		log.Printf("oidc: callback: claims decode failed: %v", err)
		h.fail(w, r, "claims_failed")
		return
	}

	// Empty-sub guard at the request boundary (mirrors bearer.go's empty-claim
	// rejection). UpsertUserFromOIDC also guards, but refuse early and loudly.
	if idToken.Subject == "" {
		log.Printf("oidc: callback: empty subject in id token")
		h.fail(w, r, "empty_sub")
		return
	}

	user, err := models.UpsertUserFromOIDC(ctx, h.db, idToken.Subject, claims.Email, claims.Name, claims.EmailVerified)
	if err != nil {
		log.Printf("oidc: callback: user upsert failed: %v", err)
		h.fail(w, r, "user_resolve_failed")
		return
	}

	// Establish the session exactly like the password / magic-link paths:
	// renew to prevent fixation, then put userID.
	if err := h.sessions.RenewToken(ctx); err != nil {
		log.Printf("oidc: callback: session renew failed: %v", err)
		h.fail(w, r, "session_failed")
		return
	}
	h.sessions.Put(ctx, "userID", user.ID)

	if err := models.EnsureUserPreferences(ctx, h.db, user.ID); err != nil {
		log.Printf("oidc: callback: ensure preferences for user %d: %v", user.ID, err)
	}

	log.Printf("oidc: login success for user %q (id=%d)", user.Username, user.ID)
	http.Redirect(w, r, returnTo, http.StatusFound)
}

// fail redirects an unauthenticated browser back to the login page with a
// machine-readable reason. The login page is the unauthenticated catch-all, so
// /login renders it; the reason is for display/telemetry only and is never a
// redirect target.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, reason string) {
	http.Redirect(w, r, "/login?error="+reason, http.StatusFound)
}

// sanitizeReturnTo permits only same-site absolute paths ("/foo"), defaulting
// to "/" — this prevents the returnTo parameter from becoming an open redirect.
func sanitizeReturnTo(rt string) string {
	if len(rt) >= 1 && rt[0] == '/' && (len(rt) == 1 || rt[1] != '/') {
		return rt
	}
	return "/"
}

// randString returns a URL-safe random string with nBytes of entropy.
func randString(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
