package models

import (
	"database/sql"
	"testing"

	"github.com/carpenike/replog/internal/database"
)

// testDB creates a fresh in-memory SQLite database with migrations applied.
// It returns the *sql.DB and a cleanup function.
func testDB(t testing.TB) *sql.DB {
	t.Helper()

	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := database.RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}
