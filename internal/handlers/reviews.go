package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// Reviews holds dependencies for workout review handlers.
type Reviews struct {
	DB        *sql.DB
	Templates TemplateCache
}

// SubmitReview creates or updates a coach review for a workout.
func (h *Reviews) SubmitReview(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid workout ID", http.StatusBadRequest)
		return
	}

	// Verify the workout belongs to the specified athlete.
	workout, err := models.GetWorkoutByID(h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get workout %d for review: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if workout.AthleteID != athleteID {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	status := r.FormValue("status")
	if status != models.ReviewStatusApproved && status != models.ReviewStatusNeedsWork {
		http.Error(w, "Invalid review status", http.StatusBadRequest)
		return
	}

	notes := r.FormValue("notes")

	_, err = models.CreateOrUpdateWorkoutReview(h.DB, workoutID, user.ID, status, notes)
	if err != nil {
		log.Printf("handlers: submit review for workout %d: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/workouts/"+strconv.FormatInt(workoutID, 10), http.StatusSeeOther)
}

// DeleteReview removes a review from a workout. Coach/admin only.
func (h *Reviews) DeleteReview(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid workout ID", http.StatusBadRequest)
		return
	}

	// Verify the workout belongs to the specified athlete.
	workout, err := models.GetWorkoutByID(h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get workout %d for review delete: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if workout.AthleteID != athleteID {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}

	review, err := models.GetWorkoutReviewByWorkoutID(h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Review not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get review for workout %d: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := models.DeleteWorkoutReview(h.DB, review.ID); err != nil {
		log.Printf("handlers: delete review %d: %v", review.ID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/workouts/"+strconv.FormatInt(workoutID, 10), http.StatusSeeOther)
}

// PendingReviews renders the coach review dashboard showing all unreviewed workouts.
func (h *Reviews) PendingReviews(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	unreviewed, err := models.ListUnreviewedWorkouts(h.DB)
	if err != nil {
		log.Printf("handlers: list unreviewed workouts: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	stats, err := models.GetReviewStats(h.DB)
	if err != nil {
		log.Printf("handlers: get review stats: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Unreviewed": unreviewed,
		"Stats":      stats,
	}
	if err := h.Templates.Render(w, r, "pending_reviews.html", data); err != nil {
		log.Printf("handlers: pending reviews template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
