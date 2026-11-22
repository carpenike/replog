package api

import (
	"context"
	"database/sql"
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

	// Admin on-behalf matches an admin session, not the reduced rlcst_ list.
	adminGet := doOnBehalf(t, env, http.MethodGet, "/api/athletes/"+itoa(theirs.ID), testOnBehalfCredential, admin.Email.String, nil)
	requireStatus(t, adminGet, http.StatusOK)
	adminList := doOnBehalf(t, env, http.MethodGet, "/api/athletes", testOnBehalfCredential, admin.Email.String, nil)
	requireStatus(t, adminList, http.StatusOK)
	var cards []AthleteCard
	decodeJSON(t, adminList, &cards)
	if len(cards) != 2 {
		t.Fatalf("admin on-behalf list = %d athletes, want 2", len(cards))
	}
	sessionList := env.do(t, http.MethodGet, "/api/athletes", nil, env.loginAs(t, admin))
	requireStatus(t, sessionList, http.StatusOK)
	var sessionCards []AthleteCard
	decodeJSON(t, sessionList, &sessionCards)
	if len(sessionCards) != len(cards) {
		t.Fatalf("admin session list = %d, on-behalf list = %d", len(sessionCards), len(cards))
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
	req := httptest.NewRequest(method, path, nil)
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
