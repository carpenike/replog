package models

import (
	"database/sql"
	"testing"
)

func TestCreateAccessoryPlan(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	ex, _ := CreateExercise(db, "Bicep Curls", "", "", "", 0, false)

	t.Run("basic create", func(t *testing.T) {
		ap, err := CreateAccessoryPlan(db, a.ID, 1, ex.ID, 3, 10, 15, 25.0, "superset with pushdowns", 0)
		if err != nil {
			t.Fatalf("create accessory plan: %v", err)
		}
		if ap.AthleteID != a.ID {
			t.Errorf("athlete_id = %d, want %d", ap.AthleteID, a.ID)
		}
		if ap.Day != 1 {
			t.Errorf("day = %d, want 1", ap.Day)
		}
		if ap.ExerciseID != ex.ID {
			t.Errorf("exercise_id = %d, want %d", ap.ExerciseID, ex.ID)
		}
		if !ap.TargetSets.Valid || ap.TargetSets.Int64 != 3 {
			t.Errorf("target_sets = %v, want 3", ap.TargetSets)
		}
		if !ap.TargetRepMin.Valid || ap.TargetRepMin.Int64 != 10 {
			t.Errorf("target_rep_min = %v, want 10", ap.TargetRepMin)
		}
		if !ap.TargetRepMax.Valid || ap.TargetRepMax.Int64 != 15 {
			t.Errorf("target_rep_max = %v, want 15", ap.TargetRepMax)
		}
		if !ap.TargetWeight.Valid || ap.TargetWeight.Float64 != 25.0 {
			t.Errorf("target_weight = %v, want 25.0", ap.TargetWeight)
		}
		if !ap.Notes.Valid || ap.Notes.String != "superset with pushdowns" {
			t.Errorf("notes = %v, want 'superset with pushdowns'", ap.Notes)
		}
		if ap.ExerciseName != "Bicep Curls" {
			t.Errorf("exercise_name = %q, want 'Bicep Curls'", ap.ExerciseName)
		}
		if !ap.Active {
			t.Error("expected active = true")
		}
	})

	t.Run("duplicate athlete-day-exercise", func(t *testing.T) {
		_, err := CreateAccessoryPlan(db, a.ID, 1, ex.ID, 4, 8, 12, 30.0, "", 0)
		if err == nil {
			t.Fatal("expected error for duplicate, got nil")
		}
	})

	t.Run("nullable fields omitted", func(t *testing.T) {
		ex2, _ := CreateExercise(db, "Tricep Pushdowns", "", "", "", 0, false)
		ap, err := CreateAccessoryPlan(db, a.ID, 1, ex2.ID, 0, 0, 0, 0, "", 1)
		if err != nil {
			t.Fatalf("create accessory plan: %v", err)
		}
		if ap.TargetSets.Valid {
			t.Errorf("expected target_sets to be null, got %d", ap.TargetSets.Int64)
		}
		if ap.TargetRepMin.Valid {
			t.Errorf("expected target_rep_min to be null, got %d", ap.TargetRepMin.Int64)
		}
		if ap.Notes.Valid {
			t.Errorf("expected notes to be null, got %q", ap.Notes.String)
		}
	})
}

func TestListAccessoryPlansForDay(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	ex1, _ := CreateExercise(db, "Curls", "", "", "", 0, false)
	ex2, _ := CreateExercise(db, "Lateral Raises", "", "", "", 0, false)

	CreateAccessoryPlan(db, a.ID, 1, ex1.ID, 3, 10, 15, 0, "", 1)
	CreateAccessoryPlan(db, a.ID, 1, ex2.ID, 3, 12, 15, 0, "", 0)
	CreateAccessoryPlan(db, a.ID, 2, ex1.ID, 4, 8, 12, 0, "", 0)

	plans, err := ListAccessoryPlansForDay(db, a.ID, 1)
	if err != nil {
		t.Fatalf("list accessory plans for day: %v", err)
	}
	if len(plans) != 2 {
		t.Fatalf("got %d plans, want 2", len(plans))
	}
	// Sorted by sort_order: ex2(0) before ex1(1).
	if plans[0].ExerciseID != ex2.ID {
		t.Errorf("first plan exercise = %d, want %d (lower sort_order)", plans[0].ExerciseID, ex2.ID)
	}

	// Day 2 should have only 1.
	plans2, err := ListAccessoryPlansForDay(db, a.ID, 2)
	if err != nil {
		t.Fatalf("list accessory plans for day 2: %v", err)
	}
	if len(plans2) != 1 {
		t.Fatalf("got %d plans for day 2, want 1", len(plans2))
	}

	// Day 3 should have 0.
	plans3, err := ListAccessoryPlansForDay(db, a.ID, 3)
	if err != nil {
		t.Fatalf("list accessory plans for day 3: %v", err)
	}
	if len(plans3) != 0 {
		t.Fatalf("got %d plans for day 3, want 0", len(plans3))
	}
}

func TestUpdateAccessoryPlan(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	ex, _ := CreateExercise(db, "Curls", "", "", "", 0, false)
	ap, _ := CreateAccessoryPlan(db, a.ID, 1, ex.ID, 3, 10, 15, 25.0, "original", 0)

	err := UpdateAccessoryPlan(db, ap.ID, 4, 8, 12, 30.0, "updated", 1)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := GetAccessoryPlanByID(db, ap.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if !got.TargetSets.Valid || got.TargetSets.Int64 != 4 {
		t.Errorf("target_sets = %v, want 4", got.TargetSets)
	}
	if !got.Notes.Valid || got.Notes.String != "updated" {
		t.Errorf("notes = %v, want 'updated'", got.Notes)
	}
	if got.SortOrder != 1 {
		t.Errorf("sort_order = %d, want 1", got.SortOrder)
	}

	t.Run("not found", func(t *testing.T) {
		err := UpdateAccessoryPlan(db, 99999, 0, 0, 0, 0, "", 0)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestDeactivateAccessoryPlan(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	ex, _ := CreateExercise(db, "Curls", "", "", "", 0, false)
	ap, _ := CreateAccessoryPlan(db, a.ID, 1, ex.ID, 3, 10, 15, 0, "", 0)

	err := DeactivateAccessoryPlan(db, ap.ID)
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	got, err := GetAccessoryPlanByID(db, ap.ID)
	if err != nil {
		t.Fatalf("get after deactivate: %v", err)
	}
	if got.Active {
		t.Error("expected active = false after deactivation")
	}

	// Deactivated plan should not appear in ListForDay.
	plans, err := ListAccessoryPlansForDay(db, a.ID, 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(plans) != 0 {
		t.Errorf("got %d active plans, want 0", len(plans))
	}

	// But should appear in ListAll.
	all, err := ListAllAccessoryPlans(db, a.ID)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("got %d plans in list all, want 1", len(all))
	}
}

func TestDeleteAccessoryPlan(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	ex, _ := CreateExercise(db, "Curls", "", "", "", 0, false)
	ap, _ := CreateAccessoryPlan(db, a.ID, 1, ex.ID, 3, 10, 15, 0, "", 0)

	err := DeleteAccessoryPlan(db, ap.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = GetAccessoryPlanByID(db, ap.ID)
	if err != ErrNotFound {
		t.Errorf("after delete, err = %v, want ErrNotFound", err)
	}

	t.Run("not found", func(t *testing.T) {
		err := DeleteAccessoryPlan(db, 99999)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestMaxAccessoryDay(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)

	// No plans → 0.
	maxDay, err := MaxAccessoryDay(db, a.ID)
	if err != nil {
		t.Fatalf("max day: %v", err)
	}
	if maxDay != 0 {
		t.Errorf("max day = %d, want 0", maxDay)
	}

	ex, _ := CreateExercise(db, "Curls", "", "", "", 0, false)
	CreateAccessoryPlan(db, a.ID, 3, ex.ID, 0, 0, 0, 0, "", 0)

	maxDay, err = MaxAccessoryDay(db, a.ID)
	if err != nil {
		t.Fatalf("max day after create: %v", err)
	}
	if maxDay != 3 {
		t.Errorf("max day = %d, want 3", maxDay)
	}
}

func TestRepRangeLabel(t *testing.T) {
	tests := []struct {
		name string
		plan AccessoryPlan
		want string
	}{
		{
			name: "sets x min-max",
			plan: AccessoryPlan{
				TargetSets:   sql.NullInt64{Int64: 3, Valid: true},
				TargetRepMin: sql.NullInt64{Int64: 10, Valid: true},
				TargetRepMax: sql.NullInt64{Int64: 15, Valid: true},
			},
			want: "3×10-15",
		},
		{
			name: "sets x same reps",
			plan: AccessoryPlan{
				TargetSets:   sql.NullInt64{Int64: 4, Valid: true},
				TargetRepMin: sql.NullInt64{Int64: 8, Valid: true},
				TargetRepMax: sql.NullInt64{Int64: 8, Valid: true},
			},
			want: "4×8",
		},
		{
			name: "min only",
			plan: AccessoryPlan{
				TargetRepMin: sql.NullInt64{Int64: 10, Valid: true},
			},
			want: "10+",
		},
		{
			name: "nothing set",
			plan: AccessoryPlan{},
			want: "—",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.plan.RepRangeLabel()
			if got != tt.want {
				t.Errorf("RepRangeLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}
