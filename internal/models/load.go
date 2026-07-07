package models

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// LoadSummary is the read-only, per-discipline training-load view for an
// athlete (ADR 018 / HOF-009). It reports rolling acute (7-day) and chronic
// (28-day) load totals plus an acute:chronic workload ratio (ACWR) for each
// discipline that contributes training load.
//
// This view is ADVISORY ONLY. It is a pure read computed from logged sessions —
// it never gates a log, never auto-actions, and writes nothing (ADR 007/018).
// Each discipline keeps its own native unit; loads are NEVER summed across
// disciplines into a single blended number. Recovery is deliberately excluded
// (it is a recovery signal, not training load), as is wearable sleep.
type LoadSummary struct {
	AthleteID   int64             `json:"athlete_id"`
	AsOf        string            `json:"as_of"` // YYYY-MM-DD window anchor (today)
	Disciplines []*DisciplineLoad `json:"disciplines"`
}

// DisciplineLoad is the load rollup for one discipline.
//
// ACWR is the COUPLED acute:chronic workload ratio = acute7 / (chronic28 / 4),
// where the 7-day acute window is itself part of the 28-day chronic window.
// ACWR is suppressed (nil, Marker "insufficient_history") until the discipline
// has at least ~28 days of logged history — otherwise the chronic baseline is
// just the acute window scaled up and the ratio pins to a false ~4.0. The
// unit/proxy is documented per discipline:
//   - resistance:   sum of reps * weight (volume-load)
//   - throwing:     sum of throw_count (throws)
//   - conditioning: sum of duration_seconds (seconds)
//   - skill:        sum of rep_count (reps)
type DisciplineLoad struct {
	Discipline string   `json:"discipline"`
	Unit       string   `json:"unit"`
	Acute7     float64  `json:"acute_7d"`
	Chronic28  float64  `json:"chronic_28d"`
	ACWR       *float64 `json:"acwr"`             // nil when chronic history is insufficient
	Marker     string   `json:"marker,omitempty"` // "insufficient_history" when ACWR is nil
}

// GetLoadSummary computes the per-discipline rolling load view for an athlete.
// It performs SELECTs only — no INSERT/UPDATE/DELETE — and returns a fixed set
// of load-bearing disciplines (resistance, throwing, conditioning, skill) even
// when their totals are zero, so the coach sees the full picture.
func GetLoadSummary(ctx context.Context, db *sql.DB, athleteID int64) (*LoadSummary, error) {
	asOf := time.Now().Format("2006-01-02")
	// Window anchors: acute = last 7 days inclusive, chronic = last 28 days.
	acuteStart := time.Now().AddDate(0, 0, -6).Format("2006-01-02")
	chronicStart := time.Now().AddDate(0, 0, -27).Format("2006-01-02")

	summary := &LoadSummary{AthleteID: athleteID, AsOf: asOf}

	// resistance: volume-load = sum(reps * weight) over completed sets.
	resAcute, err := scalarLoad(ctx, db,
		`SELECT COALESCE(SUM(ws.reps * ws.weight), 0)
		 FROM workout_sets ws
		 JOIN workouts w ON w.id = ws.workout_id
		 WHERE w.athlete_id = ? AND w.discipline = 'resistance' AND w.date >= ?`,
		athleteID, acuteStart)
	if err != nil {
		return nil, err
	}
	resChronic, err := scalarLoad(ctx, db,
		`SELECT COALESCE(SUM(ws.reps * ws.weight), 0)
		 FROM workout_sets ws
		 JOIN workouts w ON w.id = ws.workout_id
		 WHERE w.athlete_id = ? AND w.discipline = 'resistance' AND w.date >= ?`,
		athleteID, chronicStart)
	if err != nil {
		return nil, err
	}
	resEarliest, err := earliestLoadDate(ctx, db,
		`SELECT MIN(w.date)
		 FROM workout_sets ws
		 JOIN workouts w ON w.id = ws.workout_id
		 WHERE w.athlete_id = ? AND w.discipline = 'resistance'`,
		athleteID)
	if err != nil {
		return nil, err
	}
	summary.Disciplines = append(summary.Disciplines, newDisciplineLoad("resistance", "volume_load", resAcute, resChronic, resEarliest, chronicStart))

	// throwing: total throws = sum(throw_count).
	throwAcute, err := scalarLoad(ctx, db,
		`SELECT COALESCE(SUM(ts.throw_count), 0)
		 FROM throwing_sessions ts
		 JOIN workouts w ON w.id = ts.workout_id
		 WHERE w.athlete_id = ? AND w.date >= ?`,
		athleteID, acuteStart)
	if err != nil {
		return nil, err
	}
	throwChronic, err := scalarLoad(ctx, db,
		`SELECT COALESCE(SUM(ts.throw_count), 0)
		 FROM throwing_sessions ts
		 JOIN workouts w ON w.id = ts.workout_id
		 WHERE w.athlete_id = ? AND w.date >= ?`,
		athleteID, chronicStart)
	if err != nil {
		return nil, err
	}
	throwEarliest, err := earliestLoadDate(ctx, db,
		`SELECT MIN(w.date)
		 FROM throwing_sessions ts
		 JOIN workouts w ON w.id = ts.workout_id
		 WHERE w.athlete_id = ?`,
		athleteID)
	if err != nil {
		return nil, err
	}
	summary.Disciplines = append(summary.Disciplines, newDisciplineLoad("throwing", "throws", throwAcute, throwChronic, throwEarliest, chronicStart))

	// conditioning: total work seconds = sum(duration_seconds).
	condAcute, err := scalarLoad(ctx, db,
		`SELECT COALESCE(SUM(cs.duration_seconds), 0)
		 FROM conditioning_sessions cs
		 JOIN workouts w ON w.id = cs.workout_id
		 WHERE w.athlete_id = ? AND w.date >= ?`,
		athleteID, acuteStart)
	if err != nil {
		return nil, err
	}
	condChronic, err := scalarLoad(ctx, db,
		`SELECT COALESCE(SUM(cs.duration_seconds), 0)
		 FROM conditioning_sessions cs
		 JOIN workouts w ON w.id = cs.workout_id
		 WHERE w.athlete_id = ? AND w.date >= ?`,
		athleteID, chronicStart)
	if err != nil {
		return nil, err
	}
	condEarliest, err := earliestLoadDate(ctx, db,
		`SELECT MIN(w.date)
		 FROM conditioning_sessions cs
		 JOIN workouts w ON w.id = cs.workout_id
		 WHERE w.athlete_id = ?`,
		athleteID)
	if err != nil {
		return nil, err
	}
	summary.Disciplines = append(summary.Disciplines, newDisciplineLoad("conditioning", "seconds", condAcute, condChronic, condEarliest, chronicStart))

	// skill: total reps = sum(rep_count).
	skillAcute, err := scalarLoad(ctx, db,
		`SELECT COALESCE(SUM(ss.rep_count), 0)
		 FROM skill_sessions ss
		 JOIN workouts w ON w.id = ss.workout_id
		 WHERE w.athlete_id = ? AND w.date >= ?`,
		athleteID, acuteStart)
	if err != nil {
		return nil, err
	}
	skillChronic, err := scalarLoad(ctx, db,
		`SELECT COALESCE(SUM(ss.rep_count), 0)
		 FROM skill_sessions ss
		 JOIN workouts w ON w.id = ss.workout_id
		 WHERE w.athlete_id = ? AND w.date >= ?`,
		athleteID, chronicStart)
	if err != nil {
		return nil, err
	}
	skillEarliest, err := earliestLoadDate(ctx, db,
		`SELECT MIN(w.date)
		 FROM skill_sessions ss
		 JOIN workouts w ON w.id = ss.workout_id
		 WHERE w.athlete_id = ?`,
		athleteID)
	if err != nil {
		return nil, err
	}
	summary.Disciplines = append(summary.Disciplines, newDisciplineLoad("skill", "reps", skillAcute, skillChronic, skillEarliest, chronicStart))

	return summary, nil
}

// scalarLoad runs a single-value COALESCE(SUM(...),0) load query.
func scalarLoad(ctx context.Context, db *sql.DB, query string, args ...any) (float64, error) {
	var v float64
	if err := db.QueryRowContext(ctx, query, args...).Scan(&v); err != nil {
		return 0, fmt.Errorf("models: compute load: %w", err)
	}
	return v, nil
}

// earliestLoadDate returns the earliest logged date (YYYY-MM-DD) contributing
// load for a discipline, or "" when the athlete has no logged sessions for it.
func earliestLoadDate(ctx context.Context, db *sql.DB, query string, args ...any) (string, error) {
	var d sql.NullString
	if err := db.QueryRowContext(ctx, query, args...).Scan(&d); err != nil {
		return "", fmt.Errorf("models: earliest load date: %w", err)
	}
	if !d.Valid {
		return "", nil
	}
	return d.String, nil
}

// newDisciplineLoad builds a DisciplineLoad, computing the coupled ACWR =
// acute7 / (chronic28 / 4). The ratio is only meaningful once a real chronic
// baseline exists, so it is suppressed (ACWR nil, Marker "insufficient_history")
// when there is no chronic load (chronic28 == 0) OR when the discipline's logged
// history does not yet span the full ~28-day chronic window — i.e. the earliest
// logged date is after chronicStart. Without that guard, a single week of data
// makes chronic28 ≈ acute7 and the ratio pins to a false ~4.0.
func newDisciplineLoad(discipline, unit string, acute7, chronic28 float64, earliest, chronicStart string) *DisciplineLoad {
	dl := &DisciplineLoad{
		Discipline: discipline,
		Unit:       unit,
		Acute7:     acute7,
		Chronic28:  chronic28,
	}
	fullChronicWindow := earliest != "" && earliest <= chronicStart
	if chronic28 == 0 || !fullChronicWindow {
		dl.Marker = "insufficient_history"
		return dl
	}
	ratio := acute7 / (chronic28 / 4)
	dl.ACWR = &ratio
	return dl
}
