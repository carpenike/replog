package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
	"github.com/carpenike/replog/internal/notify"
)

// ReactivateAssignment reactivates a deactivated exercise assignment.
//
//	@Summary      Reactivate exercise assignment
//	@Description  Creates a fresh active row for the athlete+exercise pair (the previous one stays as history).
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                        true  "Athlete ID"
//	@Param        body  body      api.AssignExerciseRequest  true  "Assignment"
//	@Success      200  {object}  api.AthleteExercise
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/assignments/reactivate [post]
func (h *Handlers) ReactivateAssignment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "not your athlete")
		return
	}

	var req AssignExerciseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ExerciseID == 0 {
		WriteError(w, http.StatusBadRequest, "exercise_id is required")
		return
	}

	assignment, err := models.ReactivateAssignment(h.DB, athleteID, req.ExerciseID, req.TargetReps)
	if err != nil {
		log.Printf("api: reactivate assignment athlete %d exercise %d: %v", athleteID, req.ExerciseID, err)
		WriteError(w, http.StatusInternalServerError, "failed to reactivate assignment")
		return
	}

	WriteJSON(w, http.StatusOK, AthleteExerciseFromModel(assignment))
}

// DeleteReview deletes a workout review.
//
//	@Summary      Delete workout review
//	@Tags         Reviews
//	@Produce      json
//	@Param        id         path      int  true  "Athlete ID"
//	@Param        workoutID  path      int  true  "Workout ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID}/review [delete]
func (h *Handlers) DeleteReview(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid workout ID")
		return
	}

	review, err := models.GetWorkoutReviewByWorkoutID(h.DB, workoutID)
	if err != nil {
		log.Printf("api: get review for workout %d: %v", workoutID, err)
		WriteError(w, http.StatusNotFound, "review not found")
		return
	}

	if err := models.DeleteWorkoutReview(h.DB, review.ID); err != nil {
		log.Printf("api: delete review %d: %v", review.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete review")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// testNotifyTimeout caps each individual SMTP/webhook test send. Same
// rationale as testLLMPingTimeout (handlers_remaining.go): shoutrrr's
// default socket timeouts can exceed the HTTP server's WriteTimeout, which
// would silently turn into Caddy 502s with replog logging 200. Exposed as
// a var so tests can shrink it.
var testNotifyTimeout = 30 * time.Second

// TestNotifyConnection tests the notification provider connection.
//
//	@Summary      Test notification provider connection (admin)
//	@Description  Always returns 200; check `success` field. Errors are surfaced via `error` field for the SPA to display inline.
//	@Tags         Admin
//	@Produce      json
//	@Success      200  {object}  map[string]interface{}
//	@Failure      403  {object}  api.APIError
//	@Router       /admin/settings/test-notify [post]
func (h *Handlers) TestNotifyConnection(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), testNotifyTimeout)
	defer cancel()

	if err := notify.TestConnection(ctx, h.DB); err != nil {
		msg := err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			msg = "Notification provider did not respond in time. Check SMTP host and broadcast URLs."
		}
		WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": msg})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}
