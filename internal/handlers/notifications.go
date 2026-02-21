package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
	"github.com/carpenike/replog/internal/notify"
)

// Notifications handles in-app notification endpoints.
type Notifications struct {
	DB        *sql.DB
	Templates TemplateCache
}

// List renders the full notifications page for the current user.
// GET /notifications
func (h *Notifications) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	notifications, err := models.ListNotifications(h.DB, user.ID, 50, 0)
	if err != nil {
		log.Printf("handlers: list notifications for user %d: %v", user.ID, err)
		h.Templates.ServerError(w, r)
		return
	}

	unreadCount, _ := models.GetUnreadCount(h.DB, user.ID)

	data := map[string]any{
		"Notifications": notifications,
		"UnreadCount":   unreadCount,
		"TypeLabels":    notificationTypeLabels(),
	}
	if err := h.Templates.Render(w, r, "notifications.html", data); err != nil {
		log.Printf("handlers: render notifications: %v", err)
	}
}

// UnreadCount returns the unread notification badge as an HTML fragment.
// GET /notifications/count
func (h *Notifications) UnreadCount(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	count, _ := models.GetUnreadCount(h.DB, user.ID)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if count > 0 {
		badge := strconv.Itoa(count)
		if count > 99 {
			badge = "99+"
		}
		w.Write([]byte(`<span class="notification-badge" id="notification-badge" hx-get="/notifications/count" hx-trigger="every 30s" hx-swap="outerHTML">` + badge + `</span>`))
	} else {
		// Empty span that keeps polling.
		w.Write([]byte(`<span class="notification-badge notification-badge--empty" id="notification-badge" hx-get="/notifications/count" hx-trigger="every 30s" hx-swap="outerHTML"></span>`))
	}
}

// Toast returns new unread notifications as toast HTML fragments for htmx polling.
// GET /notifications/toast
func (h *Notifications) Toast(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	// Parse the "since" timestamp — client sends the last poll time.
	sinceStr := r.URL.Query().Get("since")
	since := time.Now().Add(-31 * time.Second) // default: last 31 seconds
	if sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = t
		}
	}

	notifications, err := models.GetNotificationsSince(h.DB, user.ID, since)
	if err != nil {
		log.Printf("handlers: get toast notifications for user %d: %v", user.ID, err)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if len(notifications) == 0 {
		// Return an empty container that keeps polling with updated timestamp.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		now := time.Now().UTC().Format(time.RFC3339)
		w.Write([]byte(`<div id="toast-container" hx-get="/notifications/toast?since=` + now + `" hx-trigger="every 30s" hx-swap="outerHTML"></div>`))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	now := time.Now().UTC().Format(time.RFC3339)

	// Render toast notifications.
	w.Write([]byte(`<div id="toast-container" hx-get="/notifications/toast?since=` + now + `" hx-trigger="every 30s" hx-swap="outerHTML">`))
	for _, n := range notifications {
		icon := notificationIcon(n.Type)
		linkOpen := ""
		linkClose := ""
		if n.Link.Valid && n.Link.String != "" {
			linkOpen = `<a href="` + n.Link.String + `" hx-post="/notifications/` + strconv.FormatInt(n.ID, 10) + `/read" hx-swap="none" class="toast-link">`
			linkClose = `</a>`
		}
		w.Write([]byte(`<div class="toast toast--` + n.Type + `" data-toast-id="` + strconv.FormatInt(n.ID, 10) + `">` +
			linkOpen +
			`<span class="toast-icon">` + icon + `</span>` +
			`<div class="toast-body">` +
			`<div class="toast-title">` + n.Title + `</div>` +
			messageHTML(n.Message) +
			`</div>` +
			linkClose +
			`<button class="toast-dismiss" data-action="dismiss-toast" aria-label="Dismiss">&times;</button>` +
			`</div>`))
	}
	w.Write([]byte(`</div>`))
}

// MarkRead marks a single notification as read.
// POST /notifications/{id}/read
func (h *Notifications) MarkRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid notification ID", http.StatusBadRequest)
		return
	}

	if err := models.MarkAsRead(h.DB, id, user.ID); err != nil {
		log.Printf("handlers: mark notification %d read: %v", id, err)
	}

	// Return 200 with no content (hx-swap="none").
	w.WriteHeader(http.StatusOK)
}

// MarkAllRead marks all notifications as read for the current user.
// POST /notifications/read-all
func (h *Notifications) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	if _, err := models.MarkAllAsRead(h.DB, user.ID); err != nil {
		log.Printf("handlers: mark all read for user %d: %v", user.ID, err)
	}

	// Re-render the notifications list.
	notifications, err := models.ListNotifications(h.DB, user.ID, 50, 0)
	if err != nil {
		log.Printf("handlers: list notifications after mark-all-read: %v", err)
		h.Templates.ServerError(w, r)
		return
	}

	data := map[string]any{
		"Notifications": notifications,
		"UnreadCount":   0,
		"TypeLabels":    notificationTypeLabels(),
	}
	if err := h.Templates.Render(w, r, "notifications.html", data); err != nil {
		log.Printf("handlers: render notifications: %v", err)
	}
}

// Preferences renders the notification preferences form.
// GET /notifications/preferences
func (h *Notifications) Preferences(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	prefs := models.ListNotificationPreferences(h.DB, user.ID)
	externalConfigured := models.GetSetting(h.DB, "notify.urls") != ""

	data := map[string]any{
		"NotificationPrefs":  prefs,
		"NotificationTypes":  models.AllNotificationTypes,
		"ExternalConfigured": externalConfigured,
	}
	if err := h.Templates.Render(w, r, "notification_preferences.html", data); err != nil {
		log.Printf("handlers: render notification preferences: %v", err)
	}
}

// UpdatePreferences saves notification preferences.
// POST /notifications/preferences
func (h *Notifications) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	for _, nt := range models.AllNotificationTypes {
		inApp := r.FormValue("in_app_"+nt.Type) == "on"
		external := r.FormValue("external_"+nt.Type) == "on"
		if err := models.SetNotificationPreference(h.DB, user.ID, nt.Type, inApp, external); err != nil {
			log.Printf("handlers: set notification preference %q for user %d: %v", nt.Type, user.ID, err)
		}
	}

	prefs := models.ListNotificationPreferences(h.DB, user.ID)
	externalConfigured := models.GetSetting(h.DB, "notify.urls") != ""

	data := map[string]any{
		"NotificationPrefs":  prefs,
		"NotificationTypes":  models.AllNotificationTypes,
		"ExternalConfigured": externalConfigured,
		"Success":            "Notification preferences saved.",
	}
	if err := h.Templates.Render(w, r, "notification_preferences.html", data); err != nil {
		log.Printf("handlers: render notification preferences: %v", err)
	}
}

// TestNotify sends a test notification via external channels.
// POST /admin/settings/test-notify
func (h *Notifications) TestNotify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := notify.TestConnection(h.DB); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`<span class="test-result test-error">` + err.Error() + `</span>`))
		return
	}

	w.Write([]byte(`<span class="test-result test-success">Test notification sent successfully!</span>`))
}

// --- Helpers ---

func notificationTypeLabels() map[string]string {
	labels := make(map[string]string)
	for _, nt := range models.AllNotificationTypes {
		labels[nt.Type] = nt.Label
	}
	return labels
}

func notificationIcon(nType string) string {
	switch nType {
	case models.NotifyReviewSubmitted:
		return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11.08V12a10 10 0 11-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>`
	case models.NotifyProgramAssigned:
		return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>`
	case models.NotifyTMUpdated:
		return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 6 13.5 15.5 8.5 10.5 1 18"/><polyline points="17 6 23 6 23 12"/></svg>`
	case models.NotifyWorkoutLogged:
		return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6.5 6.5h11M6.5 17.5h11"/><rect x="2" y="4" width="4" height="5" rx="1"/><rect x="18" y="4" width="4" height="5" rx="1"/><rect x="2" y="15" width="4" height="5" rx="1"/><rect x="18" y="15" width="4" height="5" rx="1"/><line x1="12" y1="2" x2="12" y2="22"/></svg>`
	case models.NotifyNoteAdded:
		return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/></svg>`
	default:
		return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 01-3.46 0"/></svg>`
	}
}

func messageHTML(msg sql.NullString) string {
	if !msg.Valid || msg.String == "" {
		return ""
	}
	return `<div class="toast-message">` + msg.String + `</div>`
}
