package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func aliasCount(t *testing.T, env *testEnv) int {
	t.Helper()
	var n int
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM exercise_aliases`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func exerciseName(t *testing.T, env *testEnv, id int64) string {
	t.Helper()
	var name string
	if err := env.DB.QueryRow(`SELECT name FROM exercises WHERE id = ?`, id).Scan(&name); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestCoachExerciseAlias_PreviewThenConfirm(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Ryan", coach.ID)
	token := env.coachServiceToken(t, coach.ID)
	back := env.createExercise(t, "Back Squat")
	front := env.createExercise(t, "Front Squat")
	base := fmt.Sprintf("/api/athletes/%d/coach/exercises/%d/aliases", athlete.ID, back.ID)

	rr := env.serveBearer(t, "POST", base+"/preview", token, `{"alias":"Squat"}`, nil)
	requireStatus(t, rr, http.StatusOK)
	var preview CoachExerciseChangePreviewResponse
	decodeJSON(t, rr, &preview)
	if preview.Action != "add_alias" || preview.ExerciseID != back.ID || preview.CurrentName != "Back Squat" || deref(preview.Alias) != "Squat" || len(preview.AliasesAfter) != 1 || !strings.HasPrefix(preview.PreviewToken, "rlex_") {
		t.Fatalf("preview = %+v", preview)
	}
	if n := aliasCount(t, env); n != 0 {
		t.Fatalf("preview wrote %d aliases", n)
	}

	rr = env.serveBearer(t, "POST", base+"/confirm", token, `{"preview_token":"`+preview.PreviewToken+`"}`, nil)
	requireStatus(t, rr, http.StatusBadRequest) // confirm must be true
	rr = env.serveBearer(t, "POST", base+"/confirm", token, `{"preview_token":"`+preview.PreviewToken+`","confirm":true}`, nil)
	requireStatus(t, rr, http.StatusOK)
	var row CoachExerciseResponse
	decodeJSON(t, rr, &row)
	if row.ID != back.ID || row.Name != "Back Squat" || len(row.Aliases) != 1 || row.Aliases[0] != "Squat" {
		t.Fatalf("confirmed = %+v", row)
	}
	rr = env.serveBearer(t, "POST", base+"/confirm", token, `{"preview_token":"`+preview.PreviewToken+`","confirm":true}`, nil)
	requireStatus(t, rr, http.StatusBadRequest) // one use

	// Conflicts, by key: another exercise's name, this exercise's name, an existing alias.
	for _, tc := range []struct {
		alias string
		want  int
	}{
		{"front-squat", http.StatusConflict},
		{"BACK squat!", http.StatusBadRequest},
		{"squat.", http.StatusBadRequest},
		{"   ", http.StatusBadRequest},
		{"--", http.StatusBadRequest},
	} {
		rr = env.serveBearer(t, "POST", base+"/preview", token, fmt.Sprintf(`{"alias":%q}`, tc.alias), nil)
		requireStatus(t, rr, tc.want)
	}
	// Front Squat cannot take "Squat": it is Back Squat's alias.
	rr = env.serveBearer(t, "POST", fmt.Sprintf("/api/athletes/%d/coach/exercises/%d/aliases/preview", athlete.ID, front.ID), token, `{"alias":"SQUAT"}`, nil)
	requireStatus(t, rr, http.StatusConflict)
	if !strings.Contains(rr.Body.String(), "Back Squat") {
		t.Fatalf("conflict should name the exercise: %s", rr.Body.String())
	}
	rr = env.serveBearer(t, "POST", base+"/preview", token, `{"alias":"Squat","extra":1}`, nil)
	requireStatus(t, rr, http.StatusBadRequest)
	rr = env.serveBearer(t, "POST", fmt.Sprintf("/api/athletes/%d/coach/exercises/99999/aliases/preview", athlete.ID), token, `{"alias":"Zercher"}`, nil)
	requireStatus(t, rr, http.StatusNotFound)

	// A catalog edit cannot take a name that is another exercise's alias.
	if _, err := models.CreateExercise(context.Background(), env.DB, "squat", "", "", "", 0); err == nil {
		t.Fatal("CreateExercise accepted a name that is an alias")
	}
	if _, err := models.UpdateExercise(context.Background(), env.DB, front.ID, "Squat", "", "", "", 0); err == nil {
		t.Fatal("UpdateExercise accepted a name that is an alias")
	}
}

func TestCoachExerciseRename_KeepsHistoryAndOldName(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Ryan", coach.ID)
	token := env.coachServiceToken(t, coach.ID)
	squat := env.createExercise(t, "Squat")
	env.createExercise(t, "Front Squat")
	workout := env.createWorkout(t, athlete.ID, "2026-11-02")
	set, err := models.AddSet(context.Background(), env.DB, workout.ID, squat.ID, 5, 225, 0, "reps", "", "")
	if err != nil {
		t.Fatal(err)
	}
	base := fmt.Sprintf("/api/athletes/%d/coach/exercises/%d/rename", athlete.ID, squat.ID)

	rr := env.serveBearer(t, "POST", base+"/preview", token, `{"name":"Back Squat"}`, nil)
	requireStatus(t, rr, http.StatusOK)
	var preview CoachExerciseChangePreviewResponse
	decodeJSON(t, rr, &preview)
	if preview.Action != "rename" || preview.CurrentName != "Squat" || preview.NewName != "Back Squat" || deref(preview.KeptAlias) != "Squat" || preview.RemovedAlias != nil {
		t.Fatalf("preview = %+v", preview)
	}
	if exerciseName(t, env, squat.ID) != "Squat" || aliasCount(t, env) != 0 {
		t.Fatal("preview wrote")
	}
	// A rename token does not confirm an alias, or another exercise.
	rr = env.serveBearer(t, "POST", fmt.Sprintf("/api/athletes/%d/coach/exercises/%d/aliases/confirm", athlete.ID, squat.ID), token, `{"preview_token":"`+preview.PreviewToken+`","confirm":true}`, nil)
	requireStatus(t, rr, http.StatusBadRequest)

	rr = env.serveBearer(t, "POST", base+"/confirm", token, `{"preview_token":"`+preview.PreviewToken+`","confirm":true}`, nil)
	requireStatus(t, rr, http.StatusOK)
	var row CoachExerciseResponse
	decodeJSON(t, rr, &row)
	if row.ID != squat.ID || row.Name != "Back Squat" || len(row.Aliases) != 1 || row.Aliases[0] != "Squat" {
		t.Fatalf("renamed = %+v", row)
	}
	var setExercise int64
	var sets int
	if err := env.DB.QueryRow(`SELECT exercise_id FROM workout_sets WHERE id = ?`, set.ID).Scan(&setExercise); err != nil {
		t.Fatal(err)
	}
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM workout_sets`).Scan(&sets); err != nil {
		t.Fatal(err)
	}
	if setExercise != squat.ID || sets != 1 {
		t.Fatalf("logged set moved: exercise %d, %d sets", setExercise, sets)
	}

	// list_exercises finds it by the old name and returns the alias.
	rr = env.serveBearer(t, "GET", fmt.Sprintf("/api/athletes/%d/coach/exercises?q=squat", athlete.ID), token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var listed []CoachExerciseResponse
	decodeJSON(t, rr, &listed)
	found := false
	for _, e := range listed {
		if e.ID == squat.ID && len(e.Aliases) == 1 && e.Aliases[0] == "Squat" {
			found = true
		}
	}
	if !found {
		t.Fatalf("list = %+v", listed)
	}

	// Renaming back onto its own alias drops that alias and keeps the old name.
	rr = env.serveBearer(t, "POST", base+"/preview", token, `{"name":"squat"}`, nil)
	requireStatus(t, rr, http.StatusOK)
	preview = CoachExerciseChangePreviewResponse{}
	decodeJSON(t, rr, &preview)
	if deref(preview.RemovedAlias) != "Squat" || deref(preview.KeptAlias) != "Back Squat" {
		t.Fatalf("own-alias rename = %+v", preview)
	}

	// A new name that is another exercise's name, by key, is refused.
	rr = env.serveBearer(t, "POST", base+"/preview", token, `{"name":"front-squat"}`, nil)
	requireStatus(t, rr, http.StatusConflict)

	// A case or punctuation fix does not keep the old spelling as an alias.
	pull := env.createExercise(t, "Pull-up")
	rr = env.serveBearer(t, "POST", fmt.Sprintf("/api/athletes/%d/coach/exercises/%d/rename/preview", athlete.ID, pull.ID), token, `{"name":"Pull Up"}`, nil)
	requireStatus(t, rr, http.StatusOK)
	preview = CoachExerciseChangePreviewResponse{}
	decodeJSON(t, rr, &preview)
	if preview.KeptAlias != nil || len(preview.AliasesAfter) != 0 {
		t.Fatalf("punctuation rename = %+v", preview)
	}
}

func TestCoachExerciseChange_StalePreviewAndAccess(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	other := env.createUser(t, "other", true, false)
	athlete := env.createAthlete(t, "Ryan", coach.ID)
	token := env.coachServiceToken(t, coach.ID)
	otherToken := env.coachServiceToken(t, other.ID)
	ex := env.createExercise(t, "Romanian Deadlift")
	env.createExercise(t, "Stiff Leg Deadlift")
	base := fmt.Sprintf("/api/athletes/%d/coach/exercises/%d", athlete.ID, ex.ID)

	// Another coach cannot preview a change through an athlete they do not coach.
	rr := env.serveBearer(t, "POST", base+"/aliases/preview", otherToken, `{"alias":"RDL"}`, nil)
	requireStatus(t, rr, http.StatusForbidden)

	rr = env.serveBearer(t, "POST", base+"/aliases/preview", token, `{"alias":"RDL"}`, nil)
	requireStatus(t, rr, http.StatusOK)
	var preview CoachExerciseChangePreviewResponse
	decodeJSON(t, rr, &preview)
	// The other coach cannot spend this token.
	rr = env.serveBearer(t, "POST", base+"/aliases/confirm", otherToken, `{"preview_token":"`+preview.PreviewToken+`","confirm":true}`, nil)
	requireStatus(t, rr, http.StatusForbidden)

	// The alias was taken after the preview: confirm re-checks and refuses.
	if _, err := env.DB.Exec(`INSERT INTO exercise_aliases (exercise_id, alias) SELECT id, 'R.D.L.' FROM exercises WHERE name = 'Stiff Leg Deadlift'`); err != nil {
		t.Fatal(err)
	}
	rr = env.serveBearer(t, "POST", base+"/aliases/confirm", token, `{"preview_token":"`+preview.PreviewToken+`","confirm":true}`, nil)
	requireStatus(t, rr, http.StatusConflict)

	// Renamed after the preview: confirm refuses.
	rr = env.serveBearer(t, "POST", base+"/rename/preview", token, `{"name":"RDL Barbell"}`, nil)
	requireStatus(t, rr, http.StatusOK)
	preview = CoachExerciseChangePreviewResponse{}
	decodeJSON(t, rr, &preview)
	if _, err := env.DB.Exec(`UPDATE exercises SET name = 'Romanian DL' WHERE id = ?`, ex.ID); err != nil {
		t.Fatal(err)
	}
	rr = env.serveBearer(t, "POST", base+"/rename/confirm", token, `{"preview_token":"`+preview.PreviewToken+`","confirm":true}`, nil)
	requireStatus(t, rr, http.StatusConflict)
	if exerciseName(t, env, ex.ID) != "Romanian DL" {
		t.Fatal("stale rename applied")
	}
}

func TestCoachProgram_MatchesByKeyAndSuggests(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Ryan", coach.ID)
	token := env.coachServiceToken(t, coach.ID)
	pull := env.createExercise(t, "Pull Up")
	back := env.createExercise(t, "Back Squat")
	front := env.createExercise(t, "Front Squat")
	if _, err := env.DB.Exec(`INSERT INTO exercise_aliases (exercise_id, alias) VALUES (?, 'Squat')`, back.ID); err != nil {
		t.Fatal(err)
	}
	program := `{"name":"Match","source":"s","author":"a","cycle":{"repeat":"weekly","weekdays":[1],"upcoming":1},
	  "days":[{"label":"D1","order":1,"kind":"Regular","blocks":[{"type":"straight","order":1,"exercises":[
	    {"name":"pull-up","order":1,"sets":3,"reps_min":5,"reps_max":5},
	    {"name":"SQUAT","order":2,"sets":3,"reps_min":5,"reps_max":5},
	    {"name":"Front squat","order":3,"sets":3,"reps_min":5,"reps_max":5},
	    {"name":"Goblet Squat","order":4,"sets":3,"reps_min":5,"reps_max":5},
	    {"name":"Pull-Ups","order":5,"sets":3,"reps_min":5,"reps_max":5}]}]}]}`
	base := fmt.Sprintf("/api/athletes/%d/coach", athlete.ID)
	rr := env.serveBearer(t, "POST", base+"/programs/preview", token, previewBody(athlete.ID, program), nil)
	requireStatus(t, rr, http.StatusOK)
	var preview CoachProgramPreviewResponse
	decodeJSON(t, rr, &preview)
	want := map[string]struct {
		id   int64
		name string
		on   string
	}{
		"pull-up":     {pull.ID, "Pull Up", "normalized_name"},
		"SQUAT":       {back.ID, "Back Squat", "alias"},
		"Front squat": {front.ID, "Front Squat", "name"},
	}
	if len(preview.Matched) != len(want) {
		t.Fatalf("matched = %+v", preview.Matched)
	}
	for _, m := range preview.Matched {
		w, ok := want[m.Name]
		if !ok || m.ExerciseID != w.id || m.ExerciseName != w.name || m.MatchedOn != w.on {
			t.Fatalf("match %+v, want %+v", m, w)
		}
	}
	if strings.Join(preview.WouldCreate, "|") != "Goblet Squat|Pull-Ups" || len(preview.Suggestions) != 2 || len(preview.Ambiguous) != 0 {
		t.Fatalf("would_create = %v suggestions = %+v ambiguous = %+v", preview.WouldCreate, preview.Suggestions, preview.Ambiguous)
	}
	sugg := map[string][]string{}
	for _, s := range preview.Suggestions {
		for _, c := range s.Suggestions {
			sugg[s.Name] = append(sugg[s.Name], c.Name)
		}
	}
	if len(sugg["Pull-Ups"]) == 0 || sugg["Pull-Ups"][0] != "Pull Up" {
		t.Fatalf("Pull-Ups suggestions = %v", sugg["Pull-Ups"])
	}
	goblet := strings.Join(sugg["Goblet Squat"], "|")
	if !strings.Contains(goblet, "Back Squat") || !strings.Contains(goblet, "Front Squat") {
		t.Fatalf("Goblet Squat suggestions = %v", sugg["Goblet Squat"])
	}
	before := programCounts(t, env)
	rr = env.serveBearer(t, "POST", base+"/programs/import", token, `{"preview_token":"`+preview.PreviewToken+`","confirm":true}`, nil)
	requireStatus(t, rr, http.StatusCreated)
	var imported CoachProgramImportResponse
	decodeJSON(t, rr, &imported)
	if strings.Join(imported.CreatedExercises, "|") != "Goblet Squat|Pull-Ups" {
		t.Fatalf("created = %v", imported.CreatedExercises)
	}
	if after := programCounts(t, env); after.exercises != before.exercises+2 {
		t.Fatalf("exercises %d -> %d", before.exercises, after.exercises)
	}
	var squatLine int64
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM program_block_exercises WHERE exercise_id = ?`, back.ID).Scan(&squatLine); err != nil {
		t.Fatal(err)
	}
	if squatLine != 1 {
		t.Fatalf("SQUAT line not on Back Squat: %d", squatLine)
	}
}

func TestCoachProgram_AmbiguousKeyIsRefused(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Ryan", coach.ID)
	token := env.coachServiceToken(t, coach.ID)
	// Legacy rows that differ only in punctuation.
	a := env.createExercise(t, "Pull Up")
	b := env.createExercise(t, "Pull-up")
	program := `{"name":"Amb","source":"s","author":"a","cycle":{"repeat":"weekly","weekdays":[1],"upcoming":1},
	  "days":[{"label":"D1","order":1,"kind":"Regular","blocks":[{"type":"straight","order":1,"exercises":[
	    {"name":"PULLUP","order":1,"sets":3,"reps_min":5,"reps_max":5}]}]}]}`
	base := fmt.Sprintf("/api/athletes/%d/coach", athlete.ID)
	rr := env.serveBearer(t, "POST", base+"/programs/preview", token, previewBody(athlete.ID, program), nil)
	requireStatus(t, rr, http.StatusOK)
	var preview CoachProgramPreviewResponse
	decodeJSON(t, rr, &preview)
	if len(preview.Ambiguous) != 1 || len(preview.Ambiguous[0].Candidates) != 2 || preview.Ambiguous[0].Candidates[0].ExerciseID != a.ID || preview.Ambiguous[0].Candidates[1].ExerciseID != b.ID || len(preview.WouldCreate) != 0 {
		t.Fatalf("preview = %+v", preview)
	}
	before := programCounts(t, env)
	rr = env.serveBearer(t, "POST", base+"/programs/import", token, `{"preview_token":"`+preview.PreviewToken+`","confirm":true}`, nil)
	requireStatus(t, rr, http.StatusConflict)
	if !strings.Contains(rr.Body.String(), "more than one exercise") {
		t.Fatalf("body = %s", rr.Body.String())
	}
	if after := programCounts(t, env); after != before {
		t.Fatalf("import wrote: %+v -> %+v", before, after)
	}
	// An exact name still wins over the key.
	exact := strings.Replace(program, "PULLUP", "pull-up", 1)
	rr = env.serveBearer(t, "POST", base+"/programs/preview", token, previewBody(athlete.ID, exact), nil)
	requireStatus(t, rr, http.StatusOK)
	preview = CoachProgramPreviewResponse{}
	decodeJSON(t, rr, &preview)
	if len(preview.Matched) != 1 || preview.Matched[0].ExerciseID != b.ID || preview.Matched[0].MatchedOn != "name" {
		t.Fatalf("exact = %+v", preview)
	}
}
