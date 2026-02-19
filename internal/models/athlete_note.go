package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// AthleteNote represents a coach's free-text note attached to an athlete.
type AthleteNote struct {
	ID        int64
	AthleteID int64
	AuthorID  sql.NullInt64
	Date      string // DATE as string (YYYY-MM-DD)
	Content   string
	IsPrivate bool
	Pinned    bool
	CreatedAt time.Time
	UpdatedAt time.Time

	// Joined field populated by list queries.
	AuthorName string
}

// CreateAthleteNote inserts a new note for an athlete.
func CreateAthleteNote(db *sql.DB, athleteID, authorID int64, date, content string, isPrivate, pinned bool) (*AthleteNote, error) {
	if content == "" {
		return nil, fmt.Errorf("models: create athlete note: %w: content is required", ErrInvalidInput)
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	privInt := 0
	if isPrivate {
		privInt = 1
	}
	pinnedInt := 0
	if pinned {
		pinnedInt = 1
	}

	var id int64
	err := db.QueryRow(
		`INSERT INTO athlete_notes (athlete_id, author_id, date, content, is_private, pinned)
		 VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		athleteID, authorID, date, content, privInt, pinnedInt,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("models: create athlete note for athlete %d: %w", athleteID, err)
	}

	return GetAthleteNoteByID(db, id)
}

// GetAthleteNoteByID retrieves a single athlete note by primary key.
func GetAthleteNoteByID(db *sql.DB, id int64) (*AthleteNote, error) {
	n := &AthleteNote{}
	var privInt, pinnedInt int
	err := db.QueryRow(
		`SELECT n.id, n.athlete_id, n.author_id, n.date, n.content,
		        n.is_private, n.pinned, n.created_at, n.updated_at,
		        COALESCE(u.name, u.username, '') AS author_name
		 FROM athlete_notes n
		 LEFT JOIN users u ON u.id = n.author_id
		 WHERE n.id = ?`, id,
	).Scan(&n.ID, &n.AthleteID, &n.AuthorID, &n.Date, &n.Content,
		&privInt, &pinnedInt, &n.CreatedAt, &n.UpdatedAt, &n.AuthorName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get athlete note %d: %w", id, err)
	}
	n.IsPrivate = privInt == 1
	n.Pinned = pinnedInt == 1
	return n, nil
}

// UpdateAthleteNote updates the content, visibility, and pinned status of a note.
func UpdateAthleteNote(db *sql.DB, id int64, content string, isPrivate, pinned bool) (*AthleteNote, error) {
	if content == "" {
		return nil, fmt.Errorf("models: update athlete note: %w: content is required", ErrInvalidInput)
	}

	privInt := 0
	if isPrivate {
		privInt = 1
	}
	pinnedInt := 0
	if pinned {
		pinnedInt = 1
	}

	result, err := db.Exec(
		`UPDATE athlete_notes SET content = ?, is_private = ?, pinned = ? WHERE id = ?`,
		content, privInt, pinnedInt, id,
	)
	if err != nil {
		return nil, fmt.Errorf("models: update athlete note %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, ErrNotFound
	}

	return GetAthleteNoteByID(db, id)
}

// DeleteAthleteNote removes a note by ID.
func DeleteAthleteNote(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM athlete_notes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete athlete note %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ListAthleteNotes returns notes for an athlete, newest first.
// If includePrivate is false, only public notes are returned (for non-coach view).
func ListAthleteNotes(db *sql.DB, athleteID int64, includePrivate bool) ([]*AthleteNote, error) {
	query := `SELECT n.id, n.athlete_id, n.author_id, n.date, n.content,
	                 n.is_private, n.pinned, n.created_at, n.updated_at,
	                 COALESCE(u.name, u.username, '') AS author_name
	          FROM athlete_notes n
	          LEFT JOIN users u ON u.id = n.author_id
	          WHERE n.athlete_id = ?`
	if !includePrivate {
		query += ` AND n.is_private = 0`
	}
	query += ` ORDER BY n.pinned DESC, n.date DESC, n.created_at DESC LIMIT 200`

	rows, err := db.Query(query, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: list athlete notes for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var notes []*AthleteNote
	for rows.Next() {
		n := &AthleteNote{}
		var privInt, pinnedInt int
		if err := rows.Scan(&n.ID, &n.AthleteID, &n.AuthorID, &n.Date, &n.Content,
			&privInt, &pinnedInt, &n.CreatedAt, &n.UpdatedAt, &n.AuthorName); err != nil {
			return nil, fmt.Errorf("models: scan athlete note: %w", err)
		}
		n.IsPrivate = privInt == 1
		n.Pinned = pinnedInt == 1
		notes = append(notes, n)
	}
	return notes, rows.Err()
}
