package main

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
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
	"github.com/carpenike/replog/internal/handlers"
	"github.com/carpenike/replog/internal/importers"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
	"github.com/carpenike/replog/internal/scheduler"
	frontend "github.com/carpenike/replog/web"
)

//go:embed all:templates
var templateFS embed.FS

//go:embed all:static
var staticFS embed.FS

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

	// Bootstrap secret key for encrypting sensitive settings.
	// Generates and stores a key automatically if REPLOG_SECRET_KEY is not set.
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

	// Parse templates once at startup.
	tc, err := handlers.NewTemplateCache(templateFS)
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Initialize cached app name from settings.
	handlers.InitAppName(models.GetAppName(db))

	// Determine base URL for generating absolute URLs (e.g. login token links).
	// When behind a reverse proxy, set this to the external URL (e.g. https://replog.example.com).
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

	// Initialize handlers.
	auth := &handlers.Auth{
		DB:        db,
		Sessions:  sessionManager,
		Templates: tc,
	}
	pages := &handlers.Pages{
		DB:        db,
		Templates: tc,
		Scheduler: maintenance,
	}
	athletes := &handlers.Athletes{
		DB:        db,
		Templates: tc,
	}
	exercises := &handlers.Exercises{
		DB:        db,
		Templates: tc,
	}
	assignments := &handlers.Assignments{
		DB:        db,
		Templates: tc,
	}
	trainingMaxes := &handlers.TrainingMaxes{
		DB:        db,
		Templates: tc,
	}
	workouts := &handlers.Workouts{
		DB:        db,
		Templates: tc,
	}
	users := &handlers.Users{
		DB:        db,
		Sessions:  sessionManager,
		Templates: tc,
		BaseURL:   baseURL,
	}
	bodyWeights := &handlers.BodyWeights{
		DB:        db,
		Templates: tc,
	}
	programs := &handlers.Programs{
		DB:        db,
		Templates: tc,
	}
	preferences := &handlers.Preferences{
		DB:        db,
		Templates: tc,
	}
	loginTokens := &handlers.LoginTokens{
		DB:        db,
		Sessions:  sessionManager,
		Templates: tc,
		BaseURL:   baseURL,
	}
	reviews := &handlers.Reviews{
		DB:        db,
		Templates: tc,
	}
	journal := &handlers.Journal{
		DB:        db,
		Templates: tc,
	}
	equipmentH := &handlers.Equipment{
		DB:        db,
		Templates: tc,
	}
	accessories := &handlers.Accessories{
		DB:        db,
		Templates: tc,
	}
	avatars := &handlers.Avatars{
		DB:        db,
		Templates: tc,
		AvatarDir: avatarDir,
	}
	importExport := &handlers.ImportExport{
		DB:        db,
		Sessions:  sessionManager,
		Templates: tc,
	}
	settings := &handlers.Settings{
		DB:        db,
		Templates: tc,
	}
	generate := &handlers.Generate{
		DB:        db,
		Sessions:  sessionManager,
		Templates: tc,
	}
	notifications := &handlers.Notifications{
		DB:        db,
		Templates: tc,
	}
	setup := &handlers.Setup{
		DB:        db,
		Sessions:  sessionManager,
		Templates: tc,
	}

	// Configure WebAuthn for passkey support.
	rpID := os.Getenv("REPLOG_WEBAUTHN_RPID")
	rpOrigins := os.Getenv("REPLOG_WEBAUTHN_ORIGINS")

	var passkeys *handlers.Passkeys
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

		passkeys = &handlers.Passkeys{
			DB:        db,
			Sessions:  sessionManager,
			WebAuthn:  wa,
			Templates: tc,
		}
		log.Printf("WebAuthn enabled: RPID=%s, Origins=%v", rpID, origins)
	} else {
		log.Printf("WebAuthn disabled: set REPLOG_WEBAUTHN_RPID and REPLOG_WEBAUTHN_ORIGINS to enable passkeys")
	}

	// Wire passkey setup into handlers that need it (if WebAuthn is enabled).
	if passkeys != nil {
		loginTokens.Setup = setup
		pages.Setup = setup
	}

	// Initialize API handlers.
	apiHandlers := &api.Handlers{
		DB:        db,
		Sessions:  sessionManager,
		AvatarDir: avatarDir,
	}

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

	// Custom error page for method-not-allowed. Wrapped with session loading so
	// logged-in users still see the sidebar navigation on error pages.
	r.MethodNotAllowed(sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc.RenderErrorPage(w, r, http.StatusMethodNotAllowed, "Method Not Allowed",
			"The requested method is not supported for this URL.")
	})).ServeHTTP)

	// Error page renderer for middleware — uses the template cache to render
	// styled error pages instead of plain text responses.
	renderError := middleware.ErrorRenderer(tc.RenderErrorPage)

	// Middleware adapters — convert existing middleware to chi-compatible middleware.
	withAuth := func(next http.Handler) http.Handler {
		return middleware.RequireAuth(sessionManager, db, next)
	}
	withCSRF := func(next http.Handler) http.Handler {
		return middleware.CSRFProtect(sessionManager, next)
	}
	withCoach := func(next http.Handler) http.Handler {
		return middleware.RequireCoach(renderError, next)
	}
	withAdmin := func(next http.Handler) http.Handler {
		return middleware.RequireAdmin(renderError, next)
	}

	// --- Public routes — no auth required ---
	r.Handle("/static/*", staticCacheControl(http.FileServerFS(staticFS)))
	r.Get("/health", handleHealthz)
	r.Get("/healthz", handleHealthz)
	r.Get("/readyz", handleReadyz(db))
	r.Get("/avatars/{filename}", avatars.Serve)

	// Rate limiter for authentication endpoints — 10 attempts per minute per IP.
	// REPLOG_TRUSTED_PROXIES is a comma-separated list of CIDRs or IPs whose
	// X-Forwarded-For headers should be trusted (e.g., "127.0.0.1,10.0.0.0/8").
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

	// --- Session-loaded routes — login/logout/token auth ---
	r.Group(func(r chi.Router) {
		r.Use(sessionManager.LoadAndSave)
		r.Use(authLimiter.Limit)

		r.Get("/login", auth.LoginPage)
		r.Post("/login", auth.LoginSubmit)
		r.Post("/logout", auth.Logout)
		r.Get("/auth/token/{token}", loginTokens.TokenLogin)

		// Passkey login ceremony (unauthenticated, session required).
		if passkeys != nil {
			r.Get("/passkeys/login/begin", passkeys.BeginLogin)
			r.Post("/passkeys/login/finish", passkeys.FinishLogin)
		}
	})

	// --- Authenticated routes — RequireAuth + CSRF ---
	r.Group(func(r chi.Router) {
		r.Use(withAuth)
		r.Use(withCSRF)

		r.Get("/", pages.Index)

		// Setup / onboarding wizard routes (authenticated, no coach role needed).
		r.Get("/setup/passkey", setup.PasskeySetup)
		r.Post("/setup/passkey/skip", setup.PasskeySetupSkip)

		// User Preferences (self-service — any authenticated user).
		r.Get("/preferences", preferences.EditForm)
		r.Post("/preferences", preferences.Update)

		// Avatar upload/delete (self-service — any authenticated user).
		r.Post("/avatars/upload", avatars.Upload)
		r.Post("/avatars/delete", avatars.Delete)

		// Athletes — read access.
		r.Get("/athletes", athletes.List)
		r.Get("/athletes/{id}", athletes.Show)

		// Exercises — read access.
		r.Get("/exercises", exercises.List)
		r.Get("/exercises/{id}", exercises.Show)

		// Equipment — read access.
		r.Get("/equipment", equipmentH.List)

		// Athlete Equipment — self-service.
		r.Get("/athletes/{id}/equipment", equipmentH.AthleteEquipmentPage)
		r.Post("/athletes/{id}/equipment", equipmentH.AddAthleteEquipment)
		r.Post("/athletes/{id}/equipment/{equipmentID}/delete", equipmentH.RemoveAthleteEquipment)

		// Accessory Plans.
		r.Get("/athletes/{id}/accessories", accessories.List)
		r.Post("/athletes/{id}/accessories", accessories.Create)
		r.Post("/athletes/{id}/accessories/{planID}/update", accessories.Update)
		r.Post("/athletes/{id}/accessories/{planID}/deactivate", accessories.Deactivate)
		r.Post("/athletes/{id}/accessories/{planID}/delete", accessories.Delete)

		// Training Max history — read access.
		r.Get("/athletes/{id}/exercises/{exerciseID}/training-maxes", trainingMaxes.History)

		// Exercise History per athlete — read access.
		r.Get("/athletes/{id}/exercises/{exerciseID}/history", exercises.ExerciseHistory)

		// Body Weights.
		r.Get("/athletes/{id}/body-weights", bodyWeights.List)
		r.Post("/athletes/{id}/body-weights", bodyWeights.Create)
		r.Post("/athletes/{id}/body-weights/{bwID}/delete", bodyWeights.Delete)

		// Workouts — athlete self-service.
		r.Get("/athletes/{id}/workouts", workouts.List)
		r.Get("/athletes/{id}/workouts/new", workouts.NewForm)
		r.Post("/athletes/{id}/workouts", workouts.Create)
		r.Get("/athletes/{id}/workouts/{workoutID}", workouts.Show)
		r.Post("/athletes/{id}/workouts/{workoutID}/notes", workouts.UpdateNotes)
		r.Post("/athletes/{id}/workouts/{workoutID}/sets", workouts.AddSet)
		r.Get("/athletes/{id}/workouts/{workoutID}/sets/{setID}/edit", workouts.EditSetForm)
		r.Post("/athletes/{id}/workouts/{workoutID}/sets/{setID}", workouts.UpdateSet)
		r.Post("/athletes/{id}/workouts/{workoutID}/sets/{setID}/delete", workouts.DeleteSet)
		r.Post("/athletes/{id}/workouts/{workoutID}/delete", workouts.Delete)

		// Athlete Programs — prescription view (athlete self-service).
		r.Get("/athletes/{id}/prescription", programs.Prescription)
		r.Get("/athletes/{id}/report", programs.CycleReport)

		// Journal — unified athlete timeline.
		r.Get("/athletes/{id}/journal", journal.Timeline)

		// Goal — self-service editing.
		r.Post("/athletes/{id}/goal", athletes.UpdateGoal)

		// Journal Notes — self-service (athletes can add their own notes).
		r.Post("/athletes/{id}/notes", journal.CreateNote)
		r.Post("/athletes/{id}/notes/{noteID}", journal.UpdateNote)

		// Export — self-service for own athlete data.
		r.Get("/athletes/{id}/export", importExport.ExportPage)
		r.Get("/athletes/{id}/export/json", importExport.ExportJSON)
		r.Get("/athletes/{id}/export/csv", importExport.ExportCSV)

		// Passkey registration (requires auth, not coach/admin).
		if passkeys != nil {
			r.Get("/passkeys/register/begin", passkeys.BeginRegistration)
			r.Post("/passkeys/register/finish", passkeys.FinishRegistration)
			r.Post("/passkeys/register/label", passkeys.SetLabel)

			// Credential management — handler checks ownership internally.
			r.Post("/users/{id}/passkeys/{credentialID}/delete", passkeys.DeleteCredential)
		}

		// Notifications — self-service for any authenticated user.
		r.Get("/notifications", notifications.List)
		r.Get("/notifications/count", notifications.UnreadCount)
		r.Get("/notifications/toast", notifications.Toast)
		r.Post("/notifications/{id}/read", notifications.MarkRead)
		r.Post("/notifications/read-all", notifications.MarkAllRead)
		r.Get("/notifications/preferences", notifications.Preferences)
		r.Post("/notifications/preferences", notifications.UpdatePreferences)
	})

	// --- Coach-only routes — RequireAuth + CSRF + RequireCoach ---
	r.Group(func(r chi.Router) {
		r.Use(withAuth)
		r.Use(withCSRF)
		r.Use(withCoach)

		// Athletes — management.
		r.Get("/athletes/new", athletes.NewForm)
		r.Post("/athletes", athletes.Create)
		r.Get("/athletes/{id}/edit", athletes.EditForm)
		r.Post("/athletes/{id}", athletes.Update)
		r.Post("/athletes/{id}/delete", athletes.Delete)
		r.Post("/athletes/{id}/promote", athletes.Promote)

		// Exercises — management.
		r.Get("/exercises/new", exercises.NewForm)
		r.Post("/exercises", exercises.Create)
		r.Get("/exercises/{id}/edit", exercises.EditForm)
		r.Post("/exercises/{id}", exercises.Update)
		r.Post("/exercises/{id}/delete", exercises.Delete)

		// Exercise Equipment — management.
		r.Post("/exercises/{id}/equipment", equipmentH.AddExerciseEquipment)
		r.Post("/exercises/{id}/equipment/{equipmentID}/delete", equipmentH.RemoveExerciseEquipment)

		// Equipment catalog — management.
		r.Get("/equipment/new", equipmentH.NewForm)
		r.Post("/equipment", equipmentH.Create)
		r.Get("/equipment/{id}/edit", equipmentH.EditForm)
		r.Post("/equipment/{id}", equipmentH.Update)
		r.Post("/equipment/{id}/delete", equipmentH.Delete)

		// Assignments (coach only).
		r.Get("/athletes/{id}/assignments/new", assignments.AssignForm)
		r.Post("/athletes/{id}/assignments", assignments.Assign)
		r.Post("/athletes/{id}/assignments/{assignmentID}/deactivate", assignments.Deactivate)
		r.Post("/athletes/{id}/assignments/reactivate", assignments.Reactivate)

		// Training Maxes — management.
		r.Get("/athletes/{id}/exercises/{exerciseID}/training-maxes/new", trainingMaxes.NewForm)
		r.Post("/athletes/{id}/exercises/{exerciseID}/training-maxes", trainingMaxes.Create)

		// Workout Reviews (coach-only).
		r.Get("/reviews/pending", reviews.PendingReviews)
		r.Post("/athletes/{id}/workouts/{workoutID}/review", reviews.SubmitReview)
		r.Post("/athletes/{id}/workouts/{workoutID}/review/delete", reviews.DeleteReview)

		// Athlete Notes — delete is coach-only.
		r.Post("/athletes/{id}/notes/{noteID}/delete", journal.DeleteNote)

		// Program Templates (coach-only for management).
		r.Get("/programs", programs.List)
		r.Get("/programs/new", programs.NewForm)
		r.Post("/programs", programs.Create)
		r.Get("/programs/{id}", programs.Show)
		r.Get("/programs/{id}/edit", programs.EditForm)
		r.Post("/programs/{id}", programs.Update)
		r.Post("/programs/{id}/delete", programs.Delete)
		r.Post("/programs/{id}/sets", programs.AddSet)
		r.Post("/programs/{id}/sets/{setID}/update", programs.UpdateSet)
		r.Post("/programs/{id}/sets/{setID}/delete", programs.DeleteSet)
		r.Post("/programs/{id}/copy-week", programs.CopyWeek)

		// Progression Rules (coach-only).
		r.Post("/programs/{id}/progression", programs.AddProgressionRule)
		r.Post("/programs/{id}/progression/{ruleID}/delete", programs.DeleteProgressionRule)

		// Athlete Programs — assignment (coach-only).
		r.Get("/athletes/{id}/program/assign", programs.AssignProgramForm)
		r.Get("/athletes/{id}/program/compatibility", programs.ProgramCompatibility)
		r.Post("/athletes/{id}/program", programs.AssignProgram)
		r.Post("/athletes/{id}/program/deactivate", programs.DeactivateProgram)

		// Training Max Setup — batch TM entry after program assignment (coach-only).
		r.Get("/athletes/{id}/training-maxes/setup", programs.TMSetupForm)
		r.Post("/athletes/{id}/training-maxes/setup", programs.TMSetupSave)

		// Cycle Review — TM bump suggestions (coach-only).
		r.Get("/athletes/{id}/cycle-review", programs.CycleReview)
		r.Post("/athletes/{id}/cycle-review", programs.ApplyTMBumps)

		// AI Coach — program generation (coach-only).
		r.Get("/athletes/{id}/programs/generate", generate.Form)
		r.Post("/athletes/{id}/programs/generate", generate.Submit)
		r.Get("/athletes/{id}/programs/generate/preview", generate.Preview)
		r.Post("/athletes/{id}/programs/generate/preview", generate.SaveEdits)
		r.Post("/athletes/{id}/programs/generate/execute", generate.Execute)
		r.Get("/athletes/{id}/context.json", generate.ContextJSON)

		// Import — coach-only.
		r.Get("/athletes/{id}/import", importExport.ImportPage)
		r.Post("/athletes/{id}/import/upload", importExport.Upload)
		r.Get("/athletes/{id}/import/map", importExport.MapPage)
		r.Post("/athletes/{id}/import/preview", importExport.Preview)
		r.Post("/athletes/{id}/import/execute", importExport.Execute)

	})

	// --- Admin-only routes — RequireAuth + CSRF + RequireAdmin ---
	r.Group(func(r chi.Router) {
		r.Use(withAuth)
		r.Use(withCSRF)
		r.Use(withAdmin)

		// Users management.
		r.Get("/users", users.List)
		r.Get("/users/new", users.NewForm)
		r.Post("/users", users.Create)
		r.Get("/users/{id}/edit", users.EditForm)
		r.Post("/users/{id}", users.Update)
		r.Post("/users/{id}/delete", users.Delete)

		// Login Token management.
		r.Post("/users/{id}/tokens", loginTokens.GenerateToken)
		r.Post("/users/{id}/tokens/{tokenID}/delete", loginTokens.DeleteToken)

		// Catalog import/export — admin-only.
		r.Get("/catalog", importExport.CatalogExportPage)
		r.Get("/catalog/export/json", importExport.CatalogExportJSON)
		r.Get("/catalog/import", importExport.CatalogImportPage)
		r.Post("/catalog/import/upload", importExport.CatalogUpload)
		r.Get("/catalog/import/map", importExport.CatalogMapPage)
		r.Post("/catalog/import/preview", importExport.CatalogPreview)
		r.Post("/catalog/import/execute", importExport.CatalogExecute)

		// Application settings — admin-only.
		r.Get("/admin/settings", settings.Show)
		r.Post("/admin/settings", settings.Update)
		r.Post("/admin/settings/test-llm", settings.TestConnection)
		r.Post("/admin/settings/test-notify", notifications.TestNotify)
	})

	// --- JSON API routes (for SPA frontend) ---
	r.Route("/api", func(r chi.Router) {
		r.Use(sessionManager.LoadAndSave)

		// Public API endpoints (login) — rate limited.
		r.Group(func(r chi.Router) {
			r.Use(authLimiter.Limit)
			r.Post("/login", apiHandlers.Login)
		})

		// Authenticated API endpoints.
		r.Group(func(r chi.Router) {
			r.Use(withAuth)

			// CSRF protection is handled by SameSite=Lax session cookies.
			// No explicit CSRF middleware needed for same-origin SPA.

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

			// Admin Settings (admin only — handler checks internally).
			r.Get("/admin/settings", apiHandlers.ListSettings)
			r.Put("/admin/settings", apiHandlers.UpdateSetting)
			r.Post("/admin/settings/test-llm", apiHandlers.TestLLMConnection)

			// Import (coach only — handler checks internally).
			r.Post("/athletes/{id}/import/upload", apiHandlers.ImportUpload)
			r.Post("/athletes/{id}/import/execute", apiHandlers.ImportExecute)

			// AI Coach Generation (coach only — handler checks internally).
			r.Get("/athletes/{id}/generate", apiHandlers.GenerateFormData)
			r.Post("/athletes/{id}/generate", apiHandlers.GenerateSubmit)
			r.Post("/athletes/{id}/generate/execute", apiHandlers.GenerateExecute)
		})
	})

	// SPA fallback — serve the React frontend for unmatched routes.
	// In production the frontend is embedded; in development it's proxied via Vite.
	spaHandler := spaFallbackHandler()
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		// API routes that don't match should return 404 JSON, not the SPA.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			api.WriteError(w, http.StatusNotFound, "endpoint not found")
			return
		}
		// SSR routes still get the template-rendered 404 (backward compatibility).
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "text/html") && !strings.HasPrefix(r.URL.Path, "/api/") {
			// Check if this could be a SPA route — try serving the SPA index.
			spaHandler.ServeHTTP(w, r)
			return
		}
		sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tc.NotFound(w, r)
		})).ServeHTTP(w, r)
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

	result, err := models.ExecuteCatalogImport(db, ms, nil)
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

// staticCacheControl wraps a handler to set Cache-Control headers on static assets.
func staticCacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		next.ServeHTTP(w, r)
	})
}

// spaFallbackHandler returns a handler that serves the React SPA from the
// embedded filesystem. For non-asset routes, it serves index.html so that
// client-side routing works correctly.
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
		// Try to serve the exact file first (JS, CSS, images).
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Check if the file exists in the embedded FS.
		if f, err := dist.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// For all other routes, serve index.html (SPA client-side routing).
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
