package models

import (
	"context"
	"database/sql"
	"testing"

	"github.com/carpenike/replog/internal/importers"
)

// wodParsedCatalog builds a single-session ParsedFile (one program, one
// week/day) the way a generated WOD's CatalogJSON parses, with two
// prescribed sets — one for an exercise that already exists in the catalog
// and one for a brand-new exercise the log path must create.
func wodParsedCatalog(existingName, newName string) *importers.ParsedFile {
	reps2 := 5
	reps1 := 8
	w1 := 95.0
	w2 := 135.0
	return &importers.ParsedFile{
		Format: importers.FormatCatalogJSON,
		Exercises: []importers.ParsedExercise{
			{Name: existingName},
			{Name: newName},
		},
		Programs: []importers.ParsedProgram{
			{
				Template: importers.ParsedProgramTemplate{
					Name:     "Ad-hoc WOD",
					NumWeeks: 1,
					NumDays:  1,
					PrescribedSets: []importers.ParsedPrescribedSet{
						{Exercise: newName, Week: 1, Day: 1, SetNumber: 1, Reps: &reps1, RepType: "reps", AbsoluteWeight: &w1, SortOrder: 1},
						{Exercise: existingName, Week: 1, Day: 1, SetNumber: 1, Reps: &reps2, RepType: "reps", AbsoluteWeight: &w2, SortOrder: 2},
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
	if _, err := CreateExercise(context.Background(), db, "Power Clean", "", "", "", 0); err != nil {
		t.Fatalf("create exercise: %v", err)
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

	// The missing exercise was created during the log.
	if _, err := getExerciseByName(context.Background(), db, "Battle Rope Wave"); err != nil {
		t.Errorf("expected 'Battle Rope Wave' to be created: %v", err)
	}
}

func TestLogWODFromCatalog_CollisionAndReplace(t *testing.T) {
	db := testDB(t)

	athlete, err := CreateAthlete(context.Background(), db, "WOD Athlete", "", "", "", "", "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}
	if _, err := CreateExercise(context.Background(), db, "Goblet Squat", "", "", "", 0); err != nil {
		t.Fatalf("create exercise: %v", err)
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
