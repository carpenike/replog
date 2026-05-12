// Tests for the WebAuthn ceremony endpoints. We can't run a real
// authenticator from a unit test, so coverage focuses on the surface that
// doesn't depend on a browser:
//
//   * BeginRegistration / BeginLogin emit the correct shape of options and
//     stash the matching session data, so the SPA can complete the ceremony.
//   * Auth gating works (the ceremony endpoints are mounted behind
//     RequireAuth in production; unauthenticated requests get 401).
//   * The Finish handlers gracefully reject requests with no in-progress
//     ceremony in the session.
//
// End-to-end ceremony testing belongs at the browser-integration layer.
package passkeys

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

func newTestWebAuthn(t *testing.T) *webauthn.WebAuthn {
	t.Helper()
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "RepLog Test",
		RPID:          "localhost",
		RPOrigins:     []string{"http://localhost:5173", "http://localhost:8080"},
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			UserVerification: protocol.VerificationPreferred,
		},
	})
	if err != nil {
		t.Fatalf("configure webauthn: %v", err)
	}
	return wa
}

func newTestHandler(t *testing.T) (*Handler, *sql.DB, *scs.SessionManager, *models.User) {
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

	user, err := models.CreateUser(db, "alice", "Alice", "password123", "alice@example.com",
		false, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	sm := scs.New()
	sm.Lifetime = 30 * 24 * time.Hour

	h := &Handler{
		DB:       db,
		Sessions: sm,
		WebAuthn: newTestWebAuthn(t),
	}
	return h, db, sm, user
}

// withUserCtx attaches an authenticated user to the request context, mirroring
// what RequireAuth would do in production.
func withUserCtx(req *http.Request, user *models.User) *http.Request {
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, user)
	return req.WithContext(ctx)
}

func TestBeginRegistration_ReturnsValidOptionsAndStashesSession(t *testing.T) {
	h, _, sm, user := newTestHandler(t)

	// Wrap the handler in scs.LoadAndSave so the session is actually persisted
	// to (and committed back from) the recorder's cookies.
	handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.BeginRegistration(w, withUserCtx(r, user))
	}))

	req := httptest.NewRequest("GET", "/api/passkeys/register/begin", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	// Response must be a valid CredentialCreation envelope with publicKey opts.
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	pubKey, ok := got["publicKey"].(map[string]any)
	if !ok {
		t.Fatalf("response missing publicKey object: %+v", got)
	}
	if pubKey["challenge"] == nil {
		t.Error("response missing challenge")
	}
	if pubKey["rp"] == nil {
		t.Error("response missing rp")
	}

	// scs sets a cookie if anything was stored. The handler stashes the
	// webauthn session under "webauthn_registration".
	if len(rr.Result().Cookies()) == 0 {
		t.Error("expected a session cookie to be set")
	}
}

func TestBeginRegistration_Unauthenticated(t *testing.T) {
	h, _, sm, _ := newTestHandler(t)

	// No user attached \u2014 simulates calling BeginRegistration with no
	// auth (which production wraps with RequireAuth, but the handler's
	// own nil-check is the second line of defense).
	handler := sm.LoadAndSave(http.HandlerFunc(h.BeginRegistration))

	req := httptest.NewRequest("GET", "/api/passkeys/register/begin", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestBeginLogin_ReturnsAssertionAndStashesSession(t *testing.T) {
	h, _, sm, _ := newTestHandler(t)

	// Discoverable login does not require an authenticated user.
	handler := sm.LoadAndSave(http.HandlerFunc(h.BeginLogin))

	req := httptest.NewRequest("GET", "/api/passkeys/login/begin", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	pubKey, ok := got["publicKey"].(map[string]any)
	if !ok {
		t.Fatalf("response missing publicKey object: %+v", got)
	}
	if pubKey["challenge"] == nil {
		t.Error("response missing challenge")
	}

	if len(rr.Result().Cookies()) == 0 {
		t.Error("expected a session cookie to be set")
	}
}

func TestFinishRegistration_NoInProgressCeremony(t *testing.T) {
	h, _, sm, user := newTestHandler(t)

	// No prior BeginRegistration call \u2014 the session has nothing stashed.
	handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.FinishRegistration(w, withUserCtx(r, user))
	}))

	req := httptest.NewRequest("POST", "/api/passkeys/register/finish", nil)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestFinishLogin_NoInProgressCeremony(t *testing.T) {
	h, _, sm, _ := newTestHandler(t)

	handler := sm.LoadAndSave(http.HandlerFunc(h.FinishLogin))

	req := httptest.NewRequest("POST", "/api/passkeys/login/finish", nil)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}
