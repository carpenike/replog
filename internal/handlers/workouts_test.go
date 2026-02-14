package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func TestWorkouts_List_CoachCanView(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/workouts", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestWorkouts_List_NonCoachCanViewOwn(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/workouts", nil, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestWorkouts_List_NonCoachForbiddenOther(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	myAthlete := seedAthlete(t, db, "Kid", "")
	otherAthlete := seedAthlete(t, db, "Other", "")
	nonCoach := seedNonCoach(t, db, myAthlete.ID)

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(otherAthlete.ID)+"/workouts", nil, nonCoach)
	req.SetPathValue("id", itoa(otherAthlete.ID))
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestWorkouts_Create_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")

	h := &Workouts{DB: db, Templates: tc}

	form := url.Values{"date": {"2026-02-10"}, "notes": {"Great session"}}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/workouts", form, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
}

func TestWorkouts_Create_DuplicateDate(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")

	// Create first workout.
	_, err := models.CreateWorkout(db, athlete.ID, "2026-02-10", "")
	if err != nil {
		t.Fatalf("create workout: %v", err)
	}

	h := &Workouts{DB: db, Templates: tc}

	// Try to create a second workout on the same date.
	form := url.Values{"date": {"2026-02-10"}}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/workouts", form, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	// Should redirect to existing workout (303) since the handler redirects on duplicate.
	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect to existing workout, got %d", rr.Code)
	}
}

func TestWorkouts_Show_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	workout, err := models.CreateWorkout(db, athlete.ID, "2026-02-10", "test")
	if err != nil {
		t.Fatalf("create workout: %v", err)
	}

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout.ID), nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.Show(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestWorkouts_UpdateNotes(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	workout, _ := models.CreateWorkout(db, athlete.ID, "2026-02-10", "")

	h := &Workouts{DB: db, Templates: tc}

	form := url.Values{"notes": {"Updated notes"}}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout.ID)+"/notes", form, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.UpdateNotes(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	updated, err := models.GetWorkoutByID(db, workout.ID)
	if err != nil {
		t.Fatalf("get workout: %v", err)
	}
	if !updated.Notes.Valid || updated.Notes.String != "Updated notes" {
		t.Errorf("expected notes 'Updated notes', got %v", updated.Notes)
	}
}

func TestWorkouts_Delete_CoachOnly(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	workout, _ := models.CreateWorkout(db, athlete.ID, "2026-02-10", "")

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout.ID)+"/delete", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
}

func TestWorkouts_Delete_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)
	workout, _ := models.CreateWorkout(db, athlete.ID, "2026-02-10", "")

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout.ID)+"/delete", nil, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestWorkouts_AddSet_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	ex := seedExercise(t, db, "Squat", "", 0)
	workout, _ := models.CreateWorkout(db, athlete.ID, "2026-02-10", "")

	h := &Workouts{DB: db, Templates: tc}

	form := url.Values{
		"exercise_id": {itoa(ex.ID)},
		"reps":        {"5"},
		"weight":      {"225"},
		"notes":       {"felt good"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout.ID)+"/sets", form, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.AddSet(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
}

func TestWorkouts_AddSet_InvalidReps(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	ex := seedExercise(t, db, "Squat", "", 0)
	workout, _ := models.CreateWorkout(db, athlete.ID, "2026-02-10", "")

	h := &Workouts{DB: db, Templates: tc}

	form := url.Values{
		"exercise_id": {itoa(ex.ID)},
		"reps":        {"0"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout.ID)+"/sets", form, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.AddSet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestWorkouts_UpdateSet_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	ex := seedExercise(t, db, "Squat", "", 0)
	workout, _ := models.CreateWorkout(db, athlete.ID, "2026-02-10", "")
	set, _ := models.AddSet(db, workout.ID, ex.ID, 5, 225, 0, "")

	h := &Workouts{DB: db, Templates: tc}

	form := url.Values{"reps": {"8"}, "weight": {"185"}, "notes": {"lighter"}}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout.ID)+"/sets/"+itoa(set.ID), form, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	req.SetPathValue("setID", itoa(set.ID))
	rr := httptest.NewRecorder()
	h.UpdateSet(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	updated, _ := models.GetSetByID(db, set.ID)
	if updated.Reps != 8 {
		t.Errorf("expected 8 reps, got %d", updated.Reps)
	}
}

func TestWorkouts_DeleteSet_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	ex := seedExercise(t, db, "Squat", "", 0)
	workout, _ := models.CreateWorkout(db, athlete.ID, "2026-02-10", "")
	set, _ := models.AddSet(db, workout.ID, ex.ID, 5, 225, 0, "")

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout.ID)+"/sets/"+itoa(set.ID)+"/delete", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	req.SetPathValue("setID", itoa(set.ID))
	rr := httptest.NewRecorder()
	h.DeleteSet(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	_, err := models.GetSetByID(db, set.ID)
	if err != models.ErrNotFound {
		t.Errorf("expected set to be deleted, got %v", err)
	}
}

// Tests for workout-to-athlete ownership verification.
// These ensure that accessing a workout via a different athlete's URL returns 404.

func TestWorkouts_Show_WrongAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete1 := seedAthlete(t, db, "Alice", "")
	athlete2 := seedAthlete(t, db, "Bob", "")
	workout, _ := models.CreateWorkout(db, athlete1.ID, "2026-02-10", "")

	h := &Workouts{DB: db, Templates: tc}

	// Try to view athlete1's workout via athlete2's URL.
	req := requestWithUser("GET", "/athletes/"+itoa(athlete2.ID)+"/workouts/"+itoa(workout.ID), nil, coach)
	req.SetPathValue("id", itoa(athlete2.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.Show(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for workout belonging to different athlete, got %d", rr.Code)
	}
}

func TestWorkouts_UpdateNotes_WrongAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete1 := seedAthlete(t, db, "Alice", "")
	athlete2 := seedAthlete(t, db, "Bob", "")
	workout, _ := models.CreateWorkout(db, athlete1.ID, "2026-02-10", "")

	h := &Workouts{DB: db, Templates: tc}

	form := url.Values{"notes": {"sneaky update"}}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete2.ID)+"/workouts/"+itoa(workout.ID)+"/notes", form, coach)
	req.SetPathValue("id", itoa(athlete2.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.UpdateNotes(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for workout belonging to different athlete, got %d", rr.Code)
	}
}

func TestWorkouts_Delete_WrongAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete1 := seedAthlete(t, db, "Alice", "")
	athlete2 := seedAthlete(t, db, "Bob", "")
	workout, _ := models.CreateWorkout(db, athlete1.ID, "2026-02-10", "")

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("POST", "/athletes/"+itoa(athlete2.ID)+"/workouts/"+itoa(workout.ID)+"/delete", nil, coach)
	req.SetPathValue("id", itoa(athlete2.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for workout belonging to different athlete, got %d", rr.Code)
	}

	// Verify workout was NOT deleted.
	_, err := models.GetWorkoutByID(db, workout.ID)
	if err != nil {
		t.Errorf("workout should still exist, got error: %v", err)
	}
}

func TestWorkouts_AddSet_WrongAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete1 := seedAthlete(t, db, "Alice", "")
	athlete2 := seedAthlete(t, db, "Bob", "")
	ex := seedExercise(t, db, "Squat", "", 0)
	workout, _ := models.CreateWorkout(db, athlete1.ID, "2026-02-10", "")

	h := &Workouts{DB: db, Templates: tc}

	form := url.Values{
		"exercise_id": {itoa(ex.ID)},
		"reps":        {"5"},
		"weight":      {"225"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete2.ID)+"/workouts/"+itoa(workout.ID)+"/sets", form, coach)
	req.SetPathValue("id", itoa(athlete2.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.AddSet(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for workout belonging to different athlete, got %d", rr.Code)
	}
}

func TestWorkouts_NewForm_CoachCanView(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/workouts/new", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.NewForm(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestWorkouts_NewForm_NonCoachOwnAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/workouts/new", nil, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.NewForm(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestWorkouts_NewForm_NonCoachOtherForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	myAthlete := seedAthlete(t, db, "Kid", "")
	otherAthlete := seedAthlete(t, db, "Other", "")
	nonCoach := seedNonCoach(t, db, myAthlete.ID)

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(otherAthlete.ID)+"/workouts/new", nil, nonCoach)
	req.SetPathValue("id", itoa(otherAthlete.ID))
	rr := httptest.NewRecorder()
	h.NewForm(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestWorkouts_NewForm_AthleteNotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/999/workouts/new", nil, coach)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()
	h.NewForm(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestWorkouts_EditSetForm_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	ex := seedExercise(t, db, "Squat", "", 0)
	workout, _ := models.CreateWorkout(db, athlete.ID, "2026-02-10", "")
	set, _ := models.AddSet(db, workout.ID, ex.ID, 5, 225, 0, "")

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout.ID)+"/sets/"+itoa(set.ID)+"/edit", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	req.SetPathValue("setID", itoa(set.ID))
	rr := httptest.NewRecorder()
	h.EditSetForm(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestWorkouts_EditSetForm_NonCoachOwnAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)
	ex := seedExercise(t, db, "Squat", "", 0)
	workout, _ := models.CreateWorkout(db, athlete.ID, "2026-02-10", "")
	set, _ := models.AddSet(db, workout.ID, ex.ID, 5, 225, 0, "")

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout.ID)+"/sets/"+itoa(set.ID)+"/edit", nil, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	req.SetPathValue("setID", itoa(set.ID))
	rr := httptest.NewRecorder()
	h.EditSetForm(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestWorkouts_EditSetForm_NonCoachOtherForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	myAthlete := seedAthlete(t, db, "Kid", "")
	otherAthlete := seedAthlete(t, db, "Other", "")
	nonCoach := seedNonCoach(t, db, myAthlete.ID)
	ex := seedExercise(t, db, "Squat", "", 0)
	workout, _ := models.CreateWorkout(db, otherAthlete.ID, "2026-02-10", "")
	set, _ := models.AddSet(db, workout.ID, ex.ID, 5, 225, 0, "")

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(otherAthlete.ID)+"/workouts/"+itoa(workout.ID)+"/sets/"+itoa(set.ID)+"/edit", nil, nonCoach)
	req.SetPathValue("id", itoa(otherAthlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	req.SetPathValue("setID", itoa(set.ID))
	rr := httptest.NewRecorder()
	h.EditSetForm(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestWorkouts_EditSetForm_WrongWorkout(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	ex := seedExercise(t, db, "Squat", "", 0)
	workout1, _ := models.CreateWorkout(db, athlete.ID, "2026-02-10", "")
	workout2, _ := models.CreateWorkout(db, athlete.ID, "2026-02-11", "")
	set, _ := models.AddSet(db, workout1.ID, ex.ID, 5, 225, 0, "")

	h := &Workouts{DB: db, Templates: tc}

	// Try to access set via the wrong workout ID.
	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout2.ID)+"/sets/"+itoa(set.ID)+"/edit", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout2.ID))
	req.SetPathValue("setID", itoa(set.ID))
	rr := httptest.NewRecorder()
	h.EditSetForm(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for set belonging to different workout, got %d", rr.Code)
	}
}

func TestWorkouts_EditSetForm_WrongAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete1 := seedAthlete(t, db, "Alice", "")
	athlete2 := seedAthlete(t, db, "Bob", "")
	ex := seedExercise(t, db, "Squat", "", 0)
	workout, _ := models.CreateWorkout(db, athlete1.ID, "2026-02-10", "")
	set, _ := models.AddSet(db, workout.ID, ex.ID, 5, 225, 0, "")

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete2.ID)+"/workouts/"+itoa(workout.ID)+"/sets/"+itoa(set.ID)+"/edit", nil, coach)
	req.SetPathValue("id", itoa(athlete2.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	req.SetPathValue("setID", itoa(set.ID))
	rr := httptest.NewRecorder()
	h.EditSetForm(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for workout belonging to different athlete, got %d", rr.Code)
	}
}

func TestWorkouts_Create_NonCoachOtherForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	myAthlete := seedAthlete(t, db, "Kid", "")
	otherAthlete := seedAthlete(t, db, "Other", "")
	nonCoach := seedNonCoach(t, db, myAthlete.ID)

	h := &Workouts{DB: db, Templates: tc}

	form := url.Values{"date": {"2026-02-10"}}
	req := requestWithUser("POST", "/athletes/"+itoa(otherAthlete.ID)+"/workouts", form, nonCoach)
	req.SetPathValue("id", itoa(otherAthlete.ID))
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestWorkouts_UpdateNotes_NonCoachForbiddenOther(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	myAthlete := seedAthlete(t, db, "Kid", "")
	otherAthlete := seedAthlete(t, db, "Other", "")
	nonCoach := seedNonCoach(t, db, myAthlete.ID)
	workout, _ := models.CreateWorkout(db, otherAthlete.ID, "2026-02-10", "")

	h := &Workouts{DB: db, Templates: tc}

	form := url.Values{"notes": {"sneaky"}}
	req := requestWithUser("POST", "/athletes/"+itoa(otherAthlete.ID)+"/workouts/"+itoa(workout.ID)+"/notes", form, nonCoach)
	req.SetPathValue("id", itoa(otherAthlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.UpdateNotes(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestWorkouts_AddSet_NonCoachForbiddenOther(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	myAthlete := seedAthlete(t, db, "Kid", "")
	otherAthlete := seedAthlete(t, db, "Other", "")
	nonCoach := seedNonCoach(t, db, myAthlete.ID)
	ex := seedExercise(t, db, "Squat", "", 0)
	workout, _ := models.CreateWorkout(db, otherAthlete.ID, "2026-02-10", "")

	h := &Workouts{DB: db, Templates: tc}

	form := url.Values{"exercise_id": {itoa(ex.ID)}, "reps": {"5"}, "weight": {"225"}}
	req := requestWithUser("POST", "/athletes/"+itoa(otherAthlete.ID)+"/workouts/"+itoa(workout.ID)+"/sets", form, nonCoach)
	req.SetPathValue("id", itoa(otherAthlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.AddSet(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestWorkouts_Show_WithPrescription(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Ryan", "")
	ex := seedExercise(t, db, "Bench Press", "", 0)

	// Set up a program template with a prescribed set.
	tmpl, err := models.CreateProgramTemplate(db, "5/3/1 BBB", "Boring But Big", 4, 4)
	if err != nil {
		t.Fatalf("create program template: %v", err)
	}
	reps := 5
	pct := 75.0
	_, err = models.CreatePrescribedSet(db, tmpl.ID, ex.ID, 1, 1, 1, &reps, &pct, "")
	if err != nil {
		t.Fatalf("create prescribed set: %v", err)
	}

	// Assign program to athlete.
	_, err = models.AssignProgram(db, athlete.ID, tmpl.ID, "2026-02-01", "")
	if err != nil {
		t.Fatalf("assign program: %v", err)
	}

	// Set a training max so target weight is calculated.
	_, err = models.SetTrainingMax(db, athlete.ID, ex.ID, 200.0, "2026-02-01", "")
	if err != nil {
		t.Fatalf("create training max: %v", err)
	}

	// Create a workout on the program start date (day 1).
	workout, err := models.CreateWorkout(db, athlete.ID, "2026-02-01", "")
	if err != nil {
		t.Fatalf("create workout: %v", err)
	}

	h := &Workouts{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout.ID), nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.Show(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Today&#39;s Prescription") && !strings.Contains(body, "Today's Prescription") {
		t.Error("expected prescription section in response body")
	}
	if !strings.Contains(body, "Bench Press") {
		t.Error("expected exercise name in prescription")
	}
	if !strings.Contains(body, "prefill-btn") {
		t.Error("expected pre-fill button in prescription")
	}
}
