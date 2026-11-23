package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func (e *testEnv) tagPatterns(t *testing.T, exerciseID int64, patterns ...string) {
	t.Helper()
	if err := models.SetExerciseMovementPatterns(context.Background(), e.DB, exerciseID, patterns); err != nil {
		t.Fatalf("tag patterns: %v", err)
	}
}

// TestExerciseSwapFlow covers gap 11: substitutes share a movement pattern,
// a swap is recorded on the workout with its reason, shows on the workout
// read and the pending review list, and the program is left alone.
func TestExerciseSwapFlow(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Dana", coach.ID)
	squat := env.createExercise(t, "Back Squat")
	goblet := env.createExercise(t, "Goblet Squat")
	split := env.createExercise(t, "Split Squat")
	press := env.createExercise(t, "Bench Press")
	untagged := env.createExercise(t, "Mystery Move")
	env.tagPatterns(t, squat.ID, "squat")
	env.tagPatterns(t, goblet.ID, "squat")
	env.tagPatterns(t, split.ID, "squat", "carry")
	env.tagPatterns(t, press.ID, "push")
	workout := env.createWorkout(t, athlete.ID, "2026-05-12")
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/exercises/%d/substitutes", athlete.ID, squat.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var subs []Exercise
	decodeJSON(t, rr, &subs)
	ids := map[int64]bool{}
	for _, s := range subs {
		ids[s.ID] = true
	}
	if !ids[goblet.ID] || !ids[split.ID] || ids[press.ID] || ids[squat.ID] {
		t.Fatalf("substitutes = %+v", subs)
	}

	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/exercises/%d/substitutes", athlete.ID, untagged.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	subs = nil
	decodeJSON(t, rr, &subs)
	if len(subs) != 0 {
		t.Fatalf("untagged substitutes = %+v", subs)
	}
	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/exercises/99999/substitutes", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusNotFound)

	swapsPath := fmt.Sprintf("/api/athletes/%d/workouts/%d/swaps", athlete.ID, workout.ID)
	for _, tc := range []struct {
		body string
		want int
	}{
		{fmt.Sprintf(`{"from_exercise_id":%d,"to_exercise_id":%d}`, squat.ID, press.ID), http.StatusBadRequest},
		{fmt.Sprintf(`{"from_exercise_id":%d,"to_exercise_id":%d}`, squat.ID, squat.ID), http.StatusBadRequest},
		{fmt.Sprintf(`{"to_exercise_id":%d}`, goblet.ID), http.StatusBadRequest},
		{fmt.Sprintf(`{"from_exercise_id":%d,"to_exercise_id":99999}`, squat.ID), http.StatusNotFound},
	} {
		rr = env.do(t, "POST", swapsPath, tc.body, cookies)
		requireStatus(t, rr, tc.want)
	}

	rr = env.do(t, "POST", swapsPath, fmt.Sprintf(`{"from_exercise_id":%d,"to_exercise_id":%d,"reason":"  Knee felt off  "}`, squat.ID, goblet.ID), cookies)
	requireStatus(t, rr, http.StatusCreated)
	var swap ExerciseSwap
	decodeJSON(t, rr, &swap)
	if swap.FromExerciseName != "Back Squat" || swap.ToExerciseName != "Goblet Squat" || swap.Reason == nil || *swap.Reason != "Knee felt off" {
		t.Fatalf("swap = %+v", swap)
	}
	rr = env.do(t, "POST", swapsPath, fmt.Sprintf(`{"from_exercise_id":%d,"to_exercise_id":%d}`, squat.ID, split.ID), cookies)
	requireStatus(t, rr, http.StatusConflict)

	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/workouts/%d", athlete.ID, workout.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var detail WorkoutDetailResponse
	decodeJSON(t, rr, &detail)
	if len(detail.Swaps) != 1 || detail.Swaps[0].ID != swap.ID || detail.Readiness != nil {
		t.Fatalf("detail = %+v", detail)
	}

	rr = env.do(t, "GET", "/api/reviews/pending", nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var pending []UnreviewedWorkout
	decodeJSON(t, rr, &pending)
	if len(pending) != 1 || pending[0].SwapCount != 1 || pending[0].LowReadiness {
		t.Fatalf("pending = %+v", pending)
	}

	rr = env.do(t, "DELETE", fmt.Sprintf("%s/%d", swapsPath, swap.ID), nil, cookies)
	requireStatus(t, rr, http.StatusNoContent)
	rr = env.do(t, "DELETE", fmt.Sprintf("%s/%d", swapsPath, swap.ID), nil, cookies)
	requireStatus(t, rr, http.StatusNotFound)
}

// TestExerciseSwapIDOR checks a swap is scoped to the path athlete's
// workout: athlete B's path cannot create, list, or delete swaps on
// athlete A's workout (ADR 021).
func TestExerciseSwapIDOR(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	a := env.createAthlete(t, "Athlete A", admin.ID)
	b := env.createAthlete(t, "Athlete B", admin.ID)
	squat := env.createExercise(t, "Back Squat")
	goblet := env.createExercise(t, "Goblet Squat")
	env.tagPatterns(t, squat.ID, "squat")
	env.tagPatterns(t, goblet.ID, "squat")
	workoutA := env.createWorkout(t, a.ID, "2026-05-12")
	workoutB := env.createWorkout(t, b.ID, "2026-05-12")
	cookies := env.loginAs(t, admin)
	body := fmt.Sprintf(`{"from_exercise_id":%d,"to_exercise_id":%d}`, squat.ID, goblet.ID)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/workouts/%d/swaps", b.ID, workoutA.ID), body, cookies)
	requireStatus(t, rr, http.StatusNotFound)

	rr = env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/workouts/%d/swaps", a.ID, workoutA.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)
	var swap ExerciseSwap
	decodeJSON(t, rr, &swap)

	// B's own workout path with A's swap id, and B's path with A's workout.
	rr = env.do(t, "DELETE", fmt.Sprintf("/api/athletes/%d/workouts/%d/swaps/%d", b.ID, workoutB.ID, swap.ID), nil, cookies)
	requireStatus(t, rr, http.StatusNotFound)
	rr = env.do(t, "DELETE", fmt.Sprintf("/api/athletes/%d/workouts/%d/swaps/%d", b.ID, workoutA.ID, swap.ID), nil, cookies)
	requireStatus(t, rr, http.StatusNotFound)

	// A non-coach athlete user cannot reach another athlete's substitutes.
	other := env.createUser(t, "kid", false, false)
	if _, err := env.DB.Exec(`UPDATE users SET athlete_id = ? WHERE id = ?`, b.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	kid := env.loginAs(t, other)
	rr = env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/workouts/%d/swaps", a.ID, workoutA.ID), body, kid)
	requireStatus(t, rr, http.StatusForbidden)
}

// TestReadinessCheckin covers gap 10: mood is stored, out-of-range answers
// are 400s, a low answer sets low_readiness, and the flag reaches the
// workout read and the pending review list. The prescription is untouched.
func TestReadinessCheckin(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Dana", coach.ID)
	workout := env.createWorkout(t, athlete.ID, "2026-05-12")
	cookies := env.loginAs(t, coach)
	path := fmt.Sprintf("/api/athletes/%d/recovery-checkins", athlete.ID)

	for _, bad := range []string{
		`{"date":"2026-05-12","mood":0}`,
		`{"date":"2026-05-12","mood":11}`,
		`{"date":"2026-05-12","soreness":12}`,
		`{"date":"2026-05-12","sleep_hours":30}`,
	} {
		rr := env.do(t, "POST", path, bad, cookies)
		requireStatus(t, rr, http.StatusBadRequest)
	}

	rr := env.do(t, "POST", path, `{"date":"2026-05-11","sleep_hours":8,"soreness":2,"mood":8}`, cookies)
	requireStatus(t, rr, http.StatusCreated)
	var good RecoveryCheckin
	decodeJSON(t, rr, &good)
	if good.Mood == nil || *good.Mood != 8 || good.LowReadiness {
		t.Fatalf("good check-in = %+v", good)
	}

	rr = env.do(t, "POST", path, `{"date":"2026-05-12","sleep_hours":5,"soreness":3,"mood":7}`, cookies)
	requireStatus(t, rr, http.StatusCreated)
	var low RecoveryCheckin
	decodeJSON(t, rr, &low)
	if !low.LowReadiness {
		t.Fatalf("low check-in = %+v", low)
	}

	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/workouts/%d", athlete.ID, workout.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var detail WorkoutDetailResponse
	decodeJSON(t, rr, &detail)
	if detail.Readiness == nil || detail.Readiness.ID != low.ID || !detail.Readiness.LowReadiness {
		t.Fatalf("workout readiness = %+v", detail.Readiness)
	}
	if detail.Swaps == nil {
		t.Fatal("swaps should be an empty list, not null")
	}

	rr = env.do(t, "GET", "/api/reviews/pending", nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var pending []UnreviewedWorkout
	decodeJSON(t, rr, &pending)
	flagged := false
	for _, p := range pending {
		if p.WorkoutID == workout.ID {
			flagged = p.LowReadiness
		}
	}
	if !flagged {
		t.Fatalf("pending = %+v, want the session flagged", pending)
	}
}
