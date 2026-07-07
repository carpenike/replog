package api

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// ExportAthleteJSON streams a complete, re-importable RepLog JSON snapshot of an
// athlete's data (ADR 006).
//
//	@Summary      Export athlete data as RepLog JSON
//	@Description  Full-fidelity per-athlete export: profile, equipment, exercises,
//	@Description  assignments, training maxes, body weights, notes, workouts (with
//	@Description  sets and reviews), programs, and multimodal sessions.
//	@Tags         Export
//	@Produce      json
//	@Param        id   path      int  true  "Athlete ID"
//	@Success      200  {object}  models.AthleteExportJSON
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/export/json [get]
func (h *Handlers) ExportAthleteJSON(w http.ResponseWriter, r *http.Request) {
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

	export, err := models.BuildAthleteExportJSON(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "athlete not found")
		return
	}
	if err != nil {
		log.Printf("api: build athlete export %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to build export")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="athlete-%d-export.json"`, athleteID))
	w.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(export); err != nil {
		// Header already written; can only log at this point.
		log.Printf("api: encode athlete export %d: %v", athleteID, err)
	}
}

// ExportAthleteCSV streams the athlete's logged resistance sets as a CSV, the
// most tabular slice of the data (ADR 006).
//
//	@Summary      Export athlete workout sets as CSV
//	@Description  One row per logged resistance set: date, exercise, set number,
//	@Description  reps, weight, rpe, rep_type, category, notes.
//	@Tags         Export
//	@Produce      text/csv
//	@Param        id   path      int  true  "Athlete ID"
//	@Success      200  {string}  string  "CSV file"
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/export/csv [get]
func (h *Handlers) ExportAthleteCSV(w http.ResponseWriter, r *http.Request) {
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

	export, err := models.BuildAthleteExportJSON(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "athlete not found")
		return
	}
	if err != nil {
		log.Printf("api: build athlete export (csv) %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to build export")
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="athlete-%d-export.csv"`, athleteID))
	w.WriteHeader(http.StatusOK)

	cw := csv.NewWriter(w)
	defer cw.Flush()

	_ = cw.Write([]string{"date", "exercise", "set_number", "reps", "weight", "rpe", "rep_type", "category", "notes"})

	for _, workout := range export.Workouts {
		// CSV carries the tabular resistance log only; multimodal sessions and
		// other disciplines are omitted (they are not per-set data).
		if workout.Discipline != "" && workout.Discipline != "resistance" {
			continue
		}
		for _, s := range workout.Sets {
			row := []string{
				workout.Date,
				s.Exercise,
				strconv.Itoa(s.SetNumber),
				strconv.Itoa(s.Reps),
				floatOrEmpty(s.Weight),
				floatOrEmpty(s.RPE),
				s.RepType,
				s.Category,
				stringOrEmpty(s.Notes),
			}
			if err := cw.Write(row); err != nil {
				log.Printf("api: write csv row for athlete %d: %v", athleteID, err)
				return
			}
		}
	}
}

// floatOrEmpty formats a *float64 as a compact string, or "" when nil.
func floatOrEmpty(f *float64) string {
	if f == nil {
		return ""
	}
	return strconv.FormatFloat(*f, 'f', -1, 64)
}

// stringOrEmpty dereferences a *string, or returns "" when nil.
func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
