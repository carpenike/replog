package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// StartImpersonation allows an admin or coach to impersonate another user.
// POST /api/admin/impersonate/{userId}
//
// Security rules:
//   - Admins can impersonate any non-admin user
//   - Coaches can only impersonate users linked to their athletes
//   - Cannot impersonate yourself
//   - Cannot impersonate while already impersonating
func (h *Handlers) StartImpersonation(w http.ResponseWriter, r *http.Request) {
	realUser := middleware.UserFromContext(r.Context())
	if realUser == nil {
		WriteError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	// Already impersonating? Must stop first.
	if h.Sessions.GetInt64(r.Context(), "impersonating_real_user_id") != 0 {
		WriteError(w, http.StatusBadRequest, "already impersonating — stop first")
		return
	}

	// Must be admin or coach.
	if !realUser.IsAdmin && !realUser.IsCoach {
		WriteError(w, http.StatusForbidden, "admin or coach access required")
		return
	}

	targetID, err := strconv.ParseInt(r.PathValue("userId"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	if targetID == realUser.ID {
		WriteError(w, http.StatusBadRequest, "cannot impersonate yourself")
		return
	}

	target, err := models.GetUserByID(h.DB, targetID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "user not found")
		return
	}

	// Admins can impersonate anyone except other admins.
	if realUser.IsAdmin && target.IsAdmin {
		WriteError(w, http.StatusForbidden, "cannot impersonate another admin")
		return
	}

	// Coaches (non-admin) can only impersonate users linked to their athletes.
	if !realUser.IsAdmin && realUser.IsCoach {
		if !target.AthleteID.Valid {
			WriteError(w, http.StatusForbidden, "user is not linked to an athlete")
			return
		}
		athlete, err := models.GetAthleteByID(h.DB, target.AthleteID.Int64)
		if err != nil || !athlete.CoachID.Valid || athlete.CoachID.Int64 != realUser.ID {
			WriteError(w, http.StatusForbidden, "can only impersonate your own athletes")
			return
		}
	}

	// Store real user ID and switch session to target.
	h.Sessions.Put(r.Context(), "impersonating_real_user_id", realUser.ID)
	h.Sessions.Put(r.Context(), "userID", targetID)

	log.Printf("api: user %q (id=%d) started impersonating user %q (id=%d)",
		realUser.Username, realUser.ID, target.Username, target.ID)

	WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"user_id": targetID,
		"message": "Now impersonating " + target.Username,
	})
}

// StopImpersonation reverts to the real user session.
// POST /api/admin/stop-impersonating
func (h *Handlers) StopImpersonation(w http.ResponseWriter, r *http.Request) {
	realUserID := h.Sessions.GetInt64(r.Context(), "impersonating_real_user_id")
	if realUserID == 0 {
		WriteError(w, http.StatusBadRequest, "not impersonating anyone")
		return
	}

	// Restore original user.
	h.Sessions.Put(r.Context(), "userID", realUserID)
	h.Sessions.Remove(r.Context(), "impersonating_real_user_id")

	log.Printf("api: user id=%d stopped impersonating", realUserID)

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ImpersonateableUsers returns the list of users the current user can impersonate.
// GET /api/admin/impersonateable
func (h *Handlers) ImpersonateableUsers(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin && !user.IsCoach {
		WriteError(w, http.StatusForbidden, "admin or coach access required")
		return
	}

	var result []struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
	}

	if user.IsAdmin {
		// Admins can impersonate all non-admin users.
		rows, err := h.DB.Query(`
			SELECT id, username, COALESCE(name, username) as name
			FROM users WHERE is_admin = 0 AND id != ?
			ORDER BY name COLLATE NOCASE`, user.ID)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var u struct {
				ID       int64  `json:"id"`
				Username string `json:"username"`
				Name     string `json:"name"`
			}
			if rows.Scan(&u.ID, &u.Username, &u.Name) == nil {
				result = append(result, u)
			}
		}
	} else {
		// Coaches can impersonate users linked to their athletes.
		rows, err := h.DB.Query(`
			SELECT u.id, u.username, COALESCE(u.name, u.username) as name
			FROM users u
			JOIN athletes a ON a.id = u.athlete_id
			WHERE a.coach_id = ? AND u.id != ?
			ORDER BY u.name COLLATE NOCASE`, user.ID, user.ID)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var u struct {
				ID       int64  `json:"id"`
				Username string `json:"username"`
				Name     string `json:"name"`
			}
			if rows.Scan(&u.ID, &u.Username, &u.Name) == nil {
				result = append(result, u)
			}
		}
	}

	if result == nil {
		result = make([]struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
			Name     string `json:"name"`
		}, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
