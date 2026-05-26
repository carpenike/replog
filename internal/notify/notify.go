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
	"context"
	"database/sql"
	"errors"
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
		// HTML email to the target user.
		link := req.Link
		if link != "" && !strings.HasPrefix(link, "http") {
			if baseURL := models.GetSetting(db, "app.base_url"); baseURL != "" {
				link = strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(link, "/")
			}
		}
		htmlBody := renderEmail("notification.html", EmailData{
			AppName: models.GetAppName(db),
			BaseURL: models.GetSetting(db, "app.base_url"),
			Title:   req.Title,
			Message: req.Message,
			Link:    link,
		})
		if htmlBody != "" {
			sendHTMLToUser(db, req.UserID, req.Title, htmlBody)
		} else {
			// Fallback to plain text if template rendering fails.
			sendToUser(db, req.UserID, req.Title, buildBody(req))
		}

		// Plain text to broadcast channels (ntfy, Discord, etc.).
		sendBroadcast(db, buildBody(req))
	}
}

// SendToUser sends an HTML email to a specific user's email address.
// Used for targeted delivery like magic links where only the recipient should
// see the message. Does not check preferences or create in-app notifications.
func SendToUser(db *sql.DB, userID int64, subject, htmlBody string) {
	sendHTMLToUser(db, userID, subject, htmlBody)
}

// sendHTMLToUser sends an HTML email to the target user using app-level SMTP settings.
// Silently returns if SMTP is not configured or the user has no email.
func sendHTMLToUser(db *sql.DB, userID int64, subject, htmlBody string) {
	smtpURL := buildSMTPURL(db, userID, subject, true)
	if smtpURL == "" {
		return
	}

	go func() {
		if err := shoutrrr.Send(smtpURL, htmlBody); err != nil {
			log.Printf("notify: HTML email send failed for user %d: %v", userID, err)
		}
	}()
}

// sendToUser sends a plain text email to the target user using app-level SMTP settings.
// Silently returns if SMTP is not configured or the user has no email.
func sendToUser(db *sql.DB, userID int64, subject, body string) {
	smtpURL := buildSMTPURL(db, userID, subject, false)
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

// sendFn is the package's reference to shoutrrr.Send, swapped by tests to
// simulate slow/hung upstream endpoints without real network calls.
var sendFn = shoutrrr.Send

// SetSendFnForTesting replaces the package's send function and returns a
// restore func. Tests in dependent packages use this to inject a stub
// (typically one that blocks on ctx.Done) without touching the network.
//
//	restore := notify.SetSendFnForTesting(myStub)
//	t.Cleanup(restore)
func SetSendFnForTesting(fn func(url, message string) error) func() {
	prev := sendFn
	sendFn = fn
	return func() { sendFn = prev }
}

// sendWithContext invokes sendFn in a goroutine and races it against
// ctx.Done. If ctx fires first, returns ctx.Err() immediately. The inner
// goroutine is left to finish on its own and its result is logged via
// `label` (it cannot exceed the underlying socket timeouts: ~30s for SMTP,
// the HTTP client default for webhooks).
//
// This is the contract documented in issue #12: shoutrrr does not honor
// a context, so the only way to bound it is to race + leak. We accept
// the bounded leak in exchange for never blocking the HTTP server past
// its WriteTimeout.
func sendWithContext(ctx context.Context, urlStr, body, label string) error {
	done := make(chan error, 1)
	go func() { done <- sendFn(urlStr, body) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		go func() {
			if err := <-done; err != nil {
				log.Printf("notify: %s send finished after timeout with error: %v", label, err)
			}
		}()
		return ctx.Err()
	}
}

// TestConnection tests all configured notification channels.
// Tests SMTP by sending to the from address, and each broadcast URL.
//
// Each individual send is bounded by ctx — a hung SMTP server or webhook
// returns ctx.Err() as a per-channel failure rather than blocking the whole
// admin test-connection request past the HTTP server's WriteTimeout.
func TestConnection(ctx context.Context, db *sql.DB) error {
	var errs []string

	// Test SMTP if configured.
	if smtpHost := models.GetSetting(db, "smtp.host"); smtpHost != "" {
		fromAddr := models.GetSetting(db, "smtp.from")
		if fromAddr == "" {
			errs = append(errs, "SMTP: from address not configured")
		} else {
			// Send test to the from address itself.
			testURL := buildSMTPURLDirect(db, fromAddr, "RepLog SMTP Test", false)
			if testURL != "" {
				if err := sendWithContext(ctx, testURL, "If you see this, RepLog SMTP is working!", "SMTP test"); err != nil {
					if errors.Is(err, context.DeadlineExceeded) {
						errs = append(errs, "SMTP: server did not respond in time")
					} else {
						errs = append(errs, fmt.Sprintf("SMTP: %v", err))
					}
				}
			}
		}
	}

	// Test broadcast URLs.
	urlsStr := models.GetSetting(db, "notify.urls")
	if urlsStr != "" {
		for _, u := range parseURLs(urlsStr) {
			if err := sendWithContext(ctx, u, "RepLog test — if you see this, notifications are working!", "broadcast "+maskURL(u)); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					errs = append(errs, fmt.Sprintf("Broadcast %s: did not respond in time", maskURL(u)))
				} else {
					errs = append(errs, fmt.Sprintf("Broadcast %s: %v", maskURL(u), err))
				}
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
// When html is true, sets usehtml=Yes so Shoutrrr sends Content-Type: text/html.
func buildSMTPURL(db *sql.DB, userID int64, subject string, html bool) string {
	user, err := models.GetUserByID(db, userID)
	if err != nil || !user.Email.Valid || user.Email.String == "" {
		return ""
	}
	return buildSMTPURLDirect(db, user.Email.String, subject, html)
}

// buildSMTPURLDirect constructs a Shoutrrr SMTP URL for a given email address.
// Returns empty string if SMTP is not configured.
// When html is true, sets usehtml=Yes so Shoutrrr sends Content-Type: text/html.
func buildSMTPURLDirect(db *sql.DB, toEmail, subject string, html bool) string {
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
	if html {
		params.Set("usehtml", "Yes")
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
