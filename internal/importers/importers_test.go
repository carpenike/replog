package importers

import (
	"strings"
	"testing"
)

// --- DetectFormat tests ---

func TestDetectFormat_RepLogJSON(t *testing.T) {
	data := []byte(`{"version": "1.0", "weight_unit": "lbs"}`)
	got := DetectFormat(data)
	if got != FormatRepLogJSON {
		t.Errorf("DetectFormat(json) = %q, want %q", got, FormatRepLogJSON)
	}
}

func TestDetectFormat_RepLogJSON_BOM(t *testing.T) {
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"version": "1.0"}`)...)
	got := DetectFormat(data)
	if got != FormatRepLogJSON {
		t.Errorf("DetectFormat(json+BOM) = %q, want %q", got, FormatRepLogJSON)
	}
}

func TestDetectFormat_StrongCSV(t *testing.T) {
	data := []byte("Date,Workout Name,Duration,Exercise Name,Set Order,Weight,Reps,Distance,Seconds,Notes,Workout Notes,RPE\n")
	got := DetectFormat(data)
	if got != FormatStrongCSV {
		t.Errorf("DetectFormat(strong) = %q, want %q", got, FormatStrongCSV)
	}
}

func TestDetectFormat_HevyCSV(t *testing.T) {
	data := []byte("title,start_time,end_time,description,exercise_title,superset_id,exercise_notes,set_index,set_type,weight_lbs,reps,rpe,duration_seconds,distance_km\n")
	got := DetectFormat(data)
	if got != FormatHevyCSV {
		t.Errorf("DetectFormat(hevy) = %q, want %q", got, FormatHevyCSV)
	}
}

func TestDetectFormat_Unknown(t *testing.T) {
	data := []byte("some random text\n")
	got := DetectFormat(data)
	if got != "" {
		t.Errorf("DetectFormat(unknown) = %q, want empty", got)
	}
}

// --- Strong CSV parser tests ---

func TestParseStrongCSV_Basic(t *testing.T) {
	csv := `Date,Workout Name,Duration,Exercise Name,Set Order,Weight,Reps,Distance,Seconds,Notes,Workout Notes,RPE
2024-01-15 08:00:00,Morning,30m,Bench Press,1,135,5,,,,,
2024-01-15 08:00:00,Morning,30m,Bench Press,2,155,5,,,,,
2024-01-15 08:00:00,Morning,30m,Squat,1,225,3,,,,,8
`
	pf, err := ParseStrongCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ParseStrongCSV: %v", err)
	}

	if pf.Format != FormatStrongCSV {
		t.Errorf("Format = %q, want %q", pf.Format, FormatStrongCSV)
	}

	if len(pf.Exercises) != 2 {
		t.Fatalf("got %d exercises, want 2", len(pf.Exercises))
	}

	// Check exercise names are collected.
	names := map[string]bool{}
	for _, e := range pf.Exercises {
		names[e.Name] = true
	}
	if !names["Bench Press"] || !names["Squat"] {
		t.Errorf("exercises = %v, want Bench Press and Squat", names)
	}

	if len(pf.Workouts) != 1 {
		t.Fatalf("got %d workouts, want 1", len(pf.Workouts))
	}

	wo := pf.Workouts[0]
	if wo.Date != "2024-01-15" {
		t.Errorf("workout date = %q, want 2024-01-15", wo.Date)
	}
	if len(wo.Sets) != 3 {
		t.Errorf("got %d sets, want 3", len(wo.Sets))
	}

	// Check RPE is parsed.
	sqSet := wo.Sets[2]
	if sqSet.RPE == nil || *sqSet.RPE != 8 {
		t.Errorf("squat RPE = %v, want 8", sqSet.RPE)
	}
}

func TestParseStrongCSV_EmptyFile(t *testing.T) {
	csv := `Date,Workout Name,Duration,Exercise Name,Set Order,Weight,Reps,Distance,Seconds,Notes,Workout Notes,RPE
`
	_, err := ParseStrongCSV(strings.NewReader(csv))
	if err == nil {
		t.Fatal("expected error for empty file, got nil")
	}
}

func TestParseStrongCSV_MultipleDates(t *testing.T) {
	csv := `Date,Workout Name,Duration,Exercise Name,Set Order,Weight,Reps,Distance,Seconds,Notes,Workout Notes,RPE
2024-01-15 08:00:00,Morning,30m,Bench Press,1,135,5,,,,,
2024-01-16 08:00:00,Evening,30m,Squat,1,225,3,,,,,
`
	pf, err := ParseStrongCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ParseStrongCSV: %v", err)
	}
	if len(pf.Workouts) != 2 {
		t.Errorf("got %d workouts, want 2", len(pf.Workouts))
	}
}

func TestParseStrongCSV_DurationSet(t *testing.T) {
	csv := `Date,Workout Name,Duration,Exercise Name,Set Order,Weight,Reps,Distance,Seconds,Notes,Workout Notes,RPE
2024-01-15 08:00:00,Morning,30m,Plank,1,0,0,,60,,,
`
	pf, err := ParseStrongCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ParseStrongCSV: %v", err)
	}
	if len(pf.Workouts) != 1 || len(pf.Workouts[0].Sets) != 1 {
		t.Fatal("expected 1 workout with 1 set")
	}
	s := pf.Workouts[0].Sets[0]
	if s.RepType != "seconds" {
		t.Errorf("rep_type = %q, want seconds", s.RepType)
	}
	if s.Reps != 60 {
		t.Errorf("reps (seconds) = %d, want 60", s.Reps)
	}
}

// --- Hevy CSV parser tests ---

func TestParseHevyCSV_Basic(t *testing.T) {
	csv := `title,start_time,end_time,description,exercise_title,superset_id,exercise_notes,set_index,set_type,weight_lbs,reps,rpe,duration_seconds,distance_km
Morning Workout,2024-01-15 08:00:00,,Leg day,Squat,,Good form,0,normal,225,5,8,,
Morning Workout,2024-01-15 08:00:00,,Leg day,Squat,,,1,normal,245,3,9,,
Morning Workout,2024-01-15 08:00:00,,Leg day,Deadlift,,,0,warmup,135,5,,,
`
	pf, err := ParseHevyCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ParseHevyCSV: %v", err)
	}

	if pf.Format != FormatHevyCSV {
		t.Errorf("Format = %q, want %q", pf.Format, FormatHevyCSV)
	}

	if len(pf.Exercises) != 2 {
		t.Fatalf("got %d exercises, want 2", len(pf.Exercises))
	}

	if len(pf.Workouts) != 1 {
		t.Fatalf("got %d workouts, want 1", len(pf.Workouts))
	}

	wo := pf.Workouts[0]
	if wo.Date != "2024-01-15" {
		t.Errorf("date = %q, want 2024-01-15", wo.Date)
	}

	// Hevy workout notes come from description.
	if wo.Notes == nil || *wo.Notes != "Leg day" {
		t.Errorf("notes = %v, want Leg day", wo.Notes)
	}

	if len(wo.Sets) != 3 {
		t.Fatalf("got %d sets, want 3", len(wo.Sets))
	}

	// Hevy set_index is 0-based, should be offset to 1-based.
	if wo.Sets[0].SetNumber != 1 {
		t.Errorf("set 0 number = %d, want 1", wo.Sets[0].SetNumber)
	}

	// Warmup sets should have [warmup] prefix in notes.
	dlSet := wo.Sets[2]
	if dlSet.Notes == nil || !strings.Contains(*dlSet.Notes, "[warmup]") {
		t.Errorf("warmup set notes = %v, want [warmup] prefix", dlSet.Notes)
	}
}

func TestParseHevyCSV_EmptyFile(t *testing.T) {
	csv := `title,start_time,end_time,description,exercise_title,superset_id,exercise_notes,set_index,set_type,weight_lbs,reps,rpe,duration_seconds,distance_km
`
	_, err := ParseHevyCSV(strings.NewReader(csv))
	if err == nil {
		t.Fatal("expected error for empty file, got nil")
	}
}

// --- RepLog JSON parser tests ---

func TestParseRepLogJSON_Basic(t *testing.T) {
	jsonData := `{
		"version": "1.0",
		"weight_unit": "lbs",
		"exercises": [
			{"name": "Bench Press"},
			{"name": "Squat"}
		],
		"workouts": [
			{
				"date": "2024-01-15",
				"sets": [
					{"exercise": "Bench Press", "set_number": 1, "reps": 5, "weight": 135}
				]
			}
		],
		"body_weights": [
			{"date": "2024-01-15", "weight": 180.5}
		]
	}`

	pf, err := ParseRepLogJSON(strings.NewReader(jsonData))
	if err != nil {
		t.Fatalf("ParseRepLogJSON: %v", err)
	}

	if pf.Format != FormatRepLogJSON {
		t.Errorf("Format = %q, want %q", pf.Format, FormatRepLogJSON)
	}
	if pf.WeightUnit != "lbs" {
		t.Errorf("WeightUnit = %q, want lbs", pf.WeightUnit)
	}
	if len(pf.Exercises) != 2 {
		t.Errorf("got %d exercises, want 2", len(pf.Exercises))
	}
	if len(pf.Workouts) != 1 {
		t.Errorf("got %d workouts, want 1", len(pf.Workouts))
	}
	if len(pf.BodyWeights) != 1 {
		t.Errorf("got %d body weights, want 1", len(pf.BodyWeights))
	}

	// rep_type should default to "reps".
	if pf.Workouts[0].Sets[0].RepType != "reps" {
		t.Errorf("rep_type = %q, want reps", pf.Workouts[0].Sets[0].RepType)
	}
}

func TestParseRepLogJSON_MissingVersion(t *testing.T) {
	jsonData := `{"weight_unit": "lbs"}`
	_, err := ParseRepLogJSON(strings.NewReader(jsonData))
	if err == nil {
		t.Fatal("expected error for missing version, got nil")
	}
}

func TestParseRepLogJSON_DefaultWeightUnit(t *testing.T) {
	jsonData := `{"version": "1.0"}`
	pf, err := ParseRepLogJSON(strings.NewReader(jsonData))
	if err != nil {
		t.Fatalf("ParseRepLogJSON: %v", err)
	}
	if pf.WeightUnit != "lbs" {
		t.Errorf("WeightUnit = %q, want lbs", pf.WeightUnit)
	}
}

func TestParseRepLogJSON_Programs(t *testing.T) {
	jsonData := `{
		"version": "1.0",
		"programs": [
			{
				"template": {
					"name": "5/3/1",
					"num_weeks": 4,
					"num_days": 4,
					"prescribed_sets": [
						{"exercise": "Squat", "week": 1, "day": 1, "set_number": 1, "reps": 5, "rep_type": "reps", "percentage": 65}
					],
					"progression_rules": [
						{"exercise": "Squat", "increment": 10}
					]
				},
				"start_date": "2024-01-01",
				"active": true
			}
		]
	}`

	pf, err := ParseRepLogJSON(strings.NewReader(jsonData))
	if err != nil {
		t.Fatalf("ParseRepLogJSON: %v", err)
	}

	if len(pf.Programs) != 1 {
		t.Fatalf("got %d programs, want 1", len(pf.Programs))
	}

	prog := pf.Programs[0]
	if prog.Template.Name != "5/3/1" {
		t.Errorf("template name = %q, want 5/3/1", prog.Template.Name)
	}
	if len(prog.Template.PrescribedSets) != 1 {
		t.Errorf("got %d prescribed sets, want 1", len(prog.Template.PrescribedSets))
	}
	if len(prog.Template.ProgressionRules) != 1 {
		t.Errorf("got %d progression rules, want 1", len(prog.Template.ProgressionRules))
	}
}

// --- Mapper tests ---

func TestBuildExerciseMappings_ExactMatch(t *testing.T) {
	parsed := []ParsedExercise{
		{Name: "Bench Press"},
		{Name: "Squat"},
		{Name: "Unknown Exercise"},
	}
	existing := []ExistingEntity{
		{ID: 1, Name: "Bench Press"},
		{ID: 2, Name: "Squat"},
	}

	mappings := BuildExerciseMappings(parsed, existing)

	if len(mappings) != 3 {
		t.Fatalf("got %d mappings, want 3", len(mappings))
	}

	if mappings[0].MappedID != 1 || mappings[0].Create {
		t.Errorf("Bench Press: ID=%d Create=%v, want ID=1 Create=false", mappings[0].MappedID, mappings[0].Create)
	}
	if mappings[1].MappedID != 2 || mappings[1].Create {
		t.Errorf("Squat: ID=%d Create=%v, want ID=2 Create=false", mappings[1].MappedID, mappings[1].Create)
	}
	if mappings[2].MappedID != 0 || !mappings[2].Create {
		t.Errorf("Unknown: ID=%d Create=%v, want ID=0 Create=true", mappings[2].MappedID, mappings[2].Create)
	}
}

func TestBuildExerciseMappings_CaseInsensitive(t *testing.T) {
	parsed := []ParsedExercise{
		{Name: "bench press"},
	}
	existing := []ExistingEntity{
		{ID: 1, Name: "Bench Press"},
	}

	mappings := BuildExerciseMappings(parsed, existing)

	if mappings[0].MappedID != 1 {
		t.Errorf("case-insensitive match failed: ID=%d, want 1", mappings[0].MappedID)
	}
}

func TestBuildEquipmentMappings(t *testing.T) {
	parsed := []ParsedEquipment{
		{Name: "Barbell"},
		{Name: "New Item"},
	}
	existing := []ExistingEntity{
		{ID: 10, Name: "Barbell"},
	}

	mappings := BuildEquipmentMappings(parsed, existing)

	if len(mappings) != 2 {
		t.Fatalf("got %d mappings, want 2", len(mappings))
	}
	if mappings[0].MappedID != 10 {
		t.Errorf("Barbell: ID=%d, want 10", mappings[0].MappedID)
	}
	if !mappings[1].Create {
		t.Error("New Item should be marked for creation")
	}
}

func TestBuildProgramMappings(t *testing.T) {
	parsed := []ParsedProgram{
		{Template: ParsedProgramTemplate{Name: "5/3/1"}},
		{Template: ParsedProgramTemplate{Name: "New Program"}},
	}
	existing := []ExistingEntity{
		{ID: 5, Name: "5/3/1"},
	}

	mappings := BuildProgramMappings(parsed, existing)

	if len(mappings) != 2 {
		t.Fatalf("got %d mappings, want 2", len(mappings))
	}
	if mappings[0].MappedID != 5 {
		t.Errorf("5/3/1: ID=%d, want 5", mappings[0].MappedID)
	}
	if !mappings[1].Create {
		t.Error("New Program should be marked for creation")
	}
}

func TestMappingState_ResolveExerciseID(t *testing.T) {
	ms := &MappingState{
		Exercises: []EntityMapping{
			{ImportName: "Bench Press", MappedID: 1},
			{ImportName: "Squat", MappedID: 2},
		},
	}

	if id := ms.ResolveExerciseID("bench press"); id != 1 {
		t.Errorf("ResolveExerciseID(bench press) = %d, want 1", id)
	}
	if id := ms.ResolveExerciseID("Unknown"); id != 0 {
		t.Errorf("ResolveExerciseID(Unknown) = %d, want 0", id)
	}
}

func TestMappingState_ResolveEquipmentID(t *testing.T) {
	ms := &MappingState{
		Equipment: []EntityMapping{
			{ImportName: "Barbell", MappedID: 10},
		},
	}

	if id := ms.ResolveEquipmentID("Barbell"); id != 10 {
		t.Errorf("ResolveEquipmentID(Barbell) = %d, want 10", id)
	}
}

// --- parseStrongDate tests ---

func TestParseStrongDate(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2024-01-15 08:00:00", "2024-01-15"},
		{"2024-01-15T08:00:00Z", "2024-01-15"},
		{"2024-01-15", "2024-01-15"},
		{"01/15/2024", "2024-01-15"},
		{"Jan 15, 2024", "2024-01-15"},
	}

	for _, tt := range tests {
		got := parseStrongDate(tt.input)
		if got != tt.want {
			t.Errorf("parseStrongDate(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- containsAll / firstLineOf tests ---

func TestContainsAll(t *testing.T) {
	if !containsAll("abc def ghi", "abc", "def") {
		t.Error("containsAll should match present substrings")
	}
	if containsAll("abc def ghi", "abc", "xyz") {
		t.Error("containsAll should not match missing substring")
	}
}

func TestFirstLineOf(t *testing.T) {
	if got := firstLineOf([]byte("abc\ndef")); got != "abc" {
		t.Errorf("firstLineOf = %q, want abc", got)
	}
	if got := firstLineOf([]byte("abc")); got != "abc" {
		t.Errorf("firstLineOf = %q, want abc (no newline)", got)
	}
}
