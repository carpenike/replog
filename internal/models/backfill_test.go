package models

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// A failing set rolls back the workout and every earlier set.
func TestApplyBackfill_IsAtomic(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	a, err := CreateAthlete(ctx, db, "Atomic", "", "", "", "", "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatal(err)
	}
	ex, err := CreateExercise(ctx, db, "Goblet Squat", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	w := 50.0
	plan := &BackfillPlan{AthleteID: a.ID, Date: "2026-11-06", Sets: []BackfillSet{
		{SetOrder: 1, ExerciseID: ex.ID, SetNumber: 1, Reps: 8, RepType: "reps", WeightLbs: &w, SetType: SetTypeWorking},
		{SetOrder: 2, ExerciseID: 999999, SetNumber: 1, Reps: 8, RepType: "reps", SetType: SetTypeWorking},
	}}
	if _, err := ApplyBackfill(ctx, db, plan); err == nil {
		t.Fatal("expected the bad set to fail")
	}
	var n int
	if err := db.QueryRow(`SELECT (SELECT COUNT(*) FROM workouts) + (SELECT COUNT(*) FROM workout_sets)`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("partial write left %d rows", n)
	}

	plan.Sets = plan.Sets[:1]
	id, err := ApplyBackfill(ctx, db, plan)
	if err != nil || id == 0 {
		t.Fatalf("apply: %d %v", id, err)
	}
	if _, err := ApplyBackfill(ctx, db, plan); !errors.Is(err, ErrBackfillDuplicate) {
		t.Fatalf("second apply = %v, want duplicate", err)
	}
}
