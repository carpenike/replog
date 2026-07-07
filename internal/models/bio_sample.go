package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// BioSample is an append-only biometric reading for an athlete (ADR 018) —
// e.g. resting HR, HRV, sleep hours. Manually entered or watch-imported.
type BioSample struct {
	ID         int64
	AthleteID  int64
	RecordedAt time.Time
	Metric     string
	Value      float64
	Unit       sql.NullString
	Source     string // manual | watch_import
	Notes      sql.NullString
	CreatedAt  time.Time
}

// BioSampleInput carries the fields for recording a bio sample.
type BioSampleInput struct {
	RecordedAt string // RFC3339 or "YYYY-MM-DD HH:MM:SS"
	Metric     string
	Value      float64
	Unit       string
	Source     string
	Notes      string
}

// CreateBioSample records a biometric reading. Rows are immutable once written.
func CreateBioSample(ctx context.Context, db *sql.DB, athleteID int64, in BioSampleInput) (*BioSample, error) {
	if in.Metric == "" {
		return nil, fmt.Errorf("models: bio sample metric required: %w", ErrInvalidInput)
	}
	if in.RecordedAt == "" {
		return nil, fmt.Errorf("models: bio sample recorded_at required: %w", ErrInvalidInput)
	}
	source := in.Source
	if source == "" {
		source = "manual"
	}
	if source != "manual" && source != "watch_import" {
		return nil, fmt.Errorf("models: invalid source %q: %w", source, ErrInvalidInput)
	}

	var id int64
	err := db.QueryRowContext(ctx,
		`INSERT INTO bio_samples (athlete_id, recorded_at, metric, value, unit, source, notes)
		 VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		athleteID, in.RecordedAt, in.Metric, in.Value,
		nullableString(in.Unit), source, nullableString(in.Notes),
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("models: create bio sample for athlete %d: %w", athleteID, err)
	}
	return GetBioSampleByID(ctx, db, id)
}

// GetBioSampleByID retrieves a bio sample by primary key.
func GetBioSampleByID(ctx context.Context, db *sql.DB, id int64) (*BioSample, error) {
	bs := &BioSample{}
	err := db.QueryRowContext(ctx,
		`SELECT id, athlete_id, recorded_at, metric, value, unit, source, notes, created_at
		 FROM bio_samples WHERE id = ?`, id,
	).Scan(&bs.ID, &bs.AthleteID, &bs.RecordedAt, &bs.Metric, &bs.Value, &bs.Unit, &bs.Source, &bs.Notes, &bs.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get bio sample %d: %w", id, err)
	}
	return bs, nil
}

// ListBioSamples returns an athlete's bio samples, newest first. If metric is
// non-empty, results are filtered to that metric.
func ListBioSamples(ctx context.Context, db *sql.DB, athleteID int64, metric string, limit int) ([]*BioSample, error) {
	if limit <= 0 {
		limit = 100
	}
	var (
		rows *sql.Rows
		err  error
	)
	if metric != "" {
		rows, err = db.QueryContext(ctx,
			`SELECT id, athlete_id, recorded_at, metric, value, unit, source, notes, created_at
			 FROM bio_samples WHERE athlete_id = ? AND metric = ?
			 ORDER BY recorded_at DESC, id DESC LIMIT ?`, athleteID, metric, limit)
	} else {
		rows, err = db.QueryContext(ctx,
			`SELECT id, athlete_id, recorded_at, metric, value, unit, source, notes, created_at
			 FROM bio_samples WHERE athlete_id = ?
			 ORDER BY recorded_at DESC, id DESC LIMIT ?`, athleteID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("models: list bio samples for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var samples []*BioSample
	for rows.Next() {
		bs := &BioSample{}
		if err := rows.Scan(&bs.ID, &bs.AthleteID, &bs.RecordedAt, &bs.Metric, &bs.Value, &bs.Unit, &bs.Source, &bs.Notes, &bs.CreatedAt); err != nil {
			return nil, fmt.Errorf("models: scan bio sample: %w", err)
		}
		samples = append(samples, bs)
	}
	return samples, rows.Err()
}
