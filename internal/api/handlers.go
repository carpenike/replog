// Package api provides JSON API handlers for the RepLog REST API.
// These handlers wrap existing business logic from the models layer
// and return JSON responses using the DTO types defined in responses.go.
package api

import (
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/alexedwards/scs/v2"
	"github.com/carpenike/replog/internal/importers"
	"github.com/carpenike/replog/internal/llm"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

func init() {
	// scs serializes session values via encoding/gob. The import flow stashes
	// an *importers.MappingState in the session between the upload and
	// execute steps; without this registration, scs fails to encode the
	// session on save and the handler's "OK" response is followed by a
	// 500 from the LoadAndSave middleware.
	gob.Register(&importers.MappingState{})
}

// Handlers holds dependencies for API handlers.
type Handlers struct {
	DB        *sql.DB
	Sessions  *scs.SessionManager
	AvatarDir string

	// LLMProviderFactory builds the LLM provider used by the AI Coach
	// generation flow. Defaults to llm.NewProviderFromSettings (read from
	// app_settings). Tests override this to inject llm.MockProvider.
	LLMProviderFactory func(*sql.DB) (llm.Provider, error)

	// generateCache holds in-progress generation results keyed by athlete ID.
	// Used instead of session storage to avoid gob encoding large structs.
	generateCache sync.Map
}

// llmProvider returns the configured LLM provider, falling back to the
// production factory if none was injected.
func (h *Handlers) llmProvider() (llm.Provider, error) {
	if h.LLMProviderFactory != nil {
		return h.LLMProviderFactory(h.DB)
	}
	return llm.NewProviderFromSettings(h.DB)
}

// Me returns the currently authenticated user.
//
//	@Summary      Get current user
//	@Description  Returns the user associated with the current session, including
//	@Description  whether they are currently impersonating another user.
//	@Tags         Auth
//	@Produce      json
//	@Success      200  {object}  api.User
//	@Failure      401  {object}  api.APIError
//	@Router       /me [get]
func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		WriteError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	resp := UserFromModel(user)

	// Check if currently impersonating.
	realUserID := h.Sessions.GetInt64(r.Context(), "impersonating_real_user_id")
	if realUserID != 0 {
		resp.Impersonating = true
		resp.RealUserID = &realUserID
	}

	WriteJSON(w, http.StatusOK, resp)
}

// Login authenticates a user and creates a session.
//
//	@Summary      Log in with username and password
//	@Description  Authenticates a user, renews the session token to prevent
//	@Description  fixation, and sets a `HttpOnly`, `SameSite=Lax` session cookie.
//	@Description  Subsequent requests in the same browser are authenticated
//	@Description  automatically.
//	@Tags         Auth
//	@Accept       json
//	@Produce      json
//	@Param        body  body      api.LoginRequest  true  "Credentials"
//	@Success      200  {object}  api.User
//	@Failure      400  {object}  api.APIError  "missing or malformed credentials"
//	@Failure      401  {object}  api.APIError  "invalid username or password"
//	@Failure      403  {object}  api.APIError  "account uses passwordless login"
//	@Router       /login [post]
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	user, err := models.Authenticate(h.DB, req.Username, req.Password)
	if err != nil {
		if errors.Is(err, models.ErrNoPassword) {
			WriteError(w, http.StatusForbidden, "this account uses passwordless login")
		} else {
			WriteError(w, http.StatusUnauthorized, "invalid username or password")
		}
		return
	}

	if err := h.Sessions.RenewToken(r.Context()); err != nil {
		log.Printf("api: session renew error: %v", err)
		WriteError(w, http.StatusInternalServerError, "session error")
		return
	}

	h.Sessions.Put(r.Context(), "userID", user.ID)

	if err := models.EnsureUserPreferences(h.DB, user.ID); err != nil {
		log.Printf("api: ensure preferences for user %d: %v", user.ID, err)
	}

	WriteJSON(w, http.StatusOK, UserFromModel(user))
}

// Logout destroys the session.
//
//	@Summary      Log out
//	@Description  Destroys the current session. Subsequent requests will be unauthenticated.
//	@Tags         Auth
//	@Produce      json
//	@Success      200  {object}  api.StatusResponse
//	@Router       /logout [post]
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.Sessions.Destroy(r.Context()); err != nil {
		log.Printf("api: session destroy error: %v", err)
	}
	WriteJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
}

// Dashboard returns aggregated stats for the home page.
//
//	@Summary      Dashboard data
//	@Description  Returns athlete cards plus aggregated stats (only available to coaches/admins).
//	@Tags         Dashboard
//	@Produce      json
//	@Success      200  {object}  api.DashboardResponse
//	@Failure      401  {object}  api.APIError
//	@Router       /dashboard [get]
func (h *Handlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	resp := DashboardResponse{}

	// Load athlete cards.
	coachFilter := middleware.CoachAthleteFilter(user)
	athletes, err := models.ListAthleteCards(h.DB, coachFilter)
	if err != nil {
		log.Printf("api: dashboard athlete cards: %v", err)
	} else {
		resp.Athletes = make([]*AthleteCard, len(athletes))
		for i, a := range athletes {
			resp.Athletes[i] = AthleteCardFromModel(a)
		}
	}

	// Dashboard stats (available to coaches/admins).
	if user.IsCoach || user.IsAdmin {
		stats, err := models.GetDashboardStats(h.DB)
		if err != nil {
			log.Printf("api: dashboard stats: %v", err)
		} else {
			resp.Stats = &DashboardStats{
				WeekSessions:     stats.WeekSessions,
				WeekVolume:       stats.WeekVolume,
				TotalAthletes:    stats.TotalAthletes,
				TrainedThisWeek:  stats.TrainedThisWeek,
				ConsecutiveWeeks: stats.ConsecutiveWeeks,
			}
		}

		reviewStats, err := models.GetReviewStats(h.DB)
		if err != nil {
			log.Printf("api: dashboard review stats: %v", err)
		} else {
			resp.ReviewStats = &ReviewStats{
				PendingCount:  reviewStats.PendingCount,
				ApprovedCount: reviewStats.ApprovedCount,
				NeedsWork:     reviewStats.NeedsWork,
			}
		}
	}

	WriteJSON(w, http.StatusOK, resp)
}

// ListAthletes returns athlete cards for the authenticated user.
//
//	@Summary      List athletes
//	@Description  Returns athlete cards visible to the caller (admins see all,
//	@Description  coaches see only athletes they own).
//	@Tags         Athletes
//	@Produce      json
//	@Success      200  {array}   api.AthleteCard
//	@Failure      401  {object}  api.APIError
//	@Router       /athletes [get]
func (h *Handlers) ListAthletes(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	coachFilter := middleware.CoachAthleteFilter(user)

	athletes, err := models.ListAthleteCards(h.DB, coachFilter)
	if err != nil {
		log.Printf("api: list athlete cards: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to list athletes")
		return
	}

	result := make([]*AthleteCard, len(athletes))
	for i, a := range athletes {
		result[i] = AthleteCardFromModel(a)
	}
	WriteJSON(w, http.StatusOK, result)
}

// GetAthlete returns a single athlete by ID.
//
//	@Summary      Get athlete
//	@Tags         Athletes
//	@Produce      json
//	@Param        id   path      int  true  "Athlete ID"
//	@Success      200  {object}  api.Athlete
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id} [get]
func (h *Handlers) GetAthlete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}

	if !middleware.CanAccessAthlete(h.DB, user, id) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "athlete not found")
		return
	}
	if err != nil {
		log.Printf("api: get athlete %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to get athlete")
		return
	}

	resp := AthleteFromModel(athlete)

	// Look up linked user's avatar and ID for this athlete.
	var avatarPath sql.NullString
	var linkedUserID sql.NullInt64
	_ = h.DB.QueryRow(
		`SELECT id, avatar_path FROM users WHERE athlete_id = ? LIMIT 1`,
		id,
	).Scan(&linkedUserID, &avatarPath)
	if avatarPath.Valid {
		resp.AvatarURL = "/avatars/" + avatarPath.String
	}
	if linkedUserID.Valid {
		resp.LinkedUserID = &linkedUserID.Int64
	}

	WriteJSON(w, http.StatusOK, resp)
}

// ListExercises returns the exercise catalog.
//
//	@Summary      List exercises
//	@Tags         Exercises
//	@Produce      json
//	@Param        tier  query     string  false  "Filter by tier (foundational, intermediate, sport_performance)"
//	@Success      200   {array}   api.Exercise
//	@Failure      401   {object}  api.APIError
//	@Router       /exercises [get]
func (h *Handlers) ListExercises(w http.ResponseWriter, r *http.Request) {
	tierFilter := r.URL.Query().Get("tier")

	exercises, err := models.ListExercises(h.DB, tierFilter)
	if err != nil {
		log.Printf("api: list exercises: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to list exercises")
		return
	}

	result := make([]*Exercise, len(exercises))
	for i, e := range exercises {
		result[i] = ExerciseFromModel(e)
	}
	WriteJSON(w, http.StatusOK, result)
}

// GetExercise returns a single exercise by ID.
//
//	@Summary      Get exercise
//	@Tags         Exercises
//	@Produce      json
//	@Param        id   path      int  true  "Exercise ID"
//	@Success      200  {object}  api.Exercise
//	@Failure      400  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /exercises/{id} [get]
func (h *Handlers) GetExercise(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid exercise ID")
		return
	}

	exercise, err := models.GetExerciseByID(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "exercise not found")
		return
	}
	if err != nil {
		log.Printf("api: get exercise %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to get exercise")
		return
	}

	WriteJSON(w, http.StatusOK, ExerciseFromModel(exercise))
}

// ListWorkouts returns paginated workouts for an athlete.
//
//	@Summary      List workouts
//	@Tags         Workouts
//	@Produce      json
//	@Param        id      path      int  true   "Athlete ID"
//	@Param        offset  query     int  false  "Pagination offset"
//	@Success      200  {object}  api.WorkoutPage
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/workouts [get]
func (h *Handlers) ListWorkouts(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}

	if !middleware.CanAccessAthlete(h.DB, user, id) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
	}

	page, err := models.ListWorkouts(h.DB, id, offset)
	if err != nil {
		log.Printf("api: list workouts for athlete %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to list workouts")
		return
	}

	WriteJSON(w, http.StatusOK, WorkoutPageFromModel(page))
}

// GetPreferences returns the authenticated user's preferences.
//
//	@Summary      Get preferences
//	@Tags         Preferences
//	@Produce      json
//	@Success      200  {object}  api.UserPreferences
//	@Failure      401  {object}  api.APIError
//	@Router       /preferences [get]
func (h *Handlers) GetPreferences(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	prefs, err := models.GetUserPreferences(h.DB, user.ID)
	if err != nil {
		log.Printf("api: get preferences for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to get preferences")
		return
	}
	WriteJSON(w, http.StatusOK, UserPreferencesFromModel(prefs))
}
