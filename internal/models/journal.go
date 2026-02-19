package models

import (
	"database/sql"
	"fmt"
)

// JournalEntry represents a single event on an athlete's timeline.
// Events are heterogeneous — the Type field identifies the source table
// and determines which detail fields are populated.
type JournalEntry struct {
	Date    string // YYYY-MM-DD
	Type    string // "workout", "body_weight", "training_max", "goal_change", "tier_change", "program_start", "review", "note"
	Summary string // Human-readable one-line summary
	ID      int64  // Source row ID (for linking)

	// Optional detail fields (populated per type).
	Detail    string // Secondary text (e.g., workout notes, exercise list)
	IsPrivate bool   // Only relevant for "note" type
	Pinned    bool   // Only relevant for "note" type
	SecondID  int64  // Secondary ID (e.g., workout_id for reviews)
	Author    string // Author/coach name for notes, reviews
	AuthorID  int64  // Author user ID (for edit permission checks on notes)
}

// ListJournalEntries returns a unified timeline of events for an athlete,
// newest first. If includePrivate is false, private notes are excluded
// (for non-coach view).
func ListJournalEntries(db *sql.DB, athleteID int64, includePrivate bool, limit int) ([]*JournalEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	privateFilter := ""
	if !includePrivate {
		privateFilter = " AND n.is_private = 0"
	}

	// Each UNION branch selects: date, type, summary, id, detail, is_private, pinned, second_id, author, author_id
	query := fmt.Sprintf(`
		SELECT date, type, summary, id, detail, is_private, pinned, second_id, author, author_id FROM (
			-- Workouts
			SELECT w.date AS date,
			       'workout' AS type,
			       COALESCE(
			           (SELECT GROUP_CONCAT(ename, ', ') FROM (
			               SELECT DISTINCT e.name AS ename
			               FROM workout_sets ws
			               JOIN exercises e ON e.id = ws.exercise_id
			               WHERE ws.workout_id = w.id
			               ORDER BY e.name
			           )),
			           'Workout'
			       ) || ' (' || (SELECT COUNT(*) FROM workout_sets WHERE workout_id = w.id) || ' sets)' AS summary,
			       w.id AS id,
			       COALESCE(w.notes, '') AS detail,
			       0 AS is_private,
			       0 AS pinned,
			       0 AS second_id,
			       '' AS author,
			       0 AS author_id
			FROM workouts w
			WHERE w.athlete_id = ?

			UNION ALL

			-- Body Weights
			SELECT bw.date AS date,
			       'body_weight' AS type,
			       'Recorded body weight: ' || CAST(bw.weight AS TEXT) AS summary,
			       bw.id AS id,
			       COALESCE(bw.notes, '') AS detail,
			       0 AS is_private,
			       0 AS pinned,
			       0 AS second_id,
			       '' AS author,
			       0 AS author_id
			FROM body_weights bw
			WHERE bw.athlete_id = ?

			UNION ALL

			-- Training Max Changes
			SELECT tm.effective_date AS date,
			       'training_max' AS type,
			       e.name || ' TM set to ' || CAST(tm.weight AS TEXT) AS summary,
			       tm.id AS id,
			       '' AS detail,
			       0 AS is_private,
			       0 AS pinned,
			       tm.exercise_id AS second_id,
			       '' AS author,
			       0 AS author_id
			FROM training_maxes tm
			JOIN exercises e ON e.id = tm.exercise_id
			WHERE tm.athlete_id = ?

			UNION ALL

			-- Goal Changes
			SELECT gh.effective_date AS date,
			       'goal_change' AS type,
			       'Goal changed to: ' || gh.goal AS summary,
			       gh.id AS id,
			       COALESCE(gh.notes, '') AS detail,
			       0 AS is_private,
			       0 AS pinned,
			       0 AS second_id,
			       COALESCE(u.name, u.username, '') AS author,
			       COALESCE(gh.set_by, 0) AS author_id
			FROM goal_history gh
			LEFT JOIN users u ON u.id = gh.set_by
			WHERE gh.athlete_id = ?

			UNION ALL

			-- Tier Changes
			SELECT th.effective_date AS date,
			       'tier_change' AS type,
			       'Tier changed to ' || th.tier ||
			           CASE WHEN th.previous_tier IS NOT NULL
			                THEN ' (from ' || th.previous_tier || ')'
			                ELSE '' END AS summary,
			       th.id AS id,
			       COALESCE(th.notes, '') AS detail,
			       0 AS is_private,
			       0 AS pinned,
			       0 AS second_id,
			       COALESCE(u.name, u.username, '') AS author,
			       COALESCE(th.set_by, 0) AS author_id
			FROM tier_history th
			LEFT JOIN users u ON u.id = th.set_by
			WHERE th.athlete_id = ?

			UNION ALL

			-- Program Starts
			SELECT ap.start_date AS date,
			       'program_start' AS type,
			       'Started program: ' || pt.name AS summary,
			       ap.id AS id,
			       COALESCE(ap.goal, '') AS detail,
			       0 AS is_private,
			       0 AS pinned,
			       ap.template_id AS second_id,
			       '' AS author,
			       0 AS author_id
			FROM athlete_programs ap
			JOIN program_templates pt ON pt.id = ap.template_id
			WHERE ap.athlete_id = ?

			UNION ALL

			-- Workout Reviews
			SELECT date(wr.created_at) AS date,
			       'review' AS type,
			       wr.status || ' — ' || COALESCE(wr.notes, 'No comment') AS summary,
			       wr.id AS id,
			       '' AS detail,
			       0 AS is_private,
			       0 AS pinned,
			       wr.workout_id AS second_id,
			       COALESCE(u.name, u.username, '') AS author,
			       COALESCE(wr.coach_id, 0) AS author_id
			FROM workout_reviews wr
			LEFT JOIN users u ON u.id = wr.coach_id
			JOIN workouts w ON w.id = wr.workout_id
			WHERE w.athlete_id = ?

			UNION ALL

			-- Athlete Notes
			SELECT n.date AS date,
			       'note' AS type,
			       n.content AS summary,
			       n.id AS id,
			       '' AS detail,
			       n.is_private AS is_private,
			       n.pinned AS pinned,
			       0 AS second_id,
			       COALESCE(u.name, u.username, '') AS author,
			       COALESCE(n.author_id, 0) AS author_id
			FROM athlete_notes n
			LEFT JOIN users u ON u.id = n.author_id
			WHERE n.athlete_id = ?%s
		)
		ORDER BY pinned DESC, date DESC
		LIMIT ?`,
		privateFilter,
	)

	rows, err := db.Query(query,
		athleteID, athleteID, athleteID, athleteID,
		athleteID, athleteID, athleteID, athleteID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list journal entries for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var entries []*JournalEntry
	for rows.Next() {
		e := &JournalEntry{}
		var privInt, pinnedInt int
		if err := rows.Scan(&e.Date, &e.Type, &e.Summary, &e.ID,
			&e.Detail, &privInt, &pinnedInt, &e.SecondID, &e.Author, &e.AuthorID); err != nil {
			return nil, fmt.Errorf("models: scan journal entry: %w", err)
		}
		e.IsPrivate = privInt == 1
		e.Pinned = pinnedInt == 1
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
