package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/carpenike/replog/internal/models"
)

// PlateWeight is a weight with its own unit (kg or lbs).
type PlateWeight struct {
	Weight float64 `json:"weight" example:"20"`
	Unit   string  `json:"unit" example:"kg"`
}

// PlatePairResponse is one plate size in an inventory. pairs is how many
// matched pairs the gym has (one plate per sleeve per pair).
type PlatePairResponse struct {
	Weight float64 `json:"weight" example:"25"`
	Unit   string  `json:"unit" example:"kg"`
	Pairs  int     `json:"pairs" example:"4"`
}

// PlateSetupResponse is an athlete's barbell loading setup. Bar, collar,
// and plates each carry their own unit, separate from the logging unit in
// preferences. collar is the weight of one collar. is_default is true when
// nothing is saved and the default kg competition setup is shown.
type PlateSetupResponse struct {
	AthleteID  int64               `json:"athlete_id"`
	Bar        PlateWeight         `json:"bar"`
	Collar     PlateWeight         `json:"collar"`
	UseCollars bool                `json:"use_collars"`
	Plates     []PlatePairResponse `json:"plates"`
	IsDefault  bool                `json:"is_default"`
	UpdatedAt  *string             `json:"updated_at,omitempty"`
}

// PlateSetupRequest is the body for PUT /api/athletes/{id}/plate-setup.
// It replaces the whole setup, including the plate inventory.
type PlateSetupRequest struct {
	Bar        PlateWeight         `json:"bar"`
	Collar     *PlateWeight        `json:"collar,omitempty"`
	UseCollars bool                `json:"use_collars,omitempty"`
	Plates     []PlatePairResponse `json:"plates"`
}

// PlateSetupPresetResponse is a named starting setup.
type PlateSetupPresetResponse struct {
	Key   string             `json:"key" example:"kg_competition"`
	Name  string             `json:"name"`
	Setup PlateSetupResponse `json:"setup"`
}

// PlateSetupFromModel converts a models.PlateSetup to its API shape.
func PlateSetupFromModel(m *models.PlateSetup) *PlateSetupResponse {
	out := &PlateSetupResponse{
		AthleteID:  m.AthleteID,
		Bar:        PlateWeight{Weight: m.BarWeight, Unit: m.BarUnit},
		Collar:     PlateWeight{Weight: m.CollarWeight, Unit: m.CollarUnit},
		UseCollars: m.UseCollars,
		Plates:     make([]PlatePairResponse, len(m.Plates)),
		IsDefault:  m.IsDefault,
	}
	for i, p := range m.Plates {
		out.Plates[i] = PlatePairResponse{Weight: p.Weight, Unit: p.Unit, Pairs: p.Pairs}
	}
	if m.UpdatedAt != nil {
		ts := fmtTime(*m.UpdatedAt)
		out.UpdatedAt = &ts
	}
	return out
}

// GetPlateSetup returns an athlete's barbell loading setup.
//
//	@Summary      Get plate setup
//	@Description  Bar, collars, and plate inventory for the athlete's gym. Each part has its own unit. When nothing is saved, the default kg competition setup is returned with is_default true.
//	@Tags         Equipment
//	@Produce      json
//	@Param        id   path      int  true  "Athlete ID"
//	@Success      200  {object}  api.PlateSetupResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/plate-setup [get]
func (h *Handlers) GetPlateSetup(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	setup, err := models.GetPlateSetup(r.Context(), h.DB, athleteID)
	if err != nil {
		log.Printf("api: get plate setup for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to get plate setup")
		return
	}
	WriteJSON(w, http.StatusOK, PlateSetupFromModel(setup))
}

// UpdatePlateSetup replaces an athlete's barbell loading setup.
//
//	@Summary      Save plate setup
//	@Description  Replaces the bar, collars, and whole plate inventory. A coach can set it for an athlete they coach; the athlete can set their own.
//	@Tags         Equipment
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                    true  "Athlete ID"
//	@Param        body  body      api.PlateSetupRequest  true  "Setup"
//	@Success      200  {object}  api.PlateSetupResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/plate-setup [put]
func (h *Handlers) UpdatePlateSetup(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	var req PlateSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	setup := &models.PlateSetup{
		AthleteID:  athleteID,
		BarWeight:  req.Bar.Weight,
		BarUnit:    req.Bar.Unit,
		CollarUnit: req.Bar.Unit,
		UseCollars: req.UseCollars,
		Plates:     make([]models.PlatePair, len(req.Plates)),
	}
	if req.Collar != nil {
		setup.CollarWeight = req.Collar.Weight
		setup.CollarUnit = req.Collar.Unit
	}
	for i, p := range req.Plates {
		setup.Plates[i] = models.PlatePair{Weight: p.Weight, Unit: p.Unit, Pairs: p.Pairs}
	}
	if field, msg := models.ValidatePlateSetup(setup); field != "" {
		WriteValidationError(w, field, msg)
		return
	}
	saved, err := models.SavePlateSetup(r.Context(), h.DB, setup)
	if err != nil {
		log.Printf("api: save plate setup for athlete %d: %v", athleteID, err)
		WriteDBError(w, err, "failed to save plate setup")
		return
	}
	WriteJSON(w, http.StatusOK, PlateSetupFromModel(saved))
}

// ResetPlateSetup deletes an athlete's saved setup and returns the default.
//
//	@Summary      Reset plate setup
//	@Description  Removes the saved setup. Reads then return the default kg competition setup.
//	@Tags         Equipment
//	@Produce      json
//	@Param        id   path      int  true  "Athlete ID"
//	@Success      200  {object}  api.PlateSetupResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/plate-setup [delete]
func (h *Handlers) ResetPlateSetup(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	if err := models.DeletePlateSetup(r.Context(), h.DB, athleteID); err != nil {
		log.Printf("api: reset plate setup for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to reset plate setup")
		return
	}
	WriteJSON(w, http.StatusOK, PlateSetupFromModel(models.DefaultPlateSetup(athleteID)))
}

// ListPlateSetupPresets returns the starting setups a coach can pick.
//
//	@Summary      List plate setup presets
//	@Description  The kg competition setup (the default) and a standard lb setup.
//	@Tags         Equipment
//	@Produce      json
//	@Success      200  {array}   api.PlateSetupPresetResponse
//	@Router       /plate-setup/presets [get]
func (h *Handlers) ListPlateSetupPresets(w http.ResponseWriter, r *http.Request) {
	presets := models.PlateSetupPresets()
	out := make([]PlateSetupPresetResponse, len(presets))
	for i, p := range presets {
		setup := p.Setup
		out[i] = PlateSetupPresetResponse{Key: p.Key, Name: p.Name, Setup: *PlateSetupFromModel(&setup)}
	}
	WriteJSON(w, http.StatusOK, out)
}
