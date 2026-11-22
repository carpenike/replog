package models

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func TestServiceToken_StoresHashOnly(t *testing.T) {
	db := testDB(t)
	user := serviceTokenUser(t, db, "coach", true, true)

	plaintext, _, err := CreateServiceToken(context.Background(), db, user.ID, "agent")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.HasPrefix(plaintext, ServiceTokenPrefix) {
		t.Fatalf("plaintext %q missing prefix", plaintext)
	}

	var stored string
	if err := db.QueryRow(`SELECT token_hash FROM service_tokens WHERE user_id = ?`, user.ID).Scan(&stored); err != nil {
		t.Fatalf("read hash: %v", err)
	}
	sum := sha256.Sum256([]byte(plaintext))
	want := hex.EncodeToString(sum[:])
	if stored != want {
		t.Fatalf("stored hash = %q, want sha256(plaintext)", stored)
	}
	if strings.Contains(stored, plaintext) || stored == plaintext {
		t.Fatal("service_tokens persisted a usable token")
	}

	got, err := ValidateServiceToken(context.Background(), db, plaintext)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got.ID != user.ID {
		t.Fatalf("user id = %d, want %d", got.ID, user.ID)
	}

	var used sql.NullTime
	if err := db.QueryRow(`SELECT last_used_at FROM service_tokens WHERE user_id = ?`, user.ID).Scan(&used); err != nil {
		t.Fatalf("last_used_at: %v", err)
	}
	if !used.Valid {
		t.Fatal("expected last_used_at to be set")
	}
}

func TestServiceToken_RevokedUnknownAndRotated(t *testing.T) {
	db := testDB(t)
	user := serviceTokenUser(t, db, "coach", true, false)

	plaintext, _, err := CreateServiceToken(context.Background(), db, user.ID, "agent")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var id int64
	if err := db.QueryRow(`SELECT id FROM service_tokens WHERE user_id = ?`, user.ID).Scan(&id); err != nil {
		t.Fatalf("id: %v", err)
	}
	if err := RevokeServiceToken(context.Background(), db, id); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := ValidateServiceToken(context.Background(), db, plaintext); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked validate err = %v, want ErrNotFound", err)
	}
	if _, err := ValidateServiceToken(context.Background(), db, ServiceTokenPrefix+"0123456789abcdef"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown validate err = %v, want ErrNotFound", err)
	}

	const first = "rlcst_aaaaaaaaaaaaaaaa"
	const second = "rlcst_bbbbbbbbbbbbbbbb"
	if err := InstallCoachServiceToken(context.Background(), db, user.ID, first); err != nil {
		t.Fatalf("install first: %v", err)
	}
	if err := InstallCoachServiceToken(context.Background(), db, user.ID, second); err != nil {
		t.Fatalf("install second: %v", err)
	}
	if _, err := ValidateServiceToken(context.Background(), db, first); !errors.Is(err, ErrNotFound) {
		t.Fatalf("rotated-out token err = %v, want ErrNotFound", err)
	}
	got, err := ValidateServiceToken(context.Background(), db, second)
	if err != nil {
		t.Fatalf("rotated-in token: %v", err)
	}
	if got.ID != user.ID {
		t.Fatalf("user id = %d, want %d", got.ID, user.ID)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM service_tokens WHERE label = ?`, coachServiceTokenLabel).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("coach-service rows = %d, want 1", n)
	}
	assertNoPlaintext(t, db, first)
	assertNoPlaintext(t, db, second)
}

func TestInstallCoachServiceToken_RejectsUnprefixedSecret(t *testing.T) {
	db := testDB(t)
	user := serviceTokenUser(t, db, "coach", true, false)
	err := InstallCoachServiceToken(context.Background(), db, user.ID, "not-a-service-token-value")
	if err == nil {
		t.Fatal("expected prefix rejection")
	}
	if strings.Contains(err.Error(), "not-a-service-token-value") {
		t.Fatal("error included the plaintext secret")
	}
	if n := countRows(t, db, "service_tokens"); n != 0 {
		t.Fatalf("rows = %d, want 0", n)
	}
}

func serviceTokenUser(t *testing.T, db *sql.DB, username string, coach, admin bool) *User {
	t.Helper()
	u, err := CreateUser(context.Background(), db, username, "", "password123", username+"@example.com", coach, admin, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func assertNoPlaintext(t *testing.T, db *sql.DB, plaintext string) {
	t.Helper()
	var hash, label string
	if err := db.QueryRow(`SELECT token_hash, IFNULL(label, '') FROM service_tokens WHERE label = ?`, coachServiceTokenLabel).Scan(&hash, &label); err != nil {
		t.Fatalf("read row: %v", err)
	}
	if strings.Contains(hash, plaintext) || strings.Contains(label, plaintext) || hash == plaintext {
		t.Fatal("service_tokens persisted a usable token")
	}
}
