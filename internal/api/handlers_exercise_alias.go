package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/carpenike/replog/internal/models"
)

// Coach exercise aliases and renames (ADR 026). Each change is a preview
// that writes nothing and returns a one-time token, then a confirm that
// re-checks and applies it. A rename changes exercises.name by id only;
// logged sets, PRs, and programs refer to the id and are not rewritten.

const exerciseConfirmPrefix = "rlex_"

// CoachExerciseAliasPreviewRequest names the alias to add.
type CoachExerciseAliasPreviewRequest struct {
	Alias string `json:"alias"`
}

// CoachExerciseRenamePreviewRequest names the new canonical name.
type CoachExerciseRenamePreviewRequest struct {
	Name string `json:"name"`
}

// CoachExerciseConfirmRequest applies a previewed alias or rename.
type CoachExerciseConfirmRequest struct {
	PreviewToken string `json:"preview_token"`
	Confirm      bool   `json:"confirm"`
}

// CoachExerciseChangePreviewResponse is the checked change. Nothing is written.
type CoachExerciseChangePreviewResponse struct {
	Action       string   `json:"action"` // add_alias or rename
	ExerciseID   int64    `json:"exercise_id"`
	CurrentName  string   `json:"current_name"`
	NewName      string   `json:"new_name"`
	Alias        *string  `json:"alias,omitempty"`
	KeptAlias    *string  `json:"kept_alias,omitempty"`
	RemovedAlias *string  `json:"removed_alias,omitempty"`
	AliasesAfter []string `json:"aliases_after"`
	PreviewToken string   `json:"preview_token"`
	ExpiresAt    string   `json:"expires_at"`
}

type exerciseGrant struct {
	userID     int64
	athleteID  int64
	exerciseID int64
	alias      *models.AliasPlan
	rename     *models.RenamePlan
	expires    time.Time
}

type exerciseGrantStore struct {
	mu     sync.Mutex
	byHash map[string]exerciseGrant
}

func (h *Handlers) exerciseGrants() *exerciseGrantStore {
	h.exerciseGrantOnce.Do(func() {
		h.exerciseGrantStore = &exerciseGrantStore{byHash: map[string]exerciseGrant{}}
	})
	return h.exerciseGrantStore
}

func (h *Handlers) issueExerciseGrant(g exerciseGrant, now time.Time) (string, time.Time, error) {
	token, key, err := models.NewConfirmToken(exerciseConfirmPrefix)
	if err != nil {
		return "", time.Time{}, err
	}
	g.expires = now.Add(programConfirmTTL)
	store := h.exerciseGrants()
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

func (h *Handlers) consumeExerciseGrant(userID, athleteID, exerciseID int64, rename bool, token string, now time.Time) (exerciseGrant, bool) {
	key := models.HashConfirmToken(token)
	store := h.exerciseGrants()
	store.mu.Lock()
	defer store.mu.Unlock()
	g, ok := store.byHash[key]
	if !ok || !g.expires.After(now) {
		delete(store.byHash, key)
		return exerciseGrant{}, false
	}
	if g.userID != userID || g.athleteID != athleteID || g.exerciseID != exerciseID || (g.rename != nil) != rename {
		return exerciseGrant{}, false
	}
	delete(store.byHash, key)
	return g, true
}

func exerciseIDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("exerciseID"), 10, 64)
	if err != nil || id <= 0 {
		WriteError(w, http.StatusBadRequest, "invalid exercise ID")
		return 0, false
	}
	return id, true
}

func writeExerciseChangeError(w http.ResponseWriter, action string, err error) {
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "exercise not found")
		return
	}
	writeProgramError(w, action, err)
}

func decodeStrict(w http.ResponseWriter, r *http.Request, allowed map[string]struct{}, hint string, into any) bool {
	raw, err := readObject(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	if extra := unknownKeys(raw, allowed, ""); len(extra) > 0 {
		WriteValidationError(w, extra[0], hint)
		return false
	}
	if err := json.Unmarshal(raw, into); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

// PreviewCoachExerciseAlias checks an alias for an exercise. It writes nothing.
//
//	@Summary      Preview an exercise alias
//	@Description  Checks that alias can be added: it must not match, ignoring case, spaces, and punctuation, any other exercise's name or alias, or this exercise's name. Does not write. The token lasts 15 minutes and works once.
//	@Tags         Coach
//	@Accept       json
//	@Produce      json
//	@Param        id          path  int                                    true  "Athlete ID"
//	@Param        exerciseID  path  int                                    true  "Exercise ID"
//	@Param        body        body  api.CoachExerciseAliasPreviewRequest   true  "Alias"
//	@Success      200  {object}  api.CoachExerciseChangePreviewResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Failure      409  {object}  api.APIError
//	@Router       /athletes/{id}/coach/exercises/{exerciseID}/aliases/preview [post]
func (h *Handlers) PreviewCoachExerciseAlias(w http.ResponseWriter, r *http.Request) {
	user, athlete, ok := h.coachProgramAccess(w, r, true)
	if !ok {
		return
	}
	exerciseID, ok := exerciseIDParam(w, r)
	if !ok {
		return
	}
	var req CoachExerciseAliasPreviewRequest
	if !decodeStrict(w, r, map[string]struct{}{"alias": {}}, "preview accepts alias", &req) {
		return
	}
	plan, err := models.PlanExerciseAlias(r.Context(), h.DB, exerciseID, req.Alias)
	if err != nil {
		writeExerciseChangeError(w, "preview alias", err)
		return
	}
	token, expires, err := h.issueExerciseGrant(exerciseGrant{userID: user.ID, athleteID: athlete.ID, exerciseID: exerciseID, alias: plan}, time.Now())
	if err != nil {
		log.Printf("api: preview alias token: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to preview the alias")
		return
	}
	alias := plan.Alias
	WriteJSON(w, http.StatusOK, CoachExerciseChangePreviewResponse{
		Action: "add_alias", ExerciseID: exerciseID, CurrentName: plan.ExerciseName, NewName: plan.ExerciseName,
		Alias: &alias, AliasesAfter: plan.Aliases, PreviewToken: token,
		ExpiresAt: expires.In(userLocation(r)).Format(time.RFC3339),
	})
}

// ConfirmCoachExerciseAlias adds a previewed alias.
//
//	@Summary      Add an exercise alias
//	@Description  confirm must be true. Re-checks the previewed alias and adds it. Changes only the alias list; logged sets and programs are not touched.
//	@Tags         Coach
//	@Accept       json
//	@Produce      json
//	@Param        id          path  int                               true  "Athlete ID"
//	@Param        exerciseID  path  int                               true  "Exercise ID"
//	@Param        body        body  api.CoachExerciseConfirmRequest   true  "Preview token"
//	@Success      200  {object}  api.CoachExerciseResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Failure      409  {object}  api.APIError
//	@Router       /athletes/{id}/coach/exercises/{exerciseID}/aliases/confirm [post]
func (h *Handlers) ConfirmCoachExerciseAlias(w http.ResponseWriter, r *http.Request) {
	h.confirmExerciseChange(w, r, false)
}

// PreviewCoachExerciseRename checks a rename. It writes nothing.
//
//	@Summary      Preview an exercise rename
//	@Description  Checks renaming the exercise by id. The new name must not match, ignoring case, spaces, and punctuation, another exercise's name or alias. The old name is kept as an alias unless it differs only in case or punctuation. Does not write. The token lasts 15 minutes and works once.
//	@Tags         Coach
//	@Accept       json
//	@Produce      json
//	@Param        id          path  int                                     true  "Athlete ID"
//	@Param        exerciseID  path  int                                     true  "Exercise ID"
//	@Param        body        body  api.CoachExerciseRenamePreviewRequest   true  "New name"
//	@Success      200  {object}  api.CoachExerciseChangePreviewResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Failure      409  {object}  api.APIError
//	@Router       /athletes/{id}/coach/exercises/{exerciseID}/rename/preview [post]
func (h *Handlers) PreviewCoachExerciseRename(w http.ResponseWriter, r *http.Request) {
	user, athlete, ok := h.coachProgramAccess(w, r, true)
	if !ok {
		return
	}
	exerciseID, ok := exerciseIDParam(w, r)
	if !ok {
		return
	}
	var req CoachExerciseRenamePreviewRequest
	if !decodeStrict(w, r, map[string]struct{}{"name": {}}, "preview accepts name", &req) {
		return
	}
	plan, err := models.PlanExerciseRename(r.Context(), h.DB, exerciseID, req.Name)
	if err != nil {
		writeExerciseChangeError(w, "preview rename", err)
		return
	}
	token, expires, err := h.issueExerciseGrant(exerciseGrant{userID: user.ID, athleteID: athlete.ID, exerciseID: exerciseID, rename: plan}, time.Now())
	if err != nil {
		log.Printf("api: preview rename token: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to preview the rename")
		return
	}
	WriteJSON(w, http.StatusOK, CoachExerciseChangePreviewResponse{
		Action: "rename", ExerciseID: exerciseID, CurrentName: plan.OldName, NewName: plan.NewName,
		KeptAlias: plan.AddAlias, RemovedAlias: plan.RemoveAlias, AliasesAfter: plan.Aliases,
		PreviewToken: token, ExpiresAt: expires.In(userLocation(r)).Format(time.RFC3339),
	})
}

// ConfirmCoachExerciseRename applies a previewed rename.
//
//	@Summary      Rename an exercise
//	@Description  confirm must be true. Re-checks the previewed rename and applies it by exercise id. Changes only the exercise name and its aliases; logged sets, PRs, and programs keep their exercise id.
//	@Tags         Coach
//	@Accept       json
//	@Produce      json
//	@Param        id          path  int                               true  "Athlete ID"
//	@Param        exerciseID  path  int                               true  "Exercise ID"
//	@Param        body        body  api.CoachExerciseConfirmRequest   true  "Preview token"
//	@Success      200  {object}  api.CoachExerciseResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Failure      409  {object}  api.APIError
//	@Router       /athletes/{id}/coach/exercises/{exerciseID}/rename/confirm [post]
func (h *Handlers) ConfirmCoachExerciseRename(w http.ResponseWriter, r *http.Request) {
	h.confirmExerciseChange(w, r, true)
}

func (h *Handlers) confirmExerciseChange(w http.ResponseWriter, r *http.Request, rename bool) {
	user, athlete, ok := h.coachProgramAccess(w, r, true)
	if !ok {
		return
	}
	exerciseID, ok := exerciseIDParam(w, r)
	if !ok {
		return
	}
	var req CoachExerciseConfirmRequest
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
	grant, ok := h.consumeExerciseGrant(user.ID, athlete.ID, exerciseID, rename, req.PreviewToken, time.Now())
	if !ok {
		WriteValidationError(w, "preview_token", "is invalid or expired")
		return
	}
	var row *models.CoachCatalogExercise
	var err error
	if rename {
		row, err = models.ApplyExerciseRename(r.Context(), h.DB, *grant.rename)
	} else {
		row, err = models.ApplyExerciseAlias(r.Context(), h.DB, *grant.alias)
	}
	if err != nil {
		writeExerciseChangeError(w, "change exercise", err)
		return
	}
	log.Printf("api: coach exercise %d changed by user %d (rename=%t)", exerciseID, user.ID, rename)
	WriteJSON(w, http.StatusOK, coachExerciseResponse(*row, false))
}
