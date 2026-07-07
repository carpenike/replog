package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrNoPitchSmartLimit is returned when no Pitch Smart bracket covers an
// athlete's age (e.g. age unknown, or outside the 7–18 reference range).
var ErrNoPitchSmartLimit = errors.New("no pitch smart limit for age")

// pitchSmartThreshold is one {pitches → rest days} row inside a bracket's
// rest_thresholds JSON, ordered ascending by Max.
type pitchSmartThreshold struct {
	Max  int `json:"max"`
	Rest int `json:"rest"`
}

// PitchSmartLimit is one age-bracket row of the seeded reference table.
type PitchSmartLimit struct {
	AgeMin     int
	AgeMax     int
	DailyMax   int
	thresholds []pitchSmartThreshold
}

// restDaysFor returns the rest days owed after throwing `pitches` in one day.
func (l *PitchSmartLimit) restDaysFor(pitches int) int {
	rest := 0
	for _, t := range l.thresholds {
		if pitches <= t.Max {
			return t.Rest
		}
		rest = t.Rest
	}
	return rest
}

// PitchSmartStatus is the coach-facing advisory computed for an athlete. It is
// READ-ONLY guidance (ADR 007/018): the app surfaces it for a coach to weigh —
// it never writes progression, never auto-rests an athlete, and never blocks a
// throwing session from being logged.
type PitchSmartStatus struct {
	AgeBracket       string // e.g. "13–14"
	DailyMax         int    // recommended max pitches in a single day
	LastSessionDate  string // date of the most recent counted pitching session (game/bullpen)
	LastThrowCount   int    // pitch count of that session
	OverDailyMax     bool   // last session exceeded DailyMax
	RestDaysRequired int    // rest days the last session's volume calls for
	RestDaysOwed     int    // rest days still owed as of `asOf` (0 if rested)
	NextEligibleDate string // earliest recommended next-throw date ("" if rested)
	Advisory         string // short human-readable summary for the coach
}

// GetPitchSmartLimitForAge returns the seeded Pitch Smart bracket covering the
// given age, or ErrNoPitchSmartLimit if none applies.
func GetPitchSmartLimitForAge(ctx context.Context, db *sql.DB, age int) (*PitchSmartLimit, error) {
	l := &PitchSmartLimit{}
	var thresholdsJSON string
	err := db.QueryRowContext(ctx,
		`SELECT age_min, age_max, daily_max, rest_thresholds
		 FROM pitch_smart_limits WHERE ? BETWEEN age_min AND age_max
		 ORDER BY age_min LIMIT 1`, age,
	).Scan(&l.AgeMin, &l.AgeMax, &l.DailyMax, &thresholdsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoPitchSmartLimit
	}
	if err != nil {
		return nil, fmt.Errorf("models: get pitch smart limit for age %d: %w", age, err)
	}
	if err := json.Unmarshal([]byte(thresholdsJSON), &l.thresholds); err != nil {
		return nil, fmt.Errorf("models: parse pitch smart thresholds: %w", err)
	}
	return l, nil
}

// ageFromDOB computes whole-year age at `asOf` from a YYYY-MM-DD birth date.
func ageFromDOB(dob string, asOf time.Time) (int, bool) {
	bd, err := time.Parse("2006-01-02", normalizeDate(dob))
	if err != nil {
		return 0, false
	}
	age := asOf.Year() - bd.Year()
	if asOf.YearDay() < bd.YearDay() {
		age--
	}
	return age, true
}

// ComputePitchSmartStatus builds the read-only Pitch Smart advisory for an
// athlete as of `asOf`. It uses the athlete's date of birth to pick a bracket
// and their most recent counted throwing session to compute rest days owed.
// Returns ErrNoPitchSmartLimit when the athlete's age can't be determined or
// no bracket applies — callers treat that as "no advisory", never an error
// that blocks logging.
func ComputePitchSmartStatus(ctx context.Context, db *sql.DB, athleteID int64, asOf time.Time) (*PitchSmartStatus, error) {
	a, err := GetAthleteByID(ctx, db, athleteID)
	if err != nil {
		return nil, err
	}
	if !a.DateOfBirth.Valid {
		return nil, ErrNoPitchSmartLimit
	}
	age, ok := ageFromDOB(a.DateOfBirth.String, asOf)
	if !ok {
		return nil, ErrNoPitchSmartLimit
	}

	limit, err := GetPitchSmartLimitForAge(ctx, db, age)
	if err != nil {
		return nil, err
	}

	status := &PitchSmartStatus{
		AgeBracket: fmt.Sprintf("%d–%d", limit.AgeMin, limit.AgeMax),
		DailyMax:   limit.DailyMax,
	}

	// Most recent PITCHING session that carried a throw count. Pitch Smart's
	// pitch-count rest math counts mound pitching only ('game','bullpen') — not
	// catch-play, long toss, infield/position throws, or (ambiguous) lessons.
	// Those still count toward the cross-modal load view (GetLoadSummary sums
	// throw_count across ALL throw types); they just don't drive the pitch-count
	// advisory. So LastSessionDate / LastThrowCount denote the latest pitching
	// session, which may be older than the athlete's latest throwing session.
	var lastDate string
	var lastCount sql.NullInt64
	err = db.QueryRowContext(ctx,
		`SELECT w.date, ts.throw_count
		 FROM throwing_sessions ts
		 JOIN workouts w ON w.id = ts.workout_id
		 WHERE w.athlete_id = ? AND ts.throw_count IS NOT NULL
		   AND ts.throw_type IN ('game','bullpen')
		 ORDER BY w.date DESC, ts.id DESC LIMIT 1`, athleteID,
	).Scan(&lastDate, &lastCount)
	if errors.Is(err, sql.ErrNoRows) {
		status.Advisory = fmt.Sprintf("No counted pitching sessions on record. Recommended daily max for age %s: %d pitches.", status.AgeBracket, status.DailyMax)
		return status, nil
	}
	if err != nil {
		return nil, fmt.Errorf("models: pitch smart last session for athlete %d: %w", athleteID, err)
	}

	status.LastSessionDate = normalizeDate(lastDate)
	status.LastThrowCount = int(lastCount.Int64)
	status.OverDailyMax = status.LastThrowCount > limit.DailyMax
	status.RestDaysRequired = limit.restDaysFor(status.LastThrowCount)

	if last, perr := time.Parse("2006-01-02", status.LastSessionDate); perr == nil && status.RestDaysRequired > 0 {
		nextEligible := last.AddDate(0, 0, status.RestDaysRequired)
		daysElapsed := int(asOf.Sub(last).Hours() / 24)
		owed := status.RestDaysRequired - daysElapsed
		if owed < 0 {
			owed = 0
		}
		status.RestDaysOwed = owed
		if owed > 0 {
			status.NextEligibleDate = nextEligible.Format("2006-01-02")
		}
	}

	switch {
	case status.RestDaysOwed > 0:
		status.Advisory = fmt.Sprintf("Last session threw %d pitches (%s), which calls for %d rest day(s). %d still owed — recommended next throw on or after %s.",
			status.LastThrowCount, status.LastSessionDate, status.RestDaysRequired, status.RestDaysOwed, status.NextEligibleDate)
	case status.OverDailyMax:
		status.Advisory = fmt.Sprintf("Last session (%d pitches on %s) exceeded the recommended daily max of %d for age %s. Rest requirement is met as of today.",
			status.LastThrowCount, status.LastSessionDate, limit.DailyMax, status.AgeBracket)
	default:
		status.Advisory = fmt.Sprintf("Rest requirement met. Recommended daily max for age %s: %d pitches.", status.AgeBracket, status.DailyMax)
	}

	return status, nil
}
