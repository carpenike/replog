package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func TestTrainingMaxes_NewForm_CoachCanView(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	ex := seedExercise(t, db, "Squat", "", 0)

	h := &TrainingMaxes{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/exercises/"+itoa(ex.ID)+"/training-max/new", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("exerciseID", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.NewForm(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestTrainingMaxes_NewForm_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)
	ex := seedExercise(t, db, "Squat", "", 0)

	h := &TrainingMaxes{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/exercises/"+itoa(ex.ID)+"/training-max/new", nil, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("exerciseID", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.NewForm(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestTrainingMaxes_Create_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	ex := seedExercise(t, db, "Squat", "", 0)

	h := &TrainingMaxes{DB: db, Templates: tc}

	form := url.Values{
		"weight":         {"315"},
		"effective_date": {"2026-02-10"},
		"notes":          {"new cycle"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/exercises/"+itoa(ex.ID)+"/training-max", form, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("exerciseID", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	// Verify training max was persisted.
	history, err := models.ListTrainingMaxHistory(db, athlete.ID, ex.ID)
	if err != nil {
		t.Fatalf("list tm history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 TM entry, got %d", len(history))
	}
	if history[0].Weight != 315 {
		t.Errorf("expected weight 315, got %f", history[0].Weight)
	}
}

func TestTrainingMaxes_Create_EmptyWeight(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	ex := seedExercise(t, db, "Squat", "", 0)

	h := &TrainingMaxes{DB: db, Templates: tc}

	form := url.Values{"weight": {""}}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/exercises/"+itoa(ex.ID)+"/training-max", form, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("exerciseID", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestTrainingMaxes_Create_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)
	ex := seedExercise(t, db, "Squat", "", 0)

	h := &TrainingMaxes{DB: db, Templates: tc}

	form := url.Values{"weight": {"225"}}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/exercises/"+itoa(ex.ID)+"/training-max", form, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("exerciseID", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestTrainingMaxes_History_CoachCanView(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	ex := seedExercise(t, db, "Squat", "", 0)

	// Seed some TM history.
	_, _ = models.SetTrainingMax(db, athlete.ID, ex.ID, 300, "2026-01-01", "")
	_, _ = models.SetTrainingMax(db, athlete.ID, ex.ID, 315, "2026-02-01", "")

	h := &TrainingMaxes{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/exercises/"+itoa(ex.ID)+"/training-max/history", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("exerciseID", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.History(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestTrainingMaxes_History_NonCoachOwnAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)
	ex := seedExercise(t, db, "Squat", "", 0)

	h := &TrainingMaxes{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/exercises/"+itoa(ex.ID)+"/training-max/history", nil, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("exerciseID", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.History(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestTrainingMaxes_History_NonCoachOtherForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	myAthlete := seedAthlete(t, db, "Kid", "")
	otherAthlete := seedAthlete(t, db, "Other", "")
	nonCoach := seedNonCoach(t, db, myAthlete.ID)
	ex := seedExercise(t, db, "Squat", "", 0)

	h := &TrainingMaxes{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(otherAthlete.ID)+"/exercises/"+itoa(ex.ID)+"/training-max/history", nil, nonCoach)
	req.SetPathValue("id", itoa(otherAthlete.ID))
	req.SetPathValue("exerciseID", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.History(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestTrainingMaxes_NewForm_AthleteNotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	ex := seedExercise(t, db, "Squat", "", 0)

	h := &TrainingMaxes{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/999/exercises/"+itoa(ex.ID)+"/training-max/new", nil, coach)
	req.SetPathValue("id", "999")
	req.SetPathValue("exerciseID", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.NewForm(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestTrainingMaxes_NewForm_ExerciseNotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")

	h := &TrainingMaxes{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/exercises/999/training-max/new", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("exerciseID", "999")
	rr := httptest.NewRecorder()
	h.NewForm(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestTrainingMaxes_History_AthleteNotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	ex := seedExercise(t, db, "Squat", "", 0)

	h := &TrainingMaxes{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/999/exercises/"+itoa(ex.ID)+"/training-max/history", nil, coach)
	req.SetPathValue("id", "999")
	req.SetPathValue("exerciseID", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.History(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestTrainingMaxes_History_ExerciseNotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")

	h := &TrainingMaxes{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/exercises/999/training-max/history", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("exerciseID", "999")
	rr := httptest.NewRecorder()
	h.History(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestTrainingMaxes_Create_InvalidWeight(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	ex := seedExercise(t, db, "Squat", "", 0)

	h := &TrainingMaxes{DB: db, Templates: tc}

	form := url.Values{"weight": {"not-a-number"}}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/exercises/"+itoa(ex.ID)+"/training-max", form, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("exerciseID", itoa(ex.ID))
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}
