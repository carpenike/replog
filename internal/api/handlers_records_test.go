package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func (e *testEnv) logTestSet(t *testing.T, athleteID, exerciseID int64, date string, reps int, weight float64, setType string) {
	t.Helper()
	ctx := context.Background()
	w, err := models.GetWorkoutByAthleteDate(ctx, e.DB, athleteID, date)
	if err != nil {
		w = e.createWorkout(t, athleteID, date)
	}
	if _, err := models.AddSetWithType(ctx, e.DB, w.ID, exerciseID, reps, weight, 0, "reps", "", "", setType); err != nil {
		t.Fatalf("add set: %v", err)
	}
}

// TestRecordsAndMilestones covers gap 6 and gap 7: the records board and
// milestones read only the athlete's own history, skip warm-ups, and an
// athlete user cannot read another athlete's.
func TestRecordsAndMilestones(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	kidAthlete := env.createAthleteWithTier(t, "Sam", "foundational", coach.ID)
	sibling := env.createAthlete(t, "Alex", coach.ID)
	squat := env.createExercise(t, "Back Squat")

	env.logTestSet(t, kidAthlete.ID, squat.ID, "2026-11-02", 5, 100, models.SetTypeWorking)
	env.logTestSet(t, kidAthlete.ID, squat.ID, "2026-11-02", 5, 45, models.SetTypeWarmup)
	env.logTestSet(t, kidAthlete.ID, squat.ID, "2026-11-09", 3, 115, models.SetTypeWorking)
	env.logTestSet(t, kidAthlete.ID, squat.ID, "2026-11-10", 8, 60, models.SetTypeWarmup) // warm-up only session
	env.logTestSet(t, sibling.ID, squat.ID, "2026-11-02", 1, 300, models.SetTypeWorking)

	kid := env.createUser(t, "sam", false, false)
	if _, err := env.DB.Exec(`UPDATE users SET athlete_id = ? WHERE id = ?`, kidAthlete.ID, kid.ID); err != nil {
		t.Fatal(err)
	}
	kidCookies := env.loginAs(t, kid)

	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/records", kidAthlete.ID), nil, kidCookies)
	requireStatus(t, rr, http.StatusOK)
	var recs RecordsResponse
	decodeJSON(t, rr, &recs)
	if len(recs.Exercises) != 1 || recs.E1RMSource != "epley" || len(recs.RepTargets) == 0 {
		t.Fatalf("records = %+v", recs)
	}
	sq := recs.Exercises[0]
	if sq.SessionCount != 2 || sq.Heaviest == nil || *sq.Heaviest.Weight != 115 || sq.Heaviest.Date != "2026-11-09" {
		t.Fatalf("squat records = %+v heaviest=%+v", sq, sq.Heaviest)
	}
	byTarget := map[int]RepMaxResponse{}
	for _, rm := range sq.RepMaxes {
		byTarget[rm.Target] = rm
	}
	if byTarget[3].Weight == nil || *byTarget[3].Weight != 115 || byTarget[5].Weight == nil || *byTarget[5].Weight != 100 {
		t.Fatalf("rep maxes = %+v", sq.RepMaxes)
	}
	if _, ok := byTarget[8]; ok {
		t.Fatal("warm-up set produced an 8RM")
	}

	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/milestones", kidAthlete.ID), nil, kidCookies)
	requireStatus(t, rr, http.StatusOK)
	var ms MilestonesResponse
	decodeJSON(t, rr, &ms)
	if ms.TotalSessions != 2 || ms.PRCount != 1 || ms.BestStreak != 2 {
		t.Fatalf("milestones summary = %+v", ms)
	}
	keys := map[string]MilestoneResponse{}
	for _, m := range ms.Achieved {
		keys[m.Key] = m
	}
	for _, key := range []string{"first_session", "first_pr", "streak_2", "tier_foundational"} {
		if _, ok := keys[key]; !ok {
			t.Errorf("missing milestone %s in %+v", key, ms.Achieved)
		}
	}
	if keys["tier_foundational"].Kind != models.MilestoneKindTier {
		t.Errorf("tier milestone kind = %q", keys["tier_foundational"].Kind)
	}
	if len(ms.Next) == 0 {
		t.Fatal("expected next goals")
	}

	// The kid cannot read a sibling's records or milestones.
	for _, path := range []string{"records", "milestones"} {
		rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/%s", sibling.ID, path), nil, kidCookies)
		requireStatus(t, rr, http.StatusForbidden)
	}

	// The coach can read both.
	coachCookies := env.loginAs(t, coach)
	for _, path := range []string{"records", "milestones"} {
		rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/%s", sibling.ID, path), nil, coachCookies)
		requireStatus(t, rr, http.StatusOK)
	}
}
