package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

// createProgramTemplate is a small fixture for tests that only care about
// existing programs (assignment, list, get, etc.).
func (e *testEnv) createProgramTemplate(t *testing.T, name string, weeks, days int) *models.ProgramTemplate {
	t.Helper()
	p, err := models.CreateProgramTemplate(e.DB, nil, name, "", weeks, days, false, "")
	if err != nil {
		t.Fatalf("create program template %q: %v", name, err)
	}
	return p
}

// --- Program template CRUD ---

func TestCreateProgramTemplate_Coach(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", "/api/programs",
		`{"name":"5/3/1","description":"main lift","num_weeks":3,"num_days":4,"is_loop":true,"audience":"adult"}`,
		cookies)
	requireStatus(t, rr, http.StatusCreated)

	var got ProgramTemplate
	decodeJSON(t, rr, &got)
	if got.Name != "5/3/1" || got.NumWeeks != 3 || got.NumDays != 4 {
		t.Errorf("got %+v, want name=5/3/1 weeks=3 days=4", got)
	}
}

func TestCreateProgramTemplate_NonCoachForbidden(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "athlete_user", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "POST", "/api/programs",
		`{"name":"x","num_weeks":1,"num_days":1}`, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestCreateProgramTemplate_Validation(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)

	cases := []struct {
		name string
		body string
	}{
		{"missing name", `{"num_weeks":3,"num_days":4}`},
		{"missing weeks", `{"name":"x","num_days":4}`},
		{"zero days", `{"name":"x","num_weeks":3,"num_days":0}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := env.do(t, "POST", "/api/programs", tc.body, cookies)
			requireStatus(t, rr, http.StatusBadRequest)
		})
	}
}

func TestListProgramTemplates_Authenticated(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	env.createProgramTemplate(t, "5/3/1", 3, 4)
	env.createProgramTemplate(t, "GZCL", 4, 4)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", "/api/programs", nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got []*ProgramTemplate
	decodeJSON(t, rr, &got)
	if len(got) != 2 {
		t.Errorf("expected 2 templates, got %d", len(got))
	}
}

func TestGetProgramTemplate_Success(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", fmt.Sprintf("/api/programs/%d", tpl.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
}

func TestUpdateProgramTemplate_Coach(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "PUT", fmt.Sprintf("/api/programs/%d", tpl.ID),
		`{"name":"5/3/1 BBB","num_weeks":3,"num_days":4,"is_loop":true}`, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got ProgramTemplate
	decodeJSON(t, rr, &got)
	if got.Name != "5/3/1 BBB" {
		t.Errorf("got name %q, want %q", got.Name, "5/3/1 BBB")
	}
}

func TestUpdateProgramTemplate_NotFound(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "PUT", "/api/programs/9999",
		`{"name":"ghost","num_weeks":1,"num_days":1}`, cookies)
	requireStatus(t, rr, http.StatusNotFound)
}

func TestDeleteProgramTemplate_Coach(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "DELETE", fmt.Sprintf("/api/programs/%d", tpl.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
}

// --- Prescribed sets ---

func TestAddPrescribedSet_DefaultRepType(t *testing.T) {
	// Regression for the same bug class as ef4f7b3 — handler used to default
	// rep_type to "standard" which violated the schema CHECK constraint.
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	exercise := env.createExercise(t, "Bench Press")
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(
		`{"exercise_id":%d,"week":1,"day":1,"set_number":1,"reps":5,"percentage":0.65}`,
		exercise.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/programs/%d/sets", tpl.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)

	var got PrescribedSet
	decodeJSON(t, rr, &got)
	if got.RepType != "reps" {
		t.Errorf("got rep_type=%q, want %q (default for prescribed sets)", got.RepType, "reps")
	}
}

func TestAddPrescribedSet_ExplicitSeconds(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	tpl := env.createProgramTemplate(t, "Conditioning", 1, 1)
	exercise := env.createExercise(t, "Plank")
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(
		`{"exercise_id":%d,"week":1,"day":1,"set_number":1,"reps":60,"rep_type":"seconds"}`,
		exercise.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/programs/%d/sets", tpl.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)

	var got PrescribedSet
	decodeJSON(t, rr, &got)
	if got.RepType != "seconds" {
		t.Errorf("got rep_type=%q, want %q", got.RepType, "seconds")
	}
}

func TestUpdatePrescribedSet_DefaultRepType(t *testing.T) {
	// Regression: UpdatePrescribedSet had no rep_type default; an empty
	// string would trip the CHECK constraint. ef4f7b3 added the default.
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	exercise := env.createExercise(t, "Bench Press")
	cookies := env.loginAs(t, coach)

	// Create a set first.
	body := fmt.Sprintf(
		`{"exercise_id":%d,"week":1,"day":1,"set_number":1,"reps":5,"rep_type":"reps"}`,
		exercise.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/programs/%d/sets", tpl.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)
	var created PrescribedSet
	decodeJSON(t, rr, &created)

	// Update without sending rep_type — handler must default it.
	updateBody := fmt.Sprintf(
		`{"exercise_id":%d,"set_number":1,"reps":3}`, exercise.ID)
	rr = env.do(t, "PUT",
		fmt.Sprintf("/api/programs/%d/sets/%d", tpl.ID, created.ID),
		updateBody, cookies)
	requireStatus(t, rr, http.StatusOK)
}

func TestDeletePrescribedSet_Success(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	exercise := env.createExercise(t, "Squat")
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(
		`{"exercise_id":%d,"week":1,"day":1,"set_number":1,"reps":5}`,
		exercise.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/programs/%d/sets", tpl.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)
	var created PrescribedSet
	decodeJSON(t, rr, &created)

	rr = env.do(t, "DELETE",
		fmt.Sprintf("/api/programs/%d/sets/%d", tpl.ID, created.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusOK)
}

// --- CopyWeek ---

func TestCopyWeek_DuplicatesSets(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	exercise := env.createExercise(t, "Bench Press")
	cookies := env.loginAs(t, coach)

	// Add two sets in week 1.
	for i := 1; i <= 2; i++ {
		body := fmt.Sprintf(
			`{"exercise_id":%d,"week":1,"day":1,"set_number":%d,"reps":5,"rep_type":"reps"}`,
			exercise.ID, i)
		rr := env.do(t, "POST", fmt.Sprintf("/api/programs/%d/sets", tpl.ID), body, cookies)
		requireStatus(t, rr, http.StatusCreated)
	}

	// Copy week 1 → week 2.
	rr := env.do(t, "POST", fmt.Sprintf("/api/programs/%d/copy-week", tpl.ID),
		`{"source_week":1,"target_week":2}`, cookies)
	requireStatus(t, rr, http.StatusOK)
}

func TestCopyWeek_NonCoachForbidden(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "athlete_user", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "POST", "/api/programs/1/copy-week",
		`{"source_week":1,"target_week":2}`, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

// --- Progression rules ---

func TestSetProgressionRule_Success(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	exercise := env.createExercise(t, "Squat")
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(`{"exercise_id":%d,"increment":5}`, exercise.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/programs/%d/rules", tpl.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)

	var got ProgressionRuleResponse
	decodeJSON(t, rr, &got)
	if got.Increment != 5 {
		t.Errorf("got increment=%v, want 5", got.Increment)
	}
}

func TestSetProgressionRule_RequiresExerciseAndIncrement(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	cookies := env.loginAs(t, coach)

	cases := []struct {
		name string
		body string
	}{
		{"missing exercise_id", `{"increment":5}`},
		{"zero increment", `{"exercise_id":1,"increment":0}`},
		{"negative increment", `{"exercise_id":1,"increment":-5}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := env.do(t, "POST", fmt.Sprintf("/api/programs/%d/rules", tpl.ID), tc.body, cookies)
			requireStatus(t, rr, http.StatusBadRequest)
		})
	}
}

func TestListProgressionRules_ReturnsCreated(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	exercise := env.createExercise(t, "Squat")
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(`{"exercise_id":%d,"increment":5}`, exercise.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/programs/%d/rules", tpl.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)

	rr = env.do(t, "GET", fmt.Sprintf("/api/programs/%d/rules", tpl.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got []*ProgressionRuleResponse
	decodeJSON(t, rr, &got)
	if len(got) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(got))
	}
}

func TestDeleteProgressionRule_Success(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	exercise := env.createExercise(t, "Squat")
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(`{"exercise_id":%d,"increment":5}`, exercise.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/programs/%d/rules", tpl.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)
	var rule ProgressionRuleResponse
	decodeJSON(t, rr, &rule)

	rr = env.do(t, "DELETE",
		fmt.Sprintf("/api/programs/%d/rules/%d", tpl.ID, rule.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusOK)
}

// --- Athlete program assignment ---

func TestAssignProgramToAthlete_Success(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(`{"template_id":%d,"start_date":"2026-05-12"}`, tpl.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/programs", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)

	var got AthleteProgram
	decodeJSON(t, rr, &got)
	if got.TemplateID != tpl.ID || !got.Active {
		t.Errorf("got %+v, want template_id=%d active=true", got, tpl.ID)
	}
	if got.StartDate != "2026-05-12" {
		t.Errorf("got start_date=%q, want 2026-05-12 (regression #4)", got.StartDate)
	}
}

func TestAssignProgramToAthlete_AutoDeactivatesPrior(t *testing.T) {
	// Per ADR 010 + the handler comment, assigning a program in a role that
	// already has an active program must auto-deactivate the prior one.
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	first := env.createProgramTemplate(t, "5/3/1", 3, 4)
	second := env.createProgramTemplate(t, "GZCL", 4, 4)
	cookies := env.loginAs(t, coach)

	for _, tpl := range []*models.ProgramTemplate{first, second} {
		body := fmt.Sprintf(`{"template_id":%d,"start_date":"2026-05-12"}`, tpl.ID)
		rr := env.do(t, "POST",
			fmt.Sprintf("/api/athletes/%d/programs", athlete.ID), body, cookies)
		requireStatus(t, rr, http.StatusCreated)
	}

	// List should show 2 programs but only 1 active in the primary role.
	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/programs", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var got []*AthleteProgram
	decodeJSON(t, rr, &got)
	if len(got) != 2 {
		t.Fatalf("expected 2 programs total, got %d", len(got))
	}
	activePrimary := 0
	for _, p := range got {
		if p.Active && p.Role == "primary" {
			activePrimary++
		}
	}
	if activePrimary != 1 {
		t.Errorf("expected exactly 1 active primary program, got %d", activePrimary)
	}
}

func TestAssignProgramToAthlete_OtherCoachForbidden(t *testing.T) {
	env := setupTest(t)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	athleteOfA := env.createAthlete(t, "Alice", coachA.ID)
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	cookies := env.loginAs(t, coachB)

	body := fmt.Sprintf(`{"template_id":%d,"start_date":"2026-05-12"}`, tpl.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/programs", athleteOfA.ID), body, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestAssignProgramToAthlete_RequiresFields(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	cases := []struct {
		name string
		body string
	}{
		{"missing template_id", `{"start_date":"2026-05-12"}`},
		{"missing start_date", `{"template_id":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/programs", athlete.ID), tc.body, cookies)
			requireStatus(t, rr, http.StatusBadRequest)
		})
	}
}

func TestDeactivateAthleteProgram_Success(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(`{"template_id":%d,"start_date":"2026-05-12"}`, tpl.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/programs", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)
	var ap AthleteProgram
	decodeJSON(t, rr, &ap)

	rr = env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/programs/%d/deactivate", athlete.ID, ap.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	// Verify it's now inactive.
	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/programs", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var got []*AthleteProgram
	decodeJSON(t, rr, &got)
	for _, p := range got {
		if p.ID == ap.ID && p.Active {
			t.Error("program should be deactivated")
		}
	}
}

func TestReactivateAthleteProgram_Success(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(`{"template_id":%d,"start_date":"2026-05-12"}`, tpl.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/programs", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)
	var ap AthleteProgram
	decodeJSON(t, rr, &ap)

	rr = env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/programs/%d/deactivate", athlete.ID, ap.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	rr = env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/programs/%d/reactivate", athlete.ID, ap.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusOK)
}

func TestDeleteAthleteProgram_Success(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	tpl := env.createProgramTemplate(t, "5/3/1", 3, 4)
	cookies := env.loginAs(t, coach)

	body := fmt.Sprintf(`{"template_id":%d,"start_date":"2026-05-12"}`, tpl.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/programs", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)
	var ap AthleteProgram
	decodeJSON(t, rr, &ap)

	rr = env.do(t, "DELETE",
		fmt.Sprintf("/api/athletes/%d/programs/%d", athlete.ID, ap.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	// Listing should now be empty.
	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/programs", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var got []*AthleteProgram
	decodeJSON(t, rr, &got)
	if len(got) != 0 {
		t.Errorf("expected 0 programs after delete, got %d", len(got))
	}
}
