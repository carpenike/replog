package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// PasskeyResponse is the JSON representation of a WebAuthn credential.
type PasskeyResponse struct {
	ID          int64   `json:"id"`
	Label       *string `json:"label"`
	CreatedAt   string  `json:"created_at"`
	LastUsedAt  *string `json:"last_used_at"`
	UseCount    int64   `json:"use_count"`
	BackupState bool    `json:"backup_state"`
}

// ListPasskeys returns the current user's registered passkey credentials.
// GET /api/passkeys
func (h *Handlers) ListPasskeys(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		WriteError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	creds, err := models.ListWebAuthnCredentialsByUser(h.DB, user.ID)
	if err != nil {
		log.Printf("api: list passkeys for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list passkeys")
		return
	}

	result := make([]PasskeyResponse, 0, len(creds))
	for _, c := range creds {
		var label *string
		if c.Label.Valid {
			label = &c.Label.String
		}
		var lastUsed *string
		if c.LastUsedAt.Valid {
			s := c.LastUsedAt.Time.Format(time.RFC3339)
			lastUsed = &s
		}
		result = append(result, PasskeyResponse{
			ID:          c.ID,
			Label:       label,
			CreatedAt:   c.CreatedAt.Format(time.RFC3339),
			LastUsedAt:  lastUsed,
			UseCount:    c.UseCount,
			BackupState: c.BackupState,
		})
	}

	WriteJSON(w, http.StatusOK, result)
}

// DeletePasskey removes one of the current user's passkey credentials.
// DELETE /api/passkeys/{id}
func (h *Handlers) DeletePasskey(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		WriteError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	credID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid credential ID")
		return
	}

	if err := models.DeleteWebAuthnCredential(h.DB, credID, user.ID); err != nil {
		log.Printf("api: delete passkey %d for user %d: %v", credID, user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete passkey")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// SetPasskeyLabel stores the passkey label in the session before the ceremony.
// POST /api/passkeys/label
func (h *Handlers) SetPasskeyLabel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.Sessions.Put(r.Context(), "webauthn_label", body.Label)
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
