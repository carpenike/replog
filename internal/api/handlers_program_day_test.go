package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

// Ryan's Trainerize export (Nov 5–22 2026), read for its shape only: the
// program is an 8-week weekly cycle that stores week 1's three days, assigned
// from Thursday 2026-11-05 on Mon/Wed/Fri. Week 1 is Mon 2 – Sun 8 Nov.
var ryanProgramDays = map[int][]string{
	1: {"Back Squat", "Flat Barbell Bench Press", "Pull Up", "DB Lateral Raise", "DB Hammer Curl", "Single Leg Calf Raise", "Kelso Shrug"},
	2: {"Barbell RDL", "High Incline DB Press", "Chest Supported DB Row", "Bench Supported Reverse Fly", "Dumbbell Flat Bench Chest Fly", "BB Shrug", "Incline Skull Crusher", "Ab Roller Wheel Abdominal Roll Out"},
	3: {"Dumbbell Split Squat", "Low Incline dumbbell press", "Barbell Bent Over Row", "Dumbbell Push Up", "Leg Curl (rowing machine)", "Dumbbell Bicep Curl", "Single Leg Calf Raise", "Hanging Knee Raise"},
}

var ryanWarmups = []string{"Cat to Cow", "Adductor Rock Back", "Shoulder Hovers"}

type ryanSession struct {
	date      string
	week, day int
	lines     []string // "Exercise|reps x lbs, ..." ; bare reps = bodyweight
}

// The seven sessions in the order Ryan trained them, with his week/day.
var ryanSessions = []ryanSession{
	{"2026-11-06", 1, 3, []string{"Dumbbell Split Squat|7x25,7x25,7x25", "Low Incline dumbbell press|7x25,7x25,7x25", "Barbell Bent Over Row|8x95,8x95,8x95", "Dumbbell Push Up|8,8", "Leg Curl (rowing machine)|6,6", "Dumbbell Bicep Curl|8x25,8x25", "Single Leg Calf Raise|8x25,10x25", "Hanging Knee Raise|10,10"}},
	{"2026-11-09", 2, 1, []string{"Back Squat|5x105,5x155,5x175", "Flat Barbell Bench Press|5x132,5x143,5x152", "Pull Up|5,5,5", "DB Lateral Raise|10x12,10x12", "DB Hammer Curl|8x25,8x25", "Single Leg Calf Raise|10x25,10x25,10x25", "Kelso Shrug|10x25,10x25,10x25"}},
	{"2026-11-11", 2, 2, []string{"Barbell RDL|8x155,8x177,8x188", "High Incline DB Press|8x25,10x25,12x25", "Chest Supported DB Row|10x25,10x25,12x25", "Bench Supported Reverse Fly|8x12,6x12", "Dumbbell Flat Bench Chest Fly|10x12,15x12", "BB Shrug|10x45,15x45", "Incline Skull Crusher|10x25,10x25", "Ab Roller Wheel Abdominal Roll Out|8,8"}},
	{"2026-11-13", 2, 3, []string{"Dumbbell Split Squat|6x25,8x25,8x25", "Low Incline dumbbell press|8x25,8x25,10x25", "Barbell Bent Over Row|10x110,10x110,10x110", "Dumbbell Push Up|12,10", "Leg Curl (rowing machine)|10,10", "Dumbbell Bicep Curl|8x25,8x25", "Single Leg Calf Raise|10x25,10x25", "Hanging Knee Raise|10,10"}},
	{"2026-11-16", 3, 1, []string{"Back Squat|5x135,5x135,5x135", "Flat Barbell Bench Press|5x135,5x135,5x135", "Pull Up|5,5,5", "DB Lateral Raise|10x12,10x12", "DB Hammer Curl|10x25,8x25", "Single Leg Calf Raise|12x25,12x25,12", "Kelso Shrug|10x25,10x25,10x25"}},
	{"2026-11-18", 3, 2, []string{"Barbell RDL|5x198,5x220,5x242", "High Incline DB Press|10x25,12x25,14x25", "Chest Supported DB Row|10x25,10x25,12x25", "Bench Supported Reverse Fly|8x12,10x12", "Dumbbell Flat Bench Chest Fly|12x12,12x12", "BB Shrug|12x45,20x45", "Incline Skull Crusher|12x15,15x15", "Ab Roller Wheel Abdominal Roll Out|8,8"}},
	{"2026-11-20", 3, 3, []string{"Dumbbell Split Squat|10x25,10x25,10x25", "Low Incline dumbbell press|15x25,15x25,15x25", "Barbell Bent Over Row|10x110,10x110,10x110", "Dumbbell Push Up|12,10", "Leg Curl (rowing machine)|10,12", "Dumbbell Bicep Curl|10x25,10x25", "Single Leg Calf Raise|12x25,12x25", "Hanging Knee Raise|10,10"}},
}

type ryanFixture struct {
	env     *testEnv
	athlete *models.Athlete
	token   string
	tpl     *models.ProgramTemplate
	program *models.AthleteProgram
	ex      map[string]int64
}

func newRyanFixture(t *testing.T) *ryanFixture {
	t.Helper()
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Throwaway Athlete", coach.ID)
	fx := &ryanFixture{env: env, athlete: athlete, token: env.coachServiceToken(t, coach.ID), ex: map[string]int64{}}
	for _, name := range ryanWarmups {
		fx.ex[name] = env.createExercise(t, name).ID
	}
	for _, names := range ryanProgramDays {
		for _, name := range names {
			if _, ok := fx.ex[name]; !ok {
				fx.ex[name] = env.createExercise(t, name).ID
			}
		}
	}
	ctx := context.Background()
	tpl, err := models.CreateProgramTemplate(ctx, env.DB, nil, "Phase One | Fat Loss", "", 8, 3, true, "")
	if err != nil {
		t.Fatal(err)
	}
	fx.tpl = tpl
	reps := 10
	for day := 1; day <= 3; day++ {
		for i, name := range ryanProgramDays[day] {
			// Week 1 only, like an imported weekly cycle.
			if _, err := models.CreatePrescribedSet(ctx, env.DB, tpl.ID, fx.ex[name], 1, day, 1, &reps, nil, nil, i, "reps", ""); err != nil {
				t.Fatal(err)
			}
		}
	}
	program, err := models.AssignProgram(ctx, env.DB, athlete.ID, tpl.ID, "2026-11-05", "", "", "primary", "[1,3,5]")
	if err != nil {
		t.Fatal(err)
	}
	fx.program = program
	return fx
}

func (fx *ryanFixture) sets(t *testing.T, s ryanSession) []map[string]any {
	t.Helper()
	var out []map[string]any
	order := 0
	add := func(name string, reps int, weight *float64, warmup bool) {
		order++
		id, ok := fx.ex[name]
		if !ok {
			t.Fatalf("no exercise %q", name)
		}
		out = append(out, map[string]any{"exercise_id": id, "reps": reps, "weight_lbs": weight, "set_order": order, "warmup": warmup})
	}
	for _, name := range ryanWarmups {
		add(name, 10, nil, true)
		add(name, 10, nil, true)
	}
	for _, line := range s.lines {
		name, spec, _ := strings.Cut(line, "|")
		for _, set := range strings.Split(spec, ",") {
			repsStr, lbs, hasWeight := strings.Cut(set, "x")
			reps, err := strconv.Atoi(repsStr)
			if err != nil {
				t.Fatal(err)
			}
			var weight *float64
			if hasWeight {
				w, err := strconv.ParseFloat(lbs, 64)
				if err != nil {
					t.Fatal(err)
				}
				weight = &w
			}
			add(name, reps, weight, false)
		}
	}
	return out
}

func (fx *ryanFixture) week(t *testing.T, monday string) CoachWeekResponse {
	t.Helper()
	rr := fx.env.serveBearer(t, "GET", fmt.Sprintf("/api/athletes/%d/coach/week?week_start=%s", fx.athlete.ID, monday), fx.token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var week CoachWeekResponse
	decodeJSON(t, rr, &week)
	return week
}

// Backfilling Ryan's seven sessions with their real week/day stores those
// days, and plan-vs-done compares each against that template day. They are
// sent newest first to show the result does not depend on insert order.
func TestBackfill_StoredProgramDays_RyanOrder(t *testing.T) {
	fx := newRyanFixture(t)
	env := fx.env
	base := fmt.Sprintf("/api/athletes/%d/coach/sessions/backfill", fx.athlete.ID)
	ids := map[string]int64{}
	for i := len(ryanSessions) - 1; i >= 0; i-- {
		s := ryanSessions[i]
		body := map[string]any{
			"date": s.date, "assignment_id": fx.program.ID, "template_id": fx.tpl.ID, "template_version": 1,
			"program_week": s.week, "program_day": s.day, "session_rpe": 6, "sets": fx.sets(t, s),
		}
		rr := env.serveBearer(t, "POST", base+"/preview", fx.token, asJSON(t, body), nil)
		requireStatus(t, rr, http.StatusOK)
		var p CoachBackfillPreviewResponse
		decodeJSON(t, rr, &p)
		want := fmt.Sprintf("Week %d Day %d", s.week, s.day)
		if p.Program == nil || p.Program.Week != s.week || p.Program.Day != s.day || p.Program.ProgramDayLabel != want || p.Program.ProgramDaySource != "given" {
			t.Fatalf("%s program = %+v", s.date, p.Program)
		}
		if len(p.Warnings) != 0 || !p.CanConfirm || p.PreviewToken == nil {
			t.Fatalf("%s warnings = %+v", s.date, p.Warnings)
		}
		rr = env.serveBearer(t, "POST", base+"/confirm", fx.token, asJSON(t, map[string]any{"preview_token": *p.PreviewToken, "confirm": true}), nil)
		requireStatus(t, rr, http.StatusCreated)
		var c CoachBackfillConfirmResponse
		decodeJSON(t, rr, &c)
		if c.ProgramDayLabel != want || c.ProgramWeek == nil || *c.ProgramWeek != s.week || c.ProgramDay == nil || *c.ProgramDay != s.day {
			t.Fatalf("%s confirm = %+v", s.date, c)
		}
		ids[s.date] = c.SessionID
		if n := countRows(t, env, `SELECT COUNT(*) FROM workouts WHERE id = ? AND program_week = ? AND program_day = ?`, c.SessionID, s.week, s.day); n != 1 {
			t.Fatalf("%s stored day missing", s.date)
		}
	}

	byDate := map[string]CoachWeekDayResponse{}
	for _, monday := range []string{"2026-11-02", "2026-11-09", "2026-11-16"} {
		for _, d := range fx.week(t, monday).Days {
			byDate[d.Date] = d
		}
	}
	// Mon 2 and Wed 4 Nov are before the Thursday start: not planned.
	if len(byDate) != 7 {
		t.Fatalf("planned days = %d", len(byDate))
	}
	for _, s := range ryanSessions {
		d, ok := byDate[s.date]
		if !ok {
			t.Fatalf("%s not planned", s.date)
		}
		planned := len(ryanProgramDays[s.day])
		if d.Week == nil || *d.Week != s.week || d.Day == nil || *d.Day != s.day || d.DaySource == nil || *d.DaySource != models.DaySourceStored {
			t.Fatalf("%s plan day = %+v", s.date, d)
		}
		if d.Status != "done" || d.PlannedExercises != planned || d.LoggedExercises != planned || d.SessionID == nil || *d.SessionID != ids[s.date] {
			t.Fatalf("%s plan-vs-done = %+v (want done %d/%d)", s.date, d, planned, planned)
		}
	}

	// The next session after Week 3 Day 3 is Week 4 Day 1.
	next := fx.week(t, "2026-11-23")
	if len(next.Days) == 0 || next.Days[0].Date != "2026-11-23" || *next.Days[0].Week != 4 || *next.Days[0].Day != 1 || *next.Days[0].DaySource != models.DaySourceNext {
		t.Fatalf("next week = %+v", next.Days)
	}

	// Coach history shows the stored labels.
	rr := env.serveBearer(t, "GET", fmt.Sprintf("/api/athletes/%d/coach/sessions?start=2026-11-01&end=2026-11-30", fx.athlete.ID), fx.token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var hist CoachHistoryResponse
	decodeJSON(t, rr, &hist)
	if len(hist.Sessions) != 7 {
		t.Fatalf("history = %d sessions", len(hist.Sessions))
	}
	for i, s := range hist.Sessions {
		want := ryanSessions[len(ryanSessions)-1-i]
		label := fmt.Sprintf("Week %d Day %d", want.week, want.day)
		if s.Date != want.date || s.ProgramDayLabel == nil || *s.ProgramDayLabel != label {
			t.Fatalf("history[%d] = %s %v, want %s %s", i, s.Date, s.ProgramDayLabel, want.date, label)
		}
	}
}

func TestBackfill_ProgramDayValidation(t *testing.T) {
	fx := newRyanFixture(t)
	base := fmt.Sprintf("/api/athletes/%d/coach/sessions/backfill/preview", fx.athlete.ID)
	s := ryanSessions[0]
	preview := func(extra map[string]any) (*CoachBackfillPreviewResponse, int, string) {
		body := map[string]any{"date": s.date, "assignment_id": fx.program.ID, "sets": fx.sets(t, s)}
		for k, v := range extra {
			body[k] = v
		}
		rr := fx.env.serveBearer(t, "POST", base, fx.token, asJSON(t, body), nil)
		if rr.Code != http.StatusOK {
			return nil, rr.Code, rr.Body.String()
		}
		var p CoachBackfillPreviewResponse
		decodeJSON(t, rr, &p)
		return &p, rr.Code, ""
	}
	for _, bad := range []map[string]any{
		{"program_day": 4},                    // the template has 3 days
		{"program_week": 9, "program_day": 1}, // and 8 weeks
		{"program_week": 2},                   // week without day
	} {
		if _, code, body := preview(bad); code != http.StatusBadRequest || !strings.Contains(body, "program_") {
			t.Fatalf("%v: %d %s", bad, code, body)
		}
	}
	// program_day alone reads the calendar week of the assignment.
	p, _, _ := preview(map[string]any{"program_day": 3})
	if p.Program.Week != 1 || p.Program.Day != 3 || p.Program.ProgramDaySource != "given" || len(p.Warnings) != 0 {
		t.Fatalf("calendar week = %+v %+v", p.Program, p.Warnings)
	}
	p, _, _ = preview(map[string]any{"date": "2026-11-18", "program_day": 2})
	if p.Program.Week != 3 || p.Program.Day != 2 {
		t.Fatalf("calendar week 3 = %+v", p.Program)
	}
	// No day sent: derived, as before.
	p, _, _ = preview(nil)
	if p.Program.Week != 1 || p.Program.Day != 1 || p.Program.ProgramDaySource != "derived" || hasWarning(*p, "program_day_mismatch") {
		t.Fatalf("derived = %+v %+v", p.Program, p.Warnings)
	}
}

// A day the template does not store is refused even inside the week/day range.
func TestValidateProgramDay_MissingTemplateDay(t *testing.T) {
	env := setupTest(t)
	ctx := context.Background()
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "A", coach.ID)
	ex := env.createExercise(t, "Squat")
	tpl := env.createProgramTemplate(t, "Two weeks", 2, 2)
	reps := 5
	for _, wd := range [][2]int{{1, 1}, {1, 2}, {2, 1}} {
		if _, err := models.CreatePrescribedSet(ctx, env.DB, tpl.ID, ex.ID, wd[0], wd[1], 1, &reps, nil, nil, 0, "reps", ""); err != nil {
			t.Fatal(err)
		}
	}
	program, err := models.AssignProgram(ctx, env.DB, athlete.ID, tpl.ID, "2026-11-02", "", "", "primary", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := models.ValidateProgramDay(ctx, env.DB, program, 1, 2, 1); err != nil {
		t.Fatalf("week 2 day 1: %v", err)
	}
	if err := models.ValidateProgramDay(ctx, env.DB, program, 1, 2, 2); err == nil || !strings.Contains(err.Error(), "has no Week 2 Day 2") {
		t.Fatalf("week 2 day 2: %v", err)
	}
}

// Live logging links the session and stores the next day by default, or the
// day the caller picks; the prescription can show a chosen day first.
func TestCreateWorkout_StoresProgramDay(t *testing.T) {
	fx := newRyanFixture(t)
	env := fx.env
	path := fmt.Sprintf("/api/athletes/%d/workouts", fx.athlete.ID)
	create := func(body map[string]any, want int) *Workout {
		t.Helper()
		rr := env.serveBearer(t, "POST", path, fx.token, asJSON(t, body), nil)
		requireStatus(t, rr, want)
		if want != http.StatusCreated {
			return nil
		}
		var w Workout
		decodeJSON(t, rr, &w)
		return &w
	}

	// Default: the next day (nothing stored yet, so the count: Week 1 Day 1).
	w := create(map[string]any{"date": "2026-11-06", "link_program": true}, http.StatusCreated)
	if w.AssignmentID == nil || *w.AssignmentID != fx.program.ID || w.ProgramWeek == nil || *w.ProgramWeek != 1 || *w.ProgramDay != 1 || w.SetCount != 0 {
		t.Fatalf("default = %+v", w)
	}
	// Next after a stored day.
	w = create(map[string]any{"date": "2026-11-09", "link_program": true}, http.StatusCreated)
	if *w.ProgramWeek != 1 || *w.ProgramDay != 2 {
		t.Fatalf("next = W%d D%d", *w.ProgramWeek, *w.ProgramDay)
	}
	// The caller picks the day; program_day implies the link.
	w = create(map[string]any{"date": "2026-11-11", "program_week": 2, "program_day": 2}, http.StatusCreated)
	if w.AssignmentID == nil || *w.ProgramWeek != 2 || *w.ProgramDay != 2 {
		t.Fatalf("chosen = %+v", w)
	}
	// from_prescription seeds the stored day's prescribed sets.
	w = create(map[string]any{"date": "2026-11-13", "from_prescription": true, "program_day": 3}, http.StatusCreated)
	if *w.ProgramWeek != 2 || *w.ProgramDay != 3 {
		t.Fatalf("seeded day = %+v", w)
	}
	if n := countRows(t, env, `SELECT COUNT(DISTINCT exercise_id) FROM workout_sets WHERE workout_id = ? AND exercise_id = ?`, w.ID, fx.ex["Hanging Knee Raise"]); n != 1 {
		t.Fatalf("seeded sets are not Day 3's")
	}
	// Bad days.
	create(map[string]any{"date": "2026-11-16", "program_day": 5}, http.StatusBadRequest)
	create(map[string]any{"date": "2026-11-16", "program_week": 2}, http.StatusBadRequest)
	create(map[string]any{"date": "2026-11-02", "program_day": 1}, http.StatusBadRequest) // before the assignment
	// No link asked: unlinked, nothing stored, as before.
	w = create(map[string]any{"date": "2026-11-17"}, http.StatusCreated)
	if w.AssignmentID != nil || w.ProgramWeek != nil {
		t.Fatalf("unlinked = %+v", w)
	}
}

func TestGetPrescription_ChosenDay(t *testing.T) {
	fx := newRyanFixture(t)
	path := fmt.Sprintf("/api/athletes/%d/prescription", fx.athlete.ID)
	rr := fx.env.serveBearer(t, "GET", path, fx.token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var p PrescriptionResponse
	decodeJSON(t, rr, &p)
	if p.NumWeeks != 8 || p.NumDays != 3 || p.DaySource != models.DaySourceCount {
		t.Fatalf("default = %+v", p)
	}
	rr = fx.env.serveBearer(t, "GET", path+"?week=2&day=3", fx.token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &p)
	if p.CurrentWeek != 2 || p.CurrentDay != 3 || p.DaySource != "chosen" || len(p.Lines) != len(ryanProgramDays[3]) {
		t.Fatalf("chosen = %+v", p)
	}
	rr = fx.env.serveBearer(t, "GET", path+"?week=2&day=4", fx.token, nil, nil)
	requireStatus(t, rr, http.StatusBadRequest)
}
