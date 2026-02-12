package database

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// RunMigrations applies all pending database migrations. It should be called
// once on startup before the HTTP server begins accepting requests.
func RunMigrations(db *sql.DB) error {
	goose.SetBaseFS(migrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("database: set dialect: %w", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("database: run migrations: %w", err)
	}

	return nil
}
