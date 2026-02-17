package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"database/sql"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// BodyWeights holds dependencies for body weight handlers.
type BodyWeights struct {
	DB        *sql.DB
	Templates TemplateCache
}

// List renders the body weight history for an athlete.
func (h *BodyWeights) List(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for body weights: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !athlete.TrackBodyWeight {
		http.Error(w, "Body weight tracking is disabled for this athlete", http.StatusNotFound)
		return
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	page, err := models.ListBodyWeights(h.DB, athleteID, offset)
	if err != nil {
		log.Printf("handlers: list body weights for athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Load chart data for body weight trend.
	prefs := middleware.PrefsFromContext(r.Context())
	unit := models.DefaultWeightUnit
	if prefs != nil {
		unit = prefs.WeightUnit
	}
	chartData, chartErr := models.BodyWeightChartData(h.DB, athleteID, 30, unit)
	if chartErr != nil {
		log.Printf("handlers: body weight chart for athlete %d: %v", athleteID, chartErr)
	}

	data := map[string]any{
		"Athlete":    athlete,
		"Entries":    page.Entries,
		"HasMore":    page.HasMore,
		"NextOffset": offset + models.BodyWeightPageSize,
		"Today":      time.Now().Format("2006-01-02"),
		"Chart":      chartData,
	}
	if err := h.Templates.Render(w, r, "body_weights.html", data); err != nil {
		log.Printf("handlers: body weights template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Create processes the body weight form submission.
func (h *BodyWeights) Create(w http.ResponseWriter, r *http.Request) {
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

	// Check if body weight tracking is enabled for this athlete.
	athleteForCheck, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for body weight create: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !athleteForCheck.TrackBodyWeight {
		http.Error(w, "Body weight tracking is disabled for this athlete", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	date := r.FormValue("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	weight, err := strconv.ParseFloat(r.FormValue("weight"), 64)
	if err != nil || weight <= 0 {
		athlete, _ := models.GetAthleteByID(h.DB, athleteID)
		data := map[string]any{
			"Athlete": athlete,
			"Error":   "Weight must be a positive number",
			"Today":   time.Now().Format("2006-01-02"),
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.Templates.Render(w, r, "body_weights.html", data)
		return
	}

	notes := r.FormValue("notes")

	_, err = models.CreateBodyWeight(h.DB, athleteID, date, weight, notes)
	if errors.Is(err, models.ErrDuplicateBodyWeight) {
		athlete, _ := models.GetAthleteByID(h.DB, athleteID)
		page, _ := models.ListBodyWeights(h.DB, athleteID, 0)
		var entries []*models.BodyWeight
		if page != nil {
			entries = page.Entries
		}
		data := map[string]any{
			"Athlete": athlete,
			"Entries": entries,
			"Error":   "A body weight entry already exists for " + date,
			"Today":   time.Now().Format("2006-01-02"),
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.Templates.Render(w, r, "body_weights.html", data)
		return
	}
	if err != nil {
		log.Printf("handlers: create body weight for athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/body-weights", http.StatusSeeOther)
}

// Delete removes a body weight entry.
func (h *BodyWeights) Delete(w http.ResponseWriter, r *http.Request) {
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

	// Check if body weight tracking is enabled for this athlete.
	athleteForCheck, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for body weight delete: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !athleteForCheck.TrackBodyWeight {
		http.Error(w, "Body weight tracking is disabled for this athlete", http.StatusNotFound)
		return
	}

	bwID, err := strconv.ParseInt(r.PathValue("bwID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid body weight ID", http.StatusBadRequest)
		return
	}

	// Verify the entry belongs to the specified athlete.
	entry, err := models.GetBodyWeightByID(h.DB, bwID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get body weight %d: %v", bwID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if entry.AthleteID != athleteID {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}

	if err := models.DeleteBodyWeight(h.DB, bwID); err != nil {
		log.Printf("handlers: delete body weight %d: %v", bwID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/body-weights", http.StatusSeeOther)
}
