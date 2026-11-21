package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// ServiceTokenIssueRequest is the optional label for a new service token.
// The label "coach-service" is reserved for REPLOG_SERVICE_TOKEN.
type ServiceTokenIssueRequest struct {
	Label string `json:"label"`
}

// ServiceTokenIssued is the one response that contains the plaintext bearer.
type ServiceTokenIssued struct {
	ID     int64   `json:"id"`
	UserID int64   `json:"user_id"`
	Label  *string `json:"label"`
	Token  string  `json:"token"`
}

// ServiceTokenInfo is a stored service token without its hash or plaintext.
type ServiceTokenInfo struct {
	ID         int64   `json:"id"`
	UserID     int64   `json:"user_id"`
	Label      *string `json:"label"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
	RevokedAt  *string `json:"revoked_at"`
}

// ListServiceTokens lists a user's service tokens. Admin browser session only.
//
//	@Summary      List service tokens
//	@Description  Admin only. Returns ids and labels. The hash and the plaintext are not included. A service token cannot call this.
//	@Tags         Admin
//	@Produce      json
//	@Param        userID  path  int  true  "User ID"
//	@Success      200  {array}   api.ServiceTokenInfo
//	@Failure      403  {object}  api.APIError
//	@Router       /users/{userID}/service-tokens [get]
func (h *Handlers) ListServiceTokens(w http.ResponseWriter, r *http.Request) {
	if !h.adminBrowser(w, r) {
		return
	}
	userID, ok := pathUserID(w, r)
	if !ok {
		return
	}
	if _, err := models.GetUserByID(r.Context(), h.DB, userID); errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		log.Printf("api: list service tokens user %d: %v", userID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list service tokens")
		return
	}
	tokens, err := models.ListServiceTokens(r.Context(), h.DB, userID)
	if err != nil {
		log.Printf("api: list service tokens user %d: %v", userID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list service tokens")
		return
	}
	out := make([]ServiceTokenInfo, 0, len(tokens))
	for _, token := range tokens {
		out = append(out, ServiceTokenInfo{
			ID:         token.ID,
			UserID:     token.UserID,
			Label:      nullStr(token.Label),
			CreatedAt:  fmtTime(token.CreatedAt),
			LastUsedAt: fmtNullTimeStr(token.LastUsedAt),
			RevokedAt:  fmtNullTimeStr(token.RevokedAt),
		})
	}
	WriteJSON(w, http.StatusOK, out)
}

// IssueServiceToken mints an rlcst_ bearer for one user. Admin browser session only.
//
//	@Summary      Issue a service token
//	@Description  Admin only. The plaintext is returned once and stored only as a SHA-256 hash, bound to this user. A service token cannot call this. REPLOG_SERVICE_TOKEN remains the env-installed coach-service row.
//	@Tags         Admin
//	@Accept       json
//	@Produce      json
//	@Param        userID  path  int                          true  "User ID"
//	@Param        body    body  api.ServiceTokenIssueRequest false "Optional label"
//	@Success      201  {object}  api.ServiceTokenIssued
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /users/{userID}/service-tokens [post]
func (h *Handlers) IssueServiceToken(w http.ResponseWriter, r *http.Request) {
	if !h.adminBrowser(w, r) {
		return
	}
	userID, ok := pathUserID(w, r)
	if !ok {
		return
	}
	if _, err := models.GetUserByID(r.Context(), h.DB, userID); errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		log.Printf("api: issue service token user %d: %v", userID, err)
		WriteError(w, http.StatusInternalServerError, "failed to issue service token")
		return
	}
	var req ServiceTokenIssueRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	label := strings.TrimSpace(req.Label)
	if label == "coach-service" {
		WriteValidationError(w, "label", "coach-service is reserved for REPLOG_SERVICE_TOKEN")
		return
	}
	plaintext, id, err := models.CreateServiceToken(r.Context(), h.DB, userID, label)
	if err != nil {
		log.Printf("api: issue service token user %d: %v", userID, err)
		WriteError(w, http.StatusInternalServerError, "failed to issue service token")
		return
	}
	var labelPtr *string
	if label != "" {
		labelPtr = &label
	}
	WriteJSON(w, http.StatusCreated, ServiceTokenIssued{
		ID: id, UserID: userID, Label: labelPtr, Token: plaintext,
	})
}

// RevokeServiceToken revokes one user's service token. Admin browser session only.
//
//	@Summary      Revoke a service token
//	@Description  Admin only. The token must belong to the user in the path. A service token cannot call this. Revoking the env-installed token does not clear REPLOG_SERVICE_TOKEN; the next startup installs it again.
//	@Tags         Admin
//	@Produce      json
//	@Param        userID   path  int  true  "User ID"
//	@Param        tokenID  path  int  true  "Token ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /users/{userID}/service-tokens/{tokenID} [delete]
func (h *Handlers) RevokeServiceToken(w http.ResponseWriter, r *http.Request) {
	if !h.adminBrowser(w, r) {
		return
	}
	userID, ok := pathUserID(w, r)
	if !ok {
		return
	}
	tokenID, err := strconv.ParseInt(r.PathValue("tokenID"), 10, 64)
	if err != nil || tokenID <= 0 {
		WriteError(w, http.StatusBadRequest, "invalid token ID")
		return
	}
	if err := models.RevokeServiceTokenForUser(r.Context(), h.DB, tokenID, userID); errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "service token not found")
		return
	} else if err != nil {
		log.Printf("api: revoke service token %d: %v", tokenID, err)
		WriteError(w, http.StatusInternalServerError, "failed to revoke service token")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// adminBrowser allows an admin session and refuses a service token, including
// one whose user is an admin. Issuing bearers stays in the browser.
func (h *Handlers) adminBrowser(w http.ResponseWriter, r *http.Request) bool {
	if middleware.IsServiceCredential(r.Context()) {
		WriteError(w, http.StatusForbidden, "service token cannot issue or revoke tokens")
		return false
	}
	user := middleware.UserFromContext(r.Context())
	if user == nil || !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return false
	}
	return true
}

func pathUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	userID, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if err != nil || userID <= 0 {
		WriteError(w, http.StatusBadRequest, "invalid user ID")
		return 0, false
	}
	return userID, true
}
