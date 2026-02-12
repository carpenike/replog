package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func TestAssignments_Assign_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	ex := seedExercise(t, db, "Squat", "", 0)

	h := &Assignments{DB: db, Templates: tc}

	form := url.Values{"exercise_id": {itoa(ex.ID)}}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/assignments", form, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.Assign(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	// Verify assignment was created.
	assignments, err := models.ListActiveAssignments(db, athlete.ID)
	if err != nil {
		t.Fatalf("list assignments: %v", err)
	}
	if len(assignments) != 1 {
		t.Errorf("expected 1 assignment, got %d", len(assignments))
	}
}

func TestAssignments_Assign_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)
	ex := seedExercise(t, db, "Squat", "", 0)

	h := &Assignments{DB: db, Templates: tc}

	form := url.Values{"exercise_id": {itoa(ex.ID)}}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/assignments", form, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.Assign(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestAssignments_Assign_AlreadyAssigned(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	ex := seedExercise(t, db, "Squat", "", 0)

	// Assign once directly.
	_, err := models.AssignExercise(db, athlete.ID, ex.ID)
	if err != nil {
		t.Fatalf("assign: %v", err)
	}

	h := &Assignments{DB: db, Templates: tc}

	form := url.Values{"exercise_id": {itoa(ex.ID)}}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/assignments", form, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.Assign(rr, req)

	// Should redirect (not error) even when already assigned.
	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rr.Code)
	}
}

func TestAssignments_Deactivate_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	ex := seedExercise(t, db, "Squat", "", 0)
	assignment, _ := models.AssignExercise(db, athlete.ID, ex.ID)

	h := &Assignments{DB: db, Templates: tc}

	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/assignments/"+itoa(assignment.ID)+"/deactivate", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("assignmentID", itoa(assignment.ID))
	rr := httptest.NewRecorder()
	h.Deactivate(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	// Verify no active assignments remain.
	active, _ := models.ListActiveAssignments(db, athlete.ID)
	if len(active) != 0 {
		t.Errorf("expected 0 active assignments, got %d", len(active))
	}
}

func TestAssignments_Deactivate_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)
	ex := seedExercise(t, db, "Squat", "", 0)
	assignment, _ := models.AssignExercise(db, athlete.ID, ex.ID)

	h := &Assignments{DB: db, Templates: tc}

	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/assignments/"+itoa(assignment.ID)+"/deactivate", nil, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("assignmentID", itoa(assignment.ID))
	rr := httptest.NewRecorder()
	h.Deactivate(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestAssignments_AssignForm_CoachCanView(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")

	h := &Assignments{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/assignments/new", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.AssignForm(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAssignments_AssignForm_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	h := &Assignments{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/assignments/new", nil, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.AssignForm(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestAssignments_Reactivate_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "")
	ex := seedExercise(t, db, "Squat", "", 0)

	// Assign then deactivate.
	assignment, _ := models.AssignExercise(db, athlete.ID, ex.ID)
	_ = models.DeactivateAssignment(db, assignment.ID)

	h := &Assignments{DB: db, Templates: tc}

	form := url.Values{"exercise_id": {itoa(ex.ID)}}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/assignments/reactivate", form, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.Reactivate(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	// Verify one active assignment exists again.
	active, _ := models.ListActiveAssignments(db, athlete.ID)
	if len(active) != 1 {
		t.Errorf("expected 1 active assignment after reactivate, got %d", len(active))
	}
}

func TestAssignments_Reactivate_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)
	ex := seedExercise(t, db, "Squat", "", 0)

	h := &Assignments{DB: db, Templates: tc}

	form := url.Values{"exercise_id": {itoa(ex.ID)}}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/assignments/reactivate", form, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.Reactivate(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}
