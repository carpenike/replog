package middleware

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/models"
)

func testDB(t testing.TB) *sql.DB {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := database.RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("run migrations: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func testSessionManager() *scs.SessionManager {
	sm := scs.New()
	sm.Lifetime = 30 * 24 * time.Hour
	return sm
}

func TestRequireAuth_RedirectsWhenNotAuthenticated(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()

	handler := RequireAuth(sm, db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for unauthenticated request")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

func TestRequireAuth_SetsUserInContext(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()

	user, err := models.CreateUser(db, "testcoach", "password123", "test@example.com", true, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	var gotUser *models.User
	handler := RequireAuth(sm, db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// Use LoadAndSave wrapping to populate the session, then make a request.
	// We need to simulate a session that has userID set.
	setupHandler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), "userID", user.ID)
		w.WriteHeader(http.StatusOK)
	}))

	// Step 1: Set session
	setupReq := httptest.NewRequest("GET", "/setup", nil)
	setupRR := httptest.NewRecorder()
	setupHandler.ServeHTTP(setupRR, setupReq)

	// Step 2: Use the session cookie
	cookies := setupRR.Result().Cookies()
	req := httptest.NewRequest("GET", "/", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if gotUser == nil {
		t.Fatal("expected user in context, got nil")
	}
	if gotUser.ID != user.ID {
		t.Errorf("expected user ID %d, got %d", user.ID, gotUser.ID)
	}
}

func TestRequireAuth_InvalidSessionRedirects(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()

	// Create and then delete the user, so the session points to a nonexistent user.
	user, err := models.CreateUser(db, "ghostuser", "password123", "", true, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	setupHandler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), "userID", user.ID)
		w.WriteHeader(http.StatusOK)
	}))
	setupReq := httptest.NewRequest("GET", "/setup", nil)
	setupRR := httptest.NewRecorder()
	setupHandler.ServeHTTP(setupRR, setupReq)

	// Delete the user so the session lookup fails.
	if err := models.DeleteUser(db, user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	handler := RequireAuth(sm, db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for deleted user")
	}))

	cookies := setupRR.Result().Cookies()
	req := httptest.NewRequest("GET", "/", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect status %d, got %d", http.StatusSeeOther, rr.Code)
	}
}

func TestUserFromContext_ReturnsNilWithoutUser(t *testing.T) {
	ctx := context.Background()
	if u := UserFromContext(ctx); u != nil {
		t.Errorf("expected nil user, got %+v", u)
	}
}

func TestCanAccessAthlete(t *testing.T) {
	tests := []struct {
		name      string
		isCoach   bool
		athleteID sql.NullInt64
		targetID  int64
		want      bool
	}{
		{
			name:     "coach can access any athlete",
			isCoach:  true,
			targetID: 99,
			want:     true,
		},
		{
			name:      "non-coach can access own athlete",
			isCoach:   false,
			athleteID: sql.NullInt64{Int64: 5, Valid: true},
			targetID:  5,
			want:      true,
		},
		{
			name:      "non-coach cannot access other athlete",
			isCoach:   false,
			athleteID: sql.NullInt64{Int64: 5, Valid: true},
			targetID:  10,
			want:      false,
		},
		{
			name:     "non-coach without linked athlete cannot access",
			isCoach:  false,
			targetID: 5,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &models.User{
				IsCoach:   tt.isCoach,
				AthleteID: tt.athleteID,
			}
			got := CanAccessAthlete(user, tt.targetID)
			if got != tt.want {
				t.Errorf("CanAccessAthlete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequireCoach_ForbidsNonCoach(t *testing.T) {
	nonCoach := &models.User{IsCoach: false}

	handler := RequireCoach(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for non-coach")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	ctx := context.WithValue(req.Context(), UserContextKey, nonCoach)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", rr.Code)
	}
}

func TestRequireCoach_AllowsCoach(t *testing.T) {
	coach := &models.User{IsCoach: true}
	called := false

	handler := RequireCoach(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	ctx := context.WithValue(req.Context(), UserContextKey, coach)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("handler should have been called for coach")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
