package handlers

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// Preferences handles user preference management (self-service).
type Preferences struct {
	DB        *sql.DB
	Templates TemplateCache
}

// EditForm renders the preferences form for the current user.
func (h *Preferences) EditForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	prefs, err := models.GetUserPreferences(h.DB, user.ID)
	if err != nil {
		log.Printf("handlers: get preferences for user %d: %v", user.ID, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	passkeys, err := models.ListWebAuthnCredentialsByUser(h.DB, user.ID)
	if err != nil {
		log.Printf("handlers: list passkeys for user %d: %v", user.ID, err)
		// Non-fatal — render without passkeys.
		passkeys = nil
	}

	data := map[string]any{
		"EditPrefs":       prefs,
		"WeightUnits":     models.ValidWeightUnits,
		"DateFormats":     models.ValidDateFormats,
		"CommonTimezones": commonTimezones,
		"Passkeys":        passkeys,
		"UserID":          user.ID,
		"AvatarUser":      user,
	}
	if err := h.Templates.Render(w, r, "preferences_form.html", data); err != nil {
		log.Printf("handlers: render preferences form: %v", err)
	}
}

// Update handles the preferences form submission.
func (h *Preferences) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	weightUnit := r.FormValue("weight_unit")
	timezone := r.FormValue("timezone")
	dateFormat := r.FormValue("date_format")

	if weightUnit == "" || timezone == "" || dateFormat == "" {
		h.renderFormError(w, r, "All fields are required.", user.ID)
		return
	}

	_, err := models.UpsertUserPreferences(h.DB, user.ID, weightUnit, timezone, dateFormat)
	if err != nil {
		log.Printf("handlers: update preferences for user %d: %v", user.ID, err)
		h.renderFormError(w, r, "Invalid preferences. Please check your selections.", user.ID)
		return
	}

	// Redirect back — preferences will reload from context on next request.
	http.Redirect(w, r, "/preferences", http.StatusSeeOther)
}

// renderFormError re-renders the preferences form with an error message.
func (h *Preferences) renderFormError(w http.ResponseWriter, r *http.Request, msg string, userID int64) {
	prefs, _ := models.GetUserPreferences(h.DB, userID)
	user, _ := models.GetUserByID(h.DB, userID)
	data := map[string]any{
		"Error":           msg,
		"EditPrefs":       prefs,
		"WeightUnits":     models.ValidWeightUnits,
		"DateFormats":     models.ValidDateFormats,
		"CommonTimezones": commonTimezones,
		"AvatarUser":      user,
	}
	w.WriteHeader(http.StatusUnprocessableEntity)
	if err := h.Templates.Render(w, r, "preferences_form.html", data); err != nil {
		log.Printf("handlers: render preferences form error: %v", err)
	}
}

// commonTimezones is a curated list of IANA timezone identifiers for the form dropdown.
var commonTimezones = []string{
	"America/New_York",
	"America/Chicago",
	"America/Denver",
	"America/Los_Angeles",
	"America/Anchorage",
	"Pacific/Honolulu",
	"America/Phoenix",
	"America/Toronto",
	"America/Vancouver",
	"Europe/London",
	"Europe/Paris",
	"Europe/Berlin",
	"Europe/Moscow",
	"Asia/Tokyo",
	"Asia/Shanghai",
	"Asia/Kolkata",
	"Asia/Dubai",
	"Australia/Sydney",
	"Australia/Perth",
	"Pacific/Auckland",
	"UTC",
}
