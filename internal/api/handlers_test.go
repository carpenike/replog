// Test harness for the internal/api package.
//
// Usage from a test file:
//
//	func TestSomething(t *testing.T) {
//	    env := setupTest(t)
//	    user := env.createUser(t, "alice", true /* coach */, true /* admin */)
//	    rr := env.do(t, "GET", "/api/me", nil, env.loginAs(t, user))
//	    requireStatus(t, rr, http.StatusOK)
//	    var got User
//	    decodeJSON(t, rr, &got)
//	    if got.Username != "alice" { t.Fatalf("got %+v", got) }
//	}
//
// The harness mounts EVERY production /api route on a chi router with the
// real session manager wiring, so r.PathValue / chi.URLParam / RequireAuth /
// session cookies all behave the same as the running server.
package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"

	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// testEnv bundles the per-test database, session manager, handlers, and an HTTP
// router that mirrors the production /api wiring.
type testEnv struct {
	DB       *sql.DB
	Sessions *scs.SessionManager
	Handlers *Handlers
	Router   chi.Router
}

// setupTest creates a fresh in-memory database with all migrations applied,
// a session manager backed by that DB, and a chi router with the same /api
// routes that main.go wires up. The DB is closed automatically at end of test.
func setupTest(t *testing.T) *testEnv {
	t.Helper()

	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := database.RunMigrations(db); err != nil {
		_ = db.Close()
		t.Fatalf("run migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	sm := scs.New()
	sm.Lifetime = 30 * 24 * time.Hour
	sm.Cookie.HttpOnly = true
	sm.Cookie.SameSite = http.SameSiteLaxMode
	// Default scs store is in-memory, fine for tests.

	h := &Handlers{
		DB:        db,
		Sessions:  sm,
		AvatarDir: t.TempDir(),
	}

	withAuth := func(next http.Handler) http.Handler {
		return middleware.RequireAuth(sm, db, next)
	}

	r := chi.NewRouter()
	r.Route("/api", func(r chi.Router) {
		r.Use(sm.LoadAndSave)

		// Public endpoints (no auth).
		r.Group(func(r chi.Router) {
			r.Post("/login", h.Login)
			r.Get("/auth/token/{token}", h.TokenLogin)
		})

		// Authenticated endpoints — mirror main.go's /api group.
		r.Group(func(r chi.Router) {
			r.Use(withAuth)

			r.Get("/me", h.Me)
			r.Post("/logout", h.Logout)
			r.Get("/preferences", h.GetPreferences)
			r.Put("/preferences", h.UpdatePreferences)
			r.Get("/dashboard", h.Dashboard)

			r.Post("/avatars/upload", h.AvatarUpload)
			r.Post("/avatars/delete", h.AvatarDelete)

			r.Get("/notifications/preferences", h.ListNotificationPreferences)
			r.Put("/notifications/preferences", h.UpdateNotificationPreference)

			r.Get("/athletes", h.ListAthletes)
			r.Post("/athletes", h.CreateAthlete)
			r.Get("/athletes/{id}", h.GetAthlete)
			r.Put("/athletes/{id}", h.UpdateAthlete)
			r.Delete("/athletes/{id}", h.DeleteAthlete)

			r.Get("/exercises", h.ListExercises)
			r.Post("/exercises", h.CreateExercise)
			r.Get("/exercises/{id}", h.GetExercise)
			r.Put("/exercises/{id}", h.UpdateExercise)
			r.Delete("/exercises/{id}", h.DeleteExercise)

			r.Get("/athletes/{id}/workouts", h.ListWorkouts)
			r.Post("/athletes/{id}/workouts", h.CreateWorkout)
			r.Get("/athletes/{id}/workouts/{workoutID}", h.GetWorkout)
			r.Delete("/athletes/{id}/workouts/{workoutID}", h.DeleteWorkout)

			r.Post("/athletes/{id}/workouts/{workoutID}/sets", h.AddWorkoutSet)
			r.Put("/athletes/{id}/workouts/{workoutID}/sets/{setID}", h.UpdateWorkoutSet)
			r.Delete("/athletes/{id}/workouts/{workoutID}/sets/{setID}", h.DeleteWorkoutSet)

			r.Put("/athletes/{id}/workouts/{workoutID}/notes", h.UpdateWorkoutNotes)

			r.Get("/athletes/{id}/body-weights", h.ListBodyWeights)
			r.Post("/athletes/{id}/body-weights", h.CreateBodyWeight)
			r.Delete("/athletes/{id}/body-weights/{bwID}", h.DeleteBodyWeight)

			r.Get("/athletes/{id}/throwing-sessions", h.ListThrowingSessions)
			r.Post("/athletes/{id}/throwing-sessions", h.CreateThrowingSession)
			r.Delete("/athletes/{id}/throwing-sessions/{sessionID}", h.DeleteThrowingSession)
			r.Get("/athletes/{id}/season-phases", h.ListSeasonPhases)
			r.Post("/athletes/{id}/season-phases", h.CreateSeasonPhase)
			r.Delete("/athletes/{id}/season-phases/{phaseID}", h.DeleteSeasonPhase)
			r.Get("/athletes/{id}/bio-samples", h.ListBioSamples)
			r.Post("/athletes/{id}/bio-samples", h.CreateBioSample)
			r.Get("/athletes/{id}/pitch-smart", h.GetPitchSmartStatus)

			r.Get("/athletes/{id}/conditioning-sessions", h.ListConditioningSessions)
			r.Post("/athletes/{id}/conditioning-sessions", h.CreateConditioningSession)
			r.Delete("/athletes/{id}/conditioning-sessions/{sessionID}", h.DeleteConditioningSession)
			r.Get("/athletes/{id}/skill-sessions", h.ListSkillSessions)
			r.Post("/athletes/{id}/skill-sessions", h.CreateSkillSession)
			r.Delete("/athletes/{id}/skill-sessions/{sessionID}", h.DeleteSkillSession)
			r.Get("/athletes/{id}/recovery-checkins", h.ListRecoveryCheckins)
			r.Post("/athletes/{id}/recovery-checkins", h.CreateRecoveryCheckin)
			r.Delete("/athletes/{id}/recovery-checkins/{checkinID}", h.DeleteRecoveryCheckin)
			r.Get("/athletes/{id}/load", h.GetLoadSummary)

			r.Get("/athletes/{id}/training-maxes", h.ListTrainingMaxes)
			r.Post("/athletes/{id}/training-maxes", h.CreateTrainingMax)
			r.Get("/athletes/{id}/exercises/{exerciseID}/training-maxes", h.GetTrainingMaxHistory)

			r.Get("/athletes/{id}/programs", h.ListAthletePrograms)
			r.Post("/athletes/{id}/programs", h.AssignProgramToAthlete)
			r.Post("/athletes/{id}/programs/{programID}/deactivate", h.DeactivateAthleteProgram)
			r.Post("/athletes/{id}/programs/{programID}/reactivate", h.ReactivateAthleteProgram)
			r.Delete("/athletes/{id}/programs/{programID}", h.DeleteAthleteProgram)

			r.Get("/athletes/{id}/journal", h.ListJournalEntries)
			r.Post("/athletes/{id}/notes", h.CreateAthleteNote)
			r.Put("/athletes/{id}/notes/{noteID}", h.UpdateAthleteNote)
			r.Delete("/athletes/{id}/notes/{noteID}", h.DeleteAthleteNote)

			r.Put("/athletes/{id}/goal", h.UpdateAthleteGoal)
			r.Get("/athletes/{id}/prescription", h.GetPrescription)
			r.Post("/athletes/{id}/promote", h.PromoteAthlete)

			r.Get("/athletes/{id}/assignments", h.ListAssignments)
			r.Post("/athletes/{id}/assignments", h.AssignExercise)
			r.Post("/athletes/{id}/assignments/{assignmentID}/deactivate", h.DeactivateAssignment)
			r.Post("/athletes/{id}/assignments/reactivate", h.ReactivateAssignment)

			// TM Setup wizard.
			r.Get("/athletes/{id}/missing-tms", h.ListMissingTMs)
			r.Post("/athletes/{id}/batch-tms", h.BatchSetTMs)

			// Accessory plans.
			r.Get("/athletes/{id}/accessories", h.ListAccessoryPlans)
			r.Post("/athletes/{id}/accessories", h.CreateAccessoryPlan)
			r.Put("/athletes/{id}/accessories/{planID}", h.UpdateAccessoryPlan)
			r.Post("/athletes/{id}/accessories/{planID}/deactivate", h.DeactivateAccessoryPlan)
			r.Delete("/athletes/{id}/accessories/{planID}", h.DeleteAccessoryPlan)

			// Athlete equipment.
			r.Get("/athletes/{id}/equipment", h.ListAthleteEquipment)
			r.Post("/athletes/{id}/equipment", h.AddAthleteEquipment)
			r.Delete("/athletes/{id}/equipment/{equipmentID}", h.RemoveAthleteEquipment)

			// Exercise equipment.
			r.Get("/exercises/{id}/equipment", h.ListExerciseEquipment)
			r.Post("/exercises/{id}/equipment", h.AddExerciseEquipment)
			r.Delete("/exercises/{id}/equipment/{equipmentID}", h.RemoveExerciseEquipment)

			// Equipment catalog.
			r.Get("/equipment", h.ListEquipment)
			r.Post("/equipment", h.CreateEquipment)
			r.Put("/equipment/{equipmentID}", h.UpdateEquipment)
			r.Delete("/equipment/{equipmentID}", h.DeleteEquipment)

			// Exercise history (per athlete).
			r.Get("/athletes/{id}/exercises/{exerciseID}/history", h.ListExerciseHistory)

			// Program compatibility check.
			r.Get("/athletes/{id}/program-compatibility", h.CheckProgramCompatibility)

			// Cycle review.
			r.Get("/athletes/{id}/cycle-review", h.GetCycleReview)
			r.Post("/athletes/{id}/cycle-review", h.ApplyTMBumps)

			// Notifications.
			r.Get("/notifications", h.ListNotifications)
			r.Get("/notifications/count", h.UnreadNotificationCount)
			r.Post("/notifications/{notificationID}/read", h.MarkNotificationRead)
			r.Post("/notifications/read-all", h.MarkAllNotificationsRead)

			// Workout reviews (coach approval flow).
			r.Get("/reviews/pending", h.ListPendingReviews)
			r.Post("/athletes/{id}/workouts/{workoutID}/review", h.SubmitReview)
			r.Delete("/athletes/{id}/workouts/{workoutID}/review", h.DeleteReview)

			// Login tokens (admin).
			r.Get("/users/{userID}/tokens", h.ListLoginTokens)
			r.Post("/users/{userID}/tokens", h.CreateLoginToken)
			r.Delete("/users/{userID}/tokens/{tokenID}", h.DeleteLoginToken)

			// Impersonation.
			r.Post("/admin/impersonate/{userId}", h.StartImpersonation)
			r.Post("/admin/stop-impersonating", h.StopImpersonation)
			r.Get("/admin/impersonateable", h.ImpersonateableUsers)

			// Program templates (catalog).
			r.Get("/programs", h.ListProgramTemplates)
			r.Post("/programs", h.CreateProgramTemplate)
			r.Get("/programs/{id}", h.GetProgramTemplate)
			r.Put("/programs/{id}", h.UpdateProgramTemplate)
			r.Delete("/programs/{id}", h.DeleteProgramTemplate)
			r.Post("/programs/{id}/copy-week", h.CopyWeek)

			// Prescribed sets.
			r.Post("/programs/{id}/sets", h.AddPrescribedSet)
			r.Put("/programs/{id}/sets/{setID}", h.UpdatePrescribedSet)
			r.Delete("/programs/{id}/sets/{setID}", h.DeletePrescribedSet)

			// Progression rules.
			r.Get("/programs/{id}/rules", h.ListProgressionRules)
			r.Post("/programs/{id}/rules", h.SetProgressionRule)
			r.Delete("/programs/{id}/rules/{ruleID}", h.DeleteProgressionRule)

			r.Get("/users", h.ListUsers)
			r.Post("/users", h.CreateUser)
			r.Get("/users/{userID}", h.GetUser)
			r.Put("/users/{userID}", h.UpdateUser)
			r.Delete("/users/{userID}", h.DeleteUser)
			r.Put("/users/{userID}/mcp", h.SetUserMCPAccess)

			// Admin settings.
			r.Get("/admin/settings", h.ListSettings)
			r.Put("/admin/settings", h.UpdateSetting)
			r.Post("/admin/settings/test-llm", h.TestLLMConnection)
			r.Post("/admin/settings/test-notify", h.TestNotifyConnection)

			// Catalog import/export.
			r.Get("/catalog/export", h.CatalogExportJSON)
			r.Post("/catalog/import/upload", h.CatalogImportUpload)
			r.Post("/catalog/import/execute", h.CatalogImportExecute)

			// Workout import (per athlete).
			r.Post("/athletes/{id}/import/upload", h.ImportUpload)
			r.Post("/athletes/{id}/import/execute", h.ImportExecute)

			// AI Coach generation (per athlete).
			r.Get("/athletes/{id}/generate", h.GenerateFormData)
			r.Post("/athletes/{id}/generate", h.GenerateSubmit)
			r.Get("/athletes/{id}/generations/{genID}", h.GenerationStatus)
			r.Post("/athletes/{id}/generations/{genID}/cancel", h.GenerationCancel)
			r.Post("/athletes/{id}/generations/{genID}/execute", h.GenerationExecute)
			r.Post("/athletes/{id}/wod", h.WODSubmit)
			r.Post("/athletes/{id}/wod/{genID}/log", h.WODLog)
		})
	})

	return &testEnv{
		DB:       db,
		Sessions: sm,
		Handlers: h,
		Router:   r,
	}
}

// createUser inserts a user with a known password and ensures default preferences.
func (e *testEnv) createUser(t *testing.T, username string, isCoach, isAdmin bool) *models.User {
	t.Helper()
	user, err := models.CreateUser(e.DB, username, username, "password123", username+"@example.com", isCoach, isAdmin, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user %q: %v", username, err)
	}
	if err := models.EnsureUserPreferences(e.DB, user.ID); err != nil {
		t.Fatalf("ensure prefs for %d: %v", user.ID, err)
	}
	return user
}

// createAthlete inserts an athlete with the given coach.
func (e *testEnv) createAthlete(t *testing.T, name string, coachID int64) *models.Athlete {
	t.Helper()
	athlete, err := models.CreateAthlete(e.DB, name, "", "", "", "", "", "",
		sql.NullInt64{Int64: coachID, Valid: coachID != 0}, false)
	if err != nil {
		t.Fatalf("create athlete %q: %v", name, err)
	}
	return athlete
}

// createAthleteWithTier is like createAthlete but binds a youth tier.
func (e *testEnv) createAthleteWithTier(t *testing.T, name, tier string, coachID int64) *models.Athlete {
	t.Helper()
	athlete, err := models.CreateAthlete(e.DB, name, tier, "", "", "", "", "",
		sql.NullInt64{Int64: coachID, Valid: coachID != 0}, false)
	if err != nil {
		t.Fatalf("create athlete %q: %v", name, err)
	}
	return athlete
}

// loginAs returns the cookies from a successful POST /api/login for the given
// user. The user must have been created via createUser (so it has the known
// password "password123").
func (e *testEnv) loginAs(t *testing.T, u *models.User) []*http.Cookie {
	t.Helper()
	body := bytes.NewBufferString(`{"username":"` + u.Username + `","password":"password123"}`)
	req := httptest.NewRequest("POST", "/api/login", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	e.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("loginAs(%s) failed: status=%d body=%q", u.Username, rr.Code, rr.Body.String())
	}
	return rr.Result().Cookies()
}

// do issues a request through the test router. Pass nil cookies for an
// unauthenticated request, or the slice returned by loginAs for an
// authenticated one. Body may be nil, a string, []byte, or any value that
// can be JSON-marshalled.
func (e *testEnv) do(t *testing.T, method, path string, body any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	contentType := ""
	switch v := body.(type) {
	case nil:
		reader = nil
	case string:
		reader = bytes.NewBufferString(v)
		contentType = "application/json"
	case []byte:
		reader = bytes.NewReader(v)
		contentType = "application/json"
	default:
		buf, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(buf)
		contentType = "application/json"
	}

	req := httptest.NewRequest(method, path, reader)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rr := httptest.NewRecorder()
	e.Router.ServeHTTP(rr, req)
	return rr
}

// requireStatus fails the test if the recorder's status code is not what we expect.
func requireStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rr.Code != want {
		t.Fatalf("expected status %d, got %d\nbody: %s", want, rr.Code, rr.Body.String())
	}
}

// decodeJSON unmarshals the response body into out. Fails the test on error.
func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(rr.Body.Bytes(), out); err != nil {
		t.Fatalf("decode JSON: %v\nbody: %s", err, rr.Body.String())
	}
}
