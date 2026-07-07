package api

import (
	"fmt"
	"net/http"
	"testing"
)

// TestCrossAthleteIDOR asserts the cross-athlete IDOR guard (app review §1):
// a child resource (workout) that belongs to athlete A must not be reachable,
// deletable, or mutable through athlete B's path segment — even for an admin
// who is authorized for BOTH athletes. The path athlete is the authorization
// boundary; a global child ID under the wrong athlete must 404, not operate.
func TestCrossAthleteIDOR(t *testing.T) {
	env := setupTest(t)
	// Admin passes CanAccessAthlete for every athlete, so a leak here would be
	// the ownership check (workout→athlete), not the access check.
	admin := env.createUser(t, "admin", true, true)
	athleteA := env.createAthlete(t, "Athlete A", admin.ID)
	athleteB := env.createAthlete(t, "Athlete B", admin.ID)
	workoutA := env.createWorkout(t, athleteA.ID, "2026-05-12")
	cookies := env.loginAs(t, admin)

	// GET A's workout via B's path → 404.
	rr := env.do(t, "GET",
		fmt.Sprintf("/api/athletes/%d/workouts/%d", athleteB.ID, workoutA.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusNotFound)

	// Update A's workout notes via B's path → 404.
	rr = env.do(t, "PUT",
		fmt.Sprintf("/api/athletes/%d/workouts/%d/notes", athleteB.ID, workoutA.ID),
		`{"notes":"tampered"}`, cookies)
	requireStatus(t, rr, http.StatusNotFound)

	// Delete A's workout via B's path → 404.
	rr = env.do(t, "DELETE",
		fmt.Sprintf("/api/athletes/%d/workouts/%d", athleteB.ID, workoutA.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusNotFound)

	// The workout must still be intact under its real owner (A).
	rr = env.do(t, "GET",
		fmt.Sprintf("/api/athletes/%d/workouts/%d", athleteA.ID, workoutA.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusOK)
}

// TestCreateWorkout_RejectsInvalidDate covers date-validation §7: a malformed
// date must 400 at the boundary (SQLite would otherwise store the garbage).
func TestCreateWorkout_RejectsInvalidDate(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Datey", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/workouts", athlete.ID),
		`{"date":"2026-13-99"}`, cookies)
	requireStatus(t, rr, http.StatusBadRequest)

	// A well-formed date still succeeds.
	rr = env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/workouts", athlete.ID),
		`{"date":"2026-05-12"}`, cookies)
	requireStatus(t, rr, http.StatusCreated)
}
