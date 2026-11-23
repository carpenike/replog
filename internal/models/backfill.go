package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Session backfill (ADR 027). A coach records a past resistance session
// in one call: the workout and all of its sets. PlanBackfill only reads.
// ApplyBackfill writes the workout and its sets in one transaction. PRs,
// e1RM, volume, and plan-vs-done read the sets, so a backfilled session
// counts the same way a live-logged one does. Existing workouts and sets
// are never changed.

const (
	// BackfillMaxSets caps one backfilled session.
	BackfillMaxSets = 200
	// BackfillMaxNotes caps the session note.
	BackfillMaxNotes = 2000
	backfillMaxReps  = 1000
	backfillMaxLbs   = 2000
)

// BackfillSetInput is one set as the coach sends it. WeightLbs nil (or 0)
// is bodyweight. Warmup marks a non-working set.
type BackfillSetInput struct {
	ExerciseID int64
	Reps       int
	WeightLbs  *float64
	RepType    string
	SetOrder   int
	Warmup     bool
}

// BackfillInput is the whole session as the coach sends it.
type BackfillInput struct {
	Date            string
	AssignmentID    int64
	TemplateID      int64
	TemplateVersion int
	ProgramWeek     int
	ProgramDay      int
	SessionRPE      *float64
	Notes           string
	Sets            []BackfillSetInput
}

// BackfillSet is one normalized set in session order.
type BackfillSet struct {
	SetOrder     int
	ExerciseID   int64
	ExerciseName string // empty when the id is unknown
	SetNumber    int    // 1-based per exercise, in session order
	Reps         int
	RepType      string
	WeightLbs    *float64
	SetType      string // working or warmup
}

// BackfillWarning is advisory unless the plan says it blocks confirm.
type BackfillWarning struct {
	Code        string
	Message     string
	ExerciseIDs []int64
}

// BackfillProgram is the assignment the session links to.
type BackfillProgram struct {
	AssignmentID    int64
	TemplateID      int64
	TemplateName    string
	TemplateVersion int
	StartDate       string
	Schedule        []int
	Week            int // position this session takes, derived from earlier linked sessions
	Day             int
	Label           string
	LaterSessions   int // linked sessions after this date; each moves one position later
}

// BackfillPlan is the checked session. It is what confirm writes.
type BackfillPlan struct {
	AthleteID   int64
	Date        string
	Weekday     int
	Program     *BackfillProgram
	SessionRPE  *float64
	Notes       string // as stored on the workout (session RPE line first)
	Sets        []BackfillSet
	Exercises   int
	Working     int
	Warmups     int
	Warnings    []BackfillWarning
	DuplicateID int64 // existing resistance session on that date, 0 if none
	Unknown     []int64
}

// CanConfirm reports whether confirm may write this plan.
func (p *BackfillPlan) CanConfirm() bool {
	return p.DuplicateID == 0 && len(p.Unknown) == 0
}

// BackfillFieldError is a 400 on one input field.
type BackfillFieldError struct {
	Field   string
	Message string
}

func (e *BackfillFieldError) Error() string { return e.Field + ": " + e.Message }

func backfillField(field, format string, args ...any) error {
	return &BackfillFieldError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// ErrBackfillDuplicate is returned by ApplyBackfill when a resistance
// session already exists for the athlete on that date.
var ErrBackfillDuplicate = errors.New("a resistance session already exists for this date")

func validBackfillRepType(t string) bool {
	switch t {
	case "reps", "each_side", "seconds", "distance":
		return true
	}
	return false
}

// PlanBackfill validates and normalizes a past session. today is the
// caller's calendar date (YYYY-MM-DD) in loc. It writes nothing.
func PlanBackfill(ctx context.Context, db *sql.DB, athleteID int64, in BackfillInput, today string, loc *time.Location) (*BackfillPlan, error) {
	day, err := time.ParseInLocation("2006-01-02", in.Date, loc)
	if err != nil {
		return nil, backfillField("date", "must be a valid date in YYYY-MM-DD format")
	}
	if in.Date > today {
		return nil, backfillField("date", "must not be in the future (today is %s)", today)
	}
	if in.SessionRPE != nil && (*in.SessionRPE < 1 || *in.SessionRPE > 10 || math.IsNaN(*in.SessionRPE)) {
		return nil, backfillField("session_rpe", "must be between 1 and 10")
	}
	notes := strings.TrimSpace(in.Notes)
	if len(notes) > BackfillMaxNotes {
		return nil, backfillField("notes", "must be at most %d characters", BackfillMaxNotes)
	}
	if len(in.Sets) == 0 {
		return nil, backfillField("sets", "must list at least one set")
	}
	if len(in.Sets) > BackfillMaxSets {
		return nil, backfillField("sets", "must list at most %d sets", BackfillMaxSets)
	}
	if in.ProgramDay < 0 || in.ProgramWeek < 0 || in.TemplateVersion < 0 || in.AssignmentID < 0 || in.TemplateID < 0 {
		return nil, backfillField("program", "ids, version, week and day must be positive")
	}
	if in.TemplateVersion > 0 && in.TemplateID == 0 && in.AssignmentID == 0 {
		return nil, backfillField("template_version", "needs template_id or assignment_id")
	}
	if (in.ProgramDay > 0 || in.ProgramWeek > 0) && in.TemplateID == 0 && in.AssignmentID == 0 {
		return nil, backfillField("program_day", "needs assignment_id or template_id")
	}

	sets := make([]BackfillSetInput, len(in.Sets))
	copy(sets, in.Sets)
	seenOrder := map[int]bool{}
	for i, s := range sets {
		field := fmt.Sprintf("sets[%d]", i)
		if s.ExerciseID <= 0 {
			return nil, backfillField(field+".exercise_id", "must be a positive id")
		}
		if s.Reps < 0 || s.Reps > backfillMaxReps {
			return nil, backfillField(field+".reps", "must be between 0 and %d", backfillMaxReps)
		}
		if s.WeightLbs != nil && (*s.WeightLbs < 0 || *s.WeightLbs > backfillMaxLbs || math.IsNaN(*s.WeightLbs)) {
			return nil, backfillField(field+".weight_lbs", "must be null (bodyweight) or between 0 and %d", backfillMaxLbs)
		}
		if s.RepType == "" {
			sets[i].RepType = "reps"
		} else if !validBackfillRepType(s.RepType) {
			return nil, backfillField(field+".rep_type", "must be one of reps, each_side, seconds, distance")
		}
		if s.SetOrder <= 0 {
			return nil, backfillField(field+".set_order", "must be a positive integer")
		}
		if seenOrder[s.SetOrder] {
			return nil, backfillField(field+".set_order", "%d is used twice", s.SetOrder)
		}
		seenOrder[s.SetOrder] = true
	}
	sort.SliceStable(sets, func(a, b int) bool { return sets[a].SetOrder < sets[b].SetOrder })

	plan := &BackfillPlan{AthleteID: athleteID, Date: in.Date, Weekday: isoWeekday(day), SessionRPE: in.SessionRPE, Warnings: []BackfillWarning{}}

	// Exercise names; unknown ids block confirm.
	names, err := exerciseNames(ctx, db, sets)
	if err != nil {
		return nil, err
	}
	perExercise := map[int64]int{}
	for _, s := range sets {
		perExercise[s.ExerciseID]++
		var weight *float64
		if s.WeightLbs != nil && *s.WeightLbs > 0 {
			v := *s.WeightLbs
			weight = &v
		}
		setType := SetTypeWorking
		if s.Warmup {
			setType = SetTypeWarmup
			plan.Warmups++
		} else {
			plan.Working++
		}
		name, ok := names[s.ExerciseID]
		if !ok && !containsID(plan.Unknown, s.ExerciseID) {
			plan.Unknown = append(plan.Unknown, s.ExerciseID)
		}
		plan.Sets = append(plan.Sets, BackfillSet{
			SetOrder: s.SetOrder, ExerciseID: s.ExerciseID, ExerciseName: name,
			SetNumber: perExercise[s.ExerciseID], Reps: s.Reps, RepType: s.RepType,
			WeightLbs: weight, SetType: setType,
		})
	}
	plan.Exercises = len(perExercise)
	if len(plan.Unknown) > 0 {
		plan.Warnings = append(plan.Warnings, BackfillWarning{
			Code:        "unknown_exercise",
			Message:     "These exercise ids are not in the catalog. Fix them before confirming; list exercises or create the missing one first.",
			ExerciseIDs: plan.Unknown,
		})
	}

	// Program link.
	program, err := resolveBackfillProgram(ctx, db, athleteID, in)
	if err != nil {
		return nil, err
	}
	noon := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, loc)
	if program != nil {
		if in.Date < dayOnly(program.StartDate) {
			return nil, backfillField("date", "is before assignment %d starts (%s)", program.ID, dayOnly(program.StartDate))
		}
		tmpl, err := GetProgramTemplateByID(ctx, db, program.TemplateID)
		if err != nil {
			return nil, fmt.Errorf("models: backfill template %d: %w", program.TemplateID, err)
		}
		if in.TemplateVersion > 0 && in.TemplateVersion != tmpl.Version {
			return nil, backfillField("template_version", "assignment %d uses template %d version %d, not version %d", program.ID, tmpl.ID, tmpl.Version, in.TemplateVersion)
		}
		var before, after int
		if err := db.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(date(date) < date(?)), 0), COALESCE(SUM(date(date) > date(?)), 0)
			   FROM workouts WHERE assignment_id = ?`, in.Date, in.Date, program.ID,
		).Scan(&before, &after); err != nil {
			return nil, fmt.Errorf("models: backfill program position: %w", err)
		}
		week, dayNum := ProgramPosition(before+1, program.NumWeeks, program.NumDays)
		bp := &BackfillProgram{
			AssignmentID: program.ID, TemplateID: program.TemplateID, TemplateName: program.TemplateName,
			TemplateVersion: tmpl.Version, StartDate: dayOnly(program.StartDate), Schedule: program.ScheduleDays(),
			Week: week, Day: dayNum, Label: ProgramDayLabel(before+1, program.NumWeeks, program.NumDays),
			LaterSessions: after,
		}
		plan.Program = bp
		if !program.IsLoop && before >= program.NumWeeks*program.NumDays {
			plan.Warnings = append(plan.Warnings, BackfillWarning{Code: "cycle_complete",
				Message: fmt.Sprintf("Assignment %d already has a full cycle of sessions before %s; this one starts a new cycle.", program.ID, in.Date)})
		}
		if bp.Schedule != nil && !containsInt(bp.Schedule, plan.Weekday) {
			plan.Warnings = append(plan.Warnings, BackfillWarning{Code: "off_schedule",
				Message: fmt.Sprintf("%s is a %s, which is not on assignment %d's schedule.", in.Date, noon.Weekday(), program.ID)})
		}
		if (in.ProgramDay > 0 && in.ProgramDay != dayNum) || (in.ProgramWeek > 0 && in.ProgramWeek != week) {
			plan.Warnings = append(plan.Warnings, BackfillWarning{Code: "program_day_mismatch",
				Message: fmt.Sprintf("RepLog will count this as %s (from the %d earlier linked sessions), not the week/day sent. Plan-vs-done compares against %s.", bp.Label, before, bp.Label)})
		}
		if after > 0 {
			plan.Warnings = append(plan.Warnings, BackfillWarning{Code: "later_sessions_shift",
				Message: fmt.Sprintf("%d later session(s) on assignment %d will each move one program day later. Backfill oldest first.", after, program.ID)})
		}
	} else {
		resolved, err := ResolveAssignment(ctx, db, athleteID, noon, loc.String())
		if err != nil {
			return nil, err
		}
		if resolved != nil && in.Date >= dayOnly(resolved.StartDate) {
			plan.Warnings = append(plan.Warnings, BackfillWarning{Code: "unlinked",
				Message: fmt.Sprintf("Assignment %d covers %s but no program link was sent. The session will not advance that program. Pass assignment_id to link it.", resolved.ID, in.Date)})
		}
	}

	plan.Notes = backfillNotes(in.SessionRPE, notes)

	existing, err := GetWorkoutByAthleteDate(ctx, db, athleteID, in.Date)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		plan.DuplicateID = existing.ID
		plan.Warnings = append(plan.Warnings, BackfillWarning{Code: "duplicate",
			Message: fmt.Sprintf("Session %d already exists for this athlete on %s with %d working set(s). Confirm will refuse. Nothing is merged or replaced.", existing.ID, in.Date, existing.SetCount)})
	}
	return plan, nil
}

func backfillNotes(rpe *float64, notes string) string {
	var parts []string
	if rpe != nil {
		parts = append(parts, "Session RPE: "+strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.1f", *rpe), "0"), ".")+"/10")
	}
	if notes != "" {
		parts = append(parts, notes)
	}
	return strings.Join(parts, "\n")
}

func resolveBackfillProgram(ctx context.Context, db *sql.DB, athleteID int64, in BackfillInput) (*AthleteProgram, error) {
	if in.AssignmentID > 0 {
		ap, err := GetAthleteProgramByID(ctx, db, in.AssignmentID)
		if err != nil || ap.AthleteID != athleteID {
			return nil, backfillField("assignment_id", "assignment %d is not one of this athlete's programs", in.AssignmentID)
		}
		if in.TemplateID > 0 && in.TemplateID != ap.TemplateID {
			return nil, backfillField("template_id", "assignment %d uses template %d, not %d", ap.ID, ap.TemplateID, in.TemplateID)
		}
		return ap, nil
	}
	if in.TemplateID == 0 {
		return nil, nil
	}
	all, err := ListAthletePrograms(ctx, db, athleteID)
	if err != nil {
		return nil, err
	}
	var best *AthleteProgram
	for _, ap := range all {
		if ap.TemplateID != in.TemplateID || dayOnly(ap.StartDate) > in.Date {
			continue
		}
		if best == nil || dayOnly(ap.StartDate) > dayOnly(best.StartDate) {
			best = ap
		}
	}
	if best == nil {
		return nil, backfillField("template_id", "no assignment of template %d for this athlete starts on or before %s", in.TemplateID, in.Date)
	}
	return best, nil
}

func exerciseNames(ctx context.Context, db *sql.DB, sets []BackfillSetInput) (map[int64]string, error) {
	ids := []any{}
	seen := map[int64]bool{}
	for _, s := range sets {
		if !seen[s.ExerciseID] {
			seen[s.ExerciseID] = true
			ids = append(ids, s.ExerciseID)
		}
	}
	rows, err := db.QueryContext(ctx, `SELECT id, name FROM exercises WHERE id IN (`+inClause(len(ids))+`)`, ids...)
	if err != nil {
		return nil, fmt.Errorf("models: backfill exercise names: %w", err)
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("models: scan backfill exercise: %w", err)
		}
		out[id] = name
	}
	return out, rows.Err()
}

func dayOnly(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func containsID(xs []int64, x int64) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func containsInt(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// ApplyBackfill writes the planned workout and every set in one
// transaction. It refuses with ErrBackfillDuplicate when a resistance
// session exists on that date, so nothing is merged or replaced.
func ApplyBackfill(ctx context.Context, db *sql.DB, plan *BackfillPlan) (int64, error) {
	if !plan.CanConfirm() {
		if plan.DuplicateID != 0 {
			return 0, ErrBackfillDuplicate
		}
		return 0, backfillField("sets", "unknown exercise ids")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("models: begin backfill: %w", err)
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workouts WHERE athlete_id = ? AND date = ? AND discipline = 'resistance'`,
		plan.AthleteID, plan.Date).Scan(&exists); err != nil {
		return 0, fmt.Errorf("models: backfill duplicate check: %w", err)
	}
	if exists > 0 {
		return 0, ErrBackfillDuplicate
	}
	var assign sql.NullInt64
	if plan.Program != nil {
		assign = sql.NullInt64{Int64: plan.Program.AssignmentID, Valid: true}
	}
	var notes sql.NullString
	if plan.Notes != "" {
		notes = sql.NullString{String: plan.Notes, Valid: true}
	}
	var workoutID int64
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO workouts (athlete_id, date, assignment_id, notes) VALUES (?, ?, ?, ?) RETURNING id`,
		plan.AthleteID, plan.Date, assign, notes).Scan(&workoutID); err != nil {
		if isUniqueViolation(err) {
			return 0, ErrBackfillDuplicate
		}
		return 0, fmt.Errorf("models: backfill workout: %w", err)
	}
	for _, s := range plan.Sets {
		var weight sql.NullFloat64
		if s.WeightLbs != nil {
			weight = sql.NullFloat64{Float64: *s.WeightLbs, Valid: true}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO workout_sets (workout_id, exercise_id, set_number, reps, weight, rep_type, category, set_type)
			 VALUES (?, ?, ?, ?, ?, ?, 'main', ?)`,
			workoutID, s.ExerciseID, s.SetNumber, s.Reps, weight, s.RepType, s.SetType); err != nil {
			return 0, fmt.Errorf("models: backfill set %d: %w", s.SetOrder, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("models: commit backfill: %w", err)
	}
	return workoutID, nil
}

// BackfillProgramLabel is the "Week N Day D" a linked session on date now
// holds within its assignment.
func BackfillProgramLabel(ctx context.Context, db *sql.DB, assignmentID int64, date string) (string, error) {
	ap, err := GetAthleteProgramByID(ctx, db, assignmentID)
	if err != nil {
		return "", err
	}
	var before int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workouts WHERE assignment_id = ? AND date(date) < date(?)`, assignmentID, date,
	).Scan(&before); err != nil {
		return "", fmt.Errorf("models: backfill label: %w", err)
	}
	return ProgramDayLabel(before+1, ap.NumWeeks, ap.NumDays), nil
}
