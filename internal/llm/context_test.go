package llm

import (
	"database/sql"
	"testing"
	"time"

	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/models"
)

func testDB(t testing.TB) *sql.DB {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := database.RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("run migrations: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func seedAthlete(t testing.TB, db *sql.DB, name, tier, goal string) int64 {
	t.Helper()
	a, err := models.CreateAthlete(db, name, tier, "", goal, sql.NullInt64{}, true)
	if err != nil {
		t.Fatalf("seed athlete: %v", err)
	}
	return a.ID
}

func seedExercise(t testing.TB, db *sql.DB, name, tier string) int64 {
	t.Helper()
	ex, err := models.CreateExercise(db, name, tier, "", "", 0)
	if err != nil {
		t.Fatalf("seed exercise: %v", err)
	}
	return ex.ID
}

func seedWorkout(t testing.TB, db *sql.DB, athleteID int64, date string) int64 {
	t.Helper()
	w, err := models.CreateWorkout(db, athleteID, date, "")
	if err != nil {
		t.Fatalf("seed workout: %v", err)
	}
	return w.ID
}

func TestBuildAthleteContext_Empty(t *testing.T) {
	db := testDB(t)
	athleteID := seedAthlete(t, db, "TestAthlete", "foundational", "get strong")

	ctx, err := BuildAthleteContext(db, athleteID, time.Now())
	if err != nil {
		t.Fatalf("BuildAthleteContext: %v", err)
	}
	if ctx.Athlete.Name != "TestAthlete" {
		t.Errorf("athlete name = %q, want TestAthlete", ctx.Athlete.Name)
	}
	if ctx.Athlete.Tier == nil || *ctx.Athlete.Tier != "foundational" {
		t.Errorf("tier = %v, want foundational", ctx.Athlete.Tier)
	}
	if ctx.Athlete.Goal == nil || *ctx.Athlete.Goal != "get strong" {
		t.Errorf("goal = %v, want get strong", ctx.Athlete.Goal)
	}
	if ctx.Athlete.TotalWorkouts != 0 {
		t.Errorf("total workouts = %d, want 0", ctx.Athlete.TotalWorkouts)
	}
	if ctx.CurrentProgram != nil {
		t.Errorf("current program should be nil, got %+v", ctx.CurrentProgram)
	}
	if len(ctx.Equipment) != 0 {
		t.Errorf("equipment should be empty, got %v", ctx.Equipment)
	}
	if ctx.Goals.Current != "get strong" {
		t.Errorf("goals.current = %q, want get strong", ctx.Goals.Current)
	}
}

func TestBuildAthleteContext_WithWorkouts(t *testing.T) {
	db := testDB(t)
	athleteID := seedAthlete(t, db, "Alice", "sport_performance", "volleyball")
	exID := seedExercise(t, db, "Squat", "sport_performance")

	w1ID := seedWorkout(t, db, athleteID, "2026-01-10")
	if _, err := models.AddSet(db, w1ID, exID, 5, 135.0, 7.0, "reps", ""); err != nil {
		t.Fatalf("add set: %v", err)
	}
	if _, err := models.AddSet(db, w1ID, exID, 5, 135.0, 7.5, "reps", ""); err != nil {
		t.Fatalf("add set: %v", err)
	}

	w2ID := seedWorkout(t, db, athleteID, "2026-01-12")
	if _, err := models.AddSet(db, w2ID, exID, 5, 140.0, 8.0, "reps", ""); err != nil {
		t.Fatalf("add set: %v", err)
	}

	now := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
	ctx, err := BuildAthleteContext(db, athleteID, now)
	if err != nil {
		t.Fatalf("BuildAthleteContext: %v", err)
	}
	if ctx.Athlete.TotalWorkouts != 2 {
		t.Errorf("total workouts = %d, want 2", ctx.Athlete.TotalWorkouts)
	}
	if ctx.Athlete.TrainingMonths < 1 {
		t.Errorf("training months = %d, want >= 1", ctx.Athlete.TrainingMonths)
	}
	if len(ctx.RecentWorkouts) != 2 {
		t.Fatalf("recent workouts = %d, want 2", len(ctx.RecentWorkouts))
	}
	if ctx.RecentWorkouts[0].Date != "2026-01-12" {
		t.Errorf("first workout date = %q, want 2026-01-12", ctx.RecentWorkouts[0].Date)
	}
}

func TestBuildAthleteContext_WithTrainingMaxes(t *testing.T) {
	db := testDB(t)
	athleteID := seedAthlete(t, db, "Bob", "intermediate", "")
	exID := seedExercise(t, db, "Bench Press", "intermediate")

	if _, err := models.AssignExercise(db, athleteID, exID, 5); err != nil {
		t.Fatalf("assign exercise: %v", err)
	}
	if _, err := models.SetTrainingMax(db, athleteID, exID, 185.0, "2026-01-01", ""); err != nil {
		t.Fatalf("set training max: %v", err)
	}

	ctx, err := BuildAthleteContext(db, athleteID, time.Now())
	if err != nil {
		t.Fatalf("BuildAthleteContext: %v", err)
	}
	if len(ctx.Performance.TrainingMaxes) != 1 {
		t.Fatalf("training maxes = %d, want 1", len(ctx.Performance.TrainingMaxes))
	}
	tm := ctx.Performance.TrainingMaxes[0]
	if tm.Exercise != "Bench Press" {
		t.Errorf("tm exercise = %q, want Bench Press", tm.Exercise)
	}
	if tm.Weight != 185.0 {
		t.Errorf("tm weight = %f, want 185.0", tm.Weight)
	}
}

func TestBuildAthleteContext_WithBodyWeights(t *testing.T) {
	db := testDB(t)
	athleteID := seedAthlete(t, db, "Carol", "", "")

	if _, err := models.CreateBodyWeight(db, athleteID, "2026-01-10", 150.0, ""); err != nil {
		t.Fatalf("create body weight: %v", err)
	}
	if _, err := models.CreateBodyWeight(db, athleteID, "2026-01-17", 151.5, ""); err != nil {
		t.Fatalf("create body weight: %v", err)
	}

	ctx, err := BuildAthleteContext(db, athleteID, time.Now())
	if err != nil {
		t.Fatalf("BuildAthleteContext: %v", err)
	}
	if len(ctx.Performance.BodyWeights) != 2 {
		t.Fatalf("body weights = %d, want 2", len(ctx.Performance.BodyWeights))
	}
	if ctx.Athlete.LatestBW == nil {
		t.Fatal("latest body weight should not be nil")
	}
	if *ctx.Athlete.LatestBW != 151.5 {
		t.Errorf("latest bw = %f, want 151.5", *ctx.Athlete.LatestBW)
	}
}

func TestBuildAthleteContext_ExerciseCatalog(t *testing.T) {
	db := testDB(t)
	athleteID := seedAthlete(t, db, "Dave", "", "")
	seedExercise(t, db, "Push-Up", "foundational")
	seedExercise(t, db, "Pull-Up", "foundational")

	ctx, err := BuildAthleteContext(db, athleteID, time.Now())
	if err != nil {
		t.Fatalf("BuildAthleteContext: %v", err)
	}
	if len(ctx.ExerciseCatalog) < 2 {
		t.Fatalf("exercise catalog = %d, want >= 2", len(ctx.ExerciseCatalog))
	}
	for _, ex := range ctx.ExerciseCatalog {
		if !ex.Compatible {
			t.Errorf("exercise %q should be compatible (no equipment requirements)", ex.Name)
		}
	}
}

func TestBuildAthleteContext_ExerciseCatalog_EquipmentFiltering(t *testing.T) {
	db := testDB(t)
	athleteID := seedAthlete(t, db, "Frank", "", "")

	// Create a bodyweight exercise (no equipment) and a barbell exercise.
	bwExID := seedExercise(t, db, "Squat Bodyweight", "foundational")
	_ = bwExID
	bbExID := seedExercise(t, db, "Barbell Squat", "sport_performance")

	// Create equipment and link it as required for the barbell exercise.
	barbell, err := models.CreateEquipment(db, "Barbell", "Standard barbell")
	if err != nil {
		t.Fatalf("create equipment: %v", err)
	}
	if err := models.AddExerciseEquipment(db, bbExID, barbell.ID, false); err != nil {
		t.Fatalf("add exercise equipment: %v", err)
	}

	// Athlete has NO equipment configured.
	ctx, err := BuildAthleteContext(db, athleteID, time.Now())
	if err != nil {
		t.Fatalf("BuildAthleteContext: %v", err)
	}

	for _, ex := range ctx.ExerciseCatalog {
		switch ex.Name {
		case "Squat Bodyweight":
			if !ex.Compatible {
				t.Errorf("Squat Bodyweight should be compatible (no equipment needed)")
			}
		case "Barbell Squat":
			if ex.Compatible {
				t.Errorf("Barbell Squat should NOT be compatible (requires barbell, athlete has none)")
			}
		}
	}
}

func TestBuildAthleteContext_PriorTemplates(t *testing.T) {
	db := testDB(t)

	if _, err := models.CreateProgramTemplate(db, nil, "Month 1", "First", 4, 3, false); err != nil {
		t.Fatalf("create template: %v", err)
	}
	if _, err := models.CreateProgramTemplate(db, nil, "Month 2", "Second", 4, 3, false); err != nil {
		t.Fatalf("create template: %v", err)
	}

	athleteID := seedAthlete(t, db, "Eve", "", "")
	ctx, err := BuildAthleteContext(db, athleteID, time.Now())
	if err != nil {
		t.Fatalf("BuildAthleteContext: %v", err)
	}
	if len(ctx.PriorTemplates) != 2 {
		t.Fatalf("prior templates = %d, want 2", len(ctx.PriorTemplates))
	}
}

func TestBuildAthleteContext_NotFound(t *testing.T) {
	db := testDB(t)
	_, err := BuildAthleteContext(db, 99999, time.Now())
	if err == nil {
		t.Fatal("expected error for nonexistent athlete")
	}
}
