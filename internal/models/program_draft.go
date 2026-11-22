package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const (
	// ProgramStatusDraft is a proposal. It is not assigned and it is not
	// used as a reference program.
	ProgramStatusDraft = "draft"
	// ProgramStatusApproved is a program a coach has accepted. Existing
	// rows default to this. Assignment and reference context use it.
	ProgramStatusApproved = "approved"
)

// ErrProgramDraft is returned when an assignment targets a program that
// is still a proposal.
var ErrProgramDraft = errors.New("program draft is not assigned until a coach approves it")

// ErrProgramNameTaken is returned when approving a draft would collide
// with an approved program of the same name in the same scope.
var ErrProgramNameTaken = errors.New("an approved program already uses this name")

// ProgramDraftSet is one prescribed set on a proposal.
type ProgramDraftSet struct {
	ExerciseID     int64
	Week           int
	Day            int
	SetNumber      int
	Reps           *int
	Percentage     *float64
	AbsoluteWeight *float64
	RestSeconds    *int
	SortOrder      int
	RepType        string
	Notes          string
	PrescribedRPE  *float64
	GroupLabel     string
	Detail         PrescribedSetDetail
}

// ProgramDraftInput is a proposal a coach (or the coach's service token)
// writes. Status is always draft. LineageID, when set, starts the next
// version of that program.
type ProgramDraftInput struct {
	AthleteID   *int64
	Name        string
	Description string
	NumWeeks    int
	NumDays     int
	IsLoop      bool
	Audience    string
	LineageID   *int64
	CoachID     int64
	SourceDate  string
	SourceText  string
	Sets        []ProgramDraftSet
}

// CreateProgramDraft inserts a draft template, one provenance row, and
// any prescribed sets. It does not assign the program.
func CreateProgramDraft(ctx context.Context, db *sql.DB, in ProgramDraftInput) (*ProgramTemplate, error) {
	if strings.TrimSpace(in.Name) == "" || in.NumWeeks < 1 || in.NumDays < 1 {
		return nil, fmt.Errorf("%w: name, num_weeks, and num_days are required", ErrInvalidInput)
	}
	sourceText := strings.TrimSpace(in.SourceText)
	if sourceText == "" {
		return nil, fmt.Errorf("%w: source text is required", ErrInvalidInput)
	}
	if in.SourceDate == "" {
		return nil, fmt.Errorf("%w: source date is required", ErrInvalidInput)
	}
	for _, set := range in.Sets {
		if set.PrescribedRPE != nil && (*set.PrescribedRPE < 1 || *set.PrescribedRPE > 10) {
			return nil, fmt.Errorf("%w: prescribed RPE must be between 1 and 10", ErrInvalidInput)
		}
		if field, msg := ValidatePrescribedSetDetail(set.Reps, set.Detail); field != "" {
			return nil, fmt.Errorf("%w: %s %s", ErrInvalidInput, field, msg)
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("models: create program draft begin: %w", err)
	}
	defer tx.Rollback()

	lineageID, version, athleteID, err := resolveDraftLineage(ctx, tx, in)
	if err != nil {
		return nil, err
	}

	var descVal sql.NullString
	if in.Description != "" {
		descVal = sql.NullString{String: in.Description, Valid: true}
	}
	var audVal sql.NullString
	if in.Audience != "" {
		audVal = sql.NullString{String: in.Audience, Valid: true}
	}
	var lineageVal any
	if lineageID != nil {
		lineageVal = *lineageID
	}

	var id int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO program_templates
		    (athlete_id, name, description, num_weeks, num_days, is_loop, audience, status, lineage_id, version)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		athleteID, strings.TrimSpace(in.Name), descVal, in.NumWeeks, in.NumDays, boolToInt(in.IsLoop), audVal,
		ProgramStatusDraft, lineageVal, version,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("models: create program draft: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO program_sources (template_id, coach_id, source_date, source_text) VALUES (?, ?, ?, ?)`,
		id, in.CoachID, in.SourceDate, sourceText,
	); err != nil {
		return nil, fmt.Errorf("models: create program source: %w", err)
	}

	for _, set := range in.Sets {
		if _, err := insertPrescribedSetRow(ctx, tx, id, set.ExerciseID, set.Week, set.Day, set.SetNumber,
			set.Reps, set.Percentage, set.AbsoluteWeight, set.RestSeconds, set.SortOrder, set.RepType, set.Notes,
			set.PrescribedRPE, set.GroupLabel, set.Detail); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: create program draft commit: %w", err)
	}
	return GetProgramTemplateByID(ctx, db, id)
}

// resolveDraftLineage picks the stored lineage id, version, and athlete
// scope. A new proposal has a NULL lineage_id and version 1. A revision
// points at the root and takes the next version. An omitted athlete on a
// revision inherits the root's athlete.
func resolveDraftLineage(ctx context.Context, tx *sql.Tx, in ProgramDraftInput) (*int64, int, *int64, error) {
	if in.LineageID == nil {
		return nil, 1, in.AthleteID, nil
	}

	var rootID int64
	var rootAthlete sql.NullInt64
	var rootLineage sql.NullInt64
	err := tx.QueryRowContext(ctx,
		`SELECT id, athlete_id, lineage_id FROM program_templates WHERE id = ?`,
		*in.LineageID,
	).Scan(&rootID, &rootAthlete, &rootLineage)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, nil, ErrNotFound
	}
	if err != nil {
		return nil, 0, nil, fmt.Errorf("models: load program lineage %d: %w", *in.LineageID, err)
	}
	if rootLineage.Valid {
		rootID = rootLineage.Int64
	}

	var inherited *int64
	if rootAthlete.Valid {
		v := rootAthlete.Int64
		inherited = &v
	}
	athleteID := in.AthleteID
	if athleteID == nil {
		athleteID = inherited
	} else if !sameAthleteID(athleteID, inherited) {
		return nil, 0, nil, fmt.Errorf("%w: lineage belongs to a different athlete", ErrInvalidInput)
	}

	var version int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) + 1
		   FROM program_templates
		  WHERE id = ? OR lineage_id = ?`,
		rootID, rootID,
	).Scan(&version); err != nil {
		return nil, 0, nil, fmt.Errorf("models: next program version for %d: %w", rootID, err)
	}
	return &rootID, version, athleteID, nil
}

func sameAthleteID(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// ApproveProgramDraft marks a draft approved. An already-approved program
// is returned unchanged. Approving does not assign the program.
func ApproveProgramDraft(ctx context.Context, db *sql.DB, id int64) (*ProgramTemplate, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("models: approve program draft begin: %w", err)
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM program_templates WHERE id = ?`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: approve program draft %d: %w", id, err)
	}
	if status == ProgramStatusApproved {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("models: approve program draft commit: %w", err)
		}
		return GetProgramTemplateByID(ctx, db, id)
	}
	if status != ProgramStatusDraft {
		return nil, fmt.Errorf("%w: program %d cannot be approved from %s", ErrInvalidInput, id, status)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE program_templates SET status = ? WHERE id = ? AND status = ?`,
		ProgramStatusApproved, id, ProgramStatusDraft,
	); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrProgramNameTaken
		}
		return nil, fmt.Errorf("models: approve program draft %d: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: approve program draft commit: %w", err)
	}
	return GetProgramTemplateByID(ctx, db, id)
}

// assignableTemplate reports whether a template may be assigned. A missing
// row is left to the insert's foreign key. Anything other than approved
// is a proposal and is skipped.
func assignableTemplate(ctx context.Context, db *sql.DB, templateID int64) error {
	var status string
	err := db.QueryRowContext(ctx, `SELECT status FROM program_templates WHERE id = ?`, templateID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("models: program template %d status: %w", templateID, err)
	}
	if status != ProgramStatusApproved {
		return ErrProgramDraft
	}
	return nil
}
