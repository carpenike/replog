package handlers

import (
	"database/sql"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/models"
)

// Athletes holds dependencies for athlete handlers.
type Athletes struct {
	DB        *sql.DB
	Templates *template.Template
}

// List renders the athlete list page.
func (h *Athletes) List(w http.ResponseWriter, r *http.Request) {
	athletes, err := models.ListAthletes(h.DB)
	if err != nil {
		log.Printf("handlers: list athletes: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athletes": athletes,
	}
	if err := h.Templates.ExecuteTemplate(w, "athletes-list", data); err != nil {
		log.Printf("handlers: athletes list template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// NewForm renders the new athlete form.
func (h *Athletes) NewForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Tiers": tierOptions(),
	}
	if err := h.Templates.ExecuteTemplate(w, "athlete-form", data); err != nil {
		log.Printf("handlers: athlete new form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Create processes the new athlete form submission.
func (h *Athletes) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		data := map[string]any{
			"Error": "Name is required",
			"Tiers": tierOptions(),
			"Form":  r.Form,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.Templates.ExecuteTemplate(w, "athlete-form", data)
		return
	}

	athlete, err := models.CreateAthlete(h.DB, name, r.FormValue("tier"), r.FormValue("notes"))
	if err != nil {
		log.Printf("handlers: create athlete: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athlete.ID, 10), http.StatusSeeOther)
}

// Show renders the athlete detail page.
func (h *Athletes) Show(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athlete": athlete,
	}
	if err := h.Templates.ExecuteTemplate(w, "athlete-detail", data); err != nil {
		log.Printf("handlers: athlete detail template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// EditForm renders the edit athlete form.
func (h *Athletes) EditForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for edit: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athlete": athlete,
		"Tiers":   tierOptions(),
	}
	if err := h.Templates.ExecuteTemplate(w, "athlete-form", data); err != nil {
		log.Printf("handlers: athlete edit form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Update processes the edit athlete form submission.
func (h *Athletes) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		athlete, _ := models.GetAthleteByID(h.DB, id)
		data := map[string]any{
			"Error":   "Name is required",
			"Athlete": athlete,
			"Tiers":   tierOptions(),
			"Form":    r.Form,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.Templates.ExecuteTemplate(w, "athlete-form", data)
		return
	}

	_, err = models.UpdateAthlete(h.DB, id, name, r.FormValue("tier"), r.FormValue("notes"))
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: update athlete %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// Delete removes an athlete.
func (h *Athletes) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	err = models.DeleteAthlete(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: delete athlete %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes", http.StatusSeeOther)
}

func tierOptions() []struct{ Value, Label string } {
	return []struct{ Value, Label string }{
		{"", "— None —"},
		{"foundational", "Foundational"},
		{"intermediate", "Intermediate"},
		{"sport_performance", "Sport Performance"},
	}
}
