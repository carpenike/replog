package notify

import (
	"bytes"
	"database/sql"
	"embed"
	"html/template"
	"log"
	"sync"

	"github.com/carpenike/replog/internal/models"
)

//go:embed templates/*.html
var emailFS embed.FS

var (
	emailTemplates map[string]*template.Template
	emailOnce      sync.Once
)

// EmailData holds the common fields available to all email templates.
type EmailData struct {
	AppName  string // Application name (from app settings).
	BaseURL  string // Application base URL (optional).
	Title    string // Notification title / heading.
	Message  string // Longer body text (optional).
	Link     string // Action URL (optional).
	LinkText string // CTA button label (optional, defaults to "View Details").
	LoginURL string // Magic link URL (magic_link template only).
}

// parseEmailTemplates parses all email templates once on first use.
func parseEmailTemplates() {
	emailOnce.Do(func() {
		emailTemplates = make(map[string]*template.Template)

		base, err := emailFS.ReadFile("templates/base.html")
		if err != nil {
			log.Printf("notify: failed to read base email template: %v", err)
			return
		}

		pages := []string{"magic_link.html", "notification.html"}
		for _, page := range pages {
			content, err := emailFS.ReadFile("templates/" + page)
			if err != nil {
				log.Printf("notify: failed to read email template %s: %v", page, err)
				continue
			}

			// Parse page first (it defines the blocks), then base (which uses them).
			// The page template calls {{ template "base.html" . }}, so base must be
			// available under that name.
			t, err := template.New(page).Parse(string(content))
			if err != nil {
				log.Printf("notify: failed to parse email template %s: %v", page, err)
				continue
			}
			t, err = t.New("base.html").Parse(string(base))
			if err != nil {
				log.Printf("notify: failed to parse base into %s: %v", page, err)
				continue
			}

			emailTemplates[page] = t
		}
	})
}

// renderEmail renders the named email template with the given data and returns
// the HTML string. Returns empty string on any error (logged internally).
func renderEmail(templateName string, data EmailData) string {
	parseEmailTemplates()

	t, ok := emailTemplates[templateName]
	if !ok {
		log.Printf("notify: unknown email template %q", templateName)
		return ""
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, templateName, data); err != nil {
		log.Printf("notify: render email template %q: %v", templateName, err)
		return ""
	}
	return buf.String()
}

// RenderMagicLinkEmail renders the magic link email template with app branding.
// Returns the full HTML body ready to pass to SendToUser.
func RenderMagicLinkEmail(db *sql.DB, loginURL string) string {
	return renderEmail("magic_link.html", EmailData{
		AppName:  models.GetAppName(db),
		BaseURL:  models.GetSetting(db, "app.base_url"),
		LoginURL: loginURL,
	})
}

// RenderNotificationEmail renders the general notification email template.
// Returns the full HTML body ready to pass to SendToUser.
func RenderNotificationEmail(db *sql.DB, title, message, link string) string {
	return renderEmail("notification.html", EmailData{
		AppName: models.GetAppName(db),
		BaseURL: models.GetSetting(db, "app.base_url"),
		Title:   title,
		Message: message,
		Link:    link,
	})
}
