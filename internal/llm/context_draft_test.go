package llm

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func TestDraftSkippedByBuildReferenceProgramsAndAssign(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	coach, err := models.CreateUser(ctx, db, "coach", "Coach", "password123", "", true, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create coach: %v", err)
	}
	approved, err := models.CreateProgramTemplate(ctx, db, nil, "Youth Approved", "", 1, 2, true, "youth")
	if err != nil {
		t.Fatalf("create approved: %v", err)
	}
	draft, err := models.CreateProgramDraft(ctx, db, models.ProgramDraftInput{
		Name:       "Youth Draft",
		NumWeeks:   1,
		NumDays:    2,
		IsLoop:     true,
		Audience:   "youth",
		CoachID:    coach.ID,
		SourceDate: "2026-11-21",
		SourceText: "session notes",
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	refs, err := buildReferencePrograms(ctx, db, "youth")
	if err != nil {
		t.Fatalf("buildReferencePrograms: %v", err)
	}
	names := map[string]bool{}
	for _, ref := range refs {
		names[ref.Name] = true
	}
	if !names["Youth Approved"] || names["Youth Draft"] {
		t.Fatalf("reference programs = %v, want Youth Approved and not Youth Draft", names)
	}

	listed, err := models.ListProgramTemplates(ctx, db)
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}
	seenDraft := false
	for _, tmpl := range listed {
		if tmpl.ID == draft.ID {
			seenDraft = true
		}
	}
	if !seenDraft {
		t.Fatal("coach program list skipped the draft")
	}

	athleteID := seedAthlete(t, db, "Riley", "foundational", "")
	if _, err := models.AssignProgram(ctx, db, athleteID, draft.ID, "2026-11-21", "", "", "primary", ""); !errors.Is(err, models.ErrProgramDraft) {
		t.Fatalf("AssignProgram(draft) = %v, want ErrProgramDraft", err)
	}
	var assignedRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM athlete_programs WHERE athlete_id = ?`, athleteID).Scan(&assignedRows); err != nil {
		t.Fatalf("count assignments: %v", err)
	}
	if assignedRows != 0 {
		t.Fatalf("draft assignment rows = %d, want 0", assignedRows)
	}

	assigned, err := models.AssignProgram(ctx, db, athleteID, approved.ID, "2026-11-21", "", "", "primary", "")
	if err != nil {
		t.Fatalf("AssignProgram(approved): %v", err)
	}
	if _, err := models.ReplaceProgram(ctx, db, athleteID, draft.ID, "2026-11-22", "", "", "primary", ""); !errors.Is(err, models.ErrProgramDraft) {
		t.Fatalf("ReplaceProgram(draft) = %v, want ErrProgramDraft", err)
	}
	var templateID, active int64
	if err := db.QueryRow(`SELECT template_id, active FROM athlete_programs WHERE id = ?`, assigned.ID).Scan(&templateID, &active); err != nil {
		t.Fatalf("reload assignment: %v", err)
	}
	if templateID != approved.ID || active != 1 {
		t.Fatalf("assignment after skipped replace = template %d active %d, want %d active 1", templateID, active, approved.ID)
	}
}
