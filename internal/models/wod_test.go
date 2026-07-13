package models

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/carpenike/replog/internal/importers"
)

// wodParsedCatalog builds a single-session ParsedFile (one program, one
// week/day) the way a generated WOD's CatalogJSON parses, with two
// prescribed sets for the two named exercises.
func wodParsedCatalog(nameA, nameB string) *importers.ParsedFile {
	reps2 := 5
	reps1 := 8
	w1 := 95.0
	w2 := 135.0
	return &importers.ParsedFile{
		Format: importers.FormatCatalogJSON,
		Programs: []importers.ParsedProgram{
			{
				Template: importers.ParsedProgramTemplate{
					Name:     "Ad-hoc WOD",
					NumWeeks: 1,
					NumDays:  1,
					PrescribedSets: []importers.ParsedPrescribedSet{
						{Exercise: nameB, Week: 1, Day: 1, SetNumber: 1, Reps: &reps1, RepType: "reps", AbsoluteWeight: &w1, SortOrder: 1},
						{Exercise: nameA, Week: 1, Day: 1, SetNumber: 1, Reps: &reps2, RepType: "reps", AbsoluteWeight: &w2, SortOrder: 2},
					},
				},
			},
		},
	}
}

func TestLogWODFromCatalog_SeedsResistanceWorkout(t *testing.T) {
	db := testDB(t)

	athlete, err := CreateAthlete(context.Background(), db, "WOD Athlete", "", "", "", "", "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}
	for _, name := range []string{"Power Clean", "Battle Rope Wave"} {
		if _, err := CreateExercise(context.Background(), db, name, "", "", "", 0); err != nil {
			t.Fatalf("create exercise %q: %v", name, err)
		}
	}

	parsed := wodParsedCatalog("Power Clean", "Battle Rope Wave")
	result, err := LogWODFromCatalog(context.Background(), db, athlete.ID, "2026-06-20", parsed, false)
	if err != nil {
		t.Fatalf("LogWODFromCatalog: %v", err)
	}
	if result.Replaced {
		t.Error("expected Replaced=false on first log")
	}
	if result.SetsCreated != 2 {
		t.Errorf("expected 2 sets created, got %d", result.SetsCreated)
	}

	// The committed workout is a resistance, assignment-less ad-hoc session.
	wk, err := GetWorkoutByID(context.Background(), db, result.WorkoutID)
	if err != nil {
		t.Fatalf("get workout: %v", err)
	}
	if wk.Discipline != "resistance" {
		t.Errorf("expected discipline=resistance, got %q", wk.Discipline)
	}
	if wk.AssignmentID.Valid {
		t.Errorf("expected assignment_id NULL, got %d", wk.AssignmentID.Int64)
	}

	// It must surface in ListWorkouts (the resistance read path that feeds
	// BuildAthleteContext) — the feedback-loop guarantee.
	page, err := ListWorkouts(context.Background(), db, athlete.ID, 0)
	if err != nil {
		t.Fatalf("list workouts: %v", err)
	}
	if len(page.Workouts) != 1 {
		t.Fatalf("expected 1 resistance workout in ListWorkouts, got %d", len(page.Workouts))
	}
}

// TestLogWODFromCatalog_RejectsUnknownExercises asserts the ADR 020 follow-up
// behavior: a WOD prescribing exercise names outside the catalog is rejected
// with *UnknownExercisesError (naming every offender), auto-creates nothing,
// and — crucially — rejects BEFORE a replace deletes the existing workout.
func TestLogWODFromCatalog_RejectsUnknownExercises(t *testing.T) {
	db := testDB(t)

	athlete, err := CreateAthlete(context.Background(), db, "WOD Athlete", "", "", "", "", "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}
	for _, name := range []string{"Power Clean", "Goblet Squat"} {
		if _, err := CreateExercise(context.Background(), db, name, "", "", "", 0); err != nil {
			t.Fatalf("create exercise %q: %v", name, err)
		}
	}

	date := "2026-06-20"

	// Log a valid WOD first so the replace path has something to protect.
	valid := wodParsedCatalog("Power Clean", "Goblet Squat")
	first, err := LogWODFromCatalog(context.Background(), db, athlete.ID, date, valid, false)
	if err != nil {
		t.Fatalf("valid log: %v", err)
	}

	// Replace with a WOD naming an invented exercise → rejected.
	bad := wodParsedCatalog("Power Clean", "Quantum Burpee")
	_, err = LogWODFromCatalog(context.Background(), db, athlete.ID, date, bad, true)
	var unknownErr *UnknownExercisesError
	if !errors.As(err, &unknownErr) {
		t.Fatalf("expected *UnknownExercisesError, got %v", err)
	}
	if len(unknownErr.Names) != 1 || unknownErr.Names[0] != "Quantum Burpee" {
		t.Errorf("unknown names = %v, want [Quantum Burpee]", unknownErr.Names)
	}

	// The invented exercise was NOT created in the catalog.
	if _, err := getExerciseByName(context.Background(), db, "Quantum Burpee"); err == nil {
		t.Error("invented exercise must not be auto-created")
	}

	// The existing same-day workout survived the failed replace.
	if _, err := GetWorkoutByID(context.Background(), db, first.WorkoutID); err != nil {
		t.Errorf("existing workout must survive a rejected replace: %v", err)
	}
}

func TestLogWODFromCatalog_CollisionAndReplace(t *testing.T) {
	db := testDB(t)

	athlete, err := CreateAthlete(context.Background(), db, "WOD Athlete", "", "", "", "", "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}
	for _, name := range []string{"Goblet Squat", "Sled Push"} {
		if _, err := CreateExercise(context.Background(), db, name, "", "", "", 0); err != nil {
			t.Fatalf("create exercise %q: %v", name, err)
		}
	}

	date := "2026-06-21"
	parsed := wodParsedCatalog("Goblet Squat", "Sled Push")

	first, err := LogWODFromCatalog(context.Background(), db, athlete.ID, date, parsed, false)
	if err != nil {
		t.Fatalf("first log: %v", err)
	}

	// Second log on the same date without replace must collide.
	if _, err := LogWODFromCatalog(context.Background(), db, athlete.ID, date, parsed, false); err != ErrWODCollision {
		t.Fatalf("expected ErrWODCollision, got %v", err)
	}

	// With replace=true it supersedes the existing resistance workout.
	second, err := LogWODFromCatalog(context.Background(), db, athlete.ID, date, parsed, true)
	if err != nil {
		t.Fatalf("replace log: %v", err)
	}
	if !second.Replaced {
		t.Error("expected Replaced=true on replace")
	}
	if second.WorkoutID == first.WorkoutID {
		t.Error("expected a new workout id after replace")
	}

	// The old workout is gone; exactly one resistance workout remains.
	if _, err := GetWorkoutByID(context.Background(), db, first.WorkoutID); err != ErrNotFound {
		t.Errorf("expected old workout %d deleted, got %v", first.WorkoutID, err)
	}
	page, err := ListWorkouts(context.Background(), db, athlete.ID, 0)
	if err != nil {
		t.Fatalf("list workouts: %v", err)
	}
	if len(page.Workouts) != 1 {
		t.Errorf("expected 1 workout after replace, got %d", len(page.Workouts))
	}
}
