package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Users (Admin Only) ---

// ListUsers returns all users. Admin only.
// ListUsers returns all users. Admin only.
//
//	@Summary      List users
//	@Tags         Users
//	@Produce      json
//	@Success      200  {array}   api.UserWithAthlete
//	@Failure      403  {object}  api.APIError
//	@Router       /users [get]
func (h *Handlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	users, err := models.ListUsers(h.DB)
	if err != nil {
		log.Printf("api: list users: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	result := make([]*UserWithAthlete, len(users))
	for i, u := range users {
		apiUser := UserFromModel(&u.User)
		result[i] = &UserWithAthlete{
			User:        *apiUser,
			AthleteName: nullStr(u.AthleteName),
		}
	}
	WriteJSON(w, http.StatusOK, result)
}

// CreateUser creates a new user. Admin only.
// CreateUser creates a new user. Admin only.
//
//	@Summary      Create user
//	@Tags         Users
//	@Accept       json
//	@Produce      json
//	@Param        body  body      api.UserRequest  true  "User"
//	@Success      201  {object}  api.User
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      409  {object}  api.APIError  "username already exists"
//	@Router       /users [post]
func (h *Handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if !authUser.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	var req UserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" {
		WriteError(w, http.StatusBadRequest, "username is required")
		return
	}

	athleteID := sql.NullInt64{}
	if req.AthleteID != nil {
		athleteID = sql.NullInt64{Int64: *req.AthleteID, Valid: true}
	}

	newUser, err := models.CreateUser(h.DB, req.Username, req.Name, req.Password, req.Email, req.IsCoach, req.IsAdmin, athleteID)
	if errors.Is(err, models.ErrDuplicateUsername) {
		WriteError(w, http.StatusConflict, "username already exists")
		return
	}
	if err != nil {
		log.Printf("api: create user: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	WriteJSON(w, http.StatusCreated, UserFromModel(newUser))
}

// DeleteUser deletes a user. Admin only.
// DeleteUser deletes a user. Admin only.
//
//	@Summary      Delete user
//	@Description  Cannot delete yourself.
//	@Tags         Users
//	@Produce      json
//	@Param        userID  path      int  true  "User ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError  "e.g. trying to delete your own account"
//	@Failure      403  {object}  api.APIError
//	@Router       /users/{userID} [delete]
func (h *Handlers) DeleteUser(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if !authUser.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid user ID")
		return
	}
	if id == authUser.ID {
		WriteError(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}

	if err := models.DeleteUser(h.DB, id); err != nil {
		log.Printf("api: delete user %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
