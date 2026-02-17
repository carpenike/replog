package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func TestAuth_LoginPage_RendersForUnauthenticated(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	tc := testTemplateCache(t)

	auth := &Auth{DB: db, Sessions: sm, Templates: tc}

	req := httptest.NewRequest("GET", "/login", nil)
	rr := httptest.NewRecorder()

	// Wrap with LoadAndSave (required by session manager).
	handler := sm.LoadAndSave(http.HandlerFunc(auth.LoginPage))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAuth_LoginPage_RedirectsWhenLoggedIn(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	tc := testTemplateCache(t)

	user, err := models.CreateUser(db, "coach", "password123", "", true, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	auth := &Auth{DB: db, Sessions: sm, Templates: tc}

	// Set up the session first.
	setup := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), "userID", user.ID)
		w.WriteHeader(http.StatusOK)
	}))
	setupReq := httptest.NewRequest("GET", "/setup", nil)
	setupRR := httptest.NewRecorder()
	setup.ServeHTTP(setupRR, setupReq)

	// Now visit login with session cookie.
	handler := sm.LoadAndSave(http.HandlerFunc(auth.LoginPage))
	req := httptest.NewRequest("GET", "/login", nil)
	for _, c := range setupRR.Result().Cookies() {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Errorf("expected redirect to /, got %q", loc)
	}
}

func TestAuth_LoginSubmit_Success(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	tc := testTemplateCache(t)

	_, err := models.CreateUser(db, "coach", "password123", "c@test.com", true, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	auth := &Auth{DB: db, Sessions: sm, Templates: tc}

	form := url.Values{"username": {"coach"}, "password": {"password123"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	handler := sm.LoadAndSave(http.HandlerFunc(auth.LoginSubmit))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Errorf("expected redirect to /, got %q", loc)
	}
}

func TestAuth_LoginSubmit_InvalidCredentials(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	tc := testTemplateCache(t)

	_, err := models.CreateUser(db, "coach", "password123", "", true, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	auth := &Auth{DB: db, Sessions: sm, Templates: tc}

	form := url.Values{"username": {"coach"}, "password": {"wrongpassword"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	handler := sm.LoadAndSave(http.HandlerFunc(auth.LoginSubmit))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

func TestAuth_LoginSubmit_EmptyFields(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	tc := testTemplateCache(t)

	auth := &Auth{DB: db, Sessions: sm, Templates: tc}

	form := url.Values{"username": {""}, "password": {""}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	handler := sm.LoadAndSave(http.HandlerFunc(auth.LoginSubmit))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rr.Code)
	}
}

func TestAuth_Logout_DestroysSession(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	tc := testTemplateCache(t)

	auth := &Auth{DB: db, Sessions: sm, Templates: tc}

	handler := sm.LoadAndSave(http.HandlerFunc(auth.Logout))
	req := httptest.NewRequest("POST", "/logout", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

func TestAuth_LoginSubmit_PasswordlessUserRejects(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	tc := testTemplateCache(t)

	// Create a passwordless user.
	_, err := models.CreateUser(db, "kidonly", "", "", false, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create passwordless user: %v", err)
	}

	auth := &Auth{DB: db, Sessions: sm, Templates: tc}

	form := url.Values{"username": {"kidonly"}, "password": {"anything"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	handler := sm.LoadAndSave(http.HandlerFunc(auth.LoginSubmit))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}
