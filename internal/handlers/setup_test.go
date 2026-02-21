package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func TestSetup_PasskeySetup_RendersPage(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Setup{DB: db, Sessions: sm, Templates: tc}

	r := requestWithUser("GET", "/setup/passkey", nil, coach)
	w := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(h.PasskeySetup)).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !containsSubstring(body, "Secure Your Account") {
		t.Error("response should contain wizard heading")
	}
	if !containsSubstring(body, "Skip for now") {
		t.Error("response should contain skip link")
	}
}

func TestSetup_PasskeySetup_ShowsSuccessOnDone(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Setup{DB: db, Sessions: sm, Templates: tc}

	r := requestWithUser("GET", "/setup/passkey?done=1", nil, coach)
	w := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(h.PasskeySetup)).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !containsSubstring(w.Body.String(), "Passkey registered!") {
		t.Error("response should contain success message")
	}
}

func TestSetup_PasskeySetupSkip_RedirectsHome(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Setup{DB: db, Sessions: sm, Templates: tc}

	r := requestWithUser("POST", "/setup/passkey/skip", nil, coach)
	w := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(h.PasskeySetupSkip)).ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/" {
		t.Errorf("redirect = %q, want /", loc)
	}
}

func TestSetup_NeedsPasskeySetup(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	tc := testTemplateCache(t)

	user, err := models.CreateUser(db, "testuser", "", "password123", "", false, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	h := &Setup{DB: db, Sessions: sm, Templates: tc}

	t.Run("true when no passkeys", func(t *testing.T) {
		r := requestWithUser("GET", "/", nil, user)
		w := httptest.NewRecorder()
		// Must run within session for skip flag check.
		sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !h.NeedsPasskeySetup(r, user.ID) {
				t.Error("NeedsPasskeySetup = false, want true for user with no passkeys")
			}
		})).ServeHTTP(w, r)
	})

	t.Run("false after skip", func(t *testing.T) {
		r := requestWithUser("GET", "/", nil, user)
		w := httptest.NewRecorder()
		sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sm.Put(r.Context(), "passkey_setup_skipped", true)
			if h.NeedsPasskeySetup(r, user.ID) {
				t.Error("NeedsPasskeySetup = true, want false after skip")
			}
		})).ServeHTTP(w, r)
	})
}
