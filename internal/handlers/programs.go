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
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
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

	isLoop := r.FormValue("is_loop") == "1"

	tmpl, err := models.CreateProgramTemplate(h.DB, name, description, numWeeks, numDays, isLoop)
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

	// Load progression rules for this template.
	progressionRules, err := models.ListProgressionRules(h.DB, id)
	if err != nil {
		log.Printf("handlers: list progression rules for template %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Program":          tmpl,
		"WeekTabs":         weekTabs,
		"CurrentWeek":      currentWeek,
		"Days":             days,
		"Exercises":        exercises,
		"ProgressionRules": progressionRules,
	}
	if err := h.Templates.Render(w, r, "program_detail.html", data); err != nil {
		log.Printf("handlers: program detail template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// EditForm renders the edit form for a program template. Coach only.
func (h *Programs) EditForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
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
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
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

	isLoop := r.FormValue("is_loop") == "1"

	_, err = models.UpdateProgramTemplate(h.DB, id, name, description, numWeeks, numDays, isLoop)
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
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
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
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
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

	var absoluteWeight *float64
	if awStr := r.FormValue("absolute_weight"); awStr != "" {
		v, err := strconv.ParseFloat(awStr, 64)
		if err == nil {
			absoluteWeight = &v
		}
	}

	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))

	notes := r.FormValue("notes")
	repType := r.FormValue("rep_type")

	_, err = models.CreatePrescribedSet(h.DB, templateID, exerciseID, week, day, setNumber, reps, percentage, absoluteWeight, sortOrder, repType, notes)
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
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
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
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
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
	goal := r.FormValue("goal")

	_, err = models.AssignProgram(h.DB, athleteID, templateID, startDate, notes, goal)
	if errors.Is(err, models.ErrProgramAlreadyActive) {
		http.Error(w, "Athlete already has an active program. Deactivate it first.", http.StatusConflict)
		return
	}
	if err != nil {
		log.Printf("handlers: assign program to athlete %d: %v", athleteID, err)
		http.Error(w, "Failed to assign program", http.StatusInternalServerError)
		return
	}

	// Auto-assign the program's exercises to the athlete so they appear
	// in the Assigned Exercises list (for TM management, history, etc.).
	if n, err := models.AssignProgramExercises(h.DB, athleteID, templateID); err != nil {
		log.Printf("handlers: auto-assign program exercises to athlete %d: %v", athleteID, err)
	} else if n > 0 {
		log.Printf("handlers: auto-assigned %d exercises from template %d to athlete %d", n, templateID, athleteID)
	}

	// Redirect to TM setup so the coach can confirm/set training maxes.
	http.Redirect(w, r, fmt.Sprintf("/athletes/%d/training-maxes/setup", athleteID), http.StatusSeeOther)
}

// TMSetupForm renders a form showing all program exercises with current TMs
// pre-filled, so the coach can confirm or set initial training maxes after
// assigning a program.
func (h *Programs) TMSetupForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if err != nil {
		log.Printf("handlers: get athlete %d for TM setup: %v", athleteID, err)
		http.Error(w, "Athlete not found", http.StatusNotFound)
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

	exerciseTMs, err := models.ListProgramExerciseTMs(h.DB, program.TemplateID, athleteID)
	if err != nil {
		log.Printf("handlers: list program exercise TMs (athlete=%d): %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athlete":     athlete,
		"Program":     program,
		"ExerciseTMs": exerciseTMs,
		"TodayDate":   time.Now().Format("2006-01-02"),
	}
	if err := h.Templates.Render(w, r, "tm_setup.html", data); err != nil {
		log.Printf("handlers: TM setup template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// TMSetupSave processes the batch TM setup form submission.
func (h *Programs) TMSetupSave(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
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

	effectiveDate := r.FormValue("effective_date")
	if effectiveDate == "" {
		effectiveDate = time.Now().Format("2006-01-02")
	}

	set := 0
	exerciseIDs := r.Form["exercise_id"]
	for _, eidStr := range exerciseIDs {
		exerciseID, err := strconv.ParseInt(eidStr, 10, 64)
		if err != nil {
			continue
		}

		weightStr := r.FormValue(fmt.Sprintf("tm_%d", exerciseID))
		weight, err := strconv.ParseFloat(weightStr, 64)
		if err != nil || weight <= 0 {
			continue // skip exercises with no weight entered
		}

		_, err = models.SetTrainingMax(h.DB, athleteID, exerciseID, weight, effectiveDate, "Initial TM setup")
		if err != nil {
			log.Printf("handlers: set TM (athlete=%d, exercise=%d): %v", athleteID, exerciseID, err)
			// Continue with remaining exercises.
		} else {
			set++
		}
	}

	log.Printf("handlers: set %d training maxes for athlete %d", set, athleteID)
	http.Redirect(w, r, fmt.Sprintf("/athletes/%d", athleteID), http.StatusSeeOther)
}

// DeactivateProgram deactivates an athlete's current program. Coach only.
func (h *Programs) DeactivateProgram(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
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
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
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

// ProgramCompatibility returns an HTML fragment showing whether an athlete
// has the required equipment for a given program template. Used by htmx on
// the assign-program form to preview equipment readiness.
func (h *Programs) ProgramCompatibility(w http.ResponseWriter, r *http.Request) {
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	templateIDStr := r.URL.Query().Get("template_id")
	if templateIDStr == "" {
		// No program selected — return empty div to clear the audit section.
		w.Write([]byte(`<section id="equipment-audit"></section>`))
		return
	}

	templateID, err := strconv.ParseInt(templateIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if err != nil {
		log.Printf("handlers: get athlete %d for compatibility: %v", athleteID, err)
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}

	compat, err := models.CheckProgramCompatibility(h.DB, athleteID, templateID)
	if err != nil {
		log.Printf("handlers: check program compatibility (athlete=%d, template=%d): %v", athleteID, templateID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athlete":       athlete,
		"Compatibility": compat,
	}

	ts, ok := h.Templates["_program_compatibility"]
	if !ok {
		log.Printf("handlers: program compatibility template not found in cache")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := ts.ExecuteTemplate(w, "program-compatibility", data); err != nil {
		log.Printf("handlers: program compatibility template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// CycleReport renders a print-friendly cycle report for an athlete.
func (h *Programs) CycleReport(w http.ResponseWriter, r *http.Request) {
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if err != nil {
		log.Printf("handlers: get athlete %d for cycle report: %v", athleteID, err)
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}

	report, err := models.GetCycleReport(h.DB, athleteID, time.Now())
	if err != nil {
		log.Printf("handlers: get cycle report for athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if report == nil {
		http.Error(w, "No active program", http.StatusNotFound)
		return
	}

	data := map[string]any{
		"Athlete": athlete,
		"Report":  report,
	}
	if err := h.Templates.Render(w, r, "cycle_report.html", data); err != nil {
		log.Printf("handlers: cycle report template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// AddProgressionRule adds or updates a progression rule for a program template. Coach only.
func (h *Programs) AddProgressionRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
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

	increment, err := strconv.ParseFloat(r.FormValue("increment"), 64)
	if err != nil || increment <= 0 {
		http.Error(w, "Increment must be a positive number", http.StatusBadRequest)
		return
	}

	_, err = models.SetProgressionRule(h.DB, templateID, exerciseID, increment)
	if err != nil {
		log.Printf("handlers: set progression rule (template=%d, exercise=%d): %v", templateID, exerciseID, err)
		http.Error(w, "Failed to save progression rule", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/programs/%d", templateID), http.StatusSeeOther)
}

// DeleteProgressionRule removes a progression rule from a program template. Coach only.
func (h *Programs) DeleteProgressionRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	templateID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid program ID", http.StatusBadRequest)
		return
	}

	ruleID, err := strconv.ParseInt(r.PathValue("ruleID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid rule ID", http.StatusBadRequest)
		return
	}

	if err := models.DeleteProgressionRule(h.DB, ruleID); err != nil {
		log.Printf("handlers: delete progression rule %d: %v", ruleID, err)
		http.Error(w, "Failed to delete progression rule", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/programs/%d", templateID), http.StatusSeeOther)
}

// CycleReview renders the cycle review page showing TM bump suggestions. Coach only.
func (h *Programs) CycleReview(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if err != nil {
		log.Printf("handlers: get athlete %d for cycle review: %v", athleteID, err)
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}

	summary, err := models.GetCycleSummary(h.DB, athleteID, time.Now())
	if err != nil {
		log.Printf("handlers: get cycle summary for athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athlete": athlete,
		"Summary": summary,
	}
	if err := h.Templates.Render(w, r, "cycle_review.html", data); err != nil {
		log.Printf("handlers: cycle review template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// ApplyTMBumps processes the coach's TM bump decisions from the cycle review form.
func (h *Programs) ApplyTMBumps(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
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

	today := time.Now().Format("2006-01-02")
	applied := 0

	// The form submits selected exercise IDs + their suggested TMs.
	exerciseIDs := r.Form["exercise_id"]
	for _, eidStr := range exerciseIDs {
		exerciseID, err := strconv.ParseInt(eidStr, 10, 64)
		if err != nil {
			continue
		}

		// Check if this exercise was selected (checkbox).
		if r.FormValue(fmt.Sprintf("apply_%d", exerciseID)) != "1" {
			continue
		}

		// Get the suggested TM value for this exercise.
		tmStr := r.FormValue(fmt.Sprintf("tm_%d", exerciseID))
		newTM, err := strconv.ParseFloat(tmStr, 64)
		if err != nil || newTM <= 0 {
			continue
		}

		notes := fmt.Sprintf("Cycle progression bump")
		_, err = models.SetTrainingMax(h.DB, athleteID, exerciseID, newTM, today, notes)
		if err != nil {
			log.Printf("handlers: apply TM bump (athlete=%d, exercise=%d): %v", athleteID, exerciseID, err)
			// Continue with other exercises even if one fails.
		} else {
			applied++
		}
	}

	log.Printf("handlers: applied %d TM bumps for athlete %d", applied, athleteID)
	http.Redirect(w, r, fmt.Sprintf("/athletes/%d", athleteID), http.StatusSeeOther)
}
