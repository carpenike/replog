// Package notify provides channel-agnostic notification dispatch.
//
// Producers call Send() with a notification request. The dispatcher resolves
// the target user, checks per-type preferences, and dispatches to enabled
// channels: in-app (SQLite insert → toast/badge) and external (Shoutrrr).
package notify

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/carpenike/replog/internal/models"
	"github.com/containrrr/shoutrrr"
)

// Request describes a notification to send.
type Request struct {
	UserID    int64         // Target user
	Type      string        // Notification type constant (e.g. models.NotifyReviewSubmitted)
	Title     string        // Short title for in-app display
	Message   string        // Longer description (optional)
	Link      string        // Relative URL to navigate to on click (optional)
	AthleteID sql.NullInt64 // Related athlete (optional, for coach-scoping)
}

// Send dispatches a notification through all enabled channels for the target user.
// It checks the user's per-type preferences and dispatches accordingly.
// Errors are logged but do not propagate — notifications must never block
// the triggering action.
func Send(db *sql.DB, req Request) {
	if req.UserID == 0 || req.Type == "" || req.Title == "" {
		return
	}

	pref := models.GetNotificationPreference(db, req.UserID, req.Type)

	// In-app channel: insert into notifications table.
	if pref.InApp {
		_, err := models.CreateNotification(db, req.UserID, req.Type, req.Title, req.Message, req.Link, req.AthleteID)
		if err != nil {
			log.Printf("notify: in-app notification failed for user %d type %q: %v", req.UserID, req.Type, err)
		}
	}

	// External channel: send via Shoutrrr.
	if pref.External {
		sendExternal(db, req)
	}
}

// sendExternal dispatches a notification via Shoutrrr URLs configured in app settings.
func sendExternal(db *sql.DB, req Request) {
	urlsStr := models.GetSetting(db, "notify.urls")
	if urlsStr == "" {
		return
	}
	urls := parseURLs(urlsStr)
	if len(urls) == 0 {
		return
	}

	// Build the message body.
	body := req.Title
	if req.Message != "" {
		body = fmt.Sprintf("%s\n%s", body, req.Message)
	}
	if req.Link != "" {
		// Append the link. External services will display it as-is.
		// For full URLs, the caller should prepend the base URL.
		body = fmt.Sprintf("%s\n%s", body, req.Link)
	}

	// Fire-and-forget in a goroutine so we don't block the HTTP request.
	go func() {
		for _, u := range urls {
			if err := shoutrrr.Send(u, body); err != nil {
				log.Printf("notify: external send failed for url %q: %v", maskURL(u), err)
			}
		}
	}()
}

// SendDirect sends a message via configured Shoutrrr URLs without checking
// preferences or creating an in-app notification. Used for one-off messages
// like magic link delivery where the content itself is the notification.
func SendDirect(db *sql.DB, message string) {
	urlsStr := models.GetSetting(db, "notify.urls")
	if urlsStr == "" {
		return
	}
	urls := parseURLs(urlsStr)
	if len(urls) == 0 {
		return
	}

	go func() {
		for _, u := range urls {
			if err := shoutrrr.Send(u, message); err != nil {
				log.Printf("notify: direct send failed for url %q: %v", maskURL(u), err)
			}
		}
	}()
}

// TestConnection sends a test message through all configured Shoutrrr URLs.
// Returns an error if any URL fails, or nil if all succeed.
func TestConnection(db *sql.DB) error {
	urlsStr := models.GetSetting(db, "notify.urls")
	if urlsStr == "" {
		return fmt.Errorf("no notification URLs configured")
	}
	urls := parseURLs(urlsStr)
	if len(urls) == 0 {
		return fmt.Errorf("no valid notification URLs configured")
	}

	for _, u := range urls {
		if err := shoutrrr.Send(u, "RepLog test notification — if you see this, notifications are working!"); err != nil {
			return fmt.Errorf("failed to send to %s: %w", maskURL(u), err)
		}
	}
	return nil
}

// parseURLs splits a comma-or-newline-separated URL string and trims whitespace.
func parseURLs(urlsStr string) []string {
	// Support both comma and newline separators.
	urlsStr = strings.ReplaceAll(urlsStr, "\n", ",")
	parts := strings.Split(urlsStr, ",")
	var urls []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			urls = append(urls, p)
		}
	}
	return urls
}

// maskURL masks credentials in a Shoutrrr URL for safe logging.
func maskURL(u string) string {
	// Show scheme and first 10 chars, mask the rest.
	if len(u) <= 15 {
		return u[:5] + "••••"
	}
	return u[:15] + "••••"
}
