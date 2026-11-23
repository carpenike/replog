package api

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Whiteboard / TV mode (gap 15) ---
//
// A read-only, large-type view of today's session for one or more
// athletes, for a garage TV or tablet: what each athlete has prescribed and
// what they have logged so far. It writes nothing and never changes a plan.

// whiteboardMaxAthletes caps one board.
const whiteboardMaxAthletes = 8

// WhiteboardResponse is GET /api/whiteboard.
type WhiteboardResponse struct {
	// Date is today (YYYY-MM-DD) in the caller's timezone.
	Date     string               `json:"date"`
	Athletes []*WhiteboardAthlete `json:"athletes"`
	// Available lists every athlete the caller can put on the board, for
	// the picker.
	Available []WhiteboardAthleteRef `json:"available"`
	// Truncated is true when the default board had more athletes than fit.
	Truncated bool `json:"truncated"`
}

// WhiteboardAthleteRef names an athlete the caller can show.
type WhiteboardAthleteRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// WhiteboardAthlete is one athlete's column on the board.
type WhiteboardAthlete struct {
	AthleteID   int64   `json:"athlete_id"`
	Name        string  `json:"name"`
	Tier        *string `json:"tier,omitempty"`
	ProgramName *string `json:"program_name,omitempty"`
	CurrentWeek int     `json:"current_week,omitempty"`
	CurrentDay  int     `json:"current_day,omitempty"`
	// WorkoutID is today's resistance workout, when one has been started.
	WorkoutID *int64            `json:"workout_id,omitempty"`
	Lines     []*WhiteboardLine `json:"lines"`
	// WorkingTarget / WorkingDone count prescribed and logged working sets
	// across the day (warm-ups excluded on both sides).
	WorkingTarget int `json:"working_target"`
	WorkingDone   int `json:"working_done"`
}

// WhiteboardLine is one exercise: its prescription (empty for an exercise
// that was logged but not prescribed) and the sets logged so far.
type WhiteboardLine struct {
	ExerciseID    int64                     `json:"exercise_id"`
	ExerciseName  string                    `json:"exercise_name"`
	GroupKey      *string                   `json:"group_key,omitempty"`
	Prescribed    []PrescriptionSetResponse `json:"prescribed"`
	Logged        []*WorkoutSet             `json:"logged"`
	WorkingTarget int                       `json:"working_target"`
	WorkingDone   int                       `json:"working_done"`
}

// GetWhiteboard returns today's board.
//
//	@Summary      Whiteboard (TV mode)
//	@Description  Read-only view of today's session for one or more athletes: each athlete's prescribed lines (from their active program) and the sets logged so far today. With no athletes parameter the board shows the caller's own athlete, or for a coach every athlete they can see (up to 8). Each named athlete must be visible to the caller. Nothing is written.
//	@Tags         Athletes
//	@Produce      json
//	@Param        athletes  query     string  false  "Comma-separated athlete IDs (at most 8)"
//	@Success      200  {object}  api.WhiteboardResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /whiteboard [get]
func (h *Handlers) GetWhiteboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	available, err := h.whiteboardAvailable(r, user)
	if err != nil {
		log.Printf("api: whiteboard athletes: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to load athletes")
		return
	}
	names := map[int64]string{}
	for _, a := range available {
		names[a.ID] = a.Name
	}

	resp := WhiteboardResponse{Date: todayInUserTZ(r), Available: available, Athletes: []*WhiteboardAthlete{}}

	var ids []int64
	if raw := strings.TrimSpace(r.URL.Query().Get("athletes")); raw != "" {
		seen := map[int64]bool{}
		for _, part := range strings.Split(raw, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if err != nil || id <= 0 {
				WriteValidationError(w, "athletes", "must be a comma-separated list of athlete IDs")
				return
			}
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
		if len(ids) > whiteboardMaxAthletes {
			WriteValidationError(w, "athletes", "at most 8 athletes fit on the board")
			return
		}
		for _, id := range ids {
			if !h.canSeeAthlete(r, user, id) {
				WriteError(w, http.StatusForbidden, "access denied")
				return
			}
		}
	} else {
		for _, a := range available {
			if len(ids) == whiteboardMaxAthletes {
				resp.Truncated = true
				break
			}
			ids = append(ids, a.ID)
		}
	}

	loc := userLocation(r)
	now := time.Now().In(loc)
	for _, id := range ids {
		col, err := h.whiteboardColumn(r, id, now, loc.String(), resp.Date)
		if errors.Is(err, models.ErrNotFound) {
			continue
		}
		if err != nil {
			log.Printf("api: whiteboard athlete %d: %v", id, err)
			WriteError(w, http.StatusInternalServerError, "failed to build whiteboard")
			return
		}
		if col.Name == "" {
			col.Name = names[id]
		}
		resp.Athletes = append(resp.Athletes, col)
	}
	WriteJSON(w, http.StatusOK, resp)
}

// canSeeAthlete is athleteAccess without a path parameter.
func (h *Handlers) canSeeAthlete(r *http.Request, user *models.User, athleteID int64) bool {
	if middleware.IsServiceCredential(r.Context()) {
		return middleware.ServiceCanAccessAthlete(r.Context(), h.DB, user, athleteID)
	}
	return middleware.CanAccessAthlete(r.Context(), h.DB, user, athleteID)
}

// whiteboardAvailable lists the athletes the caller can show: a coach's (or
// admin's) visible athletes, plus the caller's own linked athlete.
func (h *Handlers) whiteboardAvailable(r *http.Request, user *models.User) ([]WhiteboardAthleteRef, error) {
	out := []WhiteboardAthleteRef{}
	seen := map[int64]bool{}
	if user.AthleteID.Valid {
		a, err := models.GetAthleteByID(r.Context(), h.DB, user.AthleteID.Int64)
		if err == nil {
			out = append(out, WhiteboardAthleteRef{ID: a.ID, Name: a.Name})
			seen[a.ID] = true
		} else if !errors.Is(err, models.ErrNotFound) {
			return nil, err
		}
	}
	if user.IsCoach || user.IsAdmin {
		cards, err := models.ListAthleteCards(r.Context(), h.DB, coachFilterForRequest(r, user))
		if err != nil {
			return nil, err
		}
		for _, c := range cards {
			if !seen[c.ID] {
				out = append(out, WhiteboardAthleteRef{ID: c.ID, Name: c.Name})
				seen[c.ID] = true
			}
		}
	}
	return out, nil
}

// whiteboardColumn builds one athlete's column for date.
func (h *Handlers) whiteboardColumn(r *http.Request, athleteID int64, now time.Time, tz, date string) (*WhiteboardAthlete, error) {
	ctx := r.Context()
	athlete, err := models.GetAthleteByID(ctx, h.DB, athleteID)
	if err != nil {
		return nil, err
	}
	col := &WhiteboardAthlete{AthleteID: athlete.ID, Name: athlete.Name, Tier: nullStr(athlete.Tier), Lines: []*WhiteboardLine{}}

	byExercise := map[int64]*WhiteboardLine{}
	program, err := models.ResolveAssignment(ctx, h.DB, athleteID, now, tz)
	if err != nil {
		return nil, err
	}
	if program != nil {
		p, err := models.GetPrescription(ctx, h.DB, program, now)
		if err != nil {
			return nil, err
		}
		if p != nil {
			name := p.Program.TemplateName
			col.ProgramName = &name
			col.CurrentWeek, col.CurrentDay = p.CurrentWeek, p.CurrentDay
			for _, pl := range prescriptionLinesFromModel(p.Lines) {
				line := &WhiteboardLine{
					ExerciseID:   pl.ExerciseID,
					ExerciseName: pl.ExerciseName,
					GroupKey:     pl.GroupKey,
					Prescribed:   pl.Sets,
					Logged:       []*WorkoutSet{},
				}
				for _, s := range pl.Sets {
					if !s.NonWorking {
						line.WorkingTarget++
					}
				}
				col.Lines = append(col.Lines, line)
				byExercise[pl.ExerciseID] = line
			}
		}
	}

	workout, err := models.GetWorkoutByAthleteDate(ctx, h.DB, athleteID, date)
	switch {
	case errors.Is(err, models.ErrNotFound):
	case err != nil:
		return nil, err
	default:
		col.WorkoutID = &workout.ID
		groups, err := models.ListSetsByWorkout(ctx, h.DB, workout.ID)
		if err != nil {
			return nil, err
		}
		for _, g := range groups {
			line, ok := byExercise[g.ExerciseID]
			if !ok {
				line = &WhiteboardLine{
					ExerciseID:   g.ExerciseID,
					ExerciseName: g.ExerciseName,
					Prescribed:   []PrescriptionSetResponse{},
					Logged:       []*WorkoutSet{},
				}
				col.Lines = append(col.Lines, line)
				byExercise[g.ExerciseID] = line
			}
			for _, s := range g.Sets {
				line.Logged = append(line.Logged, WorkoutSetFromModel(s))
				if s.SetType != models.SetTypeWarmup {
					line.WorkingDone++
				}
			}
		}
	}
	for _, l := range col.Lines {
		col.WorkingTarget += l.WorkingTarget
		col.WorkingDone += l.WorkingDone
	}
	return col, nil
}
