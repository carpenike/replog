package api

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func TestLogin_Success(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "alice", true, true)

	rr := env.do(t, "POST", "/api/login", `{"username":"alice","password":"password123"}`, nil)
	requireStatus(t, rr, http.StatusOK)

	var got User
	decodeJSON(t, rr, &got)
	if got.ID != user.ID {
		t.Errorf("got user id %d, want %d", got.ID, user.ID)
	}
	if got.Username != "alice" {
		t.Errorf("got username %q, want %q", got.Username, "alice")
	}
	if !got.IsAdmin || !got.IsCoach {
		t.Errorf("got is_admin=%v is_coach=%v, want both true", got.IsAdmin, got.IsCoach)
	}

	// Successful login must set a session cookie.
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected a session cookie, got none")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	env := setupTest(t)
	env.createUser(t, "alice", false, false)

	rr := env.do(t, "POST", "/api/login", `{"username":"alice","password":"wrong"}`, nil)
	requireStatus(t, rr, http.StatusUnauthorized)

	var got APIError
	decodeJSON(t, rr, &got)
	if got.Error != "invalid username or password" {
		t.Errorf("got error %q, want %q", got.Error, "invalid username or password")
	}
}

func TestLogin_UnknownUser(t *testing.T) {
	env := setupTest(t)
	rr := env.do(t, "POST", "/api/login", `{"username":"nobody","password":"password123"}`, nil)
	requireStatus(t, rr, http.StatusUnauthorized)
}

func TestLogin_MissingFields(t *testing.T) {
	env := setupTest(t)
	cases := []struct {
		name string
		body string
	}{
		{"empty username", `{"username":"","password":"x"}`},
		{"empty password", `{"username":"alice","password":""}`},
		{"both empty", `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := env.do(t, "POST", "/api/login", tc.body, nil)
			requireStatus(t, rr, http.StatusBadRequest)
		})
	}
}

func TestLogin_MalformedJSON(t *testing.T) {
	env := setupTest(t)
	rr := env.do(t, "POST", "/api/login", `not json`, nil)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestMe_Unauthenticated(t *testing.T) {
	env := setupTest(t)
	rr := env.do(t, "GET", "/api/me", nil, nil)
	requireStatus(t, rr, http.StatusUnauthorized)

	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected JSON Content-Type, got %q", ct)
	}
	if loc := rr.Header().Get("Location"); loc != "" {
		t.Errorf("expected no Location header, got %q (issue #2 regression)", loc)
	}
}

func TestMe_Authenticated(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "alice", true, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "GET", "/api/me", nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got User
	decodeJSON(t, rr, &got)
	if got.ID != user.ID || got.Username != "alice" {
		t.Errorf("got %+v, want id=%d username=alice", got, user.ID)
	}
	if got.Impersonating {
		t.Error("expected impersonating=false for normal session")
	}
}

func TestLogout_DestroysSession(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "alice", false, false)
	cookies := env.loginAs(t, user)

	// Authenticated /me works.
	rr := env.do(t, "GET", "/api/me", nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	// Logout.
	rr = env.do(t, "POST", "/api/logout", nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var status StatusResponse
	decodeJSON(t, rr, &status)
	if status.Status != "ok" {
		t.Errorf("got status %q, want ok", status.Status)
	}

	// Same cookies should no longer authenticate. After scs destroys the
	// session the userID is gone, so RequireAuth returns 401.
	rr = env.do(t, "GET", "/api/me", nil, cookies)
	requireStatus(t, rr, http.StatusUnauthorized)
}

// --- Per-account login lockout (ADR 014) ---

func TestLogin_LocksAfterThresholdReturns429(t *testing.T) {
	env := setupTest(t)
	env.createUser(t, "victim", false, false)

	// LockoutThreshold wrong attempts trip the lock.
	for i := 0; i < models.LockoutThreshold; i++ {
		rr := env.do(t, "POST", "/api/login",
			`{"username":"victim","password":"wrong"}`, nil)
		requireStatus(t, rr, http.StatusUnauthorized)
	}

	// Right password is now blocked behind the lock — 429 with Retry-After.
	rr := env.do(t, "POST", "/api/login",
		`{"username":"victim","password":"password123"}`, nil)
	requireStatus(t, rr, http.StatusTooManyRequests)

	ra := rr.Header().Get("Retry-After")
	if ra == "" {
		t.Fatal("expected Retry-After header on 429 response")
	}
	secs, err := strconv.Atoi(ra)
	if err != nil {
		t.Fatalf("Retry-After = %q, want integer seconds: %v", ra, err)
	}
	maxExpected := int(models.LockoutDuration.Seconds()) + 1
	if secs < 1 || secs > maxExpected {
		t.Errorf("Retry-After = %d, want in [1, %d]", secs, maxExpected)
	}

	// Body is JSON with the friendly message, not the raw model error.
	var got APIError
	decodeJSON(t, rr, &got)
	if !strings.Contains(got.Error, "locked") {
		t.Errorf("error body = %q, want substring 'locked'", got.Error)
	}
}

func TestLogin_LockoutDoesNotApplyToUnknownUser(t *testing.T) {
	// An attacker should not be able to flood unknown usernames and
	// trigger 429s — that path stays at 401 unconditionally.
	env := setupTest(t)
	for i := 0; i < models.LockoutThreshold*2; i++ {
		rr := env.do(t, "POST", "/api/login",
			`{"username":"ghost","password":"x"}`, nil)
		requireStatus(t, rr, http.StatusUnauthorized)
	}
}
