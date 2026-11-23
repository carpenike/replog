package api

import (
	"errors"
	"log"
	"net/http"

	"github.com/carpenike/replog/internal/models"
)

// RecordSetResponse is one logged set that holds a record. weight is null
// for a bodyweight set.
type RecordSetResponse struct {
	WorkoutID int64    `json:"workout_id"`
	Date      string   `json:"date" example:"2026-11-21"`
	Reps      int      `json:"reps" example:"5"`
	Weight    *float64 `json:"weight,omitempty" example:"225"`
}

// RepMaxResponse is the heaviest load lifted for at least target reps.
type RepMaxResponse struct {
	Target int `json:"target" example:"5"`
	RecordSetResponse
}

// E1RMRecordResponse is the best estimated 1RM and the set it came from.
type E1RMRecordResponse struct {
	E1RM float64 `json:"e1rm" example:"262.5"`
	RecordSetResponse
}

// ExerciseRecordsResponse is one exercise's current personal records.
type ExerciseRecordsResponse struct {
	ExerciseID   int64               `json:"exercise_id"`
	Name         string              `json:"name"`
	SessionCount int                 `json:"session_count"`
	LastDate     string              `json:"last_date"`
	Heaviest     *RecordSetResponse  `json:"heaviest,omitempty"`
	BestE1RM     *E1RMRecordResponse `json:"best_e1rm,omitempty"`
	MostReps     *RecordSetResponse  `json:"most_reps,omitempty"`
	RepMaxes     []RepMaxResponse    `json:"rep_maxes"`
}

// RecordsResponse is GET /api/athletes/{id}/records.
type RecordsResponse struct {
	AthleteID   int64                     `json:"athlete_id"`
	RepTargets  []int                     `json:"rep_targets"`
	E1RMSource  string                    `json:"e1rm_source" example:"epley"`
	E1RMMaxReps int                       `json:"e1rm_max_reps" example:"12"`
	Exercises   []ExerciseRecordsResponse `json:"exercises"`
}

// MilestoneResponse is one achieved milestone.
type MilestoneResponse struct {
	Key        string `json:"key" example:"sessions_10"`
	Kind       string `json:"kind" enums:"tier,first,consistency"`
	Title      string `json:"title" example:"10 sessions"`
	Detail     string `json:"detail"`
	AchievedOn string `json:"achieved_on" example:"2026-11-21"`
}

// MilestoneGoalResponse is the next consistency milestone and progress toward it.
type MilestoneGoalResponse struct {
	Key     string `json:"key" example:"streak_4"`
	Title   string `json:"title" example:"4-week streak"`
	Current int    `json:"current" example:"3"`
	Target  int    `json:"target" example:"4"`
}

// MilestonesResponse is GET /api/athletes/{id}/milestones.
type MilestonesResponse struct {
	AthleteID     int64                   `json:"athlete_id"`
	TotalSessions int                     `json:"total_sessions"`
	CurrentStreak int                     `json:"current_streak"`
	BestStreak    int                     `json:"best_streak"`
	PRCount       int                     `json:"pr_count"`
	Achieved      []MilestoneResponse     `json:"achieved"`
	Next          []MilestoneGoalResponse `json:"next"`
}

func recordSetResponse(s models.RecordSet) RecordSetResponse {
	out := RecordSetResponse{WorkoutID: s.WorkoutID, Date: s.Date, Reps: s.Reps}
	if s.Weight > 0 {
		w := s.Weight
		out.Weight = &w
	}
	return out
}

// GetAthleteRecords returns an athlete's current personal records.
//
//	@Summary      Personal records board
//	@Description  Read only. Current records per exercise from the athlete's own logged sets: heaviest set, best estimated 1RM (Epley, sets of up to e1rm_max_reps reps), most reps in a set, and rep maxes (heaviest load for at least N reps). Warm-up, timed, and distance sets are excluded. Ties keep the earliest date. Loads are pounds.
//	@Tags         Workouts
//	@Produce      json
//	@Param        id   path      int  true  "Athlete ID"
//	@Success      200  {object}  api.RecordsResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/records [get]
func (h *Handlers) GetAthleteRecords(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	rows, err := models.ListAthleteRecords(r.Context(), h.DB, athleteID)
	if err != nil {
		log.Printf("api: records for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to read records")
		return
	}
	resp := RecordsResponse{
		AthleteID:   athleteID,
		RepTargets:  models.RecordRepTargets,
		E1RMSource:  "epley",
		E1RMMaxReps: models.RecordE1RMMaxReps,
		Exercises:   make([]ExerciseRecordsResponse, 0, len(rows)),
	}
	for _, row := range rows {
		item := ExerciseRecordsResponse{
			ExerciseID:   row.ExerciseID,
			Name:         row.Name,
			SessionCount: row.SessionCount,
			LastDate:     row.LastDate,
			RepMaxes:     make([]RepMaxResponse, 0, len(row.RepMaxes)),
		}
		if row.Heaviest != nil {
			s := recordSetResponse(*row.Heaviest)
			item.Heaviest = &s
		}
		if row.BestE1RM != nil {
			item.BestE1RM = &E1RMRecordResponse{E1RM: row.BestE1RM.E1RM, RecordSetResponse: recordSetResponse(row.BestE1RM.RecordSet)}
		}
		if row.MostReps != nil {
			s := recordSetResponse(*row.MostReps)
			item.MostReps = &s
		}
		for _, rm := range row.RepMaxes {
			item.RepMaxes = append(item.RepMaxes, RepMaxResponse{Target: rm.Target, RecordSetResponse: recordSetResponse(rm.RecordSet)})
		}
		resp.Exercises = append(resp.Exercises, item)
	}
	WriteJSON(w, http.StatusOK, resp)
}

// GetAthleteMilestones returns an athlete's milestones.
//
//	@Summary      Athlete milestones
//	@Description  Read only. Tier badges (from the coach's tier changes), firsts, and consistency milestones (session counts, week streaks, PR counts), computed only from this athlete's own history. Never compared with or ranked against other athletes. Nothing here changes a program or a tier. Streaks are Monday-based weeks in the viewer's time zone; the current week extends a streak but does not break it.
//	@Tags         Athletes
//	@Produce      json
//	@Param        id   path      int  true  "Athlete ID"
//	@Success      200  {object}  api.MilestonesResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/milestones [get]
func (h *Handlers) GetAthleteMilestones(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	m, err := models.AthleteMilestonesFor(r.Context(), h.DB, athleteID, todayInUserTZ(r))
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "athlete not found")
		return
	}
	if err != nil {
		log.Printf("api: milestones for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to read milestones")
		return
	}
	resp := MilestonesResponse{
		AthleteID:     athleteID,
		TotalSessions: m.TotalSessions,
		CurrentStreak: m.CurrentStreak,
		BestStreak:    m.BestStreak,
		PRCount:       m.PRCount,
		Achieved:      make([]MilestoneResponse, 0, len(m.Achieved)),
		Next:          make([]MilestoneGoalResponse, 0, len(m.Next)),
	}
	for _, a := range m.Achieved {
		resp.Achieved = append(resp.Achieved, MilestoneResponse{Key: a.Key, Kind: a.Kind, Title: a.Title, Detail: a.Detail, AchievedOn: a.AchievedOn})
	}
	for _, g := range m.Next {
		resp.Next = append(resp.Next, MilestoneGoalResponse{Key: g.Key, Title: g.Title, Current: g.Current, Target: g.Target})
	}
	WriteJSON(w, http.StatusOK, resp)
}
