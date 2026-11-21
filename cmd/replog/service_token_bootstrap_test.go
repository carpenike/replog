package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/models"
)

func TestBootstrapServiceToken_StoresHashOnly(t *testing.T) {
	db := openBootstrapDB(t)
	if _, err := models.CreateUser(context.Background(), db, "ryan", "", "password123", "ryan@example.com", true, true, sql.NullInt64{}); err != nil {
		t.Fatalf("create coach: %v", err)
	}

	const plaintext = "rlcst_0123456789abcdef0123456789abcdef"
	t.Setenv("REPLOG_SERVICE_TOKEN", plaintext)
	t.Setenv("REPLOG_SERVICE_USER", "ryan")
	t.Setenv("REPLOG_ADMIN_USER", "")

	if err := bootstrapServiceToken(db); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	var stored string
	if err := db.QueryRow(`SELECT token_hash FROM service_tokens`).Scan(&stored); err != nil {
		t.Fatalf("read hash: %v", err)
	}
	sum := sha256.Sum256([]byte(plaintext))
	if stored != hex.EncodeToString(sum[:]) {
		t.Fatal("stored value is not sha256(plaintext)")
	}
	if strings.Contains(stored, plaintext) {
		t.Fatal("database contains the usable token")
	}

	user, err := models.ValidateServiceToken(context.Background(), db, plaintext)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if user.Username != "ryan" {
		t.Fatalf("token user = %q, want ryan", user.Username)
	}
}

func TestBootstrapServiceToken_UnsetIsNoop(t *testing.T) {
	db := openBootstrapDB(t)
	t.Setenv("REPLOG_SERVICE_TOKEN", "")
	if err := bootstrapServiceToken(db); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM service_tokens`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("rows = %d, want 0", n)
	}
}

func TestBootstrapServiceToken_RejectsNonCoach(t *testing.T) {
	db := openBootstrapDB(t)
	if _, err := models.CreateUser(context.Background(), db, "kid", "", "password123", "kid@example.com", false, false, sql.NullInt64{}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Setenv("REPLOG_SERVICE_TOKEN", "rlcst_0123456789abcdef0123456789abcdef")
	t.Setenv("REPLOG_SERVICE_USER", "kid")
	t.Setenv("REPLOG_ADMIN_USER", "")
	if err := bootstrapServiceToken(db); err == nil {
		t.Fatal("expected non-coach rejection")
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM service_tokens`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("rows = %d, want 0", n)
	}
}

func openBootstrapDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := database.RunMigrations(db); err != nil {
		_ = db.Close()
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
