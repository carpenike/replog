package api

import (
	"log"
	"net/http"

	"github.com/carpenike/replog/internal/models"
)

// TokenLogin validates a magic link token and creates a session.
// GET /api/auth/token/{token}
func (h *Handlers) TokenLogin(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		WriteError(w, http.StatusBadRequest, "missing token")
		return
	}

	user, err := models.ValidateLoginToken(h.DB, token)
	if err != nil {
		log.Printf("api: token login failed: %v", err)
		WriteError(w, http.StatusUnauthorized, "invalid or expired login link")
		return
	}

	// Renew session to prevent fixation.
	if err := h.Sessions.RenewToken(r.Context()); err != nil {
		log.Printf("api: session renew error on token login: %v", err)
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	h.Sessions.Put(r.Context(), "userID", user.ID)

	if err := models.EnsureUserPreferences(h.DB, user.ID); err != nil {
		log.Printf("api: ensure preferences for user %d: %v", user.ID, err)
	}

	log.Printf("api: token login success for user %q (id=%d)", user.Username, user.ID)

	// Check if user needs passkey setup.
	needsSetup := false
	if !h.Sessions.GetBool(r.Context(), "passkey_setup_skipped") {
		creds, err := models.ListWebAuthnCredentialsByUser(h.DB, user.ID)
		if err == nil && len(creds) == 0 {
			needsSetup = true
		}
	}

	redirect := "/"
	if needsSetup {
		redirect = "/setup/passkey"
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"redirect":    redirect,
		"needs_setup": needsSetup,
	})
}

// SkipPasskeySetup marks passkey setup as skipped for this session.
// POST /api/auth/setup/passkey/skip
func (h *Handlers) SkipPasskeySetup(w http.ResponseWriter, r *http.Request) {
	h.Sessions.Put(r.Context(), "passkey_setup_skipped", true)
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
