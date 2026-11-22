package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// MaxVideoCommentLen caps a coach's comment on a form-check video.
const MaxVideoCommentLen = 2000

// SetVideo is a form-check clip attached to one logged set (gap 12). The
// file lives on disk (internal/media); this row records which set it
// belongs to, what was sniffed, and the coach's comment for the review.
type SetVideo struct {
	ID            int64
	SetID         int64
	WorkoutID     int64
	AthleteID     int64
	FileName      string
	ContentType   string
	SizeBytes     int64
	UploadedBy    sql.NullInt64
	UploaderName  sql.NullString
	CoachComment  sql.NullString
	CommentedBy   sql.NullInt64
	CommenterName sql.NullString
	CommentedAt   sql.NullTime
	CreatedAt     time.Time
}

// SetVideoInput carries a freshly stored clip's metadata.
type SetVideoInput struct {
	FileName    string
	ContentType string
	SizeBytes   int64
	UploadedBy  int64
}

const setVideoSelect = `SELECT v.id, v.set_id, ws.workout_id, w.athlete_id, v.file_name, v.content_type,
       v.size_bytes, v.uploaded_by, COALESCE(uu.name, uu.username), v.coach_comment, v.commented_by, COALESCE(cu.name, cu.username),
       v.commented_at, v.created_at
  FROM workout_set_videos v
  JOIN workout_sets ws ON ws.id = v.set_id
  JOIN workouts w ON w.id = ws.workout_id
  LEFT JOIN users uu ON uu.id = v.uploaded_by
  LEFT JOIN users cu ON cu.id = v.commented_by`

func scanSetVideo(sc interface{ Scan(...any) error }) (*SetVideo, error) {
	v := &SetVideo{}
	err := sc.Scan(&v.ID, &v.SetID, &v.WorkoutID, &v.AthleteID, &v.FileName, &v.ContentType,
		&v.SizeBytes, &v.UploadedBy, &v.UploaderName, &v.CoachComment, &v.CommentedBy,
		&v.CommenterName, &v.CommentedAt, &v.CreatedAt)
	return v, err
}

// CheckSetOwnership reports ErrNotFound unless set setID is in workout
// workoutID and that workout belongs to athleteID (ADR 021).
func CheckSetOwnership(ctx context.Context, db *sql.DB, setID, workoutID, athleteID int64) error {
	var one int
	err := db.QueryRowContext(ctx,
		`SELECT 1 FROM workout_sets ws JOIN workouts w ON w.id = ws.workout_id
		  WHERE ws.id = ? AND ws.workout_id = ? AND w.athlete_id = ?`,
		setID, workoutID, athleteID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("models: check set %d ownership: %w", setID, err)
	}
	return nil
}

// GetSetVideo returns the clip on a set, scoped to the owning workout and
// athlete. ErrNotFound when the set has no clip or is not theirs.
func GetSetVideo(ctx context.Context, db *sql.DB, setID, workoutID, athleteID int64) (*SetVideo, error) {
	v, err := scanSetVideo(db.QueryRowContext(ctx,
		setVideoSelect+` WHERE v.set_id = ? AND ws.workout_id = ? AND w.athlete_id = ?`,
		setID, workoutID, athleteID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get video for set %d: %w", setID, err)
	}
	return v, nil
}

// ReplaceSetVideo attaches a clip to a set, replacing any earlier clip (and
// its comment, which was about the old clip). It returns the new row and
// the replaced clip's file name ("" when there was none) so the caller can
// delete the old file. The caller checks ownership first.
func ReplaceSetVideo(ctx context.Context, db *sql.DB, setID int64, in SetVideoInput) (*SetVideo, string, error) {
	if in.FileName == "" || in.SizeBytes <= 0 {
		return nil, "", fmt.Errorf("models: set video file required: %w", ErrInvalidInput)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("models: begin set video tx: %w", err)
	}
	defer tx.Rollback()

	var old string
	err = tx.QueryRowContext(ctx, `SELECT file_name FROM workout_set_videos WHERE set_id = ?`, setID).Scan(&old)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, "", fmt.Errorf("models: read old video for set %d: %w", setID, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM workout_set_videos WHERE set_id = ?`, setID); err != nil {
		return nil, "", fmt.Errorf("models: clear old video for set %d: %w", setID, err)
	}
	var uploader sql.NullInt64
	if in.UploadedBy > 0 {
		uploader = sql.NullInt64{Int64: in.UploadedBy, Valid: true}
	}
	var id int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO workout_set_videos (set_id, file_name, content_type, size_bytes, uploaded_by)
		 VALUES (?, ?, ?, ?, ?) RETURNING id`,
		setID, in.FileName, in.ContentType, in.SizeBytes, uploader).Scan(&id)
	if err != nil {
		return nil, "", fmt.Errorf("models: insert video for set %d: %w", setID, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("models: commit set video: %w", err)
	}
	v, err := scanSetVideo(db.QueryRowContext(ctx, setVideoSelect+` WHERE v.id = ?`, id))
	if err != nil {
		return nil, "", fmt.Errorf("models: reload video %d: %w", id, err)
	}
	return v, old, nil
}

// DeleteSetVideo removes the clip on a set, scoped to the owning workout and
// athlete, and returns its file name for the caller to delete.
func DeleteSetVideo(ctx context.Context, db *sql.DB, setID, workoutID, athleteID int64) (string, error) {
	v, err := GetSetVideo(ctx, db, setID, workoutID, athleteID)
	if err != nil {
		return "", err
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM workout_set_videos WHERE id = ?`, v.ID); err != nil {
		return "", fmt.Errorf("models: delete video %d: %w", v.ID, err)
	}
	return v.FileName, nil
}

// CommentSetVideo sets (or, with an empty comment, clears) the coach's
// comment on a set's clip. It is review feedback only.
func CommentSetVideo(ctx context.Context, db *sql.DB, setID, workoutID, athleteID, coachID int64, comment string) (*SetVideo, error) {
	if len(comment) > MaxVideoCommentLen {
		return nil, fmt.Errorf("%w: comment must be at most %d characters", ErrInvalidInput, MaxVideoCommentLen)
	}
	v, err := GetSetVideo(ctx, db, setID, workoutID, athleteID)
	if err != nil {
		return nil, err
	}
	if comment == "" {
		_, err = db.ExecContext(ctx,
			`UPDATE workout_set_videos SET coach_comment = NULL, commented_by = NULL, commented_at = NULL WHERE id = ?`, v.ID)
	} else {
		_, err = db.ExecContext(ctx,
			`UPDATE workout_set_videos SET coach_comment = ?, commented_by = ?, commented_at = CURRENT_TIMESTAMP WHERE id = ?`,
			comment, coachID, v.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("models: comment on video %d: %w", v.ID, err)
	}
	return GetSetVideo(ctx, db, setID, workoutID, athleteID)
}

// ListSetVideosByWorkout returns the clips in a workout keyed by set ID.
func ListSetVideosByWorkout(ctx context.Context, db *sql.DB, workoutID int64) (map[int64]*SetVideo, error) {
	rows, err := db.QueryContext(ctx, setVideoSelect+` WHERE ws.workout_id = ?`, workoutID)
	if err != nil {
		return nil, fmt.Errorf("models: list videos for workout %d: %w", workoutID, err)
	}
	defer rows.Close()
	out := map[int64]*SetVideo{}
	for rows.Next() {
		v, err := scanSetVideo(rows)
		if err != nil {
			return nil, fmt.Errorf("models: scan set video: %w", err)
		}
		out[v.SetID] = v
	}
	return out, rows.Err()
}

// ListVideoFileNamesForWorkout returns the stored file names of the clips in
// a workout, so a caller deleting the workout can remove the files too.
func ListVideoFileNamesForWorkout(ctx context.Context, db *sql.DB, workoutID int64) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT v.file_name FROM workout_set_videos v JOIN workout_sets ws ON ws.id = v.set_id
		  WHERE ws.workout_id = ?`, workoutID)
	if err != nil {
		return nil, fmt.Errorf("models: list video files for workout %d: %w", workoutID, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("models: scan video file: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// SetVideoFileNames returns every stored clip file name. The scheduler uses
// it to sweep files no row references (left by a cascade delete).
func SetVideoFileNames(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT file_name FROM workout_set_videos`)
	if err != nil {
		return nil, fmt.Errorf("models: list video files: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("models: scan video file: %w", err)
		}
		out[n] = true
	}
	return out, rows.Err()
}

// SetVideoFileName returns the clip file on a set owned by athleteID, or ""
// when there is none. A caller deleting the set removes the file after.
func SetVideoFileName(ctx context.Context, db *sql.DB, setID, athleteID int64) (string, error) {
	var name string
	err := db.QueryRowContext(ctx,
		`SELECT v.file_name FROM workout_set_videos v
		   JOIN workout_sets ws ON ws.id = v.set_id
		   JOIN workouts w ON w.id = ws.workout_id
		  WHERE v.set_id = ? AND w.athlete_id = ?`, setID, athleteID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("models: video file for set %d: %w", setID, err)
	}
	return name, nil
}
