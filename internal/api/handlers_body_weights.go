package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/models"
)

// --- Body Weights ---

// ListBodyWeights returns paginated body weight entries.
//
//	@Summary      List body weights
//	@Tags         Athletes
//	@Produce      json
//	@Param        id      path      int  true   "Athlete ID"
//	@Param        offset  query     int  false  "Pagination offset"
//	@Success      200  {object}  api.BodyWeightPage
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/body-weights [get]
func (h *Handlers) ListBodyWeights(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	_, offset := parsePage(r, 1, 1)

	page, err := models.ListBodyWeights(h.DB, athleteID, offset)
	if err != nil {
		log.Printf("api: list body weights for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list body weights")
		return
	}

	WriteJSON(w, http.StatusOK, BodyWeightPageFromModel(page))
}

// CreateBodyWeight creates a new body weight entry.
//
//	@Summary      Log body weight
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                    true  "Athlete ID"
//	@Param        body  body      api.BodyWeightRequest  true  "Entry"
//	@Success      201  {object}  api.BodyWeight
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/body-weights [post]
func (h *Handlers) CreateBodyWeight(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	var req BodyWeightRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Date == "" || req.Weight <= 0 {
		WriteError(w, http.StatusBadRequest, "date and weight are required")
		return
	}

	bw, err := models.CreateBodyWeight(h.DB, athleteID, req.Date, req.Weight, req.Notes)
	if err != nil {
		log.Printf("api: create body weight for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to create body weight")
		return
	}

	WriteJSON(w, http.StatusCreated, BodyWeightFromModel(bw))
}

// DeleteBodyWeight deletes a body weight entry.
//
//	@Summary      Delete body weight
//	@Tags         Athletes
//	@Produce      json
//	@Param        id    path      int  true  "Athlete ID"
//	@Param        bwID  path      int  true  "Body weight entry ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/body-weights/{bwID} [delete]
func (h *Handlers) DeleteBodyWeight(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	bwID, err := strconv.ParseInt(r.PathValue("bwID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid body weight ID")
		return
	}

	if err := models.DeleteBodyWeight(h.DB, bwID, athleteID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "body weight entry not found")
			return
		}
		log.Printf("api: delete body weight %d: %v", bwID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete body weight")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
