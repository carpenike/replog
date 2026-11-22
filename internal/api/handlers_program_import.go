package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

const (
	programConfirmTTL    = 15 * time.Minute
	programConfirmPrefix = "rlprog_"
)

// CoachExerciseResponse is a catalog exercise the coach tools can match on.
type CoachExerciseResponse struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Aliases        []string `json:"aliases"`
	Equipment      []string `json:"equipment"`
	PrimaryMuscles []string `json:"primary_muscles"`
	Unilateral     bool     `json:"unilateral"`
	Notes          *string  `json:"notes"`
	Created        bool     `json:"created"`
}

// CoachExerciseRequest creates a catalog exercise. A name or alias that
// already exists returns that row instead of a second one.
type CoachExerciseRequest struct {
	Name           string   `json:"name"`
	Aliases        []string `json:"aliases"`
	Equipment      []string `json:"equipment"`
	PrimaryMuscles []string `json:"primary_muscles"`
	Unilateral     bool     `json:"unilateral"`
	Notes          *string  `json:"notes"`
}

// CoachProgramPreviewRequest is the program to check. athlete_id must match the path.
type CoachProgramPreviewRequest struct {
	AthleteID int64           `json:"athlete_id"`
	Program   json.RawMessage `json:"program"`
}

// CoachCycleResponse is a repeating weekly schedule. upcoming is a count, not copies of a day.
type CoachCycleResponse struct {
	Repeat   string `json:"repeat"`
	Weekdays []int  `json:"weekdays"`
	Upcoming *int   `json:"upcoming"`
}

// CoachProgramResponse is the normalized program.
type CoachProgramResponse struct {
	Name      string              `json:"name"`
	Source    string              `json:"source"`
	Author    string              `json:"author"`
	StartDate *string             `json:"start_date"`
	Weeks     *int                `json:"weeks"`
	Cycle     *CoachCycleResponse `json:"cycle"`
	Days      []CoachDayResponse  `json:"days"`
}

// CoachDayResponse is one day, including instructions and the equipment list.
type CoachDayResponse struct {
	Label            string               `json:"label"`
	Order            int                  `json:"order"`
	Week             int                  `json:"week"`
	Day              int                  `json:"day"`
	Weekday          *int                 `json:"weekday"`
	Kind             *string              `json:"kind"`
	EstimatedMinutes *int                 `json:"estimated_minutes"`
	ExerciseCount    int                  `json:"exercise_count"`
	Equipment        []string             `json:"equipment"`
	Instructions     *string              `json:"instructions"`
	Blocks           []CoachBlockResponse `json:"blocks"`
	Schedule         *CoachCycleResponse  `json:"schedule,omitempty"`
}

// CoachBlockResponse is a straight, superset, or circuit block.
type CoachBlockResponse struct {
	Order       int                 `json:"order"`
	Type        string              `json:"type"`
	Rounds      *int                `json:"rounds"`
	Sets        *int                `json:"sets"`
	RestSeconds *int                `json:"rest_seconds"`
	Rest        *string             `json:"rest"`
	NonWorking  bool                `json:"non_working"`
	Exercises   []CoachLineResponse `json:"exercises"`
}

// CoachLineResponse is one exercise line. ai is null until a gap-fill writes a blank field.
type CoachLineResponse struct {
	ExerciseID  int64            `json:"exercise_id,omitempty"`
	Name        string           `json:"name"`
	Order       int              `json:"order"`
	Sets        *int             `json:"sets"`
	RepsMin     *int             `json:"reps_min"`
	RepsMax     *int             `json:"reps_max"`
	RIRMin      *int             `json:"rir_min"`
	RIRMax      *int             `json:"rir_max"`
	PerSide     bool             `json:"per_side"`
	RestSeconds *int             `json:"rest_seconds"`
	Rest        *string          `json:"rest"`
	Notes       *string          `json:"notes"`
	NonWorking  bool             `json:"non_working"`
	AI          *CoachAIResponse `json:"ai"`
}

// CoachAIResponse is a suggestion stored apart from the coach's values.
type CoachAIResponse struct {
	Sets        *int       `json:"sets"`
	RepsMin     *int       `json:"reps_min"`
	RepsMax     *int       `json:"reps_max"`
	RIRMin      *int       `json:"rir_min"`
	RIRMax      *int       `json:"rir_max"`
	RestSeconds *int       `json:"rest_seconds"`
	Notes       *string    `json:"notes"`
	Load        *CoachLoad `json:"load"`
}

// CoachExerciseMatchResponse is a program name that already exists in the catalog.
type CoachExerciseMatchResponse struct {
	Name       string `json:"name"`
	ExerciseID int64  `json:"exercise_id"`
	MatchedOn  string `json:"matched_on"`
}

// CoachProgramPreviewResponse is the check. Nothing is written.
type CoachProgramPreviewResponse struct {
	Normalized   CoachProgramResponse         `json:"normalized"`
	Matched      []CoachExerciseMatchResponse `json:"matched_exercises"`
	WouldCreate  []string                     `json:"would_create"`
	Unmapped     []string                     `json:"unmapped"`
	PreviewToken string                       `json:"preview_token"`
	ExpiresAt    string                       `json:"expires_at"`
}

// CoachProgramImportRequest confirms a preview. The program itself is the one in the token.
type CoachProgramImportRequest struct {
	PreviewToken string `json:"preview_token"`
	Confirm      bool   `json:"confirm"`
}

// CoachProgramImportResponse is the locked version that was saved.
type CoachProgramImportResponse struct {
	TemplateID       int64                `json:"template_id"`
	Version          int                  `json:"version"`
	LineageID        *int64               `json:"lineage_id"`
	Locked           bool                 `json:"locked"`
	CreatedExercises []string             `json:"created_exercises"`
	Program          CoachProgramResponse `json:"program"`
}

// CoachProgramSummaryResponse is one imported version and, when assigned, that assignment.
type CoachProgramSummaryResponse struct {
	TemplateID      int64               `json:"template_id"`
	Name            string              `json:"name"`
	Version         int                 `json:"version"`
	LineageID       *int64              `json:"lineage_id"`
	Locked          bool                `json:"locked"`
	Source          string              `json:"source"`
	Author          string              `json:"author"`
	StartDate       *string             `json:"start_date"`
	Schedule        *CoachCycleResponse `json:"schedule"`
	AssignmentID    *int64              `json:"assignment_id"`
	AssignmentStart *string             `json:"assignment_start"`
	Days            []CoachDayResponse  `json:"days"`
}

// CoachProgramAssignRequest assigns one stored version. start_date is YYYY-MM-DD.
type CoachProgramAssignRequest struct {
	TemplateID int64  `json:"template_id"`
	Version    int    `json:"version"`
	StartDate  string `json:"start_date"`
}

// CoachProgramAssignResponse is the assignment that was saved.
type CoachProgramAssignResponse struct {
	AssignmentID     int64  `json:"assignment_id"`
	AthleteID        int64  `json:"athlete_id"`
	TemplateID       int64  `json:"template_id"`
	Version          int    `json:"version"`
	StartDate        string `json:"start_date"`
	ScheduleWeekdays []int  `json:"schedule_weekdays"`
}

// CoachGapFillRequest suggests values for fields the coach left blank.
type CoachGapFillRequest struct {
	DayOrder      int      `json:"day_order"`
	BlockOrder    int      `json:"block_order"`
	ExerciseOrder int      `json:"exercise_order"`
	Sets          *int     `json:"sets"`
	RepsMin       *int     `json:"reps_min"`
	RepsMax       *int     `json:"reps_max"`
	RIRMin        *int     `json:"rir_min"`
	RIRMax        *int     `json:"rir_max"`
	RestSeconds   *int     `json:"rest_seconds"`
	Notes         *string  `json:"notes"`
	Load          *float64 `json:"load"`
	Exercise      *string  `json:"exercise"`
}

// ListCoachExercises lists catalog exercises by name or alias.
//
//	@Summary      List exercises
//	@Description  Case-insensitive match on name or alias for an athlete this caller can see. Does not create rows. An admin service token does not see every athlete.
//	@Tags         Coach
//	@Produce      json
//	@Param        id  path   int     true   "Athlete ID"
//	@Param        q   query  string  false  "Name or alias"
//	@Success      200  {array}   api.CoachExerciseResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/coach/exercises [get]
func (h *Handlers) ListCoachExercises(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.coachProgramAccess(w, r, false); !ok {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > 100 {
		WriteValidationError(w, "q", "must be at most 100 characters")
		return
	}
	rows, err := models.SearchCoachExercises(r.Context(), h.DB, q)
	if err != nil {
		log.Printf("api: list coach exercises: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to list exercises")
		return
	}
	out := make([]CoachExerciseResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, coachExerciseResponse(row, false))
	}
	WriteJSON(w, http.StatusOK, out)
}

// CreateCoachExercise adds a catalog exercise, or returns the existing row when the name or an alias matches.
//
//	@Summary      Create exercise
//	@Description  Creates a catalog exercise for a coach of this athlete. A case-insensitive name or alias match returns the existing row and does not update it. A service token cannot create an exercise for an athlete it does not coach.
//	@Tags         Coach
//	@Accept       json
//	@Produce      json
//	@Param        id    path  int                       true  "Athlete ID"
//	@Param        body  body  api.CoachExerciseRequest  true  "Exercise"
//	@Success      200  {object}  api.CoachExerciseResponse
//	@Success      201  {object}  api.CoachExerciseResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Failure      409  {object}  api.APIError
//	@Router       /athletes/{id}/coach/exercises [post]
func (h *Handlers) CreateCoachExercise(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.coachProgramAccess(w, r, true); !ok {
		return
	}
	var req CoachExerciseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	row, created, err := models.FindOrCreateCoachExercise(r.Context(), h.DB, models.CoachExerciseDraft{
		Name: req.Name, Aliases: req.Aliases, Equipment: req.Equipment,
		PrimaryMuscles: req.PrimaryMuscles, Unilateral: req.Unilateral, Notes: req.Notes,
	})
	if err != nil {
		writeProgramError(w, "create exercise", err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	WriteJSON(w, status, coachExerciseResponse(*row, created))
}

// PreviewCoachProgram checks a program and returns a one-time token. It writes nothing.
//
//	@Summary      Preview a coach program
//	@Description  Normalizes the program, matches exercises, and lists fields that would not be stored. Does not write. The preview token lasts 15 minutes and works once. athlete_id must match the path.
//	@Tags         Coach
//	@Accept       json
//	@Produce      json
//	@Param        id    path  int                              true  "Athlete ID"
//	@Param        body  body  api.CoachProgramPreviewRequest   true  "Program"
//	@Success      200  {object}  api.CoachProgramPreviewResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/coach/programs/preview [post]
func (h *Handlers) PreviewCoachProgram(w http.ResponseWriter, r *http.Request) {
	user, athlete, ok := h.coachProgramAccess(w, r, false)
	if !ok {
		return
	}
	raw, err := readObject(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	unmapped := unknownKeys(raw, map[string]struct{}{"athlete_id": {}, "program": {}}, "")
	var req CoachProgramPreviewRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.AthleteID == 0 {
		WriteValidationError(w, "athlete_id", "is required")
		return
	}
	if req.AthleteID != athlete.ID {
		WriteValidationError(w, "athlete_id", "must match the athlete in the path")
		return
	}
	if len(req.Program) == 0 || req.Program[0] != '{' {
		WriteValidationError(w, "program", "must be an object")
		return
	}
	programRaw := req.Program
	unmapped = append(unmapped, prefixPaths("program", unknownKeys(programRaw, programKnownKeys, ""))...)
	unmapped = append(unmapped, nestedProgramUnmapped(programRaw)...)
	var input models.CoachProgramInput
	if err := json.Unmarshal(programRaw, &input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid program")
		return
	}
	program, err := models.NormalizeCoachProgram(input)
	if err != nil {
		writeProgramError(w, "preview program", err)
		return
	}
	matched, wouldCreate, err := models.MatchProgramExercises(r.Context(), h.DB, program)
	if err != nil {
		log.Printf("api: preview program athlete %d: %v", athlete.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to preview the program")
		return
	}
	token, expires, err := h.issueProgramGrant(user.ID, athlete.ID, program, time.Now())
	if err != nil {
		log.Printf("api: preview program token athlete %d: %v", athlete.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to preview the program")
		return
	}
	sort.Strings(unmapped)
	ids := map[string]int64{}
	matchOut := make([]CoachExerciseMatchResponse, 0, len(matched))
	for _, match := range matched {
		ids[strings.ToLower(match.Name)] = match.ExerciseID
		matchOut = append(matchOut, CoachExerciseMatchResponse{Name: match.Name, ExerciseID: match.ExerciseID, MatchedOn: match.MatchedOn})
	}
	WriteJSON(w, http.StatusOK, CoachProgramPreviewResponse{
		Normalized:   coachProgramResponse(program, ids),
		Matched:      matchOut,
		WouldCreate:  wouldCreate,
		Unmapped:     unmapped,
		PreviewToken: token,
		ExpiresAt:    expires.In(userLocation(r)).Format(time.RFC3339),
	})
}

// ImportCoachProgram saves the previewed program as a locked version.
//
//	@Summary      Import a coach program
//	@Description  confirm must be true. Saves the previewed program as a new locked version and creates missing exercises in that transaction. The same name creates the next version and does not change the previous one. A service token may call this for an athlete it coaches. The saved sets, reps, exercises, and notes are read-only to that token.
//	@Tags         Coach
//	@Accept       json
//	@Produce      json
//	@Param        id    path  int                             true  "Athlete ID"
//	@Param        body  body  api.CoachProgramImportRequest   true  "Preview token"
//	@Success      201  {object}  api.CoachProgramImportResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/coach/programs/import [post]
func (h *Handlers) ImportCoachProgram(w http.ResponseWriter, r *http.Request) {
	user, athlete, ok := h.coachProgramAccess(w, r, true)
	if !ok {
		return
	}
	raw, err := readObject(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	extra := unknownKeys(raw, map[string]struct{}{"preview_token": {}, "confirm": {}}, "")
	if len(extra) > 0 {
		WriteValidationError(w, extra[0], "import accepts preview_token and confirm")
		return
	}
	var req CoachProgramImportRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.PreviewToken) == "" {
		WriteValidationError(w, "preview_token", "is required")
		return
	}
	if !req.Confirm {
		WriteValidationError(w, "confirm", "must be true")
		return
	}
	program, result := h.consumeProgramGrant(user.ID, athlete.ID, req.PreviewToken, time.Now())
	if result != noteGrantOK {
		WriteValidationError(w, "preview_token", "is invalid or expired")
		return
	}
	imported, err := models.ImportCoachProgram(r.Context(), h.DB, athlete.ID, user.ID, todayInUserTZ(r), program)
	if err != nil {
		writeProgramError(w, "import program", err)
		return
	}
	WriteJSON(w, http.StatusCreated, importResponse(*imported))
}

// ListCoachPrograms lists imported program versions for one athlete.
//
//	@Summary      List coach programs
//	@Description  Imported versions for this athlete, including days. Does not change a program or a training max. An admin service token does not see every athlete.
//	@Tags         Coach
//	@Produce      json
//	@Param        id  path  int  true  "Athlete ID"
//	@Success      200  {array}   api.CoachProgramSummaryResponse
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/coach/programs [get]
func (h *Handlers) ListCoachPrograms(w http.ResponseWriter, r *http.Request) {
	_, athlete, ok := h.coachProgramAccess(w, r, false)
	if !ok {
		return
	}
	rows, err := models.ListImportedPrograms(r.Context(), h.DB, athlete.ID)
	if err != nil {
		log.Printf("api: list coach programs athlete %d: %v", athlete.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list programs")
		return
	}
	out := make([]CoachProgramSummaryResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, summaryResponse(row))
	}
	WriteJSON(w, http.StatusOK, out)
}

// AssignCoachProgram assigns one imported version to the athlete.
//
//	@Summary      Assign a coach program
//	@Description  Assigns the locked version named by template_id and version. The stored weekdays become the assignment schedule. Replaces an active primary. Does not change the template. A service token may call this for an athlete it coaches.
//	@Tags         Coach
//	@Accept       json
//	@Produce      json
//	@Param        id    path  int                            true  "Athlete ID"
//	@Param        body  body  api.CoachProgramAssignRequest  true  "Assignment"
//	@Success      201  {object}  api.CoachProgramAssignResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Failure      409  {object}  api.APIError
//	@Router       /athletes/{id}/coach/programs/assign [post]
func (h *Handlers) AssignCoachProgram(w http.ResponseWriter, r *http.Request) {
	_, athlete, ok := h.coachProgramAccess(w, r, true)
	if !ok {
		return
	}
	var req CoachProgramAssignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TemplateID <= 0 {
		WriteValidationError(w, "template_id", "is required")
		return
	}
	if req.Version < 1 {
		WriteValidationError(w, "version", "must be at least 1")
		return
	}
	if !validDate(req.StartDate) {
		WriteValidationError(w, "start_date", "must be YYYY-MM-DD")
		return
	}
	assigned, cycle, err := models.AssignImportedProgram(r.Context(), h.DB, athlete.ID, req.TemplateID, req.Version, req.StartDate)
	if err != nil {
		writeProgramError(w, "assign program", err)
		return
	}
	weekdays := []int{}
	if cycle != nil && cycle.Weekdays != nil {
		weekdays = cycle.Weekdays
	}
	WriteJSON(w, http.StatusCreated, CoachProgramAssignResponse{
		AssignmentID: assigned.ID, AthleteID: athlete.ID, TemplateID: req.TemplateID,
		Version: req.Version, StartDate: req.StartDate, ScheduleWeekdays: weekdays,
	})
}

// GapFillCoachProgram stores an AI suggestion for a field the coach left blank.
//
//	@Summary      Gap-fill a coach program
//	@Description  Writes only fields the coach left empty, marked as AI suggestions. A request that would change coach-set sets, reps, exercises, notes, or any other coach value is refused and writes nothing.
//	@Tags         Coach
//	@Accept       json
//	@Produce      json
//	@Param        id          path  int                      true  "Athlete ID"
//	@Param        templateID  path  int                      true  "Template ID"
//	@Param        body        body  api.CoachGapFillRequest  true  "Blank fields"
//	@Success      200  {object}  api.CoachLineResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Failure      409  {object}  api.APIError
//	@Router       /athletes/{id}/coach/programs/{templateID}/gap-fill [post]
func (h *Handlers) GapFillCoachProgram(w http.ResponseWriter, r *http.Request) {
	_, athlete, ok := h.coachProgramAccess(w, r, true)
	if !ok {
		return
	}
	templateID, err := strconv.ParseInt(r.PathValue("templateID"), 10, 64)
	if err != nil || templateID <= 0 {
		WriteError(w, http.StatusBadRequest, "invalid template ID")
		return
	}
	var req CoachGapFillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Load != nil && *req.Load < 0 {
		WriteValidationError(w, "load", "must be zero or greater")
		return
	}
	line, err := models.GapFillCoachProgram(r.Context(), h.DB, athlete.ID, templateID, models.GapFillInput{
		DayOrder: req.DayOrder, BlockOrder: req.BlockOrder, ExerciseOrder: req.ExerciseOrder,
		Sets: req.Sets, RepsMin: req.RepsMin, RepsMax: req.RepsMax, RIRMin: req.RIRMin, RIRMax: req.RIRMax,
		RestSeconds: req.RestSeconds, Notes: req.Notes, Load: req.Load, ExerciseName: req.Exercise,
	})
	if err != nil {
		writeProgramError(w, "gap-fill program", err)
		return
	}
	WriteJSON(w, http.StatusOK, coachLineResponse(*line))
}

func (h *Handlers) coachProgramAccess(w http.ResponseWriter, r *http.Request, write bool) (*models.User, *models.Athlete, bool) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		WriteError(w, http.StatusUnauthorized, "authentication required")
		return nil, nil, false
	}
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || athleteID <= 0 {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return nil, nil, false
	}
	athlete, err := models.GetAthleteByID(r.Context(), h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "athlete not found")
		return nil, nil, false
	}
	if err != nil {
		log.Printf("api: coach program athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to load athlete")
		return nil, nil, false
	}
	service := middleware.IsServiceCredential(r.Context())
	allowed := false
	if service {
		allowed = middleware.ServiceCanAccessAthlete(r.Context(), h.DB, user, athleteID)
	} else {
		allowed = middleware.CanAccessAthlete(r.Context(), h.DB, user, athleteID)
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "access denied")
		return nil, nil, false
	}
	if write && !coachOwns(user, athlete, service) {
		WriteError(w, http.StatusForbidden, "coach access required")
		return nil, nil, false
	}
	return user, athlete, true
}

func (h *Handlers) refuseLockedProgram(w http.ResponseWriter, r *http.Request, templateID int64) bool {
	locked, err := models.ProgramIsLocked(r.Context(), h.DB, templateID)
	if err != nil {
		log.Printf("api: program lock %d: %v", templateID, err)
		WriteError(w, http.StatusInternalServerError, "failed to read the program")
		return true
	}
	if locked {
		WriteError(w, http.StatusConflict, "program is locked")
		return true
	}
	return false
}

func (h *Handlers) refuseLockedSet(w http.ResponseWriter, r *http.Request, setID int64) bool {
	templateID, err := models.TemplateIDForPrescribedSet(r.Context(), h.DB, setID)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "set not found")
		return true
	}
	if err != nil {
		log.Printf("api: program lock for set %d: %v", setID, err)
		WriteError(w, http.StatusInternalServerError, "failed to read the set")
		return true
	}
	return h.refuseLockedProgram(w, r, templateID)
}

func coachOwns(user *models.User, athlete *models.Athlete, service bool) bool {
	if !service && user.IsAdmin {
		return true
	}
	return athlete.CoachID.Valid && athlete.CoachID.Int64 == user.ID
}

func writeProgramError(w http.ResponseWriter, action string, err error) {
	var coachField *models.CoachFieldError
	switch {
	case errors.As(err, &coachField):
		WriteError(w, http.StatusConflict, coachField.Error())
	case errors.Is(err, models.ErrVersionMismatch):
		WriteError(w, http.StatusConflict, "version does not match")
	case errors.Is(err, models.ErrExerciseAmbiguous):
		WriteError(w, http.StatusConflict, "name and alias match different exercises")
	case errors.Is(err, models.ErrNotFound):
		WriteError(w, http.StatusNotFound, "program not found")
	case errors.Is(err, models.ErrInvalidInput), errors.Is(err, models.ErrInvalidSchedule):
		WriteError(w, http.StatusBadRequest, err.Error())
	default:
		log.Printf("api: %s: %v", action, err)
		WriteError(w, http.StatusInternalServerError, "failed to "+action)
	}
}

func coachExerciseResponse(row models.CoachCatalogExercise, created bool) CoachExerciseResponse {
	return CoachExerciseResponse{
		ID: row.ID, Name: row.Name, Aliases: row.Aliases, Equipment: row.Equipment,
		PrimaryMuscles: row.PrimaryMuscles, Unilateral: row.Unilateral, Notes: row.Notes, Created: created,
	}
}

func importResponse(result models.CoachImportResult) CoachProgramImportResponse {
	return CoachProgramImportResponse{
		TemplateID: result.Program.TemplateID, Version: result.Program.Version, LineageID: result.Program.LineageID,
		Locked: result.Program.Locked, CreatedExercises: result.CreatedExercises,
		Program: storedProgramResponse(result.Program),
	}
}

func summaryResponse(row models.StoredCoachProgram) CoachProgramSummaryResponse {
	return CoachProgramSummaryResponse{
		TemplateID: row.TemplateID, Name: row.Name, Version: row.Version, LineageID: row.LineageID,
		Locked: row.Locked, Source: row.Source, Author: row.Author, StartDate: row.StartDate,
		Schedule: cycleResponse(row.Cycle), AssignmentID: row.AssignmentID, AssignmentStart: row.AssignmentStart,
		Days: dayResponses(row.Days),
	}
}

func storedProgramResponse(row models.StoredCoachProgram) CoachProgramResponse {
	var weeks *int
	if row.Cycle == nil || row.NumWeeks > 1 {
		value := row.NumWeeks
		if value < 1 {
			value = 1
		}
		weeks = &value
	}
	return CoachProgramResponse{
		Name: row.Name, Source: row.Source, Author: row.Author, StartDate: row.StartDate,
		Weeks: weeks, Cycle: cycleResponse(row.Cycle), Days: dayResponses(row.Days),
	}
}

func coachProgramResponse(program models.CoachProgram, ids map[string]int64) CoachProgramResponse {
	days := dayResponses(program.Days)
	for i := range days {
		for j := range days[i].Blocks {
			for k := range days[i].Blocks[j].Exercises {
				if id, ok := ids[strings.ToLower(days[i].Blocks[j].Exercises[k].Name)]; ok {
					days[i].Blocks[j].Exercises[k].ExerciseID = id
				}
			}
		}
	}
	return CoachProgramResponse{
		Name: program.Name, Source: program.Source, Author: program.Author, StartDate: program.StartDate,
		Weeks: program.Weeks, Cycle: cycleResponse(program.Cycle), Days: days,
	}
}

func cycleResponse(cycle *models.CoachCycle) *CoachCycleResponse {
	if cycle == nil {
		return nil
	}
	weekdays := cycle.Weekdays
	if weekdays == nil {
		weekdays = []int{}
	}
	return &CoachCycleResponse{Repeat: cycle.Repeat, Weekdays: weekdays, Upcoming: cycle.Upcoming}
}

func dayResponses(days []models.CoachDay) []CoachDayResponse {
	out := make([]CoachDayResponse, 0, len(days))
	for _, day := range days {
		equipment := day.Equipment
		if equipment == nil {
			equipment = []string{}
		}
		blocks := make([]CoachBlockResponse, 0, len(day.Blocks))
		for _, block := range day.Blocks {
			lines := make([]CoachLineResponse, 0, len(block.Exercises))
			for _, line := range block.Exercises {
				lines = append(lines, coachLineResponse(line))
			}
			blocks = append(blocks, CoachBlockResponse{
				Order: block.Order, Type: block.Type, Rounds: block.Rounds, Sets: block.Sets,
				RestSeconds: block.RestSeconds, Rest: block.Rest, NonWorking: block.NonWorking, Exercises: lines,
			})
		}
		out = append(out, CoachDayResponse{
			Label: day.Label, Order: day.Order, Week: day.Week, Day: day.Number, Weekday: day.Weekday,
			Kind: day.Kind, EstimatedMinutes: day.EstimatedMinutes, ExerciseCount: day.ExerciseCount,
			Equipment: equipment, Instructions: day.Instructions, Blocks: blocks,
		})
	}
	return out
}

func coachLineResponse(line models.CoachBlockExercise) CoachLineResponse {
	return CoachLineResponse{
		ExerciseID: line.ExerciseID, Name: line.Name, Order: line.Order, Sets: line.Sets,
		RepsMin: line.RepsMin, RepsMax: line.RepsMax, RIRMin: line.RIRMin, RIRMax: line.RIRMax,
		PerSide: line.PerSide, RestSeconds: line.RestSeconds, Rest: line.Rest, Notes: line.Notes,
		NonWorking: line.NonWorking, AI: aiResponse(line.AI),
	}
}

func aiResponse(ai *models.CoachAIFields) *CoachAIResponse {
	if ai == nil {
		return nil
	}
	var load *CoachLoad
	if ai.Load != nil {
		value := *ai.Load
		load = &CoachLoad{Value: &value, Unit: coachLoadUnit}
	} else {
		load = &CoachLoad{Unit: coachLoadUnit}
	}
	return &CoachAIResponse{
		Sets: ai.Sets, RepsMin: ai.RepsMin, RepsMax: ai.RepsMax, RIRMin: ai.RIRMin, RIRMax: ai.RIRMax,
		RestSeconds: ai.RestSeconds, Notes: ai.Notes, Load: load,
	}
}

// attachImportedDay loads the stored day onto a coach read when this template has one.
// The day's schedule is the program cycle, including the upcoming count.
func (h *Handlers) attachImportedDay(r *http.Request, templateID int64, week, day int) (*CoachDayResponse, *CoachCycleResponse) {
	if templateID == 0 || week < 1 || day < 1 {
		return nil, nil
	}
	loaded, cycle, err := models.LoadImportedDay(r.Context(), h.DB, templateID, week, day)
	if err != nil {
		log.Printf("api: coach program day template %d: %v", templateID, err)
		return nil, nil
	}
	if loaded == nil {
		return nil, nil
	}
	days := dayResponses([]models.CoachDay{*loaded})
	if len(days) == 0 {
		return nil, nil
	}
	dayResp := days[0]
	dayResp.Schedule = cycleResponse(cycle)
	return &dayResp, dayResp.Schedule
}

func overlayPrescriptions(exercises []CoachTargetExerciseResponse, day *CoachDayResponse) {
	if day == nil {
		return
	}
	byID := map[int64]CoachLineResponse{}
	for _, block := range day.Blocks {
		for _, line := range block.Exercises {
			byID[line.ExerciseID] = line
		}
	}
	for i := range exercises {
		if line, ok := byID[exercises[i].ExerciseID]; ok {
			copied := line
			exercises[i].Prescription = &copied
		}
	}
}

var (
	programKnownKeys = map[string]struct{}{
		"name": {}, "source": {}, "author": {}, "start_date": {}, "weeks": {}, "cycle": {}, "days": {},
	}
	cycleKnownKeys = map[string]struct{}{"repeat": {}, "weekdays": {}, "upcoming": {}}
	dayKnownKeys   = map[string]struct{}{
		"label": {}, "order": {}, "week": {}, "weekday": {}, "kind": {}, "estimated_minutes": {},
		"exercise_count": {}, "equipment": {}, "instructions": {}, "blocks": {},
	}
	blockKnownKeys = map[string]struct{}{
		"type": {}, "order": {}, "rounds": {}, "sets": {}, "rest_seconds": {}, "rest": {},
		"non_working": {}, "exercises": {},
	}
	lineKnownKeys = map[string]struct{}{
		"name": {}, "order": {}, "sets": {}, "reps_min": {}, "reps_max": {}, "rir_min": {}, "rir_max": {},
		"per_side": {}, "rest_seconds": {}, "rest": {}, "notes": {}, "non_working": {},
	}
)

func nestedProgramUnmapped(raw json.RawMessage) []string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	var out []string
	if cycle, ok := obj["cycle"]; ok {
		out = append(out, prefixPaths("program.cycle", unknownRaw(cycle, cycleKnownKeys))...)
	}
	days, ok := obj["days"]
	if !ok {
		return out
	}
	var list []json.RawMessage
	if err := json.Unmarshal(days, &list); err != nil {
		return out
	}
	for i, dayRaw := range list {
		dayPrefix := "program.days[" + strconv.Itoa(i) + "]"
		out = append(out, prefixPaths(dayPrefix, unknownRaw(dayRaw, dayKnownKeys))...)
		var day map[string]json.RawMessage
		if err := json.Unmarshal(dayRaw, &day); err != nil {
			continue
		}
		var blocks []json.RawMessage
		if err := json.Unmarshal(day["blocks"], &blocks); err != nil {
			continue
		}
		for j, blockRaw := range blocks {
			blockPrefix := dayPrefix + ".blocks[" + strconv.Itoa(j) + "]"
			out = append(out, prefixPaths(blockPrefix, unknownRaw(blockRaw, blockKnownKeys))...)
			var block map[string]json.RawMessage
			if err := json.Unmarshal(blockRaw, &block); err != nil {
				continue
			}
			var lines []json.RawMessage
			if err := json.Unmarshal(block["exercises"], &lines); err != nil {
				continue
			}
			for k, lineRaw := range lines {
				out = append(out, prefixPaths(blockPrefix+".exercises["+strconv.Itoa(k)+"]", unknownRaw(lineRaw, lineKnownKeys))...)
			}
		}
	}
	return out
}

func unknownRaw(raw json.RawMessage, known map[string]struct{}) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	return unknownKeys(raw, known, "")
}

func unknownKeys(raw json.RawMessage, known map[string]struct{}, prefix string) []string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	var out []string
	for key := range obj {
		if _, ok := known[key]; ok {
			continue
		}
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func prefixPaths(prefix string, paths []string) []string {
	if prefix == "" {
		return paths
	}
	out := make([]string, len(paths))
	for i, path := range paths {
		out[i] = prefix + "." + path
	}
	return out
}

func readObject(r *http.Request) (json.RawMessage, error) {
	var raw json.RawMessage
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 || raw[0] != '{' {
		return nil, errors.New("object required")
	}
	return raw, nil
}

type programGrant struct {
	userID    int64
	athleteID int64
	program   models.CoachProgram
	expires   time.Time
}

type programGrantStore struct {
	mu     sync.Mutex
	byHash map[string]programGrant
}

func (h *Handlers) programGrants() *programGrantStore {
	h.programGrantOnce.Do(func() {
		h.programGrantStore = &programGrantStore{byHash: map[string]programGrant{}}
	})
	return h.programGrantStore
}

func (h *Handlers) issueProgramGrant(userID, athleteID int64, program models.CoachProgram, now time.Time) (string, time.Time, error) {
	token, key, err := models.NewConfirmToken(programConfirmPrefix)
	if err != nil {
		return "", time.Time{}, err
	}
	expires := now.Add(programConfirmTTL)
	store := h.programGrants()
	store.mu.Lock()
	defer store.mu.Unlock()
	store.byHash[key] = programGrant{userID: userID, athleteID: athleteID, program: program, expires: expires}
	return token, expires, nil
}

func (h *Handlers) consumeProgramGrant(userID, athleteID int64, token string, now time.Time) (models.CoachProgram, noteGrantResult) {
	key := models.HashConfirmToken(token)
	store := h.programGrants()
	store.mu.Lock()
	defer store.mu.Unlock()
	grant, ok := store.byHash[key]
	if !ok || !grant.expires.After(now) {
		delete(store.byHash, key)
		return models.CoachProgram{}, noteGrantMissing
	}
	if grant.userID != userID || grant.athleteID != athleteID {
		return models.CoachProgram{}, noteGrantMismatch
	}
	delete(store.byHash, key)
	return grant.program, noteGrantOK
}
