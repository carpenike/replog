package models

import (
	"database/sql"
	"testing"
	"time"

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

// mustParseDate parses a YYYY-MM-DD string into a time.Time, panicking on error.
func mustParseDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic("mustParseDate: " + err.Error())
	}
	return t
}
