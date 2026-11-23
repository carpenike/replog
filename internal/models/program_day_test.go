package models

import (
	"context"
	"database/sql"
	"testing"
)

// Rows without a stored day keep the counted day; once a session stores
// one, the next day follows it rather than the count.
func TestSlotFor_StoredThenCountFallback(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	a, err := CreateAthlete(ctx, db, "Slots", "", "", "", "", "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := CreateProgramTemplate(ctx, db, nil, "Loop", "", 2, 3, true, "")
	if err != nil {
		t.Fatal(err)
	}
	program, err := AssignProgram(ctx, db, a.ID, tpl.ID, "2026-11-02", "", "", "primary", "[1,3,5]")
	if err != nil {
		t.Fatal(err)
	}
	program, err = GetAthleteProgramByID(ctx, db, program.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Two pre-0028 rows: counted.
	for _, d := range []string{"2026-11-02", "2026-11-04"} {
		if _, err := CreateWorkout(ctx, db, a.ID, d, "", program.ID); err != nil {
			t.Fatal(err)
		}
	}
	week, day, source, err := NextProgramDay(ctx, db, program, "2026-11-06")
	if err != nil || week != 1 || day != 3 || source != DaySourceCount {
		t.Fatalf("count = W%d D%d %s %v", week, day, source, err)
	}
	// A stored Week 2 Day 2 on Friday: Monday is Week 2 Day 3.
	w, err := CreateWorkoutOnDay(ctx, db, a.ID, "2026-11-06", "", program.ID, 2, 2)
	if err != nil || w.ProgramWeek.Int64 != 2 || w.ProgramDay.Int64 != 2 {
		t.Fatalf("stored = %+v %v", w, err)
	}
	week, day, source, _ = NextProgramDay(ctx, db, program, "2026-11-09")
	if week != 2 || day != 3 || source != DaySourceNext {
		t.Fatalf("next = W%d D%d %s", week, day, source)
	}
	// Friday itself reads its stored day.
	week, day, source, _ = NextProgramDay(ctx, db, program, "2026-11-06")
	if week != 2 || day != 2 || source != DaySourceStored {
		t.Fatalf("stored read = W%d D%d %s", week, day, source)
	}
	// Unlinked workouts never store a day.
	u, err := CreateWorkoutOnDay(ctx, db, a.ID, "2026-11-10", "", 0, 1, 1)
	if err != nil || u.ProgramWeek.Valid || u.ProgramDay.Valid {
		t.Fatalf("unlinked = %+v %v", u, err)
	}
}

func TestCalendarProgramWeek(t *testing.T) {
	for _, c := range []struct {
		date  string
		weeks int
		loop  bool
		want  int
	}{
		{"2026-11-06", 8, true, 1}, // Thursday start: Mon 2 – Sun 8 Nov is week 1
		{"2026-11-08", 8, true, 1},
		{"2026-11-09", 8, true, 2},
		{"2026-11-20", 8, true, 3},
		{"2026-11-09", 1, true, 1}, // a loop wraps
		{"2026-11-09", 1, false, 0},
		{"2026-11-04", 8, true, 0}, // before the start
	} {
		if got := CalendarProgramWeek("2026-11-05T00:00:00Z", c.date, c.weeks, c.loop); got != c.want {
			t.Errorf("%s weeks=%d loop=%v: %d, want %d", c.date, c.weeks, c.loop, got, c.want)
		}
	}
}
