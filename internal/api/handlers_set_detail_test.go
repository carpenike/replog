package api

import (
	"fmt"
	"net/http"
	"testing"
)

func findPrescribed(t *testing.T, sets []*PrescribedSet, name string) *PrescribedSet {
	t.Helper()
	for _, s := range sets {
		if s.ExerciseName == name && s.SetNumber == 1 {
			return s
		}
	}
	t.Fatalf("no prescribed set 1 for %q", name)
	return nil
}

// TestImportedProgramCarriesSetDetail checks N1 and N2: an imported program
// keeps its rep ranges, RIR, warm-ups, and superset group keys on the
// prescribed sets, and the template reports locked and its import version.
func TestImportedProgramCarriesSetDetail(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Ryan", coach.ID)
	token := env.coachServiceToken(t, coach.ID)
	base := fmt.Sprintf("/api/athletes/%d/coach", athlete.ID)
	imported := importProgram(t, env, base, token, athlete.ID, day1ProgramJSON)

	cookies := env.loginAs(t, coach)
	rr := env.do(t, "GET", fmt.Sprintf("/api/programs/%d", imported.TemplateID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var detail ProgramTemplateDetailResponse
	decodeJSON(t, rr, &detail)

	if !detail.Program.Locked || detail.Program.ImportVersion == nil || *detail.Program.ImportVersion != 1 {
		t.Fatalf("program = %+v, want locked import version 1", detail.Program)
	}

	cat := findPrescribed(t, detail.Sets, "Cat to Cow")
	if !cat.NonWorking || cat.GroupKey == nil || *cat.GroupKey != "A" || cat.RepsMax != nil {
		t.Fatalf("warm-up circuit set = %+v", cat)
	}
	squat := findPrescribed(t, detail.Sets, "Back Squat")
	if squat.Reps == nil || *squat.Reps != 5 || squat.RepsMax == nil || *squat.RepsMax != 8 ||
		squat.RIRMin == nil || *squat.RIRMin != 1 || squat.RIRMax == nil || *squat.RIRMax != 2 ||
		squat.NonWorking || squat.GroupKey != nil {
		t.Fatalf("squat = %+v", squat)
	}
	raise := findPrescribed(t, detail.Sets, "DB Lateral Raise")
	curl := findPrescribed(t, detail.Sets, "DB Hammer Curl")
	calf := findPrescribed(t, detail.Sets, "Single Leg Calf Raise")
	if raise.GroupKey == nil || *raise.GroupKey != "B" || curl.GroupKey == nil || *curl.GroupKey != "B" || calf.GroupKey == nil || *calf.GroupKey != "C" {
		t.Fatalf("superset keys = %v %v %v", raise.GroupKey, curl.GroupKey, calf.GroupKey)
	}
	if raise.RepsMax == nil || *raise.RepsMax != 15 {
		t.Fatalf("raise range = %+v", raise)
	}

	// A hand-built template is not locked and has no import version.
	hand := env.createProgramTemplate(t, "Hand Built", 1, 1)
	rr = env.do(t, "GET", fmt.Sprintf("/api/programs/%d", hand.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	detail = ProgramTemplateDetailResponse{}
	decodeJSON(t, rr, &detail)
	if detail.Program.Locked || detail.Program.ImportVersion != nil {
		t.Fatalf("hand-built program = %+v", detail.Program)
	}

	// The list endpoint reports the lock as well.
	rr = env.do(t, "GET", "/api/programs", nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var listed []ProgramTemplate
	decodeJSON(t, rr, &listed)
	found := false
	for _, p := range listed {
		if p.ID == imported.TemplateID {
			found = true
			if !p.Locked || p.ImportVersion == nil {
				t.Fatalf("listed imported program = %+v", p)
			}
		}
	}
	if !found {
		t.Fatal("imported program missing from list")
	}
}

// TestHandBuiltSetDetailAndPrescription covers Gap 3 and the prescription
// read: a coach builds grouped, ranged, warm-up sets by hand and the
// prescription returns them.
func TestHandBuiltSetDetailAndPrescription(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Dana", coach.ID)
	squat := env.createExercise(t, "Back Squat")
	row := env.createExercise(t, "Barbell Row")
	tmpl := env.createProgramTemplate(t, "Block", 1, 1)
	cookies := env.loginAs(t, coach)
	setsPath := fmt.Sprintf("/api/programs/%d/sets", tmpl.ID)

	rr := env.do(t, "POST", setsPath, fmt.Sprintf(`{"exercise_id":%d,"week":1,"day":1,"set_number":1,"reps":5,"non_working":true}`, squat.ID), cookies)
	requireStatus(t, rr, http.StatusCreated)
	rr = env.do(t, "POST", setsPath, fmt.Sprintf(`{"exercise_id":%d,"week":1,"day":1,"set_number":2,"reps":8,"reps_max":12,"rir_min":2,"rir_max":3,"group_key":"a","sort_order":1}`, squat.ID), cookies)
	requireStatus(t, rr, http.StatusCreated)
	var created PrescribedSet
	decodeJSON(t, rr, &created)
	if created.GroupKey == nil || *created.GroupKey != "A" || created.RepsMax == nil || *created.RepsMax != 12 {
		t.Fatalf("created = %+v", created)
	}
	rr = env.do(t, "POST", setsPath, fmt.Sprintf(`{"exercise_id":%d,"week":1,"day":1,"set_number":1,"reps":10,"group_key":"A","sort_order":2}`, row.ID), cookies)
	requireStatus(t, rr, http.StatusCreated)

	for _, bad := range []string{
		`{"exercise_id":%d,"week":1,"day":1,"set_number":9,"reps":8,"reps_max":6}`,
		`{"exercise_id":%d,"week":1,"day":1,"set_number":9,"reps":8,"rir_min":11}`,
		`{"exercise_id":%d,"week":1,"day":1,"set_number":9,"reps":8,"rir_min":3,"rir_max":1}`,
		`{"exercise_id":%d,"week":1,"day":1,"set_number":9,"reps":8,"group_key":"1A"}`,
	} {
		rr = env.do(t, "POST", setsPath, fmt.Sprintf(bad, squat.ID), cookies)
		requireStatus(t, rr, http.StatusBadRequest)
	}

	rr = env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/programs", athlete.ID), fmt.Sprintf(`{"template_id":%d,"start_date":"2026-01-01"}`, tmpl.ID), cookies)
	requireStatus(t, rr, http.StatusCreated)

	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/prescription", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var rx PrescriptionResponse
	decodeJSON(t, rr, &rx)
	if len(rx.Lines) != 2 {
		t.Fatalf("lines = %+v", rx.Lines)
	}
	sq := rx.Lines[0]
	if sq.ExerciseID != squat.ID || len(sq.Sets) != 2 || sq.GroupKey == nil || *sq.GroupKey != "A" {
		t.Fatalf("squat line = %+v", sq)
	}
	if !sq.Sets[0].NonWorking || sq.Sets[1].NonWorking {
		t.Fatalf("warm-up flags = %+v", sq.Sets)
	}
	work := sq.Sets[1]
	if work.RepsMax == nil || *work.RepsMax != 12 || work.RIRMin == nil || *work.RIRMin != 2 || work.RIRMax == nil || *work.RIRMax != 3 || work.GroupKey == nil || *work.GroupKey != "A" {
		t.Fatalf("working set = %+v", work)
	}
	if rx.Lines[1].GroupKey == nil || *rx.Lines[1].GroupKey != "A" {
		t.Fatalf("row line = %+v", rx.Lines[1])
	}

	// An update that omits the detail keeps it; explicit values clear it.
	setPath := fmt.Sprintf("%s/%d", setsPath, created.ID)
	rr = env.do(t, "PUT", setPath, fmt.Sprintf(`{"exercise_id":%d,"set_number":2,"reps":8,"sort_order":1}`, squat.ID), cookies)
	requireStatus(t, rr, http.StatusOK)
	var updated PrescribedSet
	decodeJSON(t, rr, &updated)
	if updated.RepsMax == nil || updated.RIRMin == nil || updated.GroupKey == nil {
		t.Fatalf("omitted detail was cleared: %+v", updated)
	}
	rr = env.do(t, "PUT", setPath, fmt.Sprintf(`{"exercise_id":%d,"set_number":2,"reps":8,"sort_order":1,"reps_max":0,"rir_min":-1,"group_key":"","non_working":true}`, squat.ID), cookies)
	requireStatus(t, rr, http.StatusOK)
	updated = PrescribedSet{}
	decodeJSON(t, rr, &updated)
	if updated.RepsMax != nil || updated.RIRMin != nil || updated.RIRMax != nil || updated.GroupKey != nil || !updated.NonWorking {
		t.Fatalf("cleared = %+v", updated)
	}
	rr = env.do(t, "PUT", setPath, fmt.Sprintf(`{"exercise_id":%d,"set_number":2,"reps":8,"group_key":"#"}`, squat.ID), cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestWorkoutSetType_CreateUpdateRead(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Dana", coach.ID)
	ex := env.createExercise(t, "Back Squat")
	w := env.createWorkout(t, athlete.ID, "2026-05-12")
	cookies := env.loginAs(t, coach)
	setsPath := fmt.Sprintf("/api/athletes/%d/workouts/%d/sets", athlete.ID, w.ID)

	rr := env.do(t, "POST", setsPath, fmt.Sprintf(`{"exercise_id":%d,"reps":5,"weight":95,"set_type":"warmup"}`, ex.ID), cookies)
	requireStatus(t, rr, http.StatusCreated)
	var warm WorkoutSet
	decodeJSON(t, rr, &warm)
	if warm.SetType != "warmup" {
		t.Fatalf("set type = %q", warm.SetType)
	}
	rr = env.do(t, "POST", setsPath, fmt.Sprintf(`{"exercise_id":%d,"reps":5,"weight":225}`, ex.ID), cookies)
	requireStatus(t, rr, http.StatusCreated)
	var work WorkoutSet
	decodeJSON(t, rr, &work)
	if work.SetType != "working" {
		t.Fatalf("default set type = %q", work.SetType)
	}
	rr = env.do(t, "POST", setsPath, fmt.Sprintf(`{"exercise_id":%d,"reps":5,"set_type":"cooldown"}`, ex.ID), cookies)
	requireStatus(t, rr, http.StatusBadRequest)

	rr = env.do(t, "PUT", fmt.Sprintf("%s/%d", setsPath, work.ID), `{"set_type":"drop"}`, cookies)
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &work)
	if work.SetType != "drop" || work.Weight == nil || *work.Weight != 225 {
		t.Fatalf("updated = %+v", work)
	}
	rr = env.do(t, "PUT", fmt.Sprintf("%s/%d", setsPath, work.ID), `{"set_type":"nope"}`, cookies)
	requireStatus(t, rr, http.StatusBadRequest)

	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/workouts/%d", athlete.ID, w.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var got struct {
		Workout Workout          `json:"workout"`
		Groups  []*ExerciseGroup `json:"groups"`
	}
	decodeJSON(t, rr, &got)
	if len(got.Groups) != 1 || got.Groups[0].Sets[0].SetType != "warmup" || got.Groups[0].Sets[1].SetType != "drop" {
		t.Fatalf("groups = %+v", got.Groups)
	}
	if got.Workout.SetCount != 1 {
		t.Fatalf("set count = %d, want 1 (warm-up excluded)", got.Workout.SetCount)
	}
}

func TestAccessoryPlanGroupKey_API(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Dana", coach.ID)
	ex := env.createExercise(t, "Face Pull")
	cookies := env.loginAs(t, coach)
	path := fmt.Sprintf("/api/athletes/%d/accessories", athlete.ID)

	rr := env.do(t, "POST", path, fmt.Sprintf(`{"day":1,"exercise_id":%d,"target_sets":3,"group_key":"b"}`, ex.ID), cookies)
	requireStatus(t, rr, http.StatusCreated)
	var plan AccessoryPlan
	decodeJSON(t, rr, &plan)
	if plan.GroupKey == nil || *plan.GroupKey != "B" {
		t.Fatalf("plan = %+v", plan)
	}
	rr = env.do(t, "POST", path, fmt.Sprintf(`{"day":2,"exercise_id":%d,"group_key":"!!"}`, ex.ID), cookies)
	requireStatus(t, rr, http.StatusBadRequest)

	rr = env.do(t, "PUT", fmt.Sprintf("%s/%d", path, plan.ID), `{"target_sets":4}`, cookies)
	requireStatus(t, rr, http.StatusOK)
	rr = env.do(t, "GET", path, nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var plans []AccessoryPlan
	decodeJSON(t, rr, &plans)
	if len(plans) != 1 || plans[0].GroupKey == nil || *plans[0].GroupKey != "B" {
		t.Fatalf("omitted group key changed: %+v", plans)
	}
	rr = env.do(t, "PUT", fmt.Sprintf("%s/%d", path, plan.ID), `{"target_sets":4,"group_key":""}`, cookies)
	requireStatus(t, rr, http.StatusOK)
	rr = env.do(t, "GET", path, nil, cookies)
	plans = nil
	decodeJSON(t, rr, &plans)
	if plans[0].GroupKey != nil {
		t.Fatalf("group key not cleared: %+v", plans[0])
	}
}

func TestPlateSetup(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	other := env.createUser(t, "other", true, false)
	athlete := env.createAthlete(t, "Kid", coach.ID)
	kid := env.createUser(t, "kid", false, false)
	if _, err := env.DB.Exec(`UPDATE users SET athlete_id = ? WHERE id = ?`, athlete.ID, kid.ID); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/athletes/%d/plate-setup", athlete.ID)
	coachCookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", path, nil, coachCookies)
	requireStatus(t, rr, http.StatusOK)
	var setup PlateSetupResponse
	decodeJSON(t, rr, &setup)
	if !setup.IsDefault || setup.Bar.Weight != 20 || setup.Bar.Unit != "kg" || setup.Collar.Weight != 2.5 || setup.UseCollars || len(setup.Plates) != 10 || setup.Plates[0].Weight != 25 || setup.Plates[0].Unit != "kg" {
		t.Fatalf("default setup = %+v", setup)
	}

	body := `{"bar":{"weight":20,"unit":"kg"},"collar":{"weight":2.5,"unit":"kg"},"use_collars":true,
	  "plates":[{"weight":1.25,"unit":"kg","pairs":1},{"weight":25,"unit":"kg","pairs":3},{"weight":10,"unit":"lbs","pairs":2}]}`
	rr = env.do(t, "PUT", path, body, coachCookies)
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &setup)
	if setup.IsDefault || !setup.UseCollars || len(setup.Plates) != 3 || setup.UpdatedAt == nil {
		t.Fatalf("saved setup = %+v", setup)
	}
	// Heaviest first, across units: 25 kg, 10 lbs (4.5 kg), 1.25 kg.
	if setup.Plates[0].Weight != 25 || setup.Plates[1].Unit != "lbs" || setup.Plates[2].Weight != 1.25 {
		t.Fatalf("plate order = %+v", setup.Plates)
	}

	// The athlete reads the same setup the coach saved.
	rr = env.do(t, "GET", path, nil, env.loginAs(t, kid))
	requireStatus(t, rr, http.StatusOK)
	var seen PlateSetupResponse
	decodeJSON(t, rr, &seen)
	if seen.IsDefault || len(seen.Plates) != 3 {
		t.Fatalf("athlete sees = %+v", seen)
	}

	// A coach who does not coach this athlete cannot read or change it.
	otherCookies := env.loginAs(t, other)
	requireStatus(t, env.do(t, "GET", path, nil, otherCookies), http.StatusForbidden)
	requireStatus(t, env.do(t, "PUT", path, body, otherCookies), http.StatusForbidden)
	requireStatus(t, env.do(t, "DELETE", path, nil, otherCookies), http.StatusForbidden)

	// The athlete can save their own setup.
	requireStatus(t, env.do(t, "PUT", path, body, env.loginAs(t, kid)), http.StatusOK)

	for _, bad := range []string{
		`{"bar":{"weight":20,"unit":"stone"},"plates":[]}`,
		`{"bar":{"weight":-1,"unit":"kg"},"plates":[]}`,
		`{"bar":{"weight":20,"unit":"kg"},"plates":[{"weight":0,"unit":"kg","pairs":1}]}`,
		`{"bar":{"weight":20,"unit":"kg"},"plates":[{"weight":5,"unit":"kg","pairs":-1}]}`,
		`{"bar":{"weight":20,"unit":"kg"},"plates":[{"weight":5,"unit":"kg","pairs":1},{"weight":5,"unit":"kg","pairs":2}]}`,
	} {
		requireStatus(t, env.do(t, "PUT", path, bad, coachCookies), http.StatusBadRequest)
	}

	rr = env.do(t, "DELETE", path, nil, coachCookies)
	requireStatus(t, rr, http.StatusOK)
	rr = env.do(t, "GET", path, nil, coachCookies)
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &setup)
	if !setup.IsDefault || len(setup.Plates) != 10 {
		t.Fatalf("after reset = %+v", setup)
	}

	rr = env.do(t, "GET", "/api/plate-setup/presets", nil, coachCookies)
	requireStatus(t, rr, http.StatusOK)
	var presets []PlateSetupPresetResponse
	decodeJSON(t, rr, &presets)
	if len(presets) != 2 || presets[0].Key != "kg_competition" || presets[1].Setup.Bar.Weight != 45 || presets[1].Setup.Bar.Unit != "lbs" {
		t.Fatalf("presets = %+v", presets)
	}
}
