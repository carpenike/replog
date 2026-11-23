package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/carpenike/replog/internal/models"
)

// Session backfill (ADR 027). A coach records a past resistance session:
// preview checks and normalizes it and writes nothing; confirm writes the
// workout and all of its sets in one transaction. The token works once and
// lasts 15 minutes. A session already on that date is never merged or
// replaced: preview flags it and confirm refuses with 409.

const backfillConfirmPrefix = "rlbf_"

// CoachBackfillSetRequest is one set of a past session.
type CoachBackfillSetRequest struct {
	ExerciseID int64    `json:"exercise_id" example:"12"`
	Reps       int      `json:"reps" example:"5"`
	WeightLbs  *float64 `json:"weight_lbs" example:"135"`
	RepType    string   `json:"rep_type,omitempty" example:"reps"`
	SetOrder   int      `json:"set_order" example:"1"`
	Warmup     bool     `json:"warmup,omitempty"`
}

// CoachBackfillPreviewRequest is a past session to check.
type CoachBackfillPreviewRequest struct {
	Date            string                    `json:"date" example:"2026-11-06"`
	AssignmentID    int64                     `json:"assignment_id,omitempty" example:"1"`
	TemplateID      int64                     `json:"template_id,omitempty" example:"13"`
	TemplateVersion int                       `json:"template_version,omitempty" example:"1"`
	ProgramWeek     int                       `json:"program_week,omitempty" example:"1"`
	ProgramDay      int                       `json:"program_day,omitempty" example:"3"`
	SessionRPE      *float64                  `json:"session_rpe,omitempty" example:"6"`
	Notes           string                    `json:"notes,omitempty"`
	Sets            []CoachBackfillSetRequest `json:"sets"`
}

// CoachBackfillConfirmRequest writes a previewed session.
type CoachBackfillConfirmRequest struct {
	PreviewToken string `json:"preview_token"`
	Confirm      bool   `json:"confirm"`
}

// CoachBackfillSet is one normalized set.
type CoachBackfillSet struct {
	SetOrder     int      `json:"set_order"`
	ExerciseID   int64    `json:"exercise_id"`
	ExerciseName *string  `json:"exercise_name"`
	SetNumber    int      `json:"set_number"`
	Reps         int      `json:"reps"`
	RepType      string   `json:"rep_type"`
	WeightLbs    *float64 `json:"weight_lbs"`
	SetType      string   `json:"set_type"`
}

// CoachBackfillWarning is advisory unless can_confirm is false.
type CoachBackfillWarning struct {
	Code        string  `json:"code"`
	Message     string  `json:"message"`
	ExerciseIDs []int64 `json:"exercise_ids,omitempty"`
}

// CoachBackfillProgram is the assignment the session links to.
type CoachBackfillProgram struct {
	AssignmentID    int64  `json:"assignment_id"`
	TemplateID      int64  `json:"template_id"`
	TemplateName    string `json:"template_name"`
	TemplateVersion int    `json:"template_version"`
	StartDate       string `json:"start_date"`
	Schedule        []int  `json:"schedule"`
	Week            int    `json:"week"`
	Day             int    `json:"day"`
	ProgramDayLabel string `json:"program_day_label"`
	LaterSessions   int    `json:"later_sessions"`
}

// CoachBackfillPreviewResponse is the checked session. Nothing is written.
type CoachBackfillPreviewResponse struct {
	AthleteID    int64                  `json:"athlete_id"`
	Date         string                 `json:"date"`
	Weekday      int                    `json:"weekday"`
	Program      *CoachBackfillProgram  `json:"program"`
	SessionRPE   *float64               `json:"session_rpe"`
	Notes        string                 `json:"notes"`
	Sets         []CoachBackfillSet     `json:"sets"`
	SetCount     int                    `json:"set_count"`
	WorkingSets  int                    `json:"working_sets"`
	WarmupSets   int                    `json:"warmup_sets"`
	Exercises    int                    `json:"exercises"`
	Warnings     []CoachBackfillWarning `json:"warnings"`
	Duplicate    *int64                 `json:"duplicate_session_id"`
	CanConfirm   bool                   `json:"can_confirm"`
	PreviewToken *string                `json:"preview_token"`
	ExpiresAt    *string                `json:"expires_at"`
}

// CoachBackfillConfirmResponse is the stored session.
type CoachBackfillConfirmResponse struct {
	SessionID       int64  `json:"session_id"`
	AthleteID       int64  `json:"athlete_id"`
	Date            string `json:"date"`
	AssignmentID    *int64 `json:"assignment_id"`
	ProgramDayLabel string `json:"program_day_label"`
	SetsWritten     int    `json:"sets_written"`
	WorkingSets     int    `json:"working_sets"`
	WarmupSets      int    `json:"warmup_sets"`
}

type backfillGrant struct {
	userID    int64
	athleteID int64
	plan      *models.BackfillPlan
	expires   time.Time
}

type backfillGrantStore struct {
	mu     sync.Mutex
	byHash map[string]backfillGrant
}

func (h *Handlers) backfillGrants() *backfillGrantStore {
	h.backfillGrantOnce.Do(func() {
		h.backfillGrantStore = &backfillGrantStore{byHash: map[string]backfillGrant{}}
	})
	return h.backfillGrantStore
}

func (h *Handlers) issueBackfillGrant(g backfillGrant, now time.Time) (string, time.Time, error) {
	token, key, err := models.NewConfirmToken(backfillConfirmPrefix)
	if err != nil {
		return "", time.Time{}, err
	}
	g.expires = now.Add(programConfirmTTL)
	store := h.backfillGrants()
	store.mu.Lock()
	defer store.mu.Unlock()
	for k, old := range store.byHash {
		if !old.expires.After(now) {
			delete(store.byHash, k)
		}
	}
	store.byHash[key] = g
	return token, g.expires, nil
}

// consumeBackfillGrant removes the token on any matching use, so it can
// never write twice.
func (h *Handlers) consumeBackfillGrant(userID, athleteID int64, token string, now time.Time) (backfillGrant, bool) {
	key := models.HashConfirmToken(token)
	store := h.backfillGrants()
	store.mu.Lock()
	defer store.mu.Unlock()
	g, ok := store.byHash[key]
	if !ok || !g.expires.After(now) {
		delete(store.byHash, key)
		return backfillGrant{}, false
	}
	if g.userID != userID || g.athleteID != athleteID {
		return backfillGrant{}, false
	}
	delete(store.byHash, key)
	return g, true
}

var backfillPreviewKeys = map[string]struct{}{
	"date": {}, "assignment_id": {}, "template_id": {}, "template_version": {},
	"program_week": {}, "program_day": {}, "session_rpe": {}, "notes": {}, "sets": {},
}

var backfillSetKeys = map[string]struct{}{
	"exercise_id": {}, "reps": {}, "weight_lbs": {}, "rep_type": {}, "set_order": {}, "warmup": {},
}

func decodeBackfillPreview(w http.ResponseWriter, r *http.Request) (*CoachBackfillPreviewRequest, bool) {
	raw, err := readObject(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return nil, false
	}
	hint := "preview accepts date, assignment_id, template_id, template_version, program_week, program_day, session_rpe, notes, sets"
	if extra := unknownKeys(raw, backfillPreviewKeys, ""); len(extra) > 0 {
		WriteValidationError(w, extra[0], hint)
		return nil, false
	}
	var top struct {
		Sets []json.RawMessage `json:"sets"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		WriteValidationError(w, "sets", "must be a list of set objects")
		return nil, false
	}
	for i, s := range top.Sets {
		if extra := unknownKeys(s, backfillSetKeys, fmt.Sprintf("sets[%d]", i)); len(extra) > 0 {
			WriteValidationError(w, extra[0], "a set accepts exercise_id, reps, weight_lbs, rep_type, set_order, warmup")
			return nil, false
		}
	}
	var req CoachBackfillPreviewRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return nil, false
	}
	return &req, true
}

func writeBackfillError(w http.ResponseWriter, action string, err error) {
	var field *models.BackfillFieldError
	switch {
	case errors.As(err, &field):
		WriteValidationError(w, field.Field, field.Message)
	case errors.Is(err, models.ErrBackfillDuplicate):
		WriteError(w, http.StatusConflict, "a resistance session already exists for this athlete on this date; nothing was written")
	default:
		log.Printf("api: %s: %v", action, err)
		WriteError(w, http.StatusInternalServerError, "failed to "+action)
	}
}

// PreviewCoachBackfill checks a past session. It writes nothing.
//
//	@Summary      Preview a session backfill
//	@Description  Checks and normalizes one past resistance session: date (not in the future), an optional program link (assignment_id and/or template_id + template_version, plus program_week/program_day) checked against the athlete's assignment, optional session_rpe and notes, and the sets in set_order. weight_lbs null is bodyweight; warmup marks a non-working set. Returns the session as it would be stored, warnings (unknown_exercise, off_schedule, program_day_mismatch, later_sessions_shift, cycle_complete, unlinked, duplicate) and, when can_confirm is true, a preview_token that works once within 15 minutes. A session already on that date sets duplicate_session_id and no token is issued. Does not write.
//	@Tags         Coach
//	@Accept       json
//	@Produce      json
//	@Param        id    path  int                               true  "Athlete ID"
//	@Param        body  body  api.CoachBackfillPreviewRequest   true  "Past session"
//	@Success      200  {object}  api.CoachBackfillPreviewResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/coach/sessions/backfill/preview [post]
func (h *Handlers) PreviewCoachBackfill(w http.ResponseWriter, r *http.Request) {
	user, athlete, ok := h.coachProgramAccess(w, r, true)
	if !ok {
		return
	}
	req, ok := decodeBackfillPreview(w, r)
	if !ok {
		return
	}
	in := models.BackfillInput{
		Date: strings.TrimSpace(req.Date), AssignmentID: req.AssignmentID, TemplateID: req.TemplateID,
		TemplateVersion: req.TemplateVersion, ProgramWeek: req.ProgramWeek, ProgramDay: req.ProgramDay,
		SessionRPE: req.SessionRPE, Notes: req.Notes,
	}
	for _, s := range req.Sets {
		in.Sets = append(in.Sets, models.BackfillSetInput{
			ExerciseID: s.ExerciseID, Reps: s.Reps, WeightLbs: s.WeightLbs,
			RepType: s.RepType, SetOrder: s.SetOrder, Warmup: s.Warmup,
		})
	}
	loc := userLocation(r)
	plan, err := models.PlanBackfill(r.Context(), h.DB, athlete.ID, in, todayInUserTZ(r), loc)
	if err != nil {
		writeBackfillError(w, "preview the backfill", err)
		return
	}
	resp := backfillPreviewResponse(plan)
	if plan.CanConfirm() {
		token, expires, err := h.issueBackfillGrant(backfillGrant{userID: user.ID, athleteID: athlete.ID, plan: plan}, time.Now())
		if err != nil {
			log.Printf("api: backfill token: %v", err)
			WriteError(w, http.StatusInternalServerError, "failed to preview the backfill")
			return
		}
		at := expires.In(loc).Format(time.RFC3339)
		resp.PreviewToken, resp.ExpiresAt = &token, &at
	}
	WriteJSON(w, http.StatusOK, resp)
}

func backfillPreviewResponse(plan *models.BackfillPlan) CoachBackfillPreviewResponse {
	resp := CoachBackfillPreviewResponse{
		AthleteID: plan.AthleteID, Date: plan.Date, Weekday: plan.Weekday, SessionRPE: plan.SessionRPE,
		Notes: plan.Notes, Sets: []CoachBackfillSet{}, SetCount: len(plan.Sets), WorkingSets: plan.Working,
		WarmupSets: plan.Warmups, Exercises: plan.Exercises, Warnings: []CoachBackfillWarning{},
		CanConfirm: plan.CanConfirm(),
	}
	if p := plan.Program; p != nil {
		schedule := p.Schedule
		if schedule == nil {
			schedule = []int{}
		}
		resp.Program = &CoachBackfillProgram{
			AssignmentID: p.AssignmentID, TemplateID: p.TemplateID, TemplateName: p.TemplateName,
			TemplateVersion: p.TemplateVersion, StartDate: isoDay(p.StartDate), Schedule: schedule,
			Week: p.Week, Day: p.Day, ProgramDayLabel: p.Label, LaterSessions: p.LaterSessions,
		}
	}
	for _, s := range plan.Sets {
		var name *string
		if s.ExerciseName != "" {
			n := s.ExerciseName
			name = &n
		}
		resp.Sets = append(resp.Sets, CoachBackfillSet{
			SetOrder: s.SetOrder, ExerciseID: s.ExerciseID, ExerciseName: name, SetNumber: s.SetNumber,
			Reps: s.Reps, RepType: s.RepType, WeightLbs: s.WeightLbs, SetType: s.SetType,
		})
	}
	for _, wn := range plan.Warnings {
		resp.Warnings = append(resp.Warnings, CoachBackfillWarning{Code: wn.Code, Message: wn.Message, ExerciseIDs: wn.ExerciseIDs})
	}
	if plan.DuplicateID != 0 {
		id := plan.DuplicateID
		resp.Duplicate = &id
	}
	return resp
}

func isoDay(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// ConfirmCoachBackfill writes a previewed past session.
//
//	@Summary      Write a session backfill
//	@Description  confirm must be true. Writes the previewed workout and all of its sets in one transaction, linked to the previewed assignment. Refuses with 409 when a resistance session already exists on that date; nothing is merged or replaced. The token works once: a replay is refused. Existing workouts and sets are not touched. PRs and plan-vs-done are read from sets, so the new session counts like a live-logged one.
//	@Tags         Coach
//	@Accept       json
//	@Produce      json
//	@Param        id    path  int                               true  "Athlete ID"
//	@Param        body  body  api.CoachBackfillConfirmRequest   true  "Preview token"
//	@Success      201  {object}  api.CoachBackfillConfirmResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Failure      409  {object}  api.APIError
//	@Router       /athletes/{id}/coach/sessions/backfill/confirm [post]
func (h *Handlers) ConfirmCoachBackfill(w http.ResponseWriter, r *http.Request) {
	user, athlete, ok := h.coachProgramAccess(w, r, true)
	if !ok {
		return
	}
	var req CoachBackfillConfirmRequest
	if !decodeStrict(w, r, map[string]struct{}{"preview_token": {}, "confirm": {}}, "confirm accepts preview_token and confirm", &req) {
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
	grant, ok := h.consumeBackfillGrant(user.ID, athlete.ID, req.PreviewToken, time.Now())
	if !ok {
		WriteValidationError(w, "preview_token", "is invalid, used, or expired")
		return
	}
	plan := grant.plan
	sessionID, err := models.ApplyBackfill(r.Context(), h.DB, plan)
	if err != nil {
		writeBackfillError(w, "write the backfill", err)
		return
	}
	// Same review bookkeeping as a set logged through the API.
	if err := models.AutoApproveWorkout(r.Context(), h.DB, sessionID, user.ID); err != nil {
		log.Printf("api: auto-approve backfilled workout %d: %v", sessionID, err)
	}
	resp := CoachBackfillConfirmResponse{
		SessionID: sessionID, AthleteID: athlete.ID, Date: plan.Date, SetsWritten: len(plan.Sets),
		WorkingSets: plan.Working, WarmupSets: plan.Warmups,
	}
	if plan.Program != nil {
		id := plan.Program.AssignmentID
		resp.AssignmentID = &id
		label, err := models.BackfillProgramLabel(r.Context(), h.DB, id, plan.Date)
		if err != nil {
			log.Printf("api: backfill label %d: %v", sessionID, err)
		}
		resp.ProgramDayLabel = label
	}
	log.Printf("api: coach backfill session %d athlete %d sets %d by user %d", sessionID, athlete.ID, len(plan.Sets), user.ID)
	WriteJSON(w, http.StatusCreated, resp)
}
