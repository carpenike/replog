package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

const (
	coachLoadUnit     = "lb"
	coachNoteMaxRunes = 4000
	noteConfirmTTL    = 15 * time.Minute
	noteConfirmPrefix = "rlnote_"
)

// CoachLoad is a stored load labeled in pounds. Value is null when the
// row has no weight. The weight column has no unit of its own.
type CoachLoad struct {
	Value *float64 `json:"value"`
	Unit  string   `json:"unit"`
}

// CoachLoggedSetResponse is one logged set. rir and set_type are null:
// those columns are not stored. category is the stored classification.
type CoachLoggedSetResponse struct {
	SetID     int64     `json:"set_id"`
	SetNumber int       `json:"set_number"`
	Reps      int       `json:"reps"`
	Load      CoachLoad `json:"load"`
	RPE       *float64  `json:"rpe"`
	RIR       *float64  `json:"rir"`
	SetType   *string   `json:"set_type"`
	Category  string    `json:"category"`
	Notes     *string   `json:"notes"`
}

// CoachLoggedExerciseResponse is one exercise and its sets in log order.
type CoachLoggedExerciseResponse struct {
	ExerciseID int64                    `json:"exercise_id"`
	Name       string                   `json:"name"`
	Sets       []CoachLoggedSetResponse `json:"sets"`
}

// CoachSessionResponse is one resistance session. started_at and duration
// are null because those columns are not stored.
type CoachSessionResponse struct {
	SessionID       int64                         `json:"session_id"`
	AthleteID       int64                         `json:"athlete_id"`
	Date            string                        `json:"date"`
	StartedAt       *string                       `json:"started_at"`
	ProgramDayLabel *string                       `json:"program_day_label"`
	Duration        *int                          `json:"duration"`
	Notes           *string                       `json:"notes"`
	Exercises       []CoachLoggedExerciseResponse `json:"exercises"`
	ProgramDay      *CoachDayResponse             `json:"program_day,omitempty"`
}

// CoachHistoryResponse is one page of session history.
type CoachHistoryResponse struct {
	Sessions   []CoachSessionResponse `json:"sessions"`
	NextCursor *string                `json:"next_cursor"`
}

// CoachWeekDayResponse is one planned day and how it was logged.
type CoachWeekDayResponse struct {
	Date             string            `json:"date"`
	Weekday          int               `json:"weekday"`
	ProgramID        int64             `json:"program_id"`
	ProgramName      string            `json:"program_name"`
	ProgramDayLabel  *string           `json:"program_day_label"`
	Week             *int              `json:"week"`
	Day              *int              `json:"day"`
	SessionID        *int64            `json:"session_id"`
	Status           string            `json:"status"`
	CompletedOn      *string           `json:"completed_on"`
	PlannedExercises int               `json:"planned_exercises"`
	LoggedExercises  int               `json:"logged_exercises"`
	ProgramDay       *CoachDayResponse `json:"program_day,omitempty"`
}

// CoachWeekResponse is planned versus done for a Monday–Sunday week.
type CoachWeekResponse struct {
	WeekStart     string                 `json:"week_start"`
	WeekEnd       string                 `json:"week_end"`
	Timezone      string                 `json:"timezone"`
	AthleteID     int64                  `json:"athlete_id"`
	PlannedCount  int                    `json:"planned_count"`
	DoneCount     int                    `json:"done_count"`
	PartialCount  int                    `json:"partial_count"`
	MissedCount   int                    `json:"missed_count"`
	UpcomingCount int                    `json:"upcoming_count"`
	Summary       string                 `json:"summary"`
	Days          []CoachWeekDayResponse `json:"days"`
}

// CoachTopSetResponse is the heaviest set of a session.
type CoachTopSetResponse struct {
	SessionID int64     `json:"session_id"`
	Date      string    `json:"date"`
	Reps      int       `json:"reps"`
	Load      CoachLoad `json:"load"`
	E1RM      *float64  `json:"e1rm"`
	RPE       *float64  `json:"rpe"`
	RIR       *float64  `json:"rir"`
}

// CoachProgressPointResponse is one session in a progress series.
type CoachProgressPointResponse struct {
	SessionID int64               `json:"session_id"`
	Date      string              `json:"date"`
	E1RM      *float64            `json:"e1rm"`
	TopSet    CoachTopSetResponse `json:"top_set"`
}

// CoachPRResponse is a session that set a new estimated 1RM.
type CoachPRResponse struct {
	SessionID int64     `json:"session_id"`
	Date      string    `json:"date"`
	E1RM      float64   `json:"e1rm"`
	Reps      int       `json:"reps"`
	Load      CoachLoad `json:"load"`
}

// CoachProgressExerciseResponse is one exercise's progress.
type CoachProgressExerciseResponse struct {
	ExerciseID        int64                        `json:"exercise_id"`
	Name              string                       `json:"name"`
	Sessions          []CoachProgressPointResponse `json:"sessions"`
	PRs               []CoachPRResponse            `json:"prs"`
	LastSessionTopSet *CoachTopSetResponse         `json:"last_session_top_set"`
}

// CoachProgressResponse is read-only progress. e1rm_source names the formula.
type CoachProgressResponse struct {
	AthleteID  int64                           `json:"athlete_id"`
	Start      *string                         `json:"start"`
	End        *string                         `json:"end"`
	E1RMSource string                          `json:"e1rm_source"`
	Exercises  []CoachProgressExerciseResponse `json:"exercises"`
}

// CoachTargetSetResponse is one prescribed set on the next session.
type CoachTargetSetResponse struct {
	SetNumber  int       `json:"set_number"`
	Reps       *int64    `json:"reps"`
	Load       CoachLoad `json:"load"`
	Percentage *float64  `json:"percentage"`
	RPE        *float64  `json:"rpe"`
	RIR        *float64  `json:"rir"`
}

// CoachTargetExerciseResponse is one exercise on the next session.
type CoachTargetExerciseResponse struct {
	ExerciseID   int64                    `json:"exercise_id"`
	Name         string                   `json:"name"`
	Sets         []CoachTargetSetResponse `json:"sets"`
	Prescription *CoachLineResponse       `json:"prescription,omitempty"`
}

// CoachNextSessionResponse is the next session. It is not created.
type CoachNextSessionResponse struct {
	Date            string                        `json:"date"`
	ProgramDayLabel string                        `json:"program_day_label"`
	Week            int                           `json:"week"`
	Day             int                           `json:"day"`
	Exercises       []CoachTargetExerciseResponse `json:"exercises"`
	ProgramDay      *CoachDayResponse             `json:"program_day,omitempty"`
}

// CoachPositionResponse is the current program position. block is null
// because the schema has no block column.
type CoachPositionResponse struct {
	AthleteID       int64                     `json:"athlete_id"`
	ProgramID       *int64                    `json:"program_id"`
	ProgramName     *string                   `json:"program_name"`
	Block           *string                   `json:"block"`
	CycleNumber     *int                      `json:"cycle_number"`
	Week            *int                      `json:"week"`
	Day             *int                      `json:"day"`
	ProgramDayLabel *string                   `json:"program_day_label"`
	CycleComplete   bool                      `json:"cycle_complete"`
	ProgramDay      *CoachDayResponse         `json:"program_day,omitempty"`
	NextSession     *CoachNextSessionResponse `json:"next_session"`
}

// CoachLastSessionResponse is the most recent logging of one exercise.
type CoachLastSessionResponse struct {
	AthleteID  int64                 `json:"athlete_id"`
	ExerciseID int64                 `json:"exercise_id"`
	Name       string                `json:"name"`
	Session    *CoachLastSessionBody `json:"session"`
}

// CoachLastSessionBody is that session and its sets for the exercise.
type CoachLastSessionBody struct {
	SessionID int64                    `json:"session_id"`
	Date      string                   `json:"date"`
	StartedAt *string                  `json:"started_at"`
	Sets      []CoachLoggedSetResponse `json:"sets"`
}

// CoachNotePreviewResponse is what a confirm would write. Nothing is stored yet.
type CoachNotePreviewResponse struct {
	WouldWrite   CoachNoteWrite `json:"would_write"`
	ConfirmToken string         `json:"confirm_token"`
	ExpiresAt    string         `json:"expires_at"`
}

// CoachNoteWrite is the session note a confirm is allowed to store.
type CoachNoteWrite struct {
	AthleteID int64  `json:"athlete_id"`
	SessionID int64  `json:"session_id"`
	Notes     string `json:"notes"`
}

// CoachNoteConfirmResponse is the note after it is stored.
type CoachNoteConfirmResponse struct {
	AthleteID int64   `json:"athlete_id"`
	SessionID int64   `json:"session_id"`
	Notes     *string `json:"notes"`
}

// ListCoachSessions returns paginated resistance session history.
//
//	@Summary      Coach session history
//	@Description  Sessions the caller can see for this athlete. Loads are pounds. started_at, duration, rir, and set_type are null because those columns are not stored. program_day_label is derived from the assignment sequence when the session is linked to a program.
//	@Tags         Workouts
//	@Produce      json
//	@Param        id        path   int     true   "Athlete ID"
//	@Param        start     query  string  true   "Range start, YYYY-MM-DD"
//	@Param        end       query  string  true   "Range end, YYYY-MM-DD"
//	@Param        exercise  query  int     false  "Exercise ID"
//	@Param        cursor    query  string  false  "Page cursor from the previous response"
//	@Success      200  {object}  api.CoachHistoryResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/coach/sessions [get]
func (h *Handlers) ListCoachSessions(w http.ResponseWriter, r *http.Request) {
	_, athleteID, ok := h.coachReadAccess(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	start, end := q.Get("start"), q.Get("end")
	if !validDate(start) || !validDate(end) || start > end {
		WriteValidationError(w, "start", "start and end must be YYYY-MM-DD dates, with start on or before end")
		return
	}
	exerciseID, ok := optionalPositiveID(w, q.Get("exercise"), "exercise")
	if !ok {
		return
	}
	cursorID, err := decodeCoachCursor(q.Get("cursor"))
	if err != nil {
		WriteValidationError(w, "cursor", "must be a cursor from this athlete")
		return
	}
	sessions, hasMore, err := models.ListCoachSessions(r.Context(), h.DB, athleteID, start, end, exerciseID, cursorID)
	if errors.Is(err, models.ErrInvalidCursor) {
		WriteValidationError(w, "cursor", "must be a cursor from this athlete")
		return
	}
	if err != nil {
		log.Printf("api: coach sessions athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}
	resp := CoachHistoryResponse{Sessions: make([]CoachSessionResponse, 0, len(sessions))}
	for _, s := range sessions {
		item := coachSessionResponse(s)
		if day, _ := h.attachImportedDay(r, s.TemplateID, s.Week, s.Day); day != nil {
			item.ProgramDay = day
			if day.Label != "" {
				item.ProgramDayLabel = &day.Label
			}
		}
		resp.Sessions = append(resp.Sessions, item)
	}
	if hasMore && len(sessions) > 0 {
		cursor := encodeCoachCursor(sessions[len(sessions)-1].ID)
		resp.NextCursor = &cursor
	}
	WriteJSON(w, http.StatusOK, resp)
}

// GetCoachWeek returns the week's planned sessions against what was logged.
//
//	@Summary      Coach week plan versus done
//	@Description  Planned weekdays from stored program schedules. week_start defaults to the current Monday in the caller's timezone. Status is done, partial, missed, or upcoming. summary is done of planned, such as "2 of 3". This route does not create or change a session.
//	@Tags         Workouts
//	@Produce      json
//	@Param        id          path   int     true  "Athlete ID"
//	@Param        week_start  query  string  false "Monday of the week, YYYY-MM-DD"
//	@Success      200  {object}  api.CoachWeekResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/coach/week [get]
func (h *Handlers) GetCoachWeek(w http.ResponseWriter, r *http.Request) {
	_, athleteID, ok := h.coachReadAccess(w, r)
	if !ok {
		return
	}
	loc := userLocation(r)
	weekStart := r.URL.Query().Get("week_start")
	if weekStart == "" {
		weekStart = mondayOn(time.Now().In(loc))
	} else if !validDate(weekStart) || !isMonday(weekStart, loc) {
		WriteValidationError(w, "week_start", "must be a Monday in YYYY-MM-DD format")
		return
	}
	today := todayInUserTZ(r)
	plan, err := models.WeekPlanForAthlete(r.Context(), h.DB, athleteID, weekStart, today, loc)
	if err != nil {
		log.Printf("api: coach week athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to read the week")
		return
	}
	resp := CoachWeekResponse{
		WeekStart:     plan.WeekStart,
		WeekEnd:       plan.WeekEnd,
		Timezone:      loc.String(),
		AthleteID:     plan.AthleteID,
		PlannedCount:  plan.Planned,
		DoneCount:     plan.Done,
		PartialCount:  plan.Partial,
		MissedCount:   plan.Missed,
		UpcomingCount: plan.Upcoming,
		Summary:       strconv.Itoa(plan.Done) + " of " + strconv.Itoa(plan.Planned),
		Days:          make([]CoachWeekDayResponse, 0, len(plan.Days)),
	}
	for _, day := range plan.Days {
		item := CoachWeekDayResponse{
			Date:             day.Date,
			Weekday:          day.Weekday,
			ProgramID:        day.ProgramID,
			ProgramName:      day.ProgramName,
			ProgramDayLabel:  emptyStringPtr(day.ProgramDayLabel),
			SessionID:        nullInt64(day.SessionID),
			Status:           day.Status,
			CompletedOn:      emptyStringPtr(day.CompletedOn),
			PlannedExercises: day.PlannedExercises,
			LoggedExercises:  day.LoggedExercises,
		}
		if day.Week > 0 {
			week, d := day.Week, day.Day
			item.Week = &week
			item.Day = &d
			if prog, _ := h.attachImportedDay(r, day.TemplateID, day.Week, day.Day); prog != nil {
				item.ProgramDay = prog
				if prog.Label != "" {
					item.ProgramDayLabel = &prog.Label
				}
			}
		}
		resp.Days = append(resp.Days, item)
	}
	WriteJSON(w, http.StatusOK, resp)
}

// GetCoachProgress returns per-exercise estimated 1RM, PRs, and the last top set.
//
//	@Summary      Coach exercise progress
//	@Description  Read only. Estimated 1RM is computed with the Epley formula already used for featured lifts and is null when that formula does not apply. It is not a stored column. Loads are pounds.
//	@Tags         Workouts
//	@Produce      json
//	@Param        id        path   int     true   "Athlete ID"
//	@Param        exercise  query  int     false  "Exercise ID"
//	@Param        start     query  string  false  "Range start, YYYY-MM-DD"
//	@Param        end       query  string  false  "Range end, YYYY-MM-DD"
//	@Success      200  {object}  api.CoachProgressResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/coach/progress [get]
func (h *Handlers) GetCoachProgress(w http.ResponseWriter, r *http.Request) {
	_, athleteID, ok := h.coachReadAccess(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	start, end := q.Get("start"), q.Get("end")
	if (start != "" && !validDate(start)) || (end != "" && !validDate(end)) || (start != "" && end != "" && start > end) {
		WriteValidationError(w, "start", "start and end must be YYYY-MM-DD dates, with start on or before end")
		return
	}
	exerciseID, ok := optionalPositiveID(w, q.Get("exercise"), "exercise")
	if !ok {
		return
	}
	rows, err := models.ListCoachProgress(r.Context(), h.DB, athleteID, exerciseID, start, end)
	if err != nil {
		log.Printf("api: coach progress athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to read progress")
		return
	}
	resp := CoachProgressResponse{
		AthleteID:  athleteID,
		Start:      emptyStringPtr(start),
		End:        emptyStringPtr(end),
		E1RMSource: "epley",
		Exercises:  make([]CoachProgressExerciseResponse, 0, len(rows)),
	}
	for _, row := range rows {
		item := CoachProgressExerciseResponse{
			ExerciseID: row.ExerciseID,
			Name:       row.Name,
			Sessions:   make([]CoachProgressPointResponse, 0, len(row.Sessions)),
			PRs:        make([]CoachPRResponse, 0, len(row.PRs)),
		}
		for _, point := range row.Sessions {
			item.Sessions = append(item.Sessions, CoachProgressPointResponse{
				SessionID: point.SessionID,
				Date:      point.Date,
				E1RM:      nullFloat(point.E1RM),
				TopSet:    topSetResponse(point),
			})
		}
		for _, pr := range row.PRs {
			weight := sql.NullFloat64{Float64: pr.Weight, Valid: true}
			item.PRs = append(item.PRs, CoachPRResponse{
				SessionID: pr.SessionID,
				Date:      pr.Date,
				E1RM:      pr.E1RM,
				Reps:      pr.Reps,
				Load:      coachLoad(weight),
			})
		}
		if row.LastTopSet != nil {
			last := topSetResponse(*row.LastTopSet)
			item.LastSessionTopSet = &last
		}
		resp.Exercises = append(resp.Exercises, item)
	}
	WriteJSON(w, http.StatusOK, resp)
}

// GetCoachPosition returns the current program position and the next session.
//
//	@Summary      Coach program position
//	@Description  Read only. Does not create or advance a session, and does not change the program or a training max. block stays null: it is a mesocycle name, and that column is not stored. program_day is the imported day when this assignment has one. Week and day come from the existing prescription count.
//	@Tags         Workouts
//	@Produce      json
//	@Param        id  path  int  true  "Athlete ID"
//	@Success      200  {object}  api.CoachPositionResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/coach/position [get]
func (h *Handlers) GetCoachPosition(w http.ResponseWriter, r *http.Request) {
	_, athleteID, ok := h.coachReadAccess(w, r)
	if !ok {
		return
	}
	loc := userLocation(r)
	pos, err := models.CoachProgramPosition(r.Context(), h.DB, athleteID, todayInUserTZ(r), loc)
	if err != nil {
		log.Printf("api: coach position athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to read program position")
		return
	}
	resp := CoachPositionResponse{AthleteID: athleteID, CycleComplete: pos.CycleComplete}
	if pos.HasProgram {
		resp.ProgramID = &pos.ProgramID
		resp.ProgramName = &pos.ProgramName
		cycle, week, day := pos.CycleNumber, pos.Week, pos.Day
		resp.CycleNumber = &cycle
		resp.Week = &week
		resp.Day = &day
		resp.ProgramDayLabel = emptyStringPtr(pos.ProgramDayLabel)
		if prog, _ := h.attachImportedDay(r, pos.TemplateID, pos.Week, pos.Day); prog != nil {
			resp.ProgramDay = prog
			if prog.Label != "" {
				resp.ProgramDayLabel = &prog.Label
			}
		}
	}
	if pos.Next != nil {
		next := &CoachNextSessionResponse{
			Date:            pos.Next.Date,
			ProgramDayLabel: pos.Next.ProgramDayLabel,
			Week:            pos.Next.Week,
			Day:             pos.Next.Day,
			Exercises:       make([]CoachTargetExerciseResponse, 0, len(pos.Next.Exercises)),
		}
		for _, ex := range pos.Next.Exercises {
			item := CoachTargetExerciseResponse{ExerciseID: ex.ExerciseID, Name: ex.Name, Sets: []CoachTargetSetResponse{}}
			for _, set := range ex.Sets {
				var target sql.NullFloat64
				if set.Target != nil {
					target = sql.NullFloat64{Float64: *set.Target, Valid: true}
				}
				item.Sets = append(item.Sets, CoachTargetSetResponse{
					SetNumber:  set.SetNumber,
					Reps:       nullInt(set.Reps),
					Load:       coachLoad(target),
					Percentage: nullFloat(set.Percentage),
					RPE:        nullFloat(set.RPE),
				})
			}
			next.Exercises = append(next.Exercises, item)
		}
		if prog, _ := h.attachImportedDay(r, pos.Next.TemplateID, pos.Next.Week, pos.Next.Day); prog != nil {
			next.ProgramDay = prog
			if prog.Label != "" {
				next.ProgramDayLabel = prog.Label
			}
			overlayPrescriptions(next.Exercises, prog)
		}
		resp.NextSession = next
	}
	WriteJSON(w, http.StatusOK, resp)
}

// GetCoachLastSession returns the most recent sets for one exercise.
//
//	@Summary      Last session for an exercise
//	@Description  The newest resistance session in which this athlete logged the exercise, and only that exercise's sets. session is null when it has not been logged. started_at, rir, and set_type are null because those columns are not stored.
//	@Tags         Workouts
//	@Produce      json
//	@Param        id          path  int  true  "Athlete ID"
//	@Param        exerciseID  path  int  true  "Exercise ID"
//	@Success      200  {object}  api.CoachLastSessionResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/coach/exercises/{exerciseID}/last-session [get]
func (h *Handlers) GetCoachLastSession(w http.ResponseWriter, r *http.Request) {
	_, athleteID, ok := h.coachReadAccess(w, r)
	if !ok {
		return
	}
	exerciseID, err := strconv.ParseInt(r.PathValue("exerciseID"), 10, 64)
	if err != nil || exerciseID <= 0 {
		WriteValidationError(w, "exerciseID", "must be a positive exercise id")
		return
	}
	exercise, err := models.GetExerciseByID(r.Context(), h.DB, exerciseID)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "exercise not found")
		return
	}
	if err != nil {
		log.Printf("api: coach last session exercise %d: %v", exerciseID, err)
		WriteError(w, http.StatusInternalServerError, "failed to read the last session")
		return
	}
	session, err := models.LastCoachSession(r.Context(), h.DB, athleteID, exerciseID)
	if err != nil {
		log.Printf("api: coach last session athlete %d exercise %d: %v", athleteID, exerciseID, err)
		WriteError(w, http.StatusInternalServerError, "failed to read the last session")
		return
	}
	resp := CoachLastSessionResponse{AthleteID: athleteID, ExerciseID: exerciseID, Name: exercise.Name}
	if session != nil {
		body := &CoachLastSessionBody{
			SessionID: session.SessionID,
			Date:      session.Date,
			Sets:      make([]CoachLoggedSetResponse, 0, len(session.Sets)),
		}
		for _, set := range session.Sets {
			body.Sets = append(body.Sets, coachSetResponse(set))
		}
		resp.Session = body
	}
	WriteJSON(w, http.StatusOK, resp)
}

// PreviewCoachSessionNote checks a session note and returns a one-time token.
//
//	@Summary      Preview a session note
//	@Description  Does not write. Returns a confirm_token bound to this coach, session, and the exact note. The token works once, for 15 minutes. Confirm writes only workouts.notes.
//	@Tags         Workouts
//	@Accept       json
//	@Produce      json
//	@Param        id         path  int  true  "Athlete ID"
//	@Param        sessionID  path  int  true  "Session ID"
//	@Param        body       body  api.CoachNotePreviewRequest  true  "Note to preview"
//	@Success      200  {object}  api.CoachNotePreviewResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/coach/sessions/{sessionID}/notes/preview [post]
func (h *Handlers) PreviewCoachSessionNote(w http.ResponseWriter, r *http.Request) {
	user, athleteID, sessionID, ok := h.coachNoteSession(w, r)
	if !ok {
		return
	}
	notes, ok := coachNoteText(w, r)
	if !ok {
		return
	}
	token, expires, err := h.issueNoteGrant(user.ID, athleteID, sessionID, notes, time.Now())
	if err != nil {
		log.Printf("api: preview session note athlete %d session %d: %v", athleteID, sessionID, err)
		WriteError(w, http.StatusInternalServerError, "failed to preview the note")
		return
	}
	WriteJSON(w, http.StatusOK, CoachNotePreviewResponse{
		WouldWrite:   CoachNoteWrite{AthleteID: athleteID, SessionID: sessionID, Notes: notes},
		ConfirmToken: token,
		ExpiresAt:    expires.In(userLocation(r)).Format(time.RFC3339),
	})
}

// ConfirmCoachSessionNote writes the previewed session note.
//
//	@Summary      Confirm a session note
//	@Description  Writes only the note from the preview, on a session this coach owns. The confirm_token works once. A different note is refused and does not consume the token. This route does not change the program, training maxes, or sets.
//	@Tags         Workouts
//	@Accept       json
//	@Produce      json
//	@Param        id         path  int  true  "Athlete ID"
//	@Param        sessionID  path  int  true  "Session ID"
//	@Param        body       body  api.CoachNoteConfirmRequest  true  "Previewed note and token"
//	@Success      200  {object}  api.CoachNoteConfirmResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Failure      409  {object}  api.APIError
//	@Router       /athletes/{id}/coach/sessions/{sessionID}/notes/confirm [post]
func (h *Handlers) ConfirmCoachSessionNote(w http.ResponseWriter, r *http.Request) {
	user, athleteID, sessionID, ok := h.coachNoteSession(w, r)
	if !ok {
		return
	}
	var req CoachNoteConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ConfirmToken == "" {
		WriteValidationError(w, "confirm_token", "is required")
		return
	}
	if !validCoachNote(req.Notes) {
		WriteValidationError(w, "notes", "must be text of at most 4000 characters")
		return
	}
	switch h.consumeNoteGrant(user.ID, athleteID, sessionID, req.Notes, req.ConfirmToken, time.Now()) {
	case noteGrantOK:
	case noteGrantMismatch:
		WriteError(w, http.StatusConflict, "note does not match the preview")
		return
	default:
		WriteValidationError(w, "confirm_token", "is invalid or expired")
		return
	}
	if err := models.UpdateWorkoutNotes(r.Context(), h.DB, sessionID, athleteID, req.Notes); err != nil {
		log.Printf("api: confirm session note athlete %d session %d: %v", athleteID, sessionID, err)
		WriteError(w, http.StatusInternalServerError, "failed to save the note")
		return
	}
	WriteJSON(w, http.StatusOK, CoachNoteConfirmResponse{
		AthleteID: athleteID,
		SessionID: sessionID,
		Notes:     emptyStringPtr(req.Notes),
	})
}

// CoachNoteConfirmRequest is the confirm body. Notes must match the preview.
type CoachNoteConfirmRequest struct {
	Notes        string `json:"notes"`
	ConfirmToken string `json:"confirm_token"`
}

func (h *Handlers) coachReadAccess(w http.ResponseWriter, r *http.Request) (*models.User, int64, bool) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || athleteID <= 0 {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return nil, 0, false
	}
	if middleware.IsServiceCredential(r.Context()) {
		if !middleware.ServiceCanAccessAthlete(r.Context(), h.DB, user, athleteID) {
			WriteError(w, http.StatusForbidden, "access denied")
			return nil, 0, false
		}
		return user, athleteID, true
	}
	if !middleware.CanAccessAthlete(r.Context(), h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return nil, 0, false
	}
	return user, athleteID, true
}

// coachNoteSession authorizes a note write: the caller must coach this
// athlete, and the session must belong to that athlete.
func (h *Handlers) coachNoteSession(w http.ResponseWriter, r *http.Request) (*models.User, int64, int64, bool) {
	user, athleteID, ok := h.coachReadAccess(w, r)
	if !ok {
		return nil, 0, 0, false
	}
	athlete, err := models.GetAthleteByID(r.Context(), h.DB, athleteID)
	if err != nil || !athlete.CoachID.Valid || athlete.CoachID.Int64 != user.ID {
		WriteError(w, http.StatusForbidden, "coach access required")
		return nil, 0, 0, false
	}
	sessionID, err := strconv.ParseInt(r.PathValue("sessionID"), 10, 64)
	if err != nil || sessionID <= 0 {
		WriteError(w, http.StatusBadRequest, "invalid session ID")
		return nil, 0, 0, false
	}
	workout, err := models.GetWorkoutByID(r.Context(), h.DB, sessionID)
	if errors.Is(err, models.ErrNotFound) || (err == nil && workout.AthleteID != athleteID) {
		WriteError(w, http.StatusNotFound, "session not found")
		return nil, 0, 0, false
	}
	if err != nil {
		log.Printf("api: coach note session %d: %v", sessionID, err)
		WriteError(w, http.StatusInternalServerError, "failed to read the session")
		return nil, 0, 0, false
	}
	return user, athleteID, sessionID, true
}

// CoachNotePreviewRequest is the preview body. notes is required and may be empty.
type CoachNotePreviewRequest struct {
	Notes *string `json:"notes"`
}

func coachNoteText(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req CoachNotePreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return "", false
	}
	if req.Notes == nil {
		WriteValidationError(w, "notes", "is required")
		return "", false
	}
	if !validCoachNote(*req.Notes) {
		WriteValidationError(w, "notes", "must be text of at most 4000 characters")
		return "", false
	}
	return *req.Notes, true
}

func validCoachNote(notes string) bool {
	n := 0
	for range notes {
		n++
		if n > coachNoteMaxRunes {
			return false
		}
	}
	return true
}

func optionalPositiveID(w http.ResponseWriter, raw, field string) (int64, bool) {
	if raw == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		WriteValidationError(w, field, "must be a positive id")
		return 0, false
	}
	return id, true
}

func mondayOn(now time.Time) string {
	wd := now.Weekday()
	offset := int(wd - time.Monday)
	if wd == time.Sunday {
		offset = 6
	}
	monday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -offset)
	return monday.Format(dateLayout)
}

func isMonday(date string, loc *time.Location) bool {
	t, err := time.ParseInLocation(dateLayout, date, loc)
	if err != nil {
		return false
	}
	return t.Weekday() == time.Monday
}

func coachLoad(weight sql.NullFloat64) CoachLoad {
	load := CoachLoad{Unit: coachLoadUnit}
	if weight.Valid {
		value := weight.Float64
		load.Value = &value
	}
	return load
}

func coachSetResponse(set models.CoachLoggedSet) CoachLoggedSetResponse {
	return CoachLoggedSetResponse{
		SetID:     set.ID,
		SetNumber: set.SetNumber,
		Reps:      set.Reps,
		Load:      coachLoad(set.Weight),
		RPE:       nullFloat(set.RPE),
		SetType:   strPtrOrNil(set.SetType),
		Category:  set.Category,
		Notes:     nullStr(set.Notes),
	}
}

// strPtrOrNil returns nil for "" and a pointer to s otherwise.
func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func coachSessionResponse(s models.CoachSession) CoachSessionResponse {
	resp := CoachSessionResponse{
		SessionID:       s.ID,
		AthleteID:       s.AthleteID,
		Date:            fmtDate(s.Date),
		ProgramDayLabel: emptyStringPtr(s.ProgramDayLabel),
		Notes:           nullStr(s.Notes),
		Exercises:       make([]CoachLoggedExerciseResponse, 0, len(s.Exercises)),
	}
	for _, ex := range s.Exercises {
		item := CoachLoggedExerciseResponse{
			ExerciseID: ex.ExerciseID,
			Name:       ex.Name,
			Sets:       make([]CoachLoggedSetResponse, 0, len(ex.Sets)),
		}
		for _, set := range ex.Sets {
			item.Sets = append(item.Sets, coachSetResponse(set))
		}
		resp.Exercises = append(resp.Exercises, item)
	}
	return resp
}

func topSetResponse(point models.CoachProgressPoint) CoachTopSetResponse {
	return CoachTopSetResponse{
		SessionID: point.SessionID,
		Date:      point.Date,
		Reps:      point.Reps,
		Load:      coachLoad(point.Weight),
		E1RM:      nullFloat(point.E1RM),
		RPE:       nullFloat(point.RPE),
	}
}

func emptyStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullInt64(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func encodeCoachCursor(id int64) string {
	return strconv.FormatInt(id, 10)
}

func decodeCoachCursor(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid cursor")
	}
	return id, nil
}

type noteGrant struct {
	userID    int64
	athleteID int64
	sessionID int64
	notes     string
	expires   time.Time
}

type noteGrantStore struct {
	mu     sync.Mutex
	byHash map[string]noteGrant
}

type noteGrantResult int

const (
	noteGrantMissing noteGrantResult = iota
	noteGrantMismatch
	noteGrantOK
)

func (h *Handlers) grants() *noteGrantStore {
	h.noteGrantOnce.Do(func() {
		h.noteGrantStore = &noteGrantStore{byHash: map[string]noteGrant{}}
	})
	return h.noteGrantStore
}

func (h *Handlers) issueNoteGrant(userID, athleteID, sessionID int64, notes string, now time.Time) (string, time.Time, error) {
	token, key, err := models.NewConfirmToken(noteConfirmPrefix)
	if err != nil {
		return "", time.Time{}, err
	}
	expires := now.Add(noteConfirmTTL)
	store := h.grants()
	store.mu.Lock()
	defer store.mu.Unlock()
	store.byHash[key] = noteGrant{
		userID: userID, athleteID: athleteID, sessionID: sessionID, notes: notes, expires: expires,
	}
	return token, expires, nil
}

func (h *Handlers) consumeNoteGrant(userID, athleteID, sessionID int64, notes, token string, now time.Time) noteGrantResult {
	key := models.HashConfirmToken(token)
	store := h.grants()
	store.mu.Lock()
	defer store.mu.Unlock()
	grant, ok := store.byHash[key]
	if !ok || !grant.expires.After(now) {
		delete(store.byHash, key)
		return noteGrantMissing
	}
	if grant.userID != userID || grant.athleteID != athleteID || grant.sessionID != sessionID || grant.notes != notes {
		return noteGrantMismatch
	}
	delete(store.byHash, key)
	return noteGrantOK
}
