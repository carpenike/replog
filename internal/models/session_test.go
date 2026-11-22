package models

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestReadinessIsLow(t *testing.T) {
	f := func(v float64) sql.NullFloat64 { return sql.NullFloat64{Float64: v, Valid: true} }
	i := func(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: true} }
	none := sql.NullInt64{}
	cases := []struct {
		name                 string
		sleep                sql.NullFloat64
		soreness, energy, md sql.NullInt64
		want                 bool
	}{
		{"all unanswered", sql.NullFloat64{}, none, none, none, false},
		{"good day", f(8), i(3), i(7), i(8), false},
		{"edge values are fine", f(6), i(7), i(4), i(4), false},
		{"short sleep", f(5.5), none, none, none, true},
		{"very sore", sql.NullFloat64{}, i(8), none, none, true},
		{"low energy", sql.NullFloat64{}, none, i(3), none, true},
		{"low mood", sql.NullFloat64{}, none, none, i(2), true},
	}
	for _, c := range cases {
		if got := ReadinessIsLow(c.sleep, c.soreness, c.energy, c.md); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestExerciseSwapModel(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	mk := func(name string, patterns ...string) *Exercise {
		e, err := CreateExercise(ctx, db, name, "", "", "", 0)
		if err != nil {
			t.Fatal(err)
		}
		if err := SetExerciseMovementPatterns(ctx, db, e.ID, patterns); err != nil {
			t.Fatal(err)
		}
		return e
	}
	squat := mk("Back Squat", "squat", "hinge")
	both := mk("Zercher Squat", "squat", "hinge")
	one := mk("Air Squat", "squat")
	press := mk("Bench Press", "push")

	subs, err := ListExerciseSubstitutes(ctx, db, squat.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 || subs[0].ID != both.ID || subs[1].ID != one.ID {
		t.Fatalf("substitutes ordered by shared patterns = %v", subs)
	}

	athlete, err := CreateAthlete(ctx, db, "Dana", "", "", "", "", "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatal(err)
	}
	w, err := CreateWorkout(ctx, db, athlete.ID, "2026-05-12", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateExerciseSwap(ctx, db, w.ID, squat.ID, press.ID, ""); !errors.Is(err, ErrNoSharedPattern) {
		t.Fatalf("cross-pattern swap err = %v", err)
	}
	s, err := CreateExerciseSwap(ctx, db, w.ID, squat.ID, one.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.Reason.Valid || s.ToName != "Air Squat" {
		t.Fatalf("swap = %+v", s)
	}
	if _, err := CreateExerciseSwap(ctx, db, w.ID, squat.ID, both.ID, ""); !errors.Is(err, ErrSwapExists) {
		t.Fatalf("second swap err = %v", err)
	}
	// The swapped-to exercise is protected like logged history.
	if err := DeleteExercise(ctx, db, one.ID); !errors.Is(err, ErrExerciseInUse) {
		t.Fatalf("delete swapped exercise err = %v", err)
	}
	if err := DeleteExerciseSwap(ctx, db, w.ID+1, s.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete from wrong workout err = %v", err)
	}
	if err := DeleteExerciseSwap(ctx, db, w.ID, s.ID); err != nil {
		t.Fatal(err)
	}
	list, err := ListExerciseSwaps(ctx, db, w.ID)
	if err != nil || len(list) != 0 {
		t.Fatalf("swaps after delete = %v, %v", list, err)
	}
}
