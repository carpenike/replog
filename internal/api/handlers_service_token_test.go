package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

// serveBearer issues a request with an Authorization bearer. cookies, when
// set, ride along so a test can prove the token is not upgraded by a session.
func (e *testEnv) serveBearer(t *testing.T, method, path, token string, body any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if s, ok := body.(string); ok && s != "" {
		reader = strings.NewReader(s)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	e.Router.ServeHTTP(rr, req)
	return rr
}

func (e *testEnv) coachServiceToken(t *testing.T, userID int64) string {
	t.Helper()
	plaintext, err := models.CreateServiceToken(context.Background(), e.DB, userID, "coach-agent")
	if err != nil {
		t.Fatalf("create service token: %v", err)
	}
	var stored string
	if err := e.DB.QueryRow(`SELECT token_hash FROM service_tokens WHERE user_id = ?`, userID).Scan(&stored); err != nil {
		t.Fatalf("read stored hash: %v", err)
	}
	if stored == plaintext || strings.Contains(stored, plaintext) {
		t.Fatal("service_tokens persisted a usable token")
	}
	return plaintext
}

func TestServiceToken_LogsSet(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Dana", coach.ID)
	exercise := env.createExercise(t, "Back Squat")
	workout := env.createWorkout(t, athlete.ID, "2026-11-21")
	token := env.coachServiceToken(t, coach.ID)

	rr := env.serveBearer(t, "GET", "/api/dashboard", token, nil, nil)
	requireStatus(t, rr, http.StatusOK)

	rr = env.serveBearer(t, "POST",
		fmt.Sprintf("/api/athletes/%d/workouts/%d/sets", athlete.ID, workout.ID),
		token,
		fmt.Sprintf(`{"exercise_id":%d,"reps":5,"weight":225}`, exercise.ID),
		nil)
	requireStatus(t, rr, http.StatusCreated)
	var set WorkoutSet
	decodeJSON(t, rr, &set)
	if set.WorkoutID != workout.ID || set.ExerciseID != exercise.ID || set.Reps != 5 {
		t.Fatalf("logged set = %+v, want workout %d exercise %d reps 5", set, workout.ID, exercise.ID)
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == "session" && c.Value != "" {
			t.Fatalf("logging a set with the service token set a session cookie")
		}
	}

	// A follow-up with no bearer is not a session the token just minted.
	rr = env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/generations/1/execute", athlete.ID),
		nil, nil)
	requireStatus(t, rr, http.StatusUnauthorized)
}

func TestServiceToken_RefusedOnExecute_BrowserSessionApproves(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)
	token := env.coachServiceToken(t, coach.ID)

	genID := submitAndWait(t, env, athlete.ID, cookies,
		`{"program_name":"Mock","num_days":4,"num_weeks":3}`)
	path := fmt.Sprintf("/api/athletes/%d/generations/%d/execute", athlete.ID, genID)

	rr := env.serveBearer(t, "POST", path, token, nil, nil)
	requireStatus(t, rr, http.StatusForbidden)
	var denied APIError
	decodeJSON(t, rr, &denied)
	if denied.Error != serviceCoachingRefusal {
		t.Fatalf("error = %q, want %q", denied.Error, serviceCoachingRefusal)
	}

	// The refusal must not have claimed the draft.
	gen, err := models.GetGeneration(context.Background(), env.DB, genID)
	if err != nil {
		t.Fatalf("reload generation: %v", err)
	}
	if gen.ExecutedAt.Valid {
		t.Fatal("service token executed the generation")
	}

	// The same bearer plus the browser cookie is still the service credential.
	rr = env.serveBearer(t, "POST", path, token, nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)

	// The browser session of the same coach approves.
	rr = env.do(t, "POST", path, nil, cookies)
	requireStatus(t, rr, http.StatusOK)
}

func TestServiceToken_RefusesAssignTMAndPromote(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthleteWithTier(t, "Casey", "foundational", coach.ID)
	exercise := env.createExercise(t, "Bench Press")
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	cookies := env.loginAs(t, coach)
	token := env.coachServiceToken(t, coach.ID)

	assign := fmt.Sprintf(`{"template_id":%d,"start_date":"2026-11-21"}`, tpl.ID)
	tm := fmt.Sprintf(`{"exercise_id":%d,"weight":100}`, exercise.ID)
	batch := fmt.Sprintf(`{"maxes":[{"exercise_id":%d,"weight":100}]}`, exercise.ID)
	bumps := fmt.Sprintf(`{"bumps":[{"exercise_id":%d,"new_weight":105}]}`, exercise.ID)
	base := fmt.Sprintf("/api/athletes/%d", athlete.ID)

	refused := []struct {
		method, path, body string
	}{
		{"POST", base + "/programs", assign},
		{"POST", base + "/training-maxes", tm},
		{"POST", base + "/batch-tms", batch},
		{"POST", base + "/cycle-review", bumps},
		{"POST", base + "/promote", ""},
	}
	for _, c := range refused {
		var body any
		if c.body != "" {
			body = c.body
		}
		rr := env.serveBearer(t, c.method, c.path, token, body, cookies)
		requireStatus(t, rr, http.StatusForbidden)
		var got APIError
		decodeJSON(t, rr, &got)
		if got.Error != serviceCoachingRefusal {
			t.Errorf("%s %s error = %q, want the service-token refusal", c.method, c.path, got.Error)
		}
	}

	var tier string
	if err := env.DB.QueryRow(`SELECT tier FROM athletes WHERE id = ?`, athlete.ID).Scan(&tier); err != nil {
		t.Fatalf("tier: %v", err)
	}
	if tier != "foundational" {
		t.Fatalf("tier = %q after refused promote, want foundational", tier)
	}

	rr := env.do(t, "POST", base+"/programs", assign, cookies)
	requireStatus(t, rr, http.StatusCreated)
	rr = env.do(t, "POST", base+"/training-maxes", tm, cookies)
	requireStatus(t, rr, http.StatusCreated)
	rr = env.do(t, "POST", base+"/batch-tms", batch, cookies)
	requireStatus(t, rr, http.StatusOK)
	rr = env.do(t, "POST", base+"/cycle-review", bumps, cookies)
	requireStatus(t, rr, http.StatusOK)
	rr = env.do(t, "POST", base+"/promote", nil, cookies)
	requireStatus(t, rr, http.StatusOK)
}

func TestServiceToken_CrossAthleteSetForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	other := env.createUser(t, "other", true, false)
	athlete := env.createAthlete(t, "Foreign", other.ID)
	exercise := env.createExercise(t, "Deadlift")
	workout := env.createWorkout(t, athlete.ID, "2026-11-21")
	token := env.coachServiceToken(t, coach.ID)

	rr := env.serveBearer(t, "POST",
		fmt.Sprintf("/api/athletes/%d/workouts/%d/sets", athlete.ID, workout.ID),
		token,
		fmt.Sprintf(`{"exercise_id":%d,"reps":3,"weight":135}`, exercise.ID),
		nil)
	requireStatus(t, rr, http.StatusForbidden)

	var n int
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM workout_sets WHERE workout_id = ?`, workout.ID).Scan(&n); err != nil {
		t.Fatalf("count sets: %v", err)
	}
	if n != 0 {
		t.Fatalf("cross-athlete set rows = %d, want 0", n)
	}
}
