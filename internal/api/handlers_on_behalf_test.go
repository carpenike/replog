package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

const testOnBehalfCredential = "test-on-behalf-credential"

func TestOnBehalfPermissionsMatchMappedUser(t *testing.T) {
	env := setupTest(t)
	t.Setenv(middleware.OnBehalfCredentialEnv, testOnBehalfCredential)

	coach := env.createUser(t, "coach", true, false)
	other := env.createUser(t, "other", true, false)
	admin := env.createUser(t, "admin", true, true)
	mine := env.createAthlete(t, "Mine", coach.ID)
	theirs := env.createAthlete(t, "Theirs", other.ID)
	kid, err := models.CreateUser(context.Background(), env.DB, "kid", "Kid", "password123", "kid@example.com", false, false, sql.NullInt64{Int64: mine.ID, Valid: true})
	if err != nil {
		t.Fatalf("create kid: %v", err)
	}
	if err := models.EnsureUserPreferences(context.Background(), env.DB, kid.ID); err != nil {
		t.Fatalf("kid prefs: %v", err)
	}

	// Coach on-behalf matches that coach's session: own athlete only.
	own := doOnBehalf(t, env, http.MethodGet, "/api/athletes/"+itoa(mine.ID), testOnBehalfCredential, coach.Email.String, nil)
	requireStatus(t, own, http.StatusOK)
	denied := doOnBehalf(t, env, http.MethodGet, "/api/athletes/"+itoa(theirs.ID), testOnBehalfCredential, coach.Email.String, nil)
	requireStatus(t, denied, http.StatusForbidden)
	sessionOwn := env.do(t, http.MethodGet, "/api/athletes/"+itoa(mine.ID), nil, env.loginAs(t, coach))
	requireStatus(t, sessionOwn, http.StatusOK)
	sessionDenied := env.do(t, http.MethodGet, "/api/athletes/"+itoa(theirs.ID), nil, env.loginAs(t, coach))
	requireStatus(t, sessionDenied, http.StatusForbidden)

	// Athlete on-behalf matches that athlete's session.
	kidOwn := doOnBehalf(t, env, http.MethodGet, "/api/athletes/"+itoa(mine.ID), testOnBehalfCredential, "KID@example.com", nil)
	requireStatus(t, kidOwn, http.StatusOK)
	kidDenied := doOnBehalf(t, env, http.MethodGet, "/api/athletes/"+itoa(theirs.ID), testOnBehalfCredential, kid.Email.String, nil)
	requireStatus(t, kidDenied, http.StatusForbidden)

	// Admin on-behalf is coach-scoped: it does not get the admin-wide view
	// an admin browser session has.
	adminGet := doOnBehalf(t, env, http.MethodGet, "/api/athletes/"+itoa(theirs.ID), testOnBehalfCredential, admin.Email.String, nil)
	requireStatus(t, adminGet, http.StatusForbidden)
	adminList := doOnBehalf(t, env, http.MethodGet, "/api/athletes", testOnBehalfCredential, admin.Email.String, nil)
	requireStatus(t, adminList, http.StatusOK)
	var cards []AthleteCard
	decodeJSON(t, adminList, &cards)
	if len(cards) != 0 {
		t.Fatalf("admin on-behalf list = %d athletes, want 0 (admin coaches none)", len(cards))
	}
	sessionList := env.do(t, http.MethodGet, "/api/athletes", nil, env.loginAs(t, admin))
	requireStatus(t, sessionList, http.StatusOK)
	var sessionCards []AthleteCard
	decodeJSON(t, sessionList, &sessionCards)
	if len(sessionCards) != 2 {
		t.Fatalf("admin session list = %d, want 2", len(sessionCards))
	}

	coachList := doOnBehalf(t, env, http.MethodGet, "/api/athletes", testOnBehalfCredential, coach.Email.String, nil)
	requireStatus(t, coachList, http.StatusOK)
	var coachCards []AthleteCard
	decodeJSON(t, coachList, &coachCards)
	if len(coachCards) != 1 || coachCards[0].ID != mine.ID {
		t.Fatalf("coach on-behalf list = %+v, want only athlete %d", coachCards, mine.ID)
	}
}

func TestOnBehalfDoesNotFallBackToCoachService(t *testing.T) {
	env := setupTest(t)
	t.Setenv(middleware.OnBehalfCredentialEnv, testOnBehalfCredential)

	coach := env.createUser(t, "coach", true, true)
	athlete := env.createAthlete(t, "Kid", coach.ID)
	const serviceToken = "rlcst_0123456789abcdef"
	if err := models.InstallCoachServiceToken(context.Background(), env.DB, coach.ID, serviceToken); err != nil {
		t.Fatalf("install coach-service token: %v", err)
	}

	unknown := doOnBehalf(t, env, http.MethodGet, "/api/me", testOnBehalfCredential, "nobody@example.com", nil)
	requireStatus(t, unknown, http.StatusForbidden)
	if strings.Contains(unknown.Body.String(), coach.Username) || strings.Contains(unknown.Body.String(), testOnBehalfCredential) {
		t.Fatalf("unknown email fell back or leaked a secret: %s", unknown.Body.String())
	}

	missing := doOnBehalf(t, env, http.MethodGet, "/api/athletes/"+itoa(athlete.ID), testOnBehalfCredential, "", nil)
	requireStatus(t, missing, http.StatusForbidden)

	// A browser session plus a spoofed header stays the session user.
	spoof := doOnBehalf(t, env, http.MethodGet, "/api/me", "", "nobody@example.com", env.loginAs(t, coach))
	requireStatus(t, spoof, http.StatusOK)
	var me User
	decodeJSON(t, spoof, &me)
	if me.ID != coach.ID {
		t.Fatalf("spoofed header switched session user to %d", me.ID)
	}

	// The rlcst_ token still authenticates the bound user and ignores the header.
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+serviceToken)
	req.Header.Set(middleware.OnBehalfOfHeader, "nobody@example.com")
	rr := httptest.NewRecorder()
	env.Router.ServeHTTP(rr, req)
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &me)
	if me.ID != coach.ID {
		t.Fatalf("service token user = %d, want %d", me.ID, coach.ID)
	}

	wrong := doOnBehalf(t, env, http.MethodGet, "/api/me", "not-the-credential", coach.Email.String, env.loginAs(t, coach))
	requireStatus(t, wrong, http.StatusUnauthorized)
}

func doOnBehalf(t *testing.T, env *testEnv, method, path, credential, email string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	return doOnBehalfBody(t, env, method, path, credential, email, "", cookies)
}

func doOnBehalfBody(t *testing.T, env *testEnv, method, path, credential, email, body string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	var req *http.Request
	if reader != nil {
		req = httptest.NewRequest(method, path, reader)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if credential != "" {
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	if email != "" {
		req.Header.Set(middleware.OnBehalfOfHeader, email)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	env.Router.ServeHTTP(rr, req)
	return rr
}

// An admin mapped through on-behalf never reaches admin-only actions, and
// the coach actions the homelab-mcp tools use still work.
func TestOnBehalfAdminIsScopedToCoach(t *testing.T) {
	env := setupTest(t)
	t.Setenv(middleware.OnBehalfCredentialEnv, testOnBehalfCredential)

	admin := env.createUser(t, "boss", true, true)
	other := env.createUser(t, "other", true, false)
	mine := env.createAthlete(t, "Mine", admin.ID)
	theirs := env.createAthlete(t, "Theirs", other.ID)
	email := admin.Email.String

	// The same admin in a browser session can issue a token, so the 403s
	// below come from the on-behalf scope and not from the handler.
	sessionIssue := env.do(t, http.MethodPost, "/api/users/"+itoa(other.ID)+"/service-tokens", `{"label":"probe"}`, env.loginAs(t, admin))
	requireStatus(t, sessionIssue, http.StatusCreated)
	var issued ServiceTokenIssued
	decodeJSON(t, sessionIssue, &issued)

	before := countServiceTokens(t, env)
	denied := []struct{ method, path, body string }{
		{http.MethodPost, "/api/users/" + itoa(other.ID) + "/service-tokens", `{"label":"x"}`},
		{http.MethodPost, "/api/users/" + itoa(admin.ID) + "/service-tokens", `{"label":"x"}`},
		{http.MethodGet, "/api/users/" + itoa(other.ID) + "/service-tokens", ""},
		{http.MethodDelete, "/api/users/" + itoa(other.ID) + "/service-tokens/" + itoa(issued.ID), ""},
		{http.MethodGet, "/api/users", ""},
		{http.MethodPost, "/api/users", `{"username":"evil","password":"password123","is_admin":true}`},
		{http.MethodGet, "/api/users/" + itoa(other.ID), ""},
		{http.MethodPut, "/api/users/" + itoa(other.ID), `{"username":"other","is_admin":true}`},
		{http.MethodDelete, "/api/users/" + itoa(other.ID), ""},
		{http.MethodPost, "/api/users/" + itoa(other.ID) + "/tokens", `{}`},
		{http.MethodGet, "/api/admin/impersonateable", ""},
		{http.MethodPost, "/api/admin/impersonate/" + itoa(other.ID), ""},
		{http.MethodPut, "/api/preferences", `{"weight_unit":"kg"}`},
		{http.MethodPost, "/api/logout", ""},
		{http.MethodPost, "/api/athletes", `{"name":"New"}`},
		{http.MethodDelete, "/api/athletes/" + itoa(mine.ID), ""},
		{http.MethodPost, "/api/programs", `{"name":"x"}`},
	}
	// Dot-segments and escaped slashes never reach a handler: the router
	// does not match them (404) or the scope refuses them (403).
	for _, p := range []string{
		"/api/athletes/" + itoa(mine.ID) + "/../../users",
		"/api/athletes/" + itoa(mine.ID) + "/coach/..%2F..%2F..%2Fusers",
	} {
		rr := doOnBehalf(t, env, http.MethodGet, p, testOnBehalfCredential, email, nil)
		if rr.Code != http.StatusForbidden && rr.Code != http.StatusNotFound {
			t.Errorf("GET %s on behalf = %d, want 403 or 404", p, rr.Code)
		}
	}
	for _, d := range denied {
		rr := doOnBehalfBody(t, env, d.method, d.path, testOnBehalfCredential, email, d.body, nil)
		if rr.Code != http.StatusForbidden {
			t.Errorf("%s %s on behalf of an admin = %d, want 403 (body %s)", d.method, d.path, rr.Code, rr.Body.String())
		}
		if strings.Contains(rr.Body.String(), testOnBehalfCredential) || strings.Contains(rr.Body.String(), "rlcst_") {
			t.Errorf("%s %s leaked a secret: %s", d.method, d.path, rr.Body.String())
		}
	}
	if got := countServiceTokens(t, env); got != before {
		t.Fatalf("service tokens = %d after on-behalf attempts, want %d", got, before)
	}
	var revoked sql.NullString
	if err := env.DB.QueryRow(`SELECT revoked_at FROM service_tokens WHERE id = ?`, issued.ID).Scan(&revoked); err != nil {
		t.Fatalf("read token: %v", err)
	}
	if revoked.Valid {
		t.Fatal("on-behalf request revoked a token")
	}
	var stillAdmin bool
	if err := env.DB.QueryRow(`SELECT is_admin FROM users WHERE id = ?`, admin.ID).Scan(&stillAdmin); err != nil {
		t.Fatalf("read admin: %v", err)
	}
	if !stillAdmin {
		t.Fatal("scoping cleared is_admin on the stored row")
	}

	// /api/me reports the scoped identity, not admin.
	me := doOnBehalf(t, env, http.MethodGet, "/api/me", testOnBehalfCredential, email, nil)
	requireStatus(t, me, http.StatusOK)
	var meBody User
	decodeJSON(t, me, &meBody)
	if meBody.ID != admin.ID || meBody.IsAdmin {
		t.Fatalf("on-behalf /api/me = %+v, want id %d without admin", meBody, admin.ID)
	}

	// Coach actions on the admin's own athlete still work.
	base := "/api/athletes/" + itoa(mine.ID) + "/coach"
	for _, p := range []string{"/api/dashboard", "/api/athletes", "/api/athletes/" + itoa(mine.ID), base + "/exercises", base + "/programs", base + "/sessions?start=2026-01-01&end=2026-12-31", base + "/week", base + "/position"} {
		rr := doOnBehalf(t, env, http.MethodGet, p, testOnBehalfCredential, email, nil)
		requireStatus(t, rr, http.StatusOK)
	}
	created := doOnBehalfBody(t, env, http.MethodPost, base+"/exercises", testOnBehalfCredential, email, `{"name":"Goblet Squat"}`, nil)
	if created.Code != http.StatusOK && created.Code != http.StatusCreated {
		t.Fatalf("create coach exercise = %d, body %s", created.Code, created.Body.String())
	}
	preview := doOnBehalfBody(t, env, http.MethodPost, base+"/programs/preview", testOnBehalfCredential, email, previewBody(mine.ID, day1ProgramJSON), nil)
	requireStatus(t, preview, http.StatusOK)
	var pv CoachProgramPreviewResponse
	decodeJSON(t, preview, &pv)
	imported := doOnBehalfBody(t, env, http.MethodPost, base+"/programs/import", testOnBehalfCredential, email, `{"preview_token":"`+pv.PreviewToken+`","confirm":true}`, nil)
	requireStatus(t, imported, http.StatusCreated)
	var im CoachProgramImportResponse
	decodeJSON(t, imported, &im)
	assign := doOnBehalfBody(t, env, http.MethodPost, base+"/programs/assign", testOnBehalfCredential, email, fmt.Sprintf(`{"template_id":%d,"version":%d,"start_date":"2026-11-23"}`, im.TemplateID, im.Version), nil)
	requireStatus(t, assign, http.StatusCreated)

	// Not the admin-wide bypass: another coach's athlete stays closed.
	for _, p := range []string{"/api/athletes/" + itoa(theirs.ID), "/api/athletes/" + itoa(theirs.ID) + "/coach/programs"} {
		rr := doOnBehalf(t, env, http.MethodGet, p, testOnBehalfCredential, email, nil)
		requireStatus(t, rr, http.StatusForbidden)
	}
	rr := doOnBehalfBody(t, env, http.MethodPost, "/api/athletes/"+itoa(theirs.ID)+"/coach/programs/preview", testOnBehalfCredential, email, previewBody(theirs.ID, day1ProgramJSON), nil)
	requireStatus(t, rr, http.StatusForbidden)
}

func countServiceTokens(t *testing.T, env *testEnv) int {
	t.Helper()
	var n int
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM service_tokens`).Scan(&n); err != nil {
		t.Fatalf("count service tokens: %v", err)
	}
	return n
}
