package api

import (
	"log"
	"net/http"
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
	ProgramName string                     `json:"program_name"`
	CurrentWeek int                        `json:"current_week"`
	CurrentDay  int                        `json:"current_day"`
	CycleNumber int                        `json:"cycle_number"`
	Lines       []PrescriptionLineResponse `json:"lines"`
}

// GetPrescription returns today's workout prescription for an athlete.
//
//	@Summary      Today's prescription
//	@Description  Returns the day's prescribed sets from the athlete's active program (if any), with TM-resolved weights.
//	@Tags         Athletes
//	@Produce      json
//	@Param        id   path      int  true  "Athlete ID"
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

	prescription, err := models.GetPrescription(r.Context(), h.DB, program, time.Now())
	if err != nil {
		log.Printf("api: get prescription for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to get prescription")
		return
	}

	lines := make([]PrescriptionLineResponse, len(prescription.Lines))
	for i, line := range prescription.Lines {
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

	WriteJSON(w, http.StatusOK, PrescriptionResponse{
		ProgramName: prescription.Program.TemplateName,
		CurrentWeek: prescription.CurrentWeek,
		CurrentDay:  prescription.CurrentDay,
		CycleNumber: prescription.CycleNumber,
		Lines:       lines,
	})
}
