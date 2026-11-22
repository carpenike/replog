package scheduler

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/carpenike/replog/internal/models"
	"github.com/carpenike/replog/internal/notify"
)

// captureDigests swaps the sender for one that records requests.
func captureDigests(t *testing.T) *[]notify.Request {
	t.Helper()
	var got []notify.Request
	prev := digestSender
	digestSender = func(_ context.Context, _ *sql.DB, req notify.Request) { got = append(got, req) }
	t.Cleanup(func() { digestSender = prev })
	return &got
}

// seedDigestWeek builds a coach with one athlete whose Mon/Wed/Fri week of
// Nov 9 2026 has one done day, one missed day, one partial day, a squat PR,
// and a low-readiness check-in.
func seedDigestWeek(t *testing.T, db *sql.DB) (*models.User, *models.Athlete) {
	t.Helper()
	ctx := context.Background()
	coach, err := models.CreateUser(ctx, db, "coach", "Coach", "", "", true, false, sql.NullInt64{})
	if err != nil {
		t.Fatal(err)
	}
	athlete, err := models.CreateAthlete(ctx, db, "Dana", "", "", "", "", "", "", sql.NullInt64{Int64: coach.ID, Valid: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	squat, err := models.CreateExercise(ctx, db, "Back Squat", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	bench, err := models.CreateExercise(ctx, db, "Bench Press", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := models.CreateProgramTemplate(ctx, db, nil, "Base", "", 1, 3, true, "")
	if err != nil {
		t.Fatal(err)
	}
	reps, load := 5, 200.0
	for day := 1; day <= 3; day++ {
		if _, err := models.CreatePrescribedSet(ctx, db, tpl.ID, squat.ID, 1, day, 1, &reps, nil, &load, 0, "reps", ""); err != nil {
			t.Fatal(err)
		}
	}
	program, err := models.AssignProgram(ctx, db, athlete.ID, tpl.ID, "2026-11-02", "", "", "primary", "[1,3,5]")
	if err != nil {
		t.Fatal(err)
	}
	logSet := func(date string, ex *models.Exercise, weight float64) {
		w, err := models.GetWorkoutByAthleteDate(ctx, db, athlete.ID, date)
		if err != nil {
			w, err = models.CreateWorkout(ctx, db, athlete.ID, date, "", program.ID)
			if err != nil {
				t.Fatal(err)
			}
		}
		if _, err := models.AddSet(ctx, db, w.ID, ex.ID, 5, weight, 0, "reps", "main", ""); err != nil {
			t.Fatal(err)
		}
	}
	logSet("2026-11-04", squat, 200) // the week before: history to beat
	logSet("2026-11-09", squat, 225) // Monday: done, and a squat PR
	logSet("2026-11-13", bench, 100) // Friday: squat not logged, partial
	sleep := 4.5
	if _, err := models.CreateRecoveryCheckin(ctx, db, athlete.ID, models.RecoveryCheckinInput{Date: "2026-11-10", SleepHours: &sleep}); err != nil {
		t.Fatal(err)
	}
	return coach, athlete
}

func TestRunWeeklyDigests(t *testing.T) {
	db := testDB(t)
	got := captureDigests(t)
	coach, _ := seedDigestWeek(t, db)
	s := New(db)
	ny, _ := time.LoadLocation("America/New_York")

	// Monday before 07:00 local: not due yet.
	if n := s.RunWeeklyDigests(time.Date(2026, 11, 16, 6, 0, 0, 0, ny)); n != 0 {
		t.Fatalf("sent %d before 07:00", n)
	}
	if n := s.RunWeeklyDigests(time.Date(2026, 11, 16, 7, 30, 0, 0, ny)); n != 1 {
		t.Fatalf("sent %d, want 1", n)
	}
	// The same week never goes out twice.
	if n := s.RunWeeklyDigests(time.Date(2026, 11, 17, 9, 0, 0, 0, ny)); n != 0 {
		t.Fatalf("resent %d", n)
	}
	if len(*got) != 1 {
		t.Fatalf("requests = %d", len(*got))
	}
	req := (*got)[0]
	if req.UserID != coach.ID || req.Type != models.NotifyWeeklyDigest {
		t.Fatalf("request = %+v", req)
	}
	if req.Title != "Weekly digest: Nov 9–15" || req.Link != "/coach/compliance?week=2026-11-09" {
		t.Fatalf("title %q link %q", req.Title, req.Link)
	}
	for _, want := range []string{
		"Dana: 1 of 3 planned sessions done (1 partial, 1 skipped).",
		"New PRs: Back Squat 225 lb × 5 (e1RM 263 lb)",
		"Low readiness check-in: Tue Nov 10",
		"RepLog does not change any plan",
	} {
		if !strings.Contains(req.Message, want) {
			t.Errorf("message missing %q:\n%s", want, req.Message)
		}
	}
	if strings.Contains(req.Message, "Bench Press") {
		t.Errorf("a first-ever bench session is not a PR:\n%s", req.Message)
	}
}

func TestRunWeeklyDigests_RespectsPreference(t *testing.T) {
	db := testDB(t)
	got := captureDigests(t)
	coach, _ := seedDigestWeek(t, db)
	if err := models.SetNotificationPreference(context.Background(), db, coach.ID, models.NotifyWeeklyDigest, false, false); err != nil {
		t.Fatal(err)
	}
	ny, _ := time.LoadLocation("America/New_York")
	if n := New(db).RunWeeklyDigests(time.Date(2026, 11, 16, 9, 0, 0, 0, ny)); n != 0 || len(*got) != 0 {
		t.Fatalf("sent %d with the digest switched off", n)
	}
	// Switching it back on mid-week still delivers last week's digest.
	if err := models.SetNotificationPreference(context.Background(), db, coach.ID, models.NotifyWeeklyDigest, true, false); err != nil {
		t.Fatal(err)
	}
	if n := New(db).RunWeeklyDigests(time.Date(2026, 11, 18, 9, 0, 0, 0, ny)); n != 1 {
		t.Fatalf("sent %d after re-enabling", n)
	}
}

func TestFormatDigest_NoSchedule(t *testing.T) {
	title, msg := FormatDigest(time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC), []*models.DigestAthlete{
		{Name: "Sam", Plan: &models.WeekPlan{}},
	})
	if title != "Weekly digest: Nov 30–Dec 6" {
		t.Fatalf("title = %q", title)
	}
	if !strings.Contains(msg, "Sam: no sessions were scheduled.") {
		t.Fatalf("message = %q", msg)
	}
}

func TestMondayOf(t *testing.T) {
	ny, _ := time.LoadLocation("America/New_York")
	for _, tc := range []struct {
		in   time.Time
		want string
	}{
		{time.Date(2026, 11, 16, 0, 30, 0, 0, ny), "2026-11-16"},
		{time.Date(2026, 11, 22, 23, 0, 0, 0, ny), "2026-11-16"},
		{time.Date(2026, 11, 18, 12, 0, 0, 0, ny), "2026-11-16"},
	} {
		if got := mondayOf(tc.in).Format("2006-01-02"); got != tc.want {
			t.Errorf("mondayOf(%v) = %s, want %s", tc.in, got, tc.want)
		}
	}
}
