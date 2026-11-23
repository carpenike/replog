package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"
)

// DigestPR is a personal record first set during the digest week.
type DigestPR struct {
	Exercise string
	Date     string
	Weight   float64 // 0 for a bodyweight rep record
	Reps     int
	E1RM     float64 // set when the week's best e1RM is a record too
}

// DigestAthlete is one coached athlete's week for the coach digest. It is
// a summary for the coach to read; nothing in it changes a plan.
type DigestAthlete struct {
	AthleteID         int64
	Name              string
	Plan              *WeekPlan
	NewPRs            []DigestPR
	LowReadinessDates []string
	// PitchSmart is set only when the athlete pitched this week or still
	// owes rest days. It is advisory, as everywhere else.
	PitchSmart *PitchSmartStatus
}

// BuildDigestAthlete summarizes one athlete's week starting weekStart (a
// Monday). today is the coach's local date, so a finished week has no
// "upcoming" days.
func BuildDigestAthlete(ctx context.Context, db *sql.DB, athlete *Athlete, weekStart, today string, loc *time.Location) (*DigestAthlete, error) {
	plan, err := WeekPlanForAthlete(ctx, db, athlete.ID, weekStart, today, loc)
	if err != nil {
		return nil, err
	}
	out := &DigestAthlete{AthleteID: athlete.ID, Name: athlete.Name, Plan: plan}

	if out.NewPRs, err = weekPRs(ctx, db, athlete.ID, plan.WeekStart, plan.WeekEnd); err != nil {
		return nil, err
	}

	checkins, err := ListRecoveryCheckins(ctx, db, athlete.ID, 31)
	if err != nil {
		return nil, err
	}
	for _, c := range checkins {
		d := isoDate(c.Date)
		if d >= plan.WeekStart && d <= plan.WeekEnd && c.ReadinessLow() {
			out.LowReadinessDates = append(out.LowReadinessDates, d)
		}
	}
	sort.Strings(out.LowReadinessDates)

	end, _ := time.ParseInLocation("2006-01-02", plan.WeekEnd, loc)
	asOf := time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 0, 0, loc)
	ps, err := ComputePitchSmartStatus(ctx, db, athlete.ID, asOf)
	switch {
	case errors.Is(err, ErrNoPitchSmartLimit), errors.Is(err, ErrNotFound):
	case err != nil:
		return nil, err
	case ps.RestDaysOwed > 0 || (ps.LastSessionDate >= plan.WeekStart && ps.LastSessionDate <= plan.WeekEnd):
		out.PitchSmart = ps
	}
	return out, nil
}

// weekPRs returns the records whose first date falls in [start, end],
// skipping exercises first logged this week (a first session is a first,
// not a PR). Records compare only this athlete's own history.
func weekPRs(ctx context.Context, db *sql.DB, athleteID int64, start, end string) ([]DigestPR, error) {
	records, err := ListAthleteRecords(ctx, db, athleteID)
	if err != nil {
		return nil, err
	}
	firsts, err := firstLoggedDates(ctx, db, athleteID)
	if err != nil {
		return nil, err
	}
	in := func(d string) bool { return d >= start && d <= end }
	var out []DigestPR
	for _, rec := range records {
		if first, ok := firsts[rec.ExerciseID]; !ok || first >= start {
			continue
		}
		var pr *DigestPR
		switch {
		case rec.Heaviest != nil && in(rec.Heaviest.Date):
			pr = &DigestPR{Exercise: rec.Name, Date: rec.Heaviest.Date, Weight: rec.Heaviest.Weight, Reps: rec.Heaviest.Reps}
		case rec.BestE1RM != nil && in(rec.BestE1RM.Date):
			pr = &DigestPR{Exercise: rec.Name, Date: rec.BestE1RM.Date, Weight: rec.BestE1RM.Weight, Reps: rec.BestE1RM.Reps}
		case rec.Heaviest == nil && rec.MostReps != nil && in(rec.MostReps.Date):
			pr = &DigestPR{Exercise: rec.Name, Date: rec.MostReps.Date, Reps: rec.MostReps.Reps}
		}
		if pr == nil {
			continue
		}
		if rec.BestE1RM != nil && in(rec.BestE1RM.Date) {
			pr.E1RM = rec.BestE1RM.E1RM
		}
		out = append(out, *pr)
	}
	return out, nil
}

func firstLoggedDates(ctx context.Context, db *sql.DB, athleteID int64) (map[int64]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT ws.exercise_id, MIN(date(w.date))
		  FROM workout_sets ws
		  JOIN workouts w ON w.id = ws.workout_id
		 WHERE w.athlete_id = ? AND `+countedSet+`
		 GROUP BY ws.exercise_id`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: first logged dates for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var d string
		if err := rows.Scan(&id, &d); err != nil {
			return nil, fmt.Errorf("models: scan first logged date: %w", err)
		}
		out[id] = d
	}
	return out, rows.Err()
}

// ListDigestCoaches returns users who coach at least one athlete.
func ListDigestCoaches(ctx context.Context, db *sql.DB) ([]*User, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT u.id FROM users u
		  JOIN athletes a ON a.coach_id = u.id
		 WHERE u.is_coach = 1 OR u.is_admin = 1
		 ORDER BY u.id`)
	if err != nil {
		return nil, fmt.Errorf("models: list digest coaches: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("models: scan digest coach: %w", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]*User, 0, len(ids))
	for _, id := range ids {
		u, err := GetUserByID(ctx, db, id)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

// ClaimWeeklyDigest records that userID's digest for weekStart is being
// sent. It returns false when that week was already claimed, so a digest
// goes out at most once per coach per week.
func ClaimWeeklyDigest(ctx context.Context, db *sql.DB, userID int64, weekStart string) (bool, error) {
	res, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO weekly_digest_log (user_id, week_start) VALUES (?, ?)`, userID, weekStart)
	if err != nil {
		return false, fmt.Errorf("models: claim weekly digest: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// WeeklyDigestSent reports whether userID's digest for weekStart went out.
func WeeklyDigestSent(ctx context.Context, db *sql.DB, userID int64, weekStart string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM weekly_digest_log WHERE user_id = ? AND week_start = ?`, userID, weekStart).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("models: weekly digest sent: %w", err)
	}
	return n > 0, nil
}
