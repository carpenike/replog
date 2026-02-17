package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/carpenike/replog/internal/models"
)

func TestLoginTokens_TokenLogin_Success(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	tc := testTemplateCache(t)

	user, err := models.CreateUser(db, "kid1", "", "password123", "", false, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	lt, err := models.CreateLoginToken(db, user.ID, "iPad", nil)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	h := &LoginTokens{DB: db, Sessions: sm, Templates: tc}

	req := httptest.NewRequest("GET", "/auth/token/"+lt.Token, nil)
	req.SetPathValue("token", lt.Token)
	rr := httptest.NewRecorder()

	handler := sm.LoadAndSave(http.HandlerFunc(h.TokenLogin))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Errorf("expected redirect to /, got %q", loc)
	}
}

func TestLoginTokens_TokenLogin_InvalidToken(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	tc := testTemplateCache(t)

	h := &LoginTokens{DB: db, Sessions: sm, Templates: tc}

	req := httptest.NewRequest("GET", "/auth/token/bogus-token", nil)
	req.SetPathValue("token", "bogus-token")
	rr := httptest.NewRecorder()

	handler := sm.LoadAndSave(http.HandlerFunc(h.TokenLogin))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

func TestLoginTokens_TokenLogin_ExpiredToken(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	tc := testTemplateCache(t)

	user, _ := models.CreateUser(db, "kid2", "", "password123", "", false, false, sql.NullInt64{})

	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	lt, _ := models.CreateLoginToken(db, user.ID, "expired", &past)

	h := &LoginTokens{DB: db, Sessions: sm, Templates: tc}

	req := httptest.NewRequest("GET", "/auth/token/"+lt.Token, nil)
	req.SetPathValue("token", lt.Token)
	rr := httptest.NewRecorder()

	handler := sm.LoadAndSave(http.HandlerFunc(h.TokenLogin))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

func TestLoginTokens_TokenLogin_EmptyToken(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	tc := testTemplateCache(t)

	h := &LoginTokens{DB: db, Sessions: sm, Templates: tc}

	req := httptest.NewRequest("GET", "/auth/token/", nil)
	req.SetPathValue("token", "")
	rr := httptest.NewRecorder()

	handler := sm.LoadAndSave(http.HandlerFunc(h.TokenLogin))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestLoginTokens_GenerateToken_CoachCanGenerate(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	kid, _ := models.CreateUser(db, "kiduser", "", "password123", "", false, false, sql.NullInt64{})

	h := &LoginTokens{DB: db, Templates: tc}

	form := url.Values{"label": {"iPad"}}
	req := requestWithUser("POST", "/users/"+itoa(kid.ID)+"/tokens", form, coach)
	req.SetPathValue("id", itoa(kid.ID))
	rr := httptest.NewRecorder()
	h.GenerateToken(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "/auth/token/") {
		t.Error("response should contain the login token URL")
	}

	// Verify token was created in DB.
	tokens, _ := models.ListLoginTokensByUser(db, kid.ID)
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if !tokens[0].Label.Valid || tokens[0].Label.String != "iPad" {
		t.Errorf("label = %v, want iPad", tokens[0].Label)
	}
}

func TestLoginTokens_GenerateToken_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "KidAthlete", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	h := &LoginTokens{DB: db, Templates: tc}

	form := url.Values{"label": {"iPad"}}
	req := requestWithUser("POST", "/users/"+itoa(nonCoach.ID)+"/tokens", form, nonCoach)
	req.SetPathValue("id", itoa(nonCoach.ID))
	rr := httptest.NewRecorder()
	h.GenerateToken(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestLoginTokens_DeleteToken_CoachCanDelete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	kid, _ := models.CreateUser(db, "kiduser2", "", "password123", "", false, false, sql.NullInt64{})
	lt, _ := models.CreateLoginToken(db, kid.ID, "iPad", nil)

	h := &LoginTokens{DB: db, Templates: tc}

	req := requestWithUser("POST", "/users/"+itoa(kid.ID)+"/tokens/"+itoa(lt.ID)+"/delete", nil, coach)
	req.SetPathValue("id", itoa(kid.ID))
	req.SetPathValue("tokenID", itoa(lt.ID))
	rr := httptest.NewRecorder()
	h.DeleteToken(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	// Verify token was deleted.
	tokens, _ := models.ListLoginTokensByUser(db, kid.ID)
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}

func TestLoginTokens_DeleteToken_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "KidAthlete2", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	lt, _ := models.CreateLoginToken(db, nonCoach.ID, "iPad", nil)

	h := &LoginTokens{DB: db, Templates: tc}

	req := requestWithUser("POST", "/users/"+itoa(nonCoach.ID)+"/tokens/"+itoa(lt.ID)+"/delete", nil, nonCoach)
	req.SetPathValue("id", itoa(nonCoach.ID))
	req.SetPathValue("tokenID", itoa(lt.ID))
	rr := httptest.NewRecorder()
	h.DeleteToken(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}
