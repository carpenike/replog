package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/importers"
	"github.com/carpenike/replog/internal/models"
)

func TestGenerate_Form(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	sm := testSessionManager()
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Tommy", "sport_performance")

	h := &Generate{DB: db, Sessions: sm, Templates: tc}

	t.Run("renders form", func(t *testing.T) {
		req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/programs/generate", nil, coach)
		req.SetPathValue("id", itoa(athlete.ID))
		rr := httptest.NewRecorder()

		h.Form(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("shows not configured", func(t *testing.T) {
		req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/programs/generate", nil, coach)
		req.SetPathValue("id", itoa(athlete.ID))
		rr := httptest.NewRecorder()

		h.Form(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		// Response should NOT contain "configured" since no provider is set.
		body := rr.Body.String()
		if contains(body, "configured") {
			t.Errorf("expected form to show not-configured state")
		}
	})

	t.Run("shows configured when provider set", func(t *testing.T) {
		models.SetSetting(db, "llm.provider", "openai")
		defer models.DeleteSetting(db, "llm.provider")

		req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/programs/generate", nil, coach)
		req.SetPathValue("id", itoa(athlete.ID))
		rr := httptest.NewRecorder()

		h.Form(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		body := rr.Body.String()
		if !contains(body, "configured") {
			t.Errorf("expected form to show configured state")
		}
	})

	t.Run("pre-fills from active program", func(t *testing.T) {
		// Create a program template and assign it.
		pt, err := models.CreateProgramTemplate(db, nil, "Sport Performance Month 3", "test", 4, 4, false, "")
		if err != nil {
			t.Fatal(err)
		}
		_, err = models.AssignProgram(db, athlete.ID, pt.ID, "2026-01-15", "", "")
		if err != nil {
			t.Fatal(err)
		}

		req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/programs/generate", nil, coach)
		req.SetPathValue("id", itoa(athlete.ID))
		rr := httptest.NewRecorder()

		h.Form(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

func TestGenerate_Form_AthleteNotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	sm := testSessionManager()
	coach := seedCoach(t, db)

	h := &Generate{DB: db, Sessions: sm, Templates: tc}

	req := requestWithUser("GET", "/athletes/99999/programs/generate", nil, coach)
	req.SetPathValue("id", "99999")
	rr := httptest.NewRecorder()

	h.Form(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestGenerate_Submit_NotConfigured(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	sm := testSessionManager()
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Tommy", "sport_performance")

	h := &Generate{DB: db, Sessions: sm, Templates: tc}

	body := url.Values{
		"program_name": {"Test Program"},
		"num_days":     {"3"},
		"num_weeks":    {"4"},
	}

	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/programs/generate", body, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()

	sm.LoadAndSave(http.HandlerFunc(h.Submit)).ServeHTTP(rr, req)

	// Without a configured provider, should get 200 with error message on form.
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 (re-rendered form with error), got %d", rr.Code)
	}
	if !contains(rr.Body.String(), "not configured") {
		t.Errorf("expected error about not configured, got: %s", rr.Body.String())
	}
}

func TestGenerate_Preview_NoSession(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	sm := testSessionManager()
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Tommy", "sport_performance")

	h := &Generate{DB: db, Sessions: sm, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/programs/generate/preview", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()

	sm.LoadAndSave(http.HandlerFunc(h.Preview)).ServeHTTP(rr, req)

	// Without session data, should redirect to form.
	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	expected := fmt.Sprintf("/athletes/%d/programs/generate", athlete.ID)
	if loc != expected {
		t.Errorf("expected redirect to %s, got %s", expected, loc)
	}
}

func TestGenerate_Execute_NoSession(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	sm := testSessionManager()
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Tommy", "sport_performance")

	h := &Generate{DB: db, Sessions: sm, Templates: tc}

	body := url.Values{}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/programs/generate/execute", body, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()

	sm.LoadAndSave(http.HandlerFunc(h.Execute)).ServeHTTP(rr, req)

	// Without session data, should redirect to form.
	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rr.Code)
	}
}

func TestGenerate_SaveEdits_NoSession(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	sm := testSessionManager()
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Tommy", "sport_performance")

	h := &Generate{DB: db, Sessions: sm, Templates: tc}

	body := url.Values{"action": {"save"}, "set_count": {"0"}}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/programs/generate/preview", body, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()

	sm.LoadAndSave(http.HandlerFunc(h.SaveEdits)).ServeHTTP(rr, req)

	// Without session data, should redirect to form.
	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	expected := fmt.Sprintf("/athletes/%d/programs/generate", athlete.ID)
	if loc != expected {
		t.Errorf("expected redirect to %s, got %s", expected, loc)
	}
}

func TestBuildEditableRows(t *testing.T) {
	fiveReps := 5
	pct75 := 0.75
	notes := "Pause at bottom"
	programs := []importers.ParsedProgram{{
		Template: importers.ParsedProgramTemplate{
			Name:     "Test Program",
			NumWeeks: 1,
			NumDays:  2,
			PrescribedSets: []importers.ParsedPrescribedSet{
				{Exercise: "Squat", Week: 1, Day: 1, SetNumber: 1, Reps: &fiveReps, RepType: "reps", Percentage: &pct75, SortOrder: 1, Notes: &notes},
				{Exercise: "Squat", Week: 1, Day: 1, SetNumber: 2, Reps: &fiveReps, RepType: "reps", Percentage: &pct75, SortOrder: 1},
				{Exercise: "Squat", Week: 1, Day: 1, SetNumber: 3, Reps: nil, RepType: "reps", Percentage: &pct75, SortOrder: 1}, // AMRAP last
				{Exercise: "Bench", Week: 1, Day: 1, SetNumber: 1, Reps: &fiveReps, RepType: "reps", Percentage: &pct75, SortOrder: 2},
				{Exercise: "Row", Week: 1, Day: 2, SetNumber: 1, Reps: &fiveReps, RepType: "reps", SortOrder: 1},
			},
		},
	}}

	rows := buildEditableRows(programs)

	if len(rows) != 3 {
		t.Fatalf("expected 3 editable rows, got %d", len(rows))
	}

	// Row 0: Squat — 3 sets, reps=5, AMRAP last=true, 75%.
	r0 := rows[0]
	if r0.Exercise != "Squat" {
		t.Errorf("row 0 exercise = %q, want Squat", r0.Exercise)
	}
	if r0.NumSets != 3 {
		t.Errorf("row 0 num_sets = %d, want 3", r0.NumSets)
	}
	if r0.Reps != "5" {
		t.Errorf("row 0 reps = %q, want 5", r0.Reps)
	}
	if !r0.AmrapLast {
		t.Error("row 0 amrap_last should be true")
	}
	if r0.LoadType != "percent" {
		t.Errorf("row 0 load_type = %q, want percent", r0.LoadType)
	}
	if r0.LoadValue != "75" {
		t.Errorf("row 0 load_value = %q, want 75", r0.LoadValue)
	}
	if r0.Notes != "Pause at bottom" {
		t.Errorf("row 0 notes = %q, want 'Pause at bottom'", r0.Notes)
	}

	// Row 1: Bench — 1 set, reps=5, no AMRAP.
	r1 := rows[1]
	if r1.Exercise != "Bench" {
		t.Errorf("row 1 exercise = %q, want Bench", r1.Exercise)
	}
	if r1.NumSets != 1 {
		t.Errorf("row 1 num_sets = %d, want 1", r1.NumSets)
	}
	if r1.AmrapLast {
		t.Error("row 1 amrap_last should be false")
	}

	// Row 2: Row — day 2.
	r2 := rows[2]
	if r2.Exercise != "Row" {
		t.Errorf("row 2 exercise = %q, want Row", r2.Exercise)
	}
	if r2.Day != 2 {
		t.Errorf("row 2 day = %d, want 2", r2.Day)
	}
	if r2.LoadType != "bodyweight" {
		t.Errorf("row 2 load_type = %q, want bodyweight", r2.LoadType)
	}
}

func TestBuildEditableRows_AbsoluteWeight(t *testing.T) {
	eightReps := 8
	weight25 := 25.0
	programs := []importers.ParsedProgram{{
		Template: importers.ParsedProgramTemplate{
			Name:     "Test",
			NumWeeks: 1,
			NumDays:  1,
			PrescribedSets: []importers.ParsedPrescribedSet{
				{Exercise: "Curl", Week: 1, Day: 1, SetNumber: 1, Reps: &eightReps, RepType: "reps", AbsoluteWeight: &weight25, SortOrder: 1},
				{Exercise: "Curl", Week: 1, Day: 1, SetNumber: 2, Reps: &eightReps, RepType: "reps", AbsoluteWeight: &weight25, SortOrder: 1},
			},
		},
	}}

	rows := buildEditableRows(programs)

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].LoadType != "absolute" {
		t.Errorf("load_type = %q, want absolute", rows[0].LoadType)
	}
	if rows[0].LoadValue != "25" {
		t.Errorf("load_value = %q, want 25", rows[0].LoadValue)
	}
}

func TestRebuildPrescribedSets(t *testing.T) {
	rows := []editableSetRow{
		{ProgramIdx: 0, Week: 1, Day: 1, Exercise: "Squat", NumSets: 3, Reps: "5", AmrapLast: true, RepType: "reps", LoadType: "percent", LoadValue: "75", SortOrder: 1, Notes: "Go deep"},
		{ProgramIdx: 0, Week: 1, Day: 1, Exercise: "Bench", NumSets: 2, Reps: "8", RepType: "reps", LoadType: "absolute", LoadValue: "135", SortOrder: 2},
		{ProgramIdx: 0, Week: 1, Day: 2, Exercise: "Pull-up", NumSets: 3, Reps: "", RepType: "reps", LoadType: "bodyweight", SortOrder: 1},
	}

	result := rebuildPrescribedSets(rows)
	sets := result[0]

	if len(sets) != 8 { // 3 + 2 + 3
		t.Fatalf("expected 8 sets, got %d", len(sets))
	}

	// Squat: 3 sets, last is AMRAP.
	if sets[0].Reps == nil || *sets[0].Reps != 5 {
		t.Errorf("squat set 1 reps should be 5")
	}
	if sets[1].Reps == nil || *sets[1].Reps != 5 {
		t.Errorf("squat set 2 reps should be 5")
	}
	if sets[2].Reps != nil {
		t.Errorf("squat set 3 (AMRAP) reps should be nil, got %d", *sets[2].Reps)
	}
	if sets[0].Percentage == nil || *sets[0].Percentage != 0.75 {
		t.Error("squat percentage should be 0.75")
	}
	if sets[0].Notes == nil || *sets[0].Notes != "Go deep" {
		t.Error("squat notes should be 'Go deep'")
	}

	// Bench: absolute weight.
	if sets[3].AbsoluteWeight == nil || *sets[3].AbsoluteWeight != 135 {
		t.Error("bench absolute weight should be 135")
	}

	// Pull-up: bodyweight (absolute=0), all AMRAP (nil reps).
	if sets[5].Reps != nil {
		t.Errorf("pullup reps should be nil (AMRAP), got %v", sets[5].Reps)
	}
	if sets[5].AbsoluteWeight == nil || *sets[5].AbsoluteWeight != 0 {
		t.Error("pullup should have absolute_weight=0 (bodyweight)")
	}
}

func TestParseEditableRows_DeleteRemovesRow(t *testing.T) {
	body := url.Values{
		"set_count":          {"2"},
		"set_0_exercise":     {"Squat"},
		"set_0_program_idx":  {"0"},
		"set_0_week":         {"1"},
		"set_0_day":          {"1"},
		"set_0_num_sets":     {"3"},
		"set_0_reps":         {"5"},
		"set_0_rep_type":     {"reps"},
		"set_0_load_type":    {"percent"},
		"set_0_load_value":   {"75"},
		"set_0_sort_order":   {"1"},
		"set_0_notes":        {""},
		"set_1_exercise":     {"Bench"},
		"set_1_program_idx":  {"0"},
		"set_1_week":         {"1"},
		"set_1_day":          {"1"},
		"set_1_num_sets":     {"3"},
		"set_1_reps":         {"5"},
		"set_1_rep_type":     {"reps"},
		"set_1_load_type":    {"percent"},
		"set_1_load_value":   {"75"},
		"set_1_sort_order":   {"2"},
		"set_1_delete":       {"1"}, // Mark for deletion.
		"set_1_notes":        {""},
	}

	r := httptest.NewRequest("POST", "/test", strings.NewReader(body.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rows, err := parseEditableRows(r)
	if err != nil {
		t.Fatalf("parseEditableRows error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after deletion, got %d", len(rows))
	}
	if rows[0].Exercise != "Squat" {
		t.Errorf("remaining row exercise = %q, want Squat", rows[0].Exercise)
	}
}

func TestSuggestNextProgramName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Sport Performance Month 3", "Sport Performance Month 4"},
		{"5/3/1 Cycle 1", "5/3/1 Cycle 2"},
		{"GZCL", "GZCL 2"},
		{"My Program 10", "My Program 11"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := suggestNextProgramName(tt.input)
			if got != tt.want {
				t.Errorf("suggestNextProgramName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// contains is a helper for checking substrings.
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
