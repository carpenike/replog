package database

import (
	"context"
	"database/sql"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/carpenike/replog/internal/models"
)

// TestEmailBackfillIsNoOp checks that migration 0020 no longer changes any
// user row. It used to backfill one address; it is now retired.
func TestEmailBackfillIsNoOp(t *testing.T) {
	db := openMigratedTo(t, 19)
	mustExec(t, db, `INSERT INTO users (username, email, is_coach, is_admin) VALUES ('coach', NULL, 1, 1)`)
	mustExec(t, db, `INSERT INTO users (username, email, is_coach, is_admin) VALUES ('sam', 'sam@example.com', 0, 0)`)
	mustExec(t, db, `INSERT INTO users (username, email, is_coach, is_admin) VALUES ('kid', '', 0, 0)`)
	applyRest(t, db)

	assertEmail(t, db, "coach", "")
	assertEmail(t, db, "sam", "sam@example.com")
	assertEmail(t, db, "kid", "")
	assertUserCount(t, db, 3)
	if _, err := models.GetUserByUsername(context.Background(), db, "missing"); err == nil {
		t.Fatal("migration created a user")
	}

	// Down and up again stay no-ops.
	if err := goose.DownTo(db, "migrations", 19); err != nil {
		t.Fatalf("down to 19: %v", err)
	}
	assertEmail(t, db, "sam", "sam@example.com")
	applyRest(t, db)
	assertEmail(t, db, "coach", "")
	assertUserCount(t, db, 3)
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
