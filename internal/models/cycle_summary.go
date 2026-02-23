package models

import (
	"database/sql"
	"fmt"
	"time"
)

// TMSuggestion represents a suggested training max bump for one exercise after a cycle.
type TMSuggestion struct {
	ExerciseID   int64
	ExerciseName string
	CurrentTM    float64 // current training max weight
	Increment    float64 // from progression rule
	SuggestedTM  float64 // current + increment
	AMRAPResults []AMRAPResult
}

// IncrementLabel returns a formatted increment (e.g. "10", "5", "2.5").
func (s *TMSuggestion) IncrementLabel() string {
	if s.Increment == float64(int(s.Increment)) {
		return fmt.Sprintf("%.0f", s.Increment)
	}
	return fmt.Sprintf("%.1f", s.Increment)
}

// AMRAPResult records what an athlete hit on an AMRAP set during a cycle.
type AMRAPResult struct {
	Week         int
	Day          int
	Reps         int
	Weight       float64
	ExerciseName string
	WorkoutDate  string
}

// CycleSummary holds the review data for a completed (or current) cycle.
type CycleSummary struct {
	Program      *AthleteProgram
	CycleNumber  int
	Suggestions  []*TMSuggestion
	AllAMRAPs    []AMRAPResult
	CycleStart   string // YYYY-MM-DD of first workout in the cycle
	CycleEnd     string // YYYY-MM-DD of last workout in the cycle
}

// GetCycleSummary produces TM bump suggestions for an athlete's last completed cycle.
// It looks at the previous cycle (the one before the current position) and joins
// progression rules with AMRAP results and current training maxes.
// If program is nil, returns nil.
func GetCycleSummary(db *sql.DB, program *AthleteProgram, today time.Time) (*CycleSummary, error) {
	if program == nil {
		return nil, nil
	}

	todayStr := today.Format("2006-01-02")

	// Count completed workouts linked to this assignment.
	var completedWorkouts int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM workouts WHERE assignment_id = ? AND date(date) < date(?)`,
		program.ID, todayStr,
	).Scan(&completedWorkouts)
	if err != nil {
		return nil, fmt.Errorf("models: count workouts for cycle summary: %w", err)
	}

	cycleLength := program.NumWeeks * program.NumDays
	if cycleLength == 0 {
		return nil, fmt.Errorf("models: program has zero cycle length")
	}

	// We want to review the last completed cycle.
	// If we're mid-cycle in cycle 1, there's no completed cycle to review.
	currentCycle := (completedWorkouts / cycleLength) + 1
	if currentCycle < 2 && completedWorkouts < cycleLength {
		// Still in cycle 1 — no completed cycle to review.
		return nil, nil
	}

	// The completed cycle is the one before the current one.
	reviewCycle := currentCycle - 1
	if completedWorkouts%cycleLength == 0 && completedWorkouts > 0 {
		// Exactly at cycle boundary — review the just-completed cycle.
		reviewCycle = currentCycle - 1
	}

	// Find workouts in the reviewed cycle by offset.
	// Cycle N covers workouts [(N-1)*cycleLength .. N*cycleLength-1] (0-indexed).
	cycleStartOffset := (reviewCycle - 1) * cycleLength
	cycleEndOffset := reviewCycle * cycleLength

	// Get workout dates for the reviewed cycle.
	cycleWorkoutRows, err := db.Query(
		`SELECT date(date) FROM workouts
		 WHERE assignment_id = ?
		 ORDER BY date(date)
		 LIMIT ? OFFSET ?`,
		program.ID, cycleLength, cycleStartOffset,
	)
	if err != nil {
		return nil, fmt.Errorf("models: get cycle workouts: %w", err)
	}

	var cycleWorkoutDates []string
	for cycleWorkoutRows.Next() {
		var d string
		if err := cycleWorkoutRows.Scan(&d); err != nil {
			cycleWorkoutRows.Close()
			return nil, fmt.Errorf("models: scan cycle workout date: %w", err)
		}
		cycleWorkoutDates = append(cycleWorkoutDates, d)
	}
	cycleWorkoutRows.Close()
	if err := cycleWorkoutRows.Err(); err != nil {
		return nil, fmt.Errorf("models: cycle workout rows: %w", err)
	}

	if len(cycleWorkoutDates) == 0 {
		return nil, nil
	}

	cycleStart := cycleWorkoutDates[0]
	cycleEnd := cycleWorkoutDates[len(cycleWorkoutDates)-1]
	_ = cycleEndOffset // used conceptually

	// Get AMRAP results: workout_sets that correspond to AMRAP prescribed sets
	// (prescribed_sets.reps IS NULL) during the reviewed cycle's date range.
	amrapRows, err := db.Query(
		`SELECT e.id, e.name, ws.reps, ws.weight, w.date, ps.week, ps.day
		 FROM workout_sets ws
		 JOIN workouts w ON w.id = ws.workout_id
		 JOIN exercises e ON e.id = ws.exercise_id
		 JOIN prescribed_sets ps ON ps.exercise_id = ws.exercise_id
		     AND ps.template_id = ?
		     AND ps.reps IS NULL
		 WHERE w.assignment_id = ?
		   AND date(w.date) >= date(?)
		   AND date(w.date) <= date(?)
		   AND ws.weight IS NOT NULL
		 ORDER BY e.name COLLATE NOCASE, w.date, ws.set_number DESC
		 LIMIT 100`,
		program.TemplateID, program.ID, cycleStart, cycleEnd,
	)
	if err != nil {
		return nil, fmt.Errorf("models: get AMRAP results: %w", err)
	}

	// Deduplicate: for each exercise+date, keep only the heaviest set (first due to ORDER BY).
	type amrapKey struct {
		exerciseID int64
		date       string
	}
	seen := make(map[amrapKey]bool)
	var amraps []AMRAPResult
	amrapByExercise := make(map[int64][]AMRAPResult)

	for amrapRows.Next() {
		var (
			exID   int64
			exName string
			reps   int
			weight sql.NullFloat64
			date   string
			week   int
			day    int
		)
		if err := amrapRows.Scan(&exID, &exName, &reps, &weight, &date, &week, &day); err != nil {
			amrapRows.Close()
			return nil, fmt.Errorf("models: scan AMRAP result: %w", err)
		}

		key := amrapKey{exID, normalizeDate(date)}
		if seen[key] {
			continue
		}
		seen[key] = true

		ar := AMRAPResult{
			Week:         week,
			Day:          day,
			Reps:         reps,
			Weight:       weight.Float64,
			ExerciseName: exName,
			WorkoutDate:  normalizeDate(date),
		}
		amraps = append(amraps, ar)
		amrapByExercise[exID] = append(amrapByExercise[exID], ar)
	}
	amrapRows.Close()
	if err := amrapRows.Err(); err != nil {
		return nil, fmt.Errorf("models: AMRAP rows: %w", err)
	}

	// Get progression rules for this template.
	rules, err := ListProgressionRules(db, program.TemplateID)
	if err != nil {
		return nil, err
	}

	// Get current training maxes.
	tms, err := ListCurrentTrainingMaxes(db, program.AthleteID)
	if err != nil {
		return nil, err
	}
	tmMap := make(map[int64]float64)
	for _, tm := range tms {
		tmMap[tm.ExerciseID] = tm.Weight
	}

	// Build suggestions for each exercise that has a progression rule + current TM.
	var suggestions []*TMSuggestion
	for _, rule := range rules {
		currentTM, hasTM := tmMap[rule.ExerciseID]
		if !hasTM {
			continue // no TM set — skip suggestion
		}

		suggestions = append(suggestions, &TMSuggestion{
			ExerciseID:   rule.ExerciseID,
			ExerciseName: rule.ExerciseName,
			CurrentTM:    currentTM,
			Increment:    rule.Increment,
			SuggestedTM:  currentTM + rule.Increment,
			AMRAPResults: amrapByExercise[rule.ExerciseID],
		})
	}

	return &CycleSummary{
		Program:     program,
		CycleNumber: reviewCycle,
		Suggestions: suggestions,
		AllAMRAPs:   amraps,
		CycleStart:  cycleStart,
		CycleEnd:    cycleEnd,
	}, nil
}
