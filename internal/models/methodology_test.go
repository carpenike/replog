package models

import (
	"database/sql"
	"errors"
	"reflect"
	"sort"
	"testing"
)

func newTestMethodology(key, name, audience string) *Methodology {
	m := &Methodology{
		Key:        key,
		Name:       name,
		Definition: "Test methodology prompt block. Per-tier specifics only — preamble/floors stay in code.",
	}
	if audience != "" {
		m.Audience = sql.NullString{String: audience, Valid: true}
	}
	return m
}

func TestCreateMethodology(t *testing.T) {
	db := testDB(t)

	t.Run("basic create", func(t *testing.T) {
		m, err := CreateMethodology(db, newTestMethodology("yessis-1x20", "Yessis 1×20", MethodologyAudienceYouth))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if m.Key != "yessis-1x20" {
			t.Errorf("Key = %q, want yessis-1x20", m.Key)
		}
		if m.Name != "Yessis 1×20" {
			t.Errorf("Name = %q", m.Name)
		}
		if !m.Audience.Valid || m.Audience.String != "youth" {
			t.Errorf("Audience = %v, want youth", m.Audience)
		}
		if m.ID == 0 {
			t.Error("ID was not assigned")
		}
	})

	t.Run("duplicate key", func(t *testing.T) {
		_, err := CreateMethodology(db, newTestMethodology("yessis-1x20", "Different name", MethodologyAudienceYouth))
		if !errors.Is(err, ErrDuplicateMethodologyKey) {
			t.Errorf("err = %v, want ErrDuplicateMethodologyKey", err)
		}
	})

	t.Run("key required", func(t *testing.T) {
		_, err := CreateMethodology(db, newTestMethodology("", "X", ""))
		if err == nil {
			t.Fatal("expected error for empty key")
		}
	})

	t.Run("name required", func(t *testing.T) {
		_, err := CreateMethodology(db, newTestMethodology("k", "", ""))
		if err == nil {
			t.Fatal("expected error for empty name")
		}
	})

	t.Run("definition required", func(t *testing.T) {
		m := newTestMethodology("k2", "K2", "")
		m.Definition = ""
		_, err := CreateMethodology(db, m)
		if err == nil {
			t.Fatal("expected error for empty definition")
		}
	})

	t.Run("invalid audience", func(t *testing.T) {
		m := newTestMethodology("k3", "K3", "")
		m.Audience = sql.NullString{String: "bogus", Valid: true}
		_, err := CreateMethodology(db, m)
		if err == nil {
			t.Fatal("expected error for invalid audience")
		}
	})

	t.Run("null audience allowed", func(t *testing.T) {
		_, err := CreateMethodology(db, newTestMethodology("k4", "K4", ""))
		if err != nil {
			t.Fatalf("create with null audience: %v", err)
		}
	})
}

func TestGetMethodology(t *testing.T) {
	db := testDB(t)
	created, _ := CreateMethodology(db, newTestMethodology("531", "5/3/1", MethodologyAudienceAdult))

	t.Run("by id", func(t *testing.T) {
		got, err := GetMethodologyByID(db, created.ID)
		if err != nil {
			t.Fatalf("get by id: %v", err)
		}
		if got.Key != "531" {
			t.Errorf("Key = %q", got.Key)
		}
	})

	t.Run("by key", func(t *testing.T) {
		got, err := GetMethodologyByKey(db, "531")
		if err != nil {
			t.Fatalf("get by key: %v", err)
		}
		if got.ID != created.ID {
			t.Errorf("ID = %d, want %d", got.ID, created.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := GetMethodologyByID(db, 999)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
		_, err = GetMethodologyByKey(db, "no-such-key")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestListMethodologies(t *testing.T) {
	db := testDB(t)
	CreateMethodology(db, newTestMethodology("yessis-1x20", "Yessis 1×20", MethodologyAudienceYouth))
	CreateMethodology(db, newTestMethodology("yessis-1x15", "Yessis 1×15", MethodologyAudienceYouth))
	CreateMethodology(db, newTestMethodology("531", "5/3/1", MethodologyAudienceAdult))

	t.Run("all", func(t *testing.T) {
		got, err := ListMethodologies(db, "")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(got) != 3 {
			t.Errorf("count = %d, want 3", len(got))
		}
	})

	t.Run("filter youth", func(t *testing.T) {
		got, err := ListMethodologies(db, MethodologyAudienceYouth)
		if err != nil {
			t.Fatalf("list youth: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("youth count = %d, want 2", len(got))
		}
	})

	t.Run("filter adult", func(t *testing.T) {
		got, err := ListMethodologies(db, MethodologyAudienceAdult)
		if err != nil {
			t.Fatalf("list adult: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("adult count = %d, want 1", len(got))
		}
	})

	t.Run("invalid filter rejected", func(t *testing.T) {
		_, err := ListMethodologies(db, "geriatric")
		if err == nil {
			t.Fatal("expected error for invalid filter")
		}
	})
}

func TestLoadMethodologyWithLinks(t *testing.T) {
	db := testDB(t)

	m, _ := CreateMethodology(db, newTestMethodology("yessis-1x20", "Yessis 1×20", MethodologyAudienceYouth))

	// Seed dependent rows we'll link to.
	eq1, _ := CreateEquipment(db, "Bodyweight", "")
	eq2, _ := CreateEquipment(db, "Light Dumbbells", "")
	ex1, _ := CreateExercise(db, "Goblet Squat", "foundational", "", "", 0)
	ex2, _ := CreateExercise(db, "Push-up", "foundational", "", "", 0)
	tpl, err := insertProgramTemplateForTest(db, "Foundations 1×20", "youth")
	if err != nil {
		t.Fatalf("seed template: %v", err)
	}

	if err := AddMethodologyAllowedEquipment(db, m.ID, []int64{eq1.ID, eq2.ID, eq1.ID /* dupe */}); err != nil {
		t.Fatalf("link equipment: %v", err)
	}
	if err := AddMethodologyAllowedExercises(db, m.ID, []int64{ex1.ID, ex2.ID}); err != nil {
		t.Fatalf("link exercises: %v", err)
	}
	if err := AddMethodologyAllowedPatterns(db, m.ID, []string{"push", "hinge", "squat", "push" /* dupe */}); err != nil {
		t.Fatalf("link patterns: %v", err)
	}
	if err := AddMethodologyReferencePrograms(db, m.ID, []int64{tpl}); err != nil {
		t.Fatalf("link reference programs: %v", err)
	}

	got, err := LoadMethodologyWithLinks(db, m.ID)
	if err != nil {
		t.Fatalf("load with links: %v", err)
	}

	if got.Key != "yessis-1x20" {
		t.Errorf("Key = %q", got.Key)
	}
	if !equalInt64Sets(got.AllowedEquipmentIDs, []int64{eq1.ID, eq2.ID}) {
		t.Errorf("AllowedEquipmentIDs = %v, want %v", got.AllowedEquipmentIDs, []int64{eq1.ID, eq2.ID})
	}
	if !equalInt64Sets(got.AllowedExerciseIDs, []int64{ex1.ID, ex2.ID}) {
		t.Errorf("AllowedExerciseIDs = %v, want %v", got.AllowedExerciseIDs, []int64{ex1.ID, ex2.ID})
	}
	if !reflect.DeepEqual(got.AllowedPatterns, []string{"hinge", "push", "squat"}) {
		t.Errorf("AllowedPatterns = %v, want [hinge push squat]", got.AllowedPatterns)
	}
	if !equalInt64Sets(got.ReferenceProgramIDs, []int64{tpl}) {
		t.Errorf("ReferenceProgramIDs = %v, want %v", got.ReferenceProgramIDs, []int64{tpl})
	}
}

func TestAddMethodologyAllowedPatternsRejectsInvalid(t *testing.T) {
	db := testDB(t)
	m, _ := CreateMethodology(db, newTestMethodology("k", "K", MethodologyAudienceAdult))
	err := AddMethodologyAllowedPatterns(db, m.ID, []string{"push", "yoga"})
	if err == nil {
		t.Fatal("expected validation error for invalid pattern")
	}
	// Ensure nothing was written before the invalid one — validation is upfront.
	out, _ := LoadMethodologyWithLinks(db, m.ID)
	if len(out.AllowedPatterns) != 0 {
		t.Errorf("AllowedPatterns = %v, want empty (validation should reject upfront)", out.AllowedPatterns)
	}
}

func TestDeleteMethodologyCascades(t *testing.T) {
	db := testDB(t)
	m, _ := CreateMethodology(db, newTestMethodology("k", "K", MethodologyAudienceAdult))
	eq, _ := CreateEquipment(db, "Barbell", "")
	if err := AddMethodologyAllowedEquipment(db, m.ID, []int64{eq.ID}); err != nil {
		t.Fatalf("link: %v", err)
	}

	if err := DeleteMethodology(db, m.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Equipment still exists; link gone via CASCADE.
	if _, err := GetEquipmentByID(db, eq.ID); err != nil {
		t.Errorf("equipment row should survive methodology delete: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM methodology_allowed_equipment WHERE methodology_id = ?`, m.ID).Scan(&n); err != nil {
		t.Fatalf("count link rows: %v", err)
	}
	if n != 0 {
		t.Errorf("link rows = %d, want 0 (CASCADE)", n)
	}

	// And not found by id afterward.
	if _, err := GetMethodologyByID(db, m.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get after delete: err = %v, want ErrNotFound", err)
	}

	// Double-delete is ErrNotFound.
	if err := DeleteMethodology(db, m.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("re-delete: err = %v, want ErrNotFound", err)
	}
}

func TestExerciseMovementPatternsRoundTrip(t *testing.T) {
	db := testDB(t)
	ex, _ := CreateExercise(db, "Trap Bar Deadlift", "intermediate", "", "", 0)

	if err := SetExerciseMovementPatterns(db, ex.ID, []string{"hinge", "squat"}); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := ListExerciseMovementPatterns(db, ex.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"hinge", "squat"}) {
		t.Errorf("patterns = %v, want [hinge squat]", got)
	}

	// Replacement (not merge).
	if err := SetExerciseMovementPatterns(db, ex.ID, []string{"carry"}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, _ = ListExerciseMovementPatterns(db, ex.ID)
	if !reflect.DeepEqual(got, []string{"carry"}) {
		t.Errorf("after replace = %v, want [carry]", got)
	}

	// Invalid value rejected up-front; existing rows unchanged.
	if err := SetExerciseMovementPatterns(db, ex.ID, []string{"carry", "bogus"}); err == nil {
		t.Error("expected validation error for invalid pattern")
	}
	got, _ = ListExerciseMovementPatterns(db, ex.ID)
	if !reflect.DeepEqual(got, []string{"carry"}) {
		t.Errorf("after failed set = %v, want unchanged [carry]", got)
	}

	// Clearing.
	if err := SetExerciseMovementPatterns(db, ex.ID, nil); err != nil {
		t.Fatalf("clear: %v", err)
	}
	got, _ = ListExerciseMovementPatterns(db, ex.ID)
	if len(got) != 0 {
		t.Errorf("after clear = %v, want empty", got)
	}
}

// insertProgramTemplateForTest inserts a minimal global program_template
// and returns its id. Avoids depending on the full importer for tests
// that only need a template_id to satisfy a FK.
func insertProgramTemplateForTest(db *sql.DB, name, audience string) (int64, error) {
	var id int64
	err := db.QueryRow(
		`INSERT INTO program_templates (athlete_id, name, num_weeks, num_days, is_loop, audience)
		 VALUES (NULL, ?, 1, 1, 0, ?) RETURNING id`,
		name, audience,
	).Scan(&id)
	return id, err
}

func equalInt64Sets(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]int64(nil), a...)
	bc := append([]int64(nil), b...)
	sort.Slice(ac, func(i, j int) bool { return ac[i] < ac[j] })
	sort.Slice(bc, func(i, j int) bool { return bc[i] < bc[j] })
	return reflect.DeepEqual(ac, bc)
}
