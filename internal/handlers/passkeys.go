package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/alexedwards/scs/v2"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// Passkeys holds dependencies for WebAuthn/passkey handlers.
type Passkeys struct {
	DB        *sql.DB
	Sessions  *scs.SessionManager
	WebAuthn  *webauthn.WebAuthn
	Templates TemplateCache
}

// BeginRegistration starts a passkey registration ceremony for the current user.
// GET /passkeys/register/begin
func (h *Passkeys) BeginRegistration(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	waUser := models.NewWebAuthnUser(user, h.DB)
	if err := waUser.LoadCredentials(); err != nil {
		log.Printf("handlers: load webauthn credentials for user %d: %v", user.ID, err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	creation, session, err := h.WebAuthn.BeginRegistration(waUser)
	if err != nil {
		log.Printf("handlers: begin webauthn registration for user %d: %v", user.ID, err)
		jsonError(w, "Failed to start registration", http.StatusInternalServerError)
		return
	}

	// Store session data in the HTTP session for the finish step.
	sessionBytes, err := json.Marshal(session)
	if err != nil {
		log.Printf("handlers: marshal webauthn session: %v", err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	h.Sessions.Put(r.Context(), "webauthn_registration", string(sessionBytes))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(creation); err != nil {
		log.Printf("handlers: encode registration options: %v", err)
	}
}

// FinishRegistration completes a passkey registration ceremony.
// POST /passkeys/register/finish
func (h *Passkeys) FinishRegistration(w http.ResponseWriter, r *http.Request) {
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
		log.Printf("handlers: unmarshal webauthn session: %v", err)
		jsonError(w, "Invalid session", http.StatusBadRequest)
		return
	}

	waUser := models.NewWebAuthnUser(user, h.DB)
	if err := waUser.LoadCredentials(); err != nil {
		log.Printf("handlers: load webauthn credentials for user %d: %v", user.ID, err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	credential, err := h.WebAuthn.FinishRegistration(waUser, session, r)
	if err != nil {
		log.Printf("handlers: finish webauthn registration for user %d: %v", user.ID, err)
		jsonError(w, "Registration failed", http.StatusBadRequest)
		return
	}

	// Extract label from query parameter (set before the ceremony started).
	label := h.Sessions.PopString(r.Context(), "webauthn_label")

	if _, err := models.CreateWebAuthnCredential(h.DB, user.ID, credential, label); err != nil {
		log.Printf("handlers: store webauthn credential for user %d: %v", user.ID, err)
		jsonError(w, "Failed to save credential", http.StatusInternalServerError)
		return
	}

	log.Printf("handlers: webauthn credential registered for user %q (id=%d)", user.Username, user.ID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// BeginLogin starts a passkey login ceremony (discoverable — no username needed).
// GET /passkeys/login/begin
func (h *Passkeys) BeginLogin(w http.ResponseWriter, r *http.Request) {
	assertion, session, err := h.WebAuthn.BeginDiscoverableLogin()
	if err != nil {
		log.Printf("handlers: begin webauthn discoverable login: %v", err)
		jsonError(w, "Failed to start login", http.StatusInternalServerError)
		return
	}

	sessionBytes, err := json.Marshal(session)
	if err != nil {
		log.Printf("handlers: marshal webauthn login session: %v", err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	h.Sessions.Put(r.Context(), "webauthn_login", string(sessionBytes))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(assertion); err != nil {
		log.Printf("handlers: encode login options: %v", err)
	}
}

// FinishLogin completes a passkey login ceremony.
// POST /passkeys/login/finish
func (h *Passkeys) FinishLogin(w http.ResponseWriter, r *http.Request) {
	sessionJSON := h.Sessions.PopString(r.Context(), "webauthn_login")
	if sessionJSON == "" {
		jsonError(w, "No login in progress", http.StatusBadRequest)
		return
	}

	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		log.Printf("handlers: unmarshal webauthn login session: %v", err)
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
		log.Printf("handlers: finish webauthn login: %v", err)
		jsonError(w, "Login failed", http.StatusUnauthorized)
		return
	}

	// Update sign count in database.
	if err := models.UpdateWebAuthnCredentialSignCount(
		h.DB, credential.ID, credential.Authenticator.SignCount, credential.Authenticator.CloneWarning,
	); err != nil {
		log.Printf("handlers: update webauthn sign count: %v", err)
		// Non-fatal — continue with login.
	}

	waUser, ok := foundUser.(*models.WebAuthnUser)
	if !ok {
		log.Printf("handlers: unexpected user type from passkey login")
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Renew session to prevent fixation.
	if err := h.Sessions.RenewToken(r.Context()); err != nil {
		log.Printf("handlers: session renew error on passkey login: %v", err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.Sessions.Put(r.Context(), "userID", waUser.User.ID)

	// Ensure default preferences.
	if err := models.EnsureUserPreferences(h.DB, waUser.User.ID); err != nil {
		log.Printf("handlers: ensure preferences for user %d: %v", waUser.User.ID, err)
	}

	log.Printf("handlers: passkey login success for user %q (id=%d)", waUser.User.Username, waUser.User.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "redirect": "/"})
}

// ListCredentials returns the passkeys management fragment for a user.
// GET /users/{id}/passkeys (htmx fragment)
func (h *Passkeys) ListCredentials(w http.ResponseWriter, r *http.Request) {
	// This is handled via the user edit form — credentials are loaded there.
	// Kept as a stub for potential future use.
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// DeleteCredential removes a passkey credential.
// POST /users/{id}/passkeys/{credentialID}/delete
func (h *Passkeys) DeleteCredential(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Only coaches can manage other users' passkeys; users can manage their own.
	if !authUser.IsCoach && authUser.ID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	credID, err := strconv.ParseInt(r.PathValue("credentialID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid credential ID", http.StatusBadRequest)
		return
	}

	if err := models.DeleteWebAuthnCredential(h.DB, credID); err != nil {
		log.Printf("handlers: delete webauthn credential %d: %v", credID, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Return updated passkeys list for htmx swap.
	creds, err := models.ListWebAuthnCredentialsByUser(h.DB, userID)
	if err != nil {
		log.Printf("handlers: list webauthn credentials for user %d: %v", userID, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Passkeys":  creds,
		"UserID":    userID,
		"CSRFToken": middleware.CSRFTokenFromContext(r.Context()),
	}

	ts, ok := h.Templates["passkeys_list.html"]
	if !ok {
		log.Printf("handlers: passkeys_list.html template not found")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if err := ts.ExecuteTemplate(w, "passkeys-list", data); err != nil {
		log.Printf("handlers: render passkeys list: %v", err)
	}
}

// SetLabel stores the passkey label in the session before the ceremony.
// POST /passkeys/register/label
func (h *Passkeys) SetLabel(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	label := r.FormValue("label")
	h.Sessions.Put(r.Context(), "webauthn_label", label)
	w.WriteHeader(http.StatusNoContent)
}

// jsonError writes a JSON error response.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
