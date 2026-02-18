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
	Program     *AthleteProgram
	CurrentWeek int
	CurrentDay  int
	CycleNumber int
	Lines       []*PrescriptionLine
	HasWorkout  bool   // true if athlete already has a workout logged today
	TodayDate   string // YYYY-MM-DD

	// Progress tracking within the current cycle.
	CompletedInCycle int     // workouts completed in the current cycle
	TotalInCycle     int     // total workouts in a full cycle (weeks × days)
	ProgressPercent  float64 // 0-100
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
	tms, err := ListCurrentTrainingMaxes(db, athleteID)
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
		// Compute per-set target weight from percentage × training max,
		// or use absolute_weight for fixed-weight prescriptions.
		if s.Percentage.Valid {
			if tm, ok := tmMap[s.ExerciseID]; ok {
				target := roundToNearest(s.Percentage.Float64/100*tm, 2.5)
				s.TargetWeight = &target
			}
		} else if s.AbsoluteWeight.Valid {
			w := s.AbsoluteWeight.Float64
			s.TargetWeight = &w
		}

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

		// Set line-level percentage and target weight from the first set that has them.
		if s.Percentage.Valid && line.Percentage == nil {
			pct := s.Percentage.Float64
			line.Percentage = &pct
			if tm, ok := tmMap[s.ExerciseID]; ok {
				line.TrainingMax = &tm
				target := roundToNearest(pct/100*tm, 2.5)
				line.TargetWeight = &target
			}
		} else if s.AbsoluteWeight.Valid && line.TargetWeight == nil {
			w := s.AbsoluteWeight.Float64
			line.TargetWeight = &w
		}
	}

	// Build ordered lines slice.
	lines := make([]*PrescriptionLine, 0, len(lineOrder))
	for _, eid := range lineOrder {
		lines = append(lines, lineMap[eid])
	}

	// Calculate progress within the current cycle.
	completedInCycle := position // position is 0-based index within cycle
	progressPct := 0.0
	if cycleLength > 0 {
		progressPct = float64(completedInCycle) / float64(cycleLength) * 100
	}

	return &Prescription{
		Program:          program,
		CurrentWeek:      currentWeek,
		CurrentDay:       currentDay,
		CycleNumber:      cycleNumber,
		Lines:            lines,
		HasWorkout:       hasWorkout,
		TodayDate:        todayStr,
		CompletedInCycle: completedInCycle,
		TotalInCycle:     cycleLength,
		ProgressPercent:  progressPct,
	}, nil
}

// roundToNearest rounds v to the nearest increment (e.g. 2.5 for plates).
func roundToNearest(v, increment float64) float64 {
	return math.Round(v/increment) * increment
}

// CycleReportDay holds the prescription lines for one day in a cycle.
type CycleReportDay struct {
	Week  int
	Day   int
	Lines []*PrescriptionLine
}

// CycleReport holds a complete cycle's worth of prescriptions for printing.
type CycleReport struct {
	Program     *AthleteProgram
	CycleNumber int
	Days        []*CycleReportDay
}

// GetCycleReport generates the full prescription for every day in the current cycle.
func GetCycleReport(db *sql.DB, athleteID int64, today time.Time) (*CycleReport, error) {
	program, err := GetActiveProgram(db, athleteID)
	if err != nil {
		return nil, err
	}
	if program == nil {
		return nil, nil
	}

	todayStr := today.Format("2006-01-02")

	// Count completed workouts to determine current cycle number.
	var completedWorkouts int
	err = db.QueryRow(
		`SELECT COUNT(*) FROM workouts WHERE athlete_id = ? AND date(date) >= date(?) AND date(date) < date(?)`,
		athleteID, program.StartDate, todayStr,
	).Scan(&completedWorkouts)
	if err != nil {
		return nil, fmt.Errorf("models: count workouts for cycle report: %w", err)
	}

	cycleLength := program.NumWeeks * program.NumDays
	if cycleLength == 0 {
		return nil, fmt.Errorf("models: program has zero cycle length")
	}
	cycleNumber := (completedWorkouts / cycleLength) + 1

	// Get training maxes.
	tms, err := ListCurrentTrainingMaxes(db, athleteID)
	if err != nil {
		return nil, err
	}
	tmMap := make(map[int64]float64)
	for _, tm := range tms {
		tmMap[tm.ExerciseID] = tm.Weight
	}

	// Build each day.
	var days []*CycleReportDay
	for w := 1; w <= program.NumWeeks; w++ {
		for d := 1; d <= program.NumDays; d++ {
			sets, err := ListPrescribedSetsForDay(db, program.TemplateID, w, d)
			if err != nil {
				return nil, err
			}

			lineMap := make(map[int64]*PrescriptionLine)
			var lineOrder []int64
			for _, s := range sets {
				// Compute per-set target weight.
				if s.Percentage.Valid {
					if tm, ok := tmMap[s.ExerciseID]; ok {
						target := roundToNearest(s.Percentage.Float64/100*tm, 2.5)
						s.TargetWeight = &target
					}
				} else if s.AbsoluteWeight.Valid {
					w := s.AbsoluteWeight.Float64
					s.TargetWeight = &w
				}

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

				if s.Percentage.Valid && line.Percentage == nil {
					pct := s.Percentage.Float64
					line.Percentage = &pct
					if tm, ok := tmMap[s.ExerciseID]; ok {
						line.TrainingMax = &tm
						target := roundToNearest(pct/100*tm, 2.5)
						line.TargetWeight = &target
					}
				} else if s.AbsoluteWeight.Valid && line.TargetWeight == nil {
					w := s.AbsoluteWeight.Float64
					line.TargetWeight = &w
				}
			}

			lines := make([]*PrescriptionLine, 0, len(lineOrder))
			for _, eid := range lineOrder {
				lines = append(lines, lineMap[eid])
			}

			days = append(days, &CycleReportDay{
				Week:  w,
				Day:   d,
				Lines: lines,
			})
		}
	}

	return &CycleReport{
		Program:     program,
		CycleNumber: cycleNumber,
		Days:        days,
	}, nil
}
