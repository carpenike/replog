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

// UpdateEquipment updates an equipment item. Coach only.
//
//	@Summary      Update equipment
//	@Tags         Equipment
//	@Accept       json
//	@Produce      json
//	@Param        equipmentID  path      int                   true  "Equipment ID"
//	@Param        body         body      api.EquipmentRequest  true  "Equipment"
//	@Success      200  {object}  api.Equipment
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /equipment/{equipmentID} [put]
func (h *Handlers) UpdateEquipment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("equipmentID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid equipment ID")
		return
	}

	var req EquipmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	equip, err := models.UpdateEquipment(h.DB, id, req.Name, req.Description)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "equipment not found")
		return
	}
	if err != nil {
		log.Printf("api: update equipment %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to update equipment")
		return
	}

	WriteJSON(w, http.StatusOK, EquipmentFromModel(equip))
}

// GetUser returns a user by ID. Admin only.
//
//	@Summary      Get user
//	@Tags         Users
//	@Produce      json
//	@Param        userID  path      int  true  "User ID"
//	@Success      200  {object}  api.User
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /users/{userID} [get]
func (h *Handlers) GetUser(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	target, err := models.GetUserByID(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		log.Printf("api: get user %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	WriteJSON(w, http.StatusOK, UserFromModel(target))
}

// UpdateUser updates a user. Admin only.
//
//	@Summary      Update user
//	@Description  Optional `password` field reissues the password (existing sessions remain valid until expiry).
//	@Tags         Users
//	@Accept       json
//	@Produce      json
//	@Param        userID  path      int                    true  "User ID"
//	@Param        body    body      api.UserUpdateRequest  true  "User"
//	@Success      200  {object}  api.User
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      409  {object}  api.APIError  "username already exists"
//	@Router       /users/{userID} [put]
func (h *Handlers) UpdateUser(w http.ResponseWriter, r *http.Request) {
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

	var req UserUpdateRequest
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

	updated, err := models.UpdateUser(h.DB, id, req.Username, req.Name, req.Email, athleteID, req.IsCoach, req.IsAdmin)
	if errors.Is(err, models.ErrDuplicateUsername) {
		WriteError(w, http.StatusConflict, "username already exists")
		return
	}
	if err != nil {
		log.Printf("api: update user %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	// Update password if provided.
	if req.Password != "" {
		if err := models.UpdatePassword(h.DB, id, req.Password); err != nil {
			log.Printf("api: set password for user %d: %v", id, err)
			WriteError(w, http.StatusInternalServerError, "user updated but password change failed")
			return
		}
	}

	WriteJSON(w, http.StatusOK, UserFromModel(updated))
}
