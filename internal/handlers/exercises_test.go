package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func TestExercises_List_ReturnsOK(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	seedExercise(t, db, "Squat", "", 0)

	h := &Exercises{DB: db, Templates: tc}
	req := requestWithUser("GET", "/exercises", nil, coach)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestExercises_List_FilterByTier(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	seedExercise(t, db, "Lunges", "foundational", 20)
	seedExercise(t, db, "Bench", "intermediate", 15)

	h := &Exercises{DB: db, Templates: tc}

	tests := []struct {
		name string
		tier string
	}{
		{"all", ""},
		{"foundational", "foundational"},
		{"no tier", "none"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := requestWithUser("GET", "/exercises?tier="+tt.tier, nil, coach)
			rr := httptest.NewRecorder()
			h.List(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rr.Code)
			}
		})
	}
}

func TestExercises_Create_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Exercises{DB: db, Templates: tc}

	form := url.Values{"name": {"Bulgarian Split Squat"}, "tier": {"foundational"}, "target_reps": {"20"}}
	req := requestWithUser("POST", "/exercises", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
}

func TestExercises_Create_EmptyName(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Exercises{DB: db, Templates: tc}

	form := url.Values{"name": {""}}
	req := requestWithUser("POST", "/exercises", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestExercises_Create_DuplicateName(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	seedExercise(t, db, "Squat", "", 0)

	h := &Exercises{DB: db, Templates: tc}

	form := url.Values{"name": {"Squat"}}
	req := requestWithUser("POST", "/exercises", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestExercises_Create_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	nonCoach := seedUnlinkedNonCoach(t, db)

	h := &Exercises{DB: db, Templates: tc}

	form := url.Values{"name": {"Squat"}}
	req := requestWithUser("POST", "/exercises", form, nonCoach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestExercises_Show_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	ex := seedExercise(t, db, "Squat", "", 0)

	h := &Exercises{DB: db, Templates: tc}

	req := requestWithUser("GET", "/exercises/"+itoa(ex.ID), nil, coach)
	req.SetPathValue("id", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.Show(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestExercises_Show_NotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Exercises{DB: db, Templates: tc}

	req := requestWithUser("GET", "/exercises/999", nil, coach)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()
	h.Show(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestExercises_Update_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	ex := seedExercise(t, db, "Squat", "", 0)

	h := &Exercises{DB: db, Templates: tc}

	form := url.Values{"name": {"Back Squat"}, "tier": {"intermediate"}}
	req := requestWithUser("POST", "/exercises/"+itoa(ex.ID), form, coach)
	req.SetPathValue("id", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	updated, err := models.GetExerciseByID(db, ex.ID)
	if err != nil {
		t.Fatalf("get exercise: %v", err)
	}
	if updated.Name != "Back Squat" {
		t.Errorf("expected 'Back Squat', got %q", updated.Name)
	}
}

func TestExercises_Delete_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	ex := seedExercise(t, db, "ToDelete", "", 0)

	h := &Exercises{DB: db, Templates: tc}

	req := requestWithUser("POST", "/exercises/"+itoa(ex.ID)+"/delete", nil, coach)
	req.SetPathValue("id", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
}

func TestExercises_Delete_InUseReturnsConflict(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	ex := seedExercise(t, db, "Squat", "", 0)
	athlete := seedAthlete(t, db, "Alice", "")

	// Create a workout and log a set to make the exercise "in use".
	w, err := models.CreateWorkout(db, athlete.ID, "2026-01-01", "")
	if err != nil {
		t.Fatalf("create workout: %v", err)
	}
	_, err = models.AddSet(db, w.ID, ex.ID, 5, 100, "")
	if err != nil {
		t.Fatalf("add set: %v", err)
	}

	h := &Exercises{DB: db, Templates: tc}

	req := requestWithUser("POST", "/exercises/"+itoa(ex.ID)+"/delete", nil, coach)
	req.SetPathValue("id", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d", rr.Code)
	}
}

func TestExercises_ExerciseHistory_CoachCanView(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	ex := seedExercise(t, db, "Squat", "", 0)

	h := &Exercises{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/exercises/"+itoa(ex.ID)+"/history", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("exerciseID", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.ExerciseHistory(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
