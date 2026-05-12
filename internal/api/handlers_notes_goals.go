package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Athlete Notes ---

// CreateAthleteNote creates a journal note for an athlete.
// CreateAthleteNote adds a journal note to an athlete.
//
//	@Summary      Create athlete journal note
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                     true  "Athlete ID"
//	@Param        body  body      api.AthleteNoteRequest  true  "Note"
//	@Success      201  {object}  api.AthleteNote
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/notes [post]
func (h *Handlers) CreateAthleteNote(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	var req AthleteNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Content == "" {
		WriteError(w, http.StatusBadRequest, "content is required")
		return
	}

	date := time.Now().Format("2006-01-02")
	note, err := models.CreateAthleteNote(h.DB, athleteID, user.ID, date, req.Content, req.IsPrivate, req.Pinned)
	if err != nil {
		log.Printf("api: create athlete note for %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to create note")
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]any{
		"id":         note.ID,
		"athlete_id": note.AthleteID,
		"content":    note.Content,
		"is_private": note.IsPrivate,
		"pinned":     note.Pinned,
		"created_at": fmtTime(note.CreatedAt),
	})
}

// --- Athlete Goal ---

// UpdateAthleteGoal updates an athlete's goal text.
// UpdateAthleteGoal updates an athlete's goal text.
//
//	@Summary      Update athlete goal
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int              true  "Athlete ID"
//	@Param        body  body      api.GoalRequest  true  "Goal"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/goal [put]
func (h *Handlers) UpdateAthleteGoal(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	var req GoalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := models.UpdateAthleteGoal(h.DB, athleteID, req.Goal); err != nil {
		log.Printf("api: update athlete %d goal: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to update goal")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Exercise History ---

// ListExerciseHistory returns paginated exercise history for an athlete.
//
//	@Summary      Exercise history for athlete
//	@Tags         Exercises
//	@Produce      json
//	@Param        id          path      int  true   "Athlete ID"
//	@Param        exerciseID  path      int  true   "Exercise ID"
//	@Param        offset      query     int  false  "Pagination offset"
//	@Success      200  {object}  api.ExerciseHistoryPage
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/exercises/{exerciseID}/history [get]
func (h *Handlers) ListExerciseHistory(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	exerciseID, err := strconv.ParseInt(r.PathValue("exerciseID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid exercise ID")
		return
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
	}

	page, err := models.ListExerciseHistory(h.DB, athleteID, exerciseID, offset)
	if err != nil {
		log.Printf("api: exercise history athlete %d exercise %d: %v", athleteID, exerciseID, err)
		WriteError(w, http.StatusInternalServerError, "failed to get exercise history")
		return
	}

	days := make([]*ExerciseHistoryDay, len(page.Days))
	for i, d := range page.Days {
		sets := make([]*ExerciseHistoryEntry, len(d.Sets))
		for j, s := range d.Sets {
			sets[j] = &ExerciseHistoryEntry{
				WorkoutID:   s.WorkoutID,
				WorkoutDate: s.WorkoutDate,
				SetNumber:   s.SetNumber,
				Reps:        s.Reps,
				Weight:      nullFloat(s.Weight),
				RPE:         nullFloat(s.RPE),
				Notes:       nullStr(s.Notes),
			}
		}
		days[i] = &ExerciseHistoryDay{
			WorkoutID:   d.WorkoutID,
			WorkoutDate: d.WorkoutDate,
			Sets:        sets,
		}
	}

	WriteJSON(w, http.StatusOK, ExerciseHistoryPage{
		Days:    days,
		HasMore: page.HasMore,
	})
}
