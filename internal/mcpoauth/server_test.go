package mcpoauth

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/models"
)

const testOrigin = "https://replog.test"

// newTestServer builds a Server WITHOUT New() — New() needs a live PocketID
// provider for OIDC discovery, but every endpoint under test here (metadata,
// DCR, authorize-validation, token) is independent of the federated leg.
func newTestServer(t *testing.T) (*Server, *sql.DB) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.RunMigrations(db); err != nil {
		_ = db.Close()
		t.Fatalf("migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return &Server{
		db:      db,
		origin:  testOrigin,
		states:  newStateStore(),
		codes:   newCodeStore(),
		regRate: newIPRateLimiter(100, time.Hour),
	}, db
}

func newTestRouter(s *Server) chi.Router {
	r := chi.NewRouter()
	s.RegisterRoutes(r)
	return r
}

func doJSON(t *testing.T, r chi.Router, method, target string, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	out := map[string]any{}
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr, out
}

func TestASMetadata_OmitsJWKS(t *testing.T) {
	s, _ := newTestServer(t)
	r := newTestRouter(s)

	rr, body := doJSON(t, r, http.MethodGet, "/.well-known/oauth-authorization-server", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if body["issuer"] != testOrigin {
		t.Errorf("issuer = %v, want %s", body["issuer"], testOrigin)
	}
	if _, present := body["jwks_uri"]; present {
		t.Error("jwks_uri must be omitted — RepLog mints opaque, not JWT, tokens")
	}
	if got := body["code_challenge_methods_supported"].([]any); got[0] != "S256" {
		t.Errorf("code_challenge_methods_supported = %v, want [S256]", got)
	}
	// "none" must be advertised: the pocketid-mcp-as conformance contract (v1.1)
	// validates the reference metadata shape, which includes "none" for
	// public/PKCE-only clients alongside the confidential-client methods.
	authMethods, _ := body["token_endpoint_auth_methods_supported"].([]any)
	var hasNone bool
	for _, m := range authMethods {
		if m == "none" {
			hasNone = true
		}
	}
	if !hasNone {
		t.Errorf("token_endpoint_auth_methods_supported = %v, must include \"none\" (conformance contract)", authMethods)
	}
}

func TestPRM_BothVariants(t *testing.T) {
	s, _ := newTestServer(t)
	r := newTestRouter(s)

	_, root := doJSON(t, r, http.MethodGet, "/.well-known/oauth-protected-resource", "")
	if root["resource"] != testOrigin {
		t.Errorf("root resource = %v, want %s", root["resource"], testOrigin)
	}

	_, res := doJSON(t, r, http.MethodGet, "/.well-known/oauth-protected-resource/api/mcp", "")
	if res["resource"] != testOrigin+"/api/mcp" {
		t.Errorf("resource = %v, want %s/api/mcp", res["resource"], testOrigin)
	}

	// Both variants must advertise bearer_methods_supported: ["header"] to match
	// the production-proven reference shape.
	for _, prm := range []map[string]any{root, res} {
		methods, _ := prm["bearer_methods_supported"].([]any)
		if len(methods) != 1 || methods[0] != "header" {
			t.Errorf("bearer_methods_supported = %v, want [header]", prm["bearer_methods_supported"])
		}
	}

	if got := s.PRMResourceURL(); got != testOrigin+"/.well-known/oauth-protected-resource/api/mcp" {
		t.Errorf("PRMResourceURL = %q", got)
	}
}

func TestRegister_FiltersDisallowedRedirects(t *testing.T) {
	s, _ := newTestServer(t)
	r := newTestRouter(s)

	// One allowed, one disallowed — the disallowed one is dropped, not fatal.
	rr, body := doJSON(t, r, http.MethodPost, "/oauth/register",
		`{"client_name":"vscode","redirect_uris":["https://claude.ai/callback","https://evil.example.com/x"]}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body=%s)", rr.Code, rr.Body.String())
	}
	uris, _ := body["redirect_uris"].([]any)
	if len(uris) != 1 || uris[0] != "https://claude.ai/callback" {
		t.Errorf("redirect_uris = %v, want only the allowed one", uris)
	}
	if body["client_secret"] == nil || body["client_secret"] == "" {
		t.Error("expected a client_secret in the registration response")
	}
	if body["client_secret_expires_at"].(float64) != 0 {
		t.Error("client_secret_expires_at should be 0 (never expires)")
	}
}

func TestRegister_RejectsWhenNoAcceptableRedirect(t *testing.T) {
	s, _ := newTestServer(t)
	r := newTestRouter(s)

	rr, body := doJSON(t, r, http.MethodPost, "/oauth/register",
		`{"client_name":"evil","redirect_uris":["https://evil.example.com/x"]}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if body["error"] != "invalid_redirect_uri" {
		t.Errorf("error = %v, want invalid_redirect_uri", body["error"])
	}
}

func TestAuthorize_ValidationFailures(t *testing.T) {
	s, db := newTestServer(t)
	r := newTestRouter(s)

	client, _, err := models.RegisterDCRClient(db, "c", []string{"https://claude.ai/cb"}, "client_secret_post")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	cases := []struct {
		name  string
		query url.Values
		want  string
	}{
		{"bad response_type", url.Values{"response_type": {"token"}}, "unsupported_response_type"},
		{"unknown client", url.Values{"response_type": {"code"}, "client_id": {"nope"}}, "invalid_client"},
		{"unregistered redirect", url.Values{
			"response_type": {"code"}, "client_id": {client.ClientID},
			"redirect_uri": {"https://claude.ai/other"},
		}, "invalid_request"},
		{"missing PKCE", url.Values{
			"response_type": {"code"}, "client_id": {client.ClientID},
			"redirect_uri": {"https://claude.ai/cb"},
		}, "invalid_request"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr, body := doJSON(t, r, http.MethodGet, "/oauth/authorize?"+tc.query.Encode(), "")
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", rr.Code, rr.Body.String())
			}
			if body["error"] != tc.want {
				t.Errorf("error = %v, want %v", body["error"], tc.want)
			}
		})
	}
}

func TestToken_FullGrant(t *testing.T) {
	s, db := newTestServer(t)
	r := newTestRouter(s)

	// A real user the code will be bound to.
	user, err := models.CreateUser(db, "coach", "", "pw12345678", "coach@example.com", true, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	client, secret, err := models.RegisterDCRClient(db, "c", []string{"https://claude.ai/cb"}, "client_secret_post")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	// Seed an authorization code as the callback would, with a known PKCE pair.
	verifier := "test-verifier-0123456789-abcdefghijklmnop"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	const code = "seeded-auth-code"
	s.codes.put(code, authzCode{
		userID:              user.ID,
		clientID:            client.ClientID,
		redirectURI:         "https://claude.ai/cb",
		claudeCodeChallenge: challenge,
		scope:               "openid",
		expiresAt:           time.Now().Add(time.Minute),
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {"https://claude.ai/cb"},
		"client_id":     {client.ClientID},
		"client_secret": {secret},
		"code_verifier": {verifier},
	}
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	tok, _ := body["access_token"].(string)
	if !strings.HasPrefix(tok, models.MCPTokenPrefix) {
		t.Errorf("access_token = %q, want %s-prefixed opaque token", tok, models.MCPTokenPrefix)
	}
	if body["token_type"] != "Bearer" {
		t.Errorf("token_type = %v, want Bearer", body["token_type"])
	}

	// The minted token must validate back to the bound user.
	got, err := models.ValidateMCPToken(db, tok)
	if err != nil {
		t.Fatalf("validate minted token: %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("token resolved to user %d, want %d", got.ID, user.ID)
	}

	// The code is single-use — a replay must fail.
	req2 := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusBadRequest {
		t.Errorf("replay status = %d, want 400 (single-use code)", rr2.Code)
	}
}

func TestToken_PKCEMismatchRejected(t *testing.T) {
	s, db := newTestServer(t)
	r := newTestRouter(s)

	user, _ := models.CreateUser(db, "coach", "", "pw12345678", "coach@example.com", true, false, sql.NullInt64{})
	client, secret, _ := models.RegisterDCRClient(db, "c", []string{"https://claude.ai/cb"}, "client_secret_post")

	sum := sha256.Sum256([]byte("the-real-verifier"))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	const code = "seeded"
	s.codes.put(code, authzCode{
		userID: user.ID, clientID: client.ClientID, redirectURI: "https://claude.ai/cb",
		claudeCodeChallenge: challenge, scope: "openid", expiresAt: time.Now().Add(time.Minute),
	})

	form := url.Values{
		"grant_type": {"authorization_code"}, "code": {code},
		"redirect_uri": {"https://claude.ai/cb"}, "client_id": {client.ClientID},
		"client_secret": {secret}, "code_verifier": {"the-WRONG-verifier"},
	}
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["error"] != "invalid_grant" {
		t.Errorf("error = %v, want invalid_grant", body["error"])
	}
}

func TestVerifyPKCES256(t *testing.T) {
	sum := sha256.Sum256([]byte("verifier"))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if !verifyPKCES256("verifier", challenge) {
		t.Error("matching verifier should pass")
	}
	if verifyPKCES256("wrong", challenge) {
		t.Error("non-matching verifier should fail")
	}
	if verifyPKCES256("", challenge) {
		t.Error("empty verifier should fail")
	}
}
