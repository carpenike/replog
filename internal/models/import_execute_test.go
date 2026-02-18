package models

import (
	"testing"

	"github.com/carpenike/replog/internal/importers"
)

func TestValidateImportData(t *testing.T) {
	t.Run("clean data returns no warnings", func(t *testing.T) {
		w := 135.0
		rpe := 7.5
		pf := &importers.ParsedFile{
			Workouts: []importers.ParsedWorkout{
				{
					Date: "2025-01-15",
					Sets: []importers.ParsedWorkoutSet{
						{Exercise: "Bench Press", Reps: 5, Weight: &w, RPE: &rpe, RepType: "reps"},
					},
				},
			},
		}
		warnings := validateImportData(pf)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("negative weight", func(t *testing.T) {
		w := -45.0
		pf := &importers.ParsedFile{
			Workouts: []importers.ParsedWorkout{
				{
					Date: "2025-01-15",
					Sets: []importers.ParsedWorkoutSet{
						{Exercise: "Bench Press", Reps: 5, Weight: &w, RepType: "reps"},
					},
				},
			},
		}
		warnings := validateImportData(pf)
		if len(warnings) != 1 {
			t.Fatalf("expected 1 warning, got %d", len(warnings))
		}
		if warnings[0].Field != "weight" {
			t.Errorf("expected weight warning, got %q", warnings[0].Field)
		}
	})

	t.Run("negative reps", func(t *testing.T) {
		pf := &importers.ParsedFile{
			Workouts: []importers.ParsedWorkout{
				{
					Date: "2025-01-15",
					Sets: []importers.ParsedWorkoutSet{
						{Exercise: "Squat", Reps: -3, RepType: "reps"},
					},
				},
			},
		}
		warnings := validateImportData(pf)
		if len(warnings) != 1 {
			t.Fatalf("expected 1 warning, got %d", len(warnings))
		}
		if warnings[0].Field != "reps" {
			t.Errorf("expected reps warning, got %q", warnings[0].Field)
		}
	})

	t.Run("RPE out of range", func(t *testing.T) {
		rpe := 11.0
		pf := &importers.ParsedFile{
			Workouts: []importers.ParsedWorkout{
				{
					Date: "2025-01-15",
					Sets: []importers.ParsedWorkoutSet{
						{Exercise: "Deadlift", Reps: 5, RPE: &rpe, RepType: "reps"},
					},
				},
			},
		}
		warnings := validateImportData(pf)
		if len(warnings) != 1 {
			t.Fatalf("expected 1 warning, got %d", len(warnings))
		}
		if warnings[0].Field != "rpe" {
			t.Errorf("expected rpe warning, got %q", warnings[0].Field)
		}
	})

	t.Run("RPE negative", func(t *testing.T) {
		rpe := -1.0
		pf := &importers.ParsedFile{
			Workouts: []importers.ParsedWorkout{
				{
					Date: "2025-01-15",
					Sets: []importers.ParsedWorkoutSet{
						{Exercise: "Deadlift", Reps: 5, RPE: &rpe, RepType: "reps"},
					},
				},
			},
		}
		warnings := validateImportData(pf)
		if len(warnings) == 0 {
			t.Fatal("expected warning for negative RPE, got none")
		}
		if warnings[0].Field != "rpe" {
			t.Errorf("expected rpe warning, got %q", warnings[0].Field)
		}
	})

	t.Run("invalid rep type", func(t *testing.T) {
		pf := &importers.ParsedFile{
			Workouts: []importers.ParsedWorkout{
				{
					Date: "2025-01-15",
					Sets: []importers.ParsedWorkoutSet{
						{Exercise: "Plank", Reps: 60, RepType: "invalid_type"},
					},
				},
			},
		}
		warnings := validateImportData(pf)
		if len(warnings) != 1 {
			t.Fatalf("expected 1 warning, got %d", len(warnings))
		}
		if warnings[0].Field != "rep_type" {
			t.Errorf("expected rep_type warning, got %q", warnings[0].Field)
		}
	})

	t.Run("future workout date", func(t *testing.T) {
		pf := &importers.ParsedFile{
			Workouts: []importers.ParsedWorkout{
				{
					Date: "2099-12-31",
					Sets: []importers.ParsedWorkoutSet{
						{Exercise: "Bench Press", Reps: 5, RepType: "reps"},
					},
				},
			},
		}
		warnings := validateImportData(pf)
		if len(warnings) != 1 {
			t.Fatalf("expected 1 warning, got %d", len(warnings))
		}
		if warnings[0].Field != "date" {
			t.Errorf("expected date warning, got %q", warnings[0].Field)
		}
	})

	t.Run("negative training max weight", func(t *testing.T) {
		pf := &importers.ParsedFile{
			TrainingMaxes: []importers.ParsedTrainingMax{
				{Exercise: "Bench Press", Weight: -100, EffectiveDate: "2025-01-15"},
			},
		}
		warnings := validateImportData(pf)
		if len(warnings) != 1 {
			t.Fatalf("expected 1 warning, got %d", len(warnings))
		}
		if warnings[0].Entity != "training_max" {
			t.Errorf("expected training_max entity, got %q", warnings[0].Entity)
		}
	})

	t.Run("invalid body weight", func(t *testing.T) {
		pf := &importers.ParsedFile{
			BodyWeights: []importers.ParsedBodyWeight{
				{Date: "2025-01-15", Weight: 0},
				{Date: "2025-01-16", Weight: -5.0},
			},
		}
		warnings := validateImportData(pf)
		if len(warnings) != 2 {
			t.Fatalf("expected 2 warnings, got %d", len(warnings))
		}
		for _, w := range warnings {
			if w.Entity != "body_weight" {
				t.Errorf("expected body_weight entity, got %q", w.Entity)
			}
		}
	})

	t.Run("multiple issues in one import", func(t *testing.T) {
		negW := -10.0
		highRPE := 15.0
		pf := &importers.ParsedFile{
			Workouts: []importers.ParsedWorkout{
				{
					Date: "2099-06-01",
					Sets: []importers.ParsedWorkoutSet{
						{Exercise: "Squat", Reps: -1, Weight: &negW, RPE: &highRPE, RepType: "bad"},
					},
				},
			},
			TrainingMaxes: []importers.ParsedTrainingMax{
				{Exercise: "Squat", Weight: -50, EffectiveDate: "2025-01-01"},
			},
			BodyWeights: []importers.ParsedBodyWeight{
				{Date: "2025-01-01", Weight: -10},
			},
		}
		warnings := validateImportData(pf)
		// future date + negative weight + negative reps + RPE out of range + invalid rep type + negative TM + invalid BW
		if len(warnings) != 7 {
			t.Errorf("expected 7 warnings, got %d", len(warnings))
			for _, w := range warnings {
				t.Logf("  %s.%s: %s", w.Entity, w.Field, w.Message)
			}
		}
	})

	t.Run("valid rep types pass", func(t *testing.T) {
		pf := &importers.ParsedFile{
			Workouts: []importers.ParsedWorkout{
				{
					Date: "2025-01-15",
					Sets: []importers.ParsedWorkoutSet{
						{Exercise: "Bench Press", Reps: 5, RepType: "reps"},
						{Exercise: "Plank", Reps: 60, RepType: "seconds"},
						{Exercise: "Lunges", Reps: 10, RepType: "each_side"},
					},
				},
			},
		}
		warnings := validateImportData(pf)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings for valid rep types, got %d", len(warnings))
		}
	})

	t.Run("zero weight is allowed", func(t *testing.T) {
		w := 0.0
		pf := &importers.ParsedFile{
			Workouts: []importers.ParsedWorkout{
				{
					Date: "2025-01-15",
					Sets: []importers.ParsedWorkoutSet{
						{Exercise: "Pull-ups", Reps: 10, Weight: &w, RepType: "reps"},
					},
				},
			},
		}
		warnings := validateImportData(pf)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings for zero weight (bodyweight exercise), got %d", len(warnings))
		}
	})

	t.Run("nil weight and RPE are allowed", func(t *testing.T) {
		pf := &importers.ParsedFile{
			Workouts: []importers.ParsedWorkout{
				{
					Date: "2025-01-15",
					Sets: []importers.ParsedWorkoutSet{
						{Exercise: "Pull-ups", Reps: 10, RepType: "reps"},
					},
				},
			},
		}
		warnings := validateImportData(pf)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings for nil weight/RPE, got %d", len(warnings))
		}
	})

	t.Run("empty parsed file returns no warnings", func(t *testing.T) {
		pf := &importers.ParsedFile{}
		warnings := validateImportData(pf)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings for empty file, got %d", len(warnings))
		}
	})
}

func TestValidRepTypes(t *testing.T) {
	expected := []string{"reps", "seconds", "each_side"}
	for _, rt := range expected {
		if !validRepTypes[rt] {
			t.Errorf("expected %q to be a valid rep type", rt)
		}
	}

	invalid := []string{"", "invalid", "minutes", "REPS"}
	for _, rt := range invalid {
		if validRepTypes[rt] {
			t.Errorf("expected %q to NOT be a valid rep type", rt)
		}
	}
}
