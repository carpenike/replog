package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// day1Instructions is the visible Trainerize text. The screenshot stops at
// "Later exercis..."; the hidden tail is not included.
const day1Instructions = "1. WARM UP\n\nGradually build towards your working weight before your first major exercise:\n\nSet 1: 50% × 5–10 reps\n\nSet 2: 75–80% × 3–5 reps\n\nOptional Set 3: 90% × 1–2 reps (heavy lifts)\n\nLater exercis..."

// day1ProgramJSON is the preview program for Phil's Day 1. Day 2 and Day 3 are not invented.
const day1ProgramJSON = `{
  "name": "Phil Full Body",
  "source": "Phil / Trainerize",
  "author": "Phil",
  "cycle": {"repeat": "weekly", "weekdays": [1, 3, 5], "upcoming": 6},
  "days": [{
    "label": "Day 1 | Full Body",
    "order": 1,
    "kind": "Regular",
    "estimated_minutes": 44,
    "equipment": ["Bench", "Body Weight", "Dumbbell", "Pull-Up Bar"],
    "instructions": "1. WARM UP\n\nGradually build towards your working weight before your first major exercise:\n\nSet 1: 50% × 5–10 reps\n\nSet 2: 75–80% × 3–5 reps\n\nOptional Set 3: 90% × 1–2 reps (heavy lifts)\n\nLater exercis...",
    "blocks": [
      {
        "type": "circuit",
        "order": 1,
        "rounds": 2,
        "non_working": true,
        "exercises": [
          {"name": "Cat to Cow", "order": 1, "sets": 2, "reps_min": 10, "reps_max": 10},
          {"name": "Adductor Rock Back", "order": 2, "sets": 2, "reps_min": 10, "reps_max": 10, "per_side": true},
          {"name": "Shoulder Hovers", "order": 3, "sets": 2, "reps_min": 5, "reps_max": 5}
        ]
      },
      {
        "type": "straight",
        "order": 2,
        "exercises": [
          {"name": "Back Squat", "order": 1, "sets": 3, "reps_min": 5, "reps_max": 8, "rir_min": 1, "rir_max": 2, "rest": "2m"},
          {"name": "Flat Barbell Bench Press", "order": 2, "sets": 3, "reps_min": 5, "reps_max": 8, "rir_min": 1, "rir_max": 2, "rest": "90s"},
          {"name": "Pull Up", "order": 3, "sets": 3, "reps_min": 5, "reps_max": 8, "rir_min": 1, "rir_max": 2, "rest": "90s", "notes": "Use assistance if necessary"}
        ]
      },
      {
        "type": "superset",
        "order": 3,
        "sets": 2,
        "exercises": [
          {"name": "DB Lateral Raise", "order": 1, "sets": 2, "reps_min": 8, "reps_max": 15, "rir_min": 1, "rir_max": 2},
          {"name": "DB Hammer Curl", "order": 2, "sets": 2, "reps_min": 8, "reps_max": 15, "rir_min": 1, "rir_max": 2}
        ]
      },
      {
        "type": "superset",
        "order": 4,
        "sets": 3,
        "rest": "90s",
        "exercises": [
          {"name": "Single Leg Calf Raise", "order": 1, "sets": 3, "reps_min": 8, "reps_max": 15, "rir_min": 1, "rir_max": 2, "per_side": true},
          {"name": "Kelso Shrug", "order": 2, "sets": 3, "reps_min": 8, "reps_max": 15, "rir_min": 1, "rir_max": 2}
        ]
      }
    ]
  }]
}`

func TestCoachExercise_DedupeByNameOrAlias(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Ryan", coach.ID)
	token := env.coachServiceToken(t, coach.ID)
	path := fmt.Sprintf("/api/athletes/%d/coach/exercises", athlete.ID)

	body := `{"name":"Back Squat","aliases":["Barbell Squat"],"equipment":["Barbell"],"primary_muscles":["quads"],"unilateral":false,"notes":"brace"}`
	rr := env.serveBearer(t, "POST", path, token, body, nil)
	requireStatus(t, rr, http.StatusCreated)
	var created CoachExerciseResponse
	decodeJSON(t, rr, &created)
	if !created.Created || created.Name != "Back Squat" || len(created.Aliases) != 1 || created.Aliases[0] != "Barbell Squat" || created.Notes == nil || *created.Notes != "brace" || created.Unilateral {
		t.Fatalf("created = %+v", created)
	}
	if len(created.Equipment) != 1 || created.Equipment[0] != "Barbell" || len(created.PrimaryMuscles) != 1 || created.PrimaryMuscles[0] != "quads" {
		t.Fatalf("catalog fields = %+v", created)
	}

	rr = env.serveBearer(t, "POST", path, token, `{"name":"back squat"}`, nil)
	requireStatus(t, rr, http.StatusOK)
	var again CoachExerciseResponse
	decodeJSON(t, rr, &again)
	if again.Created || again.ID != created.ID || again.Notes == nil || *again.Notes != "brace" {
		t.Fatalf("name dedupe = %+v", again)
	}

	rr = env.serveBearer(t, "POST", path, token, `{"name":"Safety Squat","aliases":["BARBELL SQUAT"]}`, nil)
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &again)
	if again.ID != created.ID || again.Created {
		t.Fatalf("alias dedupe = %+v", again)
	}

	rr = env.serveBearer(t, "POST", path, token, `{"name":"Front Squat"}`, nil)
	requireStatus(t, rr, http.StatusCreated)
	var front CoachExerciseResponse
	decodeJSON(t, rr, &front)

	rr = env.serveBearer(t, "POST", path, token, `{"name":"Back Squat","aliases":["Front Squat"]}`, nil)
	requireStatus(t, rr, http.StatusConflict)

	var n int
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM exercises`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("exercise count = %d, want 2", n)
	}

	rr = env.serveBearer(t, "GET", path+"?q=barbell", token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var listed []CoachExerciseResponse
	decodeJSON(t, rr, &listed)
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("search = %+v", listed)
	}
	_ = front
}

func TestCoachProgram_PreviewWritesNothing(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Ryan", coach.ID)
	token := env.coachServiceToken(t, coach.ID)
	before := programCounts(t, env)

	rr := env.serveBearer(t, "POST", fmt.Sprintf("/api/athletes/%d/coach/programs/preview", athlete.ID), token, previewBody(athlete.ID, `{"trainerize_id":"abc",`+day1ProgramJSON[1:]), nil)
	requireStatus(t, rr, http.StatusOK)
	var preview CoachProgramPreviewResponse
	decodeJSON(t, rr, &preview)
	if preview.PreviewToken == "" || len(preview.WouldCreate) != 10 || len(preview.Matched) != 0 {
		t.Fatalf("preview matches = %+v create %+v", preview.Matched, preview.WouldCreate)
	}
	if len(preview.Unmapped) != 1 || preview.Unmapped[0] != "program.trainerize_id" {
		t.Fatalf("unmapped = %+v", preview.Unmapped)
	}
	if programCounts(t, env) != before {
		t.Fatalf("preview wrote rows: before %+v after %+v", before, programCounts(t, env))
	}
}

func TestCoachProgram_Day1RoundTrip(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Ryan", coach.ID)
	token := env.coachServiceToken(t, coach.ID)
	base := fmt.Sprintf("/api/athletes/%d/coach", athlete.ID)

	rr := env.serveBearer(t, "POST", base+"/programs/preview", token, previewBody(athlete.ID, day1ProgramJSON), nil)
	requireStatus(t, rr, http.StatusOK)
	var preview CoachProgramPreviewResponse
	decodeJSON(t, rr, &preview)
	if len(preview.Unmapped) != 0 || len(preview.WouldCreate) != 10 {
		t.Fatalf("preview = unmapped %v create %v", preview.Unmapped, preview.WouldCreate)
	}
	assertDay1(t, preview.Normalized.Days[0], false)
	if preview.Normalized.Cycle == nil || preview.Normalized.Cycle.Upcoming == nil || *preview.Normalized.Cycle.Upcoming != 6 {
		t.Fatalf("cycle = %+v", preview.Normalized.Cycle)
	}
	if preview.Normalized.StartDate != nil || preview.Normalized.Weeks != nil {
		t.Fatalf("program header = %+v", preview.Normalized)
	}

	before := programCounts(t, env)
	rr = env.serveBearer(t, "POST", base+"/programs/import", token, `{"preview_token":"`+preview.PreviewToken+`","confirm":false}`, nil)
	requireStatus(t, rr, http.StatusBadRequest)
	if programCounts(t, env) != before {
		t.Fatal("confirm false wrote rows")
	}

	rr = env.serveBearer(t, "POST", base+"/programs/import", token, `{"preview_token":"`+preview.PreviewToken+`","confirm":true}`, nil)
	requireStatus(t, rr, http.StatusCreated)
	var imported CoachProgramImportResponse
	decodeJSON(t, rr, &imported)
	if !imported.Locked || imported.Version != 1 || imported.LineageID != nil || len(imported.CreatedExercises) != 10 {
		t.Fatalf("import = %+v", imported)
	}
	assertDay1(t, imported.Program.Days[0], true)

	rr = env.serveBearer(t, "POST", base+"/programs/import", token, `{"preview_token":"`+preview.PreviewToken+`","confirm":true}`, nil)
	requireStatus(t, rr, http.StatusBadRequest)

	var dayRows int
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM program_days`).Scan(&dayRows); err != nil {
		t.Fatal(err)
	}
	if dayRows != 1 {
		t.Fatalf("stored days = %d, want 1", dayRows)
	}

	rr = env.serveBearer(t, "PUT", fmt.Sprintf("/api/programs/%d", imported.TemplateID), token, `{"name":"Changed","num_weeks":1,"num_days":1}`, nil)
	requireStatus(t, rr, http.StatusConflict)
	var name string
	if err := env.DB.QueryRow(`SELECT name FROM program_templates WHERE id = ?`, imported.TemplateID).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "Phil Full Body" {
		t.Fatalf("locked name = %q", name)
	}

	target, weekStart, today := nextTrainingWeek(t)
	rr = env.serveBearer(t, "POST", base+"/programs/assign", token, fmt.Sprintf(`{"template_id":%d,"version":1,"start_date":"%s"}`, imported.TemplateID, target), nil)
	requireStatus(t, rr, http.StatusCreated)
	var assigned CoachProgramAssignResponse
	decodeJSON(t, rr, &assigned)
	if assigned.Version != 1 || len(assigned.ScheduleWeekdays) != 3 || assigned.ScheduleWeekdays[0] != 1 || assigned.ScheduleWeekdays[2] != 5 {
		t.Fatalf("assign = %+v", assigned)
	}

	rr = env.serveBearer(t, "GET", base+"/programs", token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var listed []CoachProgramSummaryResponse
	decodeJSON(t, rr, &listed)
	if len(listed) != 1 || listed[0].AssignmentID == nil || listed[0].Schedule == nil || listed[0].Schedule.Upcoming == nil || *listed[0].Schedule.Upcoming != 6 {
		t.Fatalf("list = %+v", listed)
	}
	assertDay1(t, listed[0].Days[0], true)

	rr = env.serveBearer(t, "GET", base+"/position", token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var position CoachPositionResponse
	decodeJSON(t, rr, &position)
	if position.Block != nil || position.NextSession == nil || position.NextSession.Date != target {
		t.Fatalf("position next = %+v", position.NextSession)
	}
	if position.NextSession.ProgramDay == nil || position.NextSession.ProgramDayLabel != "Day 1 | Full Body" {
		t.Fatalf("next label = %+v", position.NextSession)
	}
	assertDay1(t, *position.NextSession.ProgramDay, true)
	if position.ProgramDay == nil || position.ProgramDay.Instructions == nil || *position.ProgramDay.Instructions != day1Instructions {
		t.Fatalf("position day instructions = %+v", position.ProgramDay)
	}
	if len(position.NextSession.Exercises) != 10 {
		t.Fatalf("next exercises = %d", len(position.NextSession.Exercises))
	}
	pull := findLine(t, position.NextSession.ProgramDay, "Pull Up")
	prescribed := position.NextSession.Exercises
	var pullTarget *CoachTargetExerciseResponse
	for i := range prescribed {
		if prescribed[i].Name == "Pull Up" {
			pullTarget = &prescribed[i]
		}
	}
	if pullTarget == nil || pullTarget.Prescription == nil || pullTarget.Prescription.Notes == nil || *pullTarget.Prescription.Notes != "Use assistance if necessary" {
		t.Fatalf("pull prescription = %+v", pullTarget)
	}
	if pull.Notes == nil || *pull.Notes != "Use assistance if necessary" {
		t.Fatalf("pull line = %+v", pull)
	}

	rr = env.serveBearer(t, "GET", base+"/week?week_start="+weekStart, token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var week CoachWeekResponse
	decodeJSON(t, rr, &week)
	if len(week.Days) != 3 || week.Timezone != "America/New_York" {
		t.Fatalf("week = %+v", week)
	}
	for _, day := range week.Days {
		if day.ProgramDay == nil || day.ProgramDay.Label != "Day 1 | Full Body" || day.ProgramDayLabel == nil || *day.ProgramDayLabel != "Day 1 | Full Body" {
			t.Fatalf("week day = %+v", day)
		}
		if day.Date < today && day.Status != "missed" {
			t.Fatalf("past day status = %+v", day)
		}
		if day.Date >= today && day.Status != "upcoming" {
			t.Fatalf("future day status = %+v", day)
		}
		assertDay1(t, *day.ProgramDay, true)
	}

	rr = env.serveBearer(t, "POST", fmt.Sprintf("%s/programs/%d/gap-fill", base, imported.TemplateID), token, `{"day_order":1,"block_order":2,"exercise_order":1,"load":185}`, nil)
	requireStatus(t, rr, http.StatusOK)
	var filled CoachLineResponse
	decodeJSON(t, rr, &filled)
	if filled.Name != "Back Squat" || filled.RepsMin == nil || *filled.RepsMin != 5 || filled.RepsMax == nil || *filled.RepsMax != 8 || filled.AI == nil || filled.AI.Load == nil || filled.AI.Load.Value == nil || *filled.AI.Load.Value != 185 || filled.AI.Load.Unit != "lb" {
		t.Fatalf("gap-fill = %+v", filled)
	}

	rr = env.serveBearer(t, "POST", fmt.Sprintf("%s/programs/%d/gap-fill", base, imported.TemplateID), token, `{"day_order":1,"block_order":2,"exercise_order":3,"notes":"changed"}`, nil)
	requireStatus(t, rr, http.StatusConflict)
	rr = env.serveBearer(t, "POST", fmt.Sprintf("%s/programs/%d/gap-fill", base, imported.TemplateID), token, `{"day_order":1,"block_order":2,"exercise_order":1,"reps_min":1,"load":999}`, nil)
	requireStatus(t, rr, http.StatusConflict)
	rr = env.serveBearer(t, "POST", fmt.Sprintf("%s/programs/%d/gap-fill", base, imported.TemplateID), token, `{"day_order":1,"block_order":2,"exercise_order":1,"exercise":"Front Squat"}`, nil)
	requireStatus(t, rr, http.StatusConflict)

	rr = env.serveBearer(t, "GET", base+"/position", token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &position)
	squat := findLine(t, position.NextSession.ProgramDay, "Back Squat")
	if squat.AI == nil || squat.AI.Load == nil || squat.AI.Load.Value == nil || *squat.AI.Load.Value != 185 || squat.RepsMin == nil || *squat.RepsMin != 5 {
		t.Fatalf("squat after refused gap-fill = %+v", squat)
	}
	pull = findLine(t, position.NextSession.ProgramDay, "Pull Up")
	if pull.Notes == nil || *pull.Notes != "Use assistance if necessary" || pull.AI != nil {
		t.Fatalf("pull notes changed = %+v", pull)
	}
}

func TestCoachProgram_NewVersionDoesNotOverwrite(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Ryan", coach.ID)
	token := env.coachServiceToken(t, coach.ID)
	base := fmt.Sprintf("/api/athletes/%d/coach", athlete.ID)

	first := importProgram(t, env, base, token, athlete.ID, day1ProgramJSON)
	secondJSON := strings.Replace(day1ProgramJSON, "Later exercis...", "Later exercis... version two", 1)
	second := importProgram(t, env, base, token, athlete.ID, secondJSON)
	if second.TemplateID == first.TemplateID || second.Version != 2 || second.LineageID == nil || *second.LineageID != first.TemplateID {
		t.Fatalf("second = %+v first %d", second, first.TemplateID)
	}
	var v1 string
	if err := env.DB.QueryRow(`SELECT instructions FROM program_days WHERE template_id = ?`, first.TemplateID).Scan(&v1); err != nil {
		t.Fatal(err)
	}
	if v1 != day1Instructions {
		t.Fatalf("version 1 instructions changed: %q", v1)
	}
	if second.Program.Days[0].Instructions == nil || *second.Program.Days[0].Instructions == v1 {
		t.Fatalf("version 2 did not keep its own instructions")
	}
}

func TestCoachProgram_Scope(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	other := env.createUser(t, "other", true, false)
	admin := env.createUser(t, "admin", false, true)
	athlete := env.createAthlete(t, "Ryan", coach.ID)
	theirs := env.createAthlete(t, "Someone", other.ID)
	kid := env.createUser(t, "kid", false, false)
	if _, err := env.DB.ExecContext(context.Background(), `UPDATE users SET athlete_id = ? WHERE id = ?`, athlete.ID, kid.ID); err != nil {
		t.Fatal(err)
	}
	token := env.coachServiceToken(t, coach.ID)
	otherToken := env.coachServiceToken(t, other.ID)
	adminToken := env.coachServiceToken(t, admin.ID)
	kidToken := env.coachServiceToken(t, kid.ID)

	imported := importProgram(t, env, fmt.Sprintf("/api/athletes/%d/coach", athlete.ID), token, athlete.ID, day1ProgramJSON)

	rr := env.serveBearer(t, "GET", fmt.Sprintf("/api/athletes/%d/coach/programs", theirs.ID), token, nil, nil)
	requireStatus(t, rr, http.StatusForbidden)
	rr = env.serveBearer(t, "POST", fmt.Sprintf("/api/athletes/%d/coach/programs/preview", theirs.ID), token, previewBody(theirs.ID, day1ProgramJSON), nil)
	requireStatus(t, rr, http.StatusForbidden)
	rr = env.serveBearer(t, "POST", fmt.Sprintf("/api/athletes/%d/coach/programs/import", theirs.ID), token, `{"preview_token":"nope","confirm":true}`, nil)
	requireStatus(t, rr, http.StatusForbidden)
	rr = env.serveBearer(t, "GET", fmt.Sprintf("/api/athletes/%d/coach/programs", athlete.ID), otherToken, nil, nil)
	requireStatus(t, rr, http.StatusForbidden)
	rr = env.serveBearer(t, "POST", fmt.Sprintf("/api/athletes/%d/coach/programs/assign", theirs.ID), token, fmt.Sprintf(`{"template_id":%d,"version":1,"start_date":"2026-11-23"}`, imported.TemplateID), nil)
	requireStatus(t, rr, http.StatusForbidden)
	rr = env.serveBearer(t, "GET", "/api/athletes/999999/coach/programs", adminToken, nil, nil)
	requireStatus(t, rr, http.StatusNotFound)
	rr = env.serveBearer(t, "GET", fmt.Sprintf("/api/athletes/%d/coach/programs", athlete.ID), adminToken, nil, nil)
	requireStatus(t, rr, http.StatusForbidden)
	rr = env.serveBearer(t, "GET", fmt.Sprintf("/api/athletes/%d/coach/programs", athlete.ID), kidToken, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	rr = env.serveBearer(t, "POST", fmt.Sprintf("/api/athletes/%d/coach/programs/import", athlete.ID), kidToken, `{"preview_token":"nope","confirm":true}`, nil)
	requireStatus(t, rr, http.StatusForbidden)
	rr = env.serveBearer(t, "GET", fmt.Sprintf("/api/athletes/%d/coach/programs", theirs.ID), kidToken, nil, nil)
	requireStatus(t, rr, http.StatusForbidden)
	rr = env.serveBearer(t, "POST", fmt.Sprintf("/api/athletes/%d/coach/exercises", theirs.ID), token, `{"name":"Nope"}`, nil)
	requireStatus(t, rr, http.StatusForbidden)
}

func previewBody(athleteID int64, program string) string {
	return fmt.Sprintf(`{"athlete_id":%d,"program":%s}`, athleteID, program)
}

func importProgram(t *testing.T, env *testEnv, base, token string, athleteID int64, program string) CoachProgramImportResponse {
	t.Helper()
	rr := env.serveBearer(t, "POST", base+"/programs/preview", token, previewBody(athleteID, program), nil)
	requireStatus(t, rr, http.StatusOK)
	var preview CoachProgramPreviewResponse
	decodeJSON(t, rr, &preview)
	rr = env.serveBearer(t, "POST", base+"/programs/import", token, `{"preview_token":"`+preview.PreviewToken+`","confirm":true}`, nil)
	requireStatus(t, rr, http.StatusCreated)
	var imported CoachProgramImportResponse
	decodeJSON(t, rr, &imported)
	return imported
}

type tableCounts struct{ exercises, templates, days, imports int }

func programCounts(t *testing.T, env *testEnv) tableCounts {
	t.Helper()
	var c tableCounts
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM exercises`).Scan(&c.exercises); err != nil {
		t.Fatal(err)
	}
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM program_templates`).Scan(&c.templates); err != nil {
		t.Fatal(err)
	}
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM program_days`).Scan(&c.days); err != nil {
		t.Fatal(err)
	}
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM program_imports`).Scan(&c.imports); err != nil {
		t.Fatal(err)
	}
	return c
}

func assertDay1(t *testing.T, day CoachDayResponse, withIDs bool) {
	t.Helper()
	if day.Label != "Day 1 | Full Body" || day.Kind == nil || *day.Kind != "Regular" || day.EstimatedMinutes == nil || *day.EstimatedMinutes != 44 || day.ExerciseCount != 10 {
		t.Fatalf("day header = %+v", day)
	}
	if day.Weekday != nil {
		t.Fatalf("day weekday = %v, want null", day.Weekday)
	}
	if day.Instructions == nil || *day.Instructions != day1Instructions {
		t.Fatalf("instructions = %q", deref(day.Instructions))
	}
	wantEquip := []string{"Bench", "Body Weight", "Dumbbell", "Pull-Up Bar"}
	if len(day.Equipment) != len(wantEquip) {
		t.Fatalf("equipment = %+v", day.Equipment)
	}
	for i, name := range wantEquip {
		if day.Equipment[i] != name {
			t.Fatalf("equipment = %+v", day.Equipment)
		}
	}
	if day.Schedule != nil && (day.Schedule.Upcoming == nil || *day.Schedule.Upcoming != 6 || len(day.Schedule.Weekdays) != 3) {
		t.Fatalf("schedule = %+v", day.Schedule)
	}
	if len(day.Blocks) != 4 {
		t.Fatalf("blocks = %d", len(day.Blocks))
	}
	circuit := day.Blocks[0]
	if circuit.Type != "circuit" || circuit.Rounds == nil || *circuit.Rounds != 2 || circuit.Sets != nil || !circuit.NonWorking || circuit.RestSeconds != nil {
		t.Fatalf("circuit = %+v", circuit)
	}
	assertLine(t, circuit.Exercises[0], "Cat to Cow", 2, 10, 10, false, nil, "", true, withIDs)
	assertLine(t, circuit.Exercises[1], "Adductor Rock Back", 2, 10, 10, true, nil, "", true, withIDs)
	assertLine(t, circuit.Exercises[2], "Shoulder Hovers", 2, 5, 5, false, nil, "", true, withIDs)

	straight := day.Blocks[1]
	if straight.Type != "straight" || straight.Rounds != nil || straight.Sets != nil || straight.NonWorking || straight.RestSeconds != nil {
		t.Fatalf("straight = %+v", straight)
	}
	assertLine(t, straight.Exercises[0], "Back Squat", 3, 5, 8, false, intPtr(120), "2m", false, withIDs)
	if straight.Exercises[0].RIRMin == nil || *straight.Exercises[0].RIRMin != 1 || straight.Exercises[0].RIRMax == nil || *straight.Exercises[0].RIRMax != 2 {
		t.Fatalf("squat rir = %+v", straight.Exercises[0])
	}
	assertLine(t, straight.Exercises[1], "Flat Barbell Bench Press", 3, 5, 8, false, intPtr(90), "90s", false, withIDs)
	assertLine(t, straight.Exercises[2], "Pull Up", 3, 5, 8, false, intPtr(90), "90s", false, withIDs)
	if straight.Exercises[2].Notes == nil || *straight.Exercises[2].Notes != "Use assistance if necessary" {
		t.Fatalf("pull notes = %+v", straight.Exercises[2].Notes)
	}

	super2 := day.Blocks[2]
	if super2.Type != "superset" || super2.Sets == nil || *super2.Sets != 2 || super2.Rounds != nil || super2.RestSeconds != nil {
		t.Fatalf("superset 2 = %+v", super2)
	}
	assertLine(t, super2.Exercises[0], "DB Lateral Raise", 2, 8, 15, false, nil, "", false, withIDs)
	assertLine(t, super2.Exercises[1], "DB Hammer Curl", 2, 8, 15, false, nil, "", false, withIDs)

	super3 := day.Blocks[3]
	if super3.Type != "superset" || super3.Sets == nil || *super3.Sets != 3 || super3.RestSeconds == nil || *super3.RestSeconds != 90 || super3.Rest == nil || *super3.Rest != "90s" {
		t.Fatalf("superset 3 = %+v", super3)
	}
	assertLine(t, super3.Exercises[0], "Single Leg Calf Raise", 3, 8, 15, true, nil, "", false, withIDs)
	assertLine(t, super3.Exercises[1], "Kelso Shrug", 3, 8, 15, false, nil, "", false, withIDs)
	if super3.Exercises[0].RIRMin == nil || *super3.Exercises[0].RIRMin != 1 {
		t.Fatalf("calf rir = %+v", super3.Exercises[0])
	}
}

func assertLine(t *testing.T, line CoachLineResponse, name string, sets, repsMin, repsMax int, perSide bool, rest *int, restText string, nonWorking, withIDs bool) {
	t.Helper()
	if line.Name != name || line.Sets == nil || *line.Sets != sets || line.RepsMin == nil || *line.RepsMin != repsMin || line.RepsMax == nil || *line.RepsMax != repsMax || line.PerSide != perSide || line.NonWorking != nonWorking {
		t.Fatalf("%s = %+v", name, line)
	}
	if rest == nil {
		if line.RestSeconds != nil || line.Rest != nil {
			t.Fatalf("%s rest = %v %v", name, line.RestSeconds, line.Rest)
		}
	} else if line.RestSeconds == nil || *line.RestSeconds != *rest || line.Rest == nil || *line.Rest != restText {
		t.Fatalf("%s rest = %v %v", name, line.RestSeconds, line.Rest)
	}
	if withIDs && line.ExerciseID == 0 {
		t.Fatalf("%s missing exercise id", name)
	}
}

func findLine(t *testing.T, day *CoachDayResponse, name string) CoachLineResponse {
	t.Helper()
	if day == nil {
		t.Fatal("missing day")
	}
	for _, block := range day.Blocks {
		for _, line := range block.Exercises {
			if line.Name == name {
				return line
			}
		}
	}
	t.Fatalf("missing %s", name)
	return CoachLineResponse{}
}

func nextTrainingWeek(t *testing.T) (target, weekStart, today string) {
	t.Helper()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().In(loc)
	today = now.Format("2006-01-02")
	for i := 0; i < 7; i++ {
		day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, i)
		iso := int(day.Weekday())
		if iso == 0 {
			iso = 7
		}
		if iso == 1 || iso == 3 || iso == 5 {
			target = day.Format("2006-01-02")
			weekStart = day.AddDate(0, 0, -(iso - 1)).Format("2006-01-02")
			return target, weekStart, today
		}
	}
	t.Fatal("no training day")
	return "", "", ""
}

func intPtr(v int) *int { return &v }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
