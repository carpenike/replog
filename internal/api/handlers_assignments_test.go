package api

import (
	"fmt"
	"net/http"
	"testing"
)

func TestListAssignments_OwnCoach(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/assignments", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got []*AthleteExercise
	decodeJSON(t, rr, &got)
	if len(got) != 0 {
		t.Errorf("expected empty assignments list, got %d", len(got))
	}
}

func TestListAssignments_OtherCoachForbidden(t *testing.T) {
	env := setupTest(t)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	athleteOfA := env.createAthlete(t, "Alice", coachA.ID)
	cookies := env.loginAs(t, coachB)

	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/assignments", athleteOfA.ID), nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestAssignExercise_Success(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	exercise := env.createExercise(t, "Bench Press")
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(`{"exercise_id":%d,"target_reps":5}`, exercise.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/assignments", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)

	// List should now include the new assignment.
	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/assignments", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var got []*AthleteExercise
	decodeJSON(t, rr, &got)
	if len(got) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(got))
	}
	if got[0].ExerciseID != exercise.ID {
		t.Errorf("got exercise_id=%d, want %d", got[0].ExerciseID, exercise.ID)
	}
}

func TestAssignExercise_RequiresExerciseID(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/assignments", athlete.ID),
		`{"target_reps":5}`, cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestAssignExercise_NonCoachForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	exercise := env.createExercise(t, "Squat")
	user := env.createUser(t, "athlete_user", false, false)
	cookies := env.loginAs(t, user)

	body := fmt.Sprintf(`{"exercise_id":%d}`, exercise.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/assignments", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestAssignExercise_DuplicateActiveRejected(t *testing.T) {
	// Schema enforces UNIQUE INDEX (athlete_id, exercise_id) WHERE active = 1.
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	exercise := env.createExercise(t, "Bench Press")
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(`{"exercise_id":%d,"target_reps":5}`, exercise.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/assignments", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)

	// Second create with the same exercise should fail (unique index on active=1).
	rr = env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/assignments", athlete.ID), body, cookies)
	if rr.Code == http.StatusCreated {
		t.Errorf("expected duplicate active assignment to be rejected, got 201")
	}
}

func TestDeactivateAssignment_Success(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	exercise := env.createExercise(t, "Bench Press")
	cookies := env.loginAs(t, coach)

	// Create.
	body := fmt.Sprintf(`{"exercise_id":%d,"target_reps":5}`, exercise.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/assignments", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)
	var assignment AthleteExercise
	decodeJSON(t, rr, &assignment)

	// Deactivate.
	rr = env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/assignments/%d/deactivate", athlete.ID, assignment.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	// Now we should be able to recreate (no longer "active").
	rr = env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/assignments", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)
}

func TestDeactivateAssignment_NonCoachForbidden(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "athlete_user", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "POST", "/api/athletes/1/assignments/1/deactivate", nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestReactivateAssignment_Success(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	exercise := env.createExercise(t, "Bench Press")
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(`{"exercise_id":%d,"target_reps":5}`, exercise.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/assignments", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)
	var assignment AthleteExercise
	decodeJSON(t, rr, &assignment)

	rr = env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/assignments/%d/deactivate", athlete.ID, assignment.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	// Reactivate via the dedicated endpoint (creates a fresh active row).
	rr = env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/assignments/reactivate", athlete.ID),
		body, cookies)
	requireStatus(t, rr, http.StatusOK)

	// Listing should show one active assignment again.
	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/assignments", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var got []*AthleteExercise
	decodeJSON(t, rr, &got)
	activeCount := 0
	for _, a := range got {
		if a.Active {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Errorf("expected exactly 1 active assignment after reactivate, got %d", activeCount)
	}
}

// --- IDOR coverage (issue #5) ---

func TestAssignExercise_OtherCoachForbidden(t *testing.T) {
	env := setupTest(t)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	athleteOfA := env.createAthlete(t, "Alice", coachA.ID)
	exercise := env.createExercise(t, "Bench Press")
	cookies := env.loginAs(t, coachB)

	body := fmt.Sprintf(`{"exercise_id":%d,"target_reps":5}`, exercise.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/assignments", athleteOfA.ID), body, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestReactivateAssignment_OtherCoachForbidden(t *testing.T) {
	env := setupTest(t)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	athleteOfA := env.createAthlete(t, "Alice", coachA.ID)
	exercise := env.createExercise(t, "Bench Press")
	cookies := env.loginAs(t, coachB)

	body := fmt.Sprintf(`{"exercise_id":%d,"target_reps":5}`, exercise.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/assignments/reactivate", athleteOfA.ID), body, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}
