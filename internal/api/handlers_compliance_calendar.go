package api

import (
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// ComplianceDayResponse is one athlete-day cell. status is rest (nothing
// planned), prescribed (planned, not yet due), done, partial, skipped
// (planned day passed with no session), or extra (logged on an unplanned
// day).
type ComplianceDayResponse struct {
	Date            string  `json:"date" example:"2026-11-16"`
	Weekday         int     `json:"weekday" example:"1"`
	Status          string  `json:"status" example:"done"`
	SessionID       *int64  `json:"session_id"`
	ProgramName     *string `json:"program_name"`
	ProgramDayLabel *string `json:"program_day_label"`
}

// ComplianceAthleteResponse is one athlete's row: seven days, Monday first.
type ComplianceAthleteResponse struct {
	AthleteID       int64                   `json:"athlete_id"`
	Name            string                  `json:"name"`
	PlannedCount    int                     `json:"planned_count"`
	DoneCount       int                     `json:"done_count"`
	PartialCount    int                     `json:"partial_count"`
	SkippedCount    int                     `json:"skipped_count"`
	PrescribedCount int                     `json:"prescribed_count"`
	ExtraCount      int                     `json:"extra_count"`
	Summary         string                  `json:"summary" example:"2 of 3"`
	Days            []ComplianceDayResponse `json:"days"`
}

// ComplianceResponse is the coach compliance grid for one week.
type ComplianceResponse struct {
	WeekStart string                      `json:"week_start"`
	WeekEnd   string                      `json:"week_end"`
	Timezone  string                      `json:"timezone"`
	Athletes  []ComplianceAthleteResponse `json:"athletes"`
}

// GetCoachCompliance returns the coach's athletes by day for one week.
//
//	@Summary      Coach compliance grid
//	@Description  Each coached athlete's Monday–Sunday from the week plan-versus-done read. Admins see every athlete. week_start defaults to the current Monday in the caller's timezone. Read only: it never changes a plan.
//	@Tags         Workouts
//	@Produce      json
//	@Param        week_start  query  string  false  "Monday of the week, YYYY-MM-DD"
//	@Success      200  {object}  api.ComplianceResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /coach/compliance [get]
func (h *Handlers) GetCoachCompliance(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil || (!user.IsCoach && !user.IsAdmin) {
		WriteError(w, http.StatusForbidden, "coach access required")
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
	athletes, err := models.ListAthletes(r.Context(), h.DB, coachFilterForRequest(r, user))
	if err != nil {
		log.Printf("api: compliance athletes for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list athletes")
		return
	}
	today := todayInUserTZ(r)
	start, _ := time.ParseInLocation(dateLayout, weekStart, loc)
	resp := ComplianceResponse{
		WeekStart: weekStart,
		WeekEnd:   start.AddDate(0, 0, 6).Format(dateLayout),
		Timezone:  loc.String(),
		Athletes:  make([]ComplianceAthleteResponse, 0, len(athletes)),
	}
	for _, a := range athletes {
		row, err := models.ComplianceWeek(r.Context(), h.DB, a.ID, a.Name, weekStart, today, loc)
		if err != nil {
			log.Printf("api: compliance athlete %d: %v", a.ID, err)
			WriteError(w, http.StatusInternalServerError, "failed to read the week")
			return
		}
		resp.Athletes = append(resp.Athletes, complianceRowResponse(row))
	}
	WriteJSON(w, http.StatusOK, resp)
}

func complianceRowResponse(row *models.ComplianceRow) ComplianceAthleteResponse {
	out := ComplianceAthleteResponse{
		AthleteID:       row.AthleteID,
		Name:            row.Name,
		PlannedCount:    row.Plan.Planned,
		DoneCount:       row.Plan.Done,
		PartialCount:    row.Plan.Partial,
		SkippedCount:    row.Plan.Missed,
		PrescribedCount: row.Plan.Upcoming,
		ExtraCount:      row.Extra,
		Summary:         strconv.Itoa(row.Plan.Done) + " of " + strconv.Itoa(row.Plan.Planned),
		Days:            make([]ComplianceDayResponse, len(row.Days)),
	}
	for i, d := range row.Days {
		out.Days[i] = ComplianceDayResponse{
			Date:            d.Date,
			Weekday:         d.Weekday,
			Status:          d.Status,
			SessionID:       nullInt64(d.SessionID),
			ProgramName:     emptyStringPtr(d.ProgramName),
			ProgramDayLabel: emptyStringPtr(d.ProgramDayLabel),
		}
	}
	return out
}

// CalendarSessionResponse is one logged session on a calendar day.
type CalendarSessionResponse struct {
	WorkoutID  int64   `json:"workout_id"`
	Discipline string  `json:"discipline" example:"throwing"`
	Label      *string `json:"label" example:"bullpen"`
	SetCount   int     `json:"set_count"`
}

// CalendarDayResponse is one day. scheduled is true when a stored program
// schedule includes the weekday; status is then done, partial, missed, or
// upcoming, and null otherwise.
type CalendarDayResponse struct {
	Date        string                    `json:"date"`
	Scheduled   bool                      `json:"scheduled"`
	ProgramID   *int64                    `json:"program_id"`
	ProgramName *string                   `json:"program_name"`
	Status      *string                   `json:"status"`
	Sessions    []CalendarSessionResponse `json:"sessions"`
}

// CalendarPhaseResponse is a season phase overlapping the month.
// end_date null means the phase is ongoing.
type CalendarPhaseResponse struct {
	ID        int64   `json:"id"`
	Sport     *string `json:"sport"`
	Phase     string  `json:"phase" example:"in"`
	StartDate string  `json:"start_date"`
	EndDate   *string `json:"end_date"`
}

// CalendarResponse is one month of an athlete's calendar.
type CalendarResponse struct {
	Month  string                  `json:"month" example:"2026-11"`
	Start  string                  `json:"start"`
	End    string                  `json:"end"`
	Today  string                  `json:"today"`
	Days   []CalendarDayResponse   `json:"days"`
	Phases []CalendarPhaseResponse `json:"phases"`
}

var monthPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// GetAthleteCalendar returns a month of scheduled days, sessions and phases.
//
//	@Summary      Athlete calendar month
//	@Description  Every day of the month with its scheduled program day (from stored weekday schedules), each logged session of any discipline, and the season phases that overlap the month. month defaults to the current month in the caller's timezone. Read only.
//	@Tags         Workouts
//	@Produce      json
//	@Param        id     path   int     true   "Athlete ID"
//	@Param        month  query  string  false  "Month, YYYY-MM"
//	@Success      200  {object}  api.CalendarResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/calendar [get]
func (h *Handlers) GetAthleteCalendar(w http.ResponseWriter, r *http.Request) {
	_, athleteID, ok := h.coachReadAccess(w, r)
	if !ok {
		return
	}
	loc := userLocation(r)
	today := todayInUserTZ(r)
	month := r.URL.Query().Get("month")
	if month == "" {
		month = today[:7]
	} else if !monthPattern.MatchString(month) {
		WriteValidationError(w, "month", "must be YYYY-MM")
		return
	}
	first, _ := time.ParseInLocation(dateLayout, month+"-01", loc)
	last := first.AddDate(0, 1, -1)
	cal, err := models.AthleteCalendar(r.Context(), h.DB, athleteID, first.Format(dateLayout), last.Format(dateLayout), today, loc)
	if err != nil {
		log.Printf("api: calendar athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to read the calendar")
		return
	}
	resp := CalendarResponse{
		Month:  month,
		Start:  cal.Start,
		End:    cal.End,
		Today:  today,
		Days:   make([]CalendarDayResponse, len(cal.Days)),
		Phases: make([]CalendarPhaseResponse, 0, len(cal.Phases)),
	}
	for i, d := range cal.Days {
		day := CalendarDayResponse{
			Date:      d.Date,
			Scheduled: d.Scheduled,
			Status:    emptyStringPtr(d.Status),
			Sessions:  make([]CalendarSessionResponse, len(d.Sessions)),
		}
		if d.Scheduled {
			id := d.ProgramID
			day.ProgramID = &id
			day.ProgramName = emptyStringPtr(d.ProgramName)
		}
		for j, s := range d.Sessions {
			day.Sessions[j] = CalendarSessionResponse{
				WorkoutID:  s.WorkoutID,
				Discipline: s.Discipline,
				Label:      emptyStringPtr(s.Label),
				SetCount:   s.SetCount,
			}
		}
		resp.Days[i] = day
	}
	for _, p := range cal.Phases {
		phase := CalendarPhaseResponse{ID: p.ID, Phase: p.Phase, StartDate: isoDateString(p.StartDate)}
		if p.Sport.Valid && p.Sport.String != "" {
			s := p.Sport.String
			phase.Sport = &s
		}
		if p.EndDate.Valid && p.EndDate.String != "" {
			e := isoDateString(p.EndDate.String)
			phase.EndDate = &e
		}
		resp.Phases = append(resp.Phases, phase)
	}
	WriteJSON(w, http.StatusOK, resp)
}

// isoDateString trims a stored date or timestamp to YYYY-MM-DD.
func isoDateString(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
