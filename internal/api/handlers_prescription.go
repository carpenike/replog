package api

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// PrescriptionLineResponse is a single exercise's prescription for today.
type PrescriptionLineResponse struct {
	ExerciseName string   `json:"exercise_name"`
	ExerciseID   int64    `json:"exercise_id"`
	TrainingMax  *float64 `json:"training_max,omitempty"`
	// GroupKey is the superset/circuit group (A, B, ...) this exercise
	// belongs to; lines that share a key are done as one linked group.
	GroupKey *string                   `json:"group_key,omitempty"`
	Sets     []PrescriptionSetResponse `json:"sets"`
}

// PrescriptionSetResponse is a single prescribed set with resolved target weight.
type PrescriptionSetResponse struct {
	SetNumber      int      `json:"set_number"`
	Reps           *int64   `json:"reps"`
	Percentage     *float64 `json:"percentage,omitempty"`
	TargetWeight   *float64 `json:"target_weight,omitempty"`
	AbsoluteWeight *float64 `json:"absolute_weight,omitempty"`
	RestSeconds    *int64   `json:"rest_seconds,omitempty"`
	RepType        string   `json:"rep_type"`
	Notes          *string  `json:"notes,omitempty"`
	// RepsMax is the top of a rep range; reps is the bottom ("8–12").
	RepsMax       *int64   `json:"reps_max,omitempty"`
	RIRMin        *int64   `json:"rir_min,omitempty"`
	RIRMax        *int64   `json:"rir_max,omitempty"`
	PrescribedRPE *float64 `json:"prescribed_rpe,omitempty"`
	// NonWorking marks a warm-up set: shown, but not counted toward
	// progress or PRs.
	NonWorking bool    `json:"non_working"`
	GroupKey   *string `json:"group_key,omitempty"`
	GroupLabel *string `json:"group_label,omitempty"`
}

// PrescriptionResponse is the full prescription for today's workout.
type PrescriptionResponse struct {
	ProgramName string `json:"program_name"`
	CurrentWeek int    `json:"current_week"`
	CurrentDay  int    `json:"current_day"`
	CycleNumber int    `json:"cycle_number"`
	// NumWeeks/NumDays bound the program week/day a session can be started on.
	NumWeeks int `json:"num_weeks"`
	NumDays  int `json:"num_days"`
	// DaySource says where current_week/current_day came from: stored (today's
	// session has one), next_after_stored, count (older sessions without a
	// stored day), or chosen (the week/day query) (ADR 028).
	DaySource string                     `json:"day_source" enums:"stored,next_after_stored,count,chosen"`
	Lines     []PrescriptionLineResponse `json:"lines"`
}

// GetPrescription returns today's workout prescription for an athlete.
//
//	@Summary      Today's prescription
//	@Description  Returns the day's prescribed sets from the athlete's active program (if any), with TM-resolved weights.
//	@Tags         Athletes
//	@Produce      json
//	@Param        id    path      int  true   "Athlete ID"
//	@Param        week  query     int  false  "Program week to show instead of the next one (needs day)"
//	@Param        day   query     int  false  "Program day to show instead of the next one"
//	@Success      200  {object}  api.PrescriptionResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/prescription [get]
func (h *Handlers) GetPrescription(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	// Resolve which program assignment applies today.
	tz := "America/New_York"
	if prefs := middleware.PrefsFromContext(r.Context()); prefs != nil {
		tz = prefs.Timezone
	}

	program, err := models.ResolveAssignment(r.Context(), h.DB, athleteID, time.Now(), tz)
	if err != nil {
		log.Printf("api: resolve assignment for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to resolve program")
		return
	}
	if program == nil {
		WriteJSON(w, http.StatusOK, nil)
		return
	}

	var prescription *models.Prescription
	if q := r.URL.Query(); q.Get("day") != "" || q.Get("week") != "" {
		day, derr := strconv.Atoi(q.Get("day"))
		week := 0
		var werr error
		if q.Get("week") != "" {
			week, werr = strconv.Atoi(q.Get("week"))
		}
		if derr != nil || werr != nil || day < 1 || week < 0 {
			WriteValidationError(w, "day", "week and day must be positive integers; day is required")
			return
		}
		if week == 0 {
			week = models.CalendarProgramWeek(program.StartDate, todayInUserTZ(r), program.NumWeeks, program.IsLoop)
		}
		version := 0
		if tmpl, terr := models.GetProgramTemplateByID(r.Context(), h.DB, program.TemplateID); terr == nil {
			version = tmpl.Version
		}
		if verr := models.ValidateProgramDay(r.Context(), h.DB, program, version, week, day); verr != nil {
			var pde *models.ProgramDayError
			if errors.As(verr, &pde) {
				WriteValidationError(w, pde.Field, pde.Message)
				return
			}
			log.Printf("api: validate program day for athlete %d: %v", athleteID, verr)
			WriteError(w, http.StatusInternalServerError, "failed to get prescription")
			return
		}
		prescription, err = models.GetPrescriptionForDay(r.Context(), h.DB, program, time.Now(), week, day)
	} else {
		prescription, err = models.GetPrescription(r.Context(), h.DB, program, time.Now())
	}
	if err != nil {
		log.Printf("api: get prescription for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to get prescription")
		return
	}

	lines := prescriptionLinesFromModel(prescription.Lines)

	WriteJSON(w, http.StatusOK, PrescriptionResponse{
		ProgramName: prescription.Program.TemplateName,
		CurrentWeek: prescription.CurrentWeek,
		CurrentDay:  prescription.CurrentDay,
		CycleNumber: prescription.CycleNumber,
		NumWeeks:    program.NumWeeks,
		NumDays:     program.NumDays,
		DaySource:   prescription.DaySource,
		Lines:       lines,
	})
}

// prescriptionLinesFromModel converts a prescription's lines to the API
// shape. Shared by the prescription read and the whiteboard.
func prescriptionLinesFromModel(in []*models.PrescriptionLine) []PrescriptionLineResponse {
	lines := make([]PrescriptionLineResponse, len(in))
	for i, line := range in {
		sets := make([]PrescriptionSetResponse, len(line.Sets))
		for j, s := range line.Sets {
			sets[j] = PrescriptionSetResponse{
				SetNumber:      s.SetNumber,
				Reps:           nullInt(s.Reps),
				Percentage:     nullFloat(s.Percentage),
				TargetWeight:   s.TargetWeight,
				AbsoluteWeight: nullFloat(s.AbsoluteWeight),
				RestSeconds:    nullInt(s.RestSeconds),
				RepType:        s.RepType,
				Notes:          nullStr(s.Notes),
				RepsMax:        nullInt(s.RepsMax),
				RIRMin:         nullInt(s.RIRMin),
				RIRMax:         nullInt(s.RIRMax),
				PrescribedRPE:  nullFloat(s.PrescribedRPE),
				NonWorking:     s.NonWorking,
				GroupKey:       nullStr(s.GroupKey),
				GroupLabel:     nullStr(s.GroupLabel),
			}
		}
		lines[i] = PrescriptionLineResponse{
			ExerciseName: line.ExerciseName,
			ExerciseID:   line.ExerciseID,
			TrainingMax:  line.TrainingMax,
			GroupKey:     nullStr(line.GroupKey),
			Sets:         sets,
		}
	}
	return lines
}
