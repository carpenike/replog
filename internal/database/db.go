package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens a SQLite database at the given path and configures it for
// RepLog's requirements: WAL mode, foreign keys, and single-writer concurrency.
func Open(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("database: open %s: %w", dbPath, err)
	}

	// SQLite is single-writer — one connection avoids SQLITE_BUSY contention.
	db.SetMaxOpenConns(1)

	// Set required PRAGMAs on the connection.
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("database: exec %q: %w", p, err)
		}
	}

	return db, nil
}
