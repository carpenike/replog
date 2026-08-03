package models

import (
	"context"
	"database/sql"
	"testing"
)

func TestSetCRUD(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(context.Background(), db, "Set Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	e, _ := CreateExercise(context.Background(), db, "Test Lift", "", "", "", 0)
	w, _ := CreateWorkout(context.Background(), db, a.ID, "2026-05-01", "", 0)

	t.Run("add sets with auto set_number", func(t *testing.T) {
		s1, err := AddSet(context.Background(), db, w.ID, e.ID, 5, 135, 0, "", "", "easy")
		if err != nil {
			t.Fatalf("add set 1: %v", err)
		}
		if s1.SetNumber != 1 {
			t.Errorf("set_number = %d, want 1", s1.SetNumber)
		}
		if s1.Reps != 5 {
			t.Errorf("reps = %d, want 5", s1.Reps)
		}

		s2, err := AddSet(context.Background(), db, w.ID, e.ID, 5, 155, 0, "", "", "")
		if err != nil {
			t.Fatalf("add set 2: %v", err)
		}
		if s2.SetNumber != 2 {
			t.Errorf("set_number = %d, want 2", s2.SetNumber)
		}
	})

	t.Run("bodyweight set (null weight)", func(t *testing.T) {
		s, err := AddSet(context.Background(), db, w.ID, e.ID, 20, 0, 0, "", "", "")
		if err != nil {
			t.Fatalf("add bodyweight set: %v", err)
		}
		if s.Weight.Valid {
			t.Errorf("weight should be null for bodyweight, got %f", s.Weight.Float64)
		}
	})
}

// ptr returns a pointer to v — a test helper for the pointer-based partial
// UpdateSet signature (nil = leave unchanged).
func ptr[T any](v T) *T { return &v }

func TestUpdateSet(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(context.Background(), db, "Update Set Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	e, _ := CreateExercise(context.Background(), db, "Update Lift", "", "", "", 0)
	w, _ := CreateWorkout(context.Background(), db, a.ID, "2026-06-01", "", 0)
	s, _ := AddSet(context.Background(), db, w.ID, e.ID, 5, 100, 0, "", "", "")

	updated, err := UpdateSet(context.Background(), db, s.ID, a.ID, ptr(8), ptr(110.0), nil, ptr("form felt better"))
	if err != nil {
		t.Fatalf("update set: %v", err)
	}
	if updated.Reps != 8 {
		t.Errorf("reps = %d, want 8", updated.Reps)
	}
	if !updated.Weight.Valid || updated.Weight.Float64 != 110 {
		t.Errorf("weight = %v, want 110", updated.Weight)
	}
	if !updated.Notes.Valid || updated.Notes.String != "form felt better" {
		t.Errorf("notes = %v, want form felt better", updated.Notes)
	}
}

// TestUpdateSet_PreservesOmittedFields is the regression guard for the
// partial-update data-loss bug (HIGH): updating ONLY notes must not wipe the
// set's weight and RPE (which would corrupt the training-load / ACWR history).
func TestUpdateSet_PreservesOmittedFields(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(context.Background(), db, "Preserve Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	e, _ := CreateExercise(context.Background(), db, "Preserve Lift", "", "", "", 0)
	w, _ := CreateWorkout(context.Background(), db, a.ID, "2026-06-02", "", 0)
	s, _ := AddSet(context.Background(), db, w.ID, e.ID, 5, 225, 8.5, "", "", "first attempt")

	// Update ONLY notes; reps/weight/rpe are nil (leave unchanged).
	updated, err := UpdateSet(context.Background(), db, s.ID, a.ID, nil, nil, nil, ptr("felt strong"))
	if err != nil {
		t.Fatalf("update notes only: %v", err)
	}
	if updated.Reps != 5 {
		t.Errorf("reps = %d, want 5 (must be preserved)", updated.Reps)
	}
	if !updated.Weight.Valid || updated.Weight.Float64 != 225 {
		t.Errorf("weight = %v, want 225 (must be preserved)", updated.Weight)
	}
	if !updated.RPE.Valid || updated.RPE.Float64 != 8.5 {
		t.Errorf("rpe = %v, want 8.5 (must be preserved)", updated.RPE)
	}
	if !updated.Notes.Valid || updated.Notes.String != "felt strong" {
		t.Errorf("notes = %v, want felt strong", updated.Notes)
	}

	// A supplied 0 weight explicitly clears the column.
	cleared, err := UpdateSet(context.Background(), db, s.ID, a.ID, nil, ptr(0.0), nil, nil)
	if err != nil {
		t.Fatalf("clear weight: %v", err)
	}
	if cleared.Weight.Valid {
		t.Errorf("weight = %v, want cleared (NULL)", cleared.Weight)
	}
	// RPE and notes untouched by the weight-only update.
	if !cleared.RPE.Valid || cleared.RPE.Float64 != 8.5 {
		t.Errorf("rpe = %v, want 8.5 still preserved", cleared.RPE)
	}
}

func TestDeleteSet(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(context.Background(), db, "Del Set Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	e, _ := CreateExercise(context.Background(), db, "Del Lift", "", "", "", 0)
	w, _ := CreateWorkout(context.Background(), db, a.ID, "2026-07-01", "", 0)
	s, _ := AddSet(context.Background(), db, w.ID, e.ID, 5, 100, 0, "", "", "")

	if err := DeleteSet(context.Background(), db, s.ID, a.ID); err != nil {
		t.Fatalf("delete set: %v", err)
	}
	_, err := GetSetByID(context.Background(), db, s.ID)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestListSetsByWorkout(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(context.Background(), db, "Group Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	e1, _ := CreateExercise(context.Background(), db, "Lift A", "", "", "", 0)
	e2, _ := CreateExercise(context.Background(), db, "Lift B", "", "", "", 0)
	w, _ := CreateWorkout(context.Background(), db, a.ID, "2026-08-01", "", 0)

	AddSet(context.Background(), db, w.ID, e1.ID, 5, 100, 0, "", "", "")
	AddSet(context.Background(), db, w.ID, e1.ID, 5, 110, 0, "", "", "")
	AddSet(context.Background(), db, w.ID, e2.ID, 10, 50, 0, "", "", "")

	groups, err := ListSetsByWorkout(context.Background(), db, w.ID)
	if err != nil {
		t.Fatalf("list sets: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}
}

func TestDeleteSet_Renumbers(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(context.Background(), db, "Renum Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	e, _ := CreateExercise(context.Background(), db, "Renum Lift", "", "", "", 0)
	w, _ := CreateWorkout(context.Background(), db, a.ID, "2026-09-01", "", 0)

	s1, _ := AddSet(context.Background(), db, w.ID, e.ID, 5, 100, 0, "", "", "")
	s2, _ := AddSet(context.Background(), db, w.ID, e.ID, 5, 110, 0, "", "", "")
	s3, _ := AddSet(context.Background(), db, w.ID, e.ID, 5, 120, 0, "", "", "")

	// Delete the middle set.
	if err := DeleteSet(context.Background(), db, s2.ID, a.ID); err != nil {
		t.Fatalf("delete middle set: %v", err)
	}

	// Remaining sets should be renumbered 1, 2.
	got1, _ := GetSetByID(context.Background(), db, s1.ID)
	got3, _ := GetSetByID(context.Background(), db, s3.ID)

	if got1.SetNumber != 1 {
		t.Errorf("s1 set_number = %d, want 1", got1.SetNumber)
	}
	if got3.SetNumber != 2 {
		t.Errorf("s3 set_number = %d, want 2", got3.SetNumber)
	}
}

func TestDeleteSet_NotFound(t *testing.T) {
	db := testDB(t)

	if err := DeleteSet(context.Background(), db, 99999, 1); err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// TestSetOps_ScopedToAthlete guards against the cross-athlete IDOR: a set that
// belongs to athlete A must be invisible to (unmodifiable by) a call scoped to
// athlete B, even though the set ID is globally valid.
func TestSetOps_ScopedToAthlete(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(context.Background(), db, "Owner", "", "", "", "", "", "", sql.NullInt64{}, true)
	b, _ := CreateAthlete(context.Background(), db, "Other", "", "", "", "", "", "", sql.NullInt64{}, true)
	e, _ := CreateExercise(context.Background(), db, "Scoped Lift", "", "", "", 0)
	w, _ := CreateWorkout(context.Background(), db, a.ID, "2026-10-01", "", 0)
	s, _ := AddSet(context.Background(), db, w.ID, e.ID, 5, 100, 0, "", "", "")

	if _, err := UpdateSet(context.Background(), db, s.ID, b.ID, ptr(9), ptr(999.0), nil, ptr("hax")); err != ErrNotFound {
		t.Errorf("cross-athlete UpdateSet err = %v, want ErrNotFound", err)
	}
	if err := DeleteSet(context.Background(), db, s.ID, b.ID); err != ErrNotFound {
		t.Errorf("cross-athlete DeleteSet err = %v, want ErrNotFound", err)
	}
	// The set must still exist and be untouched.
	got, err := GetSetByID(context.Background(), db, s.ID)
	if err != nil {
		t.Fatalf("set should survive cross-athlete tamper: %v", err)
	}
	if got.Reps != 5 {
		t.Errorf("reps mutated cross-athlete: got %d, want 5", got.Reps)
	}
}

func TestAddMultipleSets(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(context.Background(), db, "Multi Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	e, _ := CreateExercise(context.Background(), db, "Multi Lift", "", "", "", 0)
	w, _ := CreateWorkout(context.Background(), db, a.ID, "2026-10-01", "", 0)

	t.Run("creates correct number of sets", func(t *testing.T) {
		sets, err := AddMultipleSets(context.Background(), db, w.ID, e.ID, 5, 5, 135, 0, "", "", "")
		if err != nil {
			t.Fatalf("add multiple sets: %v", err)
		}
		if len(sets) != 5 {
			t.Fatalf("got %d sets, want 5", len(sets))
		}
		for i, s := range sets {
			if s.SetNumber != i+1 {
				t.Errorf("set %d: set_number = %d, want %d", i, s.SetNumber, i+1)
			}
			if s.Reps != 5 {
				t.Errorf("set %d: reps = %d, want 5", i, s.Reps)
			}
			if !s.Weight.Valid || s.Weight.Float64 != 135 {
				t.Errorf("set %d: weight = %v, want 135", i, s.Weight)
			}
		}
	})

	t.Run("continues numbering after existing sets", func(t *testing.T) {
		// Already have 5 sets (1-5), adding 3 more should start at 6.
		sets, err := AddMultipleSets(context.Background(), db, w.ID, e.ID, 3, 3, 155, 0, "", "", "")
		if err != nil {
			t.Fatalf("add more sets: %v", err)
		}
		if sets[0].SetNumber != 6 {
			t.Errorf("first new set_number = %d, want 6", sets[0].SetNumber)
		}
		if sets[2].SetNumber != 8 {
			t.Errorf("last new set_number = %d, want 8", sets[2].SetNumber)
		}
	})

	t.Run("count=1 delegates to AddSet", func(t *testing.T) {
		e2, _ := CreateExercise(context.Background(), db, "Single Lift", "", "", "", 0)
		sets, err := AddMultipleSets(context.Background(), db, w.ID, e2.ID, 1, 10, 50, 0, "", "", "")
		if err != nil {
			t.Fatalf("add single set via multi: %v", err)
		}
		if len(sets) != 1 {
			t.Fatalf("got %d sets, want 1", len(sets))
		}
		if sets[0].SetNumber != 1 {
			t.Errorf("set_number = %d, want 1", sets[0].SetNumber)
		}
	})

	t.Run("count=0 returns error", func(t *testing.T) {
		_, err := AddMultipleSets(context.Background(), db, w.ID, e.ID, 0, 5, 100, 0, "", "", "")
		if err == nil {
			t.Error("expected error for count=0")
		}
	})

	t.Run("preserves RPE and notes", func(t *testing.T) {
		e3, _ := CreateExercise(context.Background(), db, "RPE Lift", "", "", "", 0)
		sets, err := AddMultipleSets(context.Background(), db, w.ID, e3.ID, 2, 5, 100, 8.5, "", "", "heavy")
		if err != nil {
			t.Fatalf("add sets with RPE: %v", err)
		}
		for i, s := range sets {
			if !s.RPE.Valid || s.RPE.Float64 != 8.5 {
				t.Errorf("set %d: RPE = %v, want 8.5", i, s.RPE)
			}
			if !s.Notes.Valid || s.Notes.String != "heavy" {
				t.Errorf("set %d: notes = %v, want heavy", i, s.Notes)
			}
		}
	})
}

func TestSeedSetsFromPrescription(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(context.Background(), db, "Seed Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	squat, _ := CreateExercise(context.Background(), db, "Seed Squat", "", "", "", 0)
	chin, _ := CreateExercise(context.Background(), db, "Seed Chin", "", "", "", 0)
	w, _ := CreateWorkout(context.Background(), db, a.ID, "2026-11-01", "", 0)

	target := 185.0
	p := &Prescription{
		Lines: []*PrescriptionLine{
			{
				ExerciseID:   squat.ID,
				ExerciseName: "Seed Squat",
				Sets: []*PrescribedSet{
					{ExerciseID: squat.ID, SetNumber: 1, Reps: sql.NullInt64{Int64: 15, Valid: true}, RepType: "reps", TargetWeight: &target},
				},
			},
			{
				// Bodyweight AMRAP line: no reps, no target weight.
				ExerciseID:   chin.ID,
				ExerciseName: "Seed Chin",
				Sets: []*PrescribedSet{
					{ExerciseID: chin.ID, SetNumber: 1, RepType: "reps"},
				},
			},
		},
	}

	n, err := SeedSetsFromPrescription(context.Background(), db, w.ID, p)
	if err != nil {
		t.Fatalf("seed sets: %v", err)
	}
	if n != 2 {
		t.Fatalf("seeded %d sets, want 2", n)
	}

	groups, err := ListSetsByWorkout(context.Background(), db, w.ID)
	if err != nil {
		t.Fatalf("list sets: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("got %d exercise groups, want 2", len(groups))
	}

	var squatSet *WorkoutSet
	var chinSet *WorkoutSet
	for _, g := range groups {
		for _, s := range g.Sets {
			switch s.ExerciseID {
			case squat.ID:
				squatSet = s
			case chin.ID:
				chinSet = s
			}
		}
	}
	if squatSet == nil || chinSet == nil {
		t.Fatal("missing seeded sets")
		return
	}
	// Weighted set keeps reps + target weight.
	if squatSet.Reps != 15 {
		t.Errorf("squat reps = %d, want 15", squatSet.Reps)
	}
	if !squatSet.Weight.Valid || squatSet.Weight.Float64 != 185 {
		t.Errorf("squat weight = %v, want 185", squatSet.Weight)
	}
	// AMRAP/bodyweight seeds reps=0 and null weight for the athlete to fill in.
	if chinSet.Reps != 0 {
		t.Errorf("chin reps = %d, want 0", chinSet.Reps)
	}
	if chinSet.Weight.Valid {
		t.Errorf("chin weight should be null, got %f", chinSet.Weight.Float64)
	}
	if chinSet.RPE.Valid {
		t.Errorf("seeded set RPE should be null, got %f", chinSet.RPE.Float64)
	}
}

func TestSeedSetsFromPrescription_NilOrEmpty(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(context.Background(), db, "Empty Seed Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	w, _ := CreateWorkout(context.Background(), db, a.ID, "2026-11-02", "", 0)

	if n, err := SeedSetsFromPrescription(context.Background(), db, w.ID, nil); err != nil || n != 0 {
		t.Errorf("nil prescription: n=%d err=%v, want 0/nil", n, err)
	}
	if n, err := SeedSetsFromPrescription(context.Background(), db, w.ID, &Prescription{}); err != nil || n != 0 {
		t.Errorf("empty prescription: n=%d err=%v, want 0/nil", n, err)
	}
}

func TestListExerciseHistory(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(context.Background(), db, "History Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	e, _ := CreateExercise(context.Background(), db, "History Lift", "", "", "", 0)

	t.Run("empty history", func(t *testing.T) {
		page, err := ListExerciseHistory(context.Background(), db, a.ID, e.ID, 0)
		if err != nil {
			t.Fatalf("list exercise history: %v", err)
		}
		if len(page.Days) != 0 {
			t.Errorf("days = %d, want 0", len(page.Days))
		}
		if page.HasMore {
			t.Error("hasMore should be false for empty")
		}
	})

	// Create some workouts with sets.
	w1, _ := CreateWorkout(context.Background(), db, a.ID, "2026-01-01", "", 0)
	AddSet(context.Background(), db, w1.ID, e.ID, 5, 100, 0, "", "", "")
	AddSet(context.Background(), db, w1.ID, e.ID, 5, 110, 0, "", "", "")

	w2, _ := CreateWorkout(context.Background(), db, a.ID, "2026-01-02", "", 0)
	AddSet(context.Background(), db, w2.ID, e.ID, 3, 130, 0, "", "", "")

	t.Run("with data", func(t *testing.T) {
		page, err := ListExerciseHistory(context.Background(), db, a.ID, e.ID, 0)
		if err != nil {
			t.Fatalf("list exercise history: %v", err)
		}
		if len(page.Days) != 2 {
			t.Fatalf("days = %d, want 2", len(page.Days))
		}
		// Most recent first.
		if page.Days[0].WorkoutID != w2.ID {
			t.Errorf("first day workout = %d, want %d", page.Days[0].WorkoutID, w2.ID)
		}
		if len(page.Days[0].Sets) != 1 {
			t.Errorf("first day sets = %d, want 1", len(page.Days[0].Sets))
		}
		if len(page.Days[1].Sets) != 2 {
			t.Errorf("second day sets = %d, want 2", len(page.Days[1].Sets))
		}
	})

	t.Run("different exercise not included", func(t *testing.T) {
		e2, _ := CreateExercise(context.Background(), db, "Other Lift", "", "", "", 0)
		page, err := ListExerciseHistory(context.Background(), db, a.ID, e2.ID, 0)
		if err != nil {
			t.Fatalf("list exercise history: %v", err)
		}
		if len(page.Days) != 0 {
			t.Errorf("days = %d, want 0", len(page.Days))
		}
	})
}

func TestListRecentSetsForExercise(t *testing.T) {
	db := testDB(t)

	a1, _ := CreateAthlete(context.Background(), db, "Athlete A", "", "", "", "", "", "", sql.NullInt64{}, true)
	a2, _ := CreateAthlete(context.Background(), db, "Athlete B", "", "", "", "", "", "", sql.NullInt64{}, true)
	e, _ := CreateExercise(context.Background(), db, "Shared Lift", "", "", "", 0)

	w1, _ := CreateWorkout(context.Background(), db, a1.ID, "2026-01-01", "", 0)
	AddSet(context.Background(), db, w1.ID, e.ID, 5, 135, 0, "", "", "")

	w2, _ := CreateWorkout(context.Background(), db, a2.ID, "2026-01-02", "", 0)
	AddSet(context.Background(), db, w2.ID, e.ID, 8, 95, 0, "", "", "")

	sets, err := ListRecentSetsForExercise(context.Background(), db, e.ID)
	if err != nil {
		t.Fatalf("list recent sets: %v", err)
	}
	if len(sets) != 2 {
		t.Fatalf("count = %d, want 2", len(sets))
	}
	// Most recent date first.
	if sets[0].AthleteName != "Athlete B" {
		t.Errorf("first set athlete = %q, want Athlete B", sets[0].AthleteName)
	}

	t.Run("empty for unused exercise", func(t *testing.T) {
		e2, _ := CreateExercise(context.Background(), db, "Unused Lift", "", "", "", 0)
		sets, err := ListRecentSetsForExercise(context.Background(), db, e2.ID)
		if err != nil {
			t.Fatalf("list recent sets: %v", err)
		}
		if len(sets) != 0 {
			t.Errorf("count = %d, want 0", len(sets))
		}
	})
}
