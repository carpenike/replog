package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
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

// avatarMaxBytes is the hard cap on the avatar request body. Larger requests
// are rejected before any disk I/O.
const avatarMaxBytes = 2 << 20 // 2 MiB

// avatarAllowedTypes maps MIME types we accept for avatars to the canonical
// extension we will write to disk. The extension is chosen server-side from
// the sniffed content, never from the client-supplied filename, to prevent
// a client from uploading e.g. an SVG (XSS sink) or HTML file with a .jpg
// name and having the FileServer hand it back with a guessed Content-Type.
var avatarAllowedTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// AvatarUpload handles avatar file upload for the authenticated user.
// AvatarUpload handles avatar file upload for the authenticated user.
//
//	@Summary      Upload avatar
//	@Description  Multipart upload; field name is 'avatar'. Server sniffs the file's content type and accepts only JPEG, PNG, WebP, or GIF. Returns the new avatar URL.
//	@Tags         Avatars
//	@Accept       multipart/form-data
//	@Produce      json
//	@Param        avatar formData file true "Avatar image file (JPEG/PNG/WebP/GIF, ≤ 2 MiB)"
//	@Success      200  {object}  map[string]string  "e.g. {\"avatar_url\": \"/avatars/abc.jpg\"}"
//	@Failure      400  {object}  api.APIError
//	@Failure      413  {object}  api.APIError
//	@Failure      415  {object}  api.APIError
//	@Router       /avatars/upload [post]
func (h *Handlers) AvatarUpload(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	// Cap the *entire* request body, not just the in-memory portion of
	// ParseMultipartForm. Otherwise, a client streaming a multi-GB body
	// would spill the excess to /tmp before being rejected.
	r.Body = http.MaxBytesReader(w, r.Body, avatarMaxBytes)

	if err := r.ParseMultipartForm(avatarMaxBytes); err != nil {
		WriteError(w, http.StatusRequestEntityTooLarge, "file too large (max 2 MiB)")
		return
	}

	file, _, err := r.FormFile("avatar")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "avatar file is required")
		return
	}
	defer file.Close()

	// Sniff the first 512 bytes — net/http.DetectContentType requires
	// exactly that many bytes (or all of the file, whichever is smaller).
	sniffBuf := make([]byte, 512)
	n, err := io.ReadFull(file, sniffBuf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		WriteError(w, http.StatusBadRequest, "failed to read upload")
		return
	}
	contentType := http.DetectContentType(sniffBuf[:n])
	ext, ok := avatarAllowedTypes[contentType]
	if !ok {
		WriteError(w, http.StatusUnsupportedMediaType, "avatar must be JPEG, PNG, WebP, or GIF")
		return
	}

	// Random suffix so avatar URLs are not enumerable from a numeric user
	// ID. The /avatars/{filename} route is public.
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		log.Printf("api: avatar random suffix: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to save avatar")
		return
	}
	filename := fmt.Sprintf("%d_%s%s", user.ID, hex.EncodeToString(suffix), ext)
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

	// Write the sniffed prefix first, then stream the rest.
	if _, err := dst.Write(sniffBuf[:n]); err != nil {
		log.Printf("api: write avatar prefix: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to save avatar")
		return
	}
	if _, err := io.Copy(dst, file); err != nil {
		// MaxBytesReader returns an error here if the body exceeded the cap.
		log.Printf("api: write avatar file: %v", err)
		WriteError(w, http.StatusRequestEntityTooLarge, "file too large (max 2 MiB)")
		return
	}

	// Best-effort cleanup of the previous avatar so we don't leak files
	// on disk every time the user uploads a new one.
	if user.HasAvatar() {
		if old := filepath.Join(h.AvatarDir, user.AvatarPath.String); old != fullPath {
			_ = os.Remove(old)
		}
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
//
//	@Summary      Test LLM provider connection (admin)
//	@Description  Always returns 200; check `success` field for actual result. Errors are surfaced via `error` field for the SPA to display inline.
//	@Tags         Admin
//	@Produce      json
//	@Success      200  {object}  map[string]interface{}
//	@Failure      403  {object}  api.APIError
//	@Router       /admin/settings/test-llm [post]
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
