package database

import (
	"database/sql"
	"testing"

	"github.com/pressly/goose/v3"
)

// TestSetDetailBackfill checks that migration 0021 re-derives rep range,
// RIR, warm-up, and group keys for prescribed sets an earlier import
// expanded, and leaves hand-built sets alone.
func TestSetDetailBackfill(t *testing.T) {
	db := openMigratedTo(t, 20)
	mustExec(t, db, `INSERT INTO athletes (id, name) VALUES (1, 'Ryan')`)
	mustExec(t, db, `INSERT INTO exercises (id, name) VALUES (1, 'Mobility Flow'), (2, 'Back Squat'), (3, 'Lateral Raise'), (4, 'Hammer Curl'), (5, 'Calf Raise')`)
	mustExec(t, db, `INSERT INTO program_templates (id, athlete_id, name, num_weeks, num_days, is_loop, version) VALUES (10, 1, 'Imported', 1, 1, 1, 1)`)
	mustExec(t, db, `INSERT INTO program_templates (id, athlete_id, name, num_weeks, num_days, is_loop, version) VALUES (11, NULL, 'Hand Built', 1, 1, 1, 1)`)
	mustExec(t, db, `INSERT INTO program_imports (template_id, source_name, author) VALUES (10, 'src', 'Phil')`)
	mustExec(t, db, `INSERT INTO program_days (id, template_id, week, day_number, day_order, label, exercise_count, equipment_json) VALUES (100, 10, 1, 1, 1, 'Day 1', 5, '[]')`)
	mustExec(t, db, `INSERT INTO program_blocks (id, day_id, block_order, block_type, rounds, non_working) VALUES
	    (200, 100, 1, 'circuit', 2, 1),
	    (201, 100, 2, 'straight', NULL, 0),
	    (202, 100, 3, 'superset', NULL, 0),
	    (203, 100, 4, 'superset', NULL, 0)`)
	mustExec(t, db, `INSERT INTO program_block_exercises (block_id, exercise_id, sort_order, sets, reps_min, reps_max, rir_min, rir_max, non_working) VALUES
	    (200, 1, 1, 2, 10, 10, NULL, NULL, 1),
	    (201, 2, 1, 3, 5, 8, 1, 2, 0),
	    (202, 3, 1, 2, 8, 15, 1, 2, 0),
	    (202, 4, 2, 2, 8, 15, 1, 2, 0),
	    (203, 5, 1, 3, 8, 15, 1, 2, 0)`)
	// Old expansion: reps = reps_min, group_label = block type, nothing else.
	for _, row := range []struct {
		exercise int
		sets     int
		reps     int
		label    string
		sort     int
	}{{1, 2, 10, "circuit", 0}, {2, 3, 5, "straight", 10}, {3, 2, 8, "superset", 20}, {4, 2, 8, "superset", 30}, {5, 3, 8, "superset", 40}} {
		for n := 1; n <= row.sets; n++ {
			if _, err := db.Exec(`INSERT INTO prescribed_sets (template_id, exercise_id, week, day, set_number, reps, group_label, sort_order) VALUES (10, ?, 1, 1, ?, ?, ?, ?)`,
				row.exercise, n, row.reps, row.label, row.sort); err != nil {
				t.Fatal(err)
			}
		}
	}
	mustExec(t, db, `INSERT INTO prescribed_sets (template_id, exercise_id, week, day, set_number, reps) VALUES (11, 2, 1, 1, 1, 5)`)
	mustExec(t, db, `INSERT INTO workouts (id, athlete_id, date) VALUES (1, 1, '2026-01-01')`)
	mustExec(t, db, `INSERT INTO workout_sets (workout_id, exercise_id, set_number, reps) VALUES (1, 2, 1, 5)`)

	applyRest(t, db)

	type detail struct {
		repsMax, rirMin, rirMax sql.NullInt64
		nonWorking              int
		groupKey                sql.NullString
	}
	get := func(template, exercise int) detail {
		t.Helper()
		var d detail
		if err := db.QueryRow(`SELECT reps_max, rir_min, rir_max, non_working, group_key FROM prescribed_sets
		    WHERE template_id = ? AND exercise_id = ? AND set_number = 1`, template, exercise).
			Scan(&d.repsMax, &d.rirMin, &d.rirMax, &d.nonWorking, &d.groupKey); err != nil {
			t.Fatal(err)
		}
		return d
	}

	if d := get(10, 1); d.repsMax.Valid || d.nonWorking != 1 || d.groupKey.String != "A" {
		t.Fatalf("warm-up circuit = %+v", d)
	}
	if d := get(10, 2); d.repsMax.Int64 != 8 || d.rirMin.Int64 != 1 || d.rirMax.Int64 != 2 || d.nonWorking != 0 || d.groupKey.Valid {
		t.Fatalf("straight squat = %+v", d)
	}
	if d3, d4 := get(10, 3), get(10, 4); d3.groupKey.String != "B" || d4.groupKey.String != "B" || d3.repsMax.Int64 != 15 {
		t.Fatalf("first superset = %+v / %+v", d3, d4)
	}
	if d := get(10, 5); d.groupKey.String != "C" {
		t.Fatalf("second superset = %+v", d)
	}
	if d := get(11, 2); d.repsMax.Valid || d.rirMin.Valid || d.nonWorking != 0 || d.groupKey.Valid {
		t.Fatalf("hand-built set changed = %+v", d)
	}
	var setType string
	if err := db.QueryRow(`SELECT set_type FROM workout_sets`).Scan(&setType); err != nil || setType != "working" {
		t.Fatalf("existing set type = %q, %v", setType, err)
	}

	// Down removes the columns and the plate tables cleanly.
	if err := goose.DownTo(db, "migrations", 20); err != nil {
		t.Fatalf("down to 20: %v", err)
	}
	if _, err := db.Exec(`SELECT reps_max FROM prescribed_sets`); err == nil {
		t.Fatal("reps_max survived down")
	}
	if _, err := db.Exec(`SELECT 1 FROM plate_setups`); err == nil {
		t.Fatal("plate_setups survived down")
	}
}
