package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/carpenike/replog/internal/api"
	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/importers"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
	"github.com/carpenike/replog/internal/notify"
	"github.com/carpenike/replog/internal/passkeys"
	"github.com/carpenike/replog/internal/scheduler"
	frontend "github.com/carpenike/replog/web"
)

// Build-time variables injected via ldflags (e.g., by GoReleaser).
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Determine database path — default to ./replog.db, override with REPLOG_DB_PATH.
	dbPath := os.Getenv("REPLOG_DB_PATH")
	if dbPath == "" {
		dbPath = "replog.db"
	}

	// Determine listen address — default to :8080, override with REPLOG_ADDR.
	addr := os.Getenv("REPLOG_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	// Determine avatar storage directory — defaults to a sibling of the DB file.
	avatarDir := os.Getenv("REPLOG_AVATAR_DIR")
	if avatarDir == "" {
		avatarDir = filepath.Join(filepath.Dir(dbPath), "avatars")
	}

	// Open database and run migrations.
	db, err := database.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Printf("Database ready: %s", filepath.Clean(dbPath))

	// Mark any AI Coach generations left in pending/running by a previous
	// process as failed. The detached goroutines that owned them are gone,
	// so without this the SPA would show them as forever-spinning drafts.
	// List the affected rows BEFORE the UPDATE so we still have the
	// (id, athlete_id, requested_by) tuples needed to notify each
	// requester — the SPA promises a notification will arrive on failure
	// and the startup sweep is one of those failure paths (HOF-001 #13).
	// Race-free: the HTTP server isn't accepting requests yet and any
	// prior process's goroutines are dead.
	stale, err := models.ListStaleRunningGenerations(db)
	if err != nil {
		log.Printf("Warning: list stale generations failed: %v", err)
	}
	if reset, err := models.ResetStaleRunningGenerations(db); err != nil {
		log.Printf("Warning: reset stale generations failed: %v", err)
	} else if reset > 0 {
		log.Printf("Reset %d stale AI Coach generation(s) from prior process", reset)
	}
	for _, g := range stale {
		athleteName := "athlete"
		if a, err := models.GetAthleteByID(db, g.AthleteID); err == nil && a != nil {
			athleteName = a.Name
		}
		notify.Send(db, notify.Request{
			UserID:    g.RequestedBy,
			Type:      models.NotifyGenerationFailed,
			Title:     fmt.Sprintf("AI Coach draft failed for %s", athleteName),
			Message:   "Server restarted during generation. Please try again.",
			Link:      fmt.Sprintf("/athletes/%d/generate", g.AthleteID),
			AthleteID: sql.NullInt64{Int64: g.AthleteID, Valid: true},
		})
	}

	// Bootstrap secret key for encrypting sensitive settings.
	if _, source, err := models.GetOrCreateSecretKey(db); err != nil {
		log.Printf("Warning: secret key not available — sensitive settings will not be encrypted: %v", err)
	} else {
		switch source {
		case "generated":
			log.Printf("Secret key generated and stored in database")
		case "database":
			log.Printf("Secret key loaded from database")
		case "env":
			log.Printf("Secret key loaded from REPLOG_SECRET_KEY environment variable")
		}
	}

	// Bootstrap admin user if no users exist.
	if err := bootstrapAdmin(db); err != nil {
		log.Fatalf("Failed to bootstrap admin: %v", err)
	}

	// Bootstrap seed catalog (equipment, exercises, programs) on first run.
	if err := bootstrapCatalog(db); err != nil {
		log.Fatalf("Failed to bootstrap seed catalog: %v", err)
	}

	// Start background maintenance scheduler (daily: expired tokens, old notifications).
	maintenance := scheduler.New(db)
	maintenance.Start()

	// Determine base URL for generating absolute URLs (e.g. login token links).
	baseURL := strings.TrimRight(os.Getenv("REPLOG_BASE_URL"), "/")
	if baseURL != "" {
		if _, err := url.Parse(baseURL); err != nil {
			log.Fatalf("Invalid REPLOG_BASE_URL: %v", err)
		}
		log.Printf("Base URL: %s", baseURL)
	}

	// Set up session manager with SQLite store.
	sessionManager := scs.New()
	sessionStore := sqlite3store.New(db)
	sessionManager.Store = sessionStore
	sessionManager.Lifetime = 30 * 24 * time.Hour // 30 days
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	// Secure cookies: explicit override via REPLOG_SECURE_COOKIES, or auto-derived from base URL scheme.
	switch {
	case os.Getenv("REPLOG_SECURE_COOKIES") != "":
		sessionManager.Cookie.Secure = os.Getenv("REPLOG_SECURE_COOKIES") == "true"
	case strings.HasPrefix(baseURL, "https://"):
		sessionManager.Cookie.Secure = true
	}

	// Enable HSTS whenever cookies are flagged Secure — both signal that the
	// deployment is reachable over HTTPS. Do not pin browsers to HTTPS for
	// plaintext localhost dev (default off).
	middleware.EnableHSTS(sessionManager.Cookie.Secure)

	// Compute CSP script-src allow-list from the embedded SPA's
	// inline scripts so we can drop 'unsafe-inline' (issue #8).
	configureCSPFromFrontend()

	// Configure WebAuthn for passkey support (optional).
	rpID := os.Getenv("REPLOG_WEBAUTHN_RPID")
	rpOrigins := os.Getenv("REPLOG_WEBAUTHN_ORIGINS")

	var passkeysHandler *passkeys.Handler
	if rpID != "" && rpOrigins != "" {
		origins := strings.Split(rpOrigins, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}

		wa, err := webauthn.New(&webauthn.Config{
			RPDisplayName: "RepLog",
			RPID:          rpID,
			RPOrigins:     origins,
			AuthenticatorSelection: protocol.AuthenticatorSelection{
				UserVerification: protocol.VerificationPreferred,
			},
		})
		if err != nil {
			log.Fatalf("Failed to configure WebAuthn: %v", err)
		}

		passkeysHandler = &passkeys.Handler{
			DB:       db,
			Sessions: sessionManager,
			WebAuthn: wa,
		}
		log.Printf("WebAuthn enabled: RPID=%s, Origins=%v", rpID, origins)
	} else {
		log.Printf("WebAuthn disabled: set REPLOG_WEBAUTHN_RPID and REPLOG_WEBAUTHN_ORIGINS to enable passkeys")
	}

	// Initialize API handlers.
	apiHandlers := &api.Handlers{
		DB:        db,
		Sessions:  sessionManager,
		AvatarDir: avatarDir,
	}

	// Avatar file server (public — no auth required to load avatar images).
	avatarFS := http.FileServer(http.Dir(avatarDir))

	// Set up router.
	r := chi.NewRouter()

	// Global middleware — applied to every request.
	r.Use(middleware.RequestLogger)
	r.Use(middleware.SecurityHeaders)

	// CORS middleware for development (Vite dev server on different port).
	if corsCfg := middleware.CORSFromEnv(os.Getenv("REPLOG_CORS_ORIGINS")); corsCfg != nil {
		r.Use(middleware.CORS(*corsCfg))
		log.Printf("CORS enabled for origins: %v", corsCfg.AllowedOrigins)
	}

	// Rate limiter for authentication endpoints — 10 attempts per minute per IP.
	var trustedProxies []string
	if tp := os.Getenv("REPLOG_TRUSTED_PROXIES"); tp != "" {
		for _, p := range strings.Split(tp, ",") {
			if p = strings.TrimSpace(p); p != "" {
				trustedProxies = append(trustedProxies, p)
			}
		}
		log.Printf("Trusted proxies: %v", trustedProxies)
	}
	authLimiter := middleware.NewRateLimiter(10, time.Minute, trustedProxies...)

	// Middleware adapter — converts existing RequireAuth middleware to chi-compatible.
	withAuth := func(next http.Handler) http.Handler {
		return middleware.RequireAuth(sessionManager, db, next)
	}

	// --- Health checks (public) ---
	r.Get("/health", handleHealthz)
	r.Get("/healthz", handleHealthz)
	r.Get("/readyz", handleReadyz(db))

	// --- Avatars (public — image responses, no auth required) ---
	r.Get("/avatars/{filename}", func(w http.ResponseWriter, r *http.Request) {
		// Strip the route prefix and serve from the avatar directory.
		http.StripPrefix("/avatars/", avatarFS).ServeHTTP(w, r)
	})

	// --- API documentation (public) ---
	r.Get("/api/docs", api.DocsHandler)
	r.Get("/api/docs/openapi.yaml", api.SpecHandler)

	// --- JSON API routes ---
	r.Route("/api", func(r chi.Router) {
		r.Use(sessionManager.LoadAndSave)

		// Public API endpoints (login) — rate limited.
		r.Group(func(r chi.Router) {
			r.Use(authLimiter.Limit)
			r.Post("/login", apiHandlers.Login)
			r.Get("/auth/token/{token}", apiHandlers.TokenLogin)
			// Passkey login ceremony (unauthenticated).
			if passkeysHandler != nil {
				r.Get("/passkeys/login/begin", passkeysHandler.BeginLogin)
				r.Post("/passkeys/login/finish", passkeysHandler.FinishLogin)
			}
		})

		// Authenticated API endpoints.
		r.Group(func(r chi.Router) {
			r.Use(withAuth)

			// CSRF protection comes from the session cookie's SameSite=Lax
			// attribute plus same-origin deployment (the SPA is served from
			// this same binary in production). A cross-site form post or
			// fetch from another origin will not include the session
			// cookie, so no token check is needed at the handler layer.
			//
			// If we ever ship a non-same-origin client (mobile WebView,
			// desktop wrapper, third-party automation) we will need an
			// explicit CSRF token middleware — see git history for a
			// previous implementation.

			r.Get("/me", apiHandlers.Me)
			r.Post("/logout", apiHandlers.Logout)
			r.Get("/preferences", apiHandlers.GetPreferences)
			r.Put("/preferences", apiHandlers.UpdatePreferences)
			r.Get("/dashboard", apiHandlers.Dashboard)

			// Avatars.
			r.Post("/avatars/upload", apiHandlers.AvatarUpload)
			r.Post("/avatars/delete", apiHandlers.AvatarDelete)

			// Notification Preferences.
			r.Get("/notifications/preferences", apiHandlers.ListNotificationPreferences)
			r.Put("/notifications/preferences", apiHandlers.UpdateNotificationPreference)

			// Athletes.
			r.Get("/athletes", apiHandlers.ListAthletes)
			r.Post("/athletes", apiHandlers.CreateAthlete)
			r.Get("/athletes/{id}", apiHandlers.GetAthlete)
			r.Put("/athletes/{id}", apiHandlers.UpdateAthlete)
			r.Delete("/athletes/{id}", apiHandlers.DeleteAthlete)

			// Exercises.
			r.Get("/exercises", apiHandlers.ListExercises)
			r.Post("/exercises", apiHandlers.CreateExercise)
			r.Get("/exercises/{id}", apiHandlers.GetExercise)
			r.Put("/exercises/{id}", apiHandlers.UpdateExercise)
			r.Delete("/exercises/{id}", apiHandlers.DeleteExercise)

			// Workouts.
			r.Get("/athletes/{id}/workouts", apiHandlers.ListWorkouts)
			r.Post("/athletes/{id}/workouts", apiHandlers.CreateWorkout)
			r.Get("/athletes/{id}/workouts/{workoutID}", apiHandlers.GetWorkout)
			r.Delete("/athletes/{id}/workouts/{workoutID}", apiHandlers.DeleteWorkout)

			// Workout Sets.
			r.Post("/athletes/{id}/workouts/{workoutID}/sets", apiHandlers.AddWorkoutSet)
			r.Put("/athletes/{id}/workouts/{workoutID}/sets/{setID}", apiHandlers.UpdateWorkoutSet)
			r.Delete("/athletes/{id}/workouts/{workoutID}/sets/{setID}", apiHandlers.DeleteWorkoutSet)

			// Workout Notes.
			r.Put("/athletes/{id}/workouts/{workoutID}/notes", apiHandlers.UpdateWorkoutNotes)

			// Body Weights.
			r.Get("/athletes/{id}/body-weights", apiHandlers.ListBodyWeights)
			r.Post("/athletes/{id}/body-weights", apiHandlers.CreateBodyWeight)
			r.Delete("/athletes/{id}/body-weights/{bwID}", apiHandlers.DeleteBodyWeight)

			// Training Maxes.
			r.Get("/athletes/{id}/training-maxes", apiHandlers.ListTrainingMaxes)
			r.Post("/athletes/{id}/training-maxes", apiHandlers.CreateTrainingMax)
			r.Get("/athletes/{id}/exercises/{exerciseID}/training-maxes", apiHandlers.GetTrainingMaxHistory)

			// Athlete Programs.
			r.Get("/athletes/{id}/programs", apiHandlers.ListAthletePrograms)
			r.Post("/athletes/{id}/programs", apiHandlers.AssignProgramToAthlete)
			r.Post("/athletes/{id}/programs/{programID}/deactivate", apiHandlers.DeactivateAthleteProgram)
			r.Post("/athletes/{id}/programs/{programID}/reactivate", apiHandlers.ReactivateAthleteProgram)
			r.Delete("/athletes/{id}/programs/{programID}", apiHandlers.DeleteAthleteProgram)

			// Accessory Plans.
			r.Get("/athletes/{id}/accessories", apiHandlers.ListAccessoryPlans)
			r.Post("/athletes/{id}/accessories", apiHandlers.CreateAccessoryPlan)
			r.Put("/athletes/{id}/accessories/{planID}", apiHandlers.UpdateAccessoryPlan)
			r.Post("/athletes/{id}/accessories/{planID}/deactivate", apiHandlers.DeactivateAccessoryPlan)
			r.Delete("/athletes/{id}/accessories/{planID}", apiHandlers.DeleteAccessoryPlan)

			// Journal.
			r.Get("/athletes/{id}/journal", apiHandlers.ListJournalEntries)
			r.Post("/athletes/{id}/notes", apiHandlers.CreateAthleteNote)
			r.Put("/athletes/{id}/notes/{noteID}", apiHandlers.UpdateAthleteNote)
			r.Delete("/athletes/{id}/notes/{noteID}", apiHandlers.DeleteAthleteNote)

			// Athlete Goal.
			r.Put("/athletes/{id}/goal", apiHandlers.UpdateAthleteGoal)

			// Prescription (today's workout).
			r.Get("/athletes/{id}/prescription", apiHandlers.GetPrescription)

			// Athlete Promotion.
			r.Post("/athletes/{id}/promote", apiHandlers.PromoteAthlete)

			// Exercise History.
			r.Get("/athletes/{id}/exercises/{exerciseID}/history", apiHandlers.ListExerciseHistory)

			// Exercise Equipment.
			r.Get("/exercises/{id}/equipment", apiHandlers.ListExerciseEquipment)
			r.Post("/exercises/{id}/equipment", apiHandlers.AddExerciseEquipment)
			r.Delete("/exercises/{id}/equipment/{equipmentID}", apiHandlers.RemoveExerciseEquipment)

			// Athlete Equipment.
			r.Get("/athletes/{id}/equipment", apiHandlers.ListAthleteEquipment)
			r.Post("/athletes/{id}/equipment", apiHandlers.AddAthleteEquipment)
			r.Delete("/athletes/{id}/equipment/{equipmentID}", apiHandlers.RemoveAthleteEquipment)

			// Exercise Assignments.
			r.Get("/athletes/{id}/assignments", apiHandlers.ListAssignments)
			r.Post("/athletes/{id}/assignments", apiHandlers.AssignExercise)
			r.Post("/athletes/{id}/assignments/{assignmentID}/deactivate", apiHandlers.DeactivateAssignment)
			r.Post("/athletes/{id}/assignments/reactivate", apiHandlers.ReactivateAssignment)

			// Program Compatibility.
			r.Get("/athletes/{id}/program-compatibility", apiHandlers.CheckProgramCompatibility)

			// TM Setup.
			r.Get("/athletes/{id}/missing-tms", apiHandlers.ListMissingTMs)
			r.Post("/athletes/{id}/batch-tms", apiHandlers.BatchSetTMs)

			// Cycle Review.
			r.Get("/athletes/{id}/cycle-review", apiHandlers.GetCycleReview)
			r.Post("/athletes/{id}/cycle-review", apiHandlers.ApplyTMBumps)

			// Equipment Catalog.
			r.Get("/equipment", apiHandlers.ListEquipment)
			r.Post("/equipment", apiHandlers.CreateEquipment)
			r.Put("/equipment/{equipmentID}", apiHandlers.UpdateEquipment)
			r.Delete("/equipment/{equipmentID}", apiHandlers.DeleteEquipment)

			// Program Templates.
			r.Get("/programs", apiHandlers.ListProgramTemplates)
			r.Post("/programs", apiHandlers.CreateProgramTemplate)
			r.Get("/programs/{id}", apiHandlers.GetProgramTemplate)
			r.Put("/programs/{id}", apiHandlers.UpdateProgramTemplate)
			r.Delete("/programs/{id}", apiHandlers.DeleteProgramTemplate)
			r.Post("/programs/{id}/copy-week", apiHandlers.CopyWeek)

			// Prescribed Sets.
			r.Post("/programs/{id}/sets", apiHandlers.AddPrescribedSet)
			r.Put("/programs/{id}/sets/{setID}", apiHandlers.UpdatePrescribedSet)
			r.Delete("/programs/{id}/sets/{setID}", apiHandlers.DeletePrescribedSet)

			// Progression Rules.
			r.Get("/programs/{id}/rules", apiHandlers.ListProgressionRules)
			r.Post("/programs/{id}/rules", apiHandlers.SetProgressionRule)
			r.Delete("/programs/{id}/rules/{ruleID}", apiHandlers.DeleteProgressionRule)

			// Notifications.
			r.Get("/notifications", apiHandlers.ListNotifications)
			r.Get("/notifications/count", apiHandlers.UnreadNotificationCount)
			r.Post("/notifications/{notificationID}/read", apiHandlers.MarkNotificationRead)
			r.Post("/notifications/read-all", apiHandlers.MarkAllNotificationsRead)

			// Users (admin only — handler checks IsAdmin internally).
			r.Get("/users", apiHandlers.ListUsers)
			r.Post("/users", apiHandlers.CreateUser)
			r.Get("/users/{userID}", apiHandlers.GetUser)
			r.Put("/users/{userID}", apiHandlers.UpdateUser)
			r.Delete("/users/{userID}", apiHandlers.DeleteUser)

			// Login Tokens (admin only).
			r.Get("/users/{userID}/tokens", apiHandlers.ListLoginTokens)
			r.Post("/users/{userID}/tokens", apiHandlers.CreateLoginToken)
			r.Delete("/users/{userID}/tokens/{tokenID}", apiHandlers.DeleteLoginToken)

			// Reviews (coach only — handler checks internally).
			r.Get("/reviews/pending", apiHandlers.ListPendingReviews)
			r.Post("/athletes/{id}/workouts/{workoutID}/review", apiHandlers.SubmitReview)
			r.Delete("/athletes/{id}/workouts/{workoutID}/review", apiHandlers.DeleteReview)

			// Admin Settings (admin only — handler checks internally).
			r.Get("/admin/settings", apiHandlers.ListSettings)
			r.Put("/admin/settings", apiHandlers.UpdateSetting)
			r.Post("/admin/settings/test-llm", apiHandlers.TestLLMConnection)
			r.Post("/admin/settings/test-notify", apiHandlers.TestNotifyConnection)

			// Catalog (admin only — handler checks internally).
			r.Get("/catalog/export", apiHandlers.CatalogExportJSON)
			r.Post("/catalog/import/upload", apiHandlers.CatalogImportUpload)
			r.Post("/catalog/import/execute", apiHandlers.CatalogImportExecute)

			// Import (coach only — handler checks internally).
			r.Post("/athletes/{id}/import/upload", apiHandlers.ImportUpload)
			r.Post("/athletes/{id}/import/execute", apiHandlers.ImportExecute)

			// AI Coach Generation (coach only — handler checks internally).
			r.Get("/athletes/{id}/generate", apiHandlers.GenerateFormData)
			r.Post("/athletes/{id}/generate", apiHandlers.GenerateSubmit)
			r.Get("/athletes/{id}/generations/{genID}", apiHandlers.GenerationStatus)
			r.Post("/athletes/{id}/generations/{genID}/cancel", apiHandlers.GenerationCancel)
			r.Post("/athletes/{id}/generations/{genID}/execute", apiHandlers.GenerationExecute)

			// Passkeys (user's own credentials).
			r.Get("/passkeys", apiHandlers.ListPasskeys)
			r.Delete("/passkeys/{id}", apiHandlers.DeletePasskey)
			r.Post("/passkeys/label", apiHandlers.SetPasskeyLabel)
			if passkeysHandler != nil {
				r.Get("/passkeys/register/begin", passkeysHandler.BeginRegistration)
				r.Post("/passkeys/register/finish", passkeysHandler.FinishRegistration)
			}

			// Setup wizard.
			r.Post("/setup/passkey/skip", apiHandlers.SkipPasskeySetup)

			// Impersonation.
			r.Post("/admin/impersonate/{userId}", apiHandlers.StartImpersonation)
			r.Post("/admin/stop-impersonating", apiHandlers.StopImpersonation)
			r.Get("/admin/impersonateable", apiHandlers.ImpersonateableUsers)
		})
	})

	// --- SPA fallback for all other routes ---
	// React Router handles client-side routing; the SPA serves all browser navigation.
	spaHandler := spaFallbackHandler()
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		// API routes that don't match should return 404 JSON, not the SPA.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			api.WriteError(w, http.StatusNotFound, "endpoint not found")
			return
		}
		spaHandler.ServeHTTP(w, r)
	})

	// Method-not-allowed responses: JSON for /api/, plain text otherwise.
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			api.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	// Start server with graceful shutdown.
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Listen for OS signals to trigger graceful shutdown.
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("RepLog %s (%s, %s) listening on %s", version, commit, date, addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Block until we receive a shutdown signal.
	sig := <-shutdownCh
	log.Printf("Received %s, shutting down gracefully...", sig)

	// Stop background goroutines.
	authLimiter.Stop()
	maintenance.Stop()
	sessionStore.StopCleanup()

	// Give in-flight requests up to 30 seconds to complete.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Forced shutdown: %v", err)
	}

	// Wait for in-flight AI Coach generation goroutines to land their
	// results in the DB. Bounded by generationTimeout (5 min) but in
	// practice provider calls finish fast and any stragglers will be
	// reset on next startup by ResetStaleRunningGenerations.
	genDone := make(chan struct{})
	go func() {
		apiHandlers.WaitForGenerations()
		close(genDone)
	}()
	select {
	case <-genDone:
	case <-time.After(10 * time.Second):
		log.Printf("Warning: AI Coach generations still running at shutdown; will reset on next start")
	}

	// Run SQLite optimize on shutdown — updates query planner statistics
	// so the next startup benefits from accurate stats.
	if _, err := db.Exec("PRAGMA optimize"); err != nil {
		log.Printf("Warning: PRAGMA optimize failed: %v", err)
	}

	log.Println("Server stopped")
}

// bootstrapAdmin creates the initial admin user from environment variables
// if no users exist in the database.
func bootstrapAdmin(db *sql.DB) error {
	count, err := models.CountUsers(db)
	if err != nil {
		return fmt.Errorf("check user count: %w", err)
	}
	if count > 0 {
		return nil
	}

	username := os.Getenv("REPLOG_ADMIN_USER")
	password := os.Getenv("REPLOG_ADMIN_PASS")
	email := os.Getenv("REPLOG_ADMIN_EMAIL")

	if username == "" || password == "" {
		return fmt.Errorf("no users exist and REPLOG_ADMIN_USER / REPLOG_ADMIN_PASS env vars are not set")
	}

	user, err := models.CreateUser(db, username, "", password, email, true, true, sql.NullInt64{})
	if err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}

	log.Printf("Bootstrapped admin user: %s (id=%d)", user.Username, user.ID)
	return nil
}

// bootstrapCatalog seeds the database with default equipment, exercises,
// and program templates from the embedded seed catalog on first run.
// If exercises already exist, seeding is skipped.
// Set REPLOG_SEED_CATALOG to an absolute path to use a custom catalog file.
func bootstrapCatalog(db *sql.DB) error {
	exercises, err := models.ListExercises(db, "")
	if err != nil {
		return fmt.Errorf("check exercises: %w", err)
	}
	if len(exercises) > 0 {
		return nil
	}

	// Load catalog data — env override or embedded default.
	var data []byte
	if path := os.Getenv("REPLOG_SEED_CATALOG"); path != "" {
		data, err = os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read seed catalog %s: %w", path, err)
		}
		log.Printf("Using custom seed catalog: %s", path)
	} else {
		data = database.SeedCatalog()
	}

	parsed, err := importers.ParseCatalogJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("parse seed catalog: %w", err)
	}

	// Build mappings — DB is empty so all entities will be created.
	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(parsed.Exercises, nil),
		Equipment: importers.BuildEquipmentMappings(parsed.Equipment, nil),
		Programs:  importers.BuildProgramMappings(parsed.Programs, nil),
		Parsed:    parsed,
	}

	result, err := models.ExecuteCatalogImport(db, ms, nil, false)
	if err != nil {
		return fmt.Errorf("execute seed catalog import: %w", err)
	}

	log.Printf("Seeded catalog: %d equipment, %d exercises, %d programs (%d prescribed sets, %d progression rules)",
		result.EquipmentCreated, result.ExercisesCreated, result.ProgramsCreated,
		result.PrescribedSets, result.ProgressionRules)

	return nil
}

// handleHealthz is a liveness probe — always returns 200 if the process is running.
func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, "ok")
}

// handleReadyz is a readiness probe — checks database connectivity.
func handleReadyz(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		if err := db.PingContext(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "not ready: %v\n", err)
			return
		}
		fmt.Fprintln(w, "ok")
	}
}

// spaFallbackHandler returns a handler that serves the React SPA from the
// embedded filesystem. For non-asset routes, it serves index.html so that
// client-side routing works correctly.
//
// Cache strategy (so deploys don't strand users):
//
//   - /assets/* — Vite hashes the filenames, so they're safe to cache
//     forever (immutable). On a new deploy the HTML references a new
//     hashed name; nothing reads the old one once a fresh HTML loads.
//   - index.html — no-cache so the browser always revalidates and
//     picks up new chunk references on the next navigation, instead of
//     trying to import() an asset path the server no longer has.
//
// Critical: when /assets/<something>.js is missing (stale cached HTML
// from a prior deploy), we MUST return 404 — falling back to index.html
// makes the browser try to evaluate HTML as JavaScript and emit the
// useless "text/html is not a valid JavaScript MIME type" error. A 404
// lets the user's hard reload (or our SW, if/when we add one) recover
// cleanly.
func spaFallbackHandler() http.Handler {
	// Strip the "dist" prefix so the embedded files are served at root.
	dist, err := fs.Sub(frontend.DistFS, "dist")
	if err != nil {
		// If the frontend is not embedded (dev mode), return a placeholder.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Frontend not available — run the Vite dev server", http.StatusNotFound)
		})
	}

	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try to serve the exact file first (JS, CSS, images).
		if f, err := dist.Open(path); err == nil {
			f.Close()
			// Cache-control headers per-asset class. Set BEFORE the
			// file server runs so they're attached to the response.
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else if path == "index.html" {
				w.Header().Set("Cache-Control", "no-cache")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// Stale-asset request from a cached HTML referencing chunks the
		// new build no longer contains. Return 404 instead of HTML so
		// the browser doesn't try to evaluate index.html as JavaScript.
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			http.NotFound(w, r)
			return
		}

		// For all other routes, serve index.html (SPA client-side routing).
		// no-cache on the HTML so the next request picks up any new asset
		// references after a deploy.
		w.Header().Set("Cache-Control", "no-cache")
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
