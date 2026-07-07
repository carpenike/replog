package models

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// TestCreateGeneration_InflightUnique covers race fix §3a: the partial unique
// index must reject a second in-flight generation of the same kind for one
// athlete, surfacing as ErrGenerationInFlight (mapped to 409 by the handler).
func TestCreateGeneration_InflightUnique(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(context.Background(), db, "Gen Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	u, _ := CreateUser(context.Background(), db, "gencoach", "", "password123", "", true, false, sql.NullInt64{})

	if _, err := CreateGeneration(context.Background(), db, a.ID, u.ID, "{}"); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := CreateGeneration(context.Background(), db, a.ID, u.ID, "{}")
	if !errors.Is(err, ErrGenerationInFlight) {
		t.Fatalf("second concurrent create err = %v, want ErrGenerationInFlight", err)
	}

	// A different kind for the same athlete is allowed to coexist.
	if _, err := CreateGenerationWithKind(context.Background(), db, a.ID, u.ID, "{}", GenerationKindWOD); err != nil {
		t.Errorf("distinct-kind create should succeed, got %v", err)
	}
}

// TestMarkGenerationExecuted_ClaimOnce covers race fix §3b: the claiming update
// succeeds exactly once; a second claim reports ErrNotFound so a concurrent
// execute cannot double-import. Unmark releases the claim for retry.
func TestMarkGenerationExecuted_ClaimOnce(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(context.Background(), db, "Exec Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	u, _ := CreateUser(context.Background(), db, "execcoach", "", "password123", "", true, false, sql.NullInt64{})
	gen, _ := CreateGeneration(context.Background(), db, a.ID, u.ID, "{}")

	if err := MarkGenerationExecuted(context.Background(), db, gen.ID); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if err := MarkGenerationExecuted(context.Background(), db, gen.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second claim err = %v, want ErrNotFound", err)
	}

	// Rollback releases the claim so a retry can succeed.
	if err := UnmarkGenerationExecuted(context.Background(), db, gen.ID); err != nil {
		t.Fatalf("unmark: %v", err)
	}
	if err := MarkGenerationExecuted(context.Background(), db, gen.ID); err != nil {
		t.Errorf("re-claim after unmark: %v", err)
	}
}
