package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func TestReviews_SubmitReview_CoachCanReview(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "ReviewKid", "")
	workout, _ := models.CreateWorkout(db, athlete.ID, "2026-02-15", "")

	h := &Reviews{DB: db, Templates: tc}

	body := url.Values{
		"status": {"approved"},
		"notes":  {"Great session!"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout.ID)+"/review", body, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.SubmitReview(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	// Verify the review was created.
	rev, err := models.GetWorkoutReviewByWorkoutID(db, workout.ID)
	if err != nil {
		t.Fatalf("get review: %v", err)
	}
	if rev.Status != models.ReviewStatusApproved {
		t.Errorf("status = %q, want %q", rev.Status, models.ReviewStatusApproved)
	}
	if !rev.Notes.Valid || rev.Notes.String != "Great session!" {
		t.Errorf("notes = %v, want 'Great session!'", rev.Notes)
	}
}

func TestReviews_SubmitReview_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)
	workout, _ := models.CreateWorkout(db, athlete.ID, "2026-02-15", "")

	h := &Reviews{DB: db, Templates: tc}

	body := url.Values{
		"status": {"approved"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout.ID)+"/review", body, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.SubmitReview(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestReviews_SubmitReview_InvalidStatus(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Kid", "")
	workout, _ := models.CreateWorkout(db, athlete.ID, "2026-02-15", "")

	h := &Reviews{DB: db, Templates: tc}

	body := url.Values{
		"status": {"invalid_status"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout.ID)+"/review", body, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.SubmitReview(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestReviews_SubmitReview_UpdateExisting(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Kid", "")
	workout, _ := models.CreateWorkout(db, athlete.ID, "2026-02-15", "")

	// Create initial review.
	models.CreateWorkoutReview(db, workout.ID, coach.ID, models.ReviewStatusApproved, "Good")

	h := &Reviews{DB: db, Templates: tc}

	body := url.Values{
		"status": {"needs_work"},
		"notes":  {"Actually, fix your form"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout.ID)+"/review", body, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.SubmitReview(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	rev, err := models.GetWorkoutReviewByWorkoutID(db, workout.ID)
	if err != nil {
		t.Fatalf("get review: %v", err)
	}
	if rev.Status != models.ReviewStatusNeedsWork {
		t.Errorf("status = %q, want %q", rev.Status, models.ReviewStatusNeedsWork)
	}
}

func TestReviews_SubmitReview_WrongAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete1 := seedAthlete(t, db, "Kid1", "")
	athlete2 := seedAthlete(t, db, "Kid2", "")
	workout, _ := models.CreateWorkout(db, athlete1.ID, "2026-02-15", "")

	h := &Reviews{DB: db, Templates: tc}

	body := url.Values{
		"status": {"approved"},
	}
	// Use athlete2's URL path but workout belongs to athlete1.
	req := requestWithUser("POST", "/athletes/"+itoa(athlete2.ID)+"/workouts/"+itoa(workout.ID)+"/review", body, coach)
	req.SetPathValue("id", itoa(athlete2.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.SubmitReview(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestReviews_DeleteReview(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Kid", "")
	workout, _ := models.CreateWorkout(db, athlete.ID, "2026-02-15", "")

	models.CreateWorkoutReview(db, workout.ID, coach.ID, models.ReviewStatusApproved, "")

	h := &Reviews{DB: db, Templates: tc}

	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/workouts/"+itoa(workout.ID)+"/review/delete", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", itoa(workout.ID))
	rr := httptest.NewRecorder()
	h.DeleteReview(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	// Verify the review was deleted.
	_, err := models.GetWorkoutReviewByWorkoutID(db, workout.ID)
	if err != models.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestReviews_DeleteReview_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	h := &Reviews{DB: db, Templates: tc}

	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/workouts/1/review/delete", nil, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	req.SetPathValue("workoutID", "1")
	rr := httptest.NewRecorder()
	h.DeleteReview(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestReviews_PendingReviews(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Kid", "")

	// Create some workouts.
	w1, _ := models.CreateWorkout(db, athlete.ID, "2026-02-10", "")
	models.CreateWorkout(db, athlete.ID, "2026-02-11", "")

	// Review one.
	models.CreateWorkoutReview(db, w1.ID, coach.ID, models.ReviewStatusApproved, "")

	h := &Reviews{DB: db, Templates: tc}

	req := requestWithUser("GET", "/reviews/pending", nil, coach)
	rr := httptest.NewRecorder()
	h.PendingReviews(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestReviews_PendingReviews_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	h := &Reviews{DB: db, Templates: tc}

	req := requestWithUser("GET", "/reviews/pending", nil, nonCoach)
	rr := httptest.NewRecorder()
	h.PendingReviews(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}
