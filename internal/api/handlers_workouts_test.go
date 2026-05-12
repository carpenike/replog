package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

// createExercise is a small fixture helper; not on testEnv because only a few
// test files need it.
func (e *testEnv) createExercise(t *testing.T, name string) *models.Exercise {
	t.Helper()
	ex, err := models.CreateExercise(e.DB, name, "", "", "", 0)
	if err != nil {
		t.Fatalf("create exercise %q: %v", name, err)
	}
	return ex
}

// createWorkout is a small fixture helper that creates a workout via the model
// layer (skipping HTTP) so tests can focus on the endpoint they're exercising.
func (e *testEnv) createWorkout(t *testing.T, athleteID int64, date string) *models.Workout {
	t.Helper()
	w, err := models.CreateWorkout(e.DB, athleteID, date, "", 0)
	if err != nil {
		t.Fatalf("create workout: %v", err)
	}
	return w
}

func TestCreateWorkout_Success(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/workouts", athlete.ID),
		`{"date":"2026-05-12","notes":"felt strong"}`, cookies)
	requireStatus(t, rr, http.StatusCreated)

	var got Workout
	decodeJSON(t, rr, &got)
	if got.AthleteID != athlete.ID {
		t.Errorf("got athlete_id=%d, want %d", got.AthleteID, athlete.ID)
	}
	if got.Date != "2026-05-12" {
		t.Errorf("got date=%q, want %q", got.Date, "2026-05-12")
	}
}

func TestCreateWorkout_DefaultsToToday(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/workouts", athlete.ID),
		`{}`, cookies)
	requireStatus(t, rr, http.StatusCreated)

	var got Workout
	decodeJSON(t, rr, &got)
	if got.Date == "" {
		t.Error("expected default date, got empty string")
	}
}

func TestCreateWorkout_OneWorkoutPerDay(t *testing.T) {
	// Schema: UNIQUE(athlete_id, date)
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	body := `{"date":"2026-05-12"}`
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/workouts", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)

	rr = env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/workouts", athlete.ID), body, cookies)
	if rr.Code == http.StatusCreated {
		t.Errorf("expected duplicate-date workout to be rejected, got 201")
	}
}

func TestCreateWorkout_RequiresAccess(t *testing.T) {
	env := setupTest(t)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	athleteOfA := env.createAthlete(t, "Charlie", coachA.ID)

	cookiesB := env.loginAs(t, coachB)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/workouts", athleteOfA.ID),
		`{"date":"2026-05-12"}`, cookiesB)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestCreateWorkout_Unauthenticated(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/workouts", athlete.ID),
		`{"date":"2026-05-12"}`, nil)
	requireStatus(t, rr, http.StatusUnauthorized)
}

// --- AddWorkoutSet ---

// TestAddWorkoutSet_DefaultRepTypeAndCategory regresses the bug fixed in ef4f7b3
// where the handler defaulted RepType to "standard" and Category to "working",
// both of which violated the schema CHECK constraints.
func TestAddWorkoutSet_DefaultRepTypeAndCategory(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	exercise := env.createExercise(t, "Bench Press")
	workout := env.createWorkout(t, athlete.ID, "2026-05-12")
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(`{"exercise_id":%d,"reps":5,"weight":135,"rpe":7}`, exercise.ID)
	rr := env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/workouts/%d/sets", athlete.ID, workout.ID),
		body, cookies)
	requireStatus(t, rr, http.StatusCreated)

	var got WorkoutSet
	decodeJSON(t, rr, &got)
	if got.WorkoutID != workout.ID {
		t.Errorf("got workout_id=%d, want %d", got.WorkoutID, workout.ID)
	}
	if got.Reps != 5 {
		t.Errorf("got reps=%d, want 5", got.Reps)
	}
	if got.RepType != "reps" {
		t.Errorf("got rep_type=%q, want %q (schema requires reps/each_side/seconds/distance)", got.RepType, "reps")
	}
	if got.Category != "main" {
		t.Errorf("got category=%q, want %q (schema requires main/supplemental/accessory)", got.Category, "main")
	}
}

func TestAddWorkoutSet_ExplicitRepTypeRespected(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	exercise := env.createExercise(t, "Plank")
	workout := env.createWorkout(t, athlete.ID, "2026-05-12")
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(
		`{"exercise_id":%d,"reps":60,"rep_type":"seconds","category":"accessory"}`,
		exercise.ID)
	rr := env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/workouts/%d/sets", athlete.ID, workout.ID),
		body, cookies)
	requireStatus(t, rr, http.StatusCreated)

	var got WorkoutSet
	decodeJSON(t, rr, &got)
	if got.RepType != "seconds" {
		t.Errorf("got rep_type=%q, want %q", got.RepType, "seconds")
	}
	if got.Category != "accessory" {
		t.Errorf("got category=%q, want %q", got.Category, "accessory")
	}
}

func TestAddWorkoutSet_ValidatesRequired(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	exercise := env.createExercise(t, "Squat")
	workout := env.createWorkout(t, athlete.ID, "2026-05-12")
	cookies := env.loginAs(t, coach)

	cases := []struct {
		name string
		body string
	}{
		{"missing exercise_id", `{"reps":5}`},
		{"missing reps", fmt.Sprintf(`{"exercise_id":%d}`, exercise.ID)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := env.do(t, "POST",
				fmt.Sprintf("/api/athletes/%d/workouts/%d/sets", athlete.ID, workout.ID),
				tc.body, cookies)
			requireStatus(t, rr, http.StatusBadRequest)
		})
	}
}

// --- ListWorkouts ---

func TestListWorkouts_ReturnsCreated(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	env.createWorkout(t, athlete.ID, "2026-05-10")
	env.createWorkout(t, athlete.ID, "2026-05-11")
	env.createWorkout(t, athlete.ID, "2026-05-12")
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/workouts", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var page WorkoutPage
	decodeJSON(t, rr, &page)
	if len(page.Workouts) != 3 {
		t.Errorf("got %d workouts, want 3", len(page.Workouts))
	}
}

// --- DeleteWorkout ---

func TestDeleteWorkout_Success(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	workout := env.createWorkout(t, athlete.ID, "2026-05-12")
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "DELETE",
		fmt.Sprintf("/api/athletes/%d/workouts/%d", athlete.ID, workout.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	// Subsequent GET should 404.
	rr = env.do(t, "GET",
		fmt.Sprintf("/api/athletes/%d/workouts/%d", athlete.ID, workout.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusNotFound)
}
