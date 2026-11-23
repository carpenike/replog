package models

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

// RecordRepTargets are the rep-max columns on the records board. A rep
// max for N is the heaviest load lifted for at least N reps, so a 6-rep
// set at a new high also counts as a 5RM.
var RecordRepTargets = []int{1, 2, 3, 5, 8, 10, 12}

// RecordE1RMMaxReps caps which sets feed the estimated 1RM. Estimates from
// very high-rep sets are unreliable; the frontend Bests card uses the same cap.
const RecordE1RMMaxReps = 12

// RecordSet is one logged set that holds a record.
type RecordSet struct {
	WorkoutID int64
	Date      string
	Reps      int
	Weight    float64 // 0 for a bodyweight set
}

// RepMaxRecord is the heaviest load lifted for at least Target reps.
type RepMaxRecord struct {
	Target int
	RecordSet
}

// E1RMRecord is the best estimated 1RM and the set it came from.
type E1RMRecord struct {
	E1RM float64
	RecordSet
}

// ExerciseRecords is one exercise's current personal records.
type ExerciseRecords struct {
	ExerciseID   int64
	Name         string
	SessionCount int
	LastDate     string
	Heaviest     *RecordSet
	BestE1RM     *E1RMRecord
	MostReps     *RecordSet
	RepMaxes     []RepMaxRecord
}

type recordRow struct {
	exerciseID int64
	name       string
	set        RecordSet
}

// ListAthleteRecords returns the athlete's current personal records for
// every resistance exercise they have logged. Records compare only this
// athlete's own sets. Warm-up sets are excluded, as are timed and distance
// sets (their reps column is not a rep count). Ties keep the earliest date,
// the day the record was first set. Loads are the logged unit (pounds).
func ListAthleteRecords(ctx context.Context, db *sql.DB, athleteID int64) ([]ExerciseRecords, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT e.id, e.name, w.id, w.date, ws.reps, COALESCE(ws.weight, 0)
		  FROM workout_sets ws
		  JOIN workouts w ON w.id = ws.workout_id
		  JOIN exercises e ON e.id = ws.exercise_id
		 WHERE w.athlete_id = ?
		   AND w.discipline = 'resistance'
		   AND `+countedSet+`
		   AND ws.rep_type IN ('reps', 'each_side')
		   AND ws.reps > 0
		 ORDER BY e.name COLLATE NOCASE, e.id, date(w.date), w.id, ws.id`,
		athleteID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: records for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var flat []recordRow
	for rows.Next() {
		var r recordRow
		if err := rows.Scan(&r.exerciseID, &r.name, &r.set.WorkoutID, &r.set.Date, &r.set.Reps, &r.set.Weight); err != nil {
			return nil, fmt.Errorf("models: scan record set: %w", err)
		}
		r.set.Date = isoDate(r.set.Date)
		flat = append(flat, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []ExerciseRecords
	var cur *ExerciseRecords
	sessions := map[int64]struct{}{}
	for _, r := range flat {
		if cur == nil || cur.ExerciseID != r.exerciseID {
			if cur != nil {
				out = append(out, *cur)
			}
			cur = &ExerciseRecords{ExerciseID: r.exerciseID, Name: r.name, RepMaxes: []RepMaxRecord{}}
			sessions = map[int64]struct{}{}
		}
		if _, seen := sessions[r.set.WorkoutID]; !seen {
			sessions[r.set.WorkoutID] = struct{}{}
			cur.SessionCount++
		}
		cur.LastDate = r.set.Date
		applyRecordSet(cur, r.set)
	}
	if cur != nil {
		out = append(out, *cur)
	}
	return out, nil
}

// applyRecordSet folds one set into an exercise's records. Sets arrive in
// date order, so a strict "greater than" keeps the earliest tie.
func applyRecordSet(rec *ExerciseRecords, s RecordSet) {
	if rec.MostReps == nil || s.Reps > rec.MostReps.Reps {
		cp := s
		rec.MostReps = &cp
	}
	if s.Weight <= 0 {
		return
	}
	if rec.Heaviest == nil || s.Weight > rec.Heaviest.Weight {
		cp := s
		rec.Heaviest = &cp
	}
	if s.Reps <= RecordE1RMMaxReps {
		if e1rm, ok := Epley1RM(s.Weight, s.Reps); ok && (rec.BestE1RM == nil || e1rm > rec.BestE1RM.E1RM) {
			rec.BestE1RM = &E1RMRecord{E1RM: e1rm, RecordSet: s}
		}
	}
	for _, target := range RecordRepTargets {
		if s.Reps < target {
			break
		}
		idx := -1
		for i := range rec.RepMaxes {
			if rec.RepMaxes[i].Target == target {
				idx = i
				break
			}
		}
		if idx < 0 {
			rec.RepMaxes = append(rec.RepMaxes, RepMaxRecord{Target: target, RecordSet: s})
			continue
		}
		if s.Weight > rec.RepMaxes[idx].Weight {
			rec.RepMaxes[idx].RecordSet = s
		}
	}
	sort.Slice(rec.RepMaxes, func(i, j int) bool { return rec.RepMaxes[i].Target < rec.RepMaxes[j].Target })
}
