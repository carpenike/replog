package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Notifications ---

// ListNotifications returns notifications for the authenticated user.
//
//	@Summary      List notifications
//	@Tags         Notifications
//	@Produce      json
//	@Success      200  {array}   api.Notification
//	@Router       /notifications [get]
func (h *Handlers) ListNotifications(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	// Clamp caller-supplied paging: ListNotifications passes limit straight to
	// SQL where LIMIT -1 means "unlimited", so an unclamped limit=-1 would dump
	// every notification. parsePage caps it and rejects negatives.
	limit, offset := parsePage(r, 50, 200)

	notifications, err := models.ListNotifications(h.DB, user.ID, limit, offset)
	if err != nil {
		log.Printf("api: list notifications for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list notifications")
		return
	}

	result := make([]*Notification, len(notifications))
	for i, n := range notifications {
		result[i] = NotificationFromModel(n)
	}
	WriteJSON(w, http.StatusOK, result)
}

// UnreadNotificationCount returns the number of unread notifications.
//
//	@Summary      Unread notification count
//	@Tags         Notifications
//	@Produce      json
//	@Success      200  {object}  map[string]int  "e.g. {\"count\": 3}"
//	@Router       /notifications/count [get]
func (h *Handlers) UnreadNotificationCount(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	count, err := models.GetUnreadCount(h.DB, user.ID)
	if err != nil {
		log.Printf("api: unread count for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to get count")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]int{"count": count})
}

// MarkNotificationRead marks a notification as read.
//
//	@Summary      Mark notification read
//	@Tags         Notifications
//	@Produce      json
//	@Param        notificationID  path      int  true  "Notification ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Router       /notifications/{notificationID}/read [post]
func (h *Handlers) MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("notificationID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid notification ID")
		return
	}

	if err := models.MarkAsRead(h.DB, id, user.ID); err != nil {
		log.Printf("api: mark notification %d read: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to mark read")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// MarkAllNotificationsRead marks all notifications as read.
//
//	@Summary      Mark all notifications read
//	@Tags         Notifications
//	@Produce      json
//	@Success      200  {object}  map[string]int  "e.g. {\"marked\": 5}"
//	@Router       /notifications/read-all [post]
func (h *Handlers) MarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	count, err := models.MarkAllAsRead(h.DB, user.ID)
	if err != nil {
		log.Printf("api: mark all notifications read for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to mark all read")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]int64{"marked": count})
}

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
