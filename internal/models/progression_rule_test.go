package models

import (
	"errors"
	"testing"
)

func TestSetProgressionRule(t *testing.T) {
	db := testDB(t)

	// Seed exercise and template.
	ex, err := CreateExercise(db, "Squat", "", "", "", 0)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}
	tmpl, err := CreateProgramTemplate(db, nil, "5/3/1", "", 4, 4, false)
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	t.Run("create new rule", func(t *testing.T) {
		pr, err := SetProgressionRule(db, tmpl.ID, ex.ID, 10.0)
		if err != nil {
			t.Fatalf("set progression rule: %v", err)
		}
		if pr.Increment != 10.0 {
			t.Errorf("expected increment 10.0, got %f", pr.Increment)
		}
		if pr.ExerciseName != "Squat" {
			t.Errorf("expected exercise name Squat, got %q", pr.ExerciseName)
		}
	})

	t.Run("upsert updates existing", func(t *testing.T) {
		pr, err := SetProgressionRule(db, tmpl.ID, ex.ID, 5.0)
		if err != nil {
			t.Fatalf("upsert progression rule: %v", err)
		}
		if pr.Increment != 5.0 {
			t.Errorf("expected increment 5.0 after upsert, got %f", pr.Increment)
		}
	})
}

func TestGetProgressionRule_NotFound(t *testing.T) {
	db := testDB(t)

	_, err := GetProgressionRule(db, 999, 999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListProgressionRules(t *testing.T) {
	db := testDB(t)

	tmpl, err := CreateProgramTemplate(db, nil, "5/3/1", "", 4, 4, false)
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	squat, err := CreateExercise(db, "Squat", "", "", "", 0)
	if err != nil {
		t.Fatalf("create squat: %v", err)
	}
	bench, err := CreateExercise(db, "Bench Press", "", "", "", 0)
	if err != nil {
		t.Fatalf("create bench: %v", err)
	}

	SetProgressionRule(db, tmpl.ID, squat.ID, 10.0)
	SetProgressionRule(db, tmpl.ID, bench.ID, 5.0)

	rules, err := ListProgressionRules(db, tmpl.ID)
	if err != nil {
		t.Fatalf("list progression rules: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}

	// Should be ordered by name (Bench Press before Squat).
	if rules[0].ExerciseName != "Bench Press" {
		t.Errorf("expected first rule to be Bench Press, got %q", rules[0].ExerciseName)
	}
	if rules[1].ExerciseName != "Squat" {
		t.Errorf("expected second rule to be Squat, got %q", rules[1].ExerciseName)
	}
}

func TestDeleteProgressionRule(t *testing.T) {
	db := testDB(t)

	tmpl, err := CreateProgramTemplate(db, nil, "5/3/1", "", 4, 4, false)
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	ex, err := CreateExercise(db, "Squat", "", "", "", 0)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	pr, err := SetProgressionRule(db, tmpl.ID, ex.ID, 10.0)
	if err != nil {
		t.Fatalf("set rule: %v", err)
	}

	if err := DeleteProgressionRule(db, pr.ID); err != nil {
		t.Fatalf("delete rule: %v", err)
	}

	// Should not be found after deletion.
	_, err = GetProgressionRule(db, tmpl.ID, ex.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}

	// Deleting again should return ErrNotFound.
	if err := DeleteProgressionRule(db, pr.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound on second delete, got %v", err)
	}
}

func TestProgressionRule_IncrementLabel(t *testing.T) {
	tests := []struct {
		increment float64
		want      string
	}{
		{5.0, "5"},
		{10.0, "10"},
		{2.5, "2.5"},
		{7.5, "7.5"},
	}

	for _, tt := range tests {
		pr := &ProgressionRule{Increment: tt.increment}
		if got := pr.IncrementLabel(); got != tt.want {
			t.Errorf("IncrementLabel(%f) = %q, want %q", tt.increment, got, tt.want)
		}
	}
}

func TestProgressionRule_CascadeDeleteTemplate(t *testing.T) {
	db := testDB(t)

	tmpl, err := CreateProgramTemplate(db, nil, "Temp", "", 1, 1, false)
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	ex, err := CreateExercise(db, "Squat", "", "", "", 0)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	_, err = SetProgressionRule(db, tmpl.ID, ex.ID, 10.0)
	if err != nil {
		t.Fatalf("set rule: %v", err)
	}

	// Delete the template — rule should cascade.
	if err := DeleteProgramTemplate(db, tmpl.ID); err != nil {
		t.Fatalf("delete template: %v", err)
	}

	rules, err := ListProgressionRules(db, tmpl.ID)
	if err != nil {
		t.Fatalf("list rules after template delete: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules after template cascade delete, got %d", len(rules))
	}
}
