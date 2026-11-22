package models

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Compliance cell statuses. Planned days carry the week plan's status
// renamed for the grid (upcoming → prescribed, missed → skipped); a day
// with no planned session is rest, or extra when a lifting session was
// logged anyway.
const (
	ComplianceRest       = "rest"
	CompliancePrescribed = "prescribed"
	ComplianceDone       = "done"
	CompliancePartial    = "partial"
	ComplianceSkipped    = "skipped"
	ComplianceExtra      = "extra"
)

// ComplianceCell is one athlete-day in the coach compliance grid.
type ComplianceCell struct {
	Date            string
	Weekday         int // ISO: 1 = Monday … 7 = Sunday
	Status          string
	SessionID       sql.NullInt64
	ProgramName     string
	ProgramDayLabel string
}

// ComplianceRow is one athlete's Monday–Sunday row.
type ComplianceRow struct {
	AthleteID int64
	Name      string
	Plan      *WeekPlan
	Extra     int
	Days      []ComplianceCell // always 7, Monday first
}

// ComplianceWeek builds one athlete's grid row from the week
// plan-versus-done read. It only reads; nothing here changes a plan.
func ComplianceWeek(ctx context.Context, db *sql.DB, athleteID int64, name, weekStart, today string, loc *time.Location) (*ComplianceRow, error) {
	plan, err := WeekPlanForAthlete(ctx, db, athleteID, weekStart, today, loc)
	if err != nil {
		return nil, err
	}
	logged, err := resistanceSessionsBetween(ctx, db, athleteID, plan.WeekStart, plan.WeekEnd)
	if err != nil {
		return nil, err
	}
	planned := make(map[string]PlannedDay, len(plan.Days))
	for _, d := range plan.Days {
		planned[d.Date] = d
	}
	start, err := time.ParseInLocation("2006-01-02", plan.WeekStart, loc)
	if err != nil {
		return nil, fmt.Errorf("models: compliance week start: %w", err)
	}
	row := &ComplianceRow{AthleteID: athleteID, Name: name, Plan: plan, Days: make([]ComplianceCell, 0, 7)}
	for i := 0; i < 7; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		cell := ComplianceCell{Date: date, Weekday: i + 1, Status: ComplianceRest}
		if day, ok := planned[date]; ok {
			cell.Status = complianceStatus(day.Status)
			cell.SessionID = day.SessionID
			cell.ProgramName = day.ProgramName
			cell.ProgramDayLabel = day.ProgramDayLabel
		} else if id, ok := logged[date]; ok {
			cell.Status = ComplianceExtra
			cell.SessionID = sql.NullInt64{Int64: id, Valid: true}
			row.Extra++
		}
		row.Days = append(row.Days, cell)
	}
	return row, nil
}

func complianceStatus(planStatus string) string {
	switch planStatus {
	case "done":
		return ComplianceDone
	case "partial":
		return CompliancePartial
	case "missed":
		return ComplianceSkipped
	default:
		return CompliancePrescribed
	}
}

// resistanceSessionsBetween maps date → workout id for lifting sessions
// with at least one counted (non-warm-up) set in [start, end].
func resistanceSessionsBetween(ctx context.Context, db *sql.DB, athleteID int64, start, end string) (map[string]int64, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT w.id, date(w.date)
		  FROM workouts w
		 WHERE w.athlete_id = ?
		   AND w.discipline = 'resistance'
		   AND date(w.date) BETWEEN ? AND ?
		   AND EXISTS (SELECT 1 FROM workout_sets ws WHERE ws.workout_id = w.id AND `+countedSet+`)
		 ORDER BY w.id`, athleteID, start, end)
	if err != nil {
		return nil, fmt.Errorf("models: resistance sessions for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id int64
		var date string
		if err := rows.Scan(&id, &date); err != nil {
			return nil, fmt.Errorf("models: scan resistance session: %w", err)
		}
		if _, seen := out[date]; !seen {
			out[date] = id
		}
	}
	return out, rows.Err()
}
