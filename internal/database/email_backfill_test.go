package database

import (
	"context"
	"database/sql"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/carpenike/replog/internal/models"
)

func TestRyanEmailBackfill(t *testing.T) {
	t.Run("sets empty ryan email only", func(t *testing.T) {
		db := openMigratedTo(t, 19)
		mustExec(t, db, `INSERT INTO users (username, email, is_coach, is_admin) VALUES ('Ryan', NULL, 1, 1)`)
		mustExec(t, db, `INSERT INTO users (username, email, is_coach, is_admin) VALUES ('sam', 'sam@example.com', 0, 0)`)
		mustExec(t, db, `INSERT INTO users (username, email, is_coach, is_admin) VALUES ('kid', NULL, 0, 0)`)
		mustExec(t, db, `INSERT INTO users (username, email, is_coach, is_admin) VALUES ('filled', 'kept@example.com', 0, 0)`)
		applyRest(t, db)

		assertEmail(t, db, "ryan", "ryan@ryanholt.net")
		assertEmail(t, db, "sam", "sam@example.com")
		assertEmail(t, db, "kid", "")
		assertEmail(t, db, "filled", "kept@example.com")
		assertUserCount(t, db, 4)
	})

	t.Run("leaves an existing ryan email", func(t *testing.T) {
		db := openMigratedTo(t, 19)
		mustExec(t, db, `INSERT INTO users (username, email, is_coach, is_admin) VALUES ('ryan', 'already@example.com', 1, 1)`)
		applyRest(t, db)
		assertEmail(t, db, "ryan", "already@example.com")
	})

	t.Run("does not invent ryan", func(t *testing.T) {
		db := openMigratedTo(t, 19)
		mustExec(t, db, `INSERT INTO users (username, email, is_coach, is_admin) VALUES ('sam', NULL, 0, 0)`)
		applyRest(t, db)
		assertUserCount(t, db, 1)
		assertEmail(t, db, "sam", "")
		if _, err := models.GetUserByUsername(context.Background(), db, "ryan"); err == nil {
			t.Fatal("migration created a ryan user")
		}
	})

	t.Run("skips when the address is taken", func(t *testing.T) {
		db := openMigratedTo(t, 19)
		mustExec(t, db, `INSERT INTO users (username, email, is_coach, is_admin) VALUES ('ryan', NULL, 1, 1)`)
		mustExec(t, db, `INSERT INTO users (username, email, is_coach, is_admin) VALUES ('other', 'ryan@ryanholt.net', 0, 0)`)
		applyRest(t, db)
		assertEmail(t, db, "ryan", "")
		assertEmail(t, db, "other", "ryan@ryanholt.net")
	})
}

func openMigratedTo(t *testing.T, version int64) *sql.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	if err := goose.UpTo(db, "migrations", version); err != nil {
		t.Fatalf("up to %d: %v", version, err)
	}
	return db
}

func applyRest(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("up: %v", err)
	}
}

func mustExec(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err != nil {
		t.Fatalf("exec %s: %v", query, err)
	}
}

func assertEmail(t *testing.T, db *sql.DB, username, want string) {
	t.Helper()
	user, err := models.GetUserByUsername(context.Background(), db, username)
	if err != nil {
		t.Fatalf("get %s: %v", username, err)
	}
	got := ""
	if user.Email.Valid {
		got = user.Email.String
	}
	if got != want {
		t.Fatalf("%s email = %q, want %q", username, got, want)
	}
}

func assertUserCount(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != want {
		t.Fatalf("users = %d, want %d", n, want)
	}
}
