package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- TM Setup ---

// MissingTMResponse represents an exercise missing a training max.
type MissingTMResponse struct {
	ExerciseID   int64  `json:"exercise_id"`
	ExerciseName string `json:"exercise_name"`
}

// ListMissingTMs returns exercises in a program that need training maxes set.
func (h *Handlers) ListMissingTMs(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}

	templateID, err := strconv.ParseInt(r.URL.Query().Get("template_id"), 10, 64)
	if err != nil || templateID == 0 {
		WriteError(w, http.StatusBadRequest, "template_id query parameter required")
		return
	}

	missing, err := models.ListMissingProgramTMs(h.DB, templateID, athleteID)
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
func (h *Handlers) BatchSetTMs(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
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
			if _, err := models.SetTrainingMax(h.DB, athleteID, m.ExerciseID, m.Weight, date, "Batch TM setup"); err != nil {
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

	tokens, err := models.ListLoginTokensByUser(h.DB, userID)
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

	token, err := models.CreateLoginToken(h.DB, userID, req.Label, nil)
	if err != nil {
		log.Printf("api: create login token for user %d: %v", userID, err)
		WriteError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]any{
		"id":    token.ID,
		"token": token.Token,
		"url":   "/auth/token/" + token.Token,
	})
}

// DeleteLoginToken deletes a login token. Admin only.
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

	if err := models.DeleteLoginToken(h.DB, tokenID, userID); err != nil {
		log.Printf("api: delete login token %d: %v", tokenID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete token")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
