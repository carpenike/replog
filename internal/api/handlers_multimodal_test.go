package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/carpenike/replog/internal/models"
)

// createAthleteWithDOB creates an athlete carrying a date of birth, needed for
// Pitch Smart age-bracket resolution.
func (e *testEnv) createAthleteWithDOB(t *testing.T, name, dob string, coachID int64) *models.Athlete {
	t.Helper()
	athlete, err := models.CreateAthlete(context.Background(), e.DB, name, "", "", "", dob, "", "",
		sql.NullInt64{Int64: coachID, Valid: coachID != 0}, false)
	if err != nil {
		t.Fatalf("create athlete %q: %v", name, err)
	}
	return athlete
}

// TestMultiModal_SameDayLiftAndThrow is the headline ADR-018 acceptance test:
// an athlete can log a resistance workout AND a throwing session on the same
// date. The widened UNIQUE(athlete_id, date, discipline) must allow both.
func TestMultiModal_SameDayLiftAndThrow(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	const date = "2026-05-12"

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/workouts", athlete.ID),
		fmt.Sprintf(`{"date":%q}`, date), cookies)
	requireStatus(t, rr, http.StatusCreated)
	var lift Workout
	decodeJSON(t, rr, &lift)
	if lift.Discipline != "resistance" {
		t.Errorf("lift discipline=%q, want resistance", lift.Discipline)
	}

	rr = env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/throwing-sessions", athlete.ID),
		fmt.Sprintf(`{"date":%q,"throw_type":"bullpen","throw_count":30}`, date), cookies)
	requireStatus(t, rr, http.StatusCreated)
	var throw ThrowingSession
	decodeJSON(t, rr, &throw)
	if throw.Date != date {
		t.Errorf("throw date=%q, want %q", throw.Date, date)
	}
	if throw.WorkoutID == lift.ID {
		t.Error("throwing session must have its own parent workout, not the lift's")
	}
}

// TestMultiModal_JournalRendersBoth verifies the journal shows a resistance
// workout and a throwing session as distinct, correctly-typed entries — and
// that the resistance discipline filter keeps a throwing parent from rendering
// as a bogus "Workout (0 sets)" row.
func TestMultiModal_JournalRendersBoth(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	exercise := env.createExercise(t, "Bench Press")
	cookies := env.loginAs(t, coach)

	const date = "2026-05-12"
	// Resistance workout with one set.
	lift := env.createWorkout(t, athlete.ID, date)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/workouts/%d/sets", athlete.ID, lift.ID),
		fmt.Sprintf(`{"exercise_id":%d,"reps":5,"weight":135}`, exercise.ID), cookies)
	requireStatus(t, rr, http.StatusCreated)

	// Throwing session same day.
	rr = env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/throwing-sessions", athlete.ID),
		fmt.Sprintf(`{"date":%q,"throw_type":"long_toss","throw_count":40,"velocity":68}`, date), cookies)
	requireStatus(t, rr, http.StatusCreated)

	entries, err := models.ListJournalEntries(context.Background(), env.DB, athlete.ID, true, 100)
	if err != nil {
		t.Fatalf("list journal: %v", err)
	}

	var workoutEntries, throwingEntries int
	for _, e := range entries {
		switch e.Type {
		case "workout":
			workoutEntries++
			if e.Summary == "Workout (0 sets)" {
				t.Errorf("resistance workout entry shows 0 sets; expected the logged set: %q", e.Summary)
			}
		case "throwing":
			throwingEntries++
		}
	}
	if workoutEntries != 1 {
		t.Errorf("got %d workout journal entries, want 1", workoutEntries)
	}
	if throwingEntries != 1 {
		t.Errorf("got %d throwing journal entries, want 1", throwingEntries)
	}
}

// TestMultiModal_OverLimitThrowStillLogs is the safety-line test: a throwing
// session that exceeds the Pitch Smart daily max MUST still be logged (201).
// Pitch Smart is advisory (ADR 007) — a flag, never a gate.
func TestMultiModal_OverLimitThrowStillLogs(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	// Age ~12 → Pitch Smart daily max 85.
	dob := time.Now().AddDate(-12, 0, 0).Format("2006-01-02")
	athlete := env.createAthleteWithDOB(t, "Charlie", dob, coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/throwing-sessions", athlete.ID),
		`{"date":"2026-05-12","throw_type":"game","throw_count":120}`, cookies)
	requireStatus(t, rr, http.StatusCreated) // over-limit, still logged

	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/pitch-smart", athlete.ID), "", cookies)
	requireStatus(t, rr, http.StatusOK)
	var status PitchSmartStatus
	decodeJSON(t, rr, &status)
	if status.DailyMax != 85 {
		t.Errorf("daily max=%d, want 85 for age 12", status.DailyMax)
	}
	if !status.OverDailyMax {
		t.Error("expected over_daily_max=true for a 120-pitch session against an 85 cap")
	}
	if status.Advisory == "" {
		t.Error("expected a non-empty advisory string")
	}
}

// TestCreateSeasonPhase_CoachAllowed pins the happy path: a coach can record a
// season phase for their athlete (the coach-only gate must not break it).
func TestCreateSeasonPhase_CoachAllowed(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/season-phases", athlete.ID),
		`{"phase":"in","start_date":"2026-05-01"}`, cookies)
	requireStatus(t, rr, http.StatusCreated)
}

// TestCreateSeasonPhase_NonCoachForbidden verifies the coach-only gate: a
// non-coach user (even with athlete access) cannot create a season phase.
func TestCreateSeasonPhase_NonCoachForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	user := env.createUser(t, "athlete_user", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/season-phases", athlete.ID),
		`{"phase":"in","start_date":"2026-05-01"}`, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

// TestDeleteSeasonPhase_NonCoachForbidden verifies the coach-only gate on the
// delete path: a non-coach user cannot delete a season phase.
func TestDeleteSeasonPhase_NonCoachForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	coachCookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/season-phases", athlete.ID),
		`{"phase":"in","start_date":"2026-05-01"}`, coachCookies)
	requireStatus(t, rr, http.StatusCreated)
	var phase SeasonPhase
	decodeJSON(t, rr, &phase)

	user := env.createUser(t, "athlete_user", false, false)
	userCookies := env.loginAs(t, user)
	rr = env.do(t, "DELETE", fmt.Sprintf("/api/athletes/%d/season-phases/%d", athlete.ID, phase.ID),
		"", userCookies)
	requireStatus(t, rr, http.StatusForbidden)
}
