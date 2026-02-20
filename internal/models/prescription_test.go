package models

import (
	"database/sql"
	"fmt"
	"testing"
)

func TestPrescriptionLine_PercentageLabel(t *testing.T) {
	tests := []struct {
		name string
		pct  *float64
		want string
	}{
		{"nil", nil, ""},
		{"75%", ptrFloat(75.0), "75%"},
		{"85%", ptrFloat(85.5), "86%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pl := &PrescriptionLine{Percentage: tt.pct}
			if got := pl.PercentageLabel(); got != tt.want {
				t.Errorf("PercentageLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrescriptionLine_TargetWeightLabel(t *testing.T) {
	tests := []struct {
		name   string
		weight *float64
		want   string
	}{
		{"nil", nil, ""},
		{"130", ptrFloat(130.0), "130.0"},
		{"132.5", ptrFloat(132.5), "132.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pl := &PrescriptionLine{TargetWeight: tt.weight}
			if got := pl.TargetWeightLabel(); got != tt.want {
				t.Errorf("TargetWeightLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrescriptionLine_SetsSummary(t *testing.T) {
	tests := []struct {
		name string
		sets []*PrescribedSet
		want string
	}{
		{"empty", nil, ""},
		{
			"uniform 3x5",
			[]*PrescribedSet{
				{Reps: sql.NullInt64{Int64: 5, Valid: true}},
				{Reps: sql.NullInt64{Int64: 5, Valid: true}},
				{Reps: sql.NullInt64{Int64: 5, Valid: true}},
			},
			"3×5",
		},
		{
			"5/3/1+ style",
			[]*PrescribedSet{
				{Reps: sql.NullInt64{Int64: 5, Valid: true}},
				{Reps: sql.NullInt64{Int64: 3, Valid: true}},
				{Reps: sql.NullInt64{Valid: false}}, // AMRAP
			},
			"5/3/AMRAP",
		},
		{
			"single AMRAP",
			[]*PrescribedSet{
				{Reps: sql.NullInt64{Valid: false}},
			},
			"1×AMRAP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pl := &PrescriptionLine{Sets: tt.sets}
			if got := pl.SetsSummary(); got != tt.want {
				t.Errorf("SetsSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRoundToNearest(t *testing.T) {
	tests := []struct {
		value     float64
		increment float64
		want      float64
	}{
		{130.0, 2.5, 130.0},
		{131.0, 2.5, 130.0},   // 131 / 2.5 = 52.4 -> round(52.4) = 52 -> 52 * 2.5 = 130.0
		{131.25, 2.5, 132.5},  // 131.25 / 2.5 = 52.5 -> round(52.5) = 53 -> 53 * 2.5 = 132.5
		{132.0, 2.5, 132.5},   // 132 / 2.5 = 52.8 -> round(52.8) = 53 -> 53 * 2.5 = 132.5
		{133.0, 2.5, 132.5},   // 133 / 2.5 = 53.2 -> 53 * 2.5 = 132.5
		{134.0, 2.5, 135.0},   // 134 / 2.5 = 53.6 -> 54 * 2.5 = 135.0
		{100.0, 5.0, 100.0},   // exact
		{102.0, 5.0, 100.0},   // rounds down
		{103.0, 5.0, 105.0},   // rounds up
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.1f/%.1f", tt.value, tt.increment), func(t *testing.T) {
			got := roundToNearest(tt.value, tt.increment)
			if got != tt.want {
				t.Errorf("roundToNearest(%.1f, %.1f) = %.1f, want %.1f", tt.value, tt.increment, got, tt.want)
			}
		})
	}
}

func TestCurrentTrainingMaxes(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "TM Athlete", "", "", "", sql.NullInt64{}, true)
	bench, _ := CreateExercise(db, "Bench Press", "", "", "", 0)
	squat, _ := CreateExercise(db, "Back Squat", "", "", "", 0)

	// Set multiple TMs for bench (should return latest).
	SetTrainingMax(db, a.ID, bench.ID, 180, "2026-01-01", "")
	SetTrainingMax(db, a.ID, bench.ID, 200, "2026-02-01", "")
	SetTrainingMax(db, a.ID, squat.ID, 300, "2026-01-15", "")

	maxes, err := ListCurrentTrainingMaxes(db, a.ID)
	if err != nil {
		t.Fatalf("current training maxes: %v", err)
	}
	if len(maxes) != 2 {
		t.Fatalf("maxes = %d, want 2", len(maxes))
	}

	// Should be ordered alphabetically: Back Squat, Bench Press.
	tmMap := make(map[string]float64)
	for _, tm := range maxes {
		tmMap[tm.ExerciseName] = tm.Weight
	}

	if w, ok := tmMap["Bench Press"]; !ok || w != 200 {
		t.Errorf("Bench TM = %.0f, want 200", w)
	}
	if w, ok := tmMap["Back Squat"]; !ok || w != 300 {
		t.Errorf("Squat TM = %.0f, want 300", w)
	}
}

func TestCurrentTrainingMaxes_Empty(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "No TM Athlete", "", "", "", sql.NullInt64{}, true)

	maxes, err := ListCurrentTrainingMaxes(db, a.ID)
	if err != nil {
		t.Fatalf("current training maxes: %v", err)
	}
	if len(maxes) != 0 {
		t.Errorf("maxes = %d, want 0", len(maxes))
	}
}

func TestGetPrescription_CycleWraparound(t *testing.T) {
	db := testDB(t)

	// 2 weeks × 2 days = 4 total positions.
	tmpl, _ := CreateProgramTemplate(db, nil, "Short Cycle", "", 2, 2, false)
	bench, _ := CreateExercise(db, "Bench", "", "", "", 0)

	// Add sets for each day.
	for w := 1; w <= 2; w++ {
		for d := 1; d <= 2; d++ {
			reps := 5
			pct := 65.0
			CreatePrescribedSet(db, tmpl.ID, bench.ID, w, d, 1, &reps, &pct, nil, 0, "", "")
		}
	}

	a, _ := CreateAthlete(db, "Cycle Athlete", "", "", "", sql.NullInt64{}, true)
	SetTrainingMax(db, a.ID, bench.ID, 200, "2026-01-01", "")
	AssignProgram(db, a.ID, tmpl.ID, "2026-02-01", "", "")

	// Log 4 workouts (one full cycle), then 1 more.
	for i := 1; i <= 5; i++ {
		date := mustParseDate("2026-02-01").AddDate(0, 0, i-1).Format("2006-01-02")
		CreateWorkout(db, a.ID, date, "")
	}

	// 5 workouts completed. 5 % 4 = position 1 → W1D2.
	today := mustParseDate("2026-02-06")
	rx, err := GetPrescription(db, a.ID, today)
	if err != nil {
		t.Fatalf("get prescription: %v", err)
	}
	if rx == nil {
		t.Fatal("expected non-nil prescription")
	}
	if rx.CycleNumber != 2 {
		t.Errorf("cycle = %d, want 2", rx.CycleNumber)
	}
	if rx.CurrentWeek != 1 || rx.CurrentDay != 2 {
		t.Errorf("position = W%dD%d, want W1D2", rx.CurrentWeek, rx.CurrentDay)
	}
}

func TestGetPrescription_NoTrainingMax(t *testing.T) {
	db := testDB(t)

	tmpl, _ := CreateProgramTemplate(db, nil, "No TM Test", "", 1, 1, false)
	bench, _ := CreateExercise(db, "Bench", "", "", "", 0)

	reps := 5
	pct := 75.0
	CreatePrescribedSet(db, tmpl.ID, bench.ID, 1, 1, 1, &reps, &pct, nil, 0, "", "")

	a, _ := CreateAthlete(db, "No TM Athlete", "", "", "", sql.NullInt64{}, true)
	// Deliberately do NOT set a training max.
	AssignProgram(db, a.ID, tmpl.ID, "2026-02-01", "", "")

	today := mustParseDate("2026-02-01")
	rx, err := GetPrescription(db, a.ID, today)
	if err != nil {
		t.Fatalf("get prescription: %v", err)
	}
	if rx == nil {
		t.Fatal("expected non-nil prescription")
	}
	if len(rx.Lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(rx.Lines))
	}

	line := rx.Lines[0]
	if line.TrainingMax != nil {
		t.Errorf("expected nil TrainingMax, got %.1f", *line.TrainingMax)
	}
	if line.TargetWeight != nil {
		t.Errorf("expected nil TargetWeight, got %.1f", *line.TargetWeight)
	}
	// But percentage should still be set.
	if line.Percentage == nil || *line.Percentage != 75.0 {
		t.Errorf("percentage = %v, want 75.0", line.Percentage)
	}
}

func TestGetPrescription_HasWorkoutToday(t *testing.T) {
	db := testDB(t)

	tmpl, _ := CreateProgramTemplate(db, nil, "Today Test", "", 1, 1, false)
	bench, _ := CreateExercise(db, "Bench", "", "", "", 0)
	reps := 5
	CreatePrescribedSet(db, tmpl.ID, bench.ID, 1, 1, 1, &reps, nil, nil, 0, "", "")

	a, _ := CreateAthlete(db, "Today Athlete", "", "", "", sql.NullInt64{}, true)
	AssignProgram(db, a.ID, tmpl.ID, "2026-02-01", "", "")

	today := mustParseDate("2026-02-01")

	// Before workout.
	rx, _ := GetPrescription(db, a.ID, today)
	if rx.HasWorkout {
		t.Error("expected HasWorkout = false before logging")
	}

	// Log workout today.
	CreateWorkout(db, a.ID, "2026-02-01", "")
	rx, _ = GetPrescription(db, a.ID, today)
	if !rx.HasWorkout {
		t.Error("expected HasWorkout = true after logging")
	}
}

// ptrFloat returns a pointer to a float64 value.
func ptrFloat(v float64) *float64 {
	return &v
}
