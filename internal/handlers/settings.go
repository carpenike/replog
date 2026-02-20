package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/carpenike/replog/internal/llm"
	"github.com/carpenike/replog/internal/models"
)

// Settings handles application settings management (admin-only).
type Settings struct {
	DB        *sql.DB
	Templates TemplateCache
}

// Show renders the settings page grouped by category.
func (h *Settings) Show(w http.ResponseWriter, r *http.Request) {
	groups := models.ListSettingsByCategory(h.DB)

	data := map[string]any{
		"SettingGroups": groups,
		"Registry":      models.SettingsRegistry,
	}
	if err := h.Templates.Render(w, r, "settings.html", data); err != nil {
		log.Printf("handlers: render settings: %v", err)
	}
}

// Update handles settings form submission.
func (h *Settings) Update(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, "Invalid form data.")
		return
	}

	var updated int
	var errors []string

	for _, def := range models.SettingsRegistry {
		// Skip read-only (env-var-set) settings.
		sv := models.GetSettingValue(h.DB, def.Key)
		if sv.ReadOnly {
			continue
		}

		newValue := r.FormValue("setting_" + def.Key)
		oldValue := sv.Value

		// If value cleared, delete the row (revert to default).
		if newValue == "" && oldValue != "" {
			if err := models.DeleteSetting(h.DB, def.Key); err != nil {
				log.Printf("handlers: delete setting %q: %v", def.Key, err)
				errors = append(errors, "Failed to clear "+def.Label)
			} else {
				updated++
			}
			continue
		}

		// For sensitive fields, if the masked placeholder was submitted, skip.
		if def.Sensitive && isMaskedPlaceholder(newValue) {
			continue
		}

		// Save if changed.
		if newValue != oldValue && newValue != "" {
			if err := models.SetSetting(h.DB, def.Key, newValue); err != nil {
				log.Printf("handlers: set setting %q: %v", def.Key, err)
				if def.Sensitive {
					errors = append(errors, "Failed to save "+def.Label+" — is REPLOG_SECRET_KEY set?")
				} else {
					errors = append(errors, "Failed to save "+def.Label)
				}
			} else {
				updated++
			}
		}
	}

	groups := models.ListSettingsByCategory(h.DB)
	data := map[string]any{
		"SettingGroups": groups,
		"Registry":      models.SettingsRegistry,
	}
	if len(errors) > 0 {
		data["Error"] = errors[0]
		w.WriteHeader(http.StatusUnprocessableEntity)
	} else if updated > 0 {
		data["Success"] = "Settings saved."
	} else {
		data["Success"] = "No changes."
	}

	if err := h.Templates.Render(w, r, "settings.html", data); err != nil {
		log.Printf("handlers: render settings: %v", err)
	}
}

// renderError renders the settings page with an error message.
func (h *Settings) renderError(w http.ResponseWriter, r *http.Request, msg string) {
	groups := models.ListSettingsByCategory(h.DB)
	data := map[string]any{
		"SettingGroups": groups,
		"Registry":      models.SettingsRegistry,
		"Error":         msg,
	}
	w.WriteHeader(http.StatusUnprocessableEntity)
	if err := h.Templates.Render(w, r, "settings.html", data); err != nil {
		log.Printf("handlers: render settings error: %v", err)
	}
}

// isMaskedPlaceholder returns true if the value is the masked placeholder
// (submitted when a sensitive field wasn't changed by the user).
func isMaskedPlaceholder(v string) bool {
	// The mask format is "xxxx••••xxxx" or "••••••••" — check for bullet chars.
	for _, r := range v {
		if r == '•' {
			return true
		}
	}
	return false
}

// TestConnection attempts to connect to the configured LLM provider and
// returns a success/failure message as an HTML fragment (for htmx swap).
func (h *Settings) TestConnection(w http.ResponseWriter, r *http.Request) {
	provider, err := llm.NewProviderFromSettings(h.DB)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`<span class="test-result test-error">Not configured. Please set an AI Coach provider, model, and API key first.</span>`))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := provider.Ping(ctx); err != nil {
		log.Printf("handlers: test LLM connection: %v", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)

		var apiErr *llm.APIError
		msg := err.Error()
		if errors.As(err, &apiErr) {
			msg = apiErr.UserMessage()
		}
		w.Write([]byte(`<span class="test-result test-error">` + msg + `</span>`))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<span class="test-result test-success">Connected to ` + provider.Name() + ` successfully!</span>`))
}
