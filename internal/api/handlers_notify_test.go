package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

// --- Notification dispatch tests (issue #10) ---
//
// Each test exercises one of the six notification types ADR 008 names by
// driving a real HTTP request through the harness and then inspecting the
// notifications table for the expected row. The test harness has no SMTP
// or Shoutrrr configured, so notify.Send only runs the in-app channel —
// which is exactly the surface we want to verify.

// linkedAthleteUser creates an athlete owned by `coach` and a user linked to
// that athlete (so notifications addressed to the athlete have a real
// recipient). Returns (athlete, recipient).
func linkedAthleteUser(t *testing.T, env *testEnv, coachID int64, athleteName, recipientName string) (*models.Athlete, *models.User) {
	t.Helper()
	athlete := env.createAthlete(t, athleteName, coachID)
	recipient, err := models.CreateUser(context.Background(), env.DB, recipientName, recipientName, "password123",
		recipientName+"@example.com", false, false,
		sql.NullInt64{Int64: athlete.ID, Valid: true})
	if err != nil {
		t.Fatalf("create linked recipient: %v", err)
	}
	if err := models.EnsureUserPreferences(context.Background(), env.DB, recipient.ID); err != nil {
		t.Fatalf("ensure prefs: %v", err)
	}
	return athlete, recipient
}

// listNotifications is a tiny helper that reads everything in the
// notifications table for a given user, newest first.
func listNotifications(t *testing.T, env *testEnv, userID int64) []*models.Notification {
	t.Helper()
	notifs, err := models.ListNotifications(context.Background(), env.DB, userID, 50, 0)
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	return notifs
}

func TestNotify_ReviewSubmitted(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete, recipient := linkedAthleteUser(t, env, coach.ID, "Charlie", "charlie")
	workout := env.createWorkout(t, athlete.ID, "2026-05-12")
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/workouts/%d/review", athlete.ID, workout.ID),
		`{"status":"approved","notes":"good lockout"}`, cookies)
	requireStatus(t, rr, http.StatusOK)

	notifs := listNotifications(t, env, recipient.ID)
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification for athlete user, got %d", len(notifs))
	}
	n := notifs[0]
	if n.Type != models.NotifyReviewSubmitted {
		t.Errorf("type = %q, want %q", n.Type, models.NotifyReviewSubmitted)
	}
	if n.Title != "Workout approved" {
		t.Errorf("title = %q, want 'Workout approved'", n.Title)
	}
	if n.Message.String != "good lockout" {
		t.Errorf("message = %q, want 'good lockout'", n.Message.String)
	}
	if !n.AthleteID.Valid || n.AthleteID.Int64 != athlete.ID {
		t.Errorf("athlete_id = %v, want %d", n.AthleteID, athlete.ID)
	}
}

func TestNotify_ReviewSubmitted_NeedsWorkTitle(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete, recipient := linkedAthleteUser(t, env, coach.ID, "Charlie", "charlie")
	workout := env.createWorkout(t, athlete.ID, "2026-05-12")
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/workouts/%d/review", athlete.ID, workout.ID),
		`{"status":"needs_work","notes":"set 3 was high"}`, cookies)
	requireStatus(t, rr, http.StatusOK)

	notifs := listNotifications(t, env, recipient.ID)
	if len(notifs) != 1 || notifs[0].Title != "Workout needs work" {
		t.Errorf("expected 'Workout needs work' notification, got %+v", notifs)
	}
}

func TestNotify_ProgramAssigned(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete, recipient := linkedAthleteUser(t, env, coach.ID, "Charlie", "charlie")
	tpl, err := models.CreateProgramTemplate(context.Background(), env.DB, nil, "5/3/1 Beginner", "", 4, 3, false, "")
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(`{"template_id":%d,"start_date":"2026-05-12"}`, tpl.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/programs", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)

	notifs := listNotifications(t, env, recipient.ID)
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifs))
	}
	n := notifs[0]
	if n.Type != models.NotifyProgramAssigned {
		t.Errorf("type = %q, want %q", n.Type, models.NotifyProgramAssigned)
	}
	if !n.Message.Valid || n.Message.String != "5/3/1 Beginner — starting 2026-05-12" {
		t.Errorf("message = %q, want '5/3/1 Beginner — starting 2026-05-12'", n.Message.String)
	}
}

func TestNotify_TrainingMaxUpdated(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete, recipient := linkedAthleteUser(t, env, coach.ID, "Charlie", "charlie")
	exercise := env.createExercise(t, "Squat")
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(`{"exercise_id":%d,"weight":315,"effective_date":"2026-05-12"}`, exercise.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/training-maxes", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)

	notifs := listNotifications(t, env, recipient.ID)
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifs))
	}
	n := notifs[0]
	if n.Type != models.NotifyTMUpdated {
		t.Errorf("type = %q, want %q", n.Type, models.NotifyTMUpdated)
	}
	if n.Message.String != "Squat: 315 lbs" {
		t.Errorf("message = %q, want 'Squat: 315 lbs'", n.Message.String)
	}
}

func TestNotify_NoteAdded_PublicOnly(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete, recipient := linkedAthleteUser(t, env, coach.ID, "Charlie", "charlie")
	cookies := env.loginAs(t, coach)

	// Public note from coach -> notifies athlete.
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/notes", athlete.ID),
		`{"content":"focus on hip drive","is_private":false}`, cookies)
	requireStatus(t, rr, http.StatusCreated)
	if got := len(listNotifications(t, env, recipient.ID)); got != 1 {
		t.Fatalf("public note: expected 1 notification, got %d", got)
	}

	// Private note from coach -> NO notification.
	rr = env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/notes", athlete.ID),
		`{"content":"internal: ask about sleep","is_private":true}`, cookies)
	requireStatus(t, rr, http.StatusCreated)
	if got := len(listNotifications(t, env, recipient.ID)); got != 1 {
		t.Errorf("private note added: expected count to stay 1, got %d", got)
	}
}

func TestNotify_NoteAdded_SkipsSelfAuthored(t *testing.T) {
	// When a user-with-linked-athlete writes a public note about
	// themselves, they should NOT receive a notification (no point
	// pinging yourself).
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	_, recipient := linkedAthleteUser(t, env, coach.ID, "Charlie", "charlie")
	// The recipient logs in and writes a public note about themselves.
	cookies := env.loginAs(t, recipient)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/notes", recipient.AthleteID.Int64),
		`{"content":"felt great today","is_private":false}`, cookies)
	requireStatus(t, rr, http.StatusCreated)

	if got := len(listNotifications(t, env, recipient.ID)); got != 0 {
		t.Errorf("self-authored note: expected 0 notifications, got %d", got)
	}
}

func TestNotify_WorkoutLogged_NotifiesCoach(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete, recipient := linkedAthleteUser(t, env, coach.ID, "Charlie", "charlie")
	// The athlete (recipient user) logs the workout — coach should be notified.
	cookies := env.loginAs(t, recipient)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/workouts", athlete.ID),
		`{"date":"2026-05-12"}`, cookies)
	requireStatus(t, rr, http.StatusCreated)

	notifs := listNotifications(t, env, coach.ID)
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification for coach, got %d", len(notifs))
	}
	n := notifs[0]
	if n.Type != models.NotifyWorkoutLogged {
		t.Errorf("type = %q, want %q", n.Type, models.NotifyWorkoutLogged)
	}
	if n.Title != "Charlie logged a workout" {
		t.Errorf("title = %q, want 'Charlie logged a workout'", n.Title)
	}
}

func TestNotify_WorkoutLogged_SkipsCoachLoggingForAthlete(t *testing.T) {
	// When the coach themselves creates the workout (e.g. logging on the
	// kid's behalf), the coach should NOT get a "look what someone logged"
	// notification — they were the one who did it.
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/workouts", athlete.ID),
		`{"date":"2026-05-12"}`, cookies)
	requireStatus(t, rr, http.StatusCreated)

	if got := len(listNotifications(t, env, coach.ID)); got != 0 {
		t.Errorf("coach-self-logged: expected 0 coach notifications, got %d", got)
	}
}

func TestNotify_MagicLinkSent(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	target := env.createUser(t, "kid", false, false)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "POST", fmt.Sprintf("/api/users/%d/tokens", target.ID),
		`{"label":"iPad"}`, cookies)
	requireStatus(t, rr, http.StatusCreated)

	notifs := listNotifications(t, env, target.ID)
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification for target, got %d", len(notifs))
	}
	n := notifs[0]
	if n.Type != models.NotifyMagicLinkSent {
		t.Errorf("type = %q, want %q", n.Type, models.NotifyMagicLinkSent)
	}
	// Security fix: the persisted notification must NOT embed the usable
	// magic-link token — a stored notification row is a durable artifact and a
	// token in it would be a replayable credential at rest. The admin shares
	// the one-time URL from the create response out-of-band instead.
	if strings.Contains(n.Link.String, "/auth/token/") {
		t.Errorf("notification link must not embed the magic-link token, got %q", n.Link.String)
	}
}

func TestNotify_NoLinkedUser_NoOp(t *testing.T) {
	// If the athlete has no linked user account (e.g. very young kids
	// whose parent does all the logging), notifyAthlete must be a quiet
	// no-op — no row, no error, no panic.
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Toddler", coach.ID)
	exercise := env.createExercise(t, "Squat")
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(`{"exercise_id":%d,"weight":135,"effective_date":"2026-05-12"}`, exercise.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/training-maxes", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)

	// Coach should not get any notification either — they only get
	// workout_logged, not tm_updated.
	if got := len(listNotifications(t, env, coach.ID)); got != 0 {
		t.Errorf("coach should have no notifications from a TM update, got %d", got)
	}
}
