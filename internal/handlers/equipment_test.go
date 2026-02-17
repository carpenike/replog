package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func TestEquipment_List_ReturnsOK(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	models.CreateEquipment(db, "Barbell", "")

	h := &Equipment{DB: db, Templates: tc}
	req := requestWithUser("GET", "/equipment", nil, coach)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestEquipment_Create_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Equipment{DB: db, Templates: tc}

	form := url.Values{"name": {"Barbell"}, "description": {"Standard 45lb barbell"}}
	req := requestWithUser("POST", "/equipment", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
}

func TestEquipment_Create_EmptyName(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Equipment{DB: db, Templates: tc}

	form := url.Values{"name": {""}}
	req := requestWithUser("POST", "/equipment", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestEquipment_Create_DuplicateName(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	models.CreateEquipment(db, "Barbell", "")

	h := &Equipment{DB: db, Templates: tc}

	form := url.Values{"name": {"Barbell"}}
	req := requestWithUser("POST", "/equipment", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestEquipment_Create_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	nonCoach := seedUnlinkedNonCoach(t, db)

	h := &Equipment{DB: db, Templates: tc}

	form := url.Values{"name": {"Barbell"}}
	req := requestWithUser("POST", "/equipment", form, nonCoach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestEquipment_EditForm_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	eq, _ := models.CreateEquipment(db, "Barbell", "")

	h := &Equipment{DB: db, Templates: tc}

	req := requestWithUser("GET", "/equipment/"+itoa(eq.ID)+"/edit", nil, coach)
	req.SetPathValue("id", itoa(eq.ID))
	rr := httptest.NewRecorder()
	h.EditForm(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestEquipment_EditForm_NotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Equipment{DB: db, Templates: tc}

	req := requestWithUser("GET", "/equipment/999/edit", nil, coach)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()
	h.EditForm(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestEquipment_Update_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	eq, _ := models.CreateEquipment(db, "Barbell", "")

	h := &Equipment{DB: db, Templates: tc}

	form := url.Values{"name": {"Olympic Barbell"}, "description": {"20kg"}}
	req := requestWithUser("POST", "/equipment/"+itoa(eq.ID), form, coach)
	req.SetPathValue("id", itoa(eq.ID))
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	updated, err := models.GetEquipmentByID(db, eq.ID)
	if err != nil {
		t.Fatalf("get equipment: %v", err)
	}
	if updated.Name != "Olympic Barbell" {
		t.Errorf("expected 'Olympic Barbell', got %q", updated.Name)
	}
}

func TestEquipment_Update_EmptyName(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	eq, _ := models.CreateEquipment(db, "Barbell", "")

	h := &Equipment{DB: db, Templates: tc}

	form := url.Values{"name": {""}}
	req := requestWithUser("POST", "/equipment/"+itoa(eq.ID), form, coach)
	req.SetPathValue("id", itoa(eq.ID))
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestEquipment_Delete_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	eq, _ := models.CreateEquipment(db, "Barbell", "")

	h := &Equipment{DB: db, Templates: tc}

	req := requestWithUser("POST", "/equipment/"+itoa(eq.ID)+"/delete", nil, coach)
	req.SetPathValue("id", itoa(eq.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
}

func TestEquipment_Delete_NotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Equipment{DB: db, Templates: tc}

	req := requestWithUser("POST", "/equipment/999/delete", nil, coach)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestEquipment_AddExerciseEquipment_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	ex := seedExercise(t, db, "Bench Press", "")
	eq, _ := models.CreateEquipment(db, "Barbell", "")

	h := &Equipment{DB: db, Templates: tc}

	form := url.Values{
		"equipment_id": {itoa(eq.ID)},
		"optional":     {"0"},
	}
	req := requestWithUser("POST", "/exercises/"+itoa(ex.ID)+"/equipment", form, coach)
	req.SetPathValue("id", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.AddExerciseEquipment(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	// Verify link exists.
	items, _ := models.ListExerciseEquipment(db, ex.ID)
	if len(items) != 1 {
		t.Errorf("exercise equipment count = %d, want 1", len(items))
	}
}

func TestEquipment_RemoveExerciseEquipment_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	ex := seedExercise(t, db, "Bench Press", "")
	eq, _ := models.CreateEquipment(db, "Barbell", "")
	models.AddExerciseEquipment(db, ex.ID, eq.ID, false)

	h := &Equipment{DB: db, Templates: tc}

	req := requestWithUser("POST", "/exercises/"+itoa(ex.ID)+"/equipment/"+itoa(eq.ID)+"/delete", nil, coach)
	req.SetPathValue("id", itoa(ex.ID))
	req.SetPathValue("equipmentID", itoa(eq.ID))
	rr := httptest.NewRecorder()
	h.RemoveExerciseEquipment(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	items, _ := models.ListExerciseEquipment(db, ex.ID)
	if len(items) != 0 {
		t.Errorf("exercise equipment count = %d, want 0", len(items))
	}
}

func TestEquipment_AthleteEquipmentPage_Coach(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Test Kid", "")

	h := &Equipment{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/equipment", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.AthleteEquipmentPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestEquipment_AddAthleteEquipment_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Test Kid", "")
	eq, _ := models.CreateEquipment(db, "Dumbbells", "")

	h := &Equipment{DB: db, Templates: tc}

	form := url.Values{"equipment_id": {itoa(eq.ID)}}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/equipment", form, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.AddAthleteEquipment(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	items, _ := models.ListAthleteEquipment(db, athlete.ID)
	if len(items) != 1 {
		t.Errorf("athlete equipment count = %d, want 1", len(items))
	}
}

func TestEquipment_RemoveAthleteEquipment_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Test Kid", "")
	eq, _ := models.CreateEquipment(db, "Dumbbells", "")
	models.AddAthleteEquipment(db, athlete.ID, eq.ID)

	h := &Equipment{DB: db, Templates: tc}

	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/equipment/"+itoa(eq.ID)+"/delete", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("equipmentID", itoa(eq.ID))
	rr := httptest.NewRecorder()
	h.RemoveAthleteEquipment(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	items, _ := models.ListAthleteEquipment(db, athlete.ID)
	if len(items) != 0 {
		t.Errorf("athlete equipment count = %d, want 0", len(items))
	}
}

func TestEquipment_AthleteEquipmentPage_UnauthorizedForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	nonCoach := seedUnlinkedNonCoach(t, db)
	athlete := seedAthlete(t, db, "Other Athlete", "")

	h := &Equipment{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/equipment", nil, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.AthleteEquipmentPage(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}
