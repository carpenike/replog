package models

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// CalendarSession is one logged session of any discipline on a day.
type CalendarSession struct {
	WorkoutID  int64
	Discipline string // resistance | throwing | conditioning | skill | recovery
	Label      string // throw type, modality, or skill type; "" otherwise
	SetCount   int    // counted (non-warm-up) sets; resistance only
}

// CalendarDay is one date in an athlete's calendar.
type CalendarDay struct {
	Date        string
	Scheduled   bool
	ProgramID   int64
	ProgramName string
	// Status is the week plan status for a scheduled day (done, partial,
	// missed, upcoming) and "" for an unscheduled day.
	Status   string
	Sessions []CalendarSession
}

// AthleteCalendarData is a date range of days plus the season phases that
// overlap it.
type AthleteCalendarData struct {
	Start  string
	End    string
	Days   []CalendarDay
	Phases []*SeasonPhase
}

// AthleteCalendar returns every day in [start, end] (YYYY-MM-DD, at most
// 62 days) with its scheduled program day, logged sessions, and the season
// phases overlapping the range. Scheduled days come from the same stored
// weekday schedules as the week plan read; nothing is invented. Read only.
func AthleteCalendar(ctx context.Context, db *sql.DB, athleteID int64, start, end, today string, loc *time.Location) (*AthleteCalendarData, error) {
	first, err := time.ParseInLocation("2006-01-02", start, loc)
	if err != nil {
		return nil, fmt.Errorf("models: calendar start: %w", err)
	}
	last, err := time.ParseInLocation("2006-01-02", end, loc)
	if err != nil {
		return nil, fmt.Errorf("models: calendar end: %w", err)
	}
	if last.Before(first) || last.Sub(first) > 62*24*time.Hour {
		return nil, fmt.Errorf("models: calendar range %s..%s is invalid", start, end)
	}

	// Scheduled days and their status, one week plan per Monday.
	planned := map[string]PlannedDay{}
	monday := first.AddDate(0, 0, -(isoWeekday(first) - 1))
	for wk := monday; !wk.After(last); wk = wk.AddDate(0, 0, 7) {
		plan, err := WeekPlanForAthlete(ctx, db, athleteID, wk.Format("2006-01-02"), today, loc)
		if err != nil {
			return nil, err
		}
		for _, d := range plan.Days {
			planned[d.Date] = d
		}
	}

	sessions, err := calendarSessions(ctx, db, athleteID, start, end)
	if err != nil {
		return nil, err
	}

	out := &AthleteCalendarData{Start: start, End: end}
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		date := d.Format("2006-01-02")
		day := CalendarDay{Date: date, Sessions: sessions[date]}
		if day.Sessions == nil {
			day.Sessions = []CalendarSession{}
		}
		if p, ok := planned[date]; ok {
			day.Scheduled = true
			day.ProgramID = p.ProgramID
			day.ProgramName = p.ProgramName
			day.Status = p.Status
		}
		out.Days = append(out.Days, day)
	}

	phases, err := ListSeasonPhases(ctx, db, athleteID)
	if err != nil {
		return nil, err
	}
	for _, p := range phases {
		pStart := isoDate(p.StartDate)
		pEnd := ""
		if p.EndDate.Valid {
			pEnd = isoDate(p.EndDate.String)
		}
		if pStart <= end && (pEnd == "" || pEnd >= start) {
			out.Phases = append(out.Phases, p)
		}
	}
	return out, nil
}

func calendarSessions(ctx context.Context, db *sql.DB, athleteID int64, start, end string) (map[string][]CalendarSession, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT w.id, date(w.date), w.discipline,
		       COALESCE(
		           (SELECT t.throw_type FROM throwing_sessions t WHERE t.workout_id = w.id LIMIT 1),
		           (SELECT c.modality FROM conditioning_sessions c WHERE c.workout_id = w.id LIMIT 1),
		           (SELECT s.skill_type FROM skill_sessions s WHERE s.workout_id = w.id LIMIT 1),
		           ''),
		       (SELECT COUNT(*) FROM workout_sets ws WHERE ws.workout_id = w.id AND `+countedSet+`)
		  FROM workouts w
		 WHERE w.athlete_id = ?
		   AND date(w.date) BETWEEN ? AND ?
		 ORDER BY date(w.date), w.id`, athleteID, start, end)
	if err != nil {
		return nil, fmt.Errorf("models: calendar sessions for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()
	out := map[string][]CalendarSession{}
	for rows.Next() {
		var s CalendarSession
		var date string
		if err := rows.Scan(&s.WorkoutID, &date, &s.Discipline, &s.Label, &s.SetCount); err != nil {
			return nil, fmt.Errorf("models: scan calendar session: %w", err)
		}
		out[date] = append(out[date], s)
	}
	return out, rows.Err()
}
