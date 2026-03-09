package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/carpenike/replog/internal/llm"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Avatar Upload/Delete ---

// AvatarUpload handles avatar file upload for the authenticated user.
func (h *Handlers) AvatarUpload(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	if err := r.ParseMultipartForm(2 << 20); err != nil {
		WriteError(w, http.StatusBadRequest, "file too large (max 2MB)")
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "avatar file is required")
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg"
	}

	filename := fmt.Sprintf("%d_%x%s", user.ID, user.ID*31+17, ext)
	fullPath := filepath.Join(h.AvatarDir, filename)

	if err := os.MkdirAll(h.AvatarDir, 0750); err != nil {
		log.Printf("api: create avatar dir: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to save avatar")
		return
	}

	dst, err := os.Create(fullPath)
	if err != nil {
		log.Printf("api: create avatar file: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to save avatar")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		log.Printf("api: write avatar file: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to save avatar")
		return
	}

	if err := models.UpdateAvatarPath(h.DB, user.ID, sql.NullString{String: filename, Valid: true}); err != nil {
		log.Printf("api: update avatar path for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to update avatar")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"avatar_url": "/avatars/" + filename})
}

// AvatarDelete deletes the authenticated user's avatar.
func (h *Handlers) AvatarDelete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	if user.HasAvatar() {
		fullPath := filepath.Join(h.AvatarDir, user.AvatarPath.String)
		os.Remove(fullPath)
	}

	if err := models.UpdateAvatarPath(h.DB, user.ID, sql.NullString{}); err != nil {
		log.Printf("api: delete avatar for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete avatar")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Notification Preferences ---

// ListNotificationPreferences returns the user's notification preferences.
func (h *Handlers) ListNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	prefs := models.ListNotificationPreferences(h.DB, user.ID)

	result := make([]map[string]any, len(prefs))
	for i, p := range prefs {
		result[i] = map[string]any{
			"type":     p.Type,
			"in_app":   p.InApp,
			"external": p.External,
		}
	}
	WriteJSON(w, http.StatusOK, result)
}

// UpdateNotificationPreference updates a notification preference.
func (h *Handlers) UpdateNotificationPreference(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	var req struct {
		Type     string `json:"type"`
		InApp    bool   `json:"in_app"`
		External bool   `json:"external"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := models.SetNotificationPreference(h.DB, user.ID, req.Type, req.InApp, req.External); err != nil {
		log.Printf("api: update notification preference for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to update preference")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Program Copy Week ---

// CopyWeek copies prescribed sets from one week to another.
func (h *Handlers) CopyWeek(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	templateID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid program ID")
		return
	}

	var req struct {
		SourceWeek int `json:"source_week"`
		TargetWeek int `json:"target_week"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SourceWeek < 1 || req.TargetWeek < 1 {
		WriteError(w, http.StatusBadRequest, "source_week and target_week must be positive")
		return
	}

	inserted, err := models.CopyWeek(h.DB, templateID, req.SourceWeek, req.TargetWeek)
	if err != nil {
		log.Printf("api: copy week for template %d: %v", templateID, err)
		WriteError(w, http.StatusInternalServerError, "failed to copy week")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]int{"sets_copied": int(inserted)})
}

// --- Accessory Plan Update & Deactivate ---

// UpdateAccessoryPlan updates an accessory plan.
func (h *Handlers) UpdateAccessoryPlan(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	planID, err := strconv.ParseInt(r.PathValue("planID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid plan ID")
		return
	}

	var req struct {
		TargetSets   int     `json:"target_sets"`
		TargetRepMin int     `json:"target_rep_min"`
		TargetRepMax int     `json:"target_rep_max"`
		TargetWeight float64 `json:"target_weight"`
		Notes        string  `json:"notes"`
		SortOrder    int     `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := models.UpdateAccessoryPlan(h.DB, planID, req.TargetSets, req.TargetRepMin, req.TargetRepMax, req.TargetWeight, req.Notes, req.SortOrder); err != nil {
		log.Printf("api: update accessory plan %d: %v", planID, err)
		WriteError(w, http.StatusInternalServerError, "failed to update plan")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeactivateAccessoryPlan deactivates an accessory plan.
func (h *Handlers) DeactivateAccessoryPlan(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	planID, err := strconv.ParseInt(r.PathValue("planID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid plan ID")
		return
	}

	if err := models.DeactivateAccessoryPlan(h.DB, planID); err != nil {
		log.Printf("api: deactivate accessory plan %d: %v", planID, err)
		WriteError(w, http.StatusInternalServerError, "failed to deactivate plan")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Test Connection Endpoints ---

// TestLLMConnection tests the LLM provider connection.
func (h *Handlers) TestLLMConnection(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	provider, err := llm.NewProviderFromSettings(h.DB)
	if err != nil {
		WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": err.Error()})
		return
	}

	if err := provider.Ping(r.Context()); err != nil {
		WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": err.Error()})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}
