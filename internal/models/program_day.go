package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Where a session's program week/day came from (ADR 028).
const (
	// DaySourceStored: the session row carries program_week/program_day.
	DaySourceStored = "stored"
	// DaySourceNext: no stored day on this date; the day after the latest
	// earlier linked session that has one.
	DaySourceNext = "next_after_stored"
	// DaySourceCount: no stored day to build on; counted from the linked
	// sessions before this date (the behaviour before ADR 028).
	DaySourceCount = "count"
)

// ProgramDayError is an invalid program week/day. Field is program_week or
// program_day.
type ProgramDayError struct {
	Field   string
	Message string
}

func (e *ProgramDayError) Error() string { return e.Field + " " + e.Message }

// programSlot is the 0-based position in one cycle a date holds on an
// assignment, and how it was found.
type programSlot struct {
	Position      int  // 0-based index within one cycle
	Completed     int  // linked resistance sessions dated before the date
	HasWorkout    bool // a linked session exists on the date
	CycleComplete bool // a non-loop program has finished its cycle
	Source        string
}

// slotFor finds the program position date holds on program. A linked
// session on the date with a stored day wins; otherwise the day after the
// latest earlier linked session with a stored day; otherwise the count of
// linked sessions before the date.
func slotFor(ctx context.Context, db *sql.DB, program *AthleteProgram, date string) (programSlot, error) {
	var slot programSlot
	cycle := program.NumWeeks * program.NumDays
	if cycle == 0 {
		return slot, fmt.Errorf("models: program has zero cycle length")
	}
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workouts WHERE assignment_id = ? AND date(date) < date(?)`,
		program.ID, date,
	).Scan(&slot.Completed); err != nil {
		return slot, fmt.Errorf("models: count workouts for prescription: %w", err)
	}

	var todayID sql.NullInt64
	var todayWeek, todayDay sql.NullInt64
	err := db.QueryRowContext(ctx,
		`SELECT id, program_week, program_day FROM workouts
		  WHERE assignment_id = ? AND date(date) = date(?)
		  ORDER BY (discipline = 'resistance') DESC, id LIMIT 1`,
		program.ID, date,
	).Scan(&todayID, &todayWeek, &todayDay)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return slot, fmt.Errorf("models: check today workout: %w", err)
	}
	slot.HasWorkout = todayID.Valid
	if pos, ok := storedPosition(program, todayWeek, todayDay); ok {
		slot.Position, slot.Source = pos, DaySourceStored
		return slot, nil
	}

	var lastWeek, lastDay sql.NullInt64
	err = db.QueryRowContext(ctx,
		`SELECT program_week, program_day FROM workouts
		  WHERE assignment_id = ? AND date(date) < date(?)
		  ORDER BY date(date) DESC, id DESC LIMIT 1`,
		program.ID, date,
	).Scan(&lastWeek, &lastDay)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return slot, fmt.Errorf("models: latest linked workout: %w", err)
	}
	if pos, ok := storedPosition(program, lastWeek, lastDay); ok {
		next := pos + 1
		if next >= cycle {
			next = 0
			slot.CycleComplete = !program.IsLoop
		}
		slot.Position, slot.Source = next, DaySourceNext
		return slot, nil
	}

	slot.Position = slot.Completed % cycle
	slot.CycleComplete = !program.IsLoop && slot.Completed > 0 && slot.Position == 0
	slot.Source = DaySourceCount
	return slot, nil
}

// storedPosition is the 0-based cycle position of a stored week/day, when
// both are set and still inside the template.
func storedPosition(program *AthleteProgram, week, day sql.NullInt64) (int, bool) {
	if !week.Valid || !day.Valid {
		return 0, false
	}
	w, d := int(week.Int64), int(day.Int64)
	if w < 1 || d < 1 || w > program.NumWeeks || d > program.NumDays {
		return 0, false
	}
	return (w-1)*program.NumDays + (d - 1), true
}

// NextProgramDay is the week/day a new linked session on date would count
// as when the caller does not choose one.
func NextProgramDay(ctx context.Context, db *sql.DB, program *AthleteProgram, date string) (week, day int, source string, err error) {
	slot, err := slotFor(ctx, db, program, date)
	if err != nil {
		return 0, 0, "", err
	}
	return slot.Position/program.NumDays + 1, slot.Position%program.NumDays + 1, slot.Source, nil
}

// TemplateWeekFor is the template week whose prescribed sets a program week
// uses. A template that stores only week 1 (an imported weekly cycle) repeats
// that week, so week 3 of an 8-week weekly cycle reads week 1's days.
func TemplateWeekFor(ctx context.Context, q queryer, templateID int64, week int) (int, error) {
	var hasWeek bool
	var maxWeek sql.NullInt64
	if err := q.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(week = ?), 0), MAX(week) FROM prescribed_sets WHERE template_id = ?`,
		week, templateID,
	).Scan(&hasWeek, &maxWeek); err != nil {
		return 0, fmt.Errorf("models: template %d weeks: %w", templateID, err)
	}
	if !hasWeek && maxWeek.Valid && maxWeek.Int64 == 1 {
		return 1, nil
	}
	return week, nil
}

// ValidateProgramDay checks week/day are inside program's template and that
// the template has prescribed sets for that day (after TemplateWeekFor).
func ValidateProgramDay(ctx context.Context, db *sql.DB, program *AthleteProgram, version, week, day int) error {
	if week < 1 || week > program.NumWeeks {
		return &ProgramDayError{"program_week", fmt.Sprintf("must be 1 to %d for template %d", program.NumWeeks, program.TemplateID)}
	}
	if day < 1 || day > program.NumDays {
		return &ProgramDayError{"program_day", fmt.Sprintf("must be 1 to %d for template %d", program.NumDays, program.TemplateID)}
	}
	tw, err := TemplateWeekFor(ctx, db, program.TemplateID, week)
	if err != nil {
		return err
	}
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM prescribed_sets WHERE template_id = ? AND week = ? AND day = ?`,
		program.TemplateID, tw, day,
	).Scan(&n); err != nil {
		return fmt.Errorf("models: template day: %w", err)
	}
	if n == 0 {
		return &ProgramDayError{"program_day", fmt.Sprintf("does not exist: template %d version %d has no Week %d Day %d", program.TemplateID, version, week, day)}
	}
	return nil
}

// CalendarProgramWeek is the program week date falls in, counting Monday to
// Sunday weeks from the week the assignment starts in (a Thursday start
// makes that Monday–Sunday week 1). A loop wraps past the last week; a
// non-loop past its last week returns 0.
func CalendarProgramWeek(startDate, date string, numWeeks int, loop bool) int {
	start, err1 := time.Parse("2006-01-02", dayOnly(startDate))
	d, err2 := time.Parse("2006-01-02", date)
	if err1 != nil || err2 != nil || numWeeks < 1 || d.Before(start) {
		return 0
	}
	monday := start.AddDate(0, 0, -(isoWeekday(start) - 1))
	week := int(d.Sub(monday).Hours()/24)/7 + 1
	if week > numWeeks {
		if !loop {
			return 0
		}
		week = (week-1)%numWeeks + 1
	}
	return week
}

// ProgramDayLabelFor is the "Week N Day D" label for a week/day.
func ProgramDayLabelFor(week, day int) string {
	if week < 1 || day < 1 {
		return ""
	}
	return fmt.Sprintf("Week %d Day %d", week, day)
}
