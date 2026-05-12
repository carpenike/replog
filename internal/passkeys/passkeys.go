// Package passkeys implements the WebAuthn ceremony endpoints used by the
// SPA frontend (registration begin/finish and discoverable login begin/finish).
//
// These handlers exchange JSON WebAuthn protocol messages with the browser.
// Credential management (list/delete/label) lives in internal/api alongside
// the rest of the JSON REST API.
package passkeys

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// Handler holds dependencies for WebAuthn passkey ceremony endpoints.
type Handler struct {
	DB       *sql.DB
	Sessions *scs.SessionManager
	WebAuthn *webauthn.WebAuthn
}

// BeginRegistration starts a passkey registration ceremony for the current user.
// GET /api/passkeys/register/begin
func (h *Handler) BeginRegistration(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	waUser := models.NewWebAuthnUser(user, h.DB)
	if err := waUser.LoadCredentials(); err != nil {
		log.Printf("passkeys: load webauthn credentials for user %d: %v", user.ID, err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	creation, session, err := h.WebAuthn.BeginRegistration(waUser)
	if err != nil {
		log.Printf("passkeys: begin webauthn registration for user %d: %v", user.ID, err)
		jsonError(w, "Failed to start registration", http.StatusInternalServerError)
		return
	}

	sessionBytes, err := json.Marshal(session)
	if err != nil {
		log.Printf("passkeys: marshal webauthn session: %v", err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	h.Sessions.Put(r.Context(), "webauthn_registration", string(sessionBytes))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(creation); err != nil {
		log.Printf("passkeys: encode registration options: %v", err)
	}
}

// FinishRegistration completes a passkey registration ceremony.
// POST /api/passkeys/register/finish
func (h *Handler) FinishRegistration(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionJSON := h.Sessions.PopString(r.Context(), "webauthn_registration")
	if sessionJSON == "" {
		jsonError(w, "No registration in progress", http.StatusBadRequest)
		return
	}

	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		log.Printf("passkeys: unmarshal webauthn session: %v", err)
		jsonError(w, "Invalid session", http.StatusBadRequest)
		return
	}

	waUser := models.NewWebAuthnUser(user, h.DB)
	if err := waUser.LoadCredentials(); err != nil {
		log.Printf("passkeys: load webauthn credentials for user %d: %v", user.ID, err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	credential, err := h.WebAuthn.FinishRegistration(waUser, session, r)
	if err != nil {
		log.Printf("passkeys: finish webauthn registration for user %d: %v", user.ID, err)
		jsonError(w, "Registration failed", http.StatusBadRequest)
		return
	}

	// Label was set in the session before the ceremony started (via /api/passkeys/label).
	label := h.Sessions.PopString(r.Context(), "webauthn_label")

	if _, err := models.CreateWebAuthnCredential(h.DB, user.ID, credential, label); err != nil {
		log.Printf("passkeys: store webauthn credential for user %d: %v", user.ID, err)
		jsonError(w, "Failed to save credential", http.StatusInternalServerError)
		return
	}

	log.Printf("passkeys: webauthn credential registered for user %q (id=%d)", user.Username, user.ID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// BeginLogin starts a passkey login ceremony (discoverable — no username needed).
// GET /api/passkeys/login/begin
func (h *Handler) BeginLogin(w http.ResponseWriter, r *http.Request) {
	assertion, session, err := h.WebAuthn.BeginDiscoverableLogin()
	if err != nil {
		log.Printf("passkeys: begin webauthn discoverable login: %v", err)
		jsonError(w, "Failed to start login", http.StatusInternalServerError)
		return
	}

	sessionBytes, err := json.Marshal(session)
	if err != nil {
		log.Printf("passkeys: marshal webauthn login session: %v", err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	h.Sessions.Put(r.Context(), "webauthn_login", string(sessionBytes))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(assertion); err != nil {
		log.Printf("passkeys: encode login options: %v", err)
	}
}

// FinishLogin completes a passkey login ceremony.
// POST /api/passkeys/login/finish
func (h *Handler) FinishLogin(w http.ResponseWriter, r *http.Request) {
	sessionJSON := h.Sessions.PopString(r.Context(), "webauthn_login")
	if sessionJSON == "" {
		jsonError(w, "No login in progress", http.StatusBadRequest)
		return
	}

	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		log.Printf("passkeys: unmarshal webauthn login session: %v", err)
		jsonError(w, "Invalid session", http.StatusBadRequest)
		return
	}

	// Discoverable login handler: look up user by credential's userHandle.
	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		userID := models.UserIDFromWebAuthnID(userHandle)
		user, err := models.GetUserByID(h.DB, userID)
		if err != nil {
			return nil, err
		}
		waUser := models.NewWebAuthnUser(user, h.DB)
		if err := waUser.LoadCredentials(); err != nil {
			return nil, err
		}
		return waUser, nil
	}

	foundUser, credential, err := h.WebAuthn.FinishPasskeyLogin(handler, session, r)
	if err != nil {
		log.Printf("passkeys: finish webauthn login: %v", err)
		jsonError(w, "Passkey not recognized. It may have been removed or belong to a different account. Try signing in with a password or login link instead.", http.StatusUnauthorized)
		return
	}

	// Update sign count in database.
	if err := models.UpdateWebAuthnCredentialSignCount(
		h.DB, credential.ID, credential.Authenticator.SignCount, credential.Authenticator.CloneWarning,
	); err != nil {
		log.Printf("passkeys: update webauthn sign count: %v", err)
		// Non-fatal — continue with login.
	}

	waUser, ok := foundUser.(*models.WebAuthnUser)
	if !ok {
		log.Printf("passkeys: unexpected user type from passkey login")
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Renew session to prevent fixation.
	if err := h.Sessions.RenewToken(r.Context()); err != nil {
		log.Printf("passkeys: session renew error on passkey login: %v", err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.Sessions.Put(r.Context(), "userID", waUser.User.ID)

	// Ensure default preferences.
	if err := models.EnsureUserPreferences(h.DB, waUser.User.ID); err != nil {
		log.Printf("passkeys: ensure preferences for user %d: %v", waUser.User.ID, err)
	}

	log.Printf("passkeys: passkey login success for user %q (id=%d)", waUser.User.Username, waUser.User.ID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "redirect": "/"})
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
