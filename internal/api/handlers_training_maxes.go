package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/models"
)

// --- Training Maxes ---

// ListTrainingMaxes returns current training maxes for an athlete.
//
//	@Summary      List current training maxes
//	@Tags         TrainingMaxes
//	@Produce      json
//	@Param        id   path      int  true  "Athlete ID"
//	@Success      200  {array}   api.TrainingMax
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/training-maxes [get]
func (h *Handlers) ListTrainingMaxes(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	tms, err := models.ListCurrentTrainingMaxes(r.Context(), h.DB, athleteID)
	if err != nil {
		log.Printf("api: list training maxes for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list training maxes")
		return
	}

	result := make([]*TrainingMax, len(tms))
	for i, tm := range tms {
		result[i] = TrainingMaxFromModel(tm)
	}
	WriteJSON(w, http.StatusOK, result)
}

// GetTrainingMaxHistory returns TM history for an athlete+exercise.
//
//	@Summary      Get training max history
//	@Tags         TrainingMaxes
//	@Produce      json
//	@Param        id          path      int  true  "Athlete ID"
//	@Param        exerciseID  path      int  true  "Exercise ID"
//	@Success      200  {array}   api.TrainingMax
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/exercises/{exerciseID}/training-maxes [get]
func (h *Handlers) GetTrainingMaxHistory(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	exerciseID, err := strconv.ParseInt(r.PathValue("exerciseID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid exercise ID")
		return
	}

	history, err := models.ListTrainingMaxHistory(r.Context(), h.DB, athleteID, exerciseID)
	if err != nil {
		log.Printf("api: training max history for athlete %d exercise %d: %v", athleteID, exerciseID, err)
		WriteError(w, http.StatusInternalServerError, "failed to get training max history")
		return
	}

	result := make([]*TrainingMax, len(history))
	for i, tm := range history {
		result[i] = TrainingMaxFromModel(tm)
	}
	WriteJSON(w, http.StatusOK, result)
}
