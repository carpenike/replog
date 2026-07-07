package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// exportRouter builds a chi router that mounts just the two export routes with
// the same session + auth wiring as production. It reuses the testEnv's session
// manager and DB, so cookies minted by env.loginAs authenticate here too. This
// is necessary because the shared setupTest router does not (yet) mount the
// export routes — main.go wires them separately.
func (e *testEnv) exportRouter() chi.Router {
	withAuth := func(next http.Handler) http.Handler {
		return middleware.RequireAuth(e.Sessions, e.DB, next)
	}
	r := chi.NewRouter()
	r.Route("/api", func(r chi.Router) {
		r.Use(e.Sessions.LoadAndSave)
		r.Group(func(r chi.Router) {
			r.Use(withAuth)
			r.Get("/athletes/{id}/export/json", e.Handlers.ExportAthleteJSON)
			r.Get("/athletes/{id}/export/csv", e.Handlers.ExportAthleteCSV)
		})
	})
	return r
}

// doOn issues a request through an arbitrary router (used for the export router).
func doOn(t *testing.T, router chi.Router, method, path string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func TestExportAthleteJSON_OwnedAthlete(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true /* coach */, false /* admin */)
	athlete := env.createAthlete(t, "Caydan", coach.ID)

	// Seed a workout with one set so the export has real content.
	ex, err := models.CreateExercise(env.DB, "Bench Press", "foundational", "", "", 120)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}
	workout, err := models.CreateWorkout(env.DB, athlete.ID, "2026-02-15", "felt strong", 0)
	if err != nil {
		t.Fatalf("create workout: %v", err)
	}
	if _, err := models.AddSet(env.DB, workout.ID, ex.ID, 5, 115.0, 7.5, "reps", "main", ""); err != nil {
		t.Fatalf("add set: %v", err)
	}
	if _, err := models.SetTrainingMax(env.DB, athlete.ID, ex.ID, 135.0, "2026-01-15", "initial"); err != nil {
		t.Fatalf("set training max: %v", err)
	}
	if _, err := models.CreateBodyWeight(env.DB, athlete.ID, "2026-02-01", 155.5, ""); err != nil {
		t.Fatalf("create body weight: %v", err)
	}

	router := env.exportRouter()
	cookies := env.loginAs(t, coach)

	rr := doOn(t, router, "GET", "/api/athletes/"+itoa(athlete.ID)+"/export/json", cookies)
	requireStatus(t, rr, http.StatusOK)

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected JSON content type, got %q", ct)
	}
	if cd := rr.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, "export.json") {
		t.Fatalf("expected attachment disposition, got %q", cd)
	}

	var got map[string]any
	decodeJSON(t, rr, &got)

	// Top-level keys per ADR 006 (plus notes + sessions).
	for _, key := range []string{
		"version", "exported_at", "type", "weight_unit", "athlete",
		"equipment", "exercises", "assignments", "training_maxes",
		"body_weights", "notes", "workouts", "programs", "sessions",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("export JSON missing top-level key %q", key)
		}
	}

	if got["type"] != "athlete" {
		t.Errorf("expected type=athlete, got %v", got["type"])
	}

	athleteObj, ok := got["athlete"].(map[string]any)
	if !ok || athleteObj["name"] != "Caydan" {
		t.Errorf("expected athlete.name=Caydan, got %v", got["athlete"])
	}

	workouts, ok := got["workouts"].([]any)
	if !ok || len(workouts) != 1 {
		t.Fatalf("expected 1 workout, got %v", got["workouts"])
	}
	w0 := workouts[0].(map[string]any)
	sets, ok := w0["sets"].([]any)
	if !ok || len(sets) != 1 {
		t.Fatalf("expected 1 set in workout, got %v", w0["sets"])
	}

	if tms, ok := got["training_maxes"].([]any); !ok || len(tms) != 1 {
		t.Errorf("expected 1 training max, got %v", got["training_maxes"])
	}
	if bws, ok := got["body_weights"].([]any); !ok || len(bws) != 1 {
		t.Errorf("expected 1 body weight, got %v", got["body_weights"])
	}
}

func TestExportAthleteCSV_OwnedAthlete(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Caydan", coach.ID)

	ex, err := models.CreateExercise(env.DB, "Squat", "foundational", "", "", 120)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}
	workout, err := models.CreateWorkout(env.DB, athlete.ID, "2026-02-15", "", 0)
	if err != nil {
		t.Fatalf("create workout: %v", err)
	}
	if _, err := models.AddSet(env.DB, workout.ID, ex.ID, 5, 225.0, 8.0, "reps", "main", "deep"); err != nil {
		t.Fatalf("add set: %v", err)
	}

	router := env.exportRouter()
	cookies := env.loginAs(t, coach)

	rr := doOn(t, router, "GET", "/api/athletes/"+itoa(athlete.ID)+"/export/csv", cookies)
	requireStatus(t, rr, http.StatusOK)

	if ct := rr.Header().Get("Content-Type"); ct != "text/csv" {
		t.Fatalf("expected text/csv content type, got %q", ct)
	}
	body := rr.Body.String()
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header + 1 data row, got %d lines:\n%s", len(lines), body)
	}
	if !strings.HasPrefix(lines[0], "date,exercise,set_number,reps,weight,rpe,rep_type,category,notes") {
		t.Errorf("unexpected CSV header: %q", lines[0])
	}
	if !strings.Contains(lines[1], "Squat") || !strings.Contains(lines[1], "225") {
		t.Errorf("unexpected CSV data row: %q", lines[1])
	}
}

func TestExportAthleteJSON_Forbidden(t *testing.T) {
	env := setupTest(t)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	athleteA := env.createAthlete(t, "OwnedByA", coachA.ID)

	router := env.exportRouter()
	cookies := env.loginAs(t, coachB) // coachB does not own athleteA

	rr := doOn(t, router, "GET", "/api/athletes/"+itoa(athleteA.ID)+"/export/json", cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestExportAthleteJSON_NotFound(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true) // admin passes CanAccessAthlete for any ID

	router := env.exportRouter()
	cookies := env.loginAs(t, admin)

	rr := doOn(t, router, "GET", "/api/athletes/999999/export/json", cookies)
	requireStatus(t, rr, http.StatusNotFound)
}

// itoa is a tiny local int64→string helper to keep call sites terse.
func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
