package api

import (
	"fmt"
	"net/http"
	"testing"
)

// --- IDOR coverage (issue #5) ---

func TestListMissingTMs_OtherCoachForbidden(t *testing.T) {
	env := setupTest(t)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	athleteOfA := env.createAthlete(t, "Alice", coachA.ID)
	cookies := env.loginAs(t, coachB)

	rr := env.do(t, "GET",
		fmt.Sprintf("/api/athletes/%d/missing-tms?template_id=1", athleteOfA.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestBatchSetTMs_OtherCoachForbidden(t *testing.T) {
	env := setupTest(t)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	athleteOfA := env.createAthlete(t, "Alice", coachA.ID)
	cookies := env.loginAs(t, coachB)

	body := `{"maxes":[{"exercise_id":1,"weight":135}]}`
	rr := env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/batch-tms", athleteOfA.ID),
		body, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}
