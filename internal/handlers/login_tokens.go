package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/alexedwards/scs/v2"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
	"github.com/carpenike/replog/internal/notify"
)

// LoginTokens holds dependencies for login token management handlers.
type LoginTokens struct {
	DB        *sql.DB
	Sessions  *scs.SessionManager
	Templates TemplateCache
	BaseURL   string // External base URL (e.g. https://replog.example.com). If empty, inferred from request.
}

// TokenLogin handles passwordless login via a magic token URL.
// GET /auth/token/{token}
func (h *LoginTokens) TokenLogin(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}

	user, err := models.ValidateLoginToken(h.DB, token)
	if err != nil {
		log.Printf("handlers: token login failed: %v", err)
		// Redirect to login page with a generic error — don't reveal token validity.
		h.Sessions.Put(r.Context(), "flash_error", "Invalid or expired login link")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Renew session token to prevent fixation.
	if err := h.Sessions.RenewToken(r.Context()); err != nil {
		log.Printf("handlers: session renew error on token login: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.Sessions.Put(r.Context(), "userID", user.ID)

	// Ensure default preferences exist for this user.
	if err := models.EnsureUserPreferences(h.DB, user.ID); err != nil {
		log.Printf("handlers: ensure preferences for user %d: %v", user.ID, err)
	}

	log.Printf("handlers: token login success for user %q (id=%d)", user.Username, user.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// GenerateToken creates a new login token for a user.
// POST /users/{id}/tokens
func (h *LoginTokens) GenerateToken(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if !authUser.IsCoach {
		h.Templates.Forbidden(w, r)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Verify user exists.
	_, err = models.GetUserByID(h.DB, id)
	if err != nil {
		log.Printf("handlers: get user %d for token: %v", id, err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	label := r.FormValue("label")

	lt, err := models.CreateLoginToken(h.DB, id, label, nil)
	if err != nil {
		log.Printf("handlers: create login token for user %d: %v", id, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Build the full login URL.
	var loginURL string
	if h.BaseURL != "" {
		loginURL = fmt.Sprintf("%s/auth/token/%s", h.BaseURL, lt.Token)
	} else {
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		loginURL = fmt.Sprintf("%s://%s/auth/token/%s", scheme, r.Host, lt.Token)
	}

	// Deliver the login link to the target user's email (fire-and-forget).
	appName := models.GetAppName(h.DB)
	htmlBody := notify.RenderMagicLinkEmail(h.DB, loginURL)
	notify.SendToUser(h.DB, id, appName+" — Login Link", htmlBody)

	// Also create an in-app notification for the target user.
	_, _ = models.CreateNotification(h.DB, id, models.NotifyMagicLinkSent,
		"Login Link Generated", "A new login link has been created for your account.",
		loginURL, sql.NullInt64{})

	// Render the token list with the new token URL highlighted.
	tokens, err := models.ListLoginTokensByUser(h.DB, id)
	if err != nil {
		log.Printf("handlers: list tokens for user %d: %v", id, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Tokens":      tokens,
		"NewTokenURL": loginURL,
		"UserID":      id,
	}
	if err := h.Templates.Render(w, r, "login_tokens.html", data); err != nil {
		log.Printf("handlers: render login tokens: %v", err)
	}
}

// DeleteToken removes a login token.
// POST /users/{id}/tokens/{tokenID}/delete
func (h *LoginTokens) DeleteToken(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if !authUser.IsCoach {
		h.Templates.Forbidden(w, r)
		return
	}

	userID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	tokenID, err := strconv.ParseInt(r.PathValue("tokenID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid token ID", http.StatusBadRequest)
		return
	}

	if err := models.DeleteLoginToken(h.DB, tokenID); err != nil {
		log.Printf("handlers: delete login token %d: %v", tokenID, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Re-render the token list.
	tokens, err := models.ListLoginTokensByUser(h.DB, userID)
	if err != nil {
		log.Printf("handlers: list tokens after delete for user %d: %v", userID, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Tokens": tokens,
		"UserID": userID,
	}
	if err := h.Templates.Render(w, r, "login_tokens.html", data); err != nil {
		log.Printf("handlers: render login tokens after delete: %v", err)
	}
}
