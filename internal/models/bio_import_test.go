package models

import (
	"context"
	"database/sql"
	"math"
	"strings"
	"testing"
	"time"
)

func TestNormalizeBioImportSample(t *testing.T) {
	ny, _ := time.LoadLocation("America/New_York")
	cases := []struct {
		name    string
		in      BioImportSample
		want    NormalizedBioSample
		wantErr string
	}{
		{
			name: "health auto export resting hr",
			in:   BioImportSample{Metric: "resting_heart_rate", Value: 52, RecordedAt: "2026-11-21 00:00:00 -0500"},
			want: NormalizedBioSample{Metric: "resting_hr", Value: 52, Unit: "bpm", RecordedAt: "2026-11-21 05:00:00"},
		},
		{
			name: "iso hrv",
			in:   BioImportSample{Metric: "Heart Rate Variability", Value: 61.234, RecordedAt: "2026-11-21T07:15:00Z"},
			want: NormalizedBioSample{Metric: "hrv", Value: 61.23, Unit: "ms", RecordedAt: "2026-11-21 07:15:00"},
		},
		{
			name: "sleep minutes converted",
			in:   BioImportSample{Metric: "sleep", Value: 450, Unit: "min", RecordedAt: "2026-11-21"},
			want: NormalizedBioSample{Metric: "sleep_hours", Value: 7.5, Unit: "h", RecordedAt: "2026-11-21 05:00:00"},
		},
		{
			name: "zone-less time read in caller tz",
			in:   BioImportSample{Metric: "step_count", Value: 8000, RecordedAt: "2026-07-04 12:00:00"},
			want: NormalizedBioSample{Metric: "steps", Value: 8000, Unit: "count", RecordedAt: "2026-07-04 16:00:00"},
		},
		{name: "unknown metric", in: BioImportSample{Metric: "vo2max", Value: 40, RecordedAt: "2026-11-21"}, wantErr: "unsupported metric"},
		{name: "out of range", in: BioImportSample{Metric: "sleep_hours", Value: 30, RecordedAt: "2026-11-21"}, wantErr: "outside"},
		{name: "nan", in: BioImportSample{Metric: "hrv", Value: math.NaN(), RecordedAt: "2026-11-21"}, wantErr: "finite"},
		{name: "bad time", in: BioImportSample{Metric: "hrv", Value: 50, RecordedAt: "yesterday"}, wantErr: "unrecognized"},
		{name: "missing time", in: BioImportSample{Metric: "hrv", Value: 50}, wantErr: "required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NormalizeBioImportSample(c.in, ny)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestImportBioSamplesIdempotent(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	a, err := CreateAthlete(ctx, db, "Watcher", "", "", "", "", "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatal(err)
	}
	// A manual reading at the same instant as one import row, written in a
	// different text shape, still counts as the same key.
	if _, err := CreateBioSample(ctx, db, a.ID, BioSampleInput{
		RecordedAt: "2026-11-20T06:00:00Z", Metric: "resting_hr", Value: 55,
	}); err != nil {
		t.Fatal(err)
	}
	batch := []NormalizedBioSample{
		{Metric: "resting_hr", Value: 55, Unit: "bpm", RecordedAt: "2026-11-20 06:00:00"},
		{Metric: "resting_hr", Value: 53, Unit: "bpm", RecordedAt: "2026-11-21 06:00:00"},
		{Metric: "hrv", Value: 60, Unit: "ms", RecordedAt: "2026-11-21 06:00:00"},
		{Metric: "hrv", Value: 60, Unit: "ms", RecordedAt: "2026-11-21 06:00:00"}, // repeated in-batch
	}
	ins, dup, err := ImportBioSamples(ctx, db, a.ID, batch)
	if err != nil {
		t.Fatal(err)
	}
	if ins != 2 || dup != 2 {
		t.Fatalf("first import inserted=%d dup=%d, want 2/2", ins, dup)
	}
	ins, dup, err = ImportBioSamples(ctx, db, a.ID, batch)
	if err != nil {
		t.Fatal(err)
	}
	if ins != 0 || dup != 4 {
		t.Fatalf("re-import inserted=%d dup=%d, want 0/4", ins, dup)
	}
	samples, err := ListBioSamples(ctx, db, a.ID, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 3 {
		t.Fatalf("stored %d samples, want 3", len(samples))
	}
	imported := 0
	for _, s := range samples {
		if s.Source == "watch_import" {
			imported++
		}
	}
	if imported != 2 {
		t.Fatalf("imported rows = %d, want 2", imported)
	}

	// The partial unique index backs the check for imported rows.
	_, err = db.Exec(`INSERT INTO bio_samples (athlete_id, recorded_at, metric, value, source)
	                  VALUES (?, '2026-11-21 06:00:00', 'hrv', 1, 'watch_import')`, a.ID)
	if err == nil {
		t.Fatal("duplicate watch_import row was accepted by the schema")
	}
}
