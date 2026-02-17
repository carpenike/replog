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

	user, err := models.CreateUser(db, "testcoach", "", "password123", "test@example.com", true, false, sql.NullInt64{})
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
	user, err := models.CreateUser(db, "ghostuser", "", "password123", "", true, false, sql.NullInt64{})
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
	db := testDB(t)

	// Create a coach user.
	coach, err := models.CreateUser(db, "coach1", "", "password123", "", true, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create coach: %v", err)
	}
	// Create an admin user.
	admin, err := models.CreateUser(db, "admin1", "", "password123", "", false, true, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}

	// Create athletes: one owned by coach, one unowned.
	ownedAthlete, err := models.CreateAthlete(db, "OwnedKid", "", "", sql.NullInt64{Int64: coach.ID, Valid: true})
	if err != nil {
		t.Fatalf("create owned athlete: %v", err)
	}
	unownedAthlete, err := models.CreateAthlete(db, "UnownedKid", "", "", sql.NullInt64{})
	if err != nil {
		t.Fatalf("create unowned athlete: %v", err)
	}

	// Create a non-coach user linked to ownedAthlete.
	kid, err := models.CreateUser(db, "kid1", "", "password123", "", false, false, sql.NullInt64{Int64: ownedAthlete.ID, Valid: true})
	if err != nil {
		t.Fatalf("create kid: %v", err)
	}

	tests := []struct {
		name     string
		user     *models.User
		targetID int64
		want     bool
	}{
		{
			name:     "admin can access any athlete",
			user:     admin,
			targetID: unownedAthlete.ID,
			want:     true,
		},
		{
			name:     "coach can access owned athlete",
			user:     coach,
			targetID: ownedAthlete.ID,
			want:     true,
		},
		{
			name:     "coach cannot access unowned athlete",
			user:     coach,
			targetID: unownedAthlete.ID,
			want:     false,
		},
		{
			name:     "non-coach can access own athlete",
			user:     kid,
			targetID: ownedAthlete.ID,
			want:     true,
		},
		{
			name:     "non-coach cannot access other athlete",
			user:     kid,
			targetID: unownedAthlete.ID,
			want:     false,
		},
		{
			name:     "non-coach without linked athlete cannot access",
			user:     &models.User{},
			targetID: ownedAthlete.ID,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanAccessAthlete(db, tt.user, tt.targetID)
			if got != tt.want {
				t.Errorf("CanAccessAthlete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequireCoach_ForbidsNonCoach(t *testing.T) {
	nonCoach := &models.User{IsCoach: false}

	handler := RequireCoach(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	handler := RequireCoach(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
