package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/carpenike/replog/internal/importers"
)

// ErrWODCollision is returned by LogWODFromCatalog when a resistance workout
// already exists for the athlete+date and the caller did not request a
// replace. The handler surfaces this as a replace-or-cancel prompt rather
// than a raw 409 (HOF-015 same-day collision decision).
var ErrWODCollision = errors.New("a resistance workout already exists for this date")

// UnknownExercisesError is returned by LogWODFromCatalog when the generated
// catalog prescribes exercise names that don't resolve to the exercise
// catalog. The log path REJECTS these rather than auto-creating them (the
// ADR 020 follow-up): a hallucinated name must not pollute the shared global
// catalog, and the coach already saw these names flagged by the generation
// lint in the preview. The coach's recourse is to regenerate.
type UnknownExercisesError struct {
	Names []string
}

func (e *UnknownExercisesError) Error() string {
	return fmt.Sprintf("models: WOD prescribes unknown exercises: %s", strings.Join(e.Names, ", "))
}

// WODLogResult summarizes what LogWODFromCatalog committed.
type WODLogResult struct {
	WorkoutID   int64
	SetsCreated int
	Replaced    bool // true when an existing same-day resistance workout was superseded
}

// LogWODFromCatalog commits a generated single-session WOD as an ad-hoc
// resistance workout (HOF-015). It is the WOD analogue of the
// from-prescription transcribe-then-confirm flow, but operates on the
// generation's CatalogJSON rather than a program-derived *Prescription:
//
//  1. Resolve every prescribed-set exercise name to an exercise ID. Names
//     that don't resolve fail the whole log with *UnknownExercisesError —
//     BEFORE any mutation, so a replace never deletes the existing workout
//     only to then reject the WOD. Unknown names are rejected rather than
//     auto-created (ADR 020 follow-up): a hallucinated exercise must not
//     pollute the shared global catalog.
//  2. In a SINGLE transaction: resolve the same-day collision (ErrWODCollision
//     when a resistance workout exists for the date and replace is false;
//     delete it when replace is true — its sets cascade), create the ad-hoc
//     workout (assignment_id NULL → discipline defaults to 'resistance',
//     which the read-path filters require so the WOD feeds future
//     BuildAthleteContext), and insert its sets. Everything commits or rolls
//     back together, so a failed replace can neither destroy the existing
//     workout without producing its replacement nor leave an empty orphan
//     workout behind (ADR 020 follow-up).
//
// The athlete then confirms/edits the seeded sets in the normal workout UI.
// A WOD has NumDays=1/NumWeeks=1, so all prescribed sets belong to the one
// session; insertion order follows (sort_order, set_number) so the circuit
// structure stays faithful.
func LogWODFromCatalog(ctx context.Context, db *sql.DB, athleteID int64, date string, parsed *importers.ParsedFile, replace bool) (*WODLogResult, error) {
	if parsed == nil {
		return nil, fmt.Errorf("models: log WOD with nil parsed catalog")
	}

	// Step 1: resolve exercise names → IDs, rejecting unknowns up front.
	nameToID := make(map[string]int64)
	exercises, err := ListExercises(ctx, db, "")
	if err != nil {
		return nil, fmt.Errorf("models: list exercises for WOD: %w", err)
	}
	for _, ex := range exercises {
		nameToID[strings.ToLower(ex.Name)] = ex.ID
	}

	// Flatten the parsed sets, resolving exercise IDs and collecting every
	// unknown name so the error reports the full list in one shot.
	type setRow struct {
		exerciseID int64
		setNumber  int
		reps       int
		weight     sql.NullFloat64
		repType    string
		notes      sql.NullString
		sortOrder  int
	}
	var rows []setRow
	var unknown []string
	seenUnknown := map[string]bool{}
	for _, prog := range parsed.Programs {
		for _, ps := range prog.Template.PrescribedSets {
			name := strings.TrimSpace(ps.Exercise)
			if name == "" {
				return nil, fmt.Errorf("models: WOD set references an empty exercise name")
			}
			exID, ok := nameToID[strings.ToLower(name)]
			if !ok {
				if !seenUnknown[strings.ToLower(name)] {
					seenUnknown[strings.ToLower(name)] = true
					unknown = append(unknown, name)
				}
				continue
			}
			r := setRow{
				exerciseID: exID,
				setNumber:  ps.SetNumber,
				sortOrder:  ps.SortOrder,
				repType:    ps.RepType,
			}
			if r.repType == "" {
				r.repType = "reps"
			}
			if ps.Reps != nil {
				r.reps = *ps.Reps
			}
			if ps.AbsoluteWeight != nil && *ps.AbsoluteWeight > 0 {
				r.weight = sql.NullFloat64{Float64: *ps.AbsoluteWeight, Valid: true}
			}
			if ps.Notes != nil && *ps.Notes != "" {
				r.notes = sql.NullString{String: *ps.Notes, Valid: true}
			}
			rows = append(rows, r)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, &UnknownExercisesError{Names: unknown}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].sortOrder != rows[j].sortOrder {
			return rows[i].sortOrder < rows[j].sortOrder
		}
		return rows[i].setNumber < rows[j].setNumber
	})

	// Step 2: collision + delete + create + sets, all in one transaction.
	// Only tx-scoped statements below — the pure-Go SQLite driver pins a
	// single connection (SetMaxOpenConns(1)), so touching db-level helpers
	// while the transaction is open would deadlock.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("models: begin tx for WOD log: %w", err)
	}
	defer tx.Rollback()

	// Same-day collision (one resistance session per athlete per day).
	replaced := false
	var existingID int64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM workouts WHERE athlete_id = ? AND date = ? AND discipline = 'resistance'`,
		athleteID, date,
	).Scan(&existingID)
	switch {
	case err == nil:
		if !replace {
			return nil, ErrWODCollision
		}
		if _, derr := tx.ExecContext(ctx, `DELETE FROM workouts WHERE id = ?`, existingID); derr != nil {
			return nil, fmt.Errorf("models: replace existing workout %d for WOD: %w", existingID, derr)
		}
		replaced = true
	case !errors.Is(err, sql.ErrNoRows):
		return nil, fmt.Errorf("models: check existing workout for WOD: %w", err)
	}

	// Create the ad-hoc resistance workout (assignment_id NULL) + its sets.
	var workoutID int64
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO workouts (athlete_id, date, assignment_id, notes) VALUES (?, ?, NULL, NULL) RETURNING id`,
		athleteID, date,
	).Scan(&workoutID); err != nil {
		return nil, fmt.Errorf("models: create ad-hoc WOD workout: %w", err)
	}
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO workout_sets (workout_id, exercise_id, set_number, reps, weight, rpe, rep_type, category, notes) VALUES (?, ?, ?, ?, ?, NULL, ?, 'main', ?)`,
			workoutID, r.exerciseID, r.setNumber, r.reps, r.weight, r.repType, r.notes,
		); err != nil {
			return nil, fmt.Errorf("models: insert WOD set for exercise %d in workout %d: %w", r.exerciseID, workoutID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: commit WOD log: %w", err)
	}

	return &WODLogResult{
		WorkoutID:   workoutID,
		SetsCreated: len(rows),
		Replaced:    replaced,
	}, nil
}
