package models

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestCreateProgramDraft_LineageSourceAndSet(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	coach, err := CreateUser(ctx, db, "coach", "Coach", "password123", "", true, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create coach: %v", err)
	}
	exercise, err := CreateExercise(ctx, db, "Back Squat", "", "", "", 0)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}
	reps := 5
	rpe := 7.5
	draft, err := CreateProgramDraft(ctx, db, ProgramDraftInput{
		Name:        "Squat block",
		Description: "proposal",
		NumWeeks:    1,
		NumDays:     1,
		Audience:    "adult",
		CoachID:     coach.ID,
		SourceDate:  "2026-11-21",
		SourceText:  "Wednesday session notes",
		Sets: []ProgramDraftSet{{
			ExerciseID:    exercise.ID,
			Week:          1,
			Day:           1,
			SetNumber:     1,
			Reps:          &reps,
			PrescribedRPE: &rpe,
			GroupLabel:    "T1",
			RepType:       "reps",
		}},
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if draft.Status != ProgramStatusDraft || draft.Version != 1 || draft.LineageID != nil {
		t.Fatalf("draft = status %q version %d lineage %v, want draft v1 with null lineage", draft.Status, draft.Version, draft.LineageID)
	}

	var coachID int64
	var sourceDate, sourceText string
	if err := db.QueryRow(
		`SELECT coach_id, date(source_date), source_text FROM program_sources WHERE template_id = ?`,
		draft.ID,
	).Scan(&coachID, &sourceDate, &sourceText); err != nil {
		t.Fatalf("read source: %v", err)
	}
	if coachID != coach.ID || sourceDate != "2026-11-21" || sourceText != "Wednesday session notes" {
		t.Fatalf("source = coach %d date %q text %q", coachID, sourceDate, sourceText)
	}

	var gotRPE sql.NullFloat64
	var group sql.NullString
	if err := db.QueryRow(
		`SELECT prescribed_rpe, group_label FROM prescribed_sets WHERE template_id = ?`,
		draft.ID,
	).Scan(&gotRPE, &group); err != nil {
		t.Fatalf("read set: %v", err)
	}
	if !gotRPE.Valid || gotRPE.Float64 != 7.5 || !group.Valid || group.String != "T1" {
		t.Fatalf("set = rpe %v group %v, want 7.5 T1", gotRPE, group)
	}

	next, err := CreateProgramDraft(ctx, db, ProgramDraftInput{
		Name:       "Squat block v2",
		NumWeeks:   1,
		NumDays:    1,
		Audience:   "adult",
		LineageID:  &draft.ID,
		CoachID:    coach.ID,
		SourceDate: "2026-11-21",
		SourceText: "revision",
	})
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}
	if next.Version != 2 || next.LineageID == nil || *next.LineageID != draft.ID || next.Status != ProgramStatusDraft {
		t.Fatalf("revision = status %q version %d lineage %v, want draft v2 of %d", next.Status, next.Version, next.LineageID, draft.ID)
	}

	approved, err := ApproveProgramDraft(ctx, db, draft.ID)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status != ProgramStatusApproved {
		t.Fatalf("approved status = %q", approved.Status)
	}

	// A draft may reuse an approved name. Approving that reuse is refused.
	_, err = CreateProgramTemplate(ctx, db, nil, "Taken Name", "", 1, 1, false, "")
	if err != nil {
		t.Fatalf("create approved name: %v", err)
	}
	collision, err := CreateProgramDraft(ctx, db, ProgramDraftInput{
		Name:       "Taken Name",
		NumWeeks:   1,
		NumDays:    1,
		CoachID:    coach.ID,
		SourceDate: "2026-11-21",
		SourceText: "same name, still a proposal",
	})
	if err != nil {
		t.Fatalf("draft with an approved name: %v", err)
	}
	if _, err := ApproveProgramDraft(ctx, db, collision.ID); !errors.Is(err, ErrProgramNameTaken) {
		t.Fatalf("approve name collision = %v, want ErrProgramNameTaken", err)
	}
}
