package handlers

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

//go:embed testdata/templates
var testTemplateFS embed.FS

// testDB creates a fresh in-memory SQLite database with migrations applied.
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

// testTemplateCache builds a minimal template cache for handler tests.
// It uses embedded stub templates that define the required blocks.
func testTemplateCache(t testing.TB) TemplateCache {
	t.Helper()

	// Re-root the embedded FS so it looks like the production layout.
	sub, err := fs.Sub(testTemplateFS, "testdata")
	if err != nil {
		t.Fatalf("sub testdata FS: %v", err)
	}
	tc, err := NewTemplateCache(sub)
	if err != nil {
		t.Fatalf("parse test templates: %v", err)
	}
	return tc
}

// testSessionManager creates a cookie-based in-memory session manager for tests.
func testSessionManager() *scs.SessionManager {
	sm := scs.New()
	sm.Lifetime = 30 * 24 * time.Hour
	sm.Cookie.HttpOnly = true
	sm.Cookie.SameSite = http.SameSiteLaxMode
	return sm
}

// seedCoach creates a coach user and returns it. Useful for setting up auth context.
func seedCoach(t testing.TB, db *sql.DB) *models.User {
	t.Helper()
	user, err := models.CreateUser(db, "coach", "password123", "coach@test.com", true, sql.NullInt64{})
	if err != nil {
		t.Fatalf("seed coach: %v", err)
	}
	return user
}

// seedNonCoach creates a non-coach user linked to the given athlete.
func seedNonCoach(t testing.TB, db *sql.DB, athleteID int64) *models.User {
	t.Helper()
	user, err := models.CreateUser(db, "kid", "password123", "", false, sql.NullInt64{Int64: athleteID, Valid: true})
	if err != nil {
		t.Fatalf("seed non-coach: %v", err)
	}
	return user
}

// seedUnlinkedNonCoach creates a non-coach user with no linked athlete.
func seedUnlinkedNonCoach(t testing.TB, db *sql.DB) *models.User {
	t.Helper()
	user, err := models.CreateUser(db, "unlinked", "password123", "", false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("seed unlinked non-coach: %v", err)
	}
	return user
}

// seedAthlete creates an athlete and returns it.
func seedAthlete(t testing.TB, db *sql.DB, name, tier string) *models.Athlete {
	t.Helper()
	a, err := models.CreateAthlete(db, name, tier, "")
	if err != nil {
		t.Fatalf("seed athlete %q: %v", name, err)
	}
	return a
}

// seedExercise creates an exercise and returns it.
func seedExercise(t testing.TB, db *sql.DB, name, tier string, targetReps int) *models.Exercise {
	t.Helper()
	e, err := models.CreateExercise(db, name, tier, targetReps, "", "", 0)
	if err != nil {
		t.Fatalf("seed exercise %q: %v", name, err)
	}
	return e
}

// requestWithUser creates an HTTP request with the given user set in context
// (simulating the RequireAuth middleware).
func requestWithUser(method, target string, body url.Values, user *models.User) *http.Request {
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, target, strings.NewReader(body.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	ctx := context.WithValue(r.Context(), middleware.UserContextKey, user)
	return r.WithContext(ctx)
}

// itoa is a shorthand for strconv.FormatInt used in test URLs.
func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}
