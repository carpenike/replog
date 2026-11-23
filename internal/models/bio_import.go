package models

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"
)

// Health import (gap 14): batched watch/phone readings written to
// bio_samples with source 'watch_import'. Imports are idempotent per
// (athlete, metric, recorded_at), so a Shortcut or Health Auto Export
// automation can resend overlapping windows safely. The readings are
// informational; nothing here changes a plan.

// MaxBioImportSamples caps one import request.
const MaxBioImportSamples = 5000

// bioImportStoreLayout is how recorded_at is written for imported rows:
// UTC, second precision, so equal instants compare equal as text and the
// partial unique index can enforce idempotency.
const bioImportStoreLayout = "2006-01-02 15:04:05"

// bioImportMetric describes one importable metric: its canonical key, the
// unit it is stored in, and the accepted value range.
type bioImportMetric struct {
	Key      string
	Unit     string
	Min, Max float64
}

var bioImportMetrics = map[string]bioImportMetric{
	"sleep_hours": {Key: "sleep_hours", Unit: "h", Min: 0, Max: 24},
	"resting_hr":  {Key: "resting_hr", Unit: "bpm", Min: 20, Max: 250},
	"hrv":         {Key: "hrv", Unit: "ms", Min: 0, Max: 500},
	"steps":       {Key: "steps", Unit: "count", Min: 0, Max: 200000},
}

// bioImportAliases maps the names Apple Health exporters use onto the
// canonical metric keys. Lookup is case-insensitive with spaces and dashes
// folded to underscores.
var bioImportAliases = map[string]string{
	"sleep":                       "sleep_hours",
	"sleep_hours":                 "sleep_hours",
	"sleep_analysis":              "sleep_hours",
	"resting_hr":                  "resting_hr",
	"resting_heart_rate":          "resting_hr",
	"hrv":                         "hrv",
	"heart_rate_variability":      "hrv",
	"heart_rate_variability_sdnn": "hrv",
	"steps":                       "steps",
	"step_count":                  "steps",
}

// BioImportMetricKeys lists the canonical metric keys the importer accepts.
func BioImportMetricKeys() []string {
	return []string{"sleep_hours", "resting_hr", "hrv", "steps"}
}

// CanonicalBioMetric resolves an exporter's metric name to a canonical key.
func CanonicalBioMetric(name string) (string, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.NewReplacer(" ", "_", "-", "_").Replace(n)
	key, ok := bioImportAliases[n]
	return key, ok
}

// bioImportTimeLayouts are the recorded_at shapes accepted, most specific
// first. Health Auto Export sends "2006-01-02 15:04:05 -0700"; Shortcuts
// usually send ISO 8601. A layout without a zone is read in the caller's
// timezone; a bare date is midnight there (daily totals such as steps).
var bioImportTimeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02 15:04:05 -0700",
	"2006-01-02 15:04:05 -07:00",
	"2006-01-02T15:04:05-0700",
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04",
	"2006-01-02",
}

// ParseBioImportTime parses a recorded_at value; zone-less values are read
// in loc. It returns the instant truncated to the second.
func ParseBioImportTime(s string, loc *time.Location) (time.Time, error) {
	s = strings.TrimSpace(s)
	if loc == nil {
		loc = time.UTC
	}
	for _, layout := range bioImportTimeLayouts {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t.Truncate(time.Second), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized recorded_at %q", s)
}

// BioImportSample is one raw reading as an exporter sent it.
type BioImportSample struct {
	Metric     string
	Value      float64
	Unit       string
	RecordedAt string
}

// NormalizedBioSample is a reading ready to store: canonical metric, value
// in the canonical unit, and recorded_at in UTC store layout.
type NormalizedBioSample struct {
	Metric     string
	Value      float64
	Unit       string
	RecordedAt string
}

// NormalizeBioImportSample validates and normalizes one reading. Sleep sent
// in minutes ("min", "minutes") is converted to hours. The error text is
// safe to return to the client.
func NormalizeBioImportSample(in BioImportSample, loc *time.Location) (NormalizedBioSample, error) {
	key, ok := CanonicalBioMetric(in.Metric)
	if !ok {
		return NormalizedBioSample{}, fmt.Errorf("unsupported metric %q (accepted: sleep_hours, resting_hr, hrv, steps)", in.Metric)
	}
	m := bioImportMetrics[key]
	v := in.Value
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return NormalizedBioSample{}, fmt.Errorf("value must be a finite number")
	}
	unit := strings.ToLower(strings.TrimSpace(in.Unit))
	if key == "sleep_hours" && (unit == "min" || unit == "mins" || unit == "minutes") {
		v /= 60
	}
	if v < m.Min || v > m.Max {
		return NormalizedBioSample{}, fmt.Errorf("%s value %g is outside %g–%g %s", key, v, m.Min, m.Max, m.Unit)
	}
	if strings.TrimSpace(in.RecordedAt) == "" {
		return NormalizedBioSample{}, fmt.Errorf("recorded_at is required")
	}
	t, err := ParseBioImportTime(in.RecordedAt, loc)
	if err != nil {
		return NormalizedBioSample{}, err
	}
	return NormalizedBioSample{
		Metric:     key,
		Value:      math.Round(v*100) / 100,
		Unit:       m.Unit,
		RecordedAt: t.UTC().Format(bioImportStoreLayout),
	}, nil
}

// ImportBioSamples writes normalized readings for an athlete in one
// transaction. A reading whose (metric, recorded_at) already exists for the
// athlete — imported or entered by hand — is skipped and counted as a
// duplicate, so re-sending the same window is a no-op.
func ImportBioSamples(ctx context.Context, db *sql.DB, athleteID int64, samples []NormalizedBioSample) (inserted, duplicates int, err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("models: begin bio import: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO bio_samples (athlete_id, recorded_at, metric, value, unit, source)
		 SELECT ?, ?, ?, ?, ?, 'watch_import'
		  WHERE NOT EXISTS (
		        SELECT 1 FROM bio_samples
		         WHERE athlete_id = ? AND metric = ?
		           AND datetime(recorded_at) = datetime(?))`)
	if err != nil {
		return 0, 0, fmt.Errorf("models: prepare bio import: %w", err)
	}
	defer stmt.Close()

	for _, s := range samples {
		res, err := stmt.ExecContext(ctx,
			athleteID, s.RecordedAt, s.Metric, s.Value, s.Unit,
			athleteID, s.Metric, s.RecordedAt)
		if err != nil {
			return 0, 0, fmt.Errorf("models: import bio sample for athlete %d: %w", athleteID, err)
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			inserted++
		} else {
			duplicates++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("models: commit bio import: %w", err)
	}
	return inserted, duplicates, nil
}
