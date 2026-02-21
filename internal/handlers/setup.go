package handlers

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// WizardStep represents a single step in a multi-step setup wizard.
// Templates use WizardSteps (slice) and WizardCurrentStep (int) to
// render a progress indicator. This type is intentionally generic so
// it can be reused across different wizard flows (passkey setup,
// athlete onboarding, program configuration, etc.).
type WizardStep struct {
	Number int
	Label  string
}

// Setup holds dependencies for setup/onboarding wizard handlers.
type Setup struct {
	DB        *sql.DB
	Sessions  *scs.SessionManager
	Templates TemplateCache
}

// PasskeySetup renders the passkey enrollment wizard page.
// GET /setup/passkey
func (h *Setup) PasskeySetup(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Check if the user just completed registration (redirected with ?done=1).
	success := r.URL.Query().Get("done") == "1"

	data := map[string]any{
		"WizardSteps": []WizardStep{
			{Number: 1, Label: "Sign In"},
			{Number: 2, Label: "Register Passkey"},
			{Number: 3, Label: "Done"},
		},
		"WizardCurrentStep": 2,
		"SkipURL":           "/",
	}

	if success {
		data["WizardCurrentStep"] = 3
		data["Success"] = true
	}

	if err := h.Templates.Render(w, r, "setup_passkey.html", data); err != nil {
		log.Printf("handlers: render passkey setup: %v", err)
		h.Templates.ServerError(w, r)
	}
}

// PasskeySetupSkip marks the passkey setup as skipped and redirects home.
// POST /setup/passkey/skip
func (h *Setup) PasskeySetupSkip(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	h.Sessions.Put(r.Context(), "passkey_setup_skipped", true)

	// For htmx requests, return empty 200 so hx-swap="delete" works.
	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// NeedsPasskeySetup reports whether a user should be directed to the passkey
// enrollment wizard. Returns true when the user has zero registered passkeys
// and hasn't explicitly skipped the setup in this session.
func (h *Setup) NeedsPasskeySetup(r *http.Request, userID int64) bool {
	// Respect the "skip" flag stored in the session.
	if h.Sessions.GetBool(r.Context(), "passkey_setup_skipped") {
		return false
	}

	creds, err := models.ListWebAuthnCredentialsByUser(h.DB, userID)
	if err != nil {
		log.Printf("handlers: check passkey setup for user %d: %v", userID, err)
		return false // fail open — don't block login
	}
	return len(creds) == 0
}
