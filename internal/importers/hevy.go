package importers

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Hevy CSV columns.
const (
	hevyColTitle           = "title"
	hevyColStartTime       = "start_time"
	hevyColDescription     = "description"
	hevyColExerciseTitle   = "exercise_title"
	hevyColSetIndex        = "set_index"
	hevyColSetType         = "set_type"
	hevyColWeightLbs       = "weight_lbs"
	hevyColReps            = "reps"
	hevyColDurationSeconds = "duration_seconds"
	hevyColExerciseNotes   = "exercise_notes"
	hevyColRPE             = "rpe"
)

// ParseHevyCSV parses workout data from a Hevy app CSV export.
func ParseHevyCSV(r io.Reader) (*ParsedFile, error) {
	cr := csv.NewReader(r)
	cr.LazyQuotes = true

	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("importers: read hevy csv: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("importers: hevy csv has no data rows")
	}

	// Build column index from header row.
	header := records[0]
	idx := make(map[string]int)
	for i, col := range header {
		idx[strings.TrimSpace(col)] = i
	}

	// Validate required columns.
	for _, required := range []string{hevyColStartTime, hevyColExerciseTitle} {
		if _, ok := idx[required]; !ok {
			return nil, fmt.Errorf("importers: hevy csv missing required column %q", required)
		}
	}

	pf := &ParsedFile{Format: FormatHevyCSV}
	exerciseSet := make(map[string]bool)

	// Workouts keyed by date string.
	workoutMap := make(map[string]*ParsedWorkout)
	workoutOrder := []string{}
	workoutNotesMap := make(map[string]string)

	for _, row := range records[1:] {
		startTime := colVal(row, idx, hevyColStartTime)
		if startTime == "" {
			continue
		}

		date := parseStrongDate(startTime) // reuse date parser

		exerciseName := colVal(row, idx, hevyColExerciseTitle)
		if exerciseName == "" {
			continue
		}

		// Collect unique exercises.
		if !exerciseSet[exerciseName] {
			exerciseSet[exerciseName] = true
			pf.Exercises = append(pf.Exercises, ParsedExercise{Name: exerciseName})
		}

		// Get or create workout.
		pw, exists := workoutMap[date]
		if !exists {
			pw = &ParsedWorkout{Date: date}
			workoutMap[date] = pw
			workoutOrder = append(workoutOrder, date)
		}

		// Workout description as notes (first non-empty per workout).
		if desc := colVal(row, idx, hevyColDescription); desc != "" {
			if _, noted := workoutNotesMap[date]; !noted {
				workoutNotesMap[date] = desc
				pw.Notes = &desc
			}
		}

		// Parse set data.
		set := ParsedWorkoutSet{
			Exercise: exerciseName,
			RepType:  "reps",
		}

		// Hevy uses 0-indexed set_index; RepLog is 1-indexed.
		if v := colVal(row, idx, hevyColSetIndex); v != "" {
			si, _ := strconv.Atoi(v)
			set.SetNumber = si + 1
		} else {
			set.SetNumber = len(pw.Sets) + 1
		}

		if v := colVal(row, idx, hevyColWeightLbs); v != "" {
			w, err := strconv.ParseFloat(v, 64)
			if err == nil && w > 0 {
				set.Weight = &w
			}
		}

		if v := colVal(row, idx, hevyColReps); v != "" {
			set.Reps, _ = strconv.Atoi(v)
		}

		// Duration-based sets: if reps is 0 but duration has a value.
		if set.Reps == 0 {
			if v := colVal(row, idx, hevyColDurationSeconds); v != "" {
				secs, _ := strconv.Atoi(v)
				if secs > 0 {
					set.Reps = secs
					set.RepType = "seconds"
				}
			}
		}

		if v := colVal(row, idx, hevyColRPE); v != "" {
			rpe, err := strconv.ParseFloat(v, 64)
			if err == nil && rpe >= 1 && rpe <= 10 {
				set.RPE = &rpe
			}
		}

		// Annotate warmup sets in notes.
		setType := colVal(row, idx, hevyColSetType)
		exerciseNotes := colVal(row, idx, hevyColExerciseNotes)

		var noteParts []string
		if setType == "warmup" {
			noteParts = append(noteParts, "[warmup]")
		}
		if exerciseNotes != "" {
			noteParts = append(noteParts, exerciseNotes)
		}
		if len(noteParts) > 0 {
			combined := strings.Join(noteParts, " ")
			set.Notes = &combined
		}

		pw.Sets = append(pw.Sets, set)
	}

	// Preserve workout order.
	for _, date := range workoutOrder {
		pf.Workouts = append(pf.Workouts, *workoutMap[date])
	}

	return pf, nil
}
