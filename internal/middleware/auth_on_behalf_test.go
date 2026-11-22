package middleware

import (
	"bytes"
	"context"
	"database/sql"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

const testOnBehalfCredential = "test-on-behalf-credential"

func TestRequireAuth_OnBehalfMapsEmail(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	coach, err := models.CreateUser(context.Background(), db, "coach", "Coach", "password123", "Coach@Example.com", true, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create coach: %v", err)
	}
	t.Setenv(OnBehalfCredentialEnv, testOnBehalfCredential)

	var gotID int64
	var service, onBehalf bool
	var headerLeft string
	handler := RequireAuth(sm, db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = UserFromContext(r.Context()).ID
		service = IsServiceCredential(r.Context())
		onBehalf = IsOnBehalf(r.Context())
		headerLeft = r.Header.Get(OnBehalfOfHeader)
		w.WriteHeader(http.StatusOK)
	}))

	logs := captureLog(t)
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+testOnBehalfCredential)
	req.Header.Set(OnBehalfOfHeader, "coach@example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	if gotID != coach.ID || service || !onBehalf {
		t.Fatalf("user=%d service=%v onBehalf=%v, want %d, false, true", gotID, service, onBehalf, coach.ID)
	}
	if headerLeft != "" {
		t.Fatalf("handler still saw %s", OnBehalfOfHeader)
	}
	if strings.Contains(rr.Body.String(), testOnBehalfCredential) {
		t.Fatal("response contains the credential")
	}
	assertLogOmitsSecrets(t, logs, testOnBehalfCredential, "coach@example.com")

	entries, err := models.ListAuditLog(context.Background(), db, 10)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("audit rows = %d, want 1", len(entries))
	}
	if entries[0].RealUserID != coach.ID || entries[0].Action != "on_behalf" || entries[0].TargetUserID.Valid {
		t.Fatalf("audit = %+v", entries[0])
	}
	if !entries[0].Details.Valid || entries[0].Details.String != "on_behalf=true proof=shared" {
		t.Fatalf("audit details = %+v", entries[0].Details)
	}
	if strings.Contains(entries[0].Details.String, testOnBehalfCredential) {
		t.Fatal("audit details contain the credential")
	}
}

func TestRequireAuth_OnBehalfSpoofedHeaderIgnored(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	sessionUser, err := models.CreateUser(context.Background(), db, "session", "", "password123", "session@example.com", true, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create session user: %v", err)
	}
	other, err := models.CreateUser(context.Background(), db, "other", "", "password123", "other@example.com", true, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create other: %v", err)
	}
	t.Setenv(OnBehalfCredentialEnv, "")

	var gotID int64
	var onBehalf bool
	var headerLeft string
	handler := RequireAuth(sm, db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = UserFromContext(r.Context()).ID
		onBehalf = IsOnBehalf(r.Context())
		headerLeft = r.Header.Get(OnBehalfOfHeader)
		w.WriteHeader(http.StatusOK)
	}))

	setup := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), "userID", sessionUser.ID)
		w.WriteHeader(http.StatusOK)
	}))
	setupRR := httptest.NewRecorder()
	setup.ServeHTTP(setupRR, httptest.NewRequest(http.MethodGet, "/setup", nil))

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	for _, c := range setupRR.Result().Cookies() {
		req.AddCookie(c)
	}
	req.Header.Set(OnBehalfOfHeader, other.Email.String)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("session status = %d, body %s", rr.Code, rr.Body.String())
	}
	if gotID != sessionUser.ID || onBehalf || headerLeft != "" {
		t.Fatalf("user=%d onBehalf=%v header=%q, want session user and a stripped header", gotID, onBehalf, headerLeft)
	}

	// Unset credential disables the path. The same bearer is not a user.
	disabled := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	disabled.Header.Set("Authorization", "Bearer "+testOnBehalfCredential)
	disabled.Header.Set(OnBehalfOfHeader, other.Email.String)
	for _, c := range setupRR.Result().Cookies() {
		disabled.AddCookie(c)
	}
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, disabled)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("disabled path status = %d, body %s", rr.Code, rr.Body.String())
	}

	// An rlcst_ token also ignores the header, including when the on-behalf
	// credential is configured.
	token, _, err := models.CreateServiceToken(context.Background(), db, sessionUser.ID, "agent")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	t.Setenv(OnBehalfCredentialEnv, testOnBehalfCredential)
	onBehalf = true
	var service bool
	handler = RequireAuth(sm, db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = UserFromContext(r.Context()).ID
		service = IsServiceCredential(r.Context())
		onBehalf = IsOnBehalf(r.Context())
		headerLeft = r.Header.Get(OnBehalfOfHeader)
		w.WriteHeader(http.StatusOK)
	}))
	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(OnBehalfOfHeader, other.Email.String)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("service token status = %d, body %s", rr.Code, rr.Body.String())
	}
	if gotID != sessionUser.ID || !service || onBehalf || headerLeft != "" {
		t.Fatalf("user=%d service=%v onBehalf=%v header=%q", gotID, service, onBehalf, headerLeft)
	}
}

func TestRequireAuth_OnBehalfWrongCredentialRejected(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	sessionUser, err := models.CreateUser(context.Background(), db, "session", "", "password123", "session@example.com", true, true, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Setenv(OnBehalfCredentialEnv, testOnBehalfCredential)

	handler := RequireAuth(sm, db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler ran for a wrong credential")
	}))

	setup := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), "userID", sessionUser.ID)
		w.WriteHeader(http.StatusOK)
	}))
	setupRR := httptest.NewRecorder()
	setup.ServeHTTP(setupRR, httptest.NewRequest(http.MethodGet, "/setup", nil))

	logs := captureLog(t)
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer wrong-on-behalf-credential")
	req.Header.Set(OnBehalfOfHeader, sessionUser.Email.String)
	for _, c := range setupRR.Result().Cookies() {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), testOnBehalfCredential) || strings.Contains(rr.Body.String(), "wrong-on-behalf-credential") {
		t.Fatal("response echoes a credential")
	}
	assertLogOmitsSecrets(t, logs, testOnBehalfCredential, "wrong-on-behalf-credential", sessionUser.Email.String)
}

func TestRequireAuth_OnBehalfUnknownOrMissingEmail(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	coach, err := models.CreateUser(context.Background(), db, "coach", "", "password123", "coach@example.com", true, true, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create coach: %v", err)
	}
	if err := models.InstallCoachServiceToken(context.Background(), db, coach.ID, "rlcst_coach_service_not_a_fallback"); err != nil {
		t.Fatalf("install coach-service token: %v", err)
	}
	t.Setenv(OnBehalfCredentialEnv, testOnBehalfCredential)

	handler := RequireAuth(sm, db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler ran without a mapped user")
	}))

	logs := captureLog(t)
	for _, email := range []string{"", "nobody@example.com"} {
		req := httptest.NewRequest(http.MethodGet, "/api/athletes", nil)
		req.Header.Set("Authorization", "Bearer "+testOnBehalfCredential)
		if email != "" {
			req.Header.Set(OnBehalfOfHeader, email)
		}
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("email %q status = %d, body %s", email, rr.Code, rr.Body.String())
		}
		body := rr.Body.String()
		if strings.Contains(body, coach.Username) || strings.Contains(body, testOnBehalfCredential) || strings.Contains(body, "rlcst_") {
			t.Fatalf("email %q response fell back or leaked a secret: %s", email, body)
		}
	}
	assertLogOmitsSecrets(t, logs, testOnBehalfCredential, "nobody@example.com", "rlcst_coach_service_not_a_fallback")

	// The coach-service bearer still works, and the email header does not
	// switch it to an unknown caller.
	var gotID int64
	var service bool
	okHandler := RequireAuth(sm, db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = UserFromContext(r.Context()).ID
		service = IsServiceCredential(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer rlcst_coach_service_not_a_fallback")
	req.Header.Set(OnBehalfOfHeader, "nobody@example.com")
	rr := httptest.NewRecorder()
	okHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || gotID != coach.ID || !service {
		t.Fatalf("coach-service status=%d user=%d service=%v body=%s", rr.Code, gotID, service, rr.Body.String())
	}
}

func TestRequireAuth_OnBehalfReservedPrefixDisabled(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	user, err := models.CreateUser(context.Background(), db, "coach", "", "password123", "coach@example.com", true, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Setenv(OnBehalfCredentialEnv, "rlcst_this_is_not_the_on_behalf_credential")

	handler := RequireAuth(sm, db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("rlcst_ prefix on the on-behalf env must not authenticate")
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+os.Getenv(OnBehalfCredentialEnv))
	req.Header.Set(OnBehalfOfHeader, user.Email.String)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func assertLogOmitsSecrets(t *testing.T, buf *bytes.Buffer, secrets ...string) {
	t.Helper()
	logged := buf.String()
	for _, secret := range secrets {
		if secret != "" && strings.Contains(logged, secret) {
			t.Fatalf("log contains %q", secret)
		}
	}
}

func TestOnBehalfRouteAllowed(t *testing.T) {
	cases := []struct {
		method, target string
		want           bool
	}{
		{http.MethodGet, "/api/me", true},
		{http.MethodGet, "/api/dashboard", true},
		{http.MethodGet, "/api/athletes", true},
		{http.MethodGet, "/api/athletes/7", true},
		{http.MethodPost, "/api/athletes/7/workouts/3/sets", true},
		{http.MethodPost, "/api/athletes/7/coach/programs/preview", true},
		{http.MethodPost, "/api/athletes/7/coach/programs/import", true},
		{http.MethodPost, "/api/athletes/7/coach/programs/assign", true},
		{http.MethodPost, "/api/athletes/7/coach/programs/9/gap-fill", true},
		{http.MethodGet, "/api/exercises", true},
		{http.MethodGet, "/api/programs/4", true},
		{http.MethodGet, "/api/reviews/pending", true},

		{http.MethodPost, "/api/athletes", false},
		{http.MethodPut, "/api/athletes/7", false},
		{http.MethodDelete, "/api/athletes/7", false},
		{http.MethodPost, "/api/exercises", false},
		{http.MethodPost, "/api/programs", false},
		{http.MethodPost, "/api/programs/4/approve", false},
		{http.MethodPost, "/api/me", false},
		{http.MethodGet, "/api/users", false},
		{http.MethodPost, "/api/users/1/service-tokens", false},
		{http.MethodDelete, "/api/users/1/service-tokens/2", false},
		{http.MethodGet, "/api/users/1/service-tokens", false},
		{http.MethodPost, "/api/users/1/tokens", false},
		{http.MethodGet, "/api/admin/settings", false},
		{http.MethodPost, "/api/admin/impersonate/2", false},
		{http.MethodPost, "/api/catalog/import/execute", false},
		{http.MethodPut, "/api/preferences", false},
		{http.MethodPost, "/api/avatars/upload", false},
		{http.MethodPost, "/api/logout", false},
		{http.MethodGet, "/api/notifications", false},
		{http.MethodGet, "/api/athletes/7/../../users", false},
		{http.MethodGet, "/api/athletes/7/coach/..%2F..%2F..%2Fusers", false},
		{http.MethodGet, "/api/athletes//coach/programs", false},
		{http.MethodGet, "/api/athletes/7/", false},
		{http.MethodGet, "/healthz", false},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, "http://replog.test"+c.target, nil)
		if got := onBehalfRouteAllowed(req); got != c.want {
			t.Errorf("%s %s allowed = %v, want %v", c.method, c.target, got, c.want)
		}
	}
}

func TestOnBehalfUserClearsAdminOnACopy(t *testing.T) {
	stored := &models.User{ID: 1, IsAdmin: true, IsCoach: true}
	scoped := onBehalfUser(stored)
	if scoped.IsAdmin || !scoped.IsCoach || scoped.ID != 1 {
		t.Fatalf("scoped = %+v", scoped)
	}
	if !stored.IsAdmin {
		t.Fatal("onBehalfUser changed the caller's user")
	}
}
