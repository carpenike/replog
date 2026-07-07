package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
	"github.com/carpenike/replog/internal/notify"
)

// --- TM Setup ---

// MissingTMResponse represents an exercise missing a training max.
type MissingTMResponse struct {
	ExerciseID   int64  `json:"exercise_id"`
	ExerciseName string `json:"exercise_name"`
}

// ListMissingTMs returns exercises in a program that need training maxes set.
// ListMissingTMs returns exercises that need a training max set for an athlete's program.
//
//	@Summary      List exercises missing a training max
//	@Tags         TrainingMaxes
//	@Produce      json
//	@Param        id   path      int  true  "Athlete ID"
//	@Success      200  {array}   api.Exercise
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/missing-tms [get]
func (h *Handlers) ListMissingTMs(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	templateID, err := strconv.ParseInt(r.URL.Query().Get("template_id"), 10, 64)
	if err != nil || templateID == 0 {
		WriteError(w, http.StatusBadRequest, "template_id query parameter required")
		return
	}

	missing, err := models.ListMissingProgramTMs(r.Context(), h.DB, templateID, athleteID)
	if err != nil {
		log.Printf("api: list missing TMs for athlete %d template %d: %v", athleteID, templateID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list missing TMs")
		return
	}

	result := make([]MissingTMResponse, len(missing))
	for i, m := range missing {
		result[i] = MissingTMResponse{ExerciseID: m.ExerciseID, ExerciseName: m.ExerciseName}
	}
	WriteJSON(w, http.StatusOK, result)
}

// BatchSetTMs sets multiple training maxes at once.
// BatchSetTMs sets several training maxes at once for an athlete (TM Setup wizard).
//
//	@Summary      Batch set training maxes
//	@Tags         TrainingMaxes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                  true  "Athlete ID"
//	@Param        body  body      api.BatchTMRequest   true  "Maxes"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/batch-tms [post]
func (h *Handlers) BatchSetTMs(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	var req struct {
		Maxes []struct {
			ExerciseID int64   `json:"exercise_id"`
			Weight     float64 `json:"weight"`
		} `json:"maxes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	set := 0
	date := time.Now().Format("2006-01-02")
	for _, m := range req.Maxes {
		if m.Weight > 0 {
			if _, err := models.SetTrainingMax(r.Context(), h.DB, athleteID, m.ExerciseID, m.Weight, date, "Batch TM setup"); err != nil {
				log.Printf("api: batch set TM athlete %d exercise %d: %v", athleteID, m.ExerciseID, err)
			} else {
				set++
			}
		}
	}

	WriteJSON(w, http.StatusOK, map[string]int{"set": set})
}

// --- Login Tokens ---

// ListLoginTokens returns login tokens for a user. Admin only.
// ListLoginTokens returns active login tokens for a user. Admin only.
//
//	@Summary      List login tokens for user
//	@Tags         Admin
//	@Produce      json
//	@Param        userID  path      int  true  "User ID"
//	@Success      200  {array}   api.LoginToken
//	@Failure      403  {object}  api.APIError
//	@Router       /users/{userID}/tokens [get]
func (h *Handlers) ListLoginTokens(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	userID, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	tokens, err := models.ListLoginTokensByUser(r.Context(), h.DB, userID)
	if err != nil {
		log.Printf("api: list login tokens for user %d: %v", userID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list tokens")
		return
	}

	result := make([]map[string]any, len(tokens))
	for i, t := range tokens {
		result[i] = map[string]any{
			"id":         t.ID,
			"label":      nullStr(t.Label),
			"expires_at": fmtNullTime(t.ExpiresAt),
			"created_at": fmtTime(t.CreatedAt),
			"expired":    t.IsExpired(),
		}
	}
	WriteJSON(w, http.StatusOK, result)
}

// CreateLoginToken generates a login token for a user. Admin only.
// CreateLoginToken creates a magic-link login token for a user. Admin only.
//
//	@Summary      Create login token
//	@Description  Returns the bare token in the response — it is shown to the admin once and never again.
//	@Tags         Admin
//	@Accept       json
//	@Produce      json
//	@Param        userID  path      int                    true  "User ID"
//	@Param        body    body      api.LoginTokenRequest  false "Optional label"
//	@Success      201  {object}  api.LoginToken
//	@Failure      403  {object}  api.APIError
//	@Router       /users/{userID}/tokens [post]
func (h *Handlers) CreateLoginToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	userID, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req struct {
		Label string `json:"label"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	token, err := models.CreateLoginToken(r.Context(), h.DB, userID, req.Label, nil)
	if err != nil {
		log.Printf("api: create login token for user %d: %v", userID, err)
		WriteError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	// Notify the user that a magic-link login token was issued for them
	// (ADR 008). We deliberately do NOT embed the usable token in the
	// persisted notification Link — a stored notification row is a durable
	// artifact and a token in it would be a replayable credential at rest.
	// The admin shares the one-time URL (returned below) out-of-band; the
	// notification only informs the user that a link exists.
	notify.Send(r.Context(), h.DB, notify.Request{
		UserID:  userID,
		Type:    models.NotifyMagicLinkSent,
		Title:   "Login link issued",
		Message: "An admin generated a single-use login link for your account.",
		Link:    "",
	})

	WriteJSON(w, http.StatusCreated, map[string]any{
		"id":    token.ID,
		"token": token.Token,
		"url":   "/auth/token/" + token.Token,
	})
}

// DeleteLoginToken deletes a login token. Admin only.
// DeleteLoginToken revokes a login token. Admin only.
//
//	@Summary      Revoke login token
//	@Tags         Admin
//	@Produce      json
//	@Param        userID   path      int  true  "User ID"
//	@Param        tokenID  path      int  true  "Token ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      403  {object}  api.APIError
//	@Router       /users/{userID}/tokens/{tokenID} [delete]
func (h *Handlers) DeleteLoginToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	tokenID, err := strconv.ParseInt(r.PathValue("tokenID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid token ID")
		return
	}

	userID, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	if err := models.DeleteLoginToken(r.Context(), h.DB, tokenID, userID); err != nil {
		log.Printf("api: delete login token %d: %v", tokenID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete token")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
