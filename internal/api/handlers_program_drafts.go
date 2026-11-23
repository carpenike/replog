package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// WriteProgramDraft stores a program proposal. A coach service token may
// call this. The row is always a draft: approving it is a separate route
// that the token cannot call. The write records provenance (coach, date,
// source text) and does not assign the program.
//
//	@Summary      Write program draft
//	@Description  Creates a draft proposal. Does not assign it. A coach service token may call this; it cannot approve the draft.
//	@Tags         Programs
//	@Accept       json
//	@Produce      json
//	@Param        body  body      api.WriteProgramDraftRequest  true  "Draft"
//	@Success      201  {object}  api.ProgramTemplate
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /programs/drafts [post]
func (h *Handlers) WriteProgramDraft(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	var req WriteProgramDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" || req.NumWeeks < 1 || req.NumDays < 1 {
		WriteError(w, http.StatusBadRequest, "name, num_weeks, and num_days are required")
		return
	}
	if strings.TrimSpace(req.SourceText) == "" {
		WriteValidationError(w, "source_text", "is required")
		return
	}
	if req.Audience != "" && req.Audience != "youth" && req.Audience != "adult" {
		WriteValidationError(w, "audience", "must be youth or adult")
		return
	}
	sourceDate := req.SourceDate
	if sourceDate == "" {
		sourceDate = todayInUserTZ(r)
	} else if !validDate(sourceDate) {
		WriteValidationError(w, "source_date", "must be YYYY-MM-DD")
		return
	}

	if req.LineageID != nil {
		parent, err := models.GetProgramTemplateByID(r.Context(), h.DB, *req.LineageID)
		if errors.Is(err, models.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "program not found")
			return
		}
		if err != nil {
			log.Printf("api: load program lineage %d: %v", *req.LineageID, err)
			WriteError(w, http.StatusInternalServerError, "failed to write draft")
			return
		}
		if parent.AthleteID != nil && !middleware.CanAccessAthlete(r.Context(), h.DB, user, *parent.AthleteID) {
			WriteError(w, http.StatusForbidden, "access denied")
			return
		}
	}
	if req.AthleteID != nil && !middleware.CanAccessAthlete(r.Context(), h.DB, user, *req.AthleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	sets := make([]models.ProgramDraftSet, len(req.Sets))
	for i, set := range req.Sets {
		if set.ExerciseID == 0 {
			WriteValidationError(w, "exercise_id", "is required")
			return
		}
		if set.PrescribedRPE != nil && (*set.PrescribedRPE < 1 || *set.PrescribedRPE > 10) {
			WriteValidationError(w, "prescribed_rpe", "must be between 1 and 10")
			return
		}
		if set.RepType != "" && !validSetRepType(set.RepType) {
			WriteValidationError(w, "rep_type", "must be reps, each_side, seconds, or distance")
			return
		}
		sets[i] = models.ProgramDraftSet{
			ExerciseID:     set.ExerciseID,
			Week:           set.Week,
			Day:            set.Day,
			SetNumber:      set.SetNumber,
			Reps:           set.Reps,
			Percentage:     set.Percentage,
			AbsoluteWeight: set.AbsoluteWeight,
			RestSeconds:    set.RestSeconds,
			SortOrder:      set.SortOrder,
			RepType:        set.RepType,
			Notes:          set.Notes,
			PrescribedRPE:  set.PrescribedRPE,
			GroupLabel:     set.GroupLabel,
			Detail: models.PrescribedSetDetail{
				RepsMax:    set.RepsMax,
				RIRMin:     set.RIRMin,
				RIRMax:     set.RIRMax,
				NonWorking: set.NonWorking,
				GroupKey:   set.GroupKey,
			},
		}
		if field, msg := models.ValidatePrescribedSetDetail(set.Reps, sets[i].Detail); field != "" {
			WriteValidationError(w, field, msg)
			return
		}
	}

	draft, err := models.CreateProgramDraft(r.Context(), h.DB, models.ProgramDraftInput{
		AthleteID:   req.AthleteID,
		Name:        req.Name,
		Description: req.Description,
		NumWeeks:    req.NumWeeks,
		NumDays:     req.NumDays,
		IsLoop:      req.IsLoop,
		Audience:    req.Audience,
		LineageID:   req.LineageID,
		CoachID:     user.ID,
		SourceDate:  sourceDate,
		SourceText:  req.SourceText,
		Sets:        sets,
	})
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "program not found")
		return
	}
	if errors.Is(err, models.ErrInvalidInput) {
		WriteError(w, http.StatusBadRequest, "invalid program draft")
		return
	}
	if err != nil {
		log.Printf("api: write program draft: %v", err)
		WriteDBError(w, err, "failed to write draft")
		return
	}

	WriteJSON(w, http.StatusCreated, ProgramTemplateFromModel(draft))
}

// ApproveProgramDraft marks a draft approved. A coach service token is
// refused. A browser session of the same coach can approve. Approving
// does not assign the program.
//
//	@Summary      Approve program draft
//	@Description  A browser session approves a draft. A coach service token receives 403. Approval does not assign the program.
//	@Tags         Programs
//	@Produce      json
//	@Param        id   path      int  true  "Program ID"
//	@Success      200  {object}  api.ProgramTemplate
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Failure      409  {object}  api.APIError
//	@Router       /programs/{id}/approve [post]
func (h *Handlers) ApproveProgramDraft(w http.ResponseWriter, r *http.Request) {
	if refuseServiceCoaching(w, r) {
		return
	}
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid program ID")
		return
	}

	program, err := models.GetProgramTemplateByID(r.Context(), h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "program not found")
		return
	}
	if err != nil {
		log.Printf("api: load program draft %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to approve draft")
		return
	}
	if program.AthleteID != nil && !middleware.CanAccessAthlete(r.Context(), h.DB, user, *program.AthleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	approved, err := models.ApproveProgramDraft(r.Context(), h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "program not found")
		return
	}
	if errors.Is(err, models.ErrProgramNameTaken) {
		WriteError(w, http.StatusConflict, "an approved program already uses this name")
		return
	}
	if errors.Is(err, models.ErrInvalidInput) {
		WriteError(w, http.StatusBadRequest, "program cannot be approved")
		return
	}
	if err != nil {
		log.Printf("api: approve program draft %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to approve draft")
		return
	}

	WriteJSON(w, http.StatusOK, ProgramTemplateFromModel(approved))
}
