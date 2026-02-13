package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// Programs holds dependencies for program template handlers.
type Programs struct {
	DB        *sql.DB
	Templates TemplateCache
}

// List renders the program templates list page. Coach only.
func (h *Programs) List(w http.ResponseWriter, r *http.Request) {
	templates, err := models.ListProgramTemplates(h.DB)
	if err != nil {
		log.Printf("handlers: list program templates: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Programs": templates,
	}
	if err := h.Templates.Render(w, r, "programs_list.html", data); err != nil {
		log.Printf("handlers: programs list template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// NewForm renders the new program template form. Coach only.
func (h *Programs) NewForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	data := map[string]any{}
	if err := h.Templates.Render(w, r, "program_form.html", data); err != nil {
		log.Printf("handlers: program new form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Create processes the new program template form submission. Coach only.
func (h *Programs) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	description := r.FormValue("description")
	numWeeks, _ := strconv.Atoi(r.FormValue("num_weeks"))
	numDays, _ := strconv.Atoi(r.FormValue("num_days"))
	if numWeeks < 1 {
		numWeeks = 1
	}
	if numDays < 1 {
		numDays = 1
	}

	tmpl, err := models.CreateProgramTemplate(h.DB, name, description, numWeeks, numDays)
	if err != nil {
		log.Printf("handlers: create program template: %v", err)
		http.Error(w, "Failed to create program template", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/programs/%d", tmpl.ID), http.StatusSeeOther)
}

// Show renders the program template detail page with its prescribed sets. Coach only.
func (h *Programs) Show(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid program ID", http.StatusBadRequest)
		return
	}

	tmpl, err := models.GetProgramTemplateByID(h.DB, id)
	if err != nil {
		log.Printf("handlers: get program template %d: %v", id, err)
		http.Error(w, "Program template not found", http.StatusNotFound)
		return
	}

	sets, err := models.ListPrescribedSets(h.DB, id)
	if err != nil {
		log.Printf("handlers: list prescribed sets for template %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Parse selected week from query string (default: 1).
	currentWeek := 1
	if wStr := r.URL.Query().Get("week"); wStr != "" {
		if w, err := strconv.Atoi(wStr); err == nil && w >= 1 && w <= tmpl.NumWeeks {
			currentWeek = w
		}
	}

	// Organize sets by week and day; compute per-week set counts.
	type DaySets struct {
		Day     int
		Sets    []*models.PrescribedSet
		NextSet int // next set_number for add form
	}
	type WeekTab struct {
		Week     int
		SetCount int
		Active   bool
	}

	weekMap := make(map[int]map[int][]*models.PrescribedSet)
	weekCounts := make(map[int]int)
	for _, s := range sets {
		if weekMap[s.Week] == nil {
			weekMap[s.Week] = make(map[int][]*models.PrescribedSet)
		}
		weekMap[s.Week][s.Day] = append(weekMap[s.Week][s.Day], s)
		weekCounts[s.Week]++
	}

	// Build week tabs.
	weekTabs := make([]WeekTab, 0, tmpl.NumWeeks)
	for w := 1; w <= tmpl.NumWeeks; w++ {
		weekTabs = append(weekTabs, WeekTab{Week: w, SetCount: weekCounts[w], Active: w == currentWeek})
	}

	// Build days for the current week only.
	var days []DaySets
	for d := 1; d <= tmpl.NumDays; d++ {
		daySets := weekMap[currentWeek][d]
		nextSet := 1
		for _, s := range daySets {
			if s.SetNumber >= nextSet {
				nextSet = s.SetNumber + 1
			}
		}
		days = append(days, DaySets{Day: d, Sets: daySets, NextSet: nextSet})
	}

	exercises, err := models.ListExercises(h.DB, "")
	if err != nil {
		log.Printf("handlers: list exercises for program form: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Program":     tmpl,
		"WeekTabs":    weekTabs,
		"CurrentWeek": currentWeek,
		"Days":        days,
		"Exercises":   exercises,
	}
	if err := h.Templates.Render(w, r, "program_detail.html", data); err != nil {
		log.Printf("handlers: program detail template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// EditForm renders the edit form for a program template. Coach only.
func (h *Programs) EditForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid program ID", http.StatusBadRequest)
		return
	}

	tmpl, err := models.GetProgramTemplateByID(h.DB, id)
	if err != nil {
		log.Printf("handlers: get program template %d: %v", id, err)
		http.Error(w, "Program template not found", http.StatusNotFound)
		return
	}

	data := map[string]any{
		"Program": tmpl,
	}
	if err := h.Templates.Render(w, r, "program_form.html", data); err != nil {
		log.Printf("handlers: program edit form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Update processes the edit program template form submission. Coach only.
func (h *Programs) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid program ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	description := r.FormValue("description")
	numWeeks, _ := strconv.Atoi(r.FormValue("num_weeks"))
	numDays, _ := strconv.Atoi(r.FormValue("num_days"))
	if numWeeks < 1 {
		numWeeks = 1
	}
	if numDays < 1 {
		numDays = 1
	}

	_, err = models.UpdateProgramTemplate(h.DB, id, name, description, numWeeks, numDays)
	if err != nil {
		log.Printf("handlers: update program template %d: %v", id, err)
		http.Error(w, "Failed to update program template", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/programs/%d", id), http.StatusSeeOther)
}

// Delete processes the delete program template action. Coach only.
func (h *Programs) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid program ID", http.StatusBadRequest)
		return
	}

	err = models.DeleteProgramTemplate(h.DB, id)
	if errors.Is(err, models.ErrTemplateInUse) {
		http.Error(w, "Cannot delete: program is assigned to one or more athletes", http.StatusConflict)
		return
	}
	if err != nil {
		log.Printf("handlers: delete program template %d: %v", id, err)
		http.Error(w, "Failed to delete program template", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/programs", http.StatusSeeOther)
}

// AddSet adds a prescribed set to a program template. Coach only.
func (h *Programs) AddSet(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	templateID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid program ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	exerciseID, err := strconv.ParseInt(r.FormValue("exercise_id"), 10, 64)
	if err != nil {
		http.Error(w, "Exercise is required", http.StatusBadRequest)
		return
	}

	week, err := strconv.Atoi(r.FormValue("week"))
	if err != nil || week < 1 {
		http.Error(w, "Valid week is required", http.StatusBadRequest)
		return
	}

	day, err := strconv.Atoi(r.FormValue("day"))
	if err != nil || day < 1 {
		http.Error(w, "Valid day is required", http.StatusBadRequest)
		return
	}

	setNumber, err := strconv.Atoi(r.FormValue("set_number"))
	if err != nil || setNumber < 1 {
		http.Error(w, "Valid set number is required", http.StatusBadRequest)
		return
	}

	var reps *int
	if repsStr := r.FormValue("reps"); repsStr != "" {
		v, err := strconv.Atoi(repsStr)
		if err == nil {
			reps = &v
		}
	}

	var percentage *float64
	if pctStr := r.FormValue("percentage"); pctStr != "" {
		v, err := strconv.ParseFloat(pctStr, 64)
		if err == nil {
			percentage = &v
		}
	}

	notes := r.FormValue("notes")

	_, err = models.CreatePrescribedSet(h.DB, templateID, exerciseID, week, day, setNumber, reps, percentage, notes)
	if err != nil {
		log.Printf("handlers: add prescribed set to template %d: %v", templateID, err)
		http.Error(w, "Failed to add prescribed set", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/programs/%d?week=%d", templateID, week), http.StatusSeeOther)
}

// DeleteSet removes a prescribed set from a program template. Coach only.
func (h *Programs) DeleteSet(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	templateID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid program ID", http.StatusBadRequest)
		return
	}

	setID, err := strconv.ParseInt(r.PathValue("setID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid set ID", http.StatusBadRequest)
		return
	}

	if err := models.DeletePrescribedSet(h.DB, setID); err != nil {
		log.Printf("handlers: delete prescribed set %d: %v", setID, err)
		http.Error(w, "Failed to delete prescribed set", http.StatusInternalServerError)
		return
	}

	// Preserve the current week tab on redirect.
	weekParam := r.URL.Query().Get("week")
	if weekParam == "" {
		weekParam = r.FormValue("week")
	}
	redirectURL := fmt.Sprintf("/programs/%d", templateID)
	if weekParam != "" {
		redirectURL += "?week=" + weekParam
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// AssignProgram assigns a program template to an athlete. Coach only.
func (h *Programs) AssignProgram(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	templateID, err := strconv.ParseInt(r.FormValue("template_id"), 10, 64)
	if err != nil {
		http.Error(w, "Program template is required", http.StatusBadRequest)
		return
	}

	startDate := r.FormValue("start_date")
	if startDate == "" {
		startDate = time.Now().Format("2006-01-02")
	}
	notes := r.FormValue("notes")

	_, err = models.AssignProgram(h.DB, athleteID, templateID, startDate, notes)
	if errors.Is(err, models.ErrProgramAlreadyActive) {
		http.Error(w, "Athlete already has an active program. Deactivate it first.", http.StatusConflict)
		return
	}
	if err != nil {
		log.Printf("handlers: assign program to athlete %d: %v", athleteID, err)
		http.Error(w, "Failed to assign program", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/athletes/%d", athleteID), http.StatusSeeOther)
}

// DeactivateProgram deactivates an athlete's current program. Coach only.
func (h *Programs) DeactivateProgram(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	program, err := models.GetActiveProgram(h.DB, athleteID)
	if err != nil {
		log.Printf("handlers: get active program for athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if program == nil {
		http.Redirect(w, r, fmt.Sprintf("/athletes/%d", athleteID), http.StatusSeeOther)
		return
	}

	if err := models.DeactivateProgram(h.DB, program.ID); err != nil {
		log.Printf("handlers: deactivate program for athlete %d: %v", athleteID, err)
		http.Error(w, "Failed to deactivate program", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/athletes/%d", athleteID), http.StatusSeeOther)
}

// Prescription renders today's training prescription for an athlete.
func (h *Programs) Prescription(w http.ResponseWriter, r *http.Request) {
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if err != nil {
		log.Printf("handlers: get athlete %d for prescription: %v", athleteID, err)
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}

	prescription, err := models.GetPrescription(h.DB, athleteID, time.Now())
	if err != nil {
		log.Printf("handlers: get prescription for athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athlete":      athlete,
		"Prescription": prescription,
	}
	if err := h.Templates.Render(w, r, "prescription.html", data); err != nil {
		log.Printf("handlers: prescription template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// AssignProgramForm renders the form to assign a program to an athlete. Coach only.
func (h *Programs) AssignProgramForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if err != nil {
		log.Printf("handlers: get athlete %d: %v", athleteID, err)
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}

	templates, err := models.ListProgramTemplates(h.DB)
	if err != nil {
		log.Printf("handlers: list templates for assign form: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athlete":   athlete,
		"Programs":  templates,
		"TodayDate": time.Now().Format("2006-01-02"),
	}
	if err := h.Templates.Render(w, r, "assign_program_form.html", data); err != nil {
		log.Printf("handlers: assign program form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
