package models

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/carpenike/replog/internal/importers"
)

func TestBuildAthleteExportJSON_PreservesProgramSchedule(t *testing.T) {
	db := testDB(t)
	athlete, err := CreateAthlete(context.Background(), db, "Export Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}
	template, err := CreateProgramTemplate(context.Background(), db, nil, "Scheduled Program", "", 1, 3, true, "adult")
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	if _, err := AssignProgram(context.Background(), db, athlete.ID, template.ID, "2026-08-03", "", "", "primary", "[1,3,5]"); err != nil {
		t.Fatalf("assign program: %v", err)
	}

	export, err := BuildAthleteExportJSON(context.Background(), db, athlete.ID)
	if err != nil {
		t.Fatalf("build athlete export: %v", err)
	}
	encoded, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("marshal athlete export: %v", err)
	}
	parsed, err := importers.ParseRepLogJSON(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("parse athlete export: %v", err)
	}
	if len(parsed.Programs) != 1 || parsed.Programs[0].Schedule == nil || *parsed.Programs[0].Schedule != "[1,3,5]" {
		t.Fatalf("exported schedule = %+v, want [1,3,5]", parsed.Programs)
	}
}
