package models

import (
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
//  1. Resolve the same-day collision — if a resistance workout already
//     exists for the date and replace is false, return ErrWODCollision so
//     the handler can prompt replace-or-cancel. When replace is true, the
//     existing resistance workout is deleted (its sets cascade).
//  2. Resolve every prescribed-set exercise name to an exercise ID,
//     creating any missing exercise from the parsed catalog metadata. This
//     runs before the insert transaction because the pure-Go SQLite driver
//     pins a single connection (SetMaxOpenConns(1)); creating rows inside an
//     open transaction would deadlock.
//  3. Create the ad-hoc workout (assignment_id NULL → discipline defaults to
//     'resistance', which the read-path filters require so the WOD feeds
//     future BuildAthleteContext) and insert its sets in one transaction.
//
// The athlete then confirms/edits the seeded sets in the normal workout UI.
// A WOD has NumDays=1/NumWeeks=1, so all prescribed sets belong to the one
// session; insertion order follows (sort_order, set_number) so the circuit
// structure stays faithful.
func LogWODFromCatalog(db *sql.DB, athleteID int64, date string, parsed *importers.ParsedFile, replace bool) (*WODLogResult, error) {
	if parsed == nil {
		return nil, fmt.Errorf("models: log WOD with nil parsed catalog")
	}

	// Step 1: same-day collision.
	existing, err := GetWorkoutByAthleteDate(db, athleteID, date)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("models: check existing workout for WOD: %w", err)
	}
	replaced := false
	if err == nil && existing != nil {
		if !replace {
			return nil, ErrWODCollision
		}
		if derr := DeleteWorkout(db, existing.ID, athleteID); derr != nil {
			return nil, fmt.Errorf("models: replace existing workout %d for WOD: %w", existing.ID, derr)
		}
		replaced = true
	}

	// Step 2: resolve exercise names → IDs (create missing) BEFORE the tx.
	nameToID := make(map[string]int64)
	exercises, err := ListExercises(db, "")
	if err != nil {
		return nil, fmt.Errorf("models: list exercises for WOD: %w", err)
	}
	for _, ex := range exercises {
		nameToID[strings.ToLower(ex.Name)] = ex.ID
	}
	parsedMeta := make(map[string]importers.ParsedExercise, len(parsed.Exercises))
	for _, pe := range parsed.Exercises {
		parsedMeta[strings.ToLower(pe.Name)] = pe
	}
	resolve := func(name string) (int64, error) {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			return 0, fmt.Errorf("models: WOD set references an empty exercise name")
		}
		if id, ok := nameToID[key]; ok {
			return id, nil
		}
		tier, formNotes, demoURL := "", "", ""
		restSeconds := 0
		if pe, ok := parsedMeta[key]; ok {
			if pe.Tier != nil {
				tier = *pe.Tier
			}
			if pe.FormNotes != nil {
				formNotes = *pe.FormNotes
			}
			if pe.DemoURL != nil {
				demoURL = *pe.DemoURL
			}
			if pe.RestSeconds != nil {
				restSeconds = *pe.RestSeconds
			}
		}
		ex, cerr := CreateExercise(db, name, tier, formNotes, demoURL, restSeconds)
		if cerr != nil {
			return 0, fmt.Errorf("models: create exercise %q for WOD: %w", name, cerr)
		}
		nameToID[key] = ex.ID
		return ex.ID, nil
	}

	// Flatten the parsed sets, resolving exercise IDs up front.
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
	for _, prog := range parsed.Programs {
		for _, ps := range prog.Template.PrescribedSets {
			exID, rerr := resolve(ps.Exercise)
			if rerr != nil {
				return nil, rerr
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
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].sortOrder != rows[j].sortOrder {
			return rows[i].sortOrder < rows[j].sortOrder
		}
		return rows[i].setNumber < rows[j].setNumber
	})

	// Step 3: create the ad-hoc resistance workout + insert sets.
	workout, err := CreateWorkout(db, athleteID, date, "", 0)
	if err != nil {
		return nil, fmt.Errorf("models: create ad-hoc WOD workout: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("models: begin tx for WOD sets: %w", err)
	}
	defer tx.Rollback()
	for _, r := range rows {
		if _, err := tx.Exec(
			`INSERT INTO workout_sets (workout_id, exercise_id, set_number, reps, weight, rpe, rep_type, category, notes) VALUES (?, ?, ?, ?, ?, NULL, ?, 'main', ?)`,
			workout.ID, r.exerciseID, r.setNumber, r.reps, r.weight, r.repType, r.notes,
		); err != nil {
			return nil, fmt.Errorf("models: insert WOD set for exercise %d in workout %d: %w", r.exerciseID, workout.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: commit WOD sets: %w", err)
	}

	return &WODLogResult{
		WorkoutID:   workout.ID,
		SetsCreated: len(rows),
		Replaced:    replaced,
	}, nil
}
