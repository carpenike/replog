package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

// seedComplianceWeek gives athlete a Mon/Wed/Fri loop program from Nov 2
// 2026 and logs the week of Nov 9: Monday done, Wednesday missed, Thursday
// an unplanned extra session, Friday partial (bench only, squat planned).
func (e *testEnv) seedComplianceWeek(t *testing.T, athlete *models.Athlete) {
	t.Helper()
	ctx := context.Background()
	squat := e.createExercise(t, "Back Squat "+athlete.Name)
	bench := e.createExercise(t, "Bench Press "+athlete.Name)
	tpl, err := models.CreateProgramTemplate(ctx, e.DB, nil, "Base "+athlete.Name, "", 1, 3, true, "")
	if err != nil {
		t.Fatal(err)
	}
	reps, load := 5, 135.0
	for day := 1; day <= 3; day++ {
		if _, err := models.CreatePrescribedSet(ctx, e.DB, tpl.ID, squat.ID, 1, day, 1, &reps, nil, &load, 0, "reps", ""); err != nil {
			t.Fatal(err)
		}
	}
	program, err := models.AssignProgram(ctx, e.DB, athlete.ID, tpl.ID, "2026-11-02", "", "", "primary", "[1,3,5]")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []struct {
		date string
		ex   *models.Exercise
	}{{"2026-11-09", squat}, {"2026-11-12", squat}, {"2026-11-13", bench}} {
		w, err := models.CreateWorkout(ctx, e.DB, athlete.ID, s.date, "", program.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := models.AddSet(ctx, e.DB, w.ID, s.ex.ID, 5, 135, 0, "reps", "main", ""); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCoachCompliance(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	otherCoach := env.createUser(t, "other", true, false)
	admin := env.createUser(t, "admin", true, true)
	kid := env.createUser(t, "kid", false, false)
	dana := env.createAthlete(t, "Dana", coach.ID)
	sam := env.createAthlete(t, "Sam", coach.ID)
	env.createAthlete(t, "Theirs", otherCoach.ID)
	env.seedComplianceWeek(t, dana)

	path := "/api/coach/compliance?week_start=2026-11-09"
	rr := env.do(t, "GET", path, nil, env.loginAs(t, coach))
	requireStatus(t, rr, http.StatusOK)
	var grid ComplianceResponse
	decodeJSON(t, rr, &grid)
	if grid.WeekStart != "2026-11-09" || grid.WeekEnd != "2026-11-15" || len(grid.Athletes) != 2 {
		t.Fatalf("grid = %+v", grid)
	}
	var danaRow, samRow ComplianceAthleteResponse
	for _, a := range grid.Athletes {
		switch a.AthleteID {
		case dana.ID:
			danaRow = a
		case sam.ID:
			samRow = a
		default:
			t.Fatalf("another coach's athlete in the grid: %+v", a)
		}
	}
	want := []string{"done", "rest", "skipped", "extra", "partial", "rest", "rest"}
	if len(danaRow.Days) != 7 {
		t.Fatalf("days = %d", len(danaRow.Days))
	}
	for i, st := range want {
		if got := danaRow.Days[i].Status; got != st {
			t.Errorf("day %d (%s) = %s, want %s", i+1, danaRow.Days[i].Date, got, st)
		}
	}
	if danaRow.Summary != "1 of 3" || danaRow.ExtraCount != 1 || danaRow.SkippedCount != 1 || danaRow.PartialCount != 1 {
		t.Fatalf("dana counts = %+v", danaRow)
	}
	if danaRow.Days[0].SessionID == nil || danaRow.Days[3].SessionID == nil || danaRow.Days[2].SessionID != nil {
		t.Fatalf("session ids = %+v", danaRow.Days)
	}
	for _, d := range samRow.Days {
		if d.Status != "rest" {
			t.Fatalf("sam has no program, got %+v", d)
		}
	}

	// Admins see every athlete.
	rr = env.do(t, "GET", path, nil, env.loginAs(t, admin))
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &grid)
	if len(grid.Athletes) != 3 {
		t.Fatalf("admin grid athletes = %d", len(grid.Athletes))
	}

	requireStatus(t, env.do(t, "GET", path, nil, env.loginAs(t, kid)), http.StatusForbidden)
	requireStatus(t, env.do(t, "GET", "/api/coach/compliance?week_start=2026-11-10", nil, env.loginAs(t, coach)), http.StatusBadRequest)
	requireStatus(t, env.do(t, "GET", "/api/coach/compliance", nil, env.loginAs(t, coach)), http.StatusOK)

	// A coach service token sees only that coach's athletes.
	rr = env.serveBearer(t, "GET", path, env.coachServiceToken(t, coach.ID), nil, nil)
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &grid)
	if len(grid.Athletes) != 2 {
		t.Fatalf("service grid athletes = %d", len(grid.Athletes))
	}
}

func TestAthleteCalendar(t *testing.T) {
	env := setupTest(t)
	ctx := context.Background()
	coach := env.createUser(t, "coach", true, false)
	dana := env.createAthlete(t, "Dana", coach.ID)
	other := env.createAthlete(t, "Other", coach.ID)
	env.seedComplianceWeek(t, dana)
	count := int64(40)
	if _, err := models.CreateThrowingSession(ctx, env.DB, dana.ID, models.ThrowingSessionInput{Date: "2026-11-10", ThrowType: "bullpen", ThrowCount: &count}); err != nil {
		t.Fatal(err)
	}
	if _, err := models.CreateSeasonPhase(ctx, env.DB, dana.ID, models.SeasonPhaseInput{Sport: "baseball", Phase: "pre", StartDate: "2026-10-15", EndDate: "2026-11-20"}); err != nil {
		t.Fatal(err)
	}
	if _, err := models.CreateSeasonPhase(ctx, env.DB, dana.ID, models.SeasonPhaseInput{Sport: "baseball", Phase: "off", StartDate: "2026-01-01", EndDate: "2026-03-01"}); err != nil {
		t.Fatal(err)
	}

	// Dana's own account can read her calendar.
	kid := env.createUser(t, "dana", false, false)
	if _, err := models.UpdateUser(ctx, env.DB, kid.ID, kid.Username, "", "", sql.NullInt64{Int64: dana.ID, Valid: true}, false, false); err != nil {
		t.Fatal(err)
	}
	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/calendar?month=2026-11", dana.ID), nil, env.loginAs(t, kid))
	requireStatus(t, rr, http.StatusOK)
	var cal CalendarResponse
	decodeJSON(t, rr, &cal)
	if cal.Month != "2026-11" || cal.Start != "2026-11-01" || cal.End != "2026-11-30" || len(cal.Days) != 30 {
		t.Fatalf("calendar range = %s %s..%s (%d days)", cal.Month, cal.Start, cal.End, len(cal.Days))
	}
	day := func(date string) CalendarDayResponse {
		for _, d := range cal.Days {
			if d.Date == date {
				return d
			}
		}
		t.Fatalf("no day %s", date)
		return CalendarDayResponse{}
	}
	if d := day("2026-11-09"); !d.Scheduled || d.Status == nil || *d.Status != "done" || len(d.Sessions) != 1 || d.Sessions[0].Discipline != "resistance" || d.Sessions[0].SetCount != 1 {
		t.Fatalf("Nov 9 = %+v", d)
	}
	if d := day("2026-11-11"); !d.Scheduled || *d.Status != "missed" || len(d.Sessions) != 0 {
		t.Fatalf("Nov 11 = %+v", d)
	}
	if d := day("2026-11-10"); d.Scheduled || d.Status != nil || len(d.Sessions) != 1 || d.Sessions[0].Discipline != "throwing" || d.Sessions[0].Label == nil || *d.Sessions[0].Label != "bullpen" {
		t.Fatalf("Nov 10 = %+v", d)
	}
	if d := day("2026-11-01"); d.Scheduled {
		t.Fatalf("Nov 1 is a Sunday, not a scheduled day: %+v", d)
	}
	if len(cal.Phases) != 1 || cal.Phases[0].Phase != "pre" || cal.Phases[0].EndDate == nil || *cal.Phases[0].EndDate != "2026-11-20" {
		t.Fatalf("phases = %+v", cal.Phases)
	}

	// Not her calendar, and bad input.
	requireStatus(t, env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/calendar?month=2026-11", other.ID), nil, env.loginAs(t, kid)), http.StatusForbidden)
	requireStatus(t, env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/calendar?month=2026-13", dana.ID), nil, env.loginAs(t, coach)), http.StatusBadRequest)
	requireStatus(t, env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/calendar", dana.ID), nil, env.loginAs(t, coach)), http.StatusOK)
}
