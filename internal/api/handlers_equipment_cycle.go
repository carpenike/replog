package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Equipment CRUD ---

// ListEquipment returns all equipment.
//
//	@Summary      List equipment
//	@Tags         Equipment
//	@Produce      json
//	@Success      200  {array}   api.Equipment
//	@Router       /equipment [get]
func (h *Handlers) ListEquipment(w http.ResponseWriter, r *http.Request) {
	equipment, err := models.ListEquipment(h.DB)
	if err != nil {
		log.Printf("api: list equipment: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to list equipment")
		return
	}

	result := make([]*Equipment, len(equipment))
	for i, e := range equipment {
		result[i] = EquipmentFromModel(e)
	}
	WriteJSON(w, http.StatusOK, result)
}

// CreateEquipment creates a new equipment item. Coach only.
//
//	@Summary      Create equipment
//	@Tags         Equipment
//	@Accept       json
//	@Produce      json
//	@Param        body  body      api.EquipmentRequest  true  "Equipment"
//	@Success      201  {object}  api.Equipment
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /equipment [post]
func (h *Handlers) CreateEquipment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	var req EquipmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	equip, err := models.CreateEquipment(h.DB, req.Name, req.Description)
	if err != nil {
		log.Printf("api: create equipment: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to create equipment")
		return
	}

	WriteJSON(w, http.StatusCreated, EquipmentFromModel(equip))
}

// DeleteEquipment deletes an equipment item. Coach only.
//
//	@Summary      Delete equipment
//	@Tags         Equipment
//	@Produce      json
//	@Param        equipmentID  path      int  true  "Equipment ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /equipment/{equipmentID} [delete]
func (h *Handlers) DeleteEquipment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("equipmentID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid equipment ID")
		return
	}

	if err := models.DeleteEquipment(h.DB, id); err != nil {
		log.Printf("api: delete equipment %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete equipment")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Exercise Equipment ---

// ListExerciseEquipment returns equipment linked to an exercise.
//
//	@Summary      List exercise's equipment requirements
//	@Tags         Equipment
//	@Produce      json
//	@Param        id   path      int  true  "Exercise ID"
//	@Success      200  {array}   api.ExerciseEquipment
//	@Failure      400  {object}  api.APIError
//	@Router       /exercises/{id}/equipment [get]
func (h *Handlers) ListExerciseEquipment(w http.ResponseWriter, r *http.Request) {
	exerciseID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid exercise ID")
		return
	}

	equipment, err := models.ListExerciseEquipment(h.DB, exerciseID)
	if err != nil {
		log.Printf("api: list exercise equipment %d: %v", exerciseID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list equipment")
		return
	}

	WriteJSON(w, http.StatusOK, equipment)
}

// AddExerciseEquipment links equipment to an exercise.
//
//	@Summary      Add equipment requirement to exercise
//	@Tags         Equipment
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                            true  "Exercise ID"
//	@Param        body  body      api.ExerciseEquipmentRequest   true  "Equipment link"
//	@Success      201  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /exercises/{id}/equipment [post]
func (h *Handlers) AddExerciseEquipment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	exerciseID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid exercise ID")
		return
	}

	var req ExerciseEquipmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := models.AddExerciseEquipment(h.DB, exerciseID, req.EquipmentID, req.Optional); err != nil {
		log.Printf("api: add exercise equipment: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to add equipment")
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// RemoveExerciseEquipment unlinks equipment from an exercise.
//
//	@Summary      Remove equipment requirement from exercise
//	@Tags         Equipment
//	@Produce      json
//	@Param        id           path      int  true  "Exercise ID"
//	@Param        equipmentID  path      int  true  "Equipment ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /exercises/{id}/equipment/{equipmentID} [delete]
func (h *Handlers) RemoveExerciseEquipment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	exerciseID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid exercise ID")
		return
	}

	equipmentID, err := strconv.ParseInt(r.PathValue("equipmentID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid equipment ID")
		return
	}

	if err := models.RemoveExerciseEquipment(h.DB, exerciseID, equipmentID); err != nil {
		log.Printf("api: remove exercise equipment: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to remove equipment")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Athlete Equipment ---

// ListAthleteEquipment returns equipment owned by an athlete.
//
//	@Summary      List athlete's equipment
//	@Tags         Equipment
//	@Produce      json
//	@Param        id   path      int  true  "Athlete ID"
//	@Success      200  {array}   api.Equipment
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/equipment [get]
func (h *Handlers) ListAthleteEquipment(w http.ResponseWriter, r *http.Request) {
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

	equipment, err := models.ListAthleteEquipment(h.DB, athleteID)
	if err != nil {
		log.Printf("api: list athlete equipment %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list equipment")
		return
	}

	WriteJSON(w, http.StatusOK, equipment)
}

// AddAthleteEquipment assigns equipment to an athlete.
//
//	@Summary      Assign equipment to athlete
//	@Tags         Equipment
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                          true  "Athlete ID"
//	@Param        body  body      api.AthleteEquipmentRequest  true  "Equipment"
//	@Success      201  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/equipment [post]
func (h *Handlers) AddAthleteEquipment(w http.ResponseWriter, r *http.Request) {
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

	var req AthleteEquipmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := models.AddAthleteEquipment(h.DB, athleteID, req.EquipmentID); err != nil {
		log.Printf("api: add athlete equipment: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to add equipment")
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// RemoveAthleteEquipment removes equipment from an athlete.
//
//	@Summary      Remove athlete's equipment
//	@Tags         Equipment
//	@Produce      json
//	@Param        id           path      int  true  "Athlete ID"
//	@Param        equipmentID  path      int  true  "Equipment ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/equipment/{equipmentID} [delete]
func (h *Handlers) RemoveAthleteEquipment(w http.ResponseWriter, r *http.Request) {
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

	equipmentID, err := strconv.ParseInt(r.PathValue("equipmentID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid equipment ID")
		return
	}

	if err := models.RemoveAthleteEquipment(h.DB, athleteID, equipmentID); err != nil {
		log.Printf("api: remove athlete equipment: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to remove equipment")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Cycle Review ---

// CycleSummaryResponse is the JSON response for cycle review data.
type CycleSummaryResponse struct {
	CycleNumber int                     `json:"cycle_number"`
	CycleStart  string                  `json:"cycle_start"`
	CycleEnd    string                  `json:"cycle_end"`
	Suggestions []TMSuggestionResponse  `json:"suggestions"`
}

// TMSuggestionResponse is a training max bump suggestion.
type TMSuggestionResponse struct {
	ExerciseID   int64   `json:"exercise_id"`
	ExerciseName string  `json:"exercise_name"`
	CurrentTM    float64 `json:"current_tm"`
	Increment    float64 `json:"increment"`
	SuggestedTM  float64 `json:"suggested_tm"`
}

// GetCycleReview returns cycle summary with TM bump suggestions. Coach only.
//
//	@Summary      Cycle review (TM bump suggestions)
//	@Tags         TrainingMaxes
//	@Produce      json
//	@Param        id   path      int  true  "Athlete ID"
//	@Success      200  {object}  api.CycleSummaryResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/cycle-review [get]
func (h *Handlers) GetCycleReview(w http.ResponseWriter, r *http.Request) {
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
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	program, err := models.GetActiveProgram(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "no active program")
		return
	}
	if err != nil {
		log.Printf("api: get active program for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to get program")
		return
	}

	summary, err := models.GetCycleSummary(h.DB, program, time.Now())
	if err != nil {
		log.Printf("api: cycle summary for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to get cycle summary")
		return
	}
	if summary == nil {
		// No completed cycle to review yet (program just started, mid-cycle 1,
		// or no logged workouts on this assignment). Return an empty summary
		// so the UI can render an explanatory empty state.
		WriteJSON(w, http.StatusOK, CycleSummaryResponse{Suggestions: []TMSuggestionResponse{}})
		return
	}

	suggestions := make([]TMSuggestionResponse, len(summary.Suggestions))
	for i, s := range summary.Suggestions {
		suggestions[i] = TMSuggestionResponse{
			ExerciseID:   s.ExerciseID,
			ExerciseName: s.ExerciseName,
			CurrentTM:    s.CurrentTM,
			Increment:    s.Increment,
			SuggestedTM:  s.SuggestedTM,
		}
	}

	WriteJSON(w, http.StatusOK, CycleSummaryResponse{
		CycleNumber: summary.CycleNumber,
		CycleStart:  summary.CycleStart,
		CycleEnd:    summary.CycleEnd,
		Suggestions: suggestions,
	})
}

// ApplyTMBumps applies selected training max bumps. Coach only.
//
//	@Summary      Apply TM bumps from cycle review
//	@Tags         TrainingMaxes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                 true  "Athlete ID"
//	@Param        body  body      api.TMBumpsRequest  true  "Bumps"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/cycle-review [post]
func (h *Handlers) ApplyTMBumps(w http.ResponseWriter, r *http.Request) {
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
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	var req struct {
		Bumps []struct {
			ExerciseID int64   `json:"exercise_id"`
			NewWeight  float64 `json:"new_weight"`
		} `json:"bumps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	applied := 0
	for _, bump := range req.Bumps {
		if bump.NewWeight > 0 {
			if _, err := models.SetTrainingMax(h.DB, athleteID, bump.ExerciseID, bump.NewWeight, time.Now().Format("2006-01-02"), "Cycle review bump"); err != nil {
				log.Printf("api: apply TM bump for athlete %d exercise %d: %v", athleteID, bump.ExerciseID, err)
			} else {
				applied++
			}
		}
	}

	WriteJSON(w, http.StatusOK, map[string]int{"applied": applied})
}
