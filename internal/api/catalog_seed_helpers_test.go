package api

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"

	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/importers"
	"github.com/carpenike/replog/internal/models"
)

// applyCatalogSeed runs the embedded seed catalog through the same
// import pipeline cmd/replog/main.go bootstrapCatalog uses. Pulled out
// of the handler tests so the methodology fixture (HOF-006) can stand
// up the equipment/exercise/program rows the methodology seed references.
func applyCatalogSeed(ctx context.Context, db *sql.DB) error {
	parsed, err := importers.ParseCatalogJSON(bytes.NewReader(database.SeedCatalog()))
	if err != nil {
		return fmt.Errorf("parse catalog: %w", err)
	}
	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(parsed.Exercises, nil),
		Equipment: importers.BuildEquipmentMappings(parsed.Equipment, nil),
		Programs:  importers.BuildProgramMappings(parsed.Programs, nil),
		Parsed:    parsed,
	}
	if _, err := models.ExecuteCatalogImport(context.Background(), db, ms, nil, false); err != nil {
		return fmt.Errorf("execute catalog import: %w", err)
	}
	return nil
}
