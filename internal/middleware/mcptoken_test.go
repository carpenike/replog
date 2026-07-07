package middleware

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/models"
)

// mcpTokenFixture stages a fresh in-memory DB for opaque-token tests.
type mcpTokenFixture struct {
	db *sql.DB
}

func newMCPTokenFixture(t *testing.T) *mcpTokenFixture {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.RunMigrations(db); err != nil {
		_ = db.Close()
		t.Fatalf("run migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &mcpTokenFixture{db: db}
}

func (f *mcpTokenFixture) createUser(t *testing.T, username, email string, mcpEnabled bool) *models.User {
	t.Helper()
	u, err := models.CreateUser(context.Background(), f.db, username, "", "password123", email, true, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user %q: %v", username, err)
	}
	if mcpEnabled {
		if err := models.SetUserMCPEnabled(context.Background(), f.db, u.ID, true); err != nil {
			t.Fatalf("enable mcp for %q: %v", username, err)
		}
	}
	return u
}

func runMCPMiddleware(t *testing.T, m *MCPTokenAuth, req *http.Request) (*httptest.ResponseRecorder, *models.User) {
	t.Helper()
	var got *models.User
	handler := m.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = UserFromContext(r.Context())
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr, got
}

func reqWithBearer(token string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/mcp", nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

func TestMCPTokenAuth_AttachesUserOnValidToken(t *testing.T) {
	f := newMCPTokenFixture(t)
	u := f.createUser(t, "coach", "coach@example.com", true)
	plaintext, _, err := models.CreateMCPToken(context.Background(), f.db, u.ID, "client-1", "")
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}

	m := NewMCPTokenAuth(f.db, "https://replog.test/.well-known/oauth-protected-resource/api/mcp")
	rr, got := runMCPMiddleware(t, m, reqWithBearer(plaintext))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if got == nil || got.ID != u.ID {
		t.Fatalf("user not attached to context: %+v", got)
	}
}

func TestMCPTokenAuth_RejectsMissingBearer(t *testing.T) {
	f := newMCPTokenFixture(t)
	m := NewMCPTokenAuth(f.db, "https://replog.test/.well-known/oauth-protected-resource/api/mcp")
	rr, _ := runMCPMiddleware(t, m, reqWithBearer(""))

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if rr.Body.Len() == 0 {
		t.Fatal("expected JSON error body")
	}
	www := rr.Header().Get("WWW-Authenticate")
	if www == "" {
		t.Fatal("expected WWW-Authenticate header")
	}
	if !strings.Contains(www, "resource_metadata=") {
		t.Fatalf("WWW-Authenticate should advertise the PRM: %q", www)
	}
	assertReason(t, rr, "missing-bearer-token")
}

func TestMCPTokenAuth_RejectsNonPrefixedToken(t *testing.T) {
	f := newMCPTokenFixture(t)
	m := NewMCPTokenAuth(f.db, "")
	// A credential that does not start with rlpat_ must be rejected before any
	// DB lookup.
	rr, _ := runMCPMiddleware(t, m, reqWithBearer("not-an-mcp-token"))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	assertReason(t, rr, "invalid-token")
}

func TestMCPTokenAuth_RejectsUnknownToken(t *testing.T) {
	f := newMCPTokenFixture(t)
	m := NewMCPTokenAuth(f.db, "")
	rr, _ := runMCPMiddleware(t, m, reqWithBearer(models.MCPTokenPrefix+"deadbeefdeadbeef"))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	assertReason(t, rr, "invalid-token")
}

func TestMCPTokenAuth_RejectsRevokedToken(t *testing.T) {
	f := newMCPTokenFixture(t)
	u := f.createUser(t, "coach", "coach@example.com", true)
	plaintext, tok, err := models.CreateMCPToken(context.Background(), f.db, u.ID, "client-1", "")
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	if err := models.RevokeMCPToken(context.Background(), f.db, tok.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	m := NewMCPTokenAuth(f.db, "")
	rr, _ := runMCPMiddleware(t, m, reqWithBearer(plaintext))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	assertReason(t, rr, "invalid-token")
}

func TestMCPTokenAuth_RejectsExpiredToken(t *testing.T) {
	f := newMCPTokenFixture(t)
	u := f.createUser(t, "coach", "coach@example.com", true)

	// Insert a token row whose expires_at is in the past.
	plaintext := models.MCPTokenPrefix + "expiredtokenexpiredtoken"
	sum := sha256.Sum256([]byte(plaintext))
	hash := hex.EncodeToString(sum[:])
	if _, err := f.db.Exec(
		`INSERT INTO mcp_tokens (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		hash, u.ID, time.Now().Add(-time.Hour), time.Now().Add(-2*time.Hour),
	); err != nil {
		t.Fatalf("insert expired token: %v", err)
	}

	m := NewMCPTokenAuth(f.db, "")
	rr, _ := runMCPMiddleware(t, m, reqWithBearer(plaintext))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	assertReason(t, rr, "invalid-token")
}

func TestMCPTokenAuth_RejectsUserWithoutMCPEnabled(t *testing.T) {
	f := newMCPTokenFixture(t)
	u := f.createUser(t, "coach", "coach@example.com", false)
	plaintext, _, err := models.CreateMCPToken(context.Background(), f.db, u.ID, "client-1", "")
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}

	m := NewMCPTokenAuth(f.db, "")
	rr, _ := runMCPMiddleware(t, m, reqWithBearer(plaintext))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
	assertReason(t, rr, "mcp-not-enabled")
}

func assertReason(t *testing.T, rr *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Reason != want {
		t.Fatalf("reason = %q, want %q", body.Reason, want)
	}
}
