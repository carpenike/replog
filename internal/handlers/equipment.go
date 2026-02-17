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

// Equipment holds dependencies for equipment handlers.
type Equipment struct {
	DB        *sql.DB
	Templates TemplateCache
}

// List renders the equipment catalog page.
func (h *Equipment) List(w http.ResponseWriter, r *http.Request) {
	items, err := models.ListEquipment(h.DB)
	if err != nil {
		log.Printf("handlers: list equipment: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Equipment": items,
	}
	if err := h.Templates.Render(w, r, "equipment_list.html", data); err != nil {
		log.Printf("handlers: equipment list template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// NewForm renders the new equipment form. Coach/admin only.
func (h *Equipment) NewForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	if err := h.Templates.Render(w, r, "equipment_form.html", nil); err != nil {
		log.Printf("handlers: equipment new form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Create processes the new equipment form submission. Coach/admin only.
func (h *Equipment) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		data := map[string]any{
			"Error": "Name is required",
			"Form":  r.Form,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.Templates.Render(w, r, "equipment_form.html", data)
		return
	}

	_, err := models.CreateEquipment(h.DB, name, r.FormValue("description"))
	if errors.Is(err, models.ErrDuplicateEquipmentName) {
		data := map[string]any{
			"Error": "An equipment item with that name already exists",
			"Form":  r.Form,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.Templates.Render(w, r, "equipment_form.html", data)
		return
	}
	if err != nil {
		log.Printf("handlers: create equipment: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/equipment", http.StatusSeeOther)
}

// EditForm renders the edit equipment form. Coach/admin only.
func (h *Equipment) EditForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid equipment ID", http.StatusBadRequest)
		return
	}

	item, err := models.GetEquipmentByID(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		h.Templates.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("handlers: get equipment %d for edit: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Equipment": item,
	}
	if err := h.Templates.Render(w, r, "equipment_form.html", data); err != nil {
		log.Printf("handlers: equipment edit form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Update processes the edit equipment form submission. Coach/admin only.
func (h *Equipment) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid equipment ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		item, _ := models.GetEquipmentByID(h.DB, id)
		data := map[string]any{
			"Error":     "Name is required",
			"Equipment": item,
			"Form":      r.Form,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.Templates.Render(w, r, "equipment_form.html", data)
		return
	}

	_, err = models.UpdateEquipment(h.DB, id, name, r.FormValue("description"))
	if errors.Is(err, models.ErrDuplicateEquipmentName) {
		item, _ := models.GetEquipmentByID(h.DB, id)
		data := map[string]any{
			"Error":     "An equipment item with that name already exists",
			"Equipment": item,
			"Form":      r.Form,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.Templates.Render(w, r, "equipment_form.html", data)
		return
	}
	if errors.Is(err, models.ErrNotFound) {
		h.Templates.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("handlers: update equipment %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/equipment", http.StatusSeeOther)
}

// Delete removes an equipment item. Coach/admin only.
func (h *Equipment) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid equipment ID", http.StatusBadRequest)
		return
	}

	err = models.DeleteEquipment(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		h.Templates.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("handlers: delete equipment %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/equipment", http.StatusSeeOther)
}

// AddExerciseEquipment links equipment to an exercise. Coach/admin only.
func (h *Equipment) AddExerciseEquipment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	exerciseID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid exercise ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	equipmentID, err := strconv.ParseInt(r.FormValue("equipment_id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid equipment ID", http.StatusBadRequest)
		return
	}

	optional := r.FormValue("optional") == "1"

	if err := models.AddExerciseEquipment(h.DB, exerciseID, equipmentID, optional); err != nil {
		log.Printf("handlers: add exercise equipment: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/exercises/"+strconv.FormatInt(exerciseID, 10), http.StatusSeeOther)
}

// RemoveExerciseEquipment unlinks equipment from an exercise. Coach/admin only.
func (h *Equipment) RemoveExerciseEquipment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	exerciseID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid exercise ID", http.StatusBadRequest)
		return
	}

	equipmentID, err := strconv.ParseInt(r.PathValue("equipmentID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid equipment ID", http.StatusBadRequest)
		return
	}

	if err := models.RemoveExerciseEquipment(h.DB, exerciseID, equipmentID); err != nil {
		log.Printf("handlers: remove exercise equipment: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/exercises/"+strconv.FormatInt(exerciseID, 10), http.StatusSeeOther)
}

// AddAthleteEquipment adds an equipment item to an athlete's inventory.
func (h *Equipment) AddAthleteEquipment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		h.Templates.Forbidden(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	equipmentID, err := strconv.ParseInt(r.FormValue("equipment_id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid equipment ID", http.StatusBadRequest)
		return
	}

	if err := models.AddAthleteEquipment(h.DB, athleteID, equipmentID); err != nil {
		log.Printf("handlers: add athlete equipment: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/equipment", http.StatusSeeOther)
}

// RemoveAthleteEquipment removes an equipment item from an athlete's inventory.
func (h *Equipment) RemoveAthleteEquipment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		h.Templates.Forbidden(w, r)
		return
	}

	equipmentID, err := strconv.ParseInt(r.PathValue("equipmentID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid equipment ID", http.StatusBadRequest)
		return
	}

	if err := models.RemoveAthleteEquipment(h.DB, athleteID, equipmentID); err != nil {
		log.Printf("handlers: remove athlete equipment: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/equipment", http.StatusSeeOther)
}

// AthleteEquipmentPage renders the equipment inventory page for an athlete.
func (h *Equipment) AthleteEquipmentPage(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		h.Templates.Forbidden(w, r)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		h.Templates.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for equipment: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get athlete's current equipment.
	athleteEquip, err := models.ListAthleteEquipment(h.DB, athleteID)
	if err != nil {
		log.Printf("handlers: list athlete equipment: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get all equipment for the add form.
	allEquipment, err := models.ListEquipment(h.DB)
	if err != nil {
		log.Printf("handlers: list all equipment: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build set of already-owned equipment IDs for filtering the dropdown.
	owned := make(map[int64]bool)
	for _, ae := range athleteEquip {
		owned[ae.EquipmentID] = true
	}

	// Filter to show only unowned equipment in the add dropdown.
	var available []*models.Equipment
	for _, eq := range allEquipment {
		if !owned[eq.ID] {
			available = append(available, eq)
		}
	}

	// Check compatibility for assigned exercises.
	compatibility, err := models.CheckAthleteExerciseCompatibility(h.DB, athleteID)
	if err != nil {
		log.Printf("handlers: check equipment compatibility: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athlete":           athlete,
		"AthleteEquipment":  athleteEquip,
		"AvailableEquipment": available,
		"Compatibility":     compatibility,
	}
	if err := h.Templates.Render(w, r, "athlete_equipment.html", data); err != nil {
		log.Printf("handlers: athlete equipment template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
