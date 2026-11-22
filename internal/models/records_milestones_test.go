package models

import (
	"context"
	"database/sql"
	"testing"
)

func logSet(t *testing.T, db *sql.DB, athleteID, exerciseID int64, date string, reps int, weight float64, setType string) int64 {
	t.Helper()
	ctx := context.Background()
	w, err := GetWorkoutByAthleteDate(ctx, db, athleteID, date)
	if err != nil {
		w, err = CreateWorkout(ctx, db, athleteID, date, "", 0)
		if err != nil {
			t.Fatalf("create workout %s: %v", date, err)
		}
	}
	if _, err := AddSetWithType(ctx, db, w.ID, exerciseID, reps, weight, 0, "reps", "", "", setType); err != nil {
		t.Fatalf("add set: %v", err)
	}
	return w.ID
}

func TestListAthleteRecords(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	a, squat := setDetailFixture(t, db)
	pushup, err := CreateExercise(ctx, db, "Push-up", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}

	logSet(t, db, a.ID, squat.ID, "2026-01-05", 5, 200, SetTypeWorking)
	logSet(t, db, a.ID, squat.ID, "2026-01-05", 3, 300, SetTypeWarmup) // warm-up: never a record
	logSet(t, db, a.ID, squat.ID, "2026-01-12", 6, 210, SetTypeWorking)
	logSet(t, db, a.ID, squat.ID, "2026-01-19", 1, 250, SetTypeWorking)
	logSet(t, db, a.ID, squat.ID, "2026-01-26", 6, 210, SetTypeWorking) // tie keeps the earlier date
	logSet(t, db, a.ID, pushup.ID, "2026-01-12", 20, 0, SetTypeWorking)

	recs, err := ListAthleteRecords(ctx, db, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 || recs[0].Name != "Back Squat" || recs[1].Name != "Push-up" {
		t.Fatalf("records = %+v", recs)
	}
	sq := recs[0]
	if sq.SessionCount != 4 || sq.LastDate != "2026-01-26" {
		t.Fatalf("squat sessions = %d last %s", sq.SessionCount, sq.LastDate)
	}
	if sq.Heaviest == nil || sq.Heaviest.Weight != 250 || sq.Heaviest.Date != "2026-01-19" {
		t.Fatalf("heaviest = %+v", sq.Heaviest)
	}
	// 210x6 Epley = 252, beats 250x1 = 250.
	if sq.BestE1RM == nil || sq.BestE1RM.Weight != 210 || sq.BestE1RM.Date != "2026-01-12" {
		t.Fatalf("best e1rm = %+v", sq.BestE1RM)
	}
	want := map[int]struct {
		weight float64
		date   string
	}{
		1: {250, "2026-01-19"},
		2: {210, "2026-01-12"},
		3: {210, "2026-01-12"},
		5: {210, "2026-01-12"},
	}
	if len(sq.RepMaxes) != len(want) {
		t.Fatalf("rep maxes = %+v", sq.RepMaxes)
	}
	for _, rm := range sq.RepMaxes {
		w, ok := want[rm.Target]
		if !ok || rm.Weight != w.weight || rm.Date != w.date {
			t.Errorf("rep max %d = %+v, want %+v", rm.Target, rm, w)
		}
	}
	pu := recs[1]
	if pu.Heaviest != nil || pu.BestE1RM != nil || len(pu.RepMaxes) != 0 {
		t.Fatalf("bodyweight records = %+v", pu)
	}
	if pu.MostReps == nil || pu.MostReps.Reps != 20 {
		t.Fatalf("most reps = %+v", pu.MostReps)
	}

	// Another athlete's sets never show up.
	other, err := CreateAthlete(ctx, db, "Other", "", "", "", "", "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatal(err)
	}
	logSet(t, db, other.ID, squat.ID, "2026-01-05", 1, 500, SetTypeWorking)
	recs, err = ListAthleteRecords(ctx, db, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recs[0].Heaviest.Weight != 250 {
		t.Fatalf("records leaked another athlete: %+v", recs[0].Heaviest)
	}
}

func findMilestone(list []Milestone, key string) *Milestone {
	for i := range list {
		if list[i].Key == key {
			return &list[i]
		}
	}
	return nil
}

func TestAthleteMilestones(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	a, squat := setDetailFixture(t, db)

	// Empty history: no achievements, first goals are the lowest targets.
	m, err := AthleteMilestonesFor(ctx, db, a.ID, "2026-02-04")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Achieved) != 0 || m.TotalSessions != 0 || len(m.Next) != 3 || m.Next[0].Target != 5 {
		t.Fatalf("empty milestones = %+v", m)
	}

	// Five sessions over three consecutive weeks (Mon 2026-01-12 .. 2026-01-28),
	// three of them in the first week, with a PR in week three.
	logSet(t, db, a.ID, squat.ID, "2026-01-12", 5, 100, SetTypeWorking)
	logSet(t, db, a.ID, squat.ID, "2026-01-14", 5, 100, SetTypeWorking)
	logSet(t, db, a.ID, squat.ID, "2026-01-16", 5, 95, SetTypeWorking)
	logSet(t, db, a.ID, squat.ID, "2026-01-21", 5, 100, SetTypeWorking)
	wid := logSet(t, db, a.ID, squat.ID, "2026-01-28", 5, 110, SetTypeWorking)
	// A warm-up-only session does not count.
	logSet(t, db, a.ID, squat.ID, "2026-01-30", 5, 45, SetTypeWarmup)

	coach, err := CreateUser(ctx, db, "coach", "Coach", "password123", "", true, false, sql.NullInt64{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateWorkoutReview(ctx, db, wid, coach.ID, "approved", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordTierChange(ctx, db, a.ID, "foundational", "", coach.ID, "2026-01-01", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordTierChange(ctx, db, a.ID, "intermediate", "foundational", coach.ID, "2026-01-29", ""); err != nil {
		t.Fatal(err)
	}

	// Today is Wed 2026-02-04: no session yet this week, so last week's
	// streak still counts (grace), and the streak is 3.
	m, err = AthleteMilestonesFor(ctx, db, a.ID, "2026-02-04")
	if err != nil {
		t.Fatal(err)
	}
	if m.TotalSessions != 5 || m.CurrentStreak != 3 || m.BestStreak != 3 || m.PRCount != 1 {
		t.Fatalf("summary = %+v", m)
	}
	checks := map[string]string{
		"first_session":         "2026-01-12",
		"first_full_week":       "2026-01-16",
		"sessions_5":            "2026-01-28",
		"streak_2":              "2026-01-21",
		"first_pr":              "2026-01-28",
		"first_approved_review": "2026-01-28",
		"tier_foundational":     "2026-01-01",
		"tier_intermediate":     "2026-01-29",
	}
	for key, date := range checks {
		got := findMilestone(m.Achieved, key)
		if got == nil || got.AchievedOn != date {
			t.Errorf("milestone %s = %+v, want %s", key, got, date)
		}
	}
	if findMilestone(m.Achieved, "streak_4") != nil || findMilestone(m.Achieved, "sessions_10") != nil {
		t.Fatalf("unearned milestone present: %+v", m.Achieved)
	}
	// Newest first.
	for i := 1; i < len(m.Achieved); i++ {
		if m.Achieved[i-1].AchievedOn < m.Achieved[i].AchievedOn {
			t.Fatalf("not newest first: %+v", m.Achieved)
		}
	}
	next := map[string]MilestoneGoal{}
	for _, g := range m.Next {
		next[g.Key] = g
	}
	if g := next["sessions_10"]; g.Current != 5 || g.Target != 10 {
		t.Errorf("next sessions = %+v", g)
	}
	if g := next["streak_4"]; g.Current != 3 || g.Target != 4 {
		t.Errorf("next streak = %+v", g)
	}
	if g := next["prs_5"]; g.Current != 1 {
		t.Errorf("next prs = %+v", g)
	}

	// Two weeks without training breaks the current streak, not the best.
	m, err = AthleteMilestonesFor(ctx, db, a.ID, "2026-02-18")
	if err != nil {
		t.Fatal(err)
	}
	if m.CurrentStreak != 0 || m.BestStreak != 3 {
		t.Fatalf("streaks after a break = %d / %d", m.CurrentStreak, m.BestStreak)
	}
}

func TestMondayIndex(t *testing.T) {
	mon, _ := mondayIndex("2026-01-12")
	sun, _ := mondayIndex("2026-01-18")
	next, _ := mondayIndex("2026-01-19")
	if mon != sun || next != mon+1 {
		t.Fatalf("mondayIndex: mon=%d sun=%d next=%d", mon, sun, next)
	}
	if _, ok := mondayIndex("nope"); ok {
		t.Fatal("bad date parsed")
	}
}
