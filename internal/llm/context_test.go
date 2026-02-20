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

	// Create global templates with audience tags.
	if _, err := models.CreateProgramTemplate(db, nil, "Foundations 1×20", "Youth foundations", 1, 2, true, "youth"); err != nil {
		t.Fatalf("create youth template: %v", err)
	}
	if _, err := models.CreateProgramTemplate(db, nil, "5/3/1 BBB", "Adult program", 4, 4, true, "adult"); err != nil {
		t.Fatalf("create adult template: %v", err)
	}

	// Create a youth athlete (has tier).
	youthID := seedAthlete(t, db, "Eve", "foundational", "general fitness")

	// Create an athlete-scoped template for the youth athlete.
	if _, err := models.CreateProgramTemplate(db, &youthID, "Eve Custom", "", 3, 3, false, ""); err != nil {
		t.Fatalf("create athlete-scoped template: %v", err)
	}

	t.Run("youth athlete sees youth reference programs", func(t *testing.T) {
		ctx, err := BuildAthleteContext(db, youthID, time.Now())
		if err != nil {
			t.Fatalf("BuildAthleteContext: %v", err)
		}
		// Should see only the youth global template in reference_programs.
		if len(ctx.ReferencePrograms) != 1 {
			t.Fatalf("reference programs = %d, want 1", len(ctx.ReferencePrograms))
		}
		if ctx.ReferencePrograms[0].Name != "Foundations 1×20" {
			t.Errorf("reference program name = %q, want Foundations 1×20", ctx.ReferencePrograms[0].Name)
		}
		// Should see the athlete-scoped template in prior_templates (not global ones).
		if len(ctx.PriorTemplates) != 1 {
			t.Fatalf("prior templates = %d, want 1", len(ctx.PriorTemplates))
		}
		if ctx.PriorTemplates[0].Name != "Eve Custom" {
			t.Errorf("prior template name = %q, want Eve Custom", ctx.PriorTemplates[0].Name)
		}
	})

	// Create an adult athlete (no tier).
	adultID := seedAthlete(t, db, "Frank", "", "strength")

	t.Run("adult athlete sees adult reference programs", func(t *testing.T) {
		ctx, err := BuildAthleteContext(db, adultID, time.Now())
		if err != nil {
			t.Fatalf("BuildAthleteContext: %v", err)
		}
		if len(ctx.ReferencePrograms) != 1 {
			t.Fatalf("reference programs = %d, want 1", len(ctx.ReferencePrograms))
		}
		if ctx.ReferencePrograms[0].Name != "5/3/1 BBB" {
			t.Errorf("reference program name = %q, want 5/3/1 BBB", ctx.ReferencePrograms[0].Name)
		}
		// Adult has no athlete-scoped templates.
		if len(ctx.PriorTemplates) != 0 {
			t.Errorf("prior templates = %d, want 0", len(ctx.PriorTemplates))
		}
	})
}

func TestBuildAthleteContext_ReferencePrograms_WithSets(t *testing.T) {
	db := testDB(t)

	// Create a global youth template with prescribed sets.
	tmpl, err := models.CreateProgramTemplate(db, nil, "Youth Test Program", "A youth reference", 1, 2, true, "youth")
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	exID := seedExercise(t, db, "Push-up", "foundational")
	reps := 20
	if _, err := models.CreatePrescribedSet(db, tmpl.ID, exID, 1, 1, 1, &reps, nil, nil, 1, "reps", "Form: full ROM"); err != nil {
		t.Fatalf("create prescribed set: %v", err)
	}

	athleteID := seedAthlete(t, db, "Grace", "foundational", "")
	ctx, err := BuildAthleteContext(db, athleteID, time.Now())
	if err != nil {
		t.Fatalf("BuildAthleteContext: %v", err)
	}

	if len(ctx.ReferencePrograms) != 1 {
		t.Fatalf("reference programs = %d, want 1", len(ctx.ReferencePrograms))
	}
	rp := ctx.ReferencePrograms[0]
	if len(rp.PrescribedSets) != 1 {
		t.Fatalf("prescribed sets = %d, want 1", len(rp.PrescribedSets))
	}
	ps := rp.PrescribedSets[0]
	if ps.Exercise != "Push-up" {
		t.Errorf("exercise = %q, want Push-up", ps.Exercise)
	}
	if ps.Reps == nil || *ps.Reps != 20 {
		t.Errorf("reps = %v, want 20", ps.Reps)
	}
	if ps.Notes != "Form: full ROM" {
		t.Errorf("notes = %q, want 'Form: full ROM'", ps.Notes)
	}
}

func TestBuildAthleteContext_NotFound(t *testing.T) {
	db := testDB(t)
	_, err := BuildAthleteContext(db, 99999, time.Now())
	if err == nil {
		t.Fatal("expected error for nonexistent athlete")
	}
}

func TestBuildAthleteContext_ExplicitReferenceTemplateIDs(t *testing.T) {
	db := testDB(t)

	// Create two youth templates and one adult template.
	youthA, err := models.CreateProgramTemplate(db, nil, "Youth A", "first", 1, 2, true, "youth")
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	_, err = models.CreateProgramTemplate(db, nil, "Youth B", "second", 1, 3, true, "youth")
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	adultC, err := models.CreateProgramTemplate(db, nil, "Adult C", "adult ref", 4, 4, false, "adult")
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	// Add a prescribed set to youthA so we can verify it loads.
	exID := seedExercise(t, db, "Squat", "foundational")
	reps := 20
	if _, err := models.CreatePrescribedSet(db, youthA.ID, exID, 1, 1, 1, &reps, nil, nil, 1, "reps", ""); err != nil {
		t.Fatalf("create prescribed set: %v", err)
	}

	// Youth athlete — default behavior would load both youth templates.
	athleteID := seedAthlete(t, db, "Zoe", "foundational", "")

	t.Run("explicit IDs override audience filter", func(t *testing.T) {
		// Request only youthA — should get just that one, not youthB.
		ctx, err := BuildAthleteContext(db, athleteID, time.Now(), youthA.ID)
		if err != nil {
			t.Fatalf("BuildAthleteContext: %v", err)
		}
		if len(ctx.ReferencePrograms) != 1 {
			t.Fatalf("reference programs = %d, want 1", len(ctx.ReferencePrograms))
		}
		if ctx.ReferencePrograms[0].Name != "Youth A" {
			t.Errorf("name = %q, want Youth A", ctx.ReferencePrograms[0].Name)
		}
		if len(ctx.ReferencePrograms[0].PrescribedSets) != 1 {
			t.Errorf("prescribed sets = %d, want 1", len(ctx.ReferencePrograms[0].PrescribedSets))
		}
	})

	t.Run("can select cross-audience template", func(t *testing.T) {
		// Youth athlete can still get the adult template if coach explicitly picks it.
		ctx, err := BuildAthleteContext(db, athleteID, time.Now(), adultC.ID)
		if err != nil {
			t.Fatalf("BuildAthleteContext: %v", err)
		}
		if len(ctx.ReferencePrograms) != 1 {
			t.Fatalf("reference programs = %d, want 1", len(ctx.ReferencePrograms))
		}
		if ctx.ReferencePrograms[0].Name != "Adult C" {
			t.Errorf("name = %q, want Adult C", ctx.ReferencePrograms[0].Name)
		}
	})

	t.Run("empty IDs falls back to audience", func(t *testing.T) {
		ctx, err := BuildAthleteContext(db, athleteID, time.Now())
		if err != nil {
			t.Fatalf("BuildAthleteContext: %v", err)
		}
		// Should get both youth templates (youthA and youthB), not adultC.
		if len(ctx.ReferencePrograms) != 2 {
			t.Fatalf("reference programs = %d, want 2", len(ctx.ReferencePrograms))
		}
		names := map[string]bool{}
		for _, rp := range ctx.ReferencePrograms {
			names[rp.Name] = true
		}
		if !names["Youth A"] || !names["Youth B"] {
			t.Errorf("expected Youth A and Youth B, got %v", names)
		}
	})
}

func TestListProgramTemplatesByIDs(t *testing.T) {
	db := testDB(t)

	a, err := models.CreateProgramTemplate(db, nil, "Prog A", "desc a", 4, 3, false, "adult")
	if err != nil {
		t.Fatal(err)
	}
	b, err := models.CreateProgramTemplate(db, nil, "Prog B", "desc b", 1, 4, true, "youth")
	if err != nil {
		t.Fatal(err)
	}
	_, err = models.CreateProgramTemplate(db, nil, "Prog C", "", 2, 2, false, "")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("returns matching templates", func(t *testing.T) {
		results, err := models.ListProgramTemplatesByIDs(db, []int64{a.ID, b.ID})
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 2 {
			t.Fatalf("got %d results, want 2", len(results))
		}
	})

	t.Run("empty IDs returns nil", func(t *testing.T) {
		results, err := models.ListProgramTemplatesByIDs(db, nil)
		if err != nil {
			t.Fatal(err)
		}
		if results != nil {
			t.Fatalf("expected nil, got %d results", len(results))
		}
	})

	t.Run("nonexistent IDs return empty", func(t *testing.T) {
		results, err := models.ListProgramTemplatesByIDs(db, []int64{99999})
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 0 {
			t.Fatalf("got %d results, want 0", len(results))
		}
	})
}

func TestBuildAthleteContext_ProgramHistory(t *testing.T) {
	db := testDB(t)
	athleteID := seedAthlete(t, db, "Max", "sport_performance", "")

	// Create two templates and assign them sequentially.
	pt1, err := models.CreateProgramTemplate(db, nil, "Month 1", "first cycle", 4, 3, false, "youth")
	if err != nil {
		t.Fatal(err)
	}
	pt2, err := models.CreateProgramTemplate(db, nil, "Month 2", "second cycle", 4, 4, false, "youth")
	if err != nil {
		t.Fatal(err)
	}

	// Assign first program, then deactivate it.
	ap1, err := models.AssignProgram(db, athleteID, pt1.ID, "2026-01-01", "starting out", "build base")
	if err != nil {
		t.Fatal(err)
	}
	if err := models.DeactivateProgram(db, ap1.ID); err != nil {
		t.Fatal(err)
	}

	// Assign second (now active).
	_, err = models.AssignProgram(db, athleteID, pt2.ID, "2026-02-01", "", "increase volume")
	if err != nil {
		t.Fatal(err)
	}

	ctx, err := BuildAthleteContext(db, athleteID, time.Now())
	if err != nil {
		t.Fatalf("BuildAthleteContext: %v", err)
	}

	if len(ctx.ProgramHistory) != 2 {
		t.Fatalf("program history = %d, want 2", len(ctx.ProgramHistory))
	}

	// Most recent first.
	if ctx.ProgramHistory[0].Name != "Month 2" {
		t.Errorf("first entry = %q, want Month 2", ctx.ProgramHistory[0].Name)
	}
	if !ctx.ProgramHistory[0].Active {
		t.Error("first entry should be active")
	}
	if ctx.ProgramHistory[0].Goal == nil || *ctx.ProgramHistory[0].Goal != "increase volume" {
		t.Errorf("first entry goal = %v, want 'increase volume'", ctx.ProgramHistory[0].Goal)
	}

	if ctx.ProgramHistory[1].Name != "Month 1" {
		t.Errorf("second entry = %q, want Month 1", ctx.ProgramHistory[1].Name)
	}
	if ctx.ProgramHistory[1].Active {
		t.Error("second entry should be inactive")
	}
	if ctx.ProgramHistory[1].Notes == nil || *ctx.ProgramHistory[1].Notes != "starting out" {
		t.Errorf("second entry notes = %v, want 'starting out'", ctx.ProgramHistory[1].Notes)
	}
	if ctx.ProgramHistory[1].Goal == nil || *ctx.ProgramHistory[1].Goal != "build base" {
		t.Errorf("second entry goal = %v, want 'build base'", ctx.ProgramHistory[1].Goal)
	}
}

func TestListAthletePrograms(t *testing.T) {
	db := testDB(t)
	athleteID := seedAthlete(t, db, "Ella", "foundational", "")

	t.Run("empty for new athlete", func(t *testing.T) {
		programs, err := models.ListAthletePrograms(db, athleteID)
		if err != nil {
			t.Fatal(err)
		}
		if len(programs) != 0 {
			t.Fatalf("got %d programs, want 0", len(programs))
		}
	})

	pt, err := models.CreateProgramTemplate(db, nil, "Test Prog", "", 4, 3, false, "")
	if err != nil {
		t.Fatal(err)
	}

	ap, err := models.AssignProgram(db, athleteID, pt.ID, "2026-01-15", "notes here", "get strong")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("returns assignments with joined fields", func(t *testing.T) {
		programs, err := models.ListAthletePrograms(db, athleteID)
		if err != nil {
			t.Fatal(err)
		}
		if len(programs) != 1 {
			t.Fatalf("got %d programs, want 1", len(programs))
		}
		p := programs[0]
		if p.TemplateName != "Test Prog" {
			t.Errorf("name = %q, want Test Prog", p.TemplateName)
		}
		if !p.Active {
			t.Error("expected active")
		}
		if !p.Notes.Valid || p.Notes.String != "notes here" {
			t.Errorf("notes = %v, want 'notes here'", p.Notes)
		}
		if !p.Goal.Valid || p.Goal.String != "get strong" {
			t.Errorf("goal = %v, want 'get strong'", p.Goal)
		}
	})

	// Deactivate and add another.
	if err := models.DeactivateProgram(db, ap.ID); err != nil {
		t.Fatal(err)
	}
	pt2, err := models.CreateProgramTemplate(db, nil, "Test Prog 2", "", 1, 4, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := models.AssignProgram(db, athleteID, pt2.ID, "2026-02-15", "", ""); err != nil {
		t.Fatal(err)
	}

	t.Run("returns all ordered by start_date desc", func(t *testing.T) {
		programs, err := models.ListAthletePrograms(db, athleteID)
		if err != nil {
			t.Fatal(err)
		}
		if len(programs) != 2 {
			t.Fatalf("got %d programs, want 2", len(programs))
		}
		if programs[0].TemplateName != "Test Prog 2" {
			t.Errorf("first = %q, want Test Prog 2", programs[0].TemplateName)
		}
		if programs[1].TemplateName != "Test Prog" {
			t.Errorf("second = %q, want Test Prog", programs[1].TemplateName)
		}
	})
}
