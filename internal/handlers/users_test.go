package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

func TestUsers_List_CoachCanView(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Users{DB: db, Templates: tc}

	req := requestWithUser("GET", "/users", nil, coach)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestUsers_List_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	h := &Users{DB: db, Templates: tc}

	req := requestWithUser("GET", "/users", nil, nonCoach)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestUsers_NewForm_CoachCanView(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Users{DB: db, Templates: tc}

	req := requestWithUser("GET", "/users/new", nil, coach)
	rr := httptest.NewRecorder()
	h.NewForm(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestUsers_Create_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Users{DB: db, Templates: tc}

	form := url.Values{
		"username": {"newuser"},
		"password": {"secret123"},
		"email":    {"new@test.com"},
	}
	req := requestWithUser("POST", "/users", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
}

func TestUsers_Create_EmptyUsername(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Users{DB: db, Templates: tc}

	form := url.Values{
		"username": {""},
		"password": {"secret123"},
	}
	req := requestWithUser("POST", "/users", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestUsers_Create_ShortPassword(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Users{DB: db, Templates: tc}

	form := url.Values{
		"username": {"newuser"},
		"password": {"abc"},
	}
	req := requestWithUser("POST", "/users", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestUsers_Create_DuplicateUsername(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Users{DB: db, Templates: tc}

	// "coach" username already exists from seedCoach.
	form := url.Values{
		"username": {"coach"},
		"password": {"secret123"},
	}
	req := requestWithUser("POST", "/users", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestUsers_Create_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	h := &Users{DB: db, Templates: tc}

	form := url.Values{
		"username": {"hacker"},
		"password": {"secret123"},
	}
	req := requestWithUser("POST", "/users", form, nonCoach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestUsers_EditForm_CoachCanView(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Users{DB: db, Templates: tc}

	req := requestWithUser("GET", "/users/"+itoa(coach.ID)+"/edit", nil, coach)
	req.SetPathValue("id", itoa(coach.ID))
	rr := httptest.NewRecorder()
	h.EditForm(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestUsers_Update_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")

	// Create a second user to update.
	target := seedNonCoach(t, db, athlete.ID)

	h := &Users{DB: db, Templates: tc}

	form := url.Values{
		"username": {"updated_kid"},
		"email":    {"kid@test.com"},
	}
	req := requestWithUser("POST", "/users/"+itoa(target.ID), form, coach)
	req.SetPathValue("id", itoa(target.ID))
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
}

func TestUsers_Delete_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	target := seedNonCoach(t, db, athlete.ID)

	h := &Users{DB: db, Templates: tc}

	req := requestWithUser("POST", "/users/"+itoa(target.ID)+"/delete", nil, coach)
	req.SetPathValue("id", itoa(target.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
}

func TestUsers_Delete_CannotDeleteSelf(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Users{DB: db, Templates: tc}

	req := requestWithUser("POST", "/users/"+itoa(coach.ID)+"/delete", nil, coach)
	req.SetPathValue("id", itoa(coach.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestUsers_Delete_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	h := &Users{DB: db, Templates: tc}

	req := requestWithUser("POST", "/users/1/delete", nil, nonCoach)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestUsers_Create_WithLinkedAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")

	h := &Users{DB: db, Templates: tc}

	form := url.Values{
		"username":   {"alice"},
		"password":   {"secret123"},
		"athlete_id": {itoa(athlete.ID)},
	}
	req := requestWithUser("POST", "/users", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	// Verify the linked athlete.
	u, err := models.GetUserByUsername(db, "alice")
	if err != nil {
		t.Fatalf("get user alice: %v", err)
	}
	if !u.AthleteID.Valid || u.AthleteID.Int64 != athlete.ID {
		t.Errorf("expected user alice linked to athlete %d, got %v", athlete.ID, u.AthleteID)
	}
}

func TestUsers_EditForm_NotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Users{DB: db, Templates: tc}

	req := requestWithUser("GET", "/users/999/edit", nil, coach)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()
	h.EditForm(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestUsers_EditForm_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	h := &Users{DB: db, Templates: tc}

	req := requestWithUser("GET", "/users/1/edit", nil, nonCoach)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	h.EditForm(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestUsers_Update_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	h := &Users{DB: db, Templates: tc}

	form := url.Values{"username": {"hacked"}}
	req := requestWithUser("POST", "/users/1", form, nonCoach)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestUsers_Update_EmptyUsername(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	target := seedNonCoach(t, db, athlete.ID)

	h := &Users{DB: db, Templates: tc}

	form := url.Values{"username": {""}}
	req := requestWithUser("POST", "/users/"+itoa(target.ID), form, coach)
	req.SetPathValue("id", itoa(target.ID))
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestUsers_Update_ShortPassword(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	target := seedNonCoach(t, db, athlete.ID)

	h := &Users{DB: db, Templates: tc}

	form := url.Values{"username": {"kid"}, "password": {"ab"}}
	req := requestWithUser("POST", "/users/"+itoa(target.ID), form, coach)
	req.SetPathValue("id", itoa(target.ID))
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestUsers_Update_DuplicateUsername(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	target := seedNonCoach(t, db, athlete.ID)

	h := &Users{DB: db, Templates: tc}

	// Try to rename target user to "coach" which already exists.
	form := url.Values{"username": {"coach"}}
	req := requestWithUser("POST", "/users/"+itoa(target.ID), form, coach)
	req.SetPathValue("id", itoa(target.ID))
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestUsers_NewForm_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	h := &Users{DB: db, Templates: tc}

	req := requestWithUser("GET", "/users/new", nil, nonCoach)
	rr := httptest.NewRecorder()
	h.NewForm(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestUsers_Create_DuplicateAthleteLink(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")

	// Link Alice to a user first.
	seedNonCoach(t, db, athlete.ID)

	h := &Users{DB: db, Templates: tc}

	// Try to create another user linked to the same athlete.
	form := url.Values{
		"username":   {"another"},
		"password":   {"secret123"},
		"athlete_id": {itoa(athlete.ID)},
	}
	req := requestWithUser("POST", "/users", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestUsers_Update_DuplicateAthleteLink(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	athlete1 := seedAthlete(t, db, "Alice", "")
	athlete2 := seedAthlete(t, db, "Bob", "")

	// Link Alice to user1, Bob to user2.
	seedNonCoach(t, db, athlete1.ID)
	user2 := seedNonCoachWithUsername(t, db, "bob_user", athlete2.ID)

	h := &Users{DB: db, Templates: tc}

	// Try to re-link user2 to Alice's athlete (already taken).
	form := url.Values{
		"username":   {"bob_user"},
		"athlete_id": {itoa(athlete1.ID)},
	}
	req := requestWithUser("POST", "/users/"+itoa(user2.ID), form, coach)
	req.SetPathValue("id", itoa(user2.ID))
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestUsers_Create_Passwordless(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Users{DB: db, Templates: tc}

	form := url.Values{
		"username": {"kiduser"},
	}
	req := requestWithUser("POST", "/users", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	u, err := models.GetUserByUsername(db, "kiduser")
	if err != nil {
		t.Fatalf("get user kiduser: %v", err)
	}
	if u.HasPassword() {
		t.Error("expected passwordless user, but HasPassword() is true")
	}
}

func TestUsers_Create_PasswordlessAutoToken(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	sm := testSessionManager()
	coach := seedCoach(t, db)

	h := &Users{DB: db, Sessions: sm, Templates: tc, BaseURL: "https://example.com"}

	form := url.Values{
		"username": {"kidtoken"},
	}

	// Track flash values set during Create.
	var flashSuccess, flashTokenURL string

	// Wrap handler with session middleware so flash values are persisted.
	handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Inject user context (normally done by auth middleware).
		ctx := context.WithValue(r.Context(), middleware.UserContextKey, coach)
		h.Create(w, r.WithContext(ctx))

		// Read flash values before the session commits.
		flashSuccess = sm.GetString(r.Context(), "flash_success")
		flashTokenURL = sm.GetString(r.Context(), "flash_token_url")
	}))

	req := httptest.NewRequest("POST", "/users", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	if flashSuccess == "" {
		t.Error("expected flash_success to be set")
	}
	if flashTokenURL == "" {
		t.Fatal("expected flash_token_url to be set")
	}
	if !strings.Contains(flashTokenURL, "https://example.com/auth/token/") {
		t.Errorf("token URL = %q, want prefix https://example.com/auth/token/", flashTokenURL)
	}

	// Verify a login token was actually created in the database.
	u, err := models.GetUserByUsername(db, "kidtoken")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	tokens, err := models.ListLoginTokensByUser(db, u.ID)
	if err != nil {
		t.Fatalf("list tokens: %v", err)
	}
	if len(tokens) != 1 {
		t.Errorf("expected 1 token, got %d", len(tokens))
	}
}

func TestUsers_Update_BlockPasswordForPasswordless(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	// Create a passwordless user.
	target, err := models.CreateUser(db, "nopw", "", "", "", false, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create passwordless user: %v", err)
	}

	h := &Users{DB: db, Templates: tc}

	form := url.Values{
		"username": {"nopw"},
		"password": {"newpassword123"},
	}
	req := requestWithUser("POST", "/users/"+itoa(target.ID), form, coach)
	req.SetPathValue("id", itoa(target.ID))
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestUsers_Create_InlineAthleteCreation(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Users{DB: db, Templates: tc}

	form := url.Values{
		"username":         {"kidwithathlete"},
		"password":         {"secret123"},
		"athlete_id":       {"__new__"},
		"new_athlete_name": {"Caydan"},
	}
	req := requestWithUser("POST", "/users", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	// Verify user was created and linked.
	u, err := models.GetUserByUsername(db, "kidwithathlete")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if !u.AthleteID.Valid {
		t.Fatal("expected user to be linked to an athlete")
	}

	// Verify athlete was created.
	athlete, err := models.GetAthleteByID(db, u.AthleteID.Int64)
	if err != nil {
		t.Fatalf("get athlete: %v", err)
	}
	if athlete.Name != "Caydan" {
		t.Errorf("athlete name = %q, want Caydan", athlete.Name)
	}
}

func TestUsers_Create_InlineAthleteDefaultsToUsername(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Users{DB: db, Templates: tc}

	form := url.Values{
		"username":         {"kidnoname"},
		"password":         {"secret123"},
		"athlete_id":       {"__new__"},
		"new_athlete_name": {""},
	}
	req := requestWithUser("POST", "/users", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	// Verify athlete was created with the username as name.
	u, err := models.GetUserByUsername(db, "kidnoname")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if !u.AthleteID.Valid {
		t.Fatal("expected user to be linked to an athlete")
	}
	athlete, err := models.GetAthleteByID(db, u.AthleteID.Int64)
	if err != nil {
		t.Fatalf("get athlete: %v", err)
	}
	if athlete.Name != "kidnoname" {
		t.Errorf("athlete name = %q, want kidnoname", athlete.Name)
	}
}
