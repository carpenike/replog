package main

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
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
	templates := template.Must(template.ParseFS(templateFS,
		"templates/layouts/*.html",
		"templates/pages/*.html",
	))

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
		Templates: templates,
	}
	pages := &handlers.Pages{
		DB:        db,
		Templates: templates,
	}
	athletes := &handlers.Athletes{
		DB:        db,
		Templates: templates,
	}
	exercises := &handlers.Exercises{
		DB:        db,
		Templates: templates,
	}
	assignments := &handlers.Assignments{
		DB:        db,
		Templates: templates,
	}
	trainingMaxes := &handlers.TrainingMaxes{
		DB:        db,
		Templates: templates,
	}
	workouts := &handlers.Workouts{
		DB:        db,
		Templates: templates,
	}
	users := &handlers.Users{
		DB:        db,
		Templates: templates,
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

	// Authenticated routes — wrapped with RequireAuth middleware.
	requireAuth := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(sessionManager, db, http.HandlerFunc(h))
	}

	mux.Handle("GET /{$}", requireAuth(pages.Index))

	// Athletes CRUD.
	mux.Handle("GET /athletes", requireAuth(athletes.List))
	mux.Handle("GET /athletes/new", requireAuth(athletes.NewForm))
	mux.Handle("POST /athletes", requireAuth(athletes.Create))
	mux.Handle("GET /athletes/{id}", requireAuth(athletes.Show))
	mux.Handle("GET /athletes/{id}/edit", requireAuth(athletes.EditForm))
	mux.Handle("POST /athletes/{id}", requireAuth(athletes.Update))
	mux.Handle("POST /athletes/{id}/delete", requireAuth(athletes.Delete))

	// Exercises CRUD.
	mux.Handle("GET /exercises", requireAuth(exercises.List))
	mux.Handle("GET /exercises/new", requireAuth(exercises.NewForm))
	mux.Handle("POST /exercises", requireAuth(exercises.Create))
	mux.Handle("GET /exercises/{id}", requireAuth(exercises.Show))
	mux.Handle("GET /exercises/{id}/edit", requireAuth(exercises.EditForm))
	mux.Handle("POST /exercises/{id}", requireAuth(exercises.Update))
	mux.Handle("POST /exercises/{id}/delete", requireAuth(exercises.Delete))

	// Assignments.
	mux.Handle("GET /athletes/{id}/assignments/new", requireAuth(assignments.AssignForm))
	mux.Handle("POST /athletes/{id}/assignments", requireAuth(assignments.Assign))
	mux.Handle("POST /athletes/{id}/assignments/{assignmentID}/deactivate", requireAuth(assignments.Deactivate))

	// Training Maxes.
	mux.Handle("GET /athletes/{id}/exercises/{exerciseID}/training-maxes", requireAuth(trainingMaxes.History))
	mux.Handle("GET /athletes/{id}/exercises/{exerciseID}/training-maxes/new", requireAuth(trainingMaxes.NewForm))
	mux.Handle("POST /athletes/{id}/exercises/{exerciseID}/training-maxes", requireAuth(trainingMaxes.Create))

	// Exercise History per athlete.
	mux.Handle("GET /athletes/{id}/exercises/{exerciseID}/history", requireAuth(exercises.ExerciseHistory))

	// Workouts.
	mux.Handle("GET /athletes/{id}/workouts", requireAuth(workouts.List))
	mux.Handle("GET /athletes/{id}/workouts/new", requireAuth(workouts.NewForm))
	mux.Handle("POST /athletes/{id}/workouts", requireAuth(workouts.Create))
	mux.Handle("GET /athletes/{id}/workouts/{workoutID}", requireAuth(workouts.Show))
	mux.Handle("POST /athletes/{id}/workouts/{workoutID}/notes", requireAuth(workouts.UpdateNotes))
	mux.Handle("POST /athletes/{id}/workouts/{workoutID}/delete", requireAuth(workouts.Delete))
	mux.Handle("POST /athletes/{id}/workouts/{workoutID}/sets", requireAuth(workouts.AddSet))
	mux.Handle("GET /athletes/{id}/workouts/{workoutID}/sets/{setID}/edit", requireAuth(workouts.EditSetForm))
	mux.Handle("POST /athletes/{id}/workouts/{workoutID}/sets/{setID}", requireAuth(workouts.UpdateSet))
	mux.Handle("POST /athletes/{id}/workouts/{workoutID}/sets/{setID}/delete", requireAuth(workouts.DeleteSet))

	// Users (coach-only).
	mux.Handle("GET /users", requireAuth(users.List))
	mux.Handle("GET /users/new", requireAuth(users.NewForm))
	mux.Handle("POST /users", requireAuth(users.Create))
	mux.Handle("GET /users/{id}/edit", requireAuth(users.EditForm))
	mux.Handle("POST /users/{id}", requireAuth(users.Update))
	mux.Handle("POST /users/{id}/delete", requireAuth(users.Delete))

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

	user, err := models.CreateUser(db, username, password, email, true)
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
