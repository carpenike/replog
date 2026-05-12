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

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Avatar Upload/Delete ---

// AvatarUpload handles avatar file upload for the authenticated user.
// AvatarUpload handles avatar file upload for the authenticated user.
//
//	@Summary      Upload avatar
//	@Description  Multipart upload; field name is 'file'. Server returns the new avatar URL.
//	@Tags         Avatars
//	@Accept       multipart/form-data
//	@Produce      json
//	@Success      200  {object}  map[string]string  "e.g. {\"avatar_url\": \"/avatars/abc.jpg\"}"
//	@Failure      400  {object}  api.APIError
//	@Router       /avatars/upload [post]
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
// AvatarDelete deletes the authenticated user's avatar file.
//
//	@Summary      Delete avatar
//	@Tags         Avatars
//	@Produce      json
//	@Success      200  {object}  api.StatusResponse
//	@Router       /avatars/delete [post]
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
//
//	@Summary      List notification preferences
//	@Tags         Notifications
//	@Produce      json
//	@Success      200  {array}   map[string]interface{}
//	@Router       /notifications/preferences [get]
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
//
//	@Summary      Update notification preference for one type
//	@Tags         Notifications
//	@Accept       json
//	@Produce      json
//	@Param        body  body      api.NotificationPreferenceRequest  true  "Preference"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Router       /notifications/preferences [put]
func (h *Handlers) UpdateNotificationPreference(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	var req NotificationPreferenceRequest
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
// CopyWeek copies all prescribed sets from one week to another within a template.
//
//	@Summary      Copy week of prescribed sets
//	@Tags         Programs
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                  true  "Program ID"
//	@Param        body  body      api.CopyWeekRequest  true  "Source/target weeks"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /programs/{id}/copy-week [post]
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

	var req CopyWeekRequest
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
// UpdateAccessoryPlan updates an accessory plan.
//
//	@Summary      Update accessory plan
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id      path      int                              true  "Athlete ID"
//	@Param        planID  path      int                              true  "Plan ID"
//	@Param        body    body      api.AccessoryPlanUpdateRequest   true  "Plan"
//	@Success      200  {object}  api.AccessoryPlan
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/accessories/{planID} [put]
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

	var req AccessoryPlanUpdateRequest
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
// DeactivateAccessoryPlan marks an accessory plan as inactive.
//
//	@Summary      Deactivate accessory plan
//	@Tags         Athletes
//	@Produce      json
//	@Param        id      path      int  true  "Athlete ID"
//	@Param        planID  path      int  true  "Plan ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/accessories/{planID}/deactivate [post]
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

	provider, err := h.llmProvider()
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
