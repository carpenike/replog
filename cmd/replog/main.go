package main

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/handlers"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

//go:embed all:templates
var templateFS embed.FS

//go:embed all:static
var staticFS embed.FS

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

	// Bootstrap admin user if no users exist.
	if err := bootstrapAdmin(db); err != nil {
		log.Fatalf("Failed to bootstrap admin: %v", err)
	}

	// Parse templates once at startup.
	tc, err := handlers.NewTemplateCache(templateFS)
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Set up session manager with SQLite store.
	sessionManager := scs.New()
	sessionManager.Store = sqlite3store.New(db)
	sessionManager.Lifetime = 30 * 24 * time.Hour // 30 days
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode
	sessionManager.Cookie.Secure = os.Getenv("REPLOG_SECURE_COOKIES") == "true"

	// Initialize handlers.
	auth := &handlers.Auth{
		DB:        db,
		Sessions:  sessionManager,
		Templates: tc,
	}
	pages := &handlers.Pages{
		DB:        db,
		Templates: tc,
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
		Templates: tc,
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

	// Set up routes.
	mux := http.NewServeMux()

	// Static files and health check — no auth required.
	mux.Handle("GET /static/", http.FileServerFS(staticFS))
	mux.HandleFunc("GET /health", handleHealth)

	// Login/logout — session loaded but no auth required.
	mux.Handle("GET /login", sessionManager.LoadAndSave(http.HandlerFunc(auth.LoginPage)))
	mux.Handle("POST /login", sessionManager.LoadAndSave(http.HandlerFunc(auth.LoginSubmit)))
	mux.Handle("POST /logout", sessionManager.LoadAndSave(http.HandlerFunc(auth.Logout)))

	// Authenticated routes — wrapped with RequireAuth + CSRF middleware.
	requireAuth := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(sessionManager, db, middleware.CSRFProtect(sessionManager, http.HandlerFunc(h)))
	}

	// Coach-only routes — RequireAuth + CSRF + RequireCoach for defense-in-depth.
	// Handlers also check is_coach inline, but this provides an extra layer.
	requireCoach := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(sessionManager, db, middleware.CSRFProtect(sessionManager, middleware.RequireCoach(http.HandlerFunc(h))))
	}

	mux.Handle("GET /{$}", requireAuth(pages.Index))

	// Athletes CRUD.
	mux.Handle("GET /athletes", requireAuth(athletes.List))
	mux.Handle("GET /athletes/new", requireCoach(athletes.NewForm))
	mux.Handle("POST /athletes", requireCoach(athletes.Create))
	mux.Handle("GET /athletes/{id}", requireAuth(athletes.Show))
	mux.Handle("GET /athletes/{id}/edit", requireCoach(athletes.EditForm))
	mux.Handle("POST /athletes/{id}", requireCoach(athletes.Update))
	mux.Handle("POST /athletes/{id}/delete", requireCoach(athletes.Delete))
	mux.Handle("POST /athletes/{id}/promote", requireCoach(athletes.Promote))

	// Exercises CRUD.
	mux.Handle("GET /exercises", requireAuth(exercises.List))
	mux.Handle("GET /exercises/new", requireCoach(exercises.NewForm))
	mux.Handle("POST /exercises", requireCoach(exercises.Create))
	mux.Handle("GET /exercises/{id}", requireAuth(exercises.Show))
	mux.Handle("GET /exercises/{id}/edit", requireCoach(exercises.EditForm))
	mux.Handle("POST /exercises/{id}", requireCoach(exercises.Update))
	mux.Handle("POST /exercises/{id}/delete", requireCoach(exercises.Delete))

	// Assignments (coach only).
	mux.Handle("GET /athletes/{id}/assignments/new", requireCoach(assignments.AssignForm))
	mux.Handle("POST /athletes/{id}/assignments", requireCoach(assignments.Assign))
	mux.Handle("POST /athletes/{id}/assignments/{assignmentID}/deactivate", requireCoach(assignments.Deactivate))
	mux.Handle("POST /athletes/{id}/assignments/reactivate", requireCoach(assignments.Reactivate))

	// Training Maxes.
	mux.Handle("GET /athletes/{id}/exercises/{exerciseID}/training-maxes", requireAuth(trainingMaxes.History))
	mux.Handle("GET /athletes/{id}/exercises/{exerciseID}/training-maxes/new", requireCoach(trainingMaxes.NewForm))
	mux.Handle("POST /athletes/{id}/exercises/{exerciseID}/training-maxes", requireCoach(trainingMaxes.Create))

	// Exercise History per athlete.
	mux.Handle("GET /athletes/{id}/exercises/{exerciseID}/history", requireAuth(exercises.ExerciseHistory))

	// Body Weights.
	mux.Handle("GET /athletes/{id}/body-weights", requireAuth(bodyWeights.List))
	mux.Handle("POST /athletes/{id}/body-weights", requireAuth(bodyWeights.Create))
	mux.Handle("POST /athletes/{id}/body-weights/{bwID}/delete", requireAuth(bodyWeights.Delete))

	// Workouts.
	mux.Handle("GET /athletes/{id}/workouts", requireAuth(workouts.List))
	mux.Handle("GET /athletes/{id}/workouts/new", requireAuth(workouts.NewForm))
	mux.Handle("POST /athletes/{id}/workouts", requireAuth(workouts.Create))
	mux.Handle("GET /athletes/{id}/workouts/{workoutID}", requireAuth(workouts.Show))
	mux.Handle("POST /athletes/{id}/workouts/{workoutID}/notes", requireAuth(workouts.UpdateNotes))
	mux.Handle("POST /athletes/{id}/workouts/{workoutID}/delete", requireCoach(workouts.Delete))
	mux.Handle("POST /athletes/{id}/workouts/{workoutID}/sets", requireAuth(workouts.AddSet))
	mux.Handle("GET /athletes/{id}/workouts/{workoutID}/sets/{setID}/edit", requireAuth(workouts.EditSetForm))
	mux.Handle("POST /athletes/{id}/workouts/{workoutID}/sets/{setID}", requireAuth(workouts.UpdateSet))
	mux.Handle("POST /athletes/{id}/workouts/{workoutID}/sets/{setID}/delete", requireAuth(workouts.DeleteSet))

	// Users (coach-only).
	mux.Handle("GET /users", requireCoach(users.List))
	mux.Handle("GET /users/new", requireCoach(users.NewForm))
	mux.Handle("POST /users", requireCoach(users.Create))
	mux.Handle("GET /users/{id}/edit", requireCoach(users.EditForm))
	mux.Handle("POST /users/{id}", requireCoach(users.Update))
	mux.Handle("POST /users/{id}/delete", requireCoach(users.Delete))

	// User Preferences (self-service — any authenticated user).
	mux.Handle("GET /preferences", requireAuth(preferences.EditForm))
	mux.Handle("POST /preferences", requireAuth(preferences.Update))

	// Program Templates (coach-only for management).
	mux.Handle("GET /programs", requireCoach(programs.List))
	mux.Handle("GET /programs/new", requireCoach(programs.NewForm))
	mux.Handle("POST /programs", requireCoach(programs.Create))
	mux.Handle("GET /programs/{id}", requireCoach(programs.Show))
	mux.Handle("GET /programs/{id}/edit", requireCoach(programs.EditForm))
	mux.Handle("POST /programs/{id}", requireCoach(programs.Update))
	mux.Handle("POST /programs/{id}/delete", requireCoach(programs.Delete))
	mux.Handle("POST /programs/{id}/sets", requireCoach(programs.AddSet))
	mux.Handle("POST /programs/{id}/sets/{setID}/delete", requireCoach(programs.DeleteSet))

	// Athlete Programs (assignment + prescription).
	mux.Handle("GET /athletes/{id}/program/assign", requireCoach(programs.AssignProgramForm))
	mux.Handle("POST /athletes/{id}/program", requireCoach(programs.AssignProgram))
	mux.Handle("POST /athletes/{id}/program/deactivate", requireCoach(programs.DeactivateProgram))
	mux.Handle("GET /athletes/{id}/prescription", requireAuth(programs.Prescription))

	// Start server.
	log.Printf("RepLog listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
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

	user, err := models.CreateUser(db, username, password, email, true, sql.NullInt64{})
	if err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}

	log.Printf("Bootstrapped admin user: %s (id=%d)", user.Username, user.ID)
	return nil
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, "ok")
}
