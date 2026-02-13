package models

import (
	"database/sql"
	"fmt"
	"time"
)

// WeeklyStreak represents one week's exercise completion data for an athlete.
type WeeklyStreak struct {
	WeekStart      string // Monday date (YYYY-MM-DD)
	WeekEnd        string // Sunday date (YYYY-MM-DD)
	AssignedCount  int    // Number of assigned exercises that week
	CompletedCount int    // Number of distinct assigned exercises logged
}

// Status returns a classification for display: "complete", "partial", "missed", or "none".
func (ws *WeeklyStreak) Status() string {
	if ws.AssignedCount == 0 {
		return "none"
	}
	if ws.CompletedCount >= ws.AssignedCount {
		return "complete"
	}
	if ws.CompletedCount > 0 {
		return "partial"
	}
	return "missed"
}

// Label returns a short display label (e.g., "3/5").
func (ws *WeeklyStreak) Label() string {
	if ws.AssignedCount == 0 {
		return "—"
	}
	return fmt.Sprintf("%d/%d", ws.CompletedCount, ws.AssignedCount)
}

// WeeklyStreaks returns completion data for the last `weeks` weeks for an athlete.
// Each week runs Monday–Sunday. The current (possibly incomplete) week is included.
//
// For each week, we count:
//   - Assigned: exercises that were actively assigned at any point during the week
//     (simplification: uses current active assignments for all weeks)
//   - Completed: distinct assigned exercises with at least one logged set that week
func WeeklyStreaks(db *sql.DB, athleteID int64, weeks int) ([]*WeeklyStreak, error) {
	if weeks <= 0 {
		weeks = 8
	}

	// Find the Monday of the current week.
	now := time.Now()
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	monday := now.AddDate(0, 0, -int(weekday-time.Monday))
	monday = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)

	// Get current active assignment exercise IDs.
	assignedRows, err := db.Query(
		`SELECT DISTINCT exercise_id FROM athlete_exercises
		 WHERE athlete_id = ? AND active = 1`,
		athleteID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: weekly streaks assignments: %w", err)
	}
	defer assignedRows.Close()

	var assignedIDs []int64
	assignedSet := make(map[int64]bool)
	for assignedRows.Next() {
		var eid int64
		if err := assignedRows.Scan(&eid); err != nil {
			return nil, fmt.Errorf("models: scan assigned exercise: %w", err)
		}
		assignedIDs = append(assignedIDs, eid)
		assignedSet[eid] = true
	}
	if err := assignedRows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate assigned exercises: %w", err)
	}

	assignedCount := len(assignedIDs)

	// Build results from oldest week to newest.
	streaks := make([]*WeeklyStreak, weeks)
	startMonday := monday.AddDate(0, 0, -(weeks-1)*7)

	for i := 0; i < weeks; i++ {
		weekStart := startMonday.AddDate(0, 0, i*7)
		weekEnd := weekStart.AddDate(0, 0, 6)
		streaks[i] = &WeeklyStreak{
			WeekStart:     weekStart.Format("2006-01-02"),
			WeekEnd:       weekEnd.Format("2006-01-02"),
			AssignedCount: assignedCount,
		}
	}

	if assignedCount == 0 {
		return streaks, nil
	}

	// Query distinct exercises logged per week in the date range.
	rangeStart := startMonday.Format("2006-01-02")
	rangeEnd := monday.AddDate(0, 0, 6).Format("2006-01-02")

	rows, err := db.Query(
		`SELECT date(w.date), ws.exercise_id
		 FROM workout_sets ws
		 JOIN workouts w ON w.id = ws.workout_id
		 WHERE w.athlete_id = ?
		   AND date(w.date) >= date(?)
		   AND date(w.date) <= date(?)
		 GROUP BY date(w.date), ws.exercise_id`,
		athleteID, rangeStart, rangeEnd,
	)
	if err != nil {
		return nil, fmt.Errorf("models: weekly streaks sets: %w", err)
	}
	defer rows.Close()

	// Map: week index → set of exercise IDs completed.
	weekCompleted := make(map[int]map[int64]bool)

	for rows.Next() {
		var dateStr string
		var exerciseID int64
		if err := rows.Scan(&dateStr, &exerciseID); err != nil {
			return nil, fmt.Errorf("models: scan streak set: %w", err)
		}

		// Only count assigned exercises.
		if !assignedSet[exerciseID] {
			continue
		}

		// Determine which week this date belongs to.
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		daysSinceStart := int(d.Sub(startMonday).Hours() / 24)
		weekIdx := daysSinceStart / 7
		if weekIdx < 0 || weekIdx >= weeks {
			continue
		}

		if weekCompleted[weekIdx] == nil {
			weekCompleted[weekIdx] = make(map[int64]bool)
		}
		weekCompleted[weekIdx][exerciseID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate streak sets: %w", err)
	}

	for idx, completed := range weekCompleted {
		streaks[idx].CompletedCount = len(completed)
	}

	return streaks, nil
}
