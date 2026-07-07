package api

import (
	"log"
	"net/http"

	"github.com/carpenike/replog/internal/models"
)

// TokenLogin validates a magic link token and creates a session.
// GET /api/auth/token/{token}
// TokenLogin authenticates a user via a magic-link token.
//
//	@Summary      Log in via magic-link token
//	@Description  Single-use token issued by an admin (POST /users/{userID}/tokens). Creates a session on success.
//	@Tags         Auth
//	@Produce      json
//	@Param        token  path      string  true  "Login token"
//	@Success      200  {object}  map[string]interface{}  "User + redirect URL"
//	@Failure      401  {object}  api.APIError
//	@Router       /auth/token/{token} [get]
func (h *Handlers) TokenLogin(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		WriteError(w, http.StatusBadRequest, "missing token")
		return
	}

	user, err := models.ValidateLoginToken(r.Context(), h.DB, token)
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

	if err := models.EnsureUserPreferences(r.Context(), h.DB, user.ID); err != nil {
		log.Printf("api: ensure preferences for user %d: %v", user.ID, err)
	}

	log.Printf("api: token login success for user %q (id=%d)", user.Username, user.ID)

	// Magic-link logins land directly in the app. The passkey-setup nudge was
	// retired with the passkey login path (ADR 019 Phase 1 — HOF-012); webui
	// auth now federates to PocketID and passwordless kids keep using magic
	// links with no setup wizard interstitial.
	WriteJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"redirect": "/",
	})
}
