package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

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
