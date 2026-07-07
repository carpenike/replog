package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Throwing Sessions (ADR 018) ---

// ListThrowingSessions returns an athlete's throwing sessions, newest first.
//
//	@Summary      List throwing sessions
//	@Tags         Athletes
//	@Produce      json
//	@Param        id  path  int  true  "Athlete ID"
//	@Success      200  {array}   api.ThrowingSession
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/throwing-sessions [get]
func (h *Handlers) ListThrowingSessions(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	sessions, err := models.ListThrowingSessions(h.DB, athleteID, 100)
	if err != nil {
		log.Printf("api: list throwing sessions for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list throwing sessions")
		return
	}

	out := make([]*ThrowingSession, len(sessions))
	for i, s := range sessions {
		out[i] = ThrowingSessionFromModel(s)
	}
	WriteJSON(w, http.StatusOK, out)
}

// CreateThrowingSession logs a throwing session for an athlete.
//
//	@Summary      Log throwing session
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                         true  "Athlete ID"
//	@Param        body  body      api.ThrowingSessionRequest  true  "Session"
//	@Success      201  {object}  api.ThrowingSession
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      409  {object}  api.APIError
//	@Router       /athletes/{id}/throwing-sessions [post]
func (h *Handlers) CreateThrowingSession(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	var req ThrowingSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ThrowType == "" {
		WriteError(w, http.StatusBadRequest, "throw_type is required")
		return
	}
	if req.Date == "" {
		req.Date = todayInUserTZ(r)
	} else if !validDate(req.Date) {
		WriteValidationError(w, "date", "must be a valid date in YYYY-MM-DD format")
		return
	}

	session, err := models.CreateThrowingSession(h.DB, athleteID, models.ThrowingSessionInput{
		Date:       req.Date,
		ThrowType:  req.ThrowType,
		ThrowCount: req.ThrowCount,
		MaxIntent:  req.MaxIntent,
		Velocity:   req.Velocity,
		Fatigue:    req.Fatigue,
		Pain:       req.Pain,
		Source:     req.Source,
		Team:       req.Team,
		Notes:      req.Notes,
	})
	if err != nil {
		switch {
		case errors.Is(err, models.ErrWorkoutExists):
			WriteError(w, http.StatusConflict, "a throwing session already exists for this date")
		case errors.Is(err, models.ErrInvalidInput):
			WriteError(w, http.StatusBadRequest, err.Error())
		default:
			log.Printf("api: create throwing session for athlete %d: %v", athleteID, err)
			WriteError(w, http.StatusInternalServerError, "failed to log throwing session")
		}
		return
	}

	WriteJSON(w, http.StatusCreated, ThrowingSessionFromModel(session))
}

// DeleteThrowingSession removes a throwing session (and its parent workout).
//
//	@Summary      Delete throwing session
//	@Tags         Athletes
//	@Param        id         path  int  true  "Athlete ID"
//	@Param        sessionID  path  int  true  "Throwing session ID"
//	@Success      204  "No Content"
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/throwing-sessions/{sessionID} [delete]
func (h *Handlers) DeleteThrowingSession(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	sessionID, err := strconv.ParseInt(r.PathValue("sessionID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid session ID")
		return
	}

	// Confirm the session belongs to this athlete before deleting.
	ts, err := models.GetThrowingSessionByID(h.DB, sessionID)
	if err != nil || ts.AthleteID != athleteID {
		WriteError(w, http.StatusNotFound, "throwing session not found")
		return
	}

	if err := models.DeleteThrowingSession(h.DB, sessionID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "throwing session not found")
			return
		}
		log.Printf("api: delete throwing session %d: %v", sessionID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete throwing session")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Season Phases (ADR 018) ---

// ListSeasonPhases returns an athlete's season phases, newest start first.
//
//	@Summary      List season phases
//	@Tags         Athletes
//	@Produce      json
//	@Param        id  path  int  true  "Athlete ID"
//	@Success      200  {array}   api.SeasonPhase
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/season-phases [get]
func (h *Handlers) ListSeasonPhases(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	phases, err := models.ListSeasonPhases(h.DB, athleteID)
	if err != nil {
		log.Printf("api: list season phases for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list season phases")
		return
	}

	out := make([]*SeasonPhase, len(phases))
	for i, p := range phases {
		out[i] = SeasonPhaseFromModel(p)
	}
	WriteJSON(w, http.StatusOK, out)
}

// CreateSeasonPhase records a season phase for an athlete.
//
//	@Summary      Record season phase
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                     true  "Athlete ID"
//	@Param        body  body      api.SeasonPhaseRequest  true  "Phase"
//	@Success      201  {object}  api.SeasonPhase
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/season-phases [post]
func (h *Handlers) CreateSeasonPhase(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	var req SeasonPhaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Phase == "" || req.StartDate == "" {
		WriteError(w, http.StatusBadRequest, "phase and start_date are required")
		return
	}

	phase, err := models.CreateSeasonPhase(h.DB, athleteID, models.SeasonPhaseInput{
		Sport:     req.Sport,
		Phase:     req.Phase,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		Notes:     req.Notes,
	})
	if err != nil {
		if errors.Is(err, models.ErrInvalidInput) {
			WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("api: create season phase for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to record season phase")
		return
	}

	WriteJSON(w, http.StatusCreated, SeasonPhaseFromModel(phase))
}

// DeleteSeasonPhase removes a season phase.
//
//	@Summary      Delete season phase
//	@Tags         Athletes
//	@Param        id       path  int  true  "Athlete ID"
//	@Param        phaseID  path  int  true  "Season phase ID"
//	@Success      204  "No Content"
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/season-phases/{phaseID} [delete]
func (h *Handlers) DeleteSeasonPhase(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	phaseID, err := strconv.ParseInt(r.PathValue("phaseID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid phase ID")
		return
	}

	sp, err := models.GetSeasonPhaseByID(h.DB, phaseID)
	if err != nil || sp.AthleteID != athleteID {
		WriteError(w, http.StatusNotFound, "season phase not found")
		return
	}

	if err := models.DeleteSeasonPhase(h.DB, phaseID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "season phase not found")
			return
		}
		log.Printf("api: delete season phase %d: %v", phaseID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete season phase")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Bio Samples (ADR 018) ---

// ListBioSamples returns an athlete's bio samples, newest first.
//
//	@Summary      List bio samples
//	@Tags         Athletes
//	@Produce      json
//	@Param        id      path   int     true   "Athlete ID"
//	@Param        metric  query  string  false  "Filter by metric"
//	@Success      200  {array}   api.BioSample
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/bio-samples [get]
func (h *Handlers) ListBioSamples(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	samples, err := models.ListBioSamples(h.DB, athleteID, r.URL.Query().Get("metric"), 100)
	if err != nil {
		log.Printf("api: list bio samples for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list bio samples")
		return
	}

	out := make([]*BioSample, len(samples))
	for i, s := range samples {
		out[i] = BioSampleFromModel(s)
	}
	WriteJSON(w, http.StatusOK, out)
}

// CreateBioSample records a biometric reading for an athlete.
//
//	@Summary      Record bio sample
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                   true  "Athlete ID"
//	@Param        body  body      api.BioSampleRequest  true  "Sample"
//	@Success      201  {object}  api.BioSample
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/bio-samples [post]
func (h *Handlers) CreateBioSample(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	var req BioSampleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Metric == "" || req.RecordedAt == "" {
		WriteError(w, http.StatusBadRequest, "metric and recorded_at are required")
		return
	}

	sample, err := models.CreateBioSample(h.DB, athleteID, models.BioSampleInput{
		RecordedAt: req.RecordedAt,
		Metric:     req.Metric,
		Value:      req.Value,
		Unit:       req.Unit,
		Source:     req.Source,
		Notes:      req.Notes,
	})
	if err != nil {
		if errors.Is(err, models.ErrInvalidInput) {
			WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("api: create bio sample for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to record bio sample")
		return
	}

	WriteJSON(w, http.StatusCreated, BioSampleFromModel(sample))
}

// --- Pitch Smart advisory (ADR 007/018) ---

// GetPitchSmartStatus returns the read-only Pitch Smart advisory for an athlete.
//
//	@Summary      Pitch Smart advisory
//	@Description  Read-only arm-care guidance (recommended daily max, rest days owed). Advisory only — never an automated action.
//	@Tags         Athletes
//	@Produce      json
//	@Param        id  path  int  true  "Athlete ID"
//	@Success      200  {object}  api.PitchSmartStatus
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/pitch-smart [get]
func (h *Handlers) GetPitchSmartStatus(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	status, err := models.ComputePitchSmartStatus(h.DB, athleteID, time.Now())
	if err != nil {
		if errors.Is(err, models.ErrNoPitchSmartLimit) {
			WriteError(w, http.StatusNotFound, "no pitch smart guidance for this athlete (age unknown or outside reference range)")
			return
		}
		log.Printf("api: pitch smart status for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to compute pitch smart status")
		return
	}

	WriteJSON(w, http.StatusOK, PitchSmartStatusFromModel(status))
}

// --- Conditioning Sessions (ADR 018) ---

// ListConditioningSessions returns an athlete's conditioning sessions, newest first.
//
//	@Summary      List conditioning sessions
//	@Tags         Athletes
//	@Produce      json
//	@Param        id  path  int  true  "Athlete ID"
//	@Success      200  {array}   api.ConditioningSession
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/conditioning-sessions [get]
func (h *Handlers) ListConditioningSessions(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	sessions, err := models.ListConditioningSessions(h.DB, athleteID, 100)
	if err != nil {
		log.Printf("api: list conditioning sessions for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list conditioning sessions")
		return
	}

	out := make([]*ConditioningSession, len(sessions))
	for i, s := range sessions {
		out[i] = ConditioningSessionFromModel(s)
	}
	WriteJSON(w, http.StatusOK, out)
}

// CreateConditioningSession logs a conditioning session for an athlete.
//
//	@Summary      Log conditioning session
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                             true  "Athlete ID"
//	@Param        body  body      api.ConditioningSessionRequest  true  "Session"
//	@Success      201  {object}  api.ConditioningSession
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      409  {object}  api.APIError
//	@Router       /athletes/{id}/conditioning-sessions [post]
func (h *Handlers) CreateConditioningSession(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	var req ConditioningSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Modality == "" {
		WriteError(w, http.StatusBadRequest, "modality is required")
		return
	}
	if req.SessionType == "" {
		WriteError(w, http.StatusBadRequest, "session_type is required")
		return
	}
	if req.Date == "" {
		req.Date = todayInUserTZ(r)
	} else if !validDate(req.Date) {
		WriteValidationError(w, "date", "must be a valid date in YYYY-MM-DD format")
		return
	}

	intervals := make([]models.ConditioningIntervalInput, len(req.Intervals))
	for i, iv := range req.Intervals {
		intervals[i] = models.ConditioningIntervalInput{
			IntervalNumber: iv.IntervalNumber,
			WorkSeconds:    iv.WorkSeconds,
			WorkDistance:   iv.WorkDistance,
			RestSeconds:    iv.RestSeconds,
			Notes:          iv.Notes,
		}
	}

	session, err := models.CreateConditioningSession(h.DB, athleteID, models.ConditioningSessionInput{
		Date:            req.Date,
		Modality:        req.Modality,
		SessionType:     req.SessionType,
		TotalDistance:   req.TotalDistance,
		DistanceUnit:    req.DistanceUnit,
		DurationSeconds: req.DurationSeconds,
		AvgHR:           req.AvgHR,
		RPE:             req.RPE,
		Notes:           req.Notes,
		Intervals:       intervals,
	})
	if err != nil {
		switch {
		case errors.Is(err, models.ErrWorkoutExists):
			WriteError(w, http.StatusConflict, "a conditioning session already exists for this date")
		case errors.Is(err, models.ErrInvalidInput):
			WriteError(w, http.StatusBadRequest, err.Error())
		default:
			log.Printf("api: create conditioning session for athlete %d: %v", athleteID, err)
			WriteError(w, http.StatusInternalServerError, "failed to log conditioning session")
		}
		return
	}

	WriteJSON(w, http.StatusCreated, ConditioningSessionFromModel(session))
}

// DeleteConditioningSession removes a conditioning session (and its parent workout).
//
//	@Summary      Delete conditioning session
//	@Tags         Athletes
//	@Param        id         path  int  true  "Athlete ID"
//	@Param        sessionID  path  int  true  "Conditioning session ID"
//	@Success      204  "No Content"
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/conditioning-sessions/{sessionID} [delete]
func (h *Handlers) DeleteConditioningSession(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	sessionID, err := strconv.ParseInt(r.PathValue("sessionID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid session ID")
		return
	}

	cs, err := models.GetConditioningSessionByID(h.DB, sessionID)
	if err != nil || cs.AthleteID != athleteID {
		WriteError(w, http.StatusNotFound, "conditioning session not found")
		return
	}

	if err := models.DeleteConditioningSession(h.DB, sessionID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "conditioning session not found")
			return
		}
		log.Printf("api: delete conditioning session %d: %v", sessionID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete conditioning session")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Skill Sessions (ADR 018) ---

// ListSkillSessions returns an athlete's skill sessions, newest first.
//
//	@Summary      List skill sessions
//	@Tags         Athletes
//	@Produce      json
//	@Param        id  path  int  true  "Athlete ID"
//	@Success      200  {array}   api.SkillSession
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/skill-sessions [get]
func (h *Handlers) ListSkillSessions(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	sessions, err := models.ListSkillSessions(h.DB, athleteID, 100)
	if err != nil {
		log.Printf("api: list skill sessions for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list skill sessions")
		return
	}

	out := make([]*SkillSession, len(sessions))
	for i, s := range sessions {
		out[i] = SkillSessionFromModel(s)
	}
	WriteJSON(w, http.StatusOK, out)
}

// CreateSkillSession logs a skill session for an athlete.
//
//	@Summary      Log skill session
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                      true  "Athlete ID"
//	@Param        body  body      api.SkillSessionRequest  true  "Session"
//	@Success      201  {object}  api.SkillSession
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      409  {object}  api.APIError
//	@Router       /athletes/{id}/skill-sessions [post]
func (h *Handlers) CreateSkillSession(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	var req SkillSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SkillType == "" {
		WriteError(w, http.StatusBadRequest, "skill_type is required")
		return
	}
	if req.Date == "" {
		req.Date = todayInUserTZ(r)
	} else if !validDate(req.Date) {
		WriteValidationError(w, "date", "must be a valid date in YYYY-MM-DD format")
		return
	}

	session, err := models.CreateSkillSession(h.DB, athleteID, models.SkillSessionInput{
		Date:            req.Date,
		SkillType:       req.SkillType,
		RepCount:        req.RepCount,
		LoadKg:          req.LoadKg,
		Velocity:        req.Velocity,
		DurationSeconds: req.DurationSeconds,
		Notes:           req.Notes,
	})
	if err != nil {
		switch {
		case errors.Is(err, models.ErrWorkoutExists):
			WriteError(w, http.StatusConflict, "a skill session already exists for this date")
		case errors.Is(err, models.ErrInvalidInput):
			WriteError(w, http.StatusBadRequest, err.Error())
		default:
			log.Printf("api: create skill session for athlete %d: %v", athleteID, err)
			WriteError(w, http.StatusInternalServerError, "failed to log skill session")
		}
		return
	}

	WriteJSON(w, http.StatusCreated, SkillSessionFromModel(session))
}

// DeleteSkillSession removes a skill session (and its parent workout).
//
//	@Summary      Delete skill session
//	@Tags         Athletes
//	@Param        id         path  int  true  "Athlete ID"
//	@Param        sessionID  path  int  true  "Skill session ID"
//	@Success      204  "No Content"
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/skill-sessions/{sessionID} [delete]
func (h *Handlers) DeleteSkillSession(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	sessionID, err := strconv.ParseInt(r.PathValue("sessionID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid session ID")
		return
	}

	ss, err := models.GetSkillSessionByID(h.DB, sessionID)
	if err != nil || ss.AthleteID != athleteID {
		WriteError(w, http.StatusNotFound, "skill session not found")
		return
	}

	if err := models.DeleteSkillSession(h.DB, sessionID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "skill session not found")
			return
		}
		log.Printf("api: delete skill session %d: %v", sessionID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete skill session")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Recovery Check-ins (ADR 018) ---

// ListRecoveryCheckins returns an athlete's recovery check-ins, newest first.
//
//	@Summary      List recovery check-ins
//	@Tags         Athletes
//	@Produce      json
//	@Param        id  path  int  true  "Athlete ID"
//	@Success      200  {array}   api.RecoveryCheckin
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/recovery-checkins [get]
func (h *Handlers) ListRecoveryCheckins(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	checkins, err := models.ListRecoveryCheckins(h.DB, athleteID, 100)
	if err != nil {
		log.Printf("api: list recovery checkins for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list recovery check-ins")
		return
	}

	out := make([]*RecoveryCheckin, len(checkins))
	for i, c := range checkins {
		out[i] = RecoveryCheckinFromModel(c)
	}
	WriteJSON(w, http.StatusOK, out)
}

// CreateRecoveryCheckin logs a recovery check-in for an athlete.
//
//	@Summary      Log recovery check-in
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                         true  "Athlete ID"
//	@Param        body  body      api.RecoveryCheckinRequest  true  "Check-in"
//	@Success      201  {object}  api.RecoveryCheckin
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      409  {object}  api.APIError
//	@Router       /athletes/{id}/recovery-checkins [post]
func (h *Handlers) CreateRecoveryCheckin(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	var req RecoveryCheckinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Date == "" {
		req.Date = todayInUserTZ(r)
	} else if !validDate(req.Date) {
		WriteValidationError(w, "date", "must be a valid date in YYYY-MM-DD format")
		return
	}

	checkin, err := models.CreateRecoveryCheckin(h.DB, athleteID, models.RecoveryCheckinInput{
		Date:       req.Date,
		SleepHours: req.SleepHours,
		Soreness:   req.Soreness,
		Energy:     req.Energy,
		Notes:      req.Notes,
	})
	if err != nil {
		switch {
		case errors.Is(err, models.ErrWorkoutExists):
			WriteError(w, http.StatusConflict, "a recovery check-in already exists for this date")
		case errors.Is(err, models.ErrInvalidInput):
			WriteError(w, http.StatusBadRequest, err.Error())
		default:
			log.Printf("api: create recovery checkin for athlete %d: %v", athleteID, err)
			WriteError(w, http.StatusInternalServerError, "failed to log recovery check-in")
		}
		return
	}

	WriteJSON(w, http.StatusCreated, RecoveryCheckinFromModel(checkin))
}

// DeleteRecoveryCheckin removes a recovery check-in (and its parent workout).
//
//	@Summary      Delete recovery check-in
//	@Tags         Athletes
//	@Param        id          path  int  true  "Athlete ID"
//	@Param        checkinID   path  int  true  "Recovery check-in ID"
//	@Success      204  "No Content"
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/recovery-checkins/{checkinID} [delete]
func (h *Handlers) DeleteRecoveryCheckin(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	checkinID, err := strconv.ParseInt(r.PathValue("checkinID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid check-in ID")
		return
	}

	rc, err := models.GetRecoveryCheckinByID(h.DB, checkinID)
	if err != nil || rc.AthleteID != athleteID {
		WriteError(w, http.StatusNotFound, "recovery check-in not found")
		return
	}

	if err := models.DeleteRecoveryCheckin(h.DB, checkinID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "recovery check-in not found")
			return
		}
		log.Printf("api: delete recovery checkin %d: %v", checkinID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete recovery check-in")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Load Summary (ADR 018) ---

// GetLoadSummary returns the read-only, advisory per-discipline training-load
// view for an athlete. It is computed on read from logged sessions and never
// gates a log or triggers any automated action (ADR 007/018).
//
//	@Summary      Get training-load summary
//	@Tags         Athletes
//	@Produce      json
//	@Param        id  path  int  true  "Athlete ID"
//	@Success      200  {object}  api.LoadSummary
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/load [get]
func (h *Handlers) GetLoadSummary(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	summary, err := models.GetLoadSummary(h.DB, athleteID)
	if err != nil {
		log.Printf("api: get load summary for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to compute load summary")
		return
	}

	WriteJSON(w, http.StatusOK, LoadSummaryFromModel(summary))
}

// athleteAccess parses the {id} path value and enforces athlete access. It
// writes the appropriate error response and returns ok=false on failure.
func (h *Handlers) athleteAccess(w http.ResponseWriter, r *http.Request) (int64, bool) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return 0, false
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return 0, false
	}
	return athleteID, true
}
