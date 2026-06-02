// Package mcpoauth implements RepLog's MCP OAuth 2.1 Authorization Server
// (ADR 019 Phases 2+3 — HOF-013).
//
// RepLog stops trusting an external homelab-mcp Authorization Server (the
// retired JWKS/RS256 path) and becomes its OWN AS for the native `/api/mcp`
// server. It exposes:
//
//   - RFC 8414 AS metadata at /.well-known/oauth-authorization-server
//     (jwks_uri deliberately omitted — RepLog mints opaque, not JWT, tokens).
//   - RFC 9728 Protected Resource Metadata in TWO variants: the bare
//     /.well-known/oauth-protected-resource (resource = origin) AND the
//     path-suffixed /.well-known/oauth-protected-resource/api/mcp (resource =
//     origin + /api/mcp). VS Code 1.106–1.107 require the suffixed variant.
//   - RFC 7591 Dynamic Client Registration at /oauth/register (rate-limited,
//     redirect_uris allowlist-filtered).
//   - The authorization-code-with-PKCE endpoints /oauth/authorize,
//     /oauth/callback, and /oauth/token.
//
// The human login is federated to PocketID over a SERVER-SIDE PKCE hop: the AS
// is itself a PocketID OIDC client. The client's PKCE (Claude/VS Code ↔ RepLog)
// and the AS's PKCE (RepLog ↔ PocketID) are kept in separate legs that never
// cross. On success the AS mints an opaque `rlpat_` bearer token bound to the
// resolved RepLog user (no token-scoped role — access is the user's existing
// authz, enforced by the reused handlers).
//
// The flow's transaction state lives in memory (see store.go), never in the
// scs cookie session — the API client has no browser session at the AS hop.
package mcpoauth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/carpenike/replog/internal/models"
	"github.com/carpenike/replog/internal/oidc"
)

// redirectURIAllowlist is the set of prefixes a DCR client's redirect_uris are
// filtered against. The loopback entries carry a trailing ":" or "/" on
// purpose: it stops "http://127.0.0.1.evil.com/" (which startsWith
// "http://127.0.0.1" would otherwise wrongly accept) from passing.
var redirectURIAllowlist = []string{
	"https://claude.ai/",
	"https://claude.com/",
	"http://127.0.0.1:",
	"http://127.0.0.1/",
	"http://localhost:",
	"http://localhost/",
	"https://vscode.dev/redirect",
	"https://insiders.vscode.dev/redirect",
}

func isAllowedRedirectURI(uri string) bool {
	for _, prefix := range redirectURIAllowlist {
		if strings.HasPrefix(uri, prefix) {
			return true
		}
	}
	return false
}

// Server is the configured MCP OAuth Authorization Server. Construct via New.
type Server struct {
	db       *sql.DB
	origin   string // REPLOG_BASE_URL with any trailing slash trimmed; the AS issuer/resource
	verifier *gooidc.IDTokenVerifier
	oauth    *oauth2.Config // PocketID hop; RedirectURL = origin + /oauth/callback

	states  *stateStore
	codes   *codeStore
	regRate *ipRateLimiter
}

// New discovers the PocketID provider (sharing oidc.BuildConfig with the webui
// relying party so the two flows can't drift) and builds the AS. origin is the
// canonical public base URL (REPLOG_BASE_URL); issuer/clientID/clientSecret are
// the same PocketID confidential client the webui uses (PocketID must register
// BOTH redirect URIs — the webui callback and origin+/oauth/callback).
func New(ctx context.Context, db *sql.DB, origin, issuer, clientID, clientSecret string) (*Server, error) {
	origin = strings.TrimRight(origin, "/")
	if origin == "" {
		return nil, errors.New("mcpoauth: origin (REPLOG_BASE_URL) is required")
	}

	verifier, oauthCfg, err := oidc.BuildConfig(ctx, issuer, clientID, clientSecret,
		origin+"/oauth/callback", []string{gooidc.ScopeOpenID, "profile", "email"})
	if err != nil {
		return nil, err
	}

	return &Server{
		db:       db,
		origin:   origin,
		verifier: verifier,
		oauth:    oauthCfg,
		states:   newStateStore(),
		codes:    newCodeStore(),
		regRate:  newIPRateLimiter(10, time.Hour),
	}, nil
}

// RegisterRoutes mounts the AS endpoints on r at the application root. These
// are public (no session/bearer middleware); the registration endpoint is
// self-rate-limited.
func (s *Server) RegisterRoutes(r chi.Router) {
	r.Get("/.well-known/oauth-authorization-server", s.handleASMetadata)
	r.Get("/.well-known/oauth-protected-resource", s.handlePRMRoot)
	r.Get("/.well-known/oauth-protected-resource/api/mcp", s.handlePRMResource)
	r.Post("/oauth/register", s.handleRegister)
	r.Get("/oauth/authorize", s.handleAuthorize)
	r.Get("/oauth/callback", s.handleCallback)
	r.Post("/oauth/token", s.handleToken)
}

// --- metadata ---------------------------------------------------------------

func (s *Server) handleASMetadata(w http.ResponseWriter, r *http.Request) {
	writeCachedJSON(w, http.StatusOK, map[string]any{
		"issuer":                                s.origin,
		"authorization_endpoint":                s.origin + "/oauth/authorize",
		"token_endpoint":                        s.origin + "/oauth/token",
		"registration_endpoint":                 s.origin + "/oauth/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic"},
		"scopes_supported":                      []string{"openid", "profile", "email"},
	})
}

func (s *Server) handlePRMRoot(w http.ResponseWriter, r *http.Request) {
	writeCachedJSON(w, http.StatusOK, map[string]any{
		"resource":              s.origin,
		"authorization_servers": []string{s.origin},
	})
}

func (s *Server) handlePRMResource(w http.ResponseWriter, r *http.Request) {
	writeCachedJSON(w, http.StatusOK, map[string]any{
		"resource":              s.origin + "/api/mcp",
		"authorization_servers": []string{s.origin},
	})
}

// PRMResourceURL is the path-suffixed Protected Resource Metadata URL the
// /api/mcp 401 advertises in its WWW-Authenticate header.
func (s *Server) PRMResourceURL() string {
	return s.origin + "/.well-known/oauth-protected-resource/api/mcp"
}

// --- dynamic client registration -------------------------------------------

type registerRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !s.regRate.allow(clientIP(r)) {
		oauthError(w, http.StatusTooManyRequests, "temporarily_unavailable", "registration rate limit exceeded")
		return
	}

	var req registerRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "invalid registration body")
		return
	}

	// Filter — don't reject — unknown redirect URIs (RFC 7591 §2: the AS may
	// drop unacceptable values). Only 400 when nothing acceptable remains.
	var filtered []string
	for _, u := range req.RedirectURIs {
		if isAllowedRedirectURI(u) {
			filtered = append(filtered, u)
		}
	}
	if len(filtered) == 0 {
		oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", "no acceptable redirect_uris")
		return
	}

	authMethod := req.TokenEndpointAuthMethod
	if authMethod == "" {
		authMethod = "client_secret_post"
	}

	client, secret, err := models.RegisterDCRClient(s.db, req.ClientName, filtered, authMethod)
	if err != nil {
		log.Printf("mcpoauth: register client: %v", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "could not register client")
		return
	}

	writeNoStoreJSON(w, http.StatusCreated, map[string]any{
		"client_id":                  client.ClientID,
		"client_secret":              secret,
		"client_id_issued_at":        client.CreatedAt.Unix(),
		"client_secret_expires_at":   0, // never expires
		"client_name":                client.ClientName,
		"redirect_uris":              client.RedirectURIs,
		"token_endpoint_auth_method": client.TokenEndpointAuthMethod,
		"grant_types":                []string{"authorization_code"},
		"response_types":             []string{"code"},
	})
}

// --- authorize --------------------------------------------------------------

func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	if q.Get("response_type") != "code" {
		oauthError(w, http.StatusBadRequest, "unsupported_response_type", "only response_type=code is supported")
		return
	}

	clientID := q.Get("client_id")
	client, err := models.GetDCRClient(s.db, clientID)
	if err != nil {
		// Unknown client: we cannot trust any redirect, so fail in-band.
		oauthError(w, http.StatusBadRequest, "invalid_client", "unknown client_id")
		return
	}

	redirectURI := q.Get("redirect_uri")
	if redirectURI == "" || !client.HasRedirectURI(redirectURI) {
		// Untrusted redirect: never redirect an error to it.
		oauthError(w, http.StatusBadRequest, "invalid_request", "redirect_uri not registered for this client")
		return
	}

	codeChallenge := q.Get("code_challenge")
	if codeChallenge == "" || q.Get("code_challenge_method") != "S256" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "code_challenge with code_challenge_method=S256 is required")
		return
	}

	scope := q.Get("scope")

	// Build the AS↔PocketID leg: a fresh state, PKCE verifier, and nonce that
	// are wholly independent of the client's PKCE.
	asState, err := randString(24)
	if err != nil {
		oauthError(w, http.StatusInternalServerError, "server_error", "could not start authorization")
		return
	}
	nonce, err := randString(24)
	if err != nil {
		oauthError(w, http.StatusInternalServerError, "server_error", "could not start authorization")
		return
	}
	pocketidVerifier := oauth2.GenerateVerifier()

	s.states.put(asState, authorizeState{
		claudeState:         q.Get("state"),
		claudeCodeChallenge: codeChallenge,
		claudeClientID:      clientID,
		claudeRedirectURI:   redirectURI,
		pocketidVerifier:    pocketidVerifier,
		nonce:               nonce,
		scope:               scope,
		expiresAt:           time.Now().Add(transactionTTL),
	})

	authURL := s.oauth.AuthCodeURL(
		asState,
		gooidc.Nonce(nonce),
		oauth2.S256ChallengeOption(pocketidVerifier),
	)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// --- callback (from PocketID) ----------------------------------------------

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	// A provider-side error: only forward to the client's redirect if we can
	// validate the state. Never forward to an unvalidated redirect.
	if e := q.Get("error"); e != "" {
		st, ok := s.states.take(q.Get("state"))
		if !ok {
			oauthError(w, http.StatusBadRequest, "invalid_request", "unknown or expired authorization state")
			return
		}
		log.Printf("mcpoauth: callback: provider error=%q", e)
		redirectError(w, r, st.claudeRedirectURI, st.claudeState, "access_denied", "identity provider returned an error")
		return
	}

	st, ok := s.states.take(q.Get("state"))
	if !ok {
		oauthError(w, http.StatusBadRequest, "invalid_request", "unknown or expired authorization state")
		return
	}

	code := q.Get("code")
	if code == "" {
		redirectError(w, r, st.claudeRedirectURI, st.claudeState, "invalid_request", "missing code")
		return
	}

	oauth2Token, err := s.oauth.Exchange(ctx, code, oauth2.VerifierOption(st.pocketidVerifier))
	if err != nil {
		log.Printf("mcpoauth: callback: code exchange failed: %v", err)
		redirectError(w, r, st.claudeRedirectURI, st.claudeState, "access_denied", "identity exchange failed")
		return
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		redirectError(w, r, st.claudeRedirectURI, st.claudeState, "access_denied", "no id_token from identity provider")
		return
	}

	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		log.Printf("mcpoauth: callback: id token verification failed: %v", err)
		redirectError(w, r, st.claudeRedirectURI, st.claudeState, "access_denied", "id_token verification failed")
		return
	}
	if idToken.Nonce != st.nonce {
		log.Printf("mcpoauth: callback: nonce mismatch")
		redirectError(w, r, st.claudeRedirectURI, st.claudeState, "access_denied", "nonce mismatch")
		return
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		redirectError(w, r, st.claudeRedirectURI, st.claudeState, "access_denied", "could not read identity claims")
		return
	}
	if idToken.Subject == "" {
		redirectError(w, r, st.claudeRedirectURI, st.claudeState, "access_denied", "identity provider returned an empty subject")
		return
	}

	// Resolve identity exactly like the webui RP — keyed on the PocketID sub,
	// with the verified-email bind and JIT-create handled in the model. The
	// REAL email_verified claim is passed through (not hardcoded), preserving
	// RepLog's verified-email hardening.
	user, err := models.UpsertUserFromOIDC(s.db, idToken.Subject, claims.Email, claims.Name, claims.EmailVerified)
	if err != nil {
		log.Printf("mcpoauth: callback: user upsert failed: %v", err)
		redirectError(w, r, st.claudeRedirectURI, st.claudeState, "access_denied", "could not resolve user")
		return
	}

	// Issue the client-facing authorization code, binding the resolved user to
	// the client/redirect/PKCE so the token endpoint can verify the verifier.
	authCode, err := randString(32)
	if err != nil {
		redirectError(w, r, st.claudeRedirectURI, st.claudeState, "server_error", "could not issue code")
		return
	}
	s.codes.put(authCode, authzCode{
		userID:              user.ID,
		clientID:            st.claudeClientID,
		redirectURI:         st.claudeRedirectURI,
		claudeCodeChallenge: st.claudeCodeChallenge,
		scope:               st.scope,
		expiresAt:           time.Now().Add(transactionTTL),
	})

	// 302 back to the client with code + original state.
	sep := "?"
	if strings.Contains(st.claudeRedirectURI, "?") {
		sep = "&"
	}
	dest := st.claudeRedirectURI + sep + "code=" + url.QueryEscape(authCode)
	if st.claudeState != "" {
		dest += "&state=" + url.QueryEscape(st.claudeState)
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

// --- token ------------------------------------------------------------------

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", "could not parse request")
		return
	}

	if r.PostForm.Get("grant_type") != "authorization_code" {
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "only authorization_code is supported")
		return
	}

	clientID, clientSecret, ok := extractClientCredentials(r)
	if !ok {
		oauthError(w, http.StatusUnauthorized, "invalid_client", "missing client credentials")
		return
	}

	if _, err := models.ValidateDCRClientSecret(s.db, clientID, clientSecret); err != nil {
		oauthError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return
	}

	code := r.PostForm.Get("code")
	authz, ok := s.codes.take(code)
	if !ok {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "unknown or expired authorization code")
		return
	}

	if authz.clientID != clientID {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "code was not issued to this client")
		return
	}
	if authz.redirectURI != r.PostForm.Get("redirect_uri") {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri mismatch")
		return
	}

	if !verifyPKCES256(r.PostForm.Get("code_verifier"), authz.claudeCodeChallenge) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
		return
	}

	plaintext, _, err := models.CreateMCPToken(s.db, authz.userID, clientID, "")
	if err != nil {
		log.Printf("mcpoauth: token: mint failed: %v", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "could not issue token")
		return
	}

	writeNoStoreJSON(w, http.StatusOK, map[string]any{
		"access_token": plaintext,
		"token_type":   "Bearer",
		"expires_in":   int(models.MCPTokenLifetime.Seconds()),
		"scope":        authz.scope,
	})
}

// --- helpers ----------------------------------------------------------------

// extractClientCredentials reads client_id/client_secret from either
// client_secret_post (form fields) or HTTP Basic (client_secret_basic).
func extractClientCredentials(r *http.Request) (clientID, clientSecret string, ok bool) {
	if id, secret, hasBasic := r.BasicAuth(); hasBasic {
		return id, secret, id != "" && secret != ""
	}
	id := r.PostForm.Get("client_id")
	secret := r.PostForm.Get("client_secret")
	return id, secret, id != "" && secret != ""
}

// verifyPKCES256 reports whether base64url(sha256(verifier)) == challenge.
func verifyPKCES256(verifier, challenge string) bool {
	if verifier == "" || challenge == "" {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return computed == challenge
}

func oauthError(w http.ResponseWriter, status int, code, desc string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             code,
		"error_description": desc,
	})
}

// redirectError 302s an OAuth error back to a VALIDATED client redirect URI.
func redirectError(w http.ResponseWriter, r *http.Request, redirectURI, state, code, desc string) {
	sep := "?"
	if strings.Contains(redirectURI, "?") {
		sep = "&"
	}
	dest := redirectURI + sep + "error=" + url.QueryEscape(code) + "&error_description=" + url.QueryEscape(desc)
	if state != "" {
		dest += "&state=" + url.QueryEscape(state)
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

func writeCachedJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeNoStoreJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// clientIP extracts a best-effort client IP for rate-limiting. The AS sits
// behind a trusted reverse proxy, so an X-Forwarded-For leftmost hop is used
// when present, else the connection's remote address.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
