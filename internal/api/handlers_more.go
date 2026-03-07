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

// --- Body Weights ---

// ListBodyWeights returns paginated body weight entries.
func (h *Handlers) ListBodyWeights(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
	}

	page, err := models.ListBodyWeights(h.DB, athleteID, offset)
	if err != nil {
		log.Printf("api: list body weights for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list body weights")
		return
	}

	WriteJSON(w, http.StatusOK, BodyWeightPageFromModel(page))
}

// CreateBodyWeight creates a new body weight entry.
func (h *Handlers) CreateBodyWeight(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	var req struct {
		Date   string  `json:"date"`
		Weight float64 `json:"weight"`
		Notes  string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Date == "" || req.Weight <= 0 {
		WriteError(w, http.StatusBadRequest, "date and weight are required")
		return
	}

	bw, err := models.CreateBodyWeight(h.DB, athleteID, req.Date, req.Weight, req.Notes)
	if err != nil {
		log.Printf("api: create body weight for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to create body weight")
		return
	}

	WriteJSON(w, http.StatusCreated, BodyWeightFromModel(bw))
}

// DeleteBodyWeight deletes a body weight entry.
func (h *Handlers) DeleteBodyWeight(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	bwID, err := strconv.ParseInt(r.PathValue("bwID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid body weight ID")
		return
	}

	if err := models.DeleteBodyWeight(h.DB, bwID); err != nil {
		log.Printf("api: delete body weight %d: %v", bwID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete body weight")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Training Maxes ---

// ListTrainingMaxes returns current training maxes for an athlete.
func (h *Handlers) ListTrainingMaxes(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	tms, err := models.ListCurrentTrainingMaxes(h.DB, athleteID)
	if err != nil {
		log.Printf("api: list training maxes for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list training maxes")
		return
	}

	result := make([]*TrainingMax, len(tms))
	for i, tm := range tms {
		result[i] = TrainingMaxFromModel(tm)
	}
	WriteJSON(w, http.StatusOK, result)
}

// GetTrainingMaxHistory returns TM history for an athlete+exercise.
func (h *Handlers) GetTrainingMaxHistory(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	exerciseID, err := strconv.ParseInt(r.PathValue("exerciseID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid exercise ID")
		return
	}

	history, err := models.ListTrainingMaxHistory(h.DB, athleteID, exerciseID)
	if err != nil {
		log.Printf("api: training max history for athlete %d exercise %d: %v", athleteID, exerciseID, err)
		WriteError(w, http.StatusInternalServerError, "failed to get training max history")
		return
	}

	result := make([]*TrainingMax, len(history))
	for i, tm := range history {
		result[i] = TrainingMaxFromModel(tm)
	}
	WriteJSON(w, http.StatusOK, result)
}

// --- Notifications ---

// ListNotifications returns notifications for the authenticated user.
func (h *Handlers) ListNotifications(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
	}

	notifications, err := models.ListNotifications(h.DB, user.ID, limit, offset)
	if err != nil {
		log.Printf("api: list notifications for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list notifications")
		return
	}

	result := make([]*Notification, len(notifications))
	for i, n := range notifications {
		result[i] = NotificationFromModel(n)
	}
	WriteJSON(w, http.StatusOK, result)
}

// UnreadNotificationCount returns the number of unread notifications.
func (h *Handlers) UnreadNotificationCount(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	count, err := models.GetUnreadCount(h.DB, user.ID)
	if err != nil {
		log.Printf("api: unread count for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to get count")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]int{"count": count})
}

// MarkNotificationRead marks a notification as read.
func (h *Handlers) MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("notificationID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid notification ID")
		return
	}

	if err := models.MarkAsRead(h.DB, id, user.ID); err != nil {
		log.Printf("api: mark notification %d read: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to mark read")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// MarkAllNotificationsRead marks all notifications as read.
func (h *Handlers) MarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	count, err := models.MarkAllAsRead(h.DB, user.ID)
	if err != nil {
		log.Printf("api: mark all notifications read for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to mark all read")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]int64{"marked": count})
}

// --- Programs ---

// ListProgramTemplates returns all program templates.
func (h *Handlers) ListProgramTemplates(w http.ResponseWriter, r *http.Request) {
	programs, err := models.ListProgramTemplates(h.DB)
	if err != nil {
		log.Printf("api: list program templates: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to list programs")
		return
	}

	result := make([]*ProgramTemplate, len(programs))
	for i, p := range programs {
		result[i] = ProgramTemplateFromModel(p)
	}
	WriteJSON(w, http.StatusOK, result)
}

// GetProgramTemplate returns a single program template.
func (h *Handlers) GetProgramTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid program ID")
		return
	}

	program, err := models.GetProgramTemplateByID(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "program not found")
		return
	}
	if err != nil {
		log.Printf("api: get program template %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to get program")
		return
	}

	sets, err := models.ListPrescribedSets(h.DB, id)
	if err != nil {
		log.Printf("api: list prescribed sets for program %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to get program sets")
		return
	}

	apiSets := make([]*PrescribedSet, len(sets))
	for i, s := range sets {
		apiSets[i] = PrescribedSetFromModel(s)
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"program": ProgramTemplateFromModel(program),
		"sets":    apiSets,
	})
}

// ListAthletePrograms returns programs assigned to an athlete.
func (h *Handlers) ListAthletePrograms(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	programs, err := models.ListAthletePrograms(h.DB, athleteID)
	if err != nil {
		log.Printf("api: list athlete programs for %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list programs")
		return
	}

	result := make([]*AthleteProgram, len(programs))
	for i, p := range programs {
		result[i] = AthleteProgramFromModel(p)
	}
	WriteJSON(w, http.StatusOK, result)
}

// --- Users (Admin Only) ---

// ListUsers returns all users. Admin only.
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
func (h *Handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if !authUser.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	var req struct {
		Username  string `json:"username"`
		Name      string `json:"name"`
		Password  string `json:"password"`
		Email     string `json:"email"`
		IsCoach   bool   `json:"is_coach"`
		IsAdmin   bool   `json:"is_admin"`
		AthleteID *int64 `json:"athlete_id"`
	}
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

// --- Journal ---

// ListJournalEntries returns the athlete's journal timeline.
func (h *Handlers) ListJournalEntries(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	// Coaches and admins see private notes.
	includePrivate := user.IsCoach || user.IsAdmin

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}

	entries, err := models.ListJournalEntries(h.DB, athleteID, includePrivate, limit)
	if err != nil {
		log.Printf("api: list journal for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list journal")
		return
	}

	result := make([]*JournalEntry, len(entries))
	for i, e := range entries {
		result[i] = JournalEntryFromModel(e)
	}
	WriteJSON(w, http.StatusOK, result)
}
