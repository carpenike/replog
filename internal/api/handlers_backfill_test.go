package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

type backfillFixture struct {
	env       *testEnv
	coach     *models.User
	athlete   *models.Athlete
	token     string
	program   *models.AthleteProgram
	tpl       *models.ProgramTemplate
	ex        map[string]int64
	mon       string
	tue       string
	wed       string
	fri       string
	base      string
	nextMon   string
	weekStart string
}

// A Trainerize "Day 3" session (20 Nov in the source export) plus the three
// warm-up drills logged every session: 25 sets.
var trainerizeDay3 = []struct {
	name   string
	reps   []int
	weight *float64
	warmup bool
}{
	{"Cat to Cow", []int{10, 10}, nil, true},
	{"Adductor Rock Back", []int{10, 10}, nil, true},
	{"Shoulder Hovers", []int{10, 10}, nil, true},
	{"Dumbbell Split Squat", []int{10, 10, 10}, f64(25), false},
	{"Low Incline Dumbbell Press", []int{15, 15, 15}, f64(25), false},
	{"Barbell Bent Over Row", []int{10, 10, 10}, f64(110), false},
	{"Dumbbell Push Up", []int{12, 10}, nil, false},
	{"Leg Curl (rowing machine)", []int{10, 12}, nil, false},
	{"Dumbbell Bicep Curl", []int{10, 10}, f64(25), false},
	{"Single Leg Calf Raise", []int{12, 12}, f64(25), false},
	{"Hanging Knee Raise", []int{10, 10}, nil, false},
}

func f64(v float64) *float64 { return &v }

func asJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func newBackfillFixture(t *testing.T) *backfillFixture {
	t.Helper()
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Test Athlete", coach.ID)
	fx := &backfillFixture{env: env, coach: coach, athlete: athlete, token: env.coachServiceToken(t, coach.ID), ex: map[string]int64{}}
	for _, e := range trainerizeDay3 {
		fx.ex[e.name] = env.createExercise(t, e.name).ID
	}
	loc, _ := time.LoadLocation("America/New_York")
	monday := dateAtMonday(time.Now().In(loc)).AddDate(0, 0, -14)
	fx.mon = monday.Format(dateLayout)
	fx.tue = monday.AddDate(0, 0, 1).Format(dateLayout)
	fx.wed = monday.AddDate(0, 0, 2).Format(dateLayout)
	fx.fri = monday.AddDate(0, 0, 4).Format(dateLayout)
	fx.nextMon = monday.AddDate(0, 0, 7).Format(dateLayout)
	fx.weekStart = fx.mon
	fx.tpl = env.createProgramTemplate(t, "Phase One", 1, 3)
	reps := 10
	for day := 1; day <= 3; day++ {
		for i, name := range []string{"Dumbbell Split Squat", "Barbell Bent Over Row"} {
			if _, err := models.CreatePrescribedSet(context.Background(), env.DB, fx.tpl.ID, fx.ex[name], 1, day, 1, &reps, nil, nil, i, "reps", ""); err != nil {
				t.Fatalf("prescribe: %v", err)
			}
		}
	}
	program, err := models.AssignProgram(context.Background(), env.DB, athlete.ID, fx.tpl.ID, fx.mon, "", "", "primary", "[1,3,5]")
	if err != nil {
		t.Fatalf("assign: %v", err)
	}
	fx.program = program
	fx.base = fmt.Sprintf("/api/athletes/%d/coach/sessions/backfill", athlete.ID)
	return fx
}

func (fx *backfillFixture) payload(date string, extra map[string]any) map[string]any {
	sets := []map[string]any{}
	order := 0
	for _, e := range trainerizeDay3 {
		for _, r := range e.reps {
			order++
			s := map[string]any{"exercise_id": fx.ex[e.name], "reps": r, "weight_lbs": e.weight, "rep_type": "reps", "set_order": order}
			if e.warmup {
				s["warmup"] = true
			}
			sets = append(sets, s)
		}
	}
	body := map[string]any{"date": date, "assignment_id": fx.program.ID, "template_id": fx.tpl.ID, "template_version": 1, "session_rpe": 6, "notes": "Backfilled from Trainerize", "sets": sets}
	for k, v := range extra {
		if v == nil {
			delete(body, k)
		} else {
			body[k] = v
		}
	}
	return body
}

func (fx *backfillFixture) preview(t *testing.T, body any, want int) CoachBackfillPreviewResponse {
	t.Helper()
	rr := fx.env.serveBearer(t, "POST", fx.base+"/preview", fx.token, asJSON(t, body), nil)
	requireStatus(t, rr, want)
	var out CoachBackfillPreviewResponse
	if want == http.StatusOK {
		decodeJSON(t, rr, &out)
	}
	return out
}

func (fx *backfillFixture) confirm(t *testing.T, token string, want int) CoachBackfillConfirmResponse {
	t.Helper()
	rr := fx.env.serveBearer(t, "POST", fx.base+"/confirm", fx.token, asJSON(t, map[string]any{"preview_token": token, "confirm": true}), nil)
	requireStatus(t, rr, want)
	var out CoachBackfillConfirmResponse
	if want == http.StatusCreated {
		decodeJSON(t, rr, &out)
	}
	return out
}

func countRows(t *testing.T, env *testEnv, q string, args ...any) int {
	t.Helper()
	var n int
	if err := env.DB.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func hasWarning(p CoachBackfillPreviewResponse, code string) bool {
	for _, w := range p.Warnings {
		if w.Code == code {
			return true
		}
	}
	return false
}

func TestCoachBackfill_PreviewConfirmAndPlanVsDone(t *testing.T) {
	fx := newBackfillFixture(t)
	env := fx.env

	// Before: Monday is a missed planned day.
	rr := env.serveBearer(t, "GET", fmt.Sprintf("/api/athletes/%d/coach/week?week_start=%s", fx.athlete.ID, fx.weekStart), fx.token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var week CoachWeekResponse
	decodeJSON(t, rr, &week)
	if week.DoneCount != 0 || week.MissedCount != 3 {
		t.Fatalf("week before = %+v", week)
	}

	p := fx.preview(t, fx.payload(fx.mon, map[string]any{"program_day": 3}), http.StatusOK)
	if p.SetCount != 25 || p.WorkingSets != 19 || p.WarmupSets != 6 || p.Exercises != 11 || !p.CanConfirm || p.PreviewToken == nil || !strings.HasPrefix(*p.PreviewToken, "rlbf_") || p.ExpiresAt == nil {
		t.Fatalf("preview = %+v", p)
	}
	// The day sent is the day stored (ADR 028): no mismatch fallback.
	if p.Program == nil || p.Program.AssignmentID != fx.program.ID || p.Program.ProgramDayLabel != "Week 1 Day 3" || p.Program.ProgramDaySource != "given" || p.Program.TemplateVersion != 1 || len(p.Program.Schedule) != 3 {
		t.Fatalf("program = %+v", p.Program)
	}
	if len(p.Warnings) != 0 || p.Duplicate != nil {
		t.Fatalf("warnings = %+v", p.Warnings)
	}
	if p.Notes != "Session RPE: 6/10\nBackfilled from Trainerize" {
		t.Fatalf("notes = %q", p.Notes)
	}
	first, last := p.Sets[0], p.Sets[24]
	if first.SetType != "warmup" || first.WeightLbs != nil || first.ExerciseName == nil || *first.ExerciseName != "Cat to Cow" || last.SetNumber != 2 || last.SetOrder != 25 {
		t.Fatalf("sets = %+v ... %+v", first, last)
	}
	if n := countRows(t, env, `SELECT COUNT(*) FROM workouts`); n != 0 {
		t.Fatalf("preview wrote %d workouts", n)
	}

	// confirm must be true; a missing token is refused.
	rr = env.serveBearer(t, "POST", fx.base+"/confirm", fx.token, asJSON(t, map[string]any{"preview_token": *p.PreviewToken}), nil)
	requireStatus(t, rr, http.StatusBadRequest)
	rr = env.serveBearer(t, "POST", fx.base+"/confirm", fx.token, asJSON(t, map[string]any{"confirm": true}), nil)
	requireStatus(t, rr, http.StatusBadRequest)

	c := fx.confirm(t, *p.PreviewToken, http.StatusCreated)
	if c.SetsWritten != 25 || c.WorkingSets != 19 || c.WarmupSets != 6 || c.AssignmentID == nil || *c.AssignmentID != fx.program.ID || c.ProgramDayLabel != "Week 1 Day 3" || c.ProgramDay == nil || *c.ProgramDay != 3 || c.Date != fx.mon {
		t.Fatalf("confirm = %+v", c)
	}
	if n := countRows(t, env, `SELECT COUNT(*) FROM workout_sets WHERE workout_id = ?`, c.SessionID); n != 25 {
		t.Fatalf("sets stored = %d", n)
	}
	if n := countRows(t, env, `SELECT COUNT(*) FROM workout_sets WHERE workout_id = ? AND set_type = 'warmup' AND weight IS NULL`, c.SessionID); n != 6 {
		t.Fatalf("warm-up sets stored = %d", n)
	}
	if n := countRows(t, env, `SELECT COUNT(*) FROM workout_sets WHERE workout_id = ? AND exercise_id = ? AND set_number = 3 AND weight = 110 AND reps = 10`, c.SessionID, fx.ex["Barbell Bent Over Row"]); n != 1 {
		t.Fatalf("row set 3 missing")
	}

	// Replay writes nothing.
	fx.confirm(t, *p.PreviewToken, http.StatusBadRequest)
	if n := countRows(t, env, `SELECT COUNT(*) FROM workout_sets`); n != 25 {
		t.Fatalf("replay wrote sets: %d", n)
	}

	// Plan-vs-done counts it like a live-logged session.
	rr = env.serveBearer(t, "GET", fmt.Sprintf("/api/athletes/%d/coach/week?week_start=%s", fx.athlete.ID, fx.weekStart), fx.token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &week)
	if week.DoneCount != 1 || week.Days[0].Status != "done" || week.Days[0].SessionID == nil || *week.Days[0].SessionID != c.SessionID ||
		*week.Days[0].Day != 3 || *week.Days[0].DaySource != models.DaySourceStored {
		t.Fatalf("week after = %+v", week)
	}

	// PRs read the new working sets; warm-ups stay out.
	records, err := models.ListAthleteRecords(context.Background(), env.DB, fx.athlete.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundRow, foundWarmup := false, false
	for _, r := range records {
		if r.ExerciseID == fx.ex["Barbell Bent Over Row"] {
			foundRow = true
		}
		if r.ExerciseID == fx.ex["Cat to Cow"] {
			foundWarmup = true
		}
	}
	if !foundRow || foundWarmup {
		t.Fatalf("records row=%v warmup=%v: %+v", foundRow, foundWarmup, records)
	}

	// The same date is a duplicate: flagged, no token.
	dup := fx.preview(t, fx.payload(fx.mon, nil), http.StatusOK)
	if dup.Duplicate == nil || *dup.Duplicate != c.SessionID || dup.CanConfirm || dup.PreviewToken != nil || !hasWarning(dup, "duplicate") {
		t.Fatalf("duplicate preview = %+v", dup)
	}
	// Without a program link the same date is still a duplicate.
	dup = fx.preview(t, fx.payload(fx.mon, map[string]any{"assignment_id": nil, "template_id": nil, "template_version": nil}), http.StatusOK)
	if dup.Duplicate == nil || dup.CanConfirm {
		t.Fatalf("unlinked duplicate preview = %+v", dup)
	}

	// Two previews for one date: the second confirm is 409 and writes nothing.
	a := fx.preview(t, fx.payload(fx.wed, nil), http.StatusOK)
	b := fx.preview(t, fx.payload(fx.wed, nil), http.StatusOK)
	// No day sent: the day after the stored Week 1 Day 3 wraps to Day 1.
	if a.Program.ProgramDayLabel != "Week 1 Day 1" || a.Program.ProgramDaySource != "derived" {
		t.Fatalf("wednesday label = %s", a.Program.ProgramDayLabel)
	}
	fx.confirm(t, *a.PreviewToken, http.StatusCreated)
	fx.confirm(t, *b.PreviewToken, http.StatusConflict)
	if n := countRows(t, env, `SELECT COUNT(*) FROM workout_sets`); n != 50 {
		t.Fatalf("sets after race = %d", n)
	}

	// The Monday session and its sets were not touched by later writes.
	if n := countRows(t, env, `SELECT COUNT(*) FROM workout_sets WHERE workout_id = ?`, c.SessionID); n != 25 {
		t.Fatalf("monday sets changed: %d", n)
	}
}

func TestCoachBackfill_Validation(t *testing.T) {
	fx := newBackfillFixture(t)
	env := fx.env
	loc, _ := time.LoadLocation("America/New_York")
	tomorrow := time.Now().In(loc).AddDate(0, 0, 1).Format(dateLayout)

	fx.preview(t, fx.payload(tomorrow, nil), http.StatusBadRequest)
	fx.preview(t, fx.payload("2026-02-30", nil), http.StatusBadRequest)
	fx.preview(t, fx.payload(fx.mon, map[string]any{"template_id": fx.tpl.ID + 99}), http.StatusBadRequest)
	fx.preview(t, fx.payload(fx.mon, map[string]any{"template_version": 2}), http.StatusBadRequest)
	fx.preview(t, fx.payload(fx.mon, map[string]any{"assignment_id": 999}), http.StatusBadRequest)
	fx.preview(t, fx.payload(fx.mon, map[string]any{"session_rpe": 11}), http.StatusBadRequest)
	fx.preview(t, fx.payload(fx.mon, map[string]any{"sets": []any{}}), http.StatusBadRequest)
	fx.preview(t, fx.payload(fx.mon, map[string]any{"surprise": 1}), http.StatusBadRequest)
	before := time.Date(2020, 1, 6, 0, 0, 0, 0, loc).Format(dateLayout)
	fx.preview(t, fx.payload(before, nil), http.StatusBadRequest) // before the assignment starts

	bad := fx.payload(fx.mon, nil)
	sets := bad["sets"].([]map[string]any)
	sets[1]["set_order"] = 1
	fx.preview(t, bad, http.StatusBadRequest)
	bad = fx.payload(fx.mon, nil)
	bad["sets"].([]map[string]any)[0]["rep_type"] = "laps"
	fx.preview(t, bad, http.StatusBadRequest)
	bad = fx.payload(fx.mon, nil)
	bad["sets"].([]map[string]any)[0]["rpe"] = 8
	fx.preview(t, bad, http.StatusBadRequest)
	bad = fx.payload(fx.mon, nil)
	bad["sets"].([]map[string]any)[0]["weight_lbs"] = -5
	fx.preview(t, bad, http.StatusBadRequest)

	// Unknown exercise: shown, no token.
	bad = fx.payload(fx.mon, nil)
	bad["sets"].([]map[string]any)[3]["exercise_id"] = 987654
	p := fx.preview(t, bad, http.StatusOK)
	if p.CanConfirm || p.PreviewToken != nil || !hasWarning(p, "unknown_exercise") || p.Sets[3].ExerciseName != nil {
		t.Fatalf("unknown exercise preview = %+v", p)
	}

	// Off-schedule day, and template-only link resolves the assignment.
	p = fx.preview(t, fx.payload(fx.tue, map[string]any{"assignment_id": nil}), http.StatusOK)
	if !hasWarning(p, "off_schedule") || p.Program == nil || p.Program.AssignmentID != fx.program.ID || !p.CanConfirm {
		t.Fatalf("tuesday preview = %+v", p)
	}
	// No link on a covered date warns it will not advance the program.
	p = fx.preview(t, fx.payload(fx.fri, map[string]any{"assignment_id": nil, "template_id": nil, "template_version": nil}), http.StatusOK)
	if !hasWarning(p, "unlinked") || p.Program != nil {
		t.Fatalf("unlinked preview = %+v", p)
	}

	// Bad program days.
	fx.preview(t, fx.payload(fx.mon, map[string]any{"program_day": 4}), http.StatusBadRequest)
	fx.preview(t, fx.payload(fx.mon, map[string]any{"program_week": 2, "program_day": 1}), http.StatusBadRequest)
	fx.preview(t, fx.payload(fx.mon, map[string]any{"program_week": 1}), http.StatusBadRequest)

	// Backfilling before a linked session with no stored day (a row from
	// before migration 0028) warns that it shifts.
	if _, err := env.DB.Exec(`INSERT INTO workouts (athlete_id, date, assignment_id) VALUES (?, ?, ?)`, fx.athlete.ID, fx.fri, fx.program.ID); err != nil {
		t.Fatal(err)
	}
	p = fx.preview(t, fx.payload(fx.wed, nil), http.StatusOK)
	if !hasWarning(p, "later_sessions_shift") || p.Program.LaterSessions != 1 {
		t.Fatalf("shift preview = %+v", p)
	}

	// A token is bound to its athlete.
	other := env.createAthlete(t, "Other", fx.coach.ID)
	tok := fx.preview(t, fx.payload(fx.mon, nil), http.StatusOK)
	rr := env.serveBearer(t, "POST", fmt.Sprintf("/api/athletes/%d/coach/sessions/backfill/confirm", other.ID), fx.token, asJSON(t, map[string]any{"preview_token": *tok.PreviewToken, "confirm": true}), nil)
	requireStatus(t, rr, http.StatusBadRequest)
	if n := countRows(t, env, `SELECT COUNT(*) FROM workouts WHERE athlete_id = ?`, other.ID); n != 0 {
		t.Fatalf("wrote to the other athlete")
	}
}

func TestCoachBackfill_AccessAndOnBehalf(t *testing.T) {
	fx := newBackfillFixture(t)
	env := fx.env
	t.Setenv(middleware.OnBehalfCredentialEnv, testOnBehalfCredential)

	// Another coach cannot preview for this athlete.
	stranger := env.createUser(t, "stranger", true, false)
	rr := env.serveBearer(t, "POST", fx.base+"/preview", env.coachServiceToken(t, stranger.ID), asJSON(t, fx.payload(fx.mon, nil)), nil)
	requireStatus(t, rr, http.StatusForbidden)

	// The athlete's own login (not the coach) cannot backfill.
	kid, err := models.CreateUser(context.Background(), env.DB, "kid", "Kid", "password123", "kid@example.com", false, false, sql.NullInt64{Int64: fx.athlete.ID, Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := models.EnsureUserPreferences(context.Background(), env.DB, kid.ID); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(fx.payload(fx.mon, nil))
	rr = doOnBehalfBody(t, env, http.MethodPost, fx.base+"/preview", testOnBehalfCredential, "kid@example.com", string(body), nil)
	requireStatus(t, rr, http.StatusForbidden)

	// The coach on behalf previews and confirms.
	rr = doOnBehalfBody(t, env, http.MethodPost, fx.base+"/preview", testOnBehalfCredential, fx.coach.Email.String, string(body), nil)
	requireStatus(t, rr, http.StatusOK)
	var p CoachBackfillPreviewResponse
	decodeJSON(t, rr, &p)
	rr = doOnBehalfBody(t, env, http.MethodPost, fx.base+"/confirm", testOnBehalfCredential, fx.coach.Email.String, `{"preview_token":"`+*p.PreviewToken+`","confirm":true}`, nil)
	requireStatus(t, rr, http.StatusCreated)

	// Admin routes stay closed on behalf.
	rr = doOnBehalf(t, env, http.MethodGet, "/api/users", testOnBehalfCredential, fx.coach.Email.String, nil)
	requireStatus(t, rr, http.StatusForbidden)
}
