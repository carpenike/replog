package models

import (
	"database/sql"
	"fmt"
	"math"
	"time"
)

// PrescriptionLine represents one exercise's prescription for today,
// with sets collapsed into a summary and target weight calculated from TM.
type PrescriptionLine struct {
	ExerciseName string
	ExerciseID   int64
	Sets         []*PrescribedSet
	TrainingMax  *float64 // nil if no TM set
	TargetWeight *float64 // calculated from percentage * TM
	Percentage   *float64 // from the prescribed set
}

// PercentageLabel returns a formatted string like "75%" or empty if nil.
func (pl *PrescriptionLine) PercentageLabel() string {
	if pl.Percentage == nil {
		return ""
	}
	return fmt.Sprintf("%.0f%%", *pl.Percentage)
}

// TargetWeightLabel returns a formatted string like "185.0" or empty if nil.
func (pl *PrescriptionLine) TargetWeightLabel() string {
	if pl.TargetWeight == nil {
		return ""
	}
	return fmt.Sprintf("%.1f", *pl.TargetWeight)
}

// SetsSummary returns a compact summary like "3×5" or "5/3/1+".
func (pl *PrescriptionLine) SetsSummary() string {
	if len(pl.Sets) == 0 {
		return ""
	}

	// Check if all sets have the same reps.
	allSame := true
	firstReps := pl.Sets[0].Reps
	for _, s := range pl.Sets[1:] {
		if s.Reps != firstReps {
			allSame = false
			break
		}
	}

	if allSame {
		repsLabel := pl.Sets[0].RepsLabel()
		return fmt.Sprintf("%d×%s", len(pl.Sets), repsLabel)
	}

	// Different reps per set — list them.
	result := ""
	for i, s := range pl.Sets {
		if i > 0 {
			result += "/"
		}
		result += s.RepsLabel()
	}
	return result
}

// Prescription holds today's training prescription for an athlete.
type Prescription struct {
	Program      *AthleteProgram
	CurrentWeek  int
	CurrentDay   int
	CycleNumber  int
	Lines        []*PrescriptionLine
	HasWorkout   bool   // true if athlete already has a workout logged today
	TodayDate    string // YYYY-MM-DD
}

// GetPrescription calculates today's training prescription for an athlete.
// Position in the program is determined by counting completed workouts since start_date.
// The cycle repeats automatically when all weeks×days are exhausted.
func GetPrescription(db *sql.DB, athleteID int64, today time.Time) (*Prescription, error) {
	// Get the athlete's active program.
	program, err := GetActiveProgram(db, athleteID)
	if err != nil {
		return nil, err
	}
	if program == nil {
		return nil, nil // No active program.
	}

	todayStr := today.Format("2006-01-02")

	// Count workouts since program start (not including today).
	// Use date() to normalize stored timestamps (modernc.org/sqlite stores DATE as Julian day reals).
	var completedWorkouts int
	err = db.QueryRow(
		`SELECT COUNT(*) FROM workouts WHERE athlete_id = ? AND date(date) >= date(?) AND date(date) < date(?)`,
		athleteID, program.StartDate, todayStr,
	).Scan(&completedWorkouts)
	if err != nil {
		return nil, fmt.Errorf("models: count workouts for prescription: %w", err)
	}

	// Check if there's a workout today.
	var todayCount int
	err = db.QueryRow(
		`SELECT COUNT(*) FROM workouts WHERE athlete_id = ? AND date(date) = date(?)`,
		athleteID, todayStr,
	).Scan(&todayCount)
	if err != nil {
		return nil, fmt.Errorf("models: check today workout: %w", err)
	}
	hasWorkout := todayCount > 0

	// Calculate position in the program cycle.
	cycleLength := program.NumWeeks * program.NumDays
	if cycleLength == 0 {
		return nil, fmt.Errorf("models: program has zero cycle length")
	}

	position := completedWorkouts % cycleLength
	cycleNumber := (completedWorkouts / cycleLength) + 1
	currentWeek := (position / program.NumDays) + 1
	currentDay := (position % program.NumDays) + 1

	// Get prescribed sets for this week/day.
	sets, err := ListPrescribedSetsForDay(db, program.TemplateID, currentWeek, currentDay)
	if err != nil {
		return nil, err
	}

	// Get current training maxes for the athlete.
	tms, err := CurrentTrainingMaxes(db, athleteID)
	if err != nil {
		return nil, err
	}

	// Build a map of exercise_id → training max weight.
	tmMap := make(map[int64]float64)
	for _, tm := range tms {
		tmMap[tm.ExerciseID] = tm.Weight
	}

	// Group sets by exercise and calculate target weights.
	lineMap := make(map[int64]*PrescriptionLine)
	var lineOrder []int64
	for _, s := range sets {
		line, exists := lineMap[s.ExerciseID]
		if !exists {
			line = &PrescriptionLine{
				ExerciseName: s.ExerciseName,
				ExerciseID:   s.ExerciseID,
			}
			lineMap[s.ExerciseID] = line
			lineOrder = append(lineOrder, s.ExerciseID)
		}
		line.Sets = append(line.Sets, s)

		// Set percentage and target weight from the first set that has them.
		if s.Percentage.Valid && line.Percentage == nil {
			pct := s.Percentage.Float64
			line.Percentage = &pct
			if tm, ok := tmMap[s.ExerciseID]; ok {
				line.TrainingMax = &tm
				target := roundToNearest(pct/100*tm, 2.5) // Round to nearest 2.5 lb
				line.TargetWeight = &target
			}
		}
	}

	// Build ordered lines slice.
	lines := make([]*PrescriptionLine, 0, len(lineOrder))
	for _, eid := range lineOrder {
		lines = append(lines, lineMap[eid])
	}

	return &Prescription{
		Program:     program,
		CurrentWeek: currentWeek,
		CurrentDay:  currentDay,
		CycleNumber: cycleNumber,
		Lines:       lines,
		HasWorkout:  hasWorkout,
		TodayDate:   todayStr,
	}, nil
}

// CurrentTrainingMaxes returns the most recent training max for each exercise
// assigned to an athlete.
func CurrentTrainingMaxes(db *sql.DB, athleteID int64) ([]*TrainingMax, error) {
	rows, err := db.Query(
		`SELECT tm.id, tm.athlete_id, tm.exercise_id, tm.weight, tm.effective_date, tm.notes, tm.created_at, e.name
		 FROM training_maxes tm
		 JOIN exercises e ON e.id = tm.exercise_id
		 WHERE tm.athlete_id = ?
		   AND tm.effective_date = (
		       SELECT MAX(tm2.effective_date)
		       FROM training_maxes tm2
		       WHERE tm2.athlete_id = tm.athlete_id AND tm2.exercise_id = tm.exercise_id
		   )
		 ORDER BY e.name COLLATE NOCASE`,
		athleteID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: current training maxes for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var maxes []*TrainingMax
	for rows.Next() {
		tm := &TrainingMax{}
		if err := rows.Scan(&tm.ID, &tm.AthleteID, &tm.ExerciseID, &tm.Weight,
			&tm.EffectiveDate, &tm.Notes, &tm.CreatedAt, &tm.ExerciseName); err != nil {
			return nil, fmt.Errorf("models: scan training max: %w", err)
		}
		maxes = append(maxes, tm)
	}
	return maxes, rows.Err()
}

// roundToNearest rounds v to the nearest increment (e.g. 2.5 for plates).
func roundToNearest(v, increment float64) float64 {
	return math.Round(v/increment) * increment
}
