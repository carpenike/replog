// Package notify provides channel-agnostic notification dispatch.
//
// Two delivery modes:
//   - Per-user: email sent to the target user's address via app-level SMTP config.
//   - Broadcast: sent to globally configured Shoutrrr URLs (ntfy, Discord, etc.).
//
// Producers call Send() with a notification request. The dispatcher checks
// per-user preferences and routes to in-app (SQLite) and/or external channels.
package notify

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
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

	// External channel: email the target user and/or broadcast.
	if pref.External {
		body := buildBody(req)
		sendToUser(db, req.UserID, req.Title, body)
		sendBroadcast(db, body)
	}
}

// SendToUser sends a message directly to a specific user's email address.
// Used for targeted delivery like magic links where only the recipient should
// see the message. Does not check preferences or create in-app notifications.
func SendToUser(db *sql.DB, userID int64, subject, body string) {
	sendToUser(db, userID, subject, body)
}

// sendToUser sends an email to the target user using app-level SMTP settings.
// Silently returns if SMTP is not configured or the user has no email.
func sendToUser(db *sql.DB, userID int64, subject, body string) {
	smtpURL := buildSMTPURL(db, userID, subject)
	if smtpURL == "" {
		return
	}

	go func() {
		if err := shoutrrr.Send(smtpURL, body); err != nil {
			log.Printf("notify: email send failed for user %d: %v", userID, err)
		}
	}()
}

// sendBroadcast sends a message to all globally configured Shoutrrr URLs
// (ntfy, Discord, etc.). These are admin/broadcast channels, not per-user.
func sendBroadcast(db *sql.DB, body string) {
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
			if err := shoutrrr.Send(u, body); err != nil {
				log.Printf("notify: broadcast send failed for url %q: %v", maskURL(u), err)
			}
		}
	}()
}

// SendBroadcast sends a message to all globally configured broadcast URLs
// without targeting a specific user. Used for system-wide announcements.
func SendBroadcast(db *sql.DB, body string) {
	sendBroadcast(db, body)
}

// TestConnection tests all configured notification channels.
// Tests SMTP by sending to the from address, and each broadcast URL.
func TestConnection(db *sql.DB) error {
	var errs []string

	// Test SMTP if configured.
	if smtpHost := models.GetSetting(db, "smtp.host"); smtpHost != "" {
		fromAddr := models.GetSetting(db, "smtp.from")
		if fromAddr == "" {
			errs = append(errs, "SMTP: from address not configured")
		} else {
			// Send test to the from address itself.
			testURL := buildSMTPURLDirect(db, fromAddr, "RepLog SMTP Test")
			if testURL != "" {
				if err := shoutrrr.Send(testURL, "If you see this, RepLog SMTP is working!"); err != nil {
					errs = append(errs, fmt.Sprintf("SMTP: %v", err))
				}
			}
		}
	}

	// Test broadcast URLs.
	urlsStr := models.GetSetting(db, "notify.urls")
	if urlsStr != "" {
		for _, u := range parseURLs(urlsStr) {
			if err := shoutrrr.Send(u, "RepLog test — if you see this, notifications are working!"); err != nil {
				errs = append(errs, fmt.Sprintf("Broadcast %s: %v", maskURL(u), err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	// Check that at least one channel is configured.
	if models.GetSetting(db, "smtp.host") == "" && urlsStr == "" {
		return fmt.Errorf("no notification channels configured (set SMTP or broadcast URLs)")
	}

	return nil
}

// --- SMTP URL builders ---

// buildSMTPURL constructs a Shoutrrr SMTP URL for a specific user.
// Returns empty string if SMTP is not configured or the user has no email.
func buildSMTPURL(db *sql.DB, userID int64, subject string) string {
	user, err := models.GetUserByID(db, userID)
	if err != nil || !user.Email.Valid || user.Email.String == "" {
		return ""
	}
	return buildSMTPURLDirect(db, user.Email.String, subject)
}

// buildSMTPURLDirect constructs a Shoutrrr SMTP URL for a given email address.
// Returns empty string if SMTP is not configured.
func buildSMTPURLDirect(db *sql.DB, toEmail, subject string) string {
	host := models.GetSetting(db, "smtp.host")
	if host == "" {
		return ""
	}
	port := models.GetSetting(db, "smtp.port")
	if port == "" {
		port = "587"
	}
	username := models.GetSetting(db, "smtp.username")
	password := models.GetSetting(db, "smtp.password")
	fromAddr := models.GetSetting(db, "smtp.from")
	if fromAddr == "" {
		return ""
	}

	// Build: smtp://username:password@host:port/?from=X&to=Y&subject=Z
	var userInfo string
	if username != "" {
		if password != "" {
			userInfo = url.PathEscape(username) + ":" + url.PathEscape(password) + "@"
		} else {
			userInfo = url.PathEscape(username) + "@"
		}
	}

	params := url.Values{}
	params.Set("from", fromAddr)
	params.Set("to", toEmail)
	if subject != "" {
		params.Set("subject", subject)
	}

	return fmt.Sprintf("smtp://%s%s:%s/?%s", userInfo, host, port, params.Encode())
}

// --- Helpers ---

// buildBody constructs the message body from a Request.
func buildBody(req Request) string {
	body := req.Title
	if req.Message != "" {
		body = fmt.Sprintf("%s\n%s", body, req.Message)
	}
	if req.Link != "" {
		body = fmt.Sprintf("%s\n%s", body, req.Link)
	}
	return body
}

// parseURLs splits a comma-or-newline-separated URL string and trims whitespace.
func parseURLs(urlsStr string) []string {
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
	if len(u) <= 15 {
		return u[:5] + "••••"
	}
	return u[:15] + "••••"
}
