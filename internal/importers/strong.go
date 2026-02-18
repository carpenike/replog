package importers

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Strong CSV columns (as exported by the Strong app).
// Date,Workout Name,Duration,Exercise Name,Set Order,Weight,Reps,Distance,Seconds,Notes,Workout Notes,RPE
const (
	strongColDate         = "Date"
	strongColWorkoutName  = "Workout Name"
	strongColExerciseName = "Exercise Name"
	strongColSetOrder     = "Set Order"
	strongColWeight       = "Weight"
	strongColReps         = "Reps"
	strongColSeconds      = "Seconds"
	strongColNotes        = "Notes"
	strongColWorkoutNotes = "Workout Notes"
	strongColRPE          = "RPE"
)

// ParseStrongCSV parses workout data from a Strong app CSV export.
func ParseStrongCSV(r io.Reader) (*ParsedFile, error) {
	cr := csv.NewReader(r)
	cr.LazyQuotes = true

	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("importers: read strong csv: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("importers: strong csv has no data rows")
	}

	// Build column index from header row.
	header := records[0]
	idx := make(map[string]int)
	for i, col := range header {
		idx[strings.TrimSpace(col)] = i
	}

	// Validate required columns.
	for _, required := range []string{strongColDate, strongColExerciseName} {
		if _, ok := idx[required]; !ok {
			return nil, fmt.Errorf("importers: strong csv missing required column %q", required)
		}
	}

	pf := &ParsedFile{Format: FormatStrongCSV}
	exerciseSet := make(map[string]bool)

	// Workouts keyed by date string.
	workoutMap := make(map[string]*ParsedWorkout)
	workoutOrder := []string{}
	workoutNotesMap := make(map[string]string)

	for _, row := range records[1:] {
		dateStr := colVal(row, idx, strongColDate)
		if dateStr == "" {
			continue
		}

		date := parseStrongDate(dateStr)

		exerciseName := colVal(row, idx, strongColExerciseName)
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

		// Workout notes (first non-empty value per workout wins).
		if wn := colVal(row, idx, strongColWorkoutNotes); wn != "" {
			if _, noted := workoutNotesMap[date]; !noted {
				workoutNotesMap[date] = wn
				pw.Notes = &wn
			}
		}

		// Parse set data.
		set := ParsedWorkoutSet{
			Exercise: exerciseName,
			RepType:  "reps",
		}

		if v := colVal(row, idx, strongColSetOrder); v != "" {
			set.SetNumber, _ = strconv.Atoi(v)
		} else {
			// Auto-number if not provided.
			set.SetNumber = len(pw.Sets) + 1
		}

		if v := colVal(row, idx, strongColWeight); v != "" {
			w, err := strconv.ParseFloat(v, 64)
			if err == nil && w > 0 {
				set.Weight = &w
			}
		}

		if v := colVal(row, idx, strongColReps); v != "" {
			set.Reps, _ = strconv.Atoi(v)
		}

		// Duration-based exercises: if reps is 0 but seconds has a value.
		if set.Reps == 0 {
			if v := colVal(row, idx, strongColSeconds); v != "" {
				secs, _ := strconv.Atoi(v)
				if secs > 0 {
					set.Reps = secs
					set.RepType = "seconds"
				}
			}
		}

		if v := colVal(row, idx, strongColRPE); v != "" {
			rpe, err := strconv.ParseFloat(v, 64)
			if err == nil && rpe >= 1 && rpe <= 10 {
				set.RPE = &rpe
			}
		}

		if v := colVal(row, idx, strongColNotes); v != "" {
			set.Notes = &v
		}

		pw.Sets = append(pw.Sets, set)
	}

	// Preserve workout order.
	for _, date := range workoutOrder {
		pf.Workouts = append(pf.Workouts, *workoutMap[date])
	}

	return pf, nil
}

// parseStrongDate parses the date formats commonly seen in Strong exports.
// Strong uses formats like "2026-02-15 14:30:00" or "2026 Feb 15".
func parseStrongDate(s string) string {
	s = strings.TrimSpace(s)

	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
		"2006 Jan 02",
		"2006 Jan 2",
		"Jan 2, 2006",
		"01/02/2006",
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.Format("2006-01-02")
		}
	}

	// Fallback: return first 10 characters if they look like a date.
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// colVal safely gets a column value from a CSV row.
func colVal(row []string, idx map[string]int, col string) string {
	i, ok := idx[col]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}
